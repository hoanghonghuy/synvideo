package openaicompat_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaicompat"
)

const testSecret = "super-secret-api-key"

func TestNewRegistrationMapsChatCompletionRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testSecret {
			t.Errorf("authorization = %q, want bearer credential", got)
		}

		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "gpt-external-v1" {
			t.Errorf("model = %q, want gpt-external-v1", request.Model)
		}
		if len(request.Messages) != 2 || request.Messages[0].Role != "system" || request.Messages[0].Content != "Be concise" || request.Messages[1].Role != "user" || request.Messages[1].Content != "Write a hook" {
			t.Errorf("messages = %#v, want provider-neutral messages mapped in order", request.Messages)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"model": "gpt-external-v1",
			"choices": []any{
				map[string]any{"message": map[string]string{"role": "assistant", "content": "Generated hook"}},
			},
			"usage": map[string]int{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18},
		})
	}))
	defer server.Close()

	registration, err := openaicompat.NewRegistration(openaicompat.Config{
		ProviderID:       "openai-compatible",
		DisplayName:      "OpenAI Compatible",
		BaseURL:          server.URL + "/v1",
		CredentialSource: openaicompat.SecretSourceFunc(func(context.Context) (string, error) { return testSecret, nil }),
		Models: []openaicompat.ModelConfig{{
			ID:              "hook-writer",
			DisplayName:     "Hook Writer",
			ExternalModelID: "gpt-external-v1",
		}},
	})
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}

	response, err := registration.Models[0].TextGenerator.GenerateText(context.Background(), providers.TextGenerationRequest{
		ProviderID: "openai-compatible",
		ModelID:    "hook-writer",
		Messages: []providers.TextMessage{
			{Role: "system", Content: "Be concise"},
			{Role: "user", Content: "Write a hook"},
		},
	})
	if err != nil {
		t.Fatalf("generate text: %v", err)
	}
	if response.ProviderID != "openai-compatible" || response.ModelID != "hook-writer" {
		t.Fatalf("stable response identity = %#v", response)
	}
	if response.Text != "Generated hook" {
		t.Fatalf("text = %q, want Generated hook", response.Text)
	}
	if response.Usage.InputTokens == nil || *response.Usage.InputTokens != 11 || response.Usage.OutputTokens == nil || *response.Usage.OutputTokens != 7 || response.Usage.TotalTokens == nil || *response.Usage.TotalTokens != 18 {
		t.Fatalf("usage = %#v, want mapped token usage", response.Usage)
	}
}

func TestAdapterClassifiesUpstreamFailuresWithoutLeakingCredentialOrBody(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "upstream body contains "+testSecret)
			}))
			defer server.Close()

			generator := newTestGenerator(t, server.URL, 0)
			_, err := generator.GenerateText(context.Background(), testRequest())
			if !errors.Is(err, providers.ErrProviderUnavailable) {
				t.Fatalf("error = %v, want provider unavailable", err)
			}
			assertSecretFree(t, err)
		})
	}
}

func TestAdapterClassifiesCredentialSourceFailureWithoutLeakingSourceError(t *testing.T) {
	registration, err := openaicompat.NewRegistration(openaicompat.Config{
		ProviderID:  "provider",
		DisplayName: "Provider",
		BaseURL:     "https://example.test/v1",
		CredentialSource: openaicompat.SecretSourceFunc(func(context.Context) (string, error) {
			return "", errors.New("secret lookup failed for " + testSecret)
		}),
		Models: []openaicompat.ModelConfig{{ID: "model", DisplayName: "Model", ExternalModelID: "external-model"}},
	})
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}

	_, err = registration.Models[0].TextGenerator.GenerateText(context.Background(), testRequest())
	if !errors.Is(err, providers.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want provider unavailable", err)
	}
	assertSecretFree(t, err)
}

func TestAdapterRejectsMalformedOrEmptySuccessResponsesSafely(t *testing.T) {
	for name, body := range map[string]string{
		"invalid json":  "{",
		"empty choices": `{"choices":[]}`,
		"empty content": `{"choices":[{"message":{"role":"assistant","content":""}}]}`,
		"non assistant": `{"choices":[{"message":{"role":"user","content":"not accepted"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()

			_, err := newTestGenerator(t, server.URL, 0).GenerateText(context.Background(), testRequest())
			if !errors.Is(err, providers.ErrProviderExecution) {
				t.Fatalf("error = %v, want provider execution", err)
			}
			assertSecretFree(t, err)
		})
	}
}

func TestAdapterPropagatesContextDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := newTestGenerator(t, server.URL, 0).GenerateText(ctx, testRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
}

func TestAdapterRejectsResponsesOverConfiguredLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"`+strings.Repeat("x", 200)+`"}}]}`)
	}))
	defer server.Close()

	_, err := newTestGenerator(t, server.URL, 64).GenerateText(context.Background(), testRequest())
	if !errors.Is(err, providers.ErrProviderExecution) {
		t.Fatalf("error = %v, want provider execution", err)
	}
}

func TestAdapterDoesNotFollowRedirectsWithCredential(t *testing.T) {
	var redirectedRequests int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests++
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("redirect target received authorization header %q", got)
		}
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	_, err := newTestGenerator(t, redirect.URL, 0).GenerateText(context.Background(), testRequest())
	if !errors.Is(err, providers.ErrProviderExecution) {
		t.Fatalf("error = %v, want provider execution for redirect response", err)
	}
	if redirectedRequests != 0 {
		t.Fatalf("redirect target requests = %d, want 0", redirectedRequests)
	}
	assertSecretFree(t, err)
}

func TestNewRegistrationValidatesConfigAndProducesTextOnlyStableMetadata(t *testing.T) {
	tests := map[string]openaicompat.Config{
		"missing provider": {
			DisplayName: "Provider", BaseURL: "https://example.test", CredentialSource: testSecretSource(), Models: []openaicompat.ModelConfig{{ID: "model", DisplayName: "Model", ExternalModelID: "external"}},
		},
		"invalid base url": {
			ProviderID: "provider", DisplayName: "Provider", BaseURL: "not a url", CredentialSource: testSecretSource(), Models: []openaicompat.ModelConfig{{ID: "model", DisplayName: "Model", ExternalModelID: "external"}},
		},
		"duplicate model": {
			ProviderID: "provider", DisplayName: "Provider", BaseURL: "https://example.test", CredentialSource: testSecretSource(), Models: []openaicompat.ModelConfig{{ID: "model", DisplayName: "Model", ExternalModelID: "one"}, {ID: "model", DisplayName: "Model 2", ExternalModelID: "two"}},
		},
		"empty models": {
			ProviderID: "provider", DisplayName: "Provider", BaseURL: "https://example.test", CredentialSource: testSecretSource(),
		},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := openaicompat.NewRegistration(config); err == nil {
				t.Fatal("expected invalid configuration error")
			}
		})
	}

	registration, err := openaicompat.NewRegistration(openaicompat.Config{
		ProviderID:       "provider",
		DisplayName:      "Provider",
		BaseURL:          "https://example.test/v1",
		CredentialSource: testSecretSource(),
		Models: []openaicompat.ModelConfig{
			{ID: "model-b", DisplayName: "Model B", ExternalModelID: "external-b"},
			{ID: "model-a", DisplayName: "Model A", ExternalModelID: "external-a"},
		},
	})
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}
	if registration.Provider.ID != "provider" || registration.Provider.DisplayName != "Provider" {
		t.Fatalf("provider metadata = %#v", registration.Provider)
	}
	if len(registration.Models) != 2 {
		t.Fatalf("model count = %d, want 2", len(registration.Models))
	}
	for _, model := range registration.Models {
		if !model.Metadata.Supports(providers.CapabilityTextGeneration) || model.TextGenerator == nil {
			t.Fatalf("model registration = %#v, want text capability and binding", model)
		}
		if strings.Contains(model.Metadata.DisplayName, "external") {
			t.Fatalf("external model name leaked into metadata: %#v", model.Metadata)
		}
	}
}

func TestAdapterDoesNotRetryUpstreamFailure(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, _ = newTestGenerator(t, server.URL, 0).GenerateText(context.Background(), testRequest())
	if requests != 1 {
		t.Fatalf("upstream requests = %d, want exactly 1", requests)
	}
}

func newTestGenerator(t *testing.T, baseURL string, maxResponseBytes int64) providers.TextGenerator {
	t.Helper()
	registration, err := openaicompat.NewRegistration(openaicompat.Config{
		ProviderID:       "provider",
		DisplayName:      "Provider",
		BaseURL:          baseURL,
		CredentialSource: testSecretSource(),
		MaxResponseBytes: maxResponseBytes,
		Models:           []openaicompat.ModelConfig{{ID: "model", DisplayName: "Model", ExternalModelID: "external-model"}},
	})
	if err != nil {
		t.Fatalf("new test registration: %v", err)
	}
	return registration.Models[0].TextGenerator
}

func testSecretSource() openaicompat.SecretSource {
	return openaicompat.SecretSourceFunc(func(context.Context) (string, error) { return testSecret, nil })
}

func testRequest() providers.TextGenerationRequest {
	return providers.TextGenerationRequest{
		ProviderID: "provider",
		ModelID:    "model",
		Messages:   []providers.TextMessage{{Role: "user", Content: "hello"}},
	}
}

func assertSecretFree(t *testing.T, err error) {
	t.Helper()
	if strings.Contains(err.Error(), testSecret) {
		t.Fatalf("error leaked credential: %q", err)
	}
	var boundaryErr *providers.BoundaryError
	if errors.As(err, &boundaryErr) && strings.Contains(boundaryErr.PresentationMessage(), testSecret) {
		t.Fatalf("presentation error leaked credential: %q", boundaryErr.PresentationMessage())
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

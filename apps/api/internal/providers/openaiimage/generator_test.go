package openaiimage_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaiimage"
)

const imageSecret = "super-secret-image-key"

var (
	pngBytes  = append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, []byte("png")...)
	jpegBytes = append([]byte{0xff, 0xd8, 0xff}, []byte("jpeg")...)
	webpBytes = append([]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}, []byte("webp")...)
)

func TestGenerateImageMapsCurrentModelPromptCountAndAspectAndDecodesBytes(t *testing.T) {
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Errorf("request = %s %s, want POST /v1/images/generations", r.Method, r.URL.Path)
		}
		var request struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			N      int    `json:"n"`
			Size   string `json:"size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "gpt-image-2" || request.Prompt != "draw a mountain" || request.N != 2 || request.Size != "1536x1024" {
			t.Errorf("request = %#v, want current model and conservative mapped fields", request)
		}
		writeImageResponse(w, imageResponse{ID: "img-request-1", Data: []imageData{
			{B64JSON: base64.StdEncoding.EncodeToString(pngBytes)},
			{B64JSON: base64.StdEncoding.EncodeToString(jpegBytes)},
		}, Usage: &imageUsage{InputTokens: intPtr(9), OutputTokens: intPtr(12), TotalTokens: intPtr(21)}})
	}))
	defer server.Close()

	registration, err := openaiimage.NewRegistration(openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		BaseURL:          server.URL + "/v1",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return imageSecret, nil }),
		Models:           []openaiimage.ModelConfig{{ID: "scene-image", DisplayName: "Scene Image", ExternalModelID: "gpt-image-2"}},
	})
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}
	count := 2
	response, err := registration.Models[0].ImageGenerator.GenerateImage(context.Background(), providers.ImageGenerationRequest{
		Prompt: "draw a mountain", AspectRatio: "16:9", OutputCount: &count,
	})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if gotAuthorization != "Bearer "+imageSecret {
		t.Fatalf("authorization = %q, want injected bearer credential", gotAuthorization)
	}
	if response.RequestID != "img-request-1" || len(response.Outputs) != 2 {
		t.Fatalf("response metadata = %#v, want request ID and two outputs", response)
	}
	if response.Usage.InputTokens == nil || *response.Usage.InputTokens != 9 || response.Usage.OutputTokens == nil || *response.Usage.OutputTokens != 12 {
		t.Fatalf("usage = %#v, want mapped image usage", response.Usage)
	}
	for i, expected := range []struct {
		mime string
		data []byte
	}{
		{"image/png", pngBytes},
		{"image/jpeg", jpegBytes},
	} {
		if response.Outputs[i].Binary.MIMEType() != expected.mime || response.Outputs[i].Binary.Size() != int64(len(expected.data)) {
			t.Fatalf("output %d metadata = %s/%d, want %s/%d", i, response.Outputs[i].Binary.MIMEType(), response.Outputs[i].Binary.Size(), expected.mime, len(expected.data))
		}
		reader, err := response.Outputs[i].Binary.Open(context.Background())
		if err != nil {
			t.Fatalf("open output %d: %v", i, err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil || string(data) != string(expected.data) {
			t.Fatalf("output %d data = %q/%v, want fixture", i, data, err)
		}
	}
}

func TestGenerateImageDefaultResponseBoundSupportsConfiguredDecodedImageBound(t *testing.T) {
	largePNG := append(append([]byte(nil), pngBytes...), bytes.Repeat([]byte{'x'}, 1<<20)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeImageResponse(w, imageResponse{Data: []imageData{{B64JSON: base64.StdEncoding.EncodeToString(largePNG)}}})
	}))
	defer server.Close()

	response, err := newGenerator(t, server.URL+"/v1", nil).GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if len(response.Outputs) != 1 || response.Outputs[0].Binary.Size() != int64(len(largePNG)) {
		t.Fatalf("output = %#v, want decoded image larger than 1 MiB", response.Outputs)
	}
}

func TestGenerateImageRejectsUnsupportedHintsAndReferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("upstream should not be called") }))
	defer server.Close()
	generator := newGenerator(t, server.URL+"/v1", nil)
	negative := "no text"
	seed := int64(7)
	for name, request := range map[string]providers.ImageGenerationRequest{
		"negative prompt": {Prompt: "hello", NegativePrompt: negative},
		"seed":            {Prompt: "hello", Seed: &seed},
		"references":      {Prompt: "hello", ReferenceImages: []providers.BinaryInput{mustInput(t)}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := generator.GenerateImage(context.Background(), request)
			if !errors.Is(err, openaiimage.ErrUnsupportedParameter) {
				t.Fatalf("error = %v, want unsupported parameter", err)
			}
		})
	}
}

func TestGenerateImageRejectsInvalidResponsesAndAggregateBounds(t *testing.T) {
	tests := map[string]struct {
		response imageResponse
		config   func(*openaiimage.Config)
	}{
		"empty data":       {response: imageResponse{}},
		"invalid base64":   {response: imageResponse{Data: []imageData{{B64JSON: "%%%"}}}},
		"non-image bytes":  {response: imageResponse{Data: []imageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("not image"))}}}},
		"too many outputs": {response: imageResponse{Data: []imageData{{B64JSON: base64.StdEncoding.EncodeToString(pngBytes)}, {B64JSON: base64.StdEncoding.EncodeToString(pngBytes)}}}, config: func(c *openaiimage.Config) { c.MaxOutputCount = 1 }},
		"aggregate bytes":  {response: imageResponse{Data: []imageData{{B64JSON: base64.StdEncoding.EncodeToString(pngBytes)}, {B64JSON: base64.StdEncoding.EncodeToString(jpegBytes)}}}, config: func(c *openaiimage.Config) { c.MaxAggregateImageBytes = int64(len(pngBytes) + len(jpegBytes) - 1) }},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeImageResponse(w, test.response) }))
			defer server.Close()
			generator := newGenerator(t, server.URL+"/v1", test.config)
			_, err := generator.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello", OutputCount: intPtr(1)})
			if !errors.Is(err, providers.ErrMalformedResponse) {
				t.Fatalf("error = %v, want malformed response", err)
			}
		})
	}
}

func TestGenerateImageRejectsOversizedAndTrailingResponses(t *testing.T) {
	for name, body := range map[string]string{
		"oversized JSON": `{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(pngBytes) + `"}]}`,
		"trailing JSON":  `{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(pngBytes) + `"}]} trailing-secret`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			customize := func(config *openaiimage.Config) {
				if name == "oversized JSON" {
					config.MaxResponseBytes = 8
				}
			}
			_, err := newGenerator(t, server.URL+"/v1", customize).GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"})
			if !errors.Is(err, providers.ErrMalformedResponse) || strings.Contains(err.Error(), "trailing-secret") {
				t.Fatalf("error = %v, want safe malformed response", err)
			}
		})
	}
}

func TestGenerateImageClosesResponseBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(pngBytes) + `"}]}`)}
	adapter, err := openaiimage.New(openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		BaseURL:          "https://api.openai.com/v1",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return imageSecret, nil }),
		Models:           []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
		})},
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	generator, err := adapter.ForModel("image")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	if _, err := generator.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"}); err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestGenerateImageMapsStatusesWithoutLeakingBodyOrSecret(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadRequest, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "upstream body contains "+imageSecret)
			}))
			defer server.Close()
			_, err := newGenerator(t, server.URL+"/v1", nil).GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"})
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				if !errors.Is(err, providers.ErrAuthenticationUnavailable) {
					t.Fatalf("error = %v, want authentication error", err)
				}
			} else if status == http.StatusTooManyRequests {
				if !errors.Is(err, providers.ErrRateLimited) {
					t.Fatalf("error = %v, want rate limit error", err)
				}
			} else if status >= http.StatusInternalServerError {
				if !errors.Is(err, providers.ErrTransientExecution) {
					t.Fatalf("error = %v, want transient error", err)
				}
			} else if !errors.Is(err, providers.ErrInvalidRequest) {
				t.Fatalf("error = %v, want invalid request error", err)
			}
			if strings.Contains(err.Error(), imageSecret) || strings.Contains(err.Error(), "upstream body") {
				t.Fatalf("error leaked upstream data: %v", err)
			}
		})
	}
}

func TestGenerateImagePropagatesCancellationAndDoesNotFollowRedirect(t *testing.T) {
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("authorization forwarded to redirect target")
		}
		writeImageResponse(w, imageResponse{Data: []imageData{{B64JSON: base64.StdEncoding.EncodeToString(pngBytes)}}})
	}))
	defer redirectTarget.Close()
	redirectSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/v1/images/generations", http.StatusFound)
	}))
	defer redirectSource.Close()
	_, err := newGenerator(t, redirectSource.URL+"/v1", nil).GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"})
	if err == nil || redirected.Load() != 0 {
		t.Fatalf("redirect error = %v, target calls = %d", err, redirected.Load())
	}

	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer blocked.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = newGenerator(t, blocked.URL+"/v1", nil).GenerateImage(ctx, providers.ImageGenerationRequest{Prompt: "hello"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled error = %v, want context.Canceled", err)
	}

	deadlineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer deadlineServer.Close()
	deadlineCtx, deadlineCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer deadlineCancel()
	_, err = newGenerator(t, deadlineServer.URL+"/v1", nil).GenerateImage(deadlineCtx, providers.ImageGenerationRequest{Prompt: "hello"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v, want context.DeadlineExceeded", err)
	}
}

func newGenerator(t *testing.T, baseURL string, customize func(*openaiimage.Config)) providers.ImageGenerator {
	t.Helper()
	config := openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		BaseURL:          baseURL,
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return imageSecret, nil }),
		Models:           []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
	}
	if customize != nil {
		customize(&config)
	}
	if baseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	adapter, err := openaiimage.New(config)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	generator, err := adapter.ForModel("image")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	if generator == nil {
		t.Fatal("expected image generator")
	}
	return generator
}

type imageData struct {
	B64JSON string `json:"b64_json,omitempty"`
	URL     string `json:"url,omitempty"`
}
type imageUsage struct {
	InputTokens  *int `json:"input_tokens,omitempty"`
	OutputTokens *int `json:"output_tokens,omitempty"`
	TotalTokens  *int `json:"total_tokens,omitempty"`
}
type imageResponse struct {
	ID    string      `json:"id,omitempty"`
	Data  []imageData `json:"data,omitempty"`
	Usage *imageUsage `json:"usage,omitempty"`
}

func writeImageResponse(w http.ResponseWriter, response imageResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
func intPtr(value int) *int { return &value }
func mustInput(t *testing.T) providers.BinaryInput {
	t.Helper()
	input, err := providers.NewBinaryInput("image/png", pngBytes)
	if err != nil {
		t.Fatalf("new input: %v", err)
	}
	return input
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type trackingBody struct {
	*strings.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

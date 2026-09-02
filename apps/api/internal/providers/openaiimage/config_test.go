package openaiimage_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaiimage"
)

func TestNewRegistrationIsDeterministicAndBindsImageCapability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	config := openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		BaseURL:          server.URL + "/v1",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return "key", nil }),
		Models: []openaiimage.ModelConfig{
			{ID: "image-z", DisplayName: "Z", ExternalModelID: "gpt-image-2"},
			{ID: "image-a", DisplayName: "A", ExternalModelID: "gpt-image-2-2026-04-21"},
		},
	}

	registration, err := openaiimage.NewRegistration(config)
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}
	if len(registration.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(registration.Models))
	}
	if registration.Models[0].Metadata.ID != "image-a" || registration.Models[1].Metadata.ID != "image-z" {
		t.Fatalf("model order = %#v, want stable ID order", registration.Models)
	}
	for _, model := range registration.Models {
		if !model.Metadata.Supports(providers.CapabilityImageGeneration) {
			t.Fatalf("model %q does not advertise image capability", model.Metadata.ID)
		}
		if model.ImageGenerator == nil || model.TextGenerator != nil || model.VideoGenerator != nil {
			t.Fatalf("model %q has unexpected capability bindings: %#v", model.Metadata.ID, model)
		}
	}
}

func TestNewRejectsUnsafeOrInvalidConfiguration(t *testing.T) {
	tests := map[string]func(*openaiimage.Config){
		"missing credential source": func(c *openaiimage.Config) { c.CredentialSource = nil },
		"missing model":             func(c *openaiimage.Config) { c.Models = nil },
		"invalid model mapping":     func(c *openaiimage.Config) { c.Models[0].ExternalModelID = " gpt-image-2" },
		"negative response bound":   func(c *openaiimage.Config) { c.MaxResponseBytes = -1 },
		"negative image bound":      func(c *openaiimage.Config) { c.MaxDecodedImageBytes = -1 },
		"zero output bound":         func(c *openaiimage.Config) { c.MaxOutputCount = -1 },
		"http public endpoint":      func(c *openaiimage.Config) { c.BaseURL = "http://api.example.com/v1" },
		"userinfo":                  func(c *openaiimage.Config) { c.BaseURL = "https://user:pass@example.test/v1" },
		"query":                     func(c *openaiimage.Config) { c.BaseURL = "https://example.test/v1?key=value" },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := validConfig()
			mutate(&config)
			if _, err := openaiimage.New(config); !errors.Is(err, openaiimage.ErrInvalidConfiguration) {
				t.Fatalf("error = %v, want invalid configuration", err)
			}
			if err := openaiimage.ValidateBaseURL(config.BaseURL); name == "http public endpoint" && err == nil {
				t.Fatal("expected public HTTP URL to be rejected")
			}
		})
	}
}

func validConfig() openaiimage.Config {
	return openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return "key", nil }),
		Models:           []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
	}
}

func TestNewUsesCanonicalDefaultBaseURL(t *testing.T) {
	config := openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return "key", nil }),
		Models:           []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
	}
	if _, err := openaiimage.New(config); err != nil {
		t.Fatalf("new with default base URL: %v", err)
	}
}

func TestCredentialErrorsAreSafeAndContextIsPreserved(t *testing.T) {
	secret := "sentinel-api-key"
	adapter, err := openaiimage.New(openaiimage.Config{
		ProviderID:  "openai",
		DisplayName: "OpenAI Images",
		BaseURL:     "https://api.openai.com/v1",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) {
			return "", errors.New("lookup failed for " + secret)
		}),
		Models: []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	generator, err := adapter.ForModel("image")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}

	_, err = generator.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "hello"})
	if !errors.Is(err, providers.ErrAuthenticationUnavailable) || strings.Contains(err.Error(), secret) {
		t.Fatalf("credential error = %v, want safe authentication error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = generator.GenerateImage(ctx, providers.ImageGenerationRequest{Prompt: "hello"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled credential error = %v, want context.Canceled", err)
	}
}

func TestNewRejectsNonPositiveConfiguredTimeout(t *testing.T) {
	config := openaiimage.Config{
		ProviderID:       "openai",
		DisplayName:      "OpenAI Images",
		CredentialSource: openaiimage.SecretSourceFunc(func(context.Context) (string, error) { return "key", nil }),
		Models:           []openaiimage.ModelConfig{{ID: "image", DisplayName: "Image", ExternalModelID: "gpt-image-2"}},
		Timeout:          -time.Nanosecond,
	}
	if _, err := openaiimage.New(config); err == nil {
		t.Fatal("expected negative timeout to be rejected")
	}
}

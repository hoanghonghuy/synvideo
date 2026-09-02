package openaitts_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaitts"
)

func TestNewRegistrationBindsConfiguredTTSModelsAndVoices(t *testing.T) {
	config := validConfig()
	config.Models = []openaitts.ModelConfig{
		{ID: "model-z", DisplayName: "Z", ExternalModelID: "gpt-4o-mini-tts"},
		{ID: "model-a", DisplayName: "A", ExternalModelID: "gpt-4o-mini-tts"},
	}
	registration, err := openaitts.NewRegistration(config)
	if err != nil {
		t.Fatalf("new registration: %v", err)
	}
	if len(registration.Models) != 2 || registration.Models[0].Metadata.ID != "model-a" {
		t.Fatalf("registration models = %#v", registration.Models)
	}
	for _, model := range registration.Models {
		if !model.Metadata.Supports(providers.CapabilityTTS) || model.SpeechSynthesizer == nil {
			t.Fatalf("model %q is not TTS-bound: %#v", model.Metadata.ID, model)
		}
	}
}

func TestNewRejectsInvalidBoundsAndUnsafeBaseURL(t *testing.T) {
	tests := map[string]func(*openaitts.Config){
		"missing credential":        func(c *openaitts.Config) { c.CredentialSource = nil },
		"missing models":            func(c *openaitts.Config) { c.Models = nil },
		"missing voices":            func(c *openaitts.Config) { c.Voices = nil },
		"zero input bound":          func(c *openaitts.Config) { c.MaxInputRunes = -1 },
		"negative input byte bound": func(c *openaitts.Config) { c.MaxInputBytes = -1 },
		"zero response bound":       func(c *openaitts.Config) { c.MaxResponseBytes = -1 },
		"public http":               func(c *openaitts.Config) { c.BaseURL = "http://api.example.com/v1" },
		"non-loopback test domain":  func(c *openaitts.Config) { c.BaseURL = "http://api.test/v1" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := validConfig()
			mutate(&config)
			if _, err := openaitts.New(config); !errors.Is(err, openaitts.ErrInvalidConfiguration) {
				t.Fatalf("error = %v, want invalid configuration", err)
			}
		})
	}
}

func validConfig() openaitts.Config {
	return openaitts.Config{
		ProviderID:  "openai",
		DisplayName: "OpenAI Speech",
		CredentialSource: openaitts.SecretSourceFunc(func(context.Context) (string, error) {
			return "test-api-key", nil
		}),
		Models: []openaitts.ModelConfig{{
			ID: "speech", DisplayName: "Speech", ExternalModelID: "gpt-4o-mini-tts",
		}},
		Voices: []openaitts.VoiceConfig{{
			ID: "narrator", DisplayName: "Narrator", ExternalVoice: "alloy", Locale: "en-US",
		}},
	}
}

func TestNewUsesCanonicalDefaultBaseURL(t *testing.T) {
	if _, err := openaitts.New(validConfig()); err != nil {
		t.Fatalf("new with default base URL: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	config := validConfig()
	config.BaseURL = server.URL + "/v1"
	if _, err := openaitts.New(config); err != nil {
		t.Fatalf("new with local base URL: %v", err)
	}

	for _, localURL := range []string{
		"http://localhost:8080/v1",
		"http://127.0.0.1:8080/v1",
		"http://[::1]:8080/v1",
	} {
		cfg := validConfig()
		cfg.BaseURL = localURL
		if _, err := openaitts.New(cfg); err != nil {
			t.Fatalf("new with local URL %q: %v", localURL, err)
		}
	}
}

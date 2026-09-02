package providers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestRegistryResolvesTTSCapabilityWithoutChangingLegacyBindings(t *testing.T) {
	tts := fake.NewSpeechSynthesizer([]byte("speech"))
	text := fake.NewTextGenerator("text")
	registry := providers.NewRegistry()
	voice := providers.VoiceMetadata{ID: "narrator", DisplayName: "Narrator", Locale: "vi-VN"}
	if err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{ID: "synvideo-lab", DisplayName: "Lab"},
		Models: []providers.ModelRegistration{{
			Metadata: providers.ModelMetadata{
				ProviderID:            "synvideo-lab",
				ID:                    "multimodal",
				DisplayName:           "Multimodal",
				SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration, providers.CapabilityTTS},
			},
			TextGenerator:     text,
			SpeechSynthesizer: tts,
		}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	resolved, metadata, err := registry.ResolveSpeechSynthesizer("synvideo-lab", "multimodal")
	if err != nil || resolved != tts {
		t.Fatalf("resolve TTS = %v, %v", resolved, err)
	}
	if !metadata.Supports(providers.CapabilityTTS) {
		t.Fatal("resolved metadata lost TTS capability")
	}
	if resolved == nil {
		t.Fatal("resolved TTS is nil")
	}
	if _, _, err := registry.ResolveTextGenerator("synvideo-lab", "multimodal"); err != nil {
		t.Fatalf("legacy text resolution changed: %v", err)
	}
	_ = voice
}

func TestRegistryRequiresMatchingTTSSynthesizerBinding(t *testing.T) {
	registry := providers.NewRegistry()
	err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{ID: "provider", DisplayName: "Provider"},
		Models: []providers.ModelRegistration{{
			Metadata: providers.ModelMetadata{
				ProviderID:            "provider",
				ID:                    "model",
				DisplayName:           "Model",
				SupportedCapabilities: []providers.Capability{providers.CapabilityTTS},
			},
		}},
	})
	if err == nil {
		t.Fatal("TTS capability without binding should be rejected")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = registry.ResolveSpeechSynthesizer("missing", "model")
	if !errors.Is(err, providers.ErrUnknownProvider) {
		t.Fatalf("unknown provider error = %v", err)
	}
	_ = ctx
}

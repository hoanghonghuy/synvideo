package main

import (
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

const task017LegacyProviderDefinitions = `[
  {
    "provider_id": "openai",
    "display_name": "OpenAI",
    "base_url": "https://api.openai.com/v1",
    "models": [
      {
        "model_id": "gpt-5-mini",
        "display_name": "GPT-5 mini",
        "external_model_id": "gpt-5-mini"
      }
    ]
  }
]`

func TestLoadProviderCatalogUnifiedConfigFailsClosedWhenCapabilityMissing(t *testing.T) {
	_, err := loadProviderCatalog(config.Config{ProviderDefinitions: task017LegacyProviderDefinitions})
	if err == nil {
		t.Fatal("expected unified provider definitions without capabilities to be rejected")
	}
}

func TestLoadProviderCatalogLegacyConfigDefaultsCapabilityToText(t *testing.T) {
	catalog, err := loadProviderCatalog(config.Config{TextProviderDefinitions: task017LegacyProviderDefinitions})
	if err != nil {
		t.Fatalf("load legacy provider definitions: %v", err)
	}

	providers := catalog.Providers()
	if len(providers) != 1 || len(providers[0].Models) != 1 {
		t.Fatalf("unexpected legacy catalog shape: %#v", providers)
	}
	caps := providers[0].Models[0].Capabilities
	if len(caps) != 1 || caps[0] != providersettings.CapabilityText {
		t.Fatalf("legacy model capabilities = %#v, want [text]", caps)
	}
}

package providersettings_test

import (
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

func TestCatalog_ValidJSON(t *testing.T) {
	jsonConfig := `[
		{
			"provider_id": "openai",
			"display_name": "OpenAI",
			"base_url": "https://api.openai.com/v1",
			"models": [
				{
					"model_id": "gpt-5-mini",
					"display_name": "GPT-5 mini",
					"external_model_id": "gpt-5-mini-2025-08-01"
				},
				{
					"model_id": "gpt-4o",
					"display_name": "GPT-4o",
					"external_model_id": "gpt-4o"
				}
			]
		},
		{
			"provider_id": "openrouter",
			"display_name": "OpenRouter",
			"base_url": "https://openrouter.ai/api/v1",
			"models": [
				{
					"model_id": "claude-3-5-sonnet",
					"display_name": "Claude 3.5 Sonnet",
					"external_model_id": "anthropic/claude-3.5-sonnet"
				}
			]
		}
	]`

	catalog, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
	if err != nil {
		t.Fatalf("unexpected error parsing catalog: %v", err)
	}

	if len(catalog.Providers()) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(catalog.Providers()))
	}

	p, ok := catalog.GetProvider(providers.ProviderID("openai"))
	if !ok {
		t.Fatalf("expected openai provider in catalog")
	}
	if p.DisplayName != "OpenAI" || len(p.Models) != 2 {
		t.Fatalf("unexpected provider details: %+v", p)
	}

	m, ok := catalog.GetModel(providers.ProviderID("openai"), providers.ModelID("gpt-5-mini"))
	if !ok {
		t.Fatalf("expected gpt-5-mini model in catalog")
	}
	if m.ExternalModelID != "gpt-5-mini-2025-08-01" {
		t.Fatalf("expected external model ID gpt-5-mini-2025-08-01, got %s", m.ExternalModelID)
	}
}

func TestCatalog_ValidationErrors(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		_, err := providersettings.NewCatalogFromJSON([]byte(`{not-json}`))
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})

	t.Run("duplicate provider ID", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI 1",
				"base_url": "https://api.openai.com/v1",
				"models": [{"model_id": "m1", "display_name": "M1", "external_model_id": "m1"}]
			},
			{
				"provider_id": "openai",
				"display_name": "OpenAI 2",
				"base_url": "https://api.openai.com/v1",
				"models": [{"model_id": "m2", "display_name": "M2", "external_model_id": "m2"}]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for duplicate provider id")
		}
	})

	t.Run("duplicate model ID under same provider", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com/v1",
				"models": [
					{"model_id": "m1", "display_name": "M1", "external_model_id": "m1-a"},
					{"model_id": "m1", "display_name": "M1 duplicate", "external_model_id": "m1-b"}
				]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for duplicate model id")
		}
	})

	t.Run("invalid base url with query", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com/v1?token=injected",
				"models": [{"model_id": "m1", "display_name": "M1", "external_model_id": "m1"}]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for base url with query parameters")
		}
	})

	t.Run("invalid base url with user info", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://user:secret@api.openai.com/v1",
				"models": [{"model_id": "m1", "display_name": "M1", "external_model_id": "m1"}]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for base url with user info")
		}
	})

	t.Run("invalid base url with fragment", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com/v1#section",
				"models": [{"model_id": "m1", "display_name": "M1", "external_model_id": "m1"}]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for base url with fragment")
		}
	})

	t.Run("invalid base url with unsupported scheme", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "ftp://api.openai.com/v1",
				"models": [{"model_id": "m1", "display_name": "M1", "external_model_id": "m1"}]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for base url with ftp scheme")
		}
	})

	t.Run("empty models list", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com/v1",
				"models": []
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for empty models list")
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		_, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
			{
				ProviderID:  "openai",
				DisplayName: "OpenAI",
				BaseURL:     "https://api.openai.com/v1",
				Timeout:     -5,
				Models:      []providersettings.ModelDefinition{{ModelID: "m1", DisplayName: "M1", ExternalModelID: "m1"}},
			},
		})
		if err == nil {
			t.Fatal("expected error for negative timeout")
		}
	})

	t.Run("negative max_response_bytes", func(t *testing.T) {
		_, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
			{
				ProviderID:       "openai",
				DisplayName:      "OpenAI",
				BaseURL:          "https://api.openai.com/v1",
				MaxResponseBytes: -100,
				Models:           []providersettings.ModelDefinition{{ModelID: "m1", DisplayName: "M1", ExternalModelID: "m1"}},
			},
		})
		if err == nil {
			t.Fatal("expected error for negative max_response_bytes")
		}
	})

	t.Run("external_model_id with leading or trailing whitespace", func(t *testing.T) {
		jsonConfig := `[
			{
				"provider_id": "openai",
				"display_name": "OpenAI",
				"base_url": "https://api.openai.com/v1",
				"models": [
					{"model_id": "m1", "display_name": "M1", "external_model_id": "  gpt-4o  "}
				]
			}
		]`
		_, err := providersettings.NewCatalogFromJSON([]byte(jsonConfig))
		if err == nil {
			t.Fatal("expected error for external_model_id with whitespace")
		}
	})
}

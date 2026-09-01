package providersettings_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

type memorySettingsRepo struct {
	settings map[string]providersettings.Setting
}

func newMemorySettingsRepo() *memorySettingsRepo {
	return &memorySettingsRepo{
		settings: make(map[string]providersettings.Setting),
	}
}

func (m *memorySettingsRepo) key(ownerID uuid.UUID, providerID providers.ProviderID) string {
	return ownerID.String() + ":" + string(providerID)
}

func (m *memorySettingsRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]providersettings.Setting, error) {
	var result []providersettings.Setting
	prefix := ownerID.String() + ":"
	for k, v := range m.settings {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *memorySettingsRepo) GetByOwnerAndProvider(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID) (providersettings.Setting, error) {
	s, ok := m.settings[m.key(ownerID, providerID)]
	if !ok {
		return providersettings.Setting{}, providersettings.ErrSettingNotFound
	}
	return s, nil
}

func (m *memorySettingsRepo) Save(ctx context.Context, setting providersettings.Setting, expectedRevision *int) (providersettings.Setting, error) {
	k := m.key(setting.OwnerID, setting.ProviderID)
	existing, exists := m.settings[k]

	if expectedRevision == nil {
		if exists {
			return providersettings.Setting{}, providersettings.ErrStaleRevision
		}
		setting.Revision = 1
		m.settings[k] = setting
		return setting, nil
	}

	if !exists {
		return providersettings.Setting{}, providersettings.ErrSettingNotFound
	}
	if existing.Revision != *expectedRevision {
		return providersettings.Setting{}, providersettings.ErrStaleRevision
	}

	setting.Revision = existing.Revision + 1
	m.settings[k] = setting
	return setting, nil
}

func (m *memorySettingsRepo) Delete(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, expectedRevision int) error {
	k := m.key(ownerID, providerID)
	existing, exists := m.settings[k]
	if !exists {
		return providersettings.ErrSettingNotFound
	}
	if existing.Revision != expectedRevision {
		return providersettings.ErrStaleRevision
	}
	delete(m.settings, k)
	return nil
}

func testCatalog(t *testing.T) *providersettings.Catalog {
	t.Helper()
	cat, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
		{
			ProviderID:  "openai",
			DisplayName: "OpenAI",
			BaseURL:     "https://api.openai.com/v1",
			Models: []providersettings.ModelDefinition{
				{
					ModelID:         "gpt-5-mini",
					DisplayName:     "GPT-5 mini",
					ExternalModelID: "gpt-5-mini",
				},
				{
					ModelID:         "gpt-4o",
					DisplayName:     "GPT-4o",
					ExternalModelID: "gpt-4o",
				},
			},
		},
		{
			ProviderID:  "openrouter",
			DisplayName: "OpenRouter",
			BaseURL:     "https://openrouter.ai/api/v1",
			Models: []providersettings.ModelDefinition{
				{
					ModelID:         "claude-3-5-sonnet",
					DisplayName:     "Claude 3.5 Sonnet",
					ExternalModelID: "anthropic/claude-3.5-sonnet",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create test catalog: %v", err)
	}
	return cat
}

func TestService_ListSettings(t *testing.T) {
	cat := testCatalog(t)
	cipher, _ := providersettings.NewAESGCMCipher(generateTestKey(t), "v1")
	repo := newMemorySettingsRepo()
	svc := providersettings.NewService(cat, repo, cipher, nil)

	ownerID := uuid.New()
	ctx := context.Background()

	// Initial list: all providers unconfigured
	resp, err := svc.ListSettings(ctx, ownerID)
	if err != nil {
		t.Fatalf("initial list: %v", err)
	}
	if len(resp.Providers) != 2 {
		t.Fatalf("expected 2 providers in list, got %d", len(resp.Providers))
	}
	for _, p := range resp.Providers {
		if p.Configured || p.Enabled || p.HasAPIKey || p.Revision != 0 {
			t.Fatalf("expected unconfigured provider, got %+v", p)
		}
		for _, m := range p.Models {
			if m.Enabled {
				t.Fatalf("expected unconfigured model disabled, got %+v", m)
			}
		}
	}
}

func TestService_PutSettingLifecycle(t *testing.T) {
	cat := testCatalog(t)
	cipher, _ := providersettings.NewAESGCMCipher(generateTestKey(t), "v1")
	repo := newMemorySettingsRepo()
	svc := providersettings.NewService(cat, repo, cipher, nil)

	ownerID := uuid.New()
	ctx := context.Background()
	providerID := providers.ProviderID("openai")
	apiKey := "sk-valid-test-key-12345"

	t.Run("first create requires api key", func(t *testing.T) {
		_, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
			APIKey:          nil,
		})
		if !errors.Is(err, providersettings.ErrCredentialRequired) {
			t.Fatalf("expected ErrCredentialRequired for initial create without key, got %v", err)
		}
	})

	t.Run("first create rejects leading/trailing whitespace", func(t *testing.T) {
		whitespaceKey := "  sk-key-with-spaces  "
		_, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
			APIKey:          &whitespaceKey,
		})
		if !errors.Is(err, providersettings.ErrInvalidSettingInput) {
			t.Fatalf("expected ErrInvalidSettingInput for whitespace key, got %v", err)
		}
	})

	t.Run("first create rejects non-nil revision", func(t *testing.T) {
		revZero := 0
		unconfProvider := providers.ProviderID("openrouter")
		_, err := svc.PutSetting(ctx, ownerID, unconfProvider, providersettings.PutSettingInput{
			Revision:        &revZero,
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"claude-3-5-sonnet"},
			APIKey:          &apiKey,
		})
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision when revision provided on first create, got %v", err)
		}
	})

	t.Run("successful initial create", func(t *testing.T) {
		view, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
			APIKey:          &apiKey,
		})
		if err != nil {
			t.Fatalf("put initial setting: %v", err)
		}
		if !view.Configured || !view.Enabled || !view.HasAPIKey || view.Revision != 1 {
			t.Fatalf("unexpected view after create: %+v", view)
		}
		if len(view.Models) != 2 || !view.Models[0].Enabled || view.Models[1].Enabled {
			t.Fatalf("unexpected models view: %+v", view.Models)
		}
	})

	t.Run("update preserving existing key", func(t *testing.T) {
		rev := 1
		view, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Revision:        &rev,
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini", "gpt-4o"},
			APIKey:          nil, // preserve key
		})
		if err != nil {
			t.Fatalf("update setting preserving key: %v", err)
		}
		if view.Revision != 2 || !view.HasAPIKey {
			t.Fatalf("expected revision 2 with has_api_key true, got %+v", view)
		}
		if !view.Models[0].Enabled || !view.Models[1].Enabled {
			t.Fatalf("expected both models enabled after update: %+v", view.Models)
		}
	})

	t.Run("update rotating key", func(t *testing.T) {
		rev := 2
		newKey := "sk-rotated-key-67890"
		view, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Revision:        &rev,
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
			APIKey:          &newKey,
		})
		if err != nil {
			t.Fatalf("rotate key: %v", err)
		}
		if view.Revision != 3 || !view.HasAPIKey {
			t.Fatalf("expected revision 3 with has_api_key true, got %+v", view)
		}
	})

	t.Run("stale revision update is rejected", func(t *testing.T) {
		oldRev := 1
		_, err := svc.PutSetting(ctx, ownerID, providerID, providersettings.PutSettingInput{
			Revision:        &oldRev,
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		})
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision for old revision, got %v", err)
		}
	})

	t.Run("delete setting with revision", func(t *testing.T) {
		// Wrong revision fails
		err := svc.DeleteSetting(ctx, ownerID, providerID, 1)
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision on delete, got %v", err)
		}

		// Correct revision 3 succeeds
		err = svc.DeleteSetting(ctx, ownerID, providerID, 3)
		if err != nil {
			t.Fatalf("delete setting: %v", err)
		}

		// Options now empty for owner
		options, err := svc.GetOwnerTextGenerationOptions(ctx, ownerID)
		if err != nil {
			t.Fatalf("get options after delete: %v", err)
		}
		if len(options.Providers) != 0 {
			t.Fatalf("expected 0 options after delete, got %d", len(options.Providers))
		}
	})
}

func TestService_OwnerIsolation(t *testing.T) {
	cat := testCatalog(t)
	cipher, _ := providersettings.NewAESGCMCipher(generateTestKey(t), "v1")
	repo := newMemorySettingsRepo()
	svc := providersettings.NewService(cat, repo, cipher, nil)

	ownerA := uuid.New()
	ownerB := uuid.New()
	ctx := context.Background()
	providerID := providers.ProviderID("openai")
	keyA := "sk-owner-a-key"

	// Configure ownerA
	_, err := svc.PutSetting(ctx, ownerA, providerID, providersettings.PutSettingInput{
		Enabled:         true,
		EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		APIKey:          &keyA,
	})
	if err != nil {
		t.Fatalf("configure ownerA: %v", err)
	}

	// Owner B list sees provider as unconfigured
	listB, err := svc.ListSettings(ctx, ownerB)
	if err != nil {
		t.Fatalf("list ownerB: %v", err)
	}
	for _, p := range listB.Providers {
		if p.ID == providerID && p.Configured {
			t.Fatalf("ownerB should see unconfigured provider, got configured")
		}
	}

	// Owner B cannot resolve generator using ownerA's credential
	_, err = svc.ResolveTextGenerator(ctx, ownerB, providerID, "gpt-5-mini")
	if !errors.Is(err, providers.ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable for unconfigured ownerB, got %v", err)
	}

	// Owner A can resolve generator
	genA, err := svc.ResolveTextGenerator(ctx, ownerA, providerID, "gpt-5-mini")
	if err != nil {
		t.Fatalf("resolve ownerA generator: %v", err)
	}
	if genA == nil {
		t.Fatalf("expected non-nil generator for ownerA")
	}
}

func TestService_LiveOpenAICompatResolution(t *testing.T) {
	var capturedAuthHeader string
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-5-mini",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "{\"title_options\":[\"Test Video\"]}"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer upstreamServer.Close()

	cat, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
		{
			ProviderID:  "openai",
			DisplayName: "OpenAI",
			BaseURL:     upstreamServer.URL,
			Models: []providersettings.ModelDefinition{
				{
					ModelID:         "gpt-5-mini",
					DisplayName:     "GPT-5 mini",
					ExternalModelID: "gpt-5-mini",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create catalog with upstream: %v", err)
	}

	cipher, _ := providersettings.NewAESGCMCipher(generateTestKey(t), "v1")
	repo := newMemorySettingsRepo()
	svc := providersettings.NewService(cat, repo, cipher, upstreamServer.Client())

	ownerA := uuid.New()
	ctx := context.Background()
	secretKey := "sk-live-creator-credential-test"

	_, err = svc.PutSetting(ctx, ownerA, "openai", providersettings.PutSettingInput{
		Enabled:         true,
		EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		APIKey:          &secretKey,
	})
	if err != nil {
		t.Fatalf("configure ownerA: %v", err)
	}

	gen, err := svc.ResolveTextGenerator(ctx, ownerA, "openai", "gpt-5-mini")
	if err != nil {
		t.Fatalf("resolve generator: %v", err)
	}

	resp, err := gen.GenerateText(ctx, providers.TextGenerationRequest{
		ProviderID: "openai",
		ModelID:    "gpt-5-mini",
		Messages: []providers.TextMessage{
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("generate text: %v", err)
	}
	if resp.Text != `{"title_options":["Test Video"]}` {
		t.Fatalf("unexpected text response: %s", resp.Text)
	}
	if capturedAuthHeader != "Bearer "+secretKey {
		t.Fatalf("expected Authorization header Bearer %s, got %s", secretKey, capturedAuthHeader)
	}
}

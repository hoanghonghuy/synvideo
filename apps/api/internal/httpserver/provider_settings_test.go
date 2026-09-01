package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

type fakeProviderSettingsService struct {
	listFn   func(ctx context.Context, ownerID uuid.UUID) (providersettings.ProviderSettingsListResponse, error)
	putFn    func(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, input providersettings.PutSettingInput) (providersettings.ProviderSettingView, error)
	deleteFn func(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, revision int) error
}

func (f *fakeProviderSettingsService) ListSettings(ctx context.Context, ownerID uuid.UUID) (providersettings.ProviderSettingsListResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, ownerID)
	}
	return providersettings.ProviderSettingsListResponse{}, nil
}

func (f *fakeProviderSettingsService) PutSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, input providersettings.PutSettingInput) (providersettings.ProviderSettingView, error) {
	if f.putFn != nil {
		return f.putFn(ctx, ownerID, providerID, input)
	}
	return providersettings.ProviderSettingView{}, nil
}

func (f *fakeProviderSettingsService) DeleteSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, revision int) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, ownerID, providerID, revision)
	}
	return nil
}

func TestProviderSettingsHTTP(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	resolver := fixedResolver{ownerID: ownerID}

	t.Run("GET /api/v1/ai/provider-settings success", func(t *testing.T) {
		svc := &fakeProviderSettingsService{
			listFn: func(ctx context.Context, oID uuid.UUID) (providersettings.ProviderSettingsListResponse, error) {
				return providersettings.ProviderSettingsListResponse{
					Providers: []providersettings.ProviderSettingView{
						{
							ID:          "openai",
							DisplayName: "OpenAI",
							Configured:  true,
							Enabled:     true,
							HasAPIKey:   true,
							Revision:    2,
							Models: []providersettings.ModelSettingView{
								{ID: "gpt-5-mini", DisplayName: "GPT-5 mini", Enabled: true},
							},
						},
					},
				}, nil
			},
		}

		server := New(
			config.Config{Environment: config.EnvironmentTest, Addr: ":8080"},
			nil, nil, nil, nil, nil, nil, nil,
			svc,
			nil, nil,
			resolver,
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/provider-settings", nil)
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp providersettings.ProviderSettingsListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(resp.Providers) != 1 || resp.Providers[0].ID != "openai" || !resp.Providers[0].HasAPIKey {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("PUT /api/v1/ai/provider-settings/{provider_id} success", func(t *testing.T) {
		key := "sk-test-creator-key"
		svc := &fakeProviderSettingsService{
			putFn: func(ctx context.Context, oID uuid.UUID, providerID providers.ProviderID, input providersettings.PutSettingInput) (providersettings.ProviderSettingView, error) {
				return providersettings.ProviderSettingView{
					ID:          providerID,
					DisplayName: "OpenAI",
					Configured:  true,
					Enabled:     true,
					HasAPIKey:   true,
					Revision:    1,
					Models: []providersettings.ModelSettingView{
						{ID: "gpt-5-mini", DisplayName: "GPT-5 mini", Enabled: true},
					},
				}, nil
			},
		}

		server := New(
			config.Config{Environment: config.EnvironmentTest, Addr: ":8080"},
			nil, nil, nil, nil, nil, nil, nil,
			svc,
			nil, nil,
			resolver,
		)

		body, _ := json.Marshal(providersettings.PutSettingInput{
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
			APIKey:          &key,
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/ai/provider-settings/openai", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Ensure plaintext sentinel is not in response
		if bytes.Contains(rec.Body.Bytes(), []byte(key)) {
			t.Fatalf("response JSON leaked submitted API key!")
		}
	})

	t.Run("PUT returns 409 on stale revision", func(t *testing.T) {
		svc := &fakeProviderSettingsService{
			putFn: func(ctx context.Context, oID uuid.UUID, providerID providers.ProviderID, input providersettings.PutSettingInput) (providersettings.ProviderSettingView, error) {
				return providersettings.ProviderSettingView{}, providersettings.ErrStaleRevision
			},
		}

		server := New(
			config.Config{Environment: config.EnvironmentTest, Addr: ":8080"},
			nil, nil, nil, nil, nil, nil, nil,
			svc,
			nil, nil,
			resolver,
		)

		rev := 1
		body, _ := json.Marshal(providersettings.PutSettingInput{
			Revision:        &rev,
			Enabled:         true,
			EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/ai/provider-settings/openai", bytes.NewReader(body))
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})

	t.Run("DELETE /api/v1/ai/provider-settings/{provider_id} requires revision query param", func(t *testing.T) {
		svc := &fakeProviderSettingsService{}
		server := New(
			config.Config{Environment: config.EnvironmentTest, Addr: ":8080"},
			nil, nil, nil, nil, nil, nil, nil,
			svc,
			nil, nil,
			resolver,
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/provider-settings/openai", nil)
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 when revision is missing, got %d", rec.Code)
		}
	})

	t.Run("DELETE /api/v1/ai/provider-settings/{provider_id}?revision=2 success", func(t *testing.T) {
		var deletedRev int
		svc := &fakeProviderSettingsService{
			deleteFn: func(ctx context.Context, oID uuid.UUID, providerID providers.ProviderID, revision int) error {
				deletedRev = revision
				return nil
			},
		}

		server := New(
			config.Config{Environment: config.EnvironmentTest, Addr: ":8080"},
			nil, nil, nil, nil, nil, nil, nil,
			svc,
			nil, nil,
			resolver,
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/provider-settings/openai?revision=2", nil)
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", rec.Code)
		}
		if deletedRev != 2 {
			t.Fatalf("expected delete revision 2, got %d", deletedRev)
		}
	})
}

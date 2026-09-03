package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeSceneNarrationBindingService struct {
	listCurrentFn func(context.Context, project.Principal, uuid.UUID, int) ([]scenenarration.CurrentSceneNarration, error)
	assignFn      func(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenenarration.Binding, error)
	listHistoryFn func(context.Context, project.Principal, uuid.UUID, int, string) ([]scenenarration.Binding, error)
}

func (s *fakeSceneNarrationBindingService) ListCurrent(ctx context.Context, p project.Principal, projID uuid.UUID, v int) ([]scenenarration.CurrentSceneNarration, error) {
	if s.listCurrentFn != nil {
		return s.listCurrentFn(ctx, p, projID, v)
	}
	return nil, nil
}

func (s *fakeSceneNarrationBindingService) AssignNarration(ctx context.Context, p project.Principal, projID uuid.UUID, v int, key string, aID uuid.UUID) (scenenarration.Binding, error) {
	if s.assignFn != nil {
		return s.assignFn(ctx, p, projID, v, key, aID)
	}
	return scenenarration.Binding{}, nil
}

func (s *fakeSceneNarrationBindingService) ListHistory(ctx context.Context, p project.Principal, projID uuid.UUID, v int, key string) ([]scenenarration.Binding, error) {
	if s.listHistoryFn != nil {
		return s.listHistoryFn(ctx, p, projID, v, key)
	}
	return nil, nil
}

type testAssetService struct {
	asset mediaasset.MediaAsset
}

func (t *testAssetService) Store(_ context.Context, _ project.Principal, _ uuid.UUID, _ mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	return t.asset, nil
}

func (t *testAssetService) Get(_ context.Context, _ project.Principal, _, _ uuid.UUID) (mediaasset.MediaAsset, error) {
	return t.asset, nil
}

func (t *testAssetService) List(_ context.Context, _ project.Principal, _ uuid.UUID, _ int) (mediaasset.ListResult, error) {
	return mediaasset.ListResult{Assets: []mediaasset.MediaAsset{t.asset}}, nil
}

func (t *testAssetService) Open(_ context.Context, _ project.Principal, _, _ uuid.UUID) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content")), nil
}

func (t *testAssetService) OpenRange(_ context.Context, _ project.Principal, _, _ uuid.UUID, _, _ int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content")), nil
}

func (t *testAssetService) Delete(_ context.Context, _ project.Principal, _, _ uuid.UUID) error {
	return nil
}

func TestSceneNarrationBindingHandler(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	resolver := actor.NewLocalResolver(config.Config{Environment: config.EnvironmentTest, LocalActorID: &ownerID})

	assetService := &testAssetService{
		asset: mediaasset.MediaAsset{
			ID:        assetID,
			OwnerID:   ownerID,
			ProjectID: projectID,
			Kind:      mediaasset.KindAudio,
			Origin:    mediaasset.OriginGeneratedAudio,
			MimeType:  "audio/mpeg",
		},
	}

	t.Run("listCurrent returns 200 with scene narrations", func(t *testing.T) {
		bindingService := &fakeSceneNarrationBindingService{
			listCurrentFn: func(_ context.Context, p project.Principal, projID uuid.UUID, v int) ([]scenenarration.CurrentSceneNarration, error) {
				return []scenenarration.CurrentSceneNarration{
					{
						Scene: sceneplan.Scene{Key: "sc-1", Narration: "Lời dẫn 1"},
						Binding: &scenenarration.Binding{
							ID:               uuid.New(),
							OwnerID:          ownerID,
							ProjectID:        projID,
							ScenePlanVersion: 1,
							SceneKey:         "sc-1",
							Role:             scenenarration.RoleNarration,
							BindingVersion:   1,
							AssetID:          assetID,
							Status:           scenenarration.StatusActive,
							CreatedAt:        time.Now().UTC(),
						},
					},
				}, nil
			},
		}

		handler := sceneNarrationHandler{
			bindings:      bindingService,
			assets:        assetService,
			actorResolver: resolver,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/narration-bindings", nil)
		req.SetPathValue("id", projectID.String())
		req.SetPathValue("version", "1")

		rr := httptest.NewRecorder()
		handler.listCurrent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("assign returns 200 with binding response", func(t *testing.T) {
		bindingService := &fakeSceneNarrationBindingService{
			assignFn: func(_ context.Context, p project.Principal, projID uuid.UUID, v int, key string, aID uuid.UUID) (scenenarration.Binding, error) {
				return scenenarration.Binding{
					ID:               uuid.New(),
					OwnerID:          ownerID,
					ProjectID:        projID,
					ScenePlanVersion: v,
					SceneKey:         key,
					Role:             scenenarration.RoleNarration,
					BindingVersion:   1,
					AssetID:          aID,
					Status:           scenenarration.StatusActive,
					CreatedAt:        time.Now().UTC(),
				}, nil
			},
		}

		handler := sceneNarrationHandler{
			bindings:      bindingService,
			assets:        assetService,
			actorResolver: resolver,
		}

		body := map[string]any{"asset_id": assetID.String()}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/scenes/sc-1/narration", bytes.NewReader(bodyBytes))
		req.SetPathValue("id", projectID.String())
		req.SetPathValue("version", "1")
		req.SetPathValue("scene_key", "sc-1")

		rr := httptest.NewRecorder()
		handler.assign(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("history returns 200 with list of bindings", func(t *testing.T) {
		bindingService := &fakeSceneNarrationBindingService{
			listHistoryFn: func(_ context.Context, p project.Principal, projID uuid.UUID, v int, key string) ([]scenenarration.Binding, error) {
				return []scenenarration.Binding{
					{
						ID:               uuid.New(),
						OwnerID:          ownerID,
						ProjectID:        projID,
						ScenePlanVersion: v,
						SceneKey:         key,
						Role:             scenenarration.RoleNarration,
						BindingVersion:   1,
						AssetID:          assetID,
						Status:           scenenarration.StatusActive,
						CreatedAt:        time.Now().UTC(),
					},
				}, nil
			},
		}

		handler := sceneNarrationHandler{
			bindings:      bindingService,
			assets:        assetService,
			actorResolver: resolver,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/scenes/sc-1/narration/history", nil)
		req.SetPathValue("id", projectID.String())
		req.SetPathValue("version", "1")
		req.SetPathValue("scene_key", "sc-1")

		rr := httptest.NewRecorder()
		handler.history(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

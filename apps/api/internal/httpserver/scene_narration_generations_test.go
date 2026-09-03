package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type fakeSceneNarrationJobService struct {
	createFn func(context.Context, project.Principal, uuid.UUID, int, string, scenenarrationjob.CreateGenerationInput) (scenenarrationjob.JobView, error)
	getFn    func(context.Context, project.Principal, uuid.UUID, uuid.UUID) (scenenarrationjob.JobView, error)
}

func (s *fakeSceneNarrationJobService) CreateGeneration(ctx context.Context, p project.Principal, projID uuid.UUID, version int, sceneKey string, input scenenarrationjob.CreateGenerationInput) (scenenarrationjob.JobView, error) {
	if s.createFn != nil {
		return s.createFn(ctx, p, projID, version, sceneKey, input)
	}
	return scenenarrationjob.JobView{}, nil
}

func (s *fakeSceneNarrationJobService) GetGeneration(ctx context.Context, p project.Principal, projID, jobID uuid.UUID) (scenenarrationjob.JobView, error) {
	if s.getFn != nil {
		return s.getFn(ctx, p, projID, jobID)
	}
	return scenenarrationjob.JobView{}, nil
}

func TestSceneNarrationGenerationHandler(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	assetID := uuid.New()
	resolver := actor.NewLocalResolver(config.Config{Environment: config.EnvironmentTest, LocalActorID: &ownerID})

	t.Run("create returns 202 with job view", func(t *testing.T) {
		service := &fakeSceneNarrationJobService{
			createFn: func(_ context.Context, p project.Principal, projID uuid.UUID, version int, sceneKey string, input scenenarrationjob.CreateGenerationInput) (scenenarrationjob.JobView, error) {
				if projID != projectID || version != 1 || sceneKey != "sc-1" || input.RequestID != jobID {
					t.Fatalf("unexpected create args")
				}
				return scenenarrationjob.JobView{
					ID:                jobID,
					State:             "queued",
					Attempt:           0,
					MaxAttempts:       3,
					MediaAssetID:      &assetID,
					DurationSeconds:   2.5,
					AssignedNarration: true,
					CreatedAt:         time.Now().UTC(),
					UpdatedAt:         time.Now().UTC(),
				}, nil
			},
		}

		handler := sceneNarrationGenerationHandler{
			service:       service,
			actorResolver: resolver,
		}

		body := map[string]any{
			"request_id":     jobID.String(),
			"provider_id":    "openai",
			"model_id":       "tts-1",
			"voice_id":       "voice-nova",
			"format":         "mp3",
			"assign_current": true,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/scenes/sc-1/narration-generations", bytes.NewReader(bodyBytes))
		req.SetPathValue("id", projectID.String())
		req.SetPathValue("version", "1")
		req.SetPathValue("scene_key", "sc-1")

		rr := httptest.NewRecorder()
		handler.create(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp sceneNarrationJobResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response error: %v", err)
		}
		if resp.ID != jobID.String() || resp.State != "queued" || !resp.AssignedNarration || resp.DurationSeconds != 2.5 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("get returns 200 with job view", func(t *testing.T) {
		service := &fakeSceneNarrationJobService{
			getFn: func(_ context.Context, p project.Principal, projID, jID uuid.UUID) (scenenarrationjob.JobView, error) {
				if projID != projectID || jID != jobID {
					t.Fatalf("unexpected get args")
				}
				return scenenarrationjob.JobView{
					ID:                jobID,
					State:             "succeeded",
					Attempt:           1,
					MaxAttempts:       3,
					MediaAssetID:      &assetID,
					DurationSeconds:   3.2,
					AssignedNarration: true,
					CreatedAt:         time.Now().UTC(),
					UpdatedAt:         time.Now().UTC(),
				}, nil
			},
		}

		handler := sceneNarrationGenerationHandler{
			service:       service,
			actorResolver: resolver,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/narration-generations/"+jobID.String(), nil)
		req.SetPathValue("id", projectID.String())
		req.SetPathValue("job_id", jobID.String())

		rr := httptest.NewRecorder()
		handler.get(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

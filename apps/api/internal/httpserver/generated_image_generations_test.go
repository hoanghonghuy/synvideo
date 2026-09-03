package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/generatedimagejob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type fakeGeneratedImageGenerationService struct {
	createFn func(context.Context, project.Principal, uuid.UUID, int, string, generatedimagejob.CreateGenerationInput) (generatedimagejob.JobView, error)
	getFn    func(context.Context, project.Principal, uuid.UUID, uuid.UUID) (generatedimagejob.JobView, error)
}

func (f fakeGeneratedImageGenerationService) CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, sceneKey string, input generatedimagejob.CreateGenerationInput) (generatedimagejob.JobView, error) {
	return f.createFn(ctx, principal, projectID, version, sceneKey, input)
}

func (f fakeGeneratedImageGenerationService) GetGeneration(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (generatedimagejob.JobView, error) {
	return f.getFn(ctx, principal, projectID, jobID)
}

func TestGeneratedImageGenerationEndpoints(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	jobID := uuid.New()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	resolver := actor.NewLocalResolver(config.Config{Environment: config.EnvironmentTest, LocalActorID: &ownerID})

	t.Run("POST returns accepted durable job view", func(t *testing.T) {
		service := fakeGeneratedImageGenerationService{
			createFn: func(_ context.Context, principal project.Principal, gotProjectID uuid.UUID, version int, sceneKey string, input generatedimagejob.CreateGenerationInput) (generatedimagejob.JobView, error) {
				if principal.OwnerID != ownerID || gotProjectID != projectID || version != 2 || sceneKey != "intro" {
					t.Fatalf("unexpected scope: principal=%v project=%s version=%d scene=%s", principal, gotProjectID, version, sceneKey)
				}
				if input.RequestID != requestID || input.ProviderID != "openai" || input.ModelID != "image-1" || !input.AssignPrimaryVisual {
					t.Fatalf("unexpected input: %+v", input)
				}
				return generatedimagejob.JobView{ID: requestID, State: string(jobs.StateQueued), Attempt: 0, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now}, nil
			},
		}
		server := New(config.Config{Environment: config.EnvironmentTest}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolver, MediaServices{GeneratedImages: service})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plans/2/scenes/intro/image-generations", bytes.NewBufferString(`{"request_id":"`+requestID.String()+`","provider_id":"openai","model_id":"image-1","assign_primary_visual":true}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.Handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response generatedImageJobResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.ID != requestID.String() || response.State != string(jobs.StateQueued) || response.MaxAttempts != 3 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GET returns durable result fields", func(t *testing.T) {
		assetID := uuid.New()
		service := fakeGeneratedImageGenerationService{
			getFn: func(_ context.Context, principal project.Principal, gotProjectID, gotJobID uuid.UUID) (generatedimagejob.JobView, error) {
				if principal.OwnerID != ownerID || gotProjectID != projectID || gotJobID != jobID {
					t.Fatalf("unexpected lookup: principal=%v project=%s job=%s", principal, gotProjectID, gotJobID)
				}
				return generatedimagejob.JobView{ID: jobID, State: string(jobs.StateSucceeded), Attempt: 1, MaxAttempts: 3, MediaAssetID: &assetID, AssignedPrimaryVisual: true, CreatedAt: now, UpdatedAt: now}, nil
			},
		}
		server := New(config.Config{Environment: config.EnvironmentTest}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolver, MediaServices{GeneratedImages: service})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/image-generations/"+jobID.String(), nil)
		recorder := httptest.NewRecorder()

		server.Handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response generatedImageJobResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.MediaAssetID == nil || *response.MediaAssetID != assetID.String() || !response.AssignedPrimaryVisual {
			t.Fatalf("unexpected result response: %+v", response)
		}
	})

	t.Run("provider unavailable is mapped to safe client error", func(t *testing.T) {
		service := fakeGeneratedImageGenerationService{
			createFn: func(context.Context, project.Principal, uuid.UUID, int, string, generatedimagejob.CreateGenerationInput) (generatedimagejob.JobView, error) {
				return generatedimagejob.JobView{}, errors.Join(generatedimagejob.ErrProviderUnavailable, errors.New("credential sentinel"))
			},
		}
		server := New(config.Config{Environment: config.EnvironmentTest}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolver, MediaServices{GeneratedImages: service})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plans/2/scenes/intro/image-generations", bytes.NewBufferString(`{"request_id":"`+requestID.String()+`","provider_id":"openai","model_id":"image-1"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.Handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if stringBody := recorder.Body.String(); stringBody == "" || strings.Contains(stringBody, "credential sentinel") {
			t.Fatalf("unsafe error response: %s", stringBody)
		}
	})
}

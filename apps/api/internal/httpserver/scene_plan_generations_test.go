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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangenerationjob"
)

type fakeScenePlanGenService struct {
	createFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, input sceneplangenerationjob.CreateScenePlanGenerationInput) (sceneplangenerationjob.ScenePlanGenerationJobView, error)
	getFn    func(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (sceneplangenerationjob.ScenePlanGenerationJobView, error)
}

func (f fakeScenePlanGenService) CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, input sceneplangenerationjob.CreateScenePlanGenerationInput) (sceneplangenerationjob.ScenePlanGenerationJobView, error) {
	if f.createFn != nil {
		return f.createFn(ctx, principal, projectID, input)
	}
	return sceneplangenerationjob.ScenePlanGenerationJobView{}, nil
}

func (f fakeScenePlanGenService) GetGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (sceneplangenerationjob.ScenePlanGenerationJobView, error) {
	if f.getFn != nil {
		return f.getFn(ctx, principal, projectID, jobID)
	}
	return sceneplangenerationjob.ScenePlanGenerationJobView{}, nil
}

func TestScenePlanGenerationEndpoints(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	jobID := uuid.New()
	now := time.Now().UTC()

	t.Run("POST /api/v1/projects/{project_id}/scene-plan-generations success returns 202 and safe fields", func(t *testing.T) {
		service := fakeScenePlanGenService{
			createFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input sceneplangenerationjob.CreateScenePlanGenerationInput) (sceneplangenerationjob.ScenePlanGenerationJobView, error) {
				if principal.OwnerID != ownerID || pID != projectID || input.RequestID != requestID {
					t.Fatalf("unexpected parameters: principal=%v, pID=%s, requestID=%s", principal, pID, input.RequestID)
				}
				return sceneplangenerationjob.ScenePlanGenerationJobView{
					ID:          requestID,
					State:       string(jobs.StateQueued),
					Attempt:     0,
					MaxAttempts: 3,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			},
		}

		resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		reqBody := `{"request_id":"` + requestID.String() + `","provider_id":"fake-provider","model_id":"fake-model"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plan-generations", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if resp["id"] != requestID.String() {
			t.Fatalf("expected job id %s, got %v", requestID, resp["id"])
		}
		if resp["state"] != string(jobs.StateQueued) {
			t.Fatalf("expected state queued, got %v", resp["state"])
		}

		// Ensure unsafe fields are never exposed
		for _, unsafe := range []string{"payload", "result", "lease_token", "lease_deadline", "api_key", "started_at", "finished_at"} {
			if _, exists := resp[unsafe]; exists {
				t.Fatalf("unsafe field %q exposed in response JSON!", unsafe)
			}
		}
	})

	t.Run("POST returns domain specific error codes", func(t *testing.T) {
		testCases := []struct {
			name           string
			errToReturn    error
			expectedStatus int
			expectedCode   string
		}{
			{
				name:           "script approval required",
				errToReturn:    sceneplangenerationjob.ErrScriptApprovalRequired,
				expectedStatus: http.StatusConflict,
				expectedCode:   "SCRIPT_APPROVAL_REQUIRED",
			},
			{
				name:           "source invalid",
				errToReturn:    sceneplangenerationjob.ErrScenePlanSourceInvalid,
				expectedStatus: http.StatusConflict,
				expectedCode:   "SCENE_PLAN_SOURCE_INVALID",
			},
			{
				name:           "provider unavailable",
				errToReturn:    sceneplangenerationjob.ErrProviderUnavailable,
				expectedStatus: http.StatusBadRequest,
				expectedCode:   "GENERATION_PROVIDER_UNAVAILABLE",
			},
			{
				name:           "request conflict",
				errToReturn:    sceneplangenerationjob.ErrGenerationRequestConflict,
				expectedStatus: http.StatusConflict,
				expectedCode:   "GENERATION_REQUEST_CONFLICT",
			},
			{
				name:           "project not found",
				errToReturn:    sceneplangenerationjob.ErrProjectNotFound,
				expectedStatus: http.StatusNotFound,
				expectedCode:   "not_found",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				service := fakeScenePlanGenService{
					createFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input sceneplangenerationjob.CreateScenePlanGenerationInput) (sceneplangenerationjob.ScenePlanGenerationJobView, error) {
						return sceneplangenerationjob.ScenePlanGenerationJobView{}, tc.errToReturn
					},
				}
				resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
				server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, service, resolver)

				reqBody := `{"request_id":"` + requestID.String() + `","provider_id":"fake-provider","model_id":"fake-model"}`
				req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plan-generations", bytes.NewBufferString(reqBody))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				server.Handler.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Fatalf("expected status %d, got %d: %s", tc.expectedStatus, rec.Code, rec.Body.String())
				}

				var errEnv errorEnvelope
				if err := json.Unmarshal(rec.Body.Bytes(), &errEnv); err != nil {
					t.Fatalf("unmarshal error envelope: %v", err)
				}
				if errEnv.Error.Code != tc.expectedCode {
					t.Fatalf("expected error code %q, got %q", tc.expectedCode, errEnv.Error.Code)
				}
			})
		}
	})

	t.Run("GET /api/v1/projects/{project_id}/scene-plan-generations/{job_id} success and not found", func(t *testing.T) {
		v := 3
		service := fakeScenePlanGenService{
			getFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, jID uuid.UUID) (sceneplangenerationjob.ScenePlanGenerationJobView, error) {
				if jID == jobID {
					return sceneplangenerationjob.ScenePlanGenerationJobView{
						ID:               jobID,
						State:            string(jobs.StateSucceeded),
						Attempt:          1,
						MaxAttempts:      3,
						ScenePlanVersion: &v,
						CreatedAt:        now,
						UpdatedAt:        now,
					}, nil
				}
				return sceneplangenerationjob.ScenePlanGenerationJobView{}, sceneplangenerationjob.ErrJobNotFound
			},
		}

		resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plan-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp scenePlanGenerationJobResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.ID != jobID.String() || resp.State != string(jobs.StateSucceeded) || resp.ScenePlanVersion == nil || *resp.ScenePlanVersion != 3 {
			t.Fatalf("unexpected response: %+v", resp)
		}

		// 404
		reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plan-generations/"+uuid.New().String(), nil)
		recNotFound := httptest.NewRecorder()
		server.Handler.ServeHTTP(recNotFound, reqNotFound)
		if recNotFound.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", recNotFound.Code)
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		service := fakeScenePlanGenService{}
		resolver := actor.NewLocalResolver(config.Config{Environment: config.EnvironmentProduction})
		server := New(config.Config{Environment: config.EnvironmentProduction}, nil, nil, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plan-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

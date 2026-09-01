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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgenerationjob"
)

type fakeScriptGenService struct {
	createGenFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, input scriptgenerationjob.CreateScriptGenerationInput) (scriptgenerationjob.ScriptGenerationJobView, error)
	getGenFn    func(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (scriptgenerationjob.ScriptGenerationJobView, error)
}

func (f fakeScriptGenService) CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, input scriptgenerationjob.CreateScriptGenerationInput) (scriptgenerationjob.ScriptGenerationJobView, error) {
	if f.createGenFn != nil {
		return f.createGenFn(ctx, principal, projectID, input)
	}
	return scriptgenerationjob.ScriptGenerationJobView{}, nil
}

func (f fakeScriptGenService) GetGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (scriptgenerationjob.ScriptGenerationJobView, error) {
	if f.getGenFn != nil {
		return f.getGenFn(ctx, principal, projectID, jobID)
	}
	return scriptgenerationjob.ScriptGenerationJobView{}, nil
}

func TestCreateScriptGenerationEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	requestID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	t.Run("valid request returns 202 with safe view", func(t *testing.T) {
		service := fakeScriptGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input scriptgenerationjob.CreateScriptGenerationInput) (scriptgenerationjob.ScriptGenerationJobView, error) {
				if principal.OwnerID != ownerID || pID != projectID || input.RequestID != requestID {
					t.Fatalf("unexpected arguments: owner=%s project=%s req=%s", principal.OwnerID, pID, input.RequestID)
				}
				now := time.Now().UTC()
				return scriptgenerationjob.ScriptGenerationJobView{
					ID:          requestID,
					State:       "queued",
					Attempt:     0,
					MaxAttempts: 3,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			},
		}

		resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		reqBody := `{"request_id":"` + requestID.String() + `","provider_id":"fake-provider","model_id":"fake-model"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/script-generations", bytes.NewBufferString(reqBody))
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
		if resp["id"] != requestID.String() || resp["state"] != "queued" {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("error mappings: proposal required, provider unavailable, conflict", func(t *testing.T) {
		testCases := []struct {
			name         string
			svcErr       error
			expectedCode int
			expectedErr  string
		}{
			{
				name:         "approved proposal required",
				svcErr:       scriptgenerationjob.ErrApprovedProposalRequired,
				expectedCode: http.StatusConflict,
				expectedErr:  "APPROVED_PROPOSAL_REQUIRED",
			},
			{
				name:         "provider unavailable",
				svcErr:       scriptgenerationjob.ErrProviderUnavailable,
				expectedCode: http.StatusBadRequest,
				expectedErr:  "GENERATION_PROVIDER_UNAVAILABLE",
			},
			{
				name:         "generation request conflict",
				svcErr:       scriptgenerationjob.ErrGenerationRequestConflict,
				expectedCode: http.StatusConflict,
				expectedErr:  "GENERATION_REQUEST_CONFLICT",
			},
			{
				name:         "project not found",
				svcErr:       scriptgenerationjob.ErrProjectNotFound,
				expectedCode: http.StatusNotFound,
				expectedErr:  "not_found",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				service := fakeScriptGenService{
					createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input scriptgenerationjob.CreateScriptGenerationInput) (scriptgenerationjob.ScriptGenerationJobView, error) {
						return scriptgenerationjob.ScriptGenerationJobView{}, tc.svcErr
					},
				}
				resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
				server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, service, resolver)

				reqBody := `{"request_id":"` + requestID.String() + `","provider_id":"fake-provider","model_id":"fake-model"}`
				req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/script-generations", bytes.NewBufferString(reqBody))
				rec := httptest.NewRecorder()
				server.Handler.ServeHTTP(rec, req)

				if rec.Code != tc.expectedCode {
					t.Fatalf("expected code %d, got %d: %s", tc.expectedCode, rec.Code, rec.Body.String())
				}

				var errResp errorEnvelope
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("unmarshal error: %v", err)
				}
				if errResp.Error.Code != tc.expectedErr {
					t.Fatalf("expected error code %q, got %q", tc.expectedErr, errResp.Error.Code)
				}
			})
		}
	})
}

func TestGetScriptGenerationEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	jobID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	t.Run("succeeded job returns 200 with script_version", func(t *testing.T) {
		version := 4
		service := fakeScriptGenService{
			getGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, jID uuid.UUID) (scriptgenerationjob.ScriptGenerationJobView, error) {
				now := time.Now().UTC()
				return scriptgenerationjob.ScriptGenerationJobView{
					ID:            jobID,
					State:         "succeeded",
					Attempt:       1,
					MaxAttempts:   3,
					ScriptVersion: &version,
					CreatedAt:     now,
					UpdatedAt:     now,
				}, nil
			},
		}

		resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/script-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp["state"] != "succeeded" || resp["script_version"] != float64(4) {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		service := fakeScriptGenService{
			getGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, jID uuid.UUID) (scriptgenerationjob.ScriptGenerationJobView, error) {
				return scriptgenerationjob.ScriptGenerationJobView{}, scriptgenerationjob.ErrJobNotFound
			},
		}

		resolver := actor.NewLocalResolver(config.Config{Environment: "test", LocalActorID: &ownerID})
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, nil, nil, service, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/script-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

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

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
)

type fakeProposalGenService struct {
	getOptionsFn func(ctx context.Context) (proposalgenerationjob.TextGenerationOptionsResponse, error)
	createGenFn  func(ctx context.Context, principal project.Principal, projectID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error)
	getGenFn     func(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (proposalgenerationjob.ProposalGenerationJobView, error)
}

func (f fakeProposalGenService) GetTextGenerationOptions(ctx context.Context) (proposalgenerationjob.TextGenerationOptionsResponse, error) {
	if f.getOptionsFn != nil {
		return f.getOptionsFn(ctx)
	}
	return proposalgenerationjob.TextGenerationOptionsResponse{}, nil
}

func (f fakeProposalGenService) GetTextGenerationOptionsForOwner(ctx context.Context, ownerID uuid.UUID) (proposalgenerationjob.TextGenerationOptionsResponse, error) {
	if f.getOptionsFn != nil {
		return f.getOptionsFn(ctx)
	}
	return proposalgenerationjob.TextGenerationOptionsResponse{}, nil
}

func (f fakeProposalGenService) CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
	if f.createGenFn != nil {
		return f.createGenFn(ctx, principal, projectID, input)
	}
	return proposalgenerationjob.ProposalGenerationJobView{}, nil
}

func (f fakeProposalGenService) GetGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (proposalgenerationjob.ProposalGenerationJobView, error) {
	if f.getGenFn != nil {
		return f.getGenFn(ctx, principal, projectID, jobID)
	}
	return proposalgenerationjob.ProposalGenerationJobView{}, nil
}

func TestGetTextGenerationOptionsEndpoint(t *testing.T) {
	service := fakeProposalGenService{
		getOptionsFn: func(ctx context.Context) (proposalgenerationjob.TextGenerationOptionsResponse, error) {
			return proposalgenerationjob.TextGenerationOptionsResponse{
				Providers: []proposalgenerationjob.TextGenerationOptionProvider{
					{
						ID:          "lab-provider",
						DisplayName: "Lab Provider",
						Models: []proposalgenerationjob.TextGenerationOptionModel{
							{ID: "lab-model-v1", DisplayName: "Lab Model V1"},
						},
					},
				},
			}, nil
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/text-generation-options", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp proposalgenerationjob.TextGenerationOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Providers) != 1 || resp.Providers[0].ID != "lab-provider" {
		t.Fatalf("unexpected providers response: %+v", resp)
	}
}

func TestCreateProposalGenerationEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	requestID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	t.Run("202 Accepted on success", func(t *testing.T) {
		service := fakeProposalGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
				if principal.OwnerID != ownerID || pID != projectID || input.RequestID != requestID {
					t.Fatalf("unexpected params: principal=%v, pID=%v, input=%+v", principal, pID, input)
				}
				return proposalgenerationjob.ProposalGenerationJobView{
					ID:          requestID,
					State:       "queued",
					Attempt:     0,
					MaxAttempts: 3,
					CreatedAt:   time.Now().UTC(),
					UpdatedAt:   time.Now().UTC(),
				}, nil
			},
		}

		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		bodyBytes, _ := json.Marshal(map[string]any{
			"request_id":  requestID.String(),
			"provider_id": "lab-provider",
			"model_id":    "lab-model-v1",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
		}

		var jobResp proposalGenerationJobResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &jobResp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if jobResp.ID != requestID.String() || jobResp.State != "queued" {
			t.Fatalf("unexpected response: %+v", jobResp)
		}
	})

	t.Run("409 CREATIVE_BRIEF_REQUIRED", func(t *testing.T) {
		service := fakeProposalGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
				return proposalgenerationjob.ProposalGenerationJobView{}, proposalgenerationjob.ErrCreativeBriefRequired
			},
		}
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		bodyBytes, _ := json.Marshal(map[string]any{
			"request_id":  requestID.String(),
			"provider_id": "lab-provider",
			"model_id":    "lab-model-v1",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "CREATIVE_BRIEF_REQUIRED" {
			t.Fatalf("expected CREATIVE_BRIEF_REQUIRED, got %q", errEnv.Error.Code)
		}
	})

	t.Run("400 GENERATION_PROVIDER_UNAVAILABLE", func(t *testing.T) {
		service := fakeProposalGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
				return proposalgenerationjob.ProposalGenerationJobView{}, proposalgenerationjob.ErrProviderUnavailable
			},
		}
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		bodyBytes, _ := json.Marshal(map[string]any{
			"request_id":  requestID.String(),
			"provider_id": "bad-provider",
			"model_id":    "bad-model",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "GENERATION_PROVIDER_UNAVAILABLE" {
			t.Fatalf("expected GENERATION_PROVIDER_UNAVAILABLE, got %q", errEnv.Error.Code)
		}
	})

	t.Run("409 GENERATION_REQUEST_CONFLICT", func(t *testing.T) {
		service := fakeProposalGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
				return proposalgenerationjob.ProposalGenerationJobView{}, proposalgenerationjob.ErrGenerationRequestConflict
			},
		}
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		bodyBytes, _ := json.Marshal(map[string]any{
			"request_id":  requestID.String(),
			"provider_id": "lab-provider",
			"model_id":    "other-model",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "GENERATION_REQUEST_CONFLICT" {
			t.Fatalf("expected GENERATION_REQUEST_CONFLICT, got %q", errEnv.Error.Code)
		}
	})

	t.Run("404 Project Not Found", func(t *testing.T) {
		service := fakeProposalGenService{
			createGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error) {
				return proposalgenerationjob.ProposalGenerationJobView{}, proposalgenerationjob.ErrProjectNotFound
			},
		}
		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		bodyBytes, _ := json.Marshal(map[string]any{
			"request_id":  requestID.String(),
			"provider_id": "lab-provider",
			"model_id":    "lab-model-v1",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestGetProposalGenerationEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	jobID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	version := 2

	t.Run("200 OK with succeeded proposal version", func(t *testing.T) {
		service := fakeProposalGenService{
			getGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, jID uuid.UUID) (proposalgenerationjob.ProposalGenerationJobView, error) {
				if principal.OwnerID != ownerID || pID != projectID || jID != jobID {
					t.Fatalf("unexpected params: %v, %v, %v", principal, pID, jID)
				}
				return proposalgenerationjob.ProposalGenerationJobView{
					ID:              jobID,
					State:           "succeeded",
					Attempt:         1,
					MaxAttempts:     3,
					ProposalVersion: &version,
					CreatedAt:       time.Now().UTC(),
					UpdatedAt:       time.Now().UTC(),
				}, nil
			},
		}

		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var jobResp proposalGenerationJobResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &jobResp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if jobResp.ID != jobID.String() || jobResp.State != "succeeded" || jobResp.ProposalVersion == nil || *jobResp.ProposalVersion != 2 {
			t.Fatalf("unexpected job response: %+v", jobResp)
		}
	})

	t.Run("404 on unknown job", func(t *testing.T) {
		service := fakeProposalGenService{
			getGenFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, jID uuid.UUID) (proposalgenerationjob.ProposalGenerationJobView, error) {
				return proposalgenerationjob.ProposalGenerationJobView{}, proposalgenerationjob.ErrJobNotFound
			},
		}

		server := New(config.Config{Environment: "test"}, nil, nil, nil, nil, nil, service, nil, fixedResolver{ownerID: ownerID})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposal-generations/"+jobID.String(), nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

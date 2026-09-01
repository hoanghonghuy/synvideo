package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
)

type ProposalGenerationService interface {
	GetTextGenerationOptions(ctx context.Context) (proposalgenerationjob.TextGenerationOptionsResponse, error)
	GetTextGenerationOptionsForOwner(ctx context.Context, ownerID uuid.UUID) (proposalgenerationjob.TextGenerationOptionsResponse, error)
	CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, input proposalgenerationjob.CreateProposalGenerationInput) (proposalgenerationjob.ProposalGenerationJobView, error)
	GetGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (proposalgenerationjob.ProposalGenerationJobView, error)
}

type creativeProposalGenerationHandler struct {
	service       ProposalGenerationService
	actorResolver actor.Resolver
}

type createProposalGenerationRequest struct {
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

type proposalGenerationJobResponse struct {
	ID              string  `json:"id"`
	State           string  `json:"state"`
	Attempt         int     `json:"attempt"`
	MaxAttempts     int     `json:"max_attempts"`
	ErrorCode       *string `json:"error_code"`
	ProposalVersion *int    `json:"proposal_version"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	StartedAt       *string `json:"started_at"`
	FinishedAt      *string `json:"finished_at"`
}

func (h creativeProposalGenerationHandler) getTextGenerationOptions(w http.ResponseWriter, r *http.Request) {
	if h.actorResolver != nil {
		if principal, err := h.actorResolver.Resolve(r); err == nil && principal.OwnerID != uuid.Nil {
			resp, err := h.service.GetTextGenerationOptionsForOwner(r.Context(), principal.OwnerID)
			if err != nil {
				writeProposalGenerationAPIError(w, err)
				return
			}
			writeProjectJSON(w, http.StatusOK, resp)
			return
		}
	}

	resp, err := h.service.GetTextGenerationOptions(r.Context())
	if err != nil {
		writeProposalGenerationAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, resp)
}

func (h creativeProposalGenerationHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	var req createProposalGenerationRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return
	}

	parsedRequestID, err := uuid.Parse(req.RequestID)
	if err != nil || parsedRequestID == uuid.Nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"request_id": "invalid_uuid"}})
		return
	}
	if req.ProviderID == "" {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"provider_id": "required"}})
		return
	}
	if req.ModelID == "" {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"model_id": "required"}})
		return
	}

	jobView, err := h.service.CreateGeneration(r.Context(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
		RequestID:  parsedRequestID,
		ProviderID: req.ProviderID,
		ModelID:    req.ModelID,
	})
	if err != nil {
		writeProposalGenerationAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusAccepted, toProposalGenerationJobResponse(jobView))
}

func (h creativeProposalGenerationHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	jobIDStr := r.PathValue("job_id")
	parsedJobID, err := uuid.Parse(jobIDStr)
	if err != nil || parsedJobID == uuid.Nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"job_id": "invalid_uuid"}})
		return
	}

	jobView, err := h.service.GetGeneration(r.Context(), principal, projectID, parsedJobID)
	if err != nil {
		writeProposalGenerationAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toProposalGenerationJobResponse(jobView))
}

func (h creativeProposalGenerationHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func writeProposalGenerationAPIError(w http.ResponseWriter, err error) {
	var validationErr project.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  validationErr.Fields,
		}})
	case errors.Is(err, proposalgenerationjob.ErrProjectNotFound), errors.Is(err, proposalgenerationjob.ErrJobNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "not_found",
			Message: "Resource was not found.",
		}})
	case errors.Is(err, proposalgenerationjob.ErrCreativeBriefRequired):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "CREATIVE_BRIEF_REQUIRED",
			Message: "Creative brief is required before generating proposals.",
		}})
	case errors.Is(err, proposalgenerationjob.ErrProviderUnavailable):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "GENERATION_PROVIDER_UNAVAILABLE",
			Message: "Selected text generation provider or model is not available.",
		}})
	case errors.Is(err, proposalgenerationjob.ErrGenerationRequestConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "GENERATION_REQUEST_CONFLICT",
			Message: "Generation request ID has already been used with different parameters.",
		}})
	case errors.Is(err, proposalgenerationjob.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{
			Code:    "principal_required",
			Message: "A request principal is required.",
		}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
			Code:    "internal_error",
			Message: "The request could not be completed.",
		}})
	}
}

func toProposalGenerationJobResponse(item proposalgenerationjob.ProposalGenerationJobView) proposalGenerationJobResponse {
	var startedAt, finishedAt *string
	if item.StartedAt != nil {
		tStr := item.StartedAt.UTC().Format(time.RFC3339Nano)
		startedAt = &tStr
	}
	if item.FinishedAt != nil {
		tStr := item.FinishedAt.UTC().Format(time.RFC3339Nano)
		finishedAt = &tStr
	}

	return proposalGenerationJobResponse{
		ID:              item.ID.String(),
		State:           item.State,
		Attempt:         item.Attempt,
		MaxAttempts:     item.MaxAttempts,
		ErrorCode:       item.ErrorCode,
		ProposalVersion: item.ProposalVersion,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
	}
}

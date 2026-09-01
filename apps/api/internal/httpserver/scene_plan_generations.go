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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangenerationjob"
)

type ScenePlanGenerationService interface {
	CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, input sceneplangenerationjob.CreateScenePlanGenerationInput) (sceneplangenerationjob.ScenePlanGenerationJobView, error)
	GetGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, jobID uuid.UUID) (sceneplangenerationjob.ScenePlanGenerationJobView, error)
}

type scenePlanGenerationHandler struct {
	service       ScenePlanGenerationService
	actorResolver actor.Resolver
}

type createScenePlanGenerationRequest struct {
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

type scenePlanGenerationJobResponse struct {
	ID               string  `json:"id"`
	State            string  `json:"state"`
	Attempt          int     `json:"attempt"`
	MaxAttempts      int     `json:"max_attempts"`
	ErrorCode        *string `json:"error_code"`
	ScenePlanVersion *int    `json:"scene_plan_version"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func (h scenePlanGenerationHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	var req createScenePlanGenerationRequest
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

	jobView, err := h.service.CreateGeneration(r.Context(), principal, projectID, sceneplangenerationjob.CreateScenePlanGenerationInput{
		RequestID:  parsedRequestID,
		ProviderID: req.ProviderID,
		ModelID:    req.ModelID,
	})
	if err != nil {
		writeScenePlanGenerationAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusAccepted, toScenePlanGenerationJobResponse(jobView))
}

func (h scenePlanGenerationHandler) get(w http.ResponseWriter, r *http.Request) {
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
		writeScenePlanGenerationAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScenePlanGenerationJobResponse(jobView))
}

func (h scenePlanGenerationHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	if h.actorResolver == nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func writeScenePlanGenerationAPIError(w http.ResponseWriter, err error) {
	var validationErr project.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  validationErr.Fields,
		}})
	case errors.Is(err, sceneplangenerationjob.ErrProjectNotFound), errors.Is(err, sceneplangenerationjob.ErrJobNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "not_found",
			Message: "Resource was not found.",
		}})
	case errors.Is(err, sceneplangenerationjob.ErrScriptApprovalRequired):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCRIPT_APPROVAL_REQUIRED",
			Message: "Approved script is required before generating scene plan.",
		}})
	case errors.Is(err, sceneplangenerationjob.ErrScenePlanSourceInvalid):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCENE_PLAN_SOURCE_INVALID",
			Message: "Source script or proposal is invalid.",
		}})
	case errors.Is(err, sceneplangenerationjob.ErrProviderUnavailable):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "GENERATION_PROVIDER_UNAVAILABLE",
			Message: "Selected text generation provider or model is not available.",
		}})
	case errors.Is(err, sceneplangenerationjob.ErrGenerationRequestConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "GENERATION_REQUEST_CONFLICT",
			Message: "Generation request ID has already been used with different parameters.",
		}})
	case errors.Is(err, sceneplangenerationjob.ErrUnauthenticated):
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

func toScenePlanGenerationJobResponse(item sceneplangenerationjob.ScenePlanGenerationJobView) scenePlanGenerationJobResponse {
	return scenePlanGenerationJobResponse{
		ID:               item.ID.String(),
		State:            item.State,
		Attempt:          item.Attempt,
		MaxAttempts:      item.MaxAttempts,
		ErrorCode:        item.ErrorCode,
		ScenePlanVersion: item.ScenePlanVersion,
		CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

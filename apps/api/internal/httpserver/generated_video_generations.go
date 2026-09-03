package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenevideojob"
)

type GeneratedVideoGenerationService interface {
	CreateGeneration(context.Context, project.Principal, uuid.UUID, int, string, scenevideojob.CreateGenerationInput) (scenevideojob.JobView, error)
	GetGeneration(context.Context, project.Principal, uuid.UUID, uuid.UUID) (scenevideojob.JobView, error)
}

type generatedVideoGenerationHandler struct {
	service       GeneratedVideoGenerationService
	actorResolver actor.Resolver
}

type createGeneratedVideoRequest struct {
	RequestID           string `json:"request_id"`
	ProviderID          string `json:"provider_id"`
	ModelID             string `json:"model_id"`
	DurationSeconds     *int   `json:"duration_seconds,omitempty"`
	AssignPrimaryVisual bool   `json:"assign_primary_visual"`
}

type generatedVideoJobResponse struct {
	ID                    string  `json:"id"`
	State                 string  `json:"state"`
	Attempt               int     `json:"attempt"`
	MaxAttempts           int     `json:"max_attempts"`
	ErrorCode             *string `json:"error_code"`
	MediaAssetID          *string `json:"media_asset_id,omitempty"`
	ExternalOperationID   *string `json:"external_operation_id,omitempty"`
	AssignedPrimaryVisual bool    `json:"assigned_primary_visual"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

func (h generatedVideoGenerationHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"version": "positive_integer"}})
		return
	}
	sceneKey := r.PathValue("scene_key")
	if sceneKey == "" {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"scene_key": "required"}})
		return
	}

	var req createGeneratedVideoRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil || requestID == uuid.Nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"request_id": "invalid_uuid"}})
		return
	}
	fields := map[string]string{}
	if req.ProviderID == "" {
		fields["provider_id"] = "required"
	}
	if req.ModelID == "" {
		fields["model_id"] = "required"
	}
	if req.DurationSeconds != nil && (*req.DurationSeconds < 1 || *req.DurationSeconds > 3600) {
		fields["duration_seconds"] = "out_of_range"
	}
	if len(fields) > 0 {
		writeAPIError(w, project.ValidationError{Fields: fields})
		return
	}

	view, err := h.service.CreateGeneration(r.Context(), principal, projectID, version, sceneKey, scenevideojob.CreateGenerationInput{
		RequestID:           requestID,
		ProviderID:          req.ProviderID,
		ModelID:             req.ModelID,
		DurationSeconds:     req.DurationSeconds,
		AssignPrimaryVisual: req.AssignPrimaryVisual,
	})
	if err != nil {
		writeGeneratedVideoAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusAccepted, toGeneratedVideoJobResponse(view))
}

func (h generatedVideoGenerationHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(r.PathValue("job_id"))
	if err != nil || jobID == uuid.Nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"job_id": "invalid_uuid"}})
		return
	}
	view, err := h.service.GetGeneration(r.Context(), principal, projectID, jobID)
	if err != nil {
		writeGeneratedVideoAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toGeneratedVideoJobResponse(view))
}

func (h generatedVideoGenerationHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
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

func toGeneratedVideoJobResponse(view scenevideojob.JobView) generatedVideoJobResponse {
	response := generatedVideoJobResponse{
		ID:                    view.ID.String(),
		State:                 view.State,
		Attempt:               view.Attempt,
		MaxAttempts:           view.MaxAttempts,
		ErrorCode:             view.ErrorCode,
		ExternalOperationID:   view.ExternalOperationID,
		AssignedPrimaryVisual: view.AssignedPrimaryVisual,
		CreatedAt:             view.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             view.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if view.MediaAssetID != nil {
		value := view.MediaAssetID.String()
		response.MediaAssetID = &value
	}
	return response
}

func writeGeneratedVideoAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scenevideojob.ErrProjectNotFound), errors.Is(err, scenevideojob.ErrScenePlanNotFound), errors.Is(err, scenevideojob.ErrSceneKeyNotFound), errors.Is(err, scenevideojob.ErrJobNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "not_found", Message: "Resource was not found."}})
	case errors.Is(err, scenevideojob.ErrScenePlanNotApproved):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_PLAN_APPROVAL_REQUIRED", Message: "Approved scene plan is required before generating scene media."}})
	case errors.Is(err, scenevideojob.ErrProviderUnavailable):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "GENERATION_PROVIDER_UNAVAILABLE", Message: "Selected video generation provider or model is not available."}})
	case errors.Is(err, scenevideojob.ErrGenerationRequestConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "GENERATION_REQUEST_CONFLICT", Message: "Generation request ID has already been used with different parameters."}})
	case errors.Is(err, scenevideojob.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

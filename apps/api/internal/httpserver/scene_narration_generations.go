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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type SceneNarrationGenerationService interface {
	CreateGeneration(context.Context, project.Principal, uuid.UUID, int, string, scenenarrationjob.CreateGenerationInput) (scenenarrationjob.JobView, error)
	GetGeneration(context.Context, project.Principal, uuid.UUID, uuid.UUID) (scenenarrationjob.JobView, error)
}

type sceneNarrationGenerationHandler struct {
	service       SceneNarrationGenerationService
	actorResolver actor.Resolver
}

type createSceneNarrationRequest struct {
	RequestID     string `json:"request_id"`
	ProviderID    string `json:"provider_id"`
	ModelID       string `json:"model_id"`
	VoiceID       string `json:"voice_id"`
	Format        string `json:"format,omitempty"`
	AssignCurrent bool   `json:"assign_current"`
}

type sceneNarrationJobResponse struct {
	ID                string  `json:"id"`
	State             string  `json:"state"`
	Attempt           int     `json:"attempt"`
	MaxAttempts       int     `json:"max_attempts"`
	ErrorCode         *string `json:"error_code"`
	MediaAssetID      *string `json:"media_asset_id,omitempty"`
	DurationSeconds   float64 `json:"duration_seconds"`
	AssignedNarration bool    `json:"assigned_narration"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

func (h sceneNarrationGenerationHandler) create(w http.ResponseWriter, r *http.Request) {
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

	var req createSceneNarrationRequest
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

	if req.ProviderID == "" || req.ModelID == "" || req.VoiceID == "" {
		fields := map[string]string{}
		if req.ProviderID == "" {
			fields["provider_id"] = "required"
		}
		if req.ModelID == "" {
			fields["model_id"] = "required"
		}
		if req.VoiceID == "" {
			fields["voice_id"] = "required"
		}
		writeAPIError(w, project.ValidationError{Fields: fields})
		return
	}

	view, err := h.service.CreateGeneration(r.Context(), principal, projectID, version, sceneKey, scenenarrationjob.CreateGenerationInput{
		RequestID:     requestID,
		ProviderID:    req.ProviderID,
		ModelID:       req.ModelID,
		VoiceID:       req.VoiceID,
		Format:        req.Format,
		AssignCurrent: req.AssignCurrent,
	})
	if err != nil {
		writeSceneNarrationJobAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusAccepted, toSceneNarrationJobResponse(view))
}

func (h sceneNarrationGenerationHandler) get(w http.ResponseWriter, r *http.Request) {
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
		writeSceneNarrationJobAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toSceneNarrationJobResponse(view))
}

func (h sceneNarrationGenerationHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
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

func toSceneNarrationJobResponse(view scenenarrationjob.JobView) sceneNarrationJobResponse {
	resp := sceneNarrationJobResponse{
		ID:                view.ID.String(),
		State:             view.State,
		Attempt:           view.Attempt,
		MaxAttempts:       view.MaxAttempts,
		ErrorCode:         view.ErrorCode,
		DurationSeconds:   view.DurationSeconds,
		AssignedNarration: view.AssignedNarration,
		CreatedAt:         view.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:         view.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if view.MediaAssetID != nil {
		val := view.MediaAssetID.String()
		resp.MediaAssetID = &val
	}
	return resp
}

func writeSceneNarrationJobAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scenenarrationjob.ErrProjectNotFound),
		errors.Is(err, scenenarrationjob.ErrScenePlanNotFound),
		errors.Is(err, scenenarrationjob.ErrSceneKeyNotFound),
		errors.Is(err, scenenarrationjob.ErrJobNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "not_found", Message: "Resource was not found."}})
	case errors.Is(err, scenenarrationjob.ErrScenePlanNotApproved):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_PLAN_APPROVAL_REQUIRED", Message: "Approved scene plan is required before generating scene narration."}})
	case errors.Is(err, scenenarrationjob.ErrEmptyNarration):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "EMPTY_SCENE_NARRATION", Message: "Scene has no narration text to synthesize."}})
	case errors.Is(err, scenenarrationjob.ErrProviderUnavailable):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "GENERATION_PROVIDER_UNAVAILABLE", Message: "Selected text-to-speech provider or voice is not available."}})
	case errors.Is(err, scenenarrationjob.ErrGenerationRequestConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "GENERATION_REQUEST_CONFLICT", Message: "Generation request ID has already been used with different parameters."}})
	case errors.Is(err, scenenarrationjob.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

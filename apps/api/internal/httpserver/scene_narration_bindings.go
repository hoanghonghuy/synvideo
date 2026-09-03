package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
)

type SceneNarrationService interface {
	ListCurrent(context.Context, project.Principal, uuid.UUID, int) ([]scenenarration.CurrentSceneNarration, error)
	AssignNarration(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenenarration.Binding, error)
	ListHistory(context.Context, project.Principal, uuid.UUID, int, string) ([]scenenarration.Binding, error)
}

type sceneNarrationHandler struct {
	bindings      SceneNarrationService
	assets        MediaAssetService
	actorResolver actor.Resolver
}

type sceneNarrationBindingResponse struct {
	SceneKey string                   `json:"scene_key"`
	Role     scenenarration.Role      `json:"role"`
	Binding  *scenenarration.Binding  `json:"binding,omitempty"`
	Asset    *mediaasset.MediaAsset   `json:"asset,omitempty"`
}

type assignNarrationRequest struct {
	AssetID string `json:"asset_id"`
}

func (h sceneNarrationHandler) listCurrent(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, false)
	if !ok {
		return
	}
	items, err := h.bindings.ListCurrent(r.Context(), principal, projectID, version)
	if err != nil {
		writeSceneNarrationAPIError(w, err)
		return
	}
	response, err := h.currentResponses(r.Context(), principal, projectID, items)
	if err != nil {
		writeSceneNarrationAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneNarrationHandler) assign(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, true)
	if !ok {
		return
	}
	var request assignNarrationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	assetID, err := uuid.Parse(request.AssetID)
	if err != nil || assetID == uuid.Nil {
		writeSceneNarrationAPIError(w, scenenarration.ErrInvalidInput)
		return
	}
	binding, err := h.bindings.AssignNarration(r.Context(), principal, projectID, version, r.PathValue("scene_key"), assetID)
	if err != nil {
		writeSceneNarrationAPIError(w, err)
		return
	}
	response, err := h.bindingResponse(r.Context(), principal, projectID, binding)
	if err != nil {
		writeSceneNarrationAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneNarrationHandler) history(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, true)
	if !ok {
		return
	}
	items, err := h.bindings.ListHistory(r.Context(), principal, projectID, version, r.PathValue("scene_key"))
	if err != nil {
		writeSceneNarrationAPIError(w, err)
		return
	}
	response := make([]sceneNarrationBindingResponse, 0, len(items))
	for _, item := range items {
		converted, err := h.bindingResponse(r.Context(), principal, projectID, item)
		if err != nil {
			writeSceneNarrationAPIError(w, err)
			return
		}
		response = append(response, converted)
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneNarrationHandler) currentResponses(ctx context.Context, principal project.Principal, projectID uuid.UUID, items []scenenarration.CurrentSceneNarration) ([]sceneNarrationBindingResponse, error) {
	response := make([]sceneNarrationBindingResponse, 0, len(items))
	for _, item := range items {
		converted := sceneNarrationBindingResponse{
			SceneKey: item.Scene.Key,
			Role:     scenenarration.RoleNarration,
		}
		if item.Binding != nil {
			bindingResponse, err := h.bindingResponse(ctx, principal, projectID, *item.Binding)
			if err != nil {
				return nil, err
			}
			converted = bindingResponse
		}
		response = append(response, converted)
	}
	return response, nil
}

func (h sceneNarrationHandler) bindingResponse(ctx context.Context, principal project.Principal, projectID uuid.UUID, binding scenenarration.Binding) (sceneNarrationBindingResponse, error) {
	asset, err := h.assets.Get(ctx, principal, projectID, binding.AssetID)
	if err != nil {
		return sceneNarrationBindingResponse{}, err
	}
	return sceneNarrationBindingResponse{
		SceneKey: binding.SceneKey,
		Role:     binding.Role,
		Binding:  &binding,
		Asset:    &asset,
	}, nil
}

func (h sceneNarrationHandler) identifiers(w http.ResponseWriter, r *http.Request, withScene bool) (project.Principal, uuid.UUID, int, bool) {
	if h.actorResolver == nil {
		writeSceneNarrationAPIError(w, scenenarration.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, 0, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeSceneNarrationAPIError(w, scenenarration.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, 0, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, 0, false
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		writeSceneNarrationAPIError(w, scenenarration.ErrScenePlanNotFound)
		return project.Principal{}, uuid.Nil, 0, false
	}
	if withScene && r.PathValue("scene_key") == "" {
		writeSceneNarrationAPIError(w, scenenarration.ErrInvalidInput)
		return project.Principal{}, uuid.Nil, 0, false
	}
	return principal, projectID, version, true
}

func writeSceneNarrationAPIError(w http.ResponseWriter, err error) {
	var validation mediaasset.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Request validation failed.", Fields: validation.Fields}})
	case errors.Is(err, scenenarration.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "validation_failed", Message: "Scene narration binding request is invalid."}})
	case errors.Is(err, scenenarration.ErrScenePlanNotFound),
		errors.Is(err, scenenarration.ErrSceneKeyNotFound),
		errors.Is(err, scenenarration.ErrMediaAssetNotFound),
		errors.Is(err, scenenarration.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "SCENE_NARRATION_NOT_FOUND", Message: "Scene narration resource was not found."}})
	case errors.Is(err, mediaasset.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "SCENE_NARRATION_NOT_FOUND", Message: "Scene narration resource was not found."}})
	case errors.Is(err, scenenarration.ErrScenePlanNotApproved):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_PLAN_NOT_APPROVED", Message: "Scene plan must be approved before narration can be assigned."}})
	case errors.Is(err, scenenarration.ErrMediaAssetNotAudio):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Only audio assets can be scene narrations."}})
	case errors.Is(err, scenenarration.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, scenenarration.ErrPersistenceFailed):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "SCENE_NARRATION_PERSISTENCE_FAILED", Message: "Scene narration binding could not be persisted."}})
	case errors.Is(err, mediaasset.ErrPersistenceFailed):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "SCENE_NARRATION_PERSISTENCE_FAILED", Message: "Scene narration binding could not be completed."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

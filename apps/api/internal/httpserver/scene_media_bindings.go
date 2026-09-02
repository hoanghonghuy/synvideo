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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

type SceneMediaService interface {
	ListCurrent(context.Context, project.Principal, uuid.UUID, int) ([]scenemedia.CurrentSceneBinding, error)
	AssignPrimaryVisual(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenemedia.Binding, error)
	ListHistory(context.Context, project.Principal, uuid.UUID, int, string) ([]scenemedia.Binding, error)
}

type sceneMediaHandler struct {
	bindings      SceneMediaService
	assets        MediaAssetService
	actorResolver actor.Resolver
}

type sceneMediaBindingResponse struct {
	SceneKey string                 `json:"scene_key"`
	Role     scenemedia.Role        `json:"role"`
	Binding  *scenemedia.Binding    `json:"binding,omitempty"`
	Asset    *mediaasset.MediaAsset `json:"asset,omitempty"`
}

type assignPrimaryVisualRequest struct {
	AssetID string `json:"asset_id"`
}

func (h sceneMediaHandler) listCurrent(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, false)
	if !ok {
		return
	}
	items, err := h.bindings.ListCurrent(r.Context(), principal, projectID, version)
	if err != nil {
		writeSceneMediaAPIError(w, err)
		return
	}
	response, err := h.currentResponses(r.Context(), principal, projectID, items)
	if err != nil {
		writeSceneMediaAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneMediaHandler) assignPrimaryVisual(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, true)
	if !ok {
		return
	}
	var request assignPrimaryVisualRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	assetID, err := uuid.Parse(request.AssetID)
	if err != nil || assetID == uuid.Nil {
		writeSceneMediaAPIError(w, scenemedia.ErrInvalidInput)
		return
	}
	binding, err := h.bindings.AssignPrimaryVisual(r.Context(), principal, projectID, version, r.PathValue("scene_key"), assetID)
	if err != nil {
		writeSceneMediaAPIError(w, err)
		return
	}
	response, err := h.bindingResponse(r.Context(), principal, projectID, binding)
	if err != nil {
		writeSceneMediaAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneMediaHandler) history(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, ok := h.identifiers(w, r, true)
	if !ok {
		return
	}
	items, err := h.bindings.ListHistory(r.Context(), principal, projectID, version, r.PathValue("scene_key"))
	if err != nil {
		writeSceneMediaAPIError(w, err)
		return
	}
	response := make([]sceneMediaBindingResponse, 0, len(items))
	for _, item := range items {
		converted, err := h.bindingResponse(r.Context(), principal, projectID, item)
		if err != nil {
			writeSceneMediaAPIError(w, err)
			return
		}
		response = append(response, converted)
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h sceneMediaHandler) currentResponses(ctx context.Context, principal project.Principal, projectID uuid.UUID, items []scenemedia.CurrentSceneBinding) ([]sceneMediaBindingResponse, error) {
	response := make([]sceneMediaBindingResponse, 0, len(items))
	for _, item := range items {
		converted := sceneMediaBindingResponse{SceneKey: item.Scene.Key, Role: scenemedia.RolePrimaryVisual}
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

func (h sceneMediaHandler) bindingResponse(ctx context.Context, principal project.Principal, projectID uuid.UUID, binding scenemedia.Binding) (sceneMediaBindingResponse, error) {
	asset, err := h.assets.Get(ctx, principal, projectID, binding.AssetID)
	if err != nil {
		return sceneMediaBindingResponse{}, err
	}
	return sceneMediaBindingResponse{SceneKey: binding.SceneKey, Role: binding.Role, Binding: &binding, Asset: &asset}, nil
}

func (h sceneMediaHandler) identifiers(w http.ResponseWriter, r *http.Request, withScene bool) (project.Principal, uuid.UUID, int, bool) {
	if h.actorResolver == nil {
		writeSceneMediaAPIError(w, scenemedia.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, 0, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeSceneMediaAPIError(w, scenemedia.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, 0, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, 0, false
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		writeSceneMediaAPIError(w, scenemedia.ErrScenePlanNotFound)
		return project.Principal{}, uuid.Nil, 0, false
	}
	if withScene && r.PathValue("scene_key") == "" {
		writeSceneMediaAPIError(w, scenemedia.ErrInvalidInput)
		return project.Principal{}, uuid.Nil, 0, false
	}
	return principal, projectID, version, true
}

func writeSceneMediaAPIError(w http.ResponseWriter, err error) {
	var validation mediaasset.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Request validation failed.", Fields: validation.Fields}})
	case errors.Is(err, scenemedia.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "validation_failed", Message: "Scene media binding request is invalid."}})
	case errors.Is(err, scenemedia.ErrScenePlanNotFound), errors.Is(err, scenemedia.ErrSceneKeyNotFound), errors.Is(err, scenemedia.ErrMediaAssetNotFound), errors.Is(err, scenemedia.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "SCENE_MEDIA_NOT_FOUND", Message: "Scene media resource was not found."}})
	case errors.Is(err, mediaasset.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "SCENE_MEDIA_NOT_FOUND", Message: "Scene media resource was not found."}})
	case errors.Is(err, scenemedia.ErrScenePlanNotApproved):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_PLAN_NOT_APPROVED", Message: "Scene plan must be approved before media can be assigned."}})
	case errors.Is(err, scenemedia.ErrMediaAssetNotVisual):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Only image or video assets can be primary visuals."}})
	case errors.Is(err, scenemedia.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, scenemedia.ErrPersistenceFailed):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "SCENE_MEDIA_PERSISTENCE_FAILED", Message: "Scene media binding could not be persisted."}})
	case errors.Is(err, mediaasset.ErrPersistenceFailed):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "SCENE_MEDIA_PERSISTENCE_FAILED", Message: "Scene media binding could not be completed."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

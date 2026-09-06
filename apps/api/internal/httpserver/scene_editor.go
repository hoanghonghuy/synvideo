package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type SceneEditorService interface {
	Create(context.Context, uuid.UUID, uuid.UUID, int, []sceneeditor.Scene, *sceneeditor.AudioMixRef) (sceneeditor.View, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (sceneeditor.View, error)
	Update(context.Context, uuid.UUID, uuid.UUID, sceneeditor.UpdateInput) (sceneeditor.View, error)
	Reorder(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, int) (sceneeditor.View, error)
	Duplicate(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int) (sceneeditor.View, error)
	Remove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int) (sceneeditor.View, error)
	PreviewReconcile(context.Context, uuid.UUID, uuid.UUID, sceneeditor.ReconcileCandidate) (sceneeditor.ReconcilePreview, error)
	Reconcile(context.Context, uuid.UUID, uuid.UUID, sceneeditor.ReconcileInput) (sceneeditor.View, error)
	Snapshot(context.Context, uuid.UUID, uuid.UUID, int) (sceneeditor.Snapshot, error)
}

type sceneEditorHandler struct {
	service       SceneEditorService
	actorResolver actor.Resolver
}

type sceneEditorReorderRequest struct {
	ExpectedRevision int `json:"expected_revision"`
	To               int `json:"to"`
}

type sceneEditorRevisionRequest struct {
	ExpectedRevision int `json:"expected_revision"`
}

type sceneEditorReconcilePreviewRequest struct {
	Candidate sceneeditor.ReconcileCandidate `json:"candidate"`
}

func (h sceneEditorHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input sceneeditor.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Create(r.Context(), principal.OwnerID, projectID, input.ScenePlanVersion, input.Scenes, input.AudioMix)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusCreated, view)
}

func (h sceneEditorHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	view, err := h.service.Get(r.Context(), principal.OwnerID, projectID)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) update(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input sceneeditor.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Update(r.Context(), principal.OwnerID, projectID, input)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) reorder(w http.ResponseWriter, r *http.Request) {
	principal, projectID, sceneID, ok := h.sceneIdentifiers(w, r)
	if !ok {
		return
	}
	var input sceneEditorReorderRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Reorder(r.Context(), principal.OwnerID, projectID, sceneID, input.To, input.ExpectedRevision)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) duplicate(w http.ResponseWriter, r *http.Request) {
	principal, projectID, sceneID, ok := h.sceneIdentifiers(w, r)
	if !ok {
		return
	}
	var input sceneEditorRevisionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Duplicate(r.Context(), principal.OwnerID, projectID, sceneID, input.ExpectedRevision)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) remove(w http.ResponseWriter, r *http.Request) {
	principal, projectID, sceneID, ok := h.sceneIdentifiers(w, r)
	if !ok {
		return
	}
	var input sceneEditorRevisionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Remove(r.Context(), principal.OwnerID, projectID, sceneID, input.ExpectedRevision)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) previewReconcile(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input sceneEditorReconcilePreviewRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	preview, err := h.service.PreviewReconcile(r.Context(), principal.OwnerID, projectID, input.Candidate)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, preview)
}

func (h sceneEditorHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input sceneeditor.ReconcileInput
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Reconcile(r.Context(), principal.OwnerID, projectID, input)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h sceneEditorHandler) snapshot(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input sceneEditorRevisionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	snapshot, err := h.service.Snapshot(r.Context(), principal.OwnerID, projectID, input.ExpectedRevision)
	if err != nil {
		writeSceneEditorAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusCreated, snapshot)
}

func (h sceneEditorHandler) identifiers(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, bool) {
	if h.actorResolver == nil {
		writeSceneEditorAPIError(w, sceneeditor.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeSceneEditorAPIError(w, sceneeditor.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, false
	}
	return principal, projectID, true
}

func (h sceneEditorHandler) sceneIdentifiers(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, uuid.UUID, bool) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, uuid.Nil, false
	}
	sceneID, err := uuid.Parse(r.PathValue("scene_id"))
	if err != nil || sceneID == uuid.Nil {
		writeSceneEditorAPIError(w, sceneeditor.ErrInvalidInput)
		return project.Principal{}, uuid.Nil, uuid.Nil, false
	}
	return principal, projectID, sceneID, true
}

func writeSceneEditorAPIError(w http.ResponseWriter, err error) {
	var validation sceneeditor.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_VALIDATION_FAILED", Message: "Scene editor validation failed.", Fields: validation.Fields}})
	case errors.Is(err, sceneeditor.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_INVALID", Message: "Scene editor request is invalid."}})
	case errors.Is(err, sceneeditor.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, sceneeditor.ErrNotFound), errors.Is(err, sceneeditor.ErrSceneNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_NOT_FOUND", Message: "Scene editor composition was not found."}})
	case errors.Is(err, sceneeditor.ErrConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_REVISION_CONFLICT", Message: "Scene editor revision is stale."}})
	case errors.Is(err, sceneeditor.ErrLastScene):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_LAST_SCENE", Message: "A composition must contain at least one scene."}})
	case errors.Is(err, sceneeditor.ErrAmbiguousMapping):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_RECONCILE_AMBIGUOUS", Message: "Scene mapping requires creator choice."}})
	case errors.Is(err, sceneeditor.ErrSnapshotBlocked):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_SNAPSHOT_BLOCKED", Message: "Resolve stale or broken dependencies before snapshot creation."}})
	case errors.Is(err, sceneeditor.ErrPersistence):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "SCENE_EDITOR_PERSISTENCE_FAILED", Message: "Scene editor state could not be persisted."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

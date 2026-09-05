package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type CaptionService interface {
	Derive(context.Context, project.Principal, uuid.UUID, int, string) (captions.View, error)
	Get(context.Context, project.Principal, uuid.UUID, int, string) (captions.View, error)
	Update(context.Context, project.Principal, uuid.UUID, int, string, captions.UpdateInput) (captions.View, error)
	Rebuild(context.Context, project.Principal, uuid.UUID, int, string, int) (captions.View, error)
	Snapshot(context.Context, project.Principal, uuid.UUID, int, string) (captions.Snapshot, error)
	History(context.Context, project.Principal, uuid.UUID, int, string) ([]captions.Document, error)
}

type captionHandler struct {
	service CaptionService
	actorResolver actor.Resolver
}

type rebuildCaptionRequest struct { ExpectedRevision int `json:"expected_revision"` }

func (h captionHandler) derive(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	view, err := h.service.Derive(r.Context(), principal, projectID, version, sceneKey)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusCreated, view)
}

func (h captionHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	view, err := h.service.Get(r.Context(), principal, projectID, version, sceneKey)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusOK, view)
}

func (h captionHandler) update(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	var request captions.UpdateInput
	if !decodeJSON(w, r, &request) { return }
	view, err := h.service.Update(r.Context(), principal, projectID, version, sceneKey, request)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusOK, view)
}

func (h captionHandler) rebuild(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	var request rebuildCaptionRequest
	if !decodeJSON(w, r, &request) { return }
	view, err := h.service.Rebuild(r.Context(), principal, projectID, version, sceneKey, request.ExpectedRevision)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusOK, view)
}

func (h captionHandler) snapshot(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	snapshot, err := h.service.Snapshot(r.Context(), principal, projectID, version, sceneKey)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusOK, snapshot)
}

func (h captionHandler) history(w http.ResponseWriter, r *http.Request) {
	principal, projectID, version, sceneKey, ok := h.identifiers(w, r); if !ok { return }
	history, err := h.service.History(r.Context(), principal, projectID, version, sceneKey)
	if err != nil { writeCaptionAPIError(w, err); return }
	writeProjectJSON(w, http.StatusOK, history)
}

func (h captionHandler) identifiers(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, int, string, bool) {
	if h.actorResolver == nil { writeCaptionAPIError(w, captions.ErrUnauthenticated); return project.Principal{}, uuid.Nil, 0, "", false }
	principal, err := h.actorResolver.Resolve(r); if err != nil { writeCaptionAPIError(w, captions.ErrUnauthenticated); return project.Principal{}, uuid.Nil, 0, "", false }
	projectID, ok := parseProjectID(w, r); if !ok { return project.Principal{}, uuid.Nil, 0, "", false }
	version, err := strconv.Atoi(r.PathValue("version")); if err != nil || version < 1 { writeCaptionAPIError(w, captions.ErrInvalidInput); return project.Principal{}, uuid.Nil, 0, "", false }
	sceneKey := r.PathValue("scene_key"); if sceneKey == "" { writeCaptionAPIError(w, captions.ErrInvalidInput); return project.Principal{}, uuid.Nil, 0, "", false }
	return principal, projectID, version, sceneKey, true
}

func writeCaptionAPIError(w http.ResponseWriter, err error) {
	var validation captions.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "CAPTION_VALIDATION_FAILED", Message: "Caption validation failed.", Fields: validation.Fields}})
	case errors.Is(err, captions.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "CAPTION_INVALID", Message: "Caption request is invalid."}})
	case errors.Is(err, captions.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, captions.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "CAPTION_NOT_FOUND", Message: "Caption document was not found."}})
	case errors.Is(err, captions.ErrSourceMissing):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "CAPTION_SOURCE_MISSING", Message: "A current measured narration source is required."}})
	case errors.Is(err, captions.ErrConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "CAPTION_REVISION_CONFLICT", Message: "Caption revision is stale or already exists."}})
	case errors.Is(err, captions.ErrStale):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "CAPTION_STALE", Message: "Stale captions cannot be selected as a current snapshot."}})
	case errors.Is(err, captions.ErrPersistence):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "CAPTION_PERSISTENCE_FAILED", Message: "Caption document could not be persisted."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

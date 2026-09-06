package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type AudioMixService interface {
	Create(context.Context, project.Principal, uuid.UUID, audiomix.CreateInput) (audiomix.View, error)
	Get(context.Context, project.Principal, uuid.UUID) (audiomix.View, error)
	Update(context.Context, project.Principal, uuid.UUID, audiomix.UpdateInput) (audiomix.View, error)
	RebindNarration(context.Context, project.Principal, uuid.UUID, int) (audiomix.View, error)
	Snapshot(context.Context, project.Principal, uuid.UUID) (audiomix.Snapshot, error)
	History(context.Context, project.Principal, uuid.UUID) ([]audiomix.Document, error)
}

type audioMixHandler struct {
	service       AudioMixService
	actorResolver actor.Resolver
}

type rebindAudioMixRequest struct {
	ExpectedRevision int `json:"expected_revision"`
}

func (h audioMixHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input audiomix.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Create(r.Context(), principal, projectID, input)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusCreated, view)
}

func (h audioMixHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	view, err := h.service.Get(r.Context(), principal, projectID)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h audioMixHandler) update(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input audiomix.UpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.Update(r.Context(), principal, projectID, input)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h audioMixHandler) rebindNarration(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	var input rebindAudioMixRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	view, err := h.service.RebindNarration(r.Context(), principal, projectID, input.ExpectedRevision)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, view)
}

func (h audioMixHandler) snapshot(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	snapshot, err := h.service.Snapshot(r.Context(), principal, projectID)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, snapshot)
}

func (h audioMixHandler) history(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	history, err := h.service.History(r.Context(), principal, projectID)
	if err != nil {
		writeAudioMixAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, history)
}

func (h audioMixHandler) identifiers(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, bool) {
	if h.actorResolver == nil {
		writeAudioMixAPIError(w, audiomix.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAudioMixAPIError(w, audiomix.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, false
	}
	return principal, projectID, true
}

func writeAudioMixAPIError(w http.ResponseWriter, err error) {
	var validation audiomix.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_VALIDATION_FAILED", Message: "Audio mix validation failed.", Fields: validation.Fields}})
	case errors.Is(err, audiomix.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_INVALID", Message: "Audio mix request is invalid."}})
	case errors.Is(err, audiomix.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, audiomix.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_NOT_FOUND", Message: "Audio mix was not found."}})
	case errors.Is(err, audiomix.ErrMusicMissing):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_MUSIC_MISSING", Message: "A current same-project audio asset is required."}})
	case errors.Is(err, audiomix.ErrNarrationMissing):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_NARRATION_MISSING", Message: "Current narration for every approved scene is required."}})
	case errors.Is(err, audiomix.ErrConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_REVISION_CONFLICT", Message: "Audio mix revision is stale or already exists."}})
	case errors.Is(err, audiomix.ErrStale):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_STALE", Message: "A stale audio mix cannot be selected as a current snapshot."}})
	case errors.Is(err, audiomix.ErrBroken):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_BROKEN", Message: "A broken audio mix cannot be selected as a current snapshot."}})
	case errors.Is(err, audiomix.ErrPersistence):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "AUDIO_MIX_PERSISTENCE_FAILED", Message: "Audio mix could not be persisted."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

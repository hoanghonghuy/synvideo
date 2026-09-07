package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type RenderExportService interface {
	Enqueue(context.Context, uuid.UUID, uuid.UUID, string) (jobs.Job, error)
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (renderexport.JobView, error)
}

type renderExportHandler struct {
	service       RenderExportService
	actorResolver actor.Resolver
}

type createRenderExportRequest struct {
	SnapshotDigest string `json:"snapshot_digest"`
}

type renderArtifactResponse struct {
	ID           string `json:"id"`
	MediaAssetID string `json:"media_asset_id"`
	ByteSize     int64  `json:"byte_size"`
	SHA256       string `json:"sha256"`
	MimeType     string `json:"mime_type"`
	DurationMS   int64  `json:"duration_ms"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Toolchain    string `json:"toolchain_version"`
	CreatedAt    string `json:"created_at"`
}

type renderExportResponse struct {
	ID             string                  `json:"id"`
	State          string                  `json:"state"`
	Attempt        int                     `json:"attempt"`
	MaxAttempts    int                     `json:"max_attempts"`
	ErrorCode      *string                 `json:"error_code,omitempty"`
	SnapshotDigest string                  `json:"snapshot_digest"`
	ProfileID      string                  `json:"profile_id"`
	Artifact       *renderArtifactResponse `json:"artifact,omitempty"`
	CreatedAt      string                  `json:"created_at"`
	UpdatedAt      string                  `json:"updated_at"`
}

func (h renderExportHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	var req createRenderExportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return
	}
	job, err := h.service.Enqueue(r.Context(), principal.OwnerID, projectID, req.SnapshotDigest)
	if err != nil {
		writeRenderExportAPIError(w, err)
		return
	}
	view, err := h.service.Get(r.Context(), principal.OwnerID, projectID, job.ID)
	if err != nil {
		writeRenderExportAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusAccepted, toRenderExportResponse(view))
}

func (h renderExportHandler) get(w http.ResponseWriter, r *http.Request) {
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
	view, err := h.service.Get(r.Context(), principal.OwnerID, projectID, jobID)
	if err != nil {
		writeRenderExportAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toRenderExportResponse(view))
}

func (h renderExportHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	if h.actorResolver == nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil || principal.OwnerID == uuid.Nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func toRenderExportResponse(view renderexport.JobView) renderExportResponse {
	response := renderExportResponse{
		ID:             view.ID.String(),
		State:          string(view.State),
		Attempt:        view.Attempt,
		MaxAttempts:    view.MaxAttempts,
		ErrorCode:      view.ErrorCode,
		SnapshotDigest: view.SnapshotDigest,
		ProfileID:      view.ProfileID,
		CreatedAt:      view.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      view.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if view.Artifact != nil {
		response.Artifact = &renderArtifactResponse{
			ID:           view.Artifact.ID.String(),
			MediaAssetID: view.Artifact.MediaAssetID.String(),
			ByteSize:     view.Artifact.ByteSize,
			SHA256:       view.Artifact.SHA256,
			MimeType:     view.Artifact.MimeType,
			DurationMS:   view.Artifact.DurationMS,
			Width:        view.Artifact.Width,
			Height:       view.Artifact.Height,
			Toolchain:    view.Artifact.ToolchainVersion,
			CreatedAt:    view.Artifact.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	return response
}

func writeRenderExportAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, renderexport.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, renderexport.ErrInvalidRequest):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "invalid_render_request", Message: "Render request is invalid."}})
	case errors.Is(err, renderexport.ErrSnapshotMismatch):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "snapshot_mismatch", Message: "Snapshot identity is not valid for this project."}})
	case errors.Is(err, renderexport.ErrRenderNotFound), errors.Is(err, sceneeditor.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "not_found", Message: "Resource was not found."}})
	case errors.Is(err, renderexport.ErrRenderArtifactUnavailable):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "render_artifact_unavailable", Message: "Render completion is not durably available."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

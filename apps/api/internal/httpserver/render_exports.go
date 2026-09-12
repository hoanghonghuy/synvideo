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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type RenderExportService interface {
	Enqueue(context.Context, uuid.UUID, uuid.UUID, string, ...renderexport.SubtitleMode) (jobs.Job, error)
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (renderexport.JobView, error)
	Cancel(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (renderexport.JobView, error)
	Retry(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (renderexport.JobView, error)
	ListHistory(context.Context, uuid.UUID, uuid.UUID, int, string) (renderexport.HistoryResult, error)
}

type renderExportHandler struct {
	service       RenderExportService
	actorResolver actor.Resolver
}

type createRenderExportRequest struct {
	SnapshotDigest string                    `json:"snapshot_digest"`
	SubtitleMode   renderexport.SubtitleMode `json:"subtitle_mode,omitempty"`
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
	ID                  string                    `json:"id"`
	State               string                    `json:"state"`
	Attempt             int                       `json:"attempt"`
	MaxAttempts         int                       `json:"max_attempts"`
	ErrorCode           *string                   `json:"error_code,omitempty"`
	SnapshotDigest      string                    `json:"snapshot_digest"`
	ProfileID           string                    `json:"profile_id"`
	SubtitleMode        renderexport.SubtitleMode `json:"subtitle_mode"`
	RetryOfRenderJobID  *string                   `json:"retry_of_render_job_id,omitempty"`
	CancellationPending bool                      `json:"cancellation_pending"`
	Artifact            *renderArtifactResponse   `json:"artifact,omitempty"`
	CreatedAt           string                    `json:"created_at"`
	UpdatedAt           string                    `json:"updated_at"`
}

type retryRenderExportRequest struct {
	RequestID string `json:"request_id"`
}

type renderExportHistoryResponse struct {
	Items      []renderExportResponse `json:"items"`
	NextCursor string                 `json:"next_cursor,omitempty"`
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
	job, err := h.service.Enqueue(r.Context(), principal.OwnerID, projectID, req.SnapshotDigest, req.SubtitleMode)
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

func (h renderExportHandler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIError(w, project.ValidationError{Fields: map[string]string{"limit": "invalid"}})
			return
		}
		limit = parsed
	}
	result, err := h.service.ListHistory(r.Context(), principal.OwnerID, projectID, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeRenderExportAPIError(w, err)
		return
	}
	items := make([]renderExportResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toRenderExportResponse(item))
	}
	writeProjectJSON(w, http.StatusOK, renderExportHistoryResponse{Items: items, NextCursor: result.NextCursor})
}

func (h renderExportHandler) cancel(w http.ResponseWriter, r *http.Request) {
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
	view, err := h.service.Cancel(r.Context(), principal.OwnerID, projectID, jobID)
	if err != nil {
		writeRenderExportAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toRenderExportResponse(view))
}

func (h renderExportHandler) retry(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	sourceJobID, err := uuid.Parse(r.PathValue("job_id"))
	if err != nil || sourceJobID == uuid.Nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"job_id": "invalid_uuid"}})
		return
	}
	var req retryRenderExportRequest
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
	view, err := h.service.Retry(r.Context(), principal.OwnerID, projectID, sourceJobID, requestID)
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
		ID:                  view.ID.String(),
		State:               string(view.State),
		Attempt:             view.Attempt,
		MaxAttempts:         view.MaxAttempts,
		ErrorCode:           view.ErrorCode,
		SnapshotDigest:      view.SnapshotDigest,
		ProfileID:           view.ProfileID,
		SubtitleMode:        view.SubtitleMode,
		CancellationPending: view.CancellationPending,
		CreatedAt:           view.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:           view.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if view.RetryOfRenderJobID != nil {
		retryID := view.RetryOfRenderJobID.String()
		response.RetryOfRenderJobID = &retryID
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
	case errors.Is(err, renderexport.ErrRenderNotCancellable):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "render_not_cancellable", Message: "Render job cannot be cancelled in its current state."}})
	case errors.Is(err, renderexport.ErrRenderNotRetryable):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "render_not_retryable", Message: "Render job cannot be retried in its current state."}})
	case errors.Is(err, renderexport.ErrRetryRequestConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "render_retry_conflict", Message: "Render retry request identity conflicts with an existing job."}})
	case errors.Is(err, renderexport.ErrInvalidHistoryCursor):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "invalid_cursor", Message: "Render history cursor is invalid."}})
	case errors.Is(err, renderexport.ErrRenderArtifactUnavailable):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "render_artifact_unavailable", Message: "Render completion is not durably available."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

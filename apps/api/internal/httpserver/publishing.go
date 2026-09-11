package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type PublishingService interface {
	ListConnections(ctx context.Context, ownerID uuid.UUID) ([]publishing.ChannelConnection, error)
	GetAttempt(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (publishing.PublishAttempt, error)
	CreateAttempt(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID, title, description string) (publishing.PublishAttempt, error)
}

type publishingHistoryService interface {
	ListAttempts(ctx context.Context, ownerID, projectID uuid.UUID) ([]publishing.PublishAttempt, error)
}

type publishingArtifactService interface {
	ListPublishArtifacts(ctx context.Context, ownerID, projectID uuid.UUID) ([]publishing.PublishArtifactSummary, error)
}

type publishingActorResolver interface {
	Resolve(*http.Request) (project.Principal, error)
}

type publishingHandler struct {
	service       PublishingService
	actorResolver publishingActorResolver
}

type createPublishAttemptRequest struct {
	ConnectionID     string `json:"connection_id"`
	RenderArtifactID string `json:"render_artifact_id"`
	RequestID        string `json:"request_id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
}

func (h publishingHandler) listConnections(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListConnections(r.Context(), principal.OwnerID)
	if err != nil {
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h publishingHandler) listArtifacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parsePublishingUUID(w, r.PathValue("id"), "project_id")
	if !ok {
		return
	}
	service, ok := h.service.(publishingArtifactService)
	if !ok {
		writePublishingAPIError(w, publishing.ErrInvalidModel)
		return
	}
	items, err := service.ListPublishArtifacts(r.Context(), principal.OwnerID, projectID)
	if err != nil {
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h publishingHandler) listAttempts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parsePublishingUUID(w, r.PathValue("id"), "project_id")
	if !ok {
		return
	}
	service, ok := h.service.(publishingHistoryService)
	if !ok {
		writePublishingAPIError(w, publishing.ErrInvalidModel)
		return
	}
	items, err := service.ListAttempts(r.Context(), principal.OwnerID, projectID)
	if err != nil {
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h publishingHandler) getAttempt(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parsePublishingUUID(w, r.PathValue("id"), "project_id")
	if !ok {
		return
	}
	attemptID, ok := parsePublishingUUID(w, r.PathValue("attempt_id"), "attempt_id")
	if !ok {
		return
	}
	attempt, err := h.service.GetAttempt(r.Context(), principal.OwnerID, projectID, attemptID)
	if err != nil {
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, attempt)
}

func (h publishingHandler) createAttempt(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parsePublishingUUID(w, r.PathValue("id"), "project_id")
	if !ok {
		return
	}

	var input createPublishAttemptRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writePublishingValidationError(w, map[string]string{"body": "invalid_json"})
		return
	}
	connectionID, ok := parsePublishingUUID(w, input.ConnectionID, "connection_id")
	if !ok {
		return
	}
	artifactID, ok := parsePublishingUUID(w, input.RenderArtifactID, "render_artifact_id")
	if !ok {
		return
	}
	requestID, ok := parsePublishingUUID(w, input.RequestID, "request_id")
	if !ok {
		return
	}
	if strings.TrimSpace(input.Title) == "" {
		writePublishingValidationError(w, map[string]string{"title": "required"})
		return
	}

	attempt, err := h.service.CreateAttempt(r.Context(), principal.OwnerID, projectID, connectionID, artifactID, requestID, input.Title, input.Description)
	if err != nil {
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusCreated, attempt)
}

func (h publishingHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil || principal.OwnerID == uuid.Nil {
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
		return project.Principal{}, false
	}
	return principal, true
}

func parsePublishingUUID(w http.ResponseWriter, raw, field string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		writePublishingValidationError(w, map[string]string{field: "invalid_uuid"})
		return uuid.Nil, false
	}
	return id, true
}

func writePublishingValidationError(w http.ResponseWriter, fields map[string]string) {
	writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "validation_failed", Message: "Request validation failed.", Fields: fields}})
}

func writePublishingAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, publishing.ErrConnectionNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "publishing_connection_not_found", Message: "Publishing connection was not found or is unavailable."}})
	case errors.Is(err, publishing.ErrAttemptNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "publishing_attempt_not_found", Message: "Publishing attempt was not found."}})
	case errors.Is(err, publishing.ErrAttemptConflict):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "publishing_attempt_conflict", Message: "A conflicting publish request already exists."}})
	case errors.Is(err, publishing.ErrInvalidModel):
		writePublishingValidationError(w, map[string]string{"request": "invalid"})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}

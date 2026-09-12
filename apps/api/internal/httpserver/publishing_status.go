package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type publishingStatusService interface {
	Reconcile(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (publishing.PublishAttempt, error)
}

type publishingStatusHandler struct {
	service       publishingStatusService
	actorResolver publishingActorResolver
}

func (h publishingStatusHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil || principal.OwnerID == uuid.Nil {
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
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

	attempt, err := h.service.Reconcile(r.Context(), principal.OwnerID, projectID, attemptID)
	if err != nil {
		if errors.Is(err, publishing.ErrPublishStatusReconcile) {
			writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "publishing_status_conflict", Message: "YouTube status cannot be refreshed for this attempt in its current state."}})
			return
		}
		writePublishingAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, attempt)
}

// WithPublishingStatusRoutes layers the post-upload provider reconciliation action
// over the existing publishing API without widening the base PublishingService.
func WithPublishingStatusRoutes(logger *slog.Logger, base http.Handler, service publishingStatusService, resolver actor.Resolver) http.Handler {
	if service == nil || resolver == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}

	handler := publishingStatusHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/projects/{id}/publishing/attempts/{attempt_id}/reconcile", requestLogger(logger, http.HandlerFunc(handler.reconcile)))
	mux.Handle("/", base)
	return mux
}

var _ project.Principal = project.Principal{}

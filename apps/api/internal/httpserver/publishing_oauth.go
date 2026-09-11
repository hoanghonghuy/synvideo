package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type YouTubeConnectService interface {
	Start(ownerID, projectID uuid.UUID) (string, error)
	Complete(rctx interface{ Done() <-chan struct{} }, rawState, code string) (publishing.ChannelConnection, uuid.UUID, error)
	ReturnURL(projectID uuid.UUID, status string) string
}

// publishingConnectService is intentionally the concrete narrow contract used here.
// Keeping it local avoids widening the main PublishingService used by existing tests.
type publishingConnectService interface {
	Start(ownerID, projectID uuid.UUID) (string, error)
	CompleteHTTP(r *http.Request, state, code string) (publishing.ChannelConnection, uuid.UUID, error)
	ReturnURL(projectID uuid.UUID, status string) string
}

type youtubeConnectAdapter struct{ service *publishing.YouTubeConnectService }

func (a youtubeConnectAdapter) Start(ownerID, projectID uuid.UUID) (string, error) {
	return a.service.Start(ownerID, projectID)
}
func (a youtubeConnectAdapter) CompleteHTTP(r *http.Request, state, code string) (publishing.ChannelConnection, uuid.UUID, error) {
	return a.service.Complete(r.Context(), state, code)
}
func (a youtubeConnectAdapter) ReturnURL(projectID uuid.UUID, status string) string {
	return a.service.ReturnURL(projectID, status)
}

type publishingOAuthHandler struct {
	service       publishingConnectService
	actorResolver publishingActorResolver
}

func (h publishingOAuthHandler) start(w http.ResponseWriter, r *http.Request) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil || principal.OwnerID == uuid.Nil {
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
		return
	}
	projectID, err := uuid.Parse(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if err != nil || projectID == uuid.Nil {
		writePublishingValidationError(w, map[string]string{"project_id": "invalid_uuid"})
		return
	}
	authorizationURL, err := h.service.Start(principal.OwnerID, projectID)
	if err != nil {
		writeProjectJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "youtube_oauth_unavailable", Message: "YouTube authorization is not available."}})
		return
	}
	writeProjectJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (h publishingOAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		h.redirectFailure(w, r, state)
		return
	}
	if state == "" || code == "" {
		h.redirectFailure(w, r, state)
		return
	}
	_, projectID, err := h.service.CompleteHTTP(r, state, code)
	if err != nil {
		h.redirectFailure(w, r, state)
		return
	}
	http.Redirect(w, r, h.service.ReturnURL(projectID, "connected"), http.StatusSeeOther)
}

func (h publishingOAuthHandler) redirectFailure(w http.ResponseWriter, r *http.Request, state string) {
	// Do not reflect state or provider error text. A failed callback may not have a
	// trustworthy project identity, so return a small safe response instead of an
	// open redirect. The Channel Hub remains recoverable through a fresh start call.
	writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "youtube_oauth_failed", Message: "YouTube authorization could not be completed. Return to Channel Hub and try again."}})
}

func WithPublishingOAuthRoutes(logger *slog.Logger, base http.Handler, service *publishing.YouTubeConnectService, resolver actor.Resolver) http.Handler {
	if service == nil || resolver == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}
	handler := publishingOAuthHandler{service: youtubeConnectAdapter{service: service}, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/publishing/youtube/oauth/start", requestLogger(logger, http.HandlerFunc(handler.start)))
	mux.Handle("GET /api/v1/publishing/youtube/oauth/callback", requestLogger(logger, http.HandlerFunc(handler.callback)))
	mux.Handle("/", base)
	return mux
}

var _ project.Principal
var _ = errors.Is

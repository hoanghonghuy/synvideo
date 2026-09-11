package httpserver

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type publishingOAuthHandler struct {
	service       *publishing.YouTubeConnectService
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
	projectID, stateErr := h.service.ProjectFromState(state)
	if stateErr != nil {
		writeOAuthFailure(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("error")) != "" || strings.TrimSpace(r.URL.Query().Get("code")) == "" {
		http.Redirect(w, r, h.service.ReturnURL(projectID, "failed"), http.StatusSeeOther)
		return
	}
	_, completedProjectID, err := h.service.Complete(r.Context(), state, r.URL.Query().Get("code"))
	if err != nil {
		http.Redirect(w, r, h.service.ReturnURL(projectID, "failed"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, h.service.ReturnURL(completedProjectID, "connected"), http.StatusSeeOther)
}

func writeOAuthFailure(w http.ResponseWriter) {
	writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "youtube_oauth_failed", Message: "YouTube authorization could not be completed. Return to Channel Hub and try again."}})
}

func WithPublishingOAuthRoutes(logger *slog.Logger, base http.Handler, service *publishing.YouTubeConnectService, resolver actor.Resolver) http.Handler {
	if service == nil || resolver == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}
	handler := publishingOAuthHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/publishing/youtube/oauth/start", requestLogger(logger, http.HandlerFunc(handler.start)))
	mux.Handle("GET /api/v1/publishing/youtube/oauth/callback", requestLogger(logger, http.HandlerFunc(handler.callback)))
	mux.Handle("/", base)
	return mux
}

package httpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

const youtubeOAuthStateCookie = "synvideo_youtube_oauth_state"

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
	parsedAuthorizationURL, err := url.Parse(authorizationURL)
	if err != nil || strings.TrimSpace(parsedAuthorizationURL.Query().Get("state")) == "" {
		writeProjectJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "youtube_oauth_unavailable", Message: "YouTube authorization is not available."}})
		return
	}
	setYouTubeOAuthCookie(w, r, oauthStateCookieValue(parsedAuthorizationURL.Query().Get("state")), 600)
	writeProjectJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (h publishingOAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	cookie, err := r.Cookie(youtubeOAuthStateCookie)
	if err != nil || !constantTimeStringEqual(cookie.Value, oauthStateCookieValue(state)) {
		writeOAuthFailure(w)
		return
	}
	setYouTubeOAuthCookie(w, r, "", -1)

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

func oauthStateCookieValue(state string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(state)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func constantTimeStringEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func setYouTubeOAuthCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     youtubeOAuthStateCookie,
		Value:    value,
		Path:     "/api/v1/publishing/youtube/oauth/callback",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
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

package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

// RegisterPublishingRoutes mounts the owner-scoped publishing API on an existing mux.
// It remains useful for focused handler tests and callers that own the mux directly.
func RegisterPublishingRoutes(mux *http.ServeMux, service PublishingService, actorResolver publishingActorResolver) {
	if mux == nil || service == nil || actorResolver == nil {
		return
	}

	handler := publishingHandler{service: service, actorResolver: actorResolver}
	mux.HandleFunc("GET /api/v1/publishing/connections", handler.listConnections)
	mux.HandleFunc("POST /api/v1/projects/{id}/publishing/attempts", handler.createAttempt)
	mux.HandleFunc("GET /api/v1/projects/{id}/publishing/attempts/{attempt_id}", handler.getAttempt)
}

// WithPublishingRoutes layers publishing endpoints over an existing server handler so
// production bootstrap can attach the feature without assuming the base is a ServeMux.
func WithPublishingRoutes(logger *slog.Logger, base http.Handler, service PublishingService, resolver actor.Resolver) http.Handler {
	if service == nil || resolver == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}

	handler := publishingHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/publishing/connections", requestLogger(logger, http.HandlerFunc(handler.listConnections)))
	mux.Handle("POST /api/v1/projects/{id}/publishing/attempts", requestLogger(logger, http.HandlerFunc(handler.createAttempt)))
	mux.Handle("GET /api/v1/projects/{id}/publishing/attempts/{attempt_id}", requestLogger(logger, http.HandlerFunc(handler.getAttempt)))
	mux.Handle("/", base)
	return mux
}

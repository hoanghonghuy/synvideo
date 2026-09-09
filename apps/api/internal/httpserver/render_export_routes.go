package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

func WithRenderExportRoutes(logger *slog.Logger, base http.Handler, service RenderExportService, resolver actor.Resolver) http.Handler {
	if service == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}
	handler := renderExportHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/projects/{id}/render-exports", requestLogger(logger, http.HandlerFunc(handler.create)))
	mux.Handle("GET /api/v1/projects/{id}/render-exports", requestLogger(logger, http.HandlerFunc(handler.list)))
	mux.Handle("GET /api/v1/projects/{id}/render-exports/{job_id}", requestLogger(logger, http.HandlerFunc(handler.get)))
	mux.Handle("POST /api/v1/projects/{id}/render-exports/{job_id}/cancel", requestLogger(logger, http.HandlerFunc(handler.cancel)))
	mux.Handle("POST /api/v1/projects/{id}/render-exports/{job_id}/retry", requestLogger(logger, http.HandlerFunc(handler.retry)))
	mux.Handle("/", base)
	return mux
}

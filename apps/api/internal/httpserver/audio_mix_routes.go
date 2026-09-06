package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

func WithAudioMixRoutes(logger *slog.Logger, base http.Handler, service AudioMixService, resolver actor.Resolver) http.Handler {
	if service == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}
	handler := audioMixHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/projects/{id}/audio-mix", requestLogger(logger, http.HandlerFunc(handler.create)))
	mux.Handle("GET /api/v1/projects/{id}/audio-mix", requestLogger(logger, http.HandlerFunc(handler.get)))
	mux.Handle("PUT /api/v1/projects/{id}/audio-mix", requestLogger(logger, http.HandlerFunc(handler.update)))
	mux.Handle("POST /api/v1/projects/{id}/audio-mix/rebind-narration", requestLogger(logger, http.HandlerFunc(handler.rebindNarration)))
	mux.Handle("GET /api/v1/projects/{id}/audio-mix/history", requestLogger(logger, http.HandlerFunc(handler.history)))
	mux.Handle("GET /api/v1/projects/{id}/audio-mix/snapshot", requestLogger(logger, http.HandlerFunc(handler.snapshot)))
	mux.Handle("/", base)
	return mux
}

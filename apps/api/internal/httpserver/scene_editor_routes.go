package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

func WithSceneEditorRoutes(logger *slog.Logger, base http.Handler, service SceneEditorService, resolver actor.Resolver) http.Handler {
	if service == nil {
		return base
	}
	if base == nil {
		base = http.NotFoundHandler()
	}
	handler := sceneEditorHandler{service: service, actorResolver: resolver}
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/projects/{id}/scene-editor", requestLogger(logger, http.HandlerFunc(handler.create)))
	mux.Handle("GET /api/v1/projects/{id}/scene-editor", requestLogger(logger, http.HandlerFunc(handler.get)))
	mux.Handle("PUT /api/v1/projects/{id}/scene-editor", requestLogger(logger, http.HandlerFunc(handler.update)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/scenes/{scene_id}/reorder", requestLogger(logger, http.HandlerFunc(handler.reorder)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/scenes/{scene_id}/duplicate", requestLogger(logger, http.HandlerFunc(handler.duplicate)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/scenes/{scene_id}/remove", requestLogger(logger, http.HandlerFunc(handler.remove)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/reconcile/preview", requestLogger(logger, http.HandlerFunc(handler.previewReconcile)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/reconcile", requestLogger(logger, http.HandlerFunc(handler.reconcile)))
	mux.Handle("POST /api/v1/projects/{id}/scene-editor/snapshots", requestLogger(logger, http.HandlerFunc(handler.snapshot)))
	mux.Handle("/", base)
	return mux
}

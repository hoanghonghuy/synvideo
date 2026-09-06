package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

// AttachStockMediaRoutes adds the optional stock-media surface without changing
// the base server constructor contract used by tests and other entrypoints.
// Existing routes keep their already-wrapped handler; stock routes receive the
// same JSON body bound and request logging guarantees.
func AttachStockMediaRoutes(server *http.Server, logger *slog.Logger, service StockMediaService, actorResolver actor.Resolver) {
	if server == nil || service == nil || actorResolver == nil {
		return
	}
	base := server.Handler
	handler := stockMediaHandler{service: service, actorResolver: actorResolver}
	stockMux := http.NewServeMux()
	stockMux.HandleFunc("GET /api/v1/projects/{id}/stock-media/search", handler.search)
	stockMux.HandleFunc("POST /api/v1/projects/{id}/stock-media/acquisitions", handler.acquire)
	stockMux.Handle("/", base)
	server.Handler = requestLogger(logger, limitJSONRequestBody(defaultMaxJSONBodyBytes, stockMux))
}

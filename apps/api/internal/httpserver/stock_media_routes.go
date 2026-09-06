package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
)

// AttachStockMediaRoutes adds the optional stock-media surface without changing
// the base server constructor contract used by tests and other entrypoints.
func AttachStockMediaRoutes(server *http.Server, logger *slog.Logger, service StockMediaService, actorResolver actor.Resolver) {
	if server == nil || service == nil || actorResolver == nil {
		return
	}
	base := server.Handler
	handler := stockMediaHandler{service: service, actorResolver: actorResolver}
	wrap := func(fn http.HandlerFunc) http.Handler {
		return requestLogger(logger, limitJSONRequestBody(defaultMaxJSONBodyBytes, fn))
	}
	stockMux := http.NewServeMux()
	stockMux.Handle("GET /api/v1/projects/{id}/stock-media/search", wrap(handler.search))
	stockMux.Handle("POST /api/v1/projects/{id}/stock-media/acquisitions", wrap(handler.acquire))
	stockMux.Handle("/", base)
	server.Handler = stockMux
}

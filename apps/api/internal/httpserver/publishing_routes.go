package httpserver

import "net/http"

// RegisterPublishingRoutes mounts the owner-scoped publishing API on an existing mux.
// Keeping route registration separate from the handlers lets production bootstrap wire
// publishing only when the service and actor resolver are configured.
func RegisterPublishingRoutes(mux *http.ServeMux, service PublishingService, actorResolver publishingActorResolver) {
	if mux == nil || service == nil || actorResolver == nil {
		return
	}

	handler := publishingHandler{service: service, actorResolver: actorResolver}
	mux.HandleFunc("GET /api/v1/publishing/connections", handler.listConnections)
	mux.HandleFunc("POST /api/v1/projects/{id}/publishing/attempts", handler.createAttempt)
	mux.HandleFunc("GET /api/v1/projects/{id}/publishing/attempts/{attempt_id}", handler.getAttempt)
}

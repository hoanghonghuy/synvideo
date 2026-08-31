package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type statusResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment,omitempty"`
}

type ProjectService interface {
	Create(ctx context.Context, principal project.Principal, input project.CreateInput) (project.Project, error)
	List(ctx context.Context, principal project.Principal, limit int, cursorValue string) (project.ListResult, string, error)
	Get(ctx context.Context, principal project.Principal, id uuid.UUID) (project.Project, error)
	Update(ctx context.Context, principal project.Principal, id uuid.UUID, input project.UpdateInput) (project.Project, error)
}

type CreativeBriefService interface {
	Get(ctx context.Context, principal project.Principal, projectID uuid.UUID) (creativebrief.CreativeBrief, error)
	Put(ctx context.Context, principal project.Principal, projectID uuid.UUID, input creativebrief.PutInput) (creativebrief.CreativeBrief, bool, error)
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	projectService ProjectService,
	creativeBriefService CreativeBriefService,
	actorResolver actor.Resolver,
) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", healthHandler)
	mux.HandleFunc("GET /api/v1/readyz", readinessHandler(cfg))
	if projectService != nil && actorResolver != nil {
		handler := projectHandler{service: projectService, actorResolver: actorResolver}
		mux.HandleFunc("POST /api/v1/projects", handler.create)
		mux.HandleFunc("GET /api/v1/projects", handler.list)
		mux.HandleFunc("GET /api/v1/projects/{id}", handler.get)
		mux.HandleFunc("PATCH /api/v1/projects/{id}", handler.update)
	}
	if creativeBriefService != nil && actorResolver != nil {
		handler := creativeBriefHandler{service: creativeBriefService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/projects/{id}/creative-brief", handler.get)
		mux.HandleFunc("PUT /api/v1/projects/{id}/creative-brief", handler.put)
	}

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: requestLogger(logger, mux),
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func readinessHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, statusResponse{
			Status:      "ready",
			Environment: cfg.Environment,
		})
	}
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("api request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body statusResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

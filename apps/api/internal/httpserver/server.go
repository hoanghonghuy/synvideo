package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
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

type CreativeProposalService interface {
	List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error)
	Get(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error)
	UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error)
	Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error)
}

type ScriptService interface {
	List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]script.Script, error)
	GetByVersion(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (script.Script, error)
	UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error)
	Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (script.Script, error)
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	projectService ProjectService,
	creativeBriefService CreativeBriefService,
	creativeProposalService CreativeProposalService,
	scriptService ScriptService,
	proposalGenerationService ProposalGenerationService,
	textProviderSettingsService TextProviderSettingsService,
	scriptGenerationService ScriptGenerationService,
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
	if creativeProposalService != nil && actorResolver != nil {
		handler := creativeProposalHandler{service: creativeProposalService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/projects/{id}/creative-proposals", handler.list)
		mux.HandleFunc("GET /api/v1/projects/{id}/creative-proposals/{version}", handler.get)
		mux.HandleFunc("PUT /api/v1/projects/{id}/creative-proposals/{version}", handler.put)
		mux.HandleFunc("POST /api/v1/projects/{id}/creative-proposals/{version}/approve", handler.approve)
	}
	if scriptService != nil && actorResolver != nil {
		handler := scriptHandler{service: scriptService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/projects/{id}/scripts", handler.list)
		mux.HandleFunc("GET /api/v1/projects/{id}/scripts/{version}", handler.get)
		mux.HandleFunc("PUT /api/v1/projects/{id}/scripts/{version}", handler.put)
		mux.HandleFunc("POST /api/v1/projects/{id}/scripts/{version}/approve", handler.approve)
	}
	if proposalGenerationService != nil {
		handler := creativeProposalGenerationHandler{service: proposalGenerationService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/ai/text-generation-options", handler.getTextGenerationOptions)
		if actorResolver != nil {
			mux.HandleFunc("POST /api/v1/projects/{id}/creative-proposal-generations", handler.create)
			mux.HandleFunc("GET /api/v1/projects/{id}/creative-proposal-generations/{job_id}", handler.get)
		}
	}
	if textProviderSettingsService != nil && actorResolver != nil {
		handler := providerSettingsHandler{service: textProviderSettingsService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/ai/provider-settings", handler.list)
		mux.HandleFunc("PUT /api/v1/ai/provider-settings/{provider_id}", handler.put)
		mux.HandleFunc("DELETE /api/v1/ai/provider-settings/{provider_id}", handler.delete)
	}
	if scriptGenerationService != nil && actorResolver != nil {
		handler := scriptGenerationHandler{service: scriptGenerationService, actorResolver: actorResolver}
		mux.HandleFunc("POST /api/v1/projects/{id}/script-generations", handler.create)
		mux.HandleFunc("GET /api/v1/projects/{id}/script-generations/{job_id}", handler.get)
	}

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: requestLogger(logger, mux),
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeProjectJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func readinessHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeProjectJSON(w, http.StatusOK, statusResponse{
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

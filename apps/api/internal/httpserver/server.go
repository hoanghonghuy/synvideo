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

type MediaServices struct {
	Assets          MediaAssetService
	Bindings        SceneMediaService
	GeneratedImages GeneratedImageGenerationService
	GeneratedVideos GeneratedVideoGenerationService
	Narrations      SceneNarrationService
	GeneratedAudio  SceneNarrationGenerationService
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	projectService ProjectService,
	creativeBriefService CreativeBriefService,
	creativeProposalService CreativeProposalService,
	scriptService ScriptService,
	scenePlanService ScenePlanService,
	proposalGenerationService ProposalGenerationService,
	providerSettingsService ProviderSettingsService,
	scriptGenerationService ScriptGenerationService,
	scenePlanGenerationService ScenePlanGenerationService,
	actorResolver actor.Resolver,
	mediaServices ...MediaServices,
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
	if scenePlanService != nil && actorResolver != nil {
		handler := scenePlanHandler{service: scenePlanService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans", handler.list)
		mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans/{version}", handler.get)
		mux.HandleFunc("PUT /api/v1/projects/{id}/scene-plans/{version}", handler.put)
		mux.HandleFunc("POST /api/v1/projects/{id}/scene-plans/{version}/approve", handler.approve)
	}
	if proposalGenerationService != nil {
		handler := creativeProposalGenerationHandler{service: proposalGenerationService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/ai/text-generation-options", handler.getTextGenerationOptions)
		if actorResolver != nil {
			mux.HandleFunc("POST /api/v1/projects/{id}/creative-proposal-generations", handler.create)
			mux.HandleFunc("GET /api/v1/projects/{id}/creative-proposal-generations/{job_id}", handler.get)
		}
	}
	if providerSettingsService != nil && actorResolver != nil {
		handler := providerSettingsHandler{service: providerSettingsService, actorResolver: actorResolver}
		mux.HandleFunc("GET /api/v1/ai/provider-settings", handler.list)
		mux.HandleFunc("PUT /api/v1/ai/provider-settings/{provider_id}", handler.put)
		mux.HandleFunc("DELETE /api/v1/ai/provider-settings/{provider_id}", handler.delete)
		mux.HandleFunc("GET /api/v1/ai/image-generation-options", handler.getImageGenerationOptions)
		mux.HandleFunc("GET /api/v1/ai/tts-options", handler.getTTSOptions)
	}
	if scriptGenerationService != nil && actorResolver != nil {
		handler := scriptGenerationHandler{service: scriptGenerationService, actorResolver: actorResolver}
		mux.HandleFunc("POST /api/v1/projects/{id}/script-generations", handler.create)
		mux.HandleFunc("GET /api/v1/projects/{id}/script-generations/{job_id}", handler.get)
	}
	if scenePlanGenerationService != nil && actorResolver != nil {
		handler := scenePlanGenerationHandler{service: scenePlanGenerationService, actorResolver: actorResolver}
		mux.HandleFunc("POST /api/v1/projects/{id}/scene-plan-generations", handler.create)
		mux.HandleFunc("GET /api/v1/projects/{id}/scene-plan-generations/{job_id}", handler.get)
	}
	if len(mediaServices) > 0 {
		services := mediaServices[0]
		if services.Assets != nil && actorResolver != nil {
			maxUploadSize := cfg.MediaStorage.MaxUploadBytes
			if maxUploadSize <= 0 {
				maxUploadSize = config.DefaultMaxUploadBytes
			}
			handler := mediaAssetHandler{service: services.Assets, actorResolver: actorResolver, maxUploadSize: maxUploadSize}
			mux.HandleFunc("POST /api/v1/projects/{id}/media-assets", handler.upload)
			mux.HandleFunc("GET /api/v1/projects/{id}/media-assets", handler.list)
			mux.HandleFunc("GET /api/v1/projects/{id}/media-assets/{asset_id}", handler.get)
			mux.HandleFunc("GET /api/v1/projects/{id}/media-assets/{asset_id}/content", handler.content)
			mux.HandleFunc("DELETE /api/v1/projects/{id}/media-assets/{asset_id}", handler.delete)
		}
		if services.Assets != nil && services.Bindings != nil && actorResolver != nil {
			handler := sceneMediaHandler{bindings: services.Bindings, assets: services.Assets, actorResolver: actorResolver}
			mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans/{version}/media-bindings", handler.listCurrent)
			mux.HandleFunc("PUT /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/primary-visual", handler.assignPrimaryVisual)
			mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/primary-visual/history", handler.history)
		}
		if services.GeneratedImages != nil && actorResolver != nil {
			handler := generatedImageGenerationHandler{service: services.GeneratedImages, actorResolver: actorResolver}
			mux.HandleFunc("POST /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/image-generations", handler.create)
			mux.HandleFunc("GET /api/v1/projects/{id}/image-generations/{job_id}", handler.get)
		}
		if services.GeneratedVideos != nil && actorResolver != nil {
			handler := generatedVideoGenerationHandler{service: services.GeneratedVideos, actorResolver: actorResolver}
			mux.HandleFunc("POST /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/video-generations", handler.create)
			mux.HandleFunc("GET /api/v1/projects/{id}/video-generations/{job_id}", handler.get)
		}
		if services.Assets != nil && services.Narrations != nil && actorResolver != nil {
			handler := sceneNarrationHandler{bindings: services.Narrations, assets: services.Assets, actorResolver: actorResolver}
			mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans/{version}/narration-bindings", handler.listCurrent)
			mux.HandleFunc("PUT /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/narration", handler.assign)
			mux.HandleFunc("GET /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/narration/history", handler.history)
		}
		if services.GeneratedAudio != nil && actorResolver != nil {
			handler := sceneNarrationGenerationHandler{service: services.GeneratedAudio, actorResolver: actorResolver}
			mux.HandleFunc("POST /api/v1/projects/{id}/scene-plans/{version}/scenes/{scene_key}/narration-generations", handler.create)
			mux.HandleFunc("GET /api/v1/projects/{id}/narration-generations/{job_id}", handler.get)
		}
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

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgenerationjob"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var server *http.Server
	var projectService *project.Service
	var creativeBriefService *creativebrief.Service
	var creativeProposalService *creativeproposal.Service
	var scriptService *script.Service
	var scenePlanService *sceneplan.Service
	var proposalGenerationService *proposalgenerationjob.Service
	var scriptGenerationService *scriptgenerationjob.Service
	var scenePlanGenerationService *sceneplangenerationjob.Service
	if cfg.DatabaseURL != "" {
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database pool failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		if err := pool.Ping(ctx); err != nil {
			logger.Error("database ping failed", "error", err)
			os.Exit(1)
		}
		projectRepo := postgres.NewProjectRepository(pool)
		briefRepo := postgres.NewCreativeBriefRepository(pool)
		proposalRepo := postgres.NewCreativeProposalRepository(pool)
		scriptRepo := postgres.NewScriptRepository(pool)
		scenePlanRepo := postgres.NewScenePlanRepository(pool)
		jobsRepo := postgres.NewJobRepository(pool)
		settingsRepo := postgres.NewTextProviderSettingRepository(pool)

		var cipher providersettings.Cipher
		if cfg.CredentialEncryptionKey != "" {
			var cipherErr error
			cipher, cipherErr = providersettings.NewAESGCMCipher(cfg.CredentialEncryptionKey, cfg.CredentialKeyVersion)
			if cipherErr != nil {
				logger.Error("credential cipher initialization failed", "error", cipherErr)
			}
		}

		var catalog *providersettings.Catalog
		if cfg.TextProviderDefinitions != "" {
			var catErr error
			catalog, catErr = providersettings.NewCatalogFromJSON([]byte(cfg.TextProviderDefinitions))
			if catErr != nil {
				logger.Error("text provider definitions parsing failed", "error", catErr)
				os.Exit(1)
			}
		} else {
			catalog, _ = providersettings.NewCatalog([]providersettings.ProviderDefinition{
				{
					ProviderID:  "openai",
					DisplayName: "OpenAI",
					BaseURL:     "https://api.openai.com/v1",
					Models: []providersettings.ModelDefinition{
						{ModelID: "gpt-5-mini", DisplayName: "GPT-5 mini", ExternalModelID: "gpt-5-mini"},
						{ModelID: "gpt-4o", DisplayName: "GPT-4o", ExternalModelID: "gpt-4o"},
					},
				},
				{
					ProviderID:  "openrouter",
					DisplayName: "OpenRouter",
					BaseURL:     "https://openrouter.ai/api/v1",
					Models: []providersettings.ModelDefinition{
						{ModelID: "claude-3-5-sonnet", DisplayName: "Claude 3.5 Sonnet", ExternalModelID: "anthropic/claude-3.5-sonnet"},
					},
				},
			})
		}

		providerSettingsService := providersettings.NewService(catalog, settingsRepo, cipher, http.DefaultClient)

		projectService = project.NewService(projectRepo)
		creativeBriefService = creativebrief.NewService(briefRepo)
		creativeProposalService = creativeproposal.NewService(proposalRepo)
		scriptService = script.NewService(scriptRepo)
		scenePlanService = sceneplan.NewService(scenePlanRepo)

		proposalJobHandler := proposalgenerationjob.NewHandlerWithResolver(providerSettingsService, proposalRepo)
		scriptJobHandler := scriptgenerationjob.NewHandlerWithResolver(providerSettingsService, scriptRepo)
		scenePlanJobHandler := sceneplangenerationjob.NewHandlerWithResolver(providerSettingsService, scenePlanRepo)

		jobsRegistry := jobs.NewRegistry()
		if err := jobsRegistry.Register(proposalgenerationjob.JobKind, proposalJobHandler); err != nil {
			logger.Error("register proposal generation job handler failed", "error", err)
			os.Exit(1)
		}
		if err := jobsRegistry.Register(scriptgenerationjob.JobKind, scriptJobHandler); err != nil {
			logger.Error("register script generation job handler failed", "error", err)
			os.Exit(1)
		}
		if err := jobsRegistry.Register(sceneplangenerationjob.JobKind, scenePlanJobHandler); err != nil {
			logger.Error("register scene plan generation job handler failed", "error", err)
			os.Exit(1)
		}

		executor := jobs.NewExecutor(jobsRepo, jobsRegistry, jobs.ExecutorConfig{
			LeaseDuration:  30 * time.Second,
			PollInterval:   1 * time.Second,
			DefaultBackoff: 10 * time.Second,
		})
		go func() {
			if err := executor.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("job executor failed", "error", err)
			}
		}()

		proposalGenerationService = proposalgenerationjob.NewServiceWithRuntime(providerSettingsService, jobsRepo, projectRepo, briefRepo)
		scriptGenerationService = scriptgenerationjob.NewServiceWithRuntime(providerSettingsService, jobsRepo, projectRepo, proposalRepo)
		scenePlanGenerationService = sceneplangenerationjob.NewServiceWithRuntime(providerSettingsService, jobsRepo, projectRepo, scriptRepo, proposalRepo)
		server = httpserver.New(cfg, logger, projectService, creativeBriefService, creativeProposalService, scriptService, scenePlanService, proposalGenerationService, providerSettingsService, scriptGenerationService, scenePlanGenerationService, actor.NewLocalResolver(cfg))
	} else {
		server = httpserver.New(cfg, logger, projectService, creativeBriefService, creativeProposalService, scriptService, scenePlanService, proposalGenerationService, nil, scriptGenerationService, scenePlanGenerationService, actor.NewLocalResolver(cfg))
	}
	errCh := make(chan error, 1)

	go func() {
		logger.Info("api server listening", "addr", cfg.Addr, "environment", cfg.Environment)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("api server shutdown failed", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}
}

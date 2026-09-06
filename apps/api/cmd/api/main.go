package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/generatedimagejob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/s3storage"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenevideojob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia/pexels"
)

func loadProviderCatalog(cfg config.Config) (*providersettings.Catalog, error) {
	if cfg.ProviderDefinitions != "" {
		return providersettings.NewCatalogFromJSON([]byte(cfg.ProviderDefinitions))
	}
	if cfg.TextProviderDefinitions != "" {
		return providersettings.NewCatalogFromLegacyJSON([]byte(cfg.TextProviderDefinitions))
	}
	return providersettings.NewCatalog([]providersettings.ProviderDefinition{
		{
			ProviderID:  "openai",
			DisplayName: "OpenAI",
			BaseURL:     "https://api.openai.com/v1",
			Models: []providersettings.ModelDefinition{
				{ModelID: "gpt-5-mini", DisplayName: "GPT-5 mini", ExternalModelID: "gpt-5-mini", Capabilities: []providersettings.Capability{providersettings.CapabilityText}},
				{ModelID: "gpt-4o", DisplayName: "GPT-4o", ExternalModelID: "gpt-4o", Capabilities: []providersettings.Capability{providersettings.CapabilityText}},
				{ModelID: "dall-e-3", DisplayName: "DALL-E 3", ExternalModelID: "dall-e-3", Capabilities: []providersettings.Capability{providersettings.CapabilityImage}},
				{ModelID: "gpt-4o-mini-tts", DisplayName: "GPT-4o mini TTS", ExternalModelID: "gpt-4o-mini-tts", Capabilities: []providersettings.Capability{providersettings.CapabilityTTS}},
			},
			Voices: []providersettings.VoiceDefinition{
				{VoiceID: "alloy", DisplayName: "Alloy", ExternalVoice: "alloy"},
				{VoiceID: "verse", DisplayName: "Verse", ExternalVoice: "verse"},
			},
		},
		{
			ProviderID:  "openrouter",
			DisplayName: "OpenRouter",
			BaseURL:     "https://openrouter.ai/api/v1",
			Models: []providersettings.ModelDefinition{
				{ModelID: "claude-3-5-sonnet", DisplayName: "Claude 3.5 Sonnet", ExternalModelID: "anthropic/claude-3.5-sonnet", Capabilities: []providersettings.Capability{providersettings.CapabilityText}},
			},
		},
		{
			ProviderID:  "runway",
			DisplayName: "Runway",
			BaseURL:     "https://api.dev.runwayml.com",
			Models: []providersettings.ModelDefinition{
				{ModelID: "gen4.5", DisplayName: "Runway Gen-4.5", ExternalModelID: "gen4.5", Capabilities: []providersettings.Capability{providersettings.CapabilityVideo}},
			},
		},
	})
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	actorResolver := actor.NewLocalResolver(cfg)

	var server *http.Server
	var projectService *project.Service
	var creativeBriefService *creativebrief.Service
	var creativeProposalService *creativeproposal.Service
	var scriptService *script.Service
	var scenePlanService *sceneplan.Service
	var proposalGenerationService *proposalgenerationjob.Service
	var scriptGenerationService *scriptgenerationjob.Service
	var scenePlanGenerationService *sceneplangenerationjob.Service
	var generatedImageGenerationService *generatedimagejob.Service
	var generatedVideoGenerationService *scenevideojob.Service
	var sceneNarrationJobService *scenenarrationjob.Service
	var mediaAssetService *mediaasset.Service
	var sceneMediaService *scenemedia.Service
	var sceneNarrationService *scenenarration.Service
	var captionService *captions.Service
	var stockMediaService *stockmedia.Service
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
		settingsRepo := postgres.NewProviderSettingRepository(pool)
		mediaAssetRepo := postgres.NewMediaAssetRepository(pool)
		bindingRepo := postgres.NewSceneMediaBindingRepository(pool)
		narrationBindingRepo := postgres.NewSceneNarrationBindingRepository(pool)
		captionRepo := postgres.NewCaptionRepository(pool)
		videoOperationRepo := postgres.NewSceneVideoOperationRepository(pool)

		var cipher providersettings.Cipher
		if cfg.CredentialEncryptionKey != "" {
			var cipherErr error
			cipher, cipherErr = providersettings.NewAESGCMCipher(cfg.CredentialEncryptionKey, cfg.CredentialKeyVersion)
			if cipherErr != nil {
				logger.Error("credential cipher initialization failed", "error", cipherErr)
			}
		}

		catalog, catErr := loadProviderCatalog(cfg)
		if catErr != nil {
			logger.Error("provider definitions parsing failed", "error", catErr)
			os.Exit(1)
		}

		providerSettingsService := providersettings.NewService(catalog, settingsRepo, cipher, http.DefaultClient)

		projectService = project.NewService(projectRepo)
		creativeBriefService = creativebrief.NewService(briefRepo)
		creativeProposalService = creativeproposal.NewService(proposalRepo)
		scriptService = script.NewService(scriptRepo)
		scenePlanService = sceneplan.NewService(scenePlanRepo)
		captionService = captions.NewService(captionRepo, scenePlanRepo, narrationBindingRepo, mediaAssetRepo)

		var storage *s3storage.Storage
		if cfg.MediaStorage.Configured() {
			var storageErr error
			storage, storageErr = s3storage.New(s3storage.Config{
				Endpoint:        cfg.MediaStorage.Endpoint,
				Region:          cfg.MediaStorage.Region,
				Bucket:          cfg.MediaStorage.Bucket,
				AccessKeyID:     cfg.MediaStorage.AccessKeyID,
				SecretAccessKey: cfg.MediaStorage.SecretAccessKey,
				UsePathStyle:    cfg.MediaStorage.UsePathStyle,
				Timeout:         cfg.MediaStorage.Timeout,
			})
			if storageErr != nil {
				logger.Error("media storage initialization failed", "error", storageErr)
				os.Exit(1)
			}
			if cfg.Environment != config.EnvironmentProduction {
				if err := storage.EnsureBucket(ctx); err != nil {
					logger.Error("media storage bucket initialization failed", "error", err)
					os.Exit(1)
				}
			}
			mediaAssetService = mediaasset.NewService(projectRepo, mediaAssetRepo, storage)
			stockProviders := map[string]stockmedia.Provider{}
			if apiKey := strings.TrimSpace(os.Getenv("SYNVIDEO_PEXELS_API_KEY")); apiKey != "" {
				adapter, providerErr := pexels.New(pexels.Config{APIKey: apiKey})
				if providerErr != nil {
					logger.Error("stock media provider initialization failed", "provider", pexels.ProviderKey, "error", providerErr)
					os.Exit(1)
				}
				stockProviders[pexels.ProviderKey] = adapter
			}
			stockMediaService, err = stockmedia.NewService(projectRepo, mediaAssetService, stockProviders, cfg.MediaStorage.MaxUploadBytes)
			if err != nil {
				logger.Error("stock media service initialization failed", "error", err)
				os.Exit(1)
			}
			sceneMediaService = scenemedia.NewService(scenePlanRepo, mediaAssetRepo, bindingRepo)
			generatedImageGenerationService = generatedimagejob.NewService(providerSettingsService, jobsRepo, projectRepo, scenePlanRepo)
			generatedVideoGenerationService = scenevideojob.NewService(providerSettingsService, jobsRepo, projectRepo, scenePlanRepo)
			sceneNarrationService = scenenarration.NewService(narrationBindingRepo, scenePlanRepo, mediaAssetRepo)
			sceneNarrationJobService = scenenarrationjob.NewService(providerSettingsService, jobsRepo, projectRepo, scenePlanRepo)
		}

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
		if mediaAssetService != nil && sceneMediaService != nil {
			generatedAssetStore := generatedimagejob.NewAssetStore(mediaAssetService, mediaAssetRepo)
			generatedImageJobHandler := generatedimagejob.NewHandler(providerSettingsService, generatedAssetStore, sceneMediaService)
			if err := jobsRegistry.Register(generatedimagejob.JobKind, generatedImageJobHandler); err != nil {
				logger.Error("register generated image job handler failed", "error", err)
				os.Exit(1)
			}

			generatedVideoAssetStore := scenevideojob.NewAssetStore(mediaAssetService, mediaAssetRepo)
			generatedVideoJobHandler := scenevideojob.NewHandler(providerSettingsService, videoOperationRepo, generatedVideoAssetStore, sceneMediaService)
			if err := jobsRegistry.Register(scenevideojob.JobKind, generatedVideoJobHandler); err != nil {
				logger.Error("register generated video job handler failed", "error", err)
				os.Exit(1)
			}
		}
		if mediaAssetService != nil && sceneNarrationService != nil {
			narrationAssetStore := scenenarrationjob.NewAssetStore(mediaAssetService, mediaAssetRepo)
			narrationChunkStore := scenenarrationjob.NewObjectStorageChunkStore(storage)
			narrationJobHandler := scenenarrationjob.NewHandler(providerSettingsService, narrationAssetStore, sceneNarrationService, narrationChunkStore)
			if err := jobsRegistry.Register(scenenarrationjob.JobKind, narrationJobHandler); err != nil {
				logger.Error("register scene narration job handler failed", "error", err)
				os.Exit(1)
			}
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
		server = httpserver.New(cfg, logger, projectService, creativeBriefService, creativeProposalService, scriptService, scenePlanService, proposalGenerationService, providerSettingsService, scriptGenerationService, scenePlanGenerationService, actorResolver, httpserver.MediaServices{
			Assets:          mediaAssetService,
			Bindings:        sceneMediaService,
			GeneratedImages: generatedImageGenerationService,
			GeneratedVideos: generatedVideoGenerationService,
			Narrations:      sceneNarrationService,
			GeneratedAudio:  sceneNarrationJobService,
			Captions:        captionService,
		})
		httpserver.AttachStockMediaRoutes(server, logger, stockMediaService, actorResolver)
		server.Handler = withAudioMixRoutes(logger, server.Handler, pool, actorResolver)
	} else {
		server = httpserver.New(cfg, logger, projectService, creativeBriefService, creativeProposalService, scriptService, scenePlanService, proposalGenerationService, nil, scriptGenerationService, scenePlanGenerationService, actorResolver)
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
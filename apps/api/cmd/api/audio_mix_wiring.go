package main

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

func withAudioMixRoutes(logger *slog.Logger, base http.Handler, pool *pgxpool.Pool, resolver actor.Resolver) http.Handler {
	// This is the final DB-backed route composition point in the current bootstrap.
	// Attach publishing independently first so it remains available even when audio
	// mix internals evolve, while avoiding a second mutation of the large main wiring.
	base = withPublishingRoutes(logger, base, pool, resolver)

	repo := postgres.NewAudioMixRepository(pool)
	plans := postgres.NewScenePlanRepository(pool)
	narrations := postgres.NewSceneNarrationBindingRepository(pool)
	assets := postgres.NewMediaAssetRepository(pool)
	service := audiomix.NewService(repo, plans, narrations, assets)
	return httpserver.WithAudioMixRoutes(logger, base, service, resolver)
}

func withPublishingRoutes(logger *slog.Logger, base http.Handler, pool *pgxpool.Pool, resolver actor.Resolver) http.Handler {
	repo := postgres.NewPublishingRepository(pool)
	service, err := publishing.NewPublishService(repo, repo, nil)
	if err != nil {
		logger.Error("publishing service initialization failed", "error", err)
		return base
	}
	return httpserver.WithPublishingRoutes(logger, base, service, resolver)
}

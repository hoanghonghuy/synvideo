package main

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
)

func withAudioMixRoutes(logger *slog.Logger, base http.Handler, pool *pgxpool.Pool, resolver actor.Resolver) http.Handler {
	repo := postgres.NewAudioMixRepository(pool)
	plans := postgres.NewScenePlanRepository(pool)
	narrations := postgres.NewSceneNarrationBindingRepository(pool)
	assets := postgres.NewMediaAssetRepository(pool)
	service := audiomix.NewService(repo, plans, narrations, assets)
	return httpserver.WithAudioMixRoutes(logger, base, service, resolver)
}

package main

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

func withAudioMixRoutes(logger *slog.Logger, base http.Handler, pool *pgxpool.Pool, resolver actor.Resolver) http.Handler {
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
	base = httpserver.WithPublishingRoutes(logger, base, service, resolver)

	clientID := strings.TrimSpace(os.Getenv("SYNVIDEO_YOUTUBE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("SYNVIDEO_YOUTUBE_CLIENT_SECRET"))
	redirectURL := strings.TrimSpace(os.Getenv("SYNVIDEO_YOUTUBE_REDIRECT_URL"))
	stateSecret := strings.TrimSpace(os.Getenv("SYNVIDEO_YOUTUBE_OAUTH_STATE_SECRET"))
	returnBase := strings.TrimSpace(os.Getenv("SYNVIDEO_YOUTUBE_OAUTH_RETURN_BASE_URL"))
	credentialSecret := os.Getenv("SYNVIDEO_CREDENTIAL_ENCRYPTION_KEY")
	if credentialSecret == "" {
		credentialSecret = os.Getenv("SYNVIDEO_BYOK_ENCRYPTION_KEY")
	}
	if clientID == "" || clientSecret == "" || redirectURL == "" || stateSecret == "" || returnBase == "" || credentialSecret == "" {
		return base
	}

	oauth, err := publishing.NewYouTubeOAuth(publishing.YouTubeOAuthConfig{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
	}, http.DefaultClient)
	if err != nil {
		logger.Error("youtube oauth initialization failed", "error", err)
		return base
	}
	keyID := strings.TrimSpace(os.Getenv("SYNVIDEO_CREDENTIAL_KEY_VERSION"))
	if keyID == "" {
		keyID = "v1"
	}
	key := sha256.Sum256([]byte(credentialSecret))
	protector, err := publishing.NewAESGCMRefreshTokenProtector(keyID, map[string][]byte{keyID: key[:]})
	if err != nil {
		logger.Error("publishing credential protector initialization failed", "error", err)
		return base
	}
	connectionService, err := publishing.NewConnectionService(repo, protector)
	if err != nil {
		logger.Error("publishing connection service initialization failed", "error", err)
		return base
	}
	connectService, err := publishing.NewYouTubeConnectService(oauth, connectionService, stateSecret, returnBase, http.DefaultClient)
	if err != nil {
		logger.Error("youtube connect service initialization failed", "error", err)
		return base
	}
	base = httpserver.WithPublishingOAuthRoutes(logger, base, connectService, resolver)

	statusClient, err := publishing.NewYouTubeStatusClient("", http.DefaultClient)
	if err != nil {
		logger.Error("youtube status client initialization failed", "error", err)
		return base
	}
	statusService, err := publishing.NewPublishStatusService(repo, connectionService, oauth, statusClient)
	if err != nil {
		logger.Error("publishing status service initialization failed", "error", err)
		return base
	}
	return httpserver.WithPublishingStatusRoutes(logger, base, statusService, resolver)
}

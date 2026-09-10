package actor

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
)

// NewResolver constructs the environment-appropriate request principal resolver.
func NewResolver(cfg config.Config, mapper identity.Mapper) (Resolver, error) {
	switch cfg.Environment {
	case config.EnvironmentProduction:
		if mapper == nil {
			return nil, fmt.Errorf("identity mapper is required in production")
		}
		httpClient := &http.Client{Timeout: cfg.Auth.JWKSFetchTimeout}
		verifier, err := auth.BuildJWTVerifier(
			context.Background(),
			cfg.Auth.OIDCIssuer,
			cfg.Auth.OIDCAudience,
			cfg.Auth.JWKSURL,
			httpClient,
			cfg.Auth.JWKSCacheTTL,
			cfg.Auth.JWKSFetchTimeout,
		)
		if err != nil {
			return nil, fmt.Errorf("jwt verifier: %w", err)
		}
		return NewProductionResolver(verifier, mapper), nil
	default:
		return NewLocalResolver(cfg), nil
	}
}

// NewResolverForTest allows injecting a verifier without discovery for unit tests.
func NewProductionResolverForTest(verifier *auth.JWTVerifier, mapper identity.Mapper) Resolver {
	return NewProductionResolver(verifier, mapper)
}

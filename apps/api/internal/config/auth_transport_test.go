package config

import (
	"strings"
	"testing"
	"time"
)

func validProductionAuthConfig() AuthConfig {
	return AuthConfig{
		OIDCIssuer:       "https://issuer.example",
		OIDCAudience:     "synvideo-api",
		JWKSFetchTimeout: 5 * time.Second,
		JWKSCacheTTL:     5 * time.Minute,
	}
}

func TestAuthConfigValidateRejectsPlaintextIssuer(t *testing.T) {
	cfg := validProductionAuthConfig()
	cfg.OIDCIssuer = "http://issuer.example"

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected plaintext issuer to be rejected, got %v", err)
	}
}

func TestAuthConfigValidateRejectsPlaintextJWKSURL(t *testing.T) {
	cfg := validProductionAuthConfig()
	cfg.JWKSURL = "http://issuer.example/.well-known/jwks.json"

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected plaintext JWKS URL to be rejected, got %v", err)
	}
}

func TestAuthConfigValidateAcceptsHTTPSIssuerAndJWKSURL(t *testing.T) {
	cfg := validProductionAuthConfig()
	cfg.JWKSURL = "https://keys.example/.well-known/jwks.json"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected HTTPS auth transport to validate, got %v", err)
	}
}

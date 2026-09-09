package config

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResolveListenAddrPrefersExplicitOverride(t *testing.T) {
	t.Setenv("SYNVIDEO_API_ADDR", "127.0.0.1:9090")
	t.Setenv("PORT", "10000")

	if addr := resolveListenAddr(); addr != "127.0.0.1:9090" {
		t.Fatalf("expected SYNVIDEO_API_ADDR override, got %q", addr)
	}
}

func TestResolveListenAddrUsesRenderPORTWhenUnset(t *testing.T) {
	os.Unsetenv("SYNVIDEO_API_ADDR")
	t.Setenv("PORT", "10000")

	if addr := resolveListenAddr(); addr != ":10000" {
		t.Fatalf("expected Render PORT fallback, got %q", addr)
	}
}

func TestResolveListenAddrFallsBackToDefaultWhenUnset(t *testing.T) {
	os.Unsetenv("SYNVIDEO_API_ADDR")
	os.Unsetenv("PORT")

	if addr := resolveListenAddr(); addr != ":8080" {
		t.Fatalf("expected default listen address, got %q", addr)
	}
}

func TestConfigValidateAcceptsDefaults(t *testing.T) {
	cfg := Config{
		Addr:        ":8080",
		Environment: EnvironmentDevelopment,
		DatabaseURL: "postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to validate: %v", err)
	}
}

func TestConfigValidateRejectsInvalidEnvironment(t *testing.T) {
	cfg := Config{Addr: ":8080", Environment: "staging"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid environment to fail validation")
	}
}

func TestConfigValidateRejectsInvalidAddress(t *testing.T) {
	cfg := Config{Addr: "8080", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid address to fail validation")
	}
}

func TestConfigValidateRejectsNonNumericPort(t *testing.T) {
	cfg := Config{Addr: ":abc", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-numeric port to fail validation")
	}
}

func TestConfigValidateRejectsOutOfRangePort(t *testing.T) {
	cfg := Config{Addr: ":70000", Environment: "development"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected out-of-range port to fail validation")
	}
}

func TestConfigValidateDoesNotResolveHostnames(t *testing.T) {
	cfg := Config{
		Addr:        "api.internal.invalid:8080",
		Environment: EnvironmentDevelopment,
		DatabaseURL: "postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected hostname syntax to validate without DNS lookup: %v", err)
	}
}

func TestConfigValidateRejectsLocalActorInProduction(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	cfg := Config{
		Addr:         ":8080",
		Environment:  EnvironmentProduction,
		LocalActorID: &actorID,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production local actor fallback to fail validation")
	}
}

func TestConfigValidateRequiresDatabaseOutsideTest(t *testing.T) {
	cfg := Config{
		Addr:        ":8080",
		Environment: EnvironmentDevelopment,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing database URL to fail outside test")
	}
}

func TestConfigValidateRejectsPartialMediaStorageConfiguration(t *testing.T) {
	cfg := Config{
		Addr:        ":8080",
		Environment: EnvironmentDevelopment,
		DatabaseURL: "postgres://example",
		MediaStorage: MediaStorageConfig{
			Endpoint: "http://localhost:8333",
			Bucket:   "synvideo",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected incomplete media storage configuration to fail validation")
	}
}

func TestConfigValidateRequiresCORSOriginsInProduction(t *testing.T) {
	cfg := Config{
		Addr:        ":8080",
		Environment: EnvironmentProduction,
		DatabaseURL: "postgres://example",
		MediaStorage: MediaStorageConfig{
			Endpoint:        "https://s3.amazonaws.com",
			Region:          "us-east-1",
			Bucket:          "synvideo",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			Timeout:         30 * time.Second,
			MaxUploadBytes:  1024,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config without CORS origins to fail validation")
	}
}

func TestConfigValidateAcceptsProductionContract(t *testing.T) {
	cfg := Config{
		Addr:               ":8080",
		Environment:        EnvironmentProduction,
		DatabaseURL:        "postgres://example",
		CORSAllowedOrigins: []string{"https://app.synvideo.example"},
		Auth: AuthConfig{
			OIDCIssuer:       "https://issuer.synvideo.example",
			OIDCAudience:     "synvideo-api",
			JWKSFetchTimeout: 5 * time.Second,
			JWKSCacheTTL:     5 * time.Minute,
		},
		MediaStorage: MediaStorageConfig{
			Endpoint:        "https://s3.amazonaws.com",
			Region:          "us-east-1",
			Bucket:          "synvideo",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			Timeout:         30 * time.Second,
			MaxUploadBytes:  1024,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected production contract to validate: %v", err)
	}
}

func TestConfigValidateRequiresAuthInProduction(t *testing.T) {
	cfg := Config{
		Addr:               ":8080",
		Environment:        EnvironmentProduction,
		DatabaseURL:        "postgres://example",
		CORSAllowedOrigins: []string{"https://app.synvideo.example"},
		MediaStorage: MediaStorageConfig{
			Endpoint:        "https://s3.amazonaws.com",
			Region:          "us-east-1",
			Bucket:          "synvideo",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			Timeout:         30 * time.Second,
			MaxUploadBytes:  1024,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config without auth to fail validation")
	}
}

func TestConfigValidateRejectsWildcardCORSOriginInProduction(t *testing.T) {
	cfg := Config{
		Addr:               ":8080",
		Environment:        EnvironmentProduction,
		DatabaseURL:        "postgres://example",
		CORSAllowedOrigins: []string{"*"},
		MediaStorage: MediaStorageConfig{
			Endpoint:        "https://s3.amazonaws.com",
			Region:          "us-east-1",
			Bucket:          "synvideo",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			Timeout:         30 * time.Second,
			MaxUploadBytes:  1024,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard CORS origin to fail production validation")
	}
}

func TestConfigValidateRequiresMediaStorageInProduction(t *testing.T) {
	cfg := Config{
		Addr:               ":8080",
		Environment:        EnvironmentProduction,
		DatabaseURL:        "postgres://example",
		CORSAllowedOrigins: []string{"https://app.synvideo.example"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config without media storage to fail validation")
	}
}

func TestConfigValidateAcceptsCompleteMediaStorageConfiguration(t *testing.T) {
	cfg := Config{
		Addr:        ":8080",
		Environment: EnvironmentDevelopment,
		DatabaseURL: "postgres://example",
		MediaStorage: MediaStorageConfig{
			Endpoint:        "http://localhost:8333",
			Region:          "local",
			Bucket:          "synvideo",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
			Timeout:         30 * time.Second,
			MaxUploadBytes:  1024,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected complete media storage configuration to validate: %v", err)
	}
}

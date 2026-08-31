package config

import (
	"testing"

	"github.com/google/uuid"
)

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

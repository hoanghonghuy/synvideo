package main

import (
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

func clearPaidGenerationEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		paidGenerationMaxRequestsEnv,
		paidGenerationWindowEnv,
		paidGenerationMaxInFlightEnv,
		paidGenerationLeaseDurationEnv,
	} {
		t.Setenv(name, "")
	}
}

func TestLoadPaidGenerationPolicyProductionFailsClosedWhenMissing(t *testing.T) {
	clearPaidGenerationEnv(t)

	_, enabled, err := loadPaidGenerationPolicy(config.EnvironmentProduction)
	if err == nil || !strings.Contains(err.Error(), "required in production") {
		t.Fatalf("expected production missing-config error, got %v", err)
	}
	if enabled {
		t.Fatal("missing production policy must not be enabled")
	}
}

func TestLoadPaidGenerationPolicyRejectsPartialConfiguration(t *testing.T) {
	clearPaidGenerationEnv(t)
	t.Setenv(paidGenerationMaxRequestsEnv, "20")

	_, enabled, err := loadPaidGenerationPolicy(config.EnvironmentDevelopment)
	if err == nil || !strings.Contains(err.Error(), "must set") {
		t.Fatalf("expected partial-config error, got %v", err)
	}
	if enabled {
		t.Fatal("partial policy must not be enabled")
	}
}

func TestLoadPaidGenerationPolicyParsesExplicitBounds(t *testing.T) {
	clearPaidGenerationEnv(t)
	t.Setenv(paidGenerationMaxRequestsEnv, "20")
	t.Setenv(paidGenerationWindowEnv, "1h")
	t.Setenv(paidGenerationMaxInFlightEnv, "3")
	t.Setenv(paidGenerationLeaseDurationEnv, "2m")

	policy, enabled, err := loadPaidGenerationPolicy(config.EnvironmentProduction)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if !enabled {
		t.Fatal("expected explicit policy to be enabled")
	}
	if policy.MaxRequests != 20 || policy.MaxInFlight != 3 || policy.Window != time.Hour || policy.LeaseDuration != 2*time.Minute {
		t.Fatalf("unexpected policy: %+v", policy)
	}
}

func TestLoadPaidGenerationPolicyRejectsNonPositiveValues(t *testing.T) {
	clearPaidGenerationEnv(t)
	t.Setenv(paidGenerationMaxRequestsEnv, "0")
	t.Setenv(paidGenerationWindowEnv, "1h")
	t.Setenv(paidGenerationMaxInFlightEnv, "3")
	t.Setenv(paidGenerationLeaseDurationEnv, "2m")

	_, _, err := loadPaidGenerationPolicy(config.EnvironmentDevelopment)
	if err == nil || !strings.Contains(err.Error(), paidGenerationMaxRequestsEnv) {
		t.Fatalf("expected positive integer validation error, got %v", err)
	}
}

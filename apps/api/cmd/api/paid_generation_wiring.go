package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
)

const (
	paidGenerationMaxRequestsEnv   = "SYNVIDEO_PAID_GENERATION_MAX_REQUESTS"
	paidGenerationWindowEnv        = "SYNVIDEO_PAID_GENERATION_WINDOW"
	paidGenerationMaxInFlightEnv   = "SYNVIDEO_PAID_GENERATION_MAX_IN_FLIGHT"
	paidGenerationLeaseDurationEnv = "SYNVIDEO_PAID_GENERATION_LEASE_DURATION"
)

type paidGenerationWiring struct {
	Guard   paidgeneration.Guard
	Policy  paidgeneration.Policy
	Enabled bool
}

// loadPaidGenerationWiring converts explicit environment configuration into
// the shared PostgreSQL guard used by all cost-bearing provider handlers.
// Production intentionally has no implicit/unlimited fallback: missing or
// partial configuration is a startup error, so paid generation fails closed.
func loadPaidGenerationWiring(environment string, pool *pgxpool.Pool) (paidGenerationWiring, error) {
	policy, enabled, err := loadPaidGenerationPolicy(environment)
	if err != nil {
		return paidGenerationWiring{}, err
	}
	if !enabled {
		return paidGenerationWiring{}, nil
	}
	if pool == nil {
		return paidGenerationWiring{}, errors.New("paid generation guard requires PostgreSQL")
	}
	return paidGenerationWiring{
		Guard:   postgres.NewPaidGenerationGuard(pool),
		Policy:  policy,
		Enabled: true,
	}, nil
}

func loadPaidGenerationPolicy(environment string) (paidgeneration.Policy, bool, error) {
	raw := map[string]string{
		paidGenerationMaxRequestsEnv:   strings.TrimSpace(os.Getenv(paidGenerationMaxRequestsEnv)),
		paidGenerationWindowEnv:        strings.TrimSpace(os.Getenv(paidGenerationWindowEnv)),
		paidGenerationMaxInFlightEnv:   strings.TrimSpace(os.Getenv(paidGenerationMaxInFlightEnv)),
		paidGenerationLeaseDurationEnv: strings.TrimSpace(os.Getenv(paidGenerationLeaseDurationEnv)),
	}

	setCount := 0
	for _, value := range raw {
		if value != "" {
			setCount++
		}
	}
	if setCount == 0 {
		if environment == config.EnvironmentProduction {
			return paidgeneration.Policy{}, false, errors.New("paid generation guard configuration is required in production")
		}
		return paidgeneration.Policy{}, false, nil
	}
	if setCount != len(raw) {
		return paidgeneration.Policy{}, false, errors.New("paid generation guard configuration must set max requests, window, max in-flight, and lease duration together")
	}

	maxRequests, err := parsePositiveIntEnv(paidGenerationMaxRequestsEnv, raw[paidGenerationMaxRequestsEnv])
	if err != nil {
		return paidgeneration.Policy{}, false, err
	}
	maxInFlight, err := parsePositiveIntEnv(paidGenerationMaxInFlightEnv, raw[paidGenerationMaxInFlightEnv])
	if err != nil {
		return paidgeneration.Policy{}, false, err
	}
	window, err := parsePositiveDurationEnv(paidGenerationWindowEnv, raw[paidGenerationWindowEnv])
	if err != nil {
		return paidgeneration.Policy{}, false, err
	}
	leaseDuration, err := parsePositiveDurationEnv(paidGenerationLeaseDurationEnv, raw[paidGenerationLeaseDurationEnv])
	if err != nil {
		return paidgeneration.Policy{}, false, err
	}

	policy := paidgeneration.Policy{
		MaxRequests:   maxRequests,
		Window:        window,
		MaxInFlight:   maxInFlight,
		LeaseDuration: leaseDuration,
	}
	if !policy.Valid() {
		return paidgeneration.Policy{}, false, errors.New("paid generation guard policy is invalid")
	}
	return policy, true, nil
}

func parsePositiveIntEnv(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parsePositiveDurationEnv(name, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}

package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultAddr        = ":8080"
	defaultEnvironment = "development"

	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentProduction  = "production"
)

var allowedEnvironments = map[string]struct{}{
	EnvironmentDevelopment: {},
	EnvironmentTest:        {},
	EnvironmentProduction:  {},
}

type Config struct {
	Addr         string
	Environment  string
	DatabaseURL  string
	LocalActorID *uuid.UUID
}

func Load() (Config, error) {
	localActorID, err := parseOptionalUUID("SYNVIDEO_LOCAL_ACTOR_ID")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:         getEnv("SYNVIDEO_API_ADDR", defaultAddr),
		Environment:  getEnv("SYNVIDEO_ENV", defaultEnvironment),
		DatabaseURL:  getEnv("SYNVIDEO_DATABASE_URL", ""),
		LocalActorID: localActorID,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if _, ok := allowedEnvironments[c.Environment]; !ok {
		return fmt.Errorf("SYNVIDEO_ENV must be one of development, test, production: %q", c.Environment)
	}
	if c.Environment == EnvironmentProduction && c.LocalActorID != nil {
		return errors.New("SYNVIDEO_LOCAL_ACTOR_ID must not be set in production")
	}

	if c.Addr == "" {
		return errors.New("SYNVIDEO_API_ADDR is required")
	}

	host, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return fmt.Errorf("SYNVIDEO_API_ADDR must be a host:port or :port address: %w", err)
	}
	if port == "" {
		return errors.New("SYNVIDEO_API_ADDR must include a port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("SYNVIDEO_API_ADDR port must be numeric: %w", err)
	}
	if portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("SYNVIDEO_API_ADDR port must be between 1 and 65535: %d", portNumber)
	}
	if host != "" {
		if strings.ContainsAny(host, " \t\r\n") {
			return fmt.Errorf("SYNVIDEO_API_ADDR host must not contain whitespace: %q", host)
		}
	}
	if c.Environment != EnvironmentTest && strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("SYNVIDEO_DATABASE_URL is required")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func parseOptionalUUID(key string) (*uuid.UUID, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be a UUID: %w", key, err)
	}
	if id == uuid.Nil {
		return nil, fmt.Errorf("%s must not be the nil UUID", key)
	}
	return &id, nil
}

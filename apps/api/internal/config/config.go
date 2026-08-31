package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultAddr        = ":8080"
	defaultEnvironment = "development"
)

var allowedEnvironments = map[string]struct{}{
	"development": {},
	"test":        {},
	"production":  {},
}

type Config struct {
	Addr        string
	Environment string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:        getEnv("SYNVIDEO_API_ADDR", defaultAddr),
		Environment: getEnv("SYNVIDEO_ENV", defaultEnvironment),
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

	return nil
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

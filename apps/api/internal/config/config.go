package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	Addr                    string
	Environment             string
	DatabaseURL             string
	LocalActorID            *uuid.UUID
	CredentialEncryptionKey string
	CredentialKeyVersion    string
	TextProviderDefinitions string
	MediaStorage            MediaStorageConfig
}

const (
	defaultMediaStorageTimeout = 30 * time.Second
	DefaultMaxUploadBytes      = 100 * 1024 * 1024
)

// MediaStorageConfig contains the application-owned storage settings. Secret
// values are only consumed while constructing the private storage adapter.
type MediaStorageConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	Timeout         time.Duration
	MaxUploadBytes  int64
}

func (c MediaStorageConfig) Configured() bool {
	return strings.TrimSpace(c.Endpoint) != "" || strings.TrimSpace(c.Region) != "" ||
		strings.TrimSpace(c.Bucket) != "" || strings.TrimSpace(c.AccessKeyID) != "" ||
		c.SecretAccessKey != "" || c.UsePathStyle || c.Timeout != 0 || c.MaxUploadBytes != 0
}

func (c MediaStorageConfig) Validate() error {
	if !c.Configured() {
		return nil
	}
	endpoint := strings.TrimSpace(c.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return errors.New("media storage endpoint must be an http(s) URL without path, query, fragment or credentials")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return errors.New("media storage bucket is required")
	}
	if strings.TrimSpace(c.AccessKeyID) == "" || c.SecretAccessKey == "" {
		return errors.New("media storage credentials are required")
	}
	if c.Timeout <= 0 {
		return errors.New("media storage timeout must be positive")
	}
	if c.MaxUploadBytes <= 0 {
		return errors.New("media storage max upload bytes must be positive")
	}
	return nil
}

func Load() (Config, error) {
	localActorID, err := parseOptionalUUID("SYNVIDEO_LOCAL_ACTOR_ID")
	if err != nil {
		return Config{}, err
	}

	byokKey := getEnv("SYNVIDEO_CREDENTIAL_ENCRYPTION_KEY", "")
	if byokKey == "" {
		byokKey = getEnv("SYNVIDEO_BYOK_ENCRYPTION_KEY", "")
	}

	mediaStorage, err := loadMediaStorageConfig()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:                    getEnv("SYNVIDEO_API_ADDR", defaultAddr),
		Environment:             getEnv("SYNVIDEO_ENV", defaultEnvironment),
		DatabaseURL:             getEnv("SYNVIDEO_DATABASE_URL", ""),
		LocalActorID:            localActorID,
		CredentialEncryptionKey: byokKey,
		CredentialKeyVersion:    getEnv("SYNVIDEO_CREDENTIAL_KEY_VERSION", "v1"),
		TextProviderDefinitions: getEnv("SYNVIDEO_TEXT_PROVIDER_DEFINITIONS", ""),
		MediaStorage:            mediaStorage,
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
	if err := c.MediaStorage.Validate(); err != nil {
		return err
	}

	return nil
}

func loadMediaStorageConfig() (MediaStorageConfig, error) {
	endpoint := getEnvAlias("SYNVIDEO_MEDIA_STORAGE_ENDPOINT", "SYNVIDEO_S3_ENDPOINT", "")
	region := getEnvAlias("SYNVIDEO_MEDIA_STORAGE_REGION", "SYNVIDEO_S3_REGION", "")
	bucket := getEnvAlias("SYNVIDEO_MEDIA_STORAGE_BUCKET", "SYNVIDEO_S3_BUCKET", "")
	accessKeyID := getEnvAlias("SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID", "SYNVIDEO_S3_ACCESS_KEY_ID", "")
	secretAccessKey := getEnvAlias("SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY", "SYNVIDEO_S3_SECRET_ACCESS_KEY", "")
	pathStyleRaw, pathStyleSet := os.LookupEnv("SYNVIDEO_MEDIA_STORAGE_PATH_STYLE")
	if !pathStyleSet {
		pathStyleRaw, pathStyleSet = os.LookupEnv("SYNVIDEO_S3_PATH_STYLE")
	}
	configured := endpoint != "" || region != "" || bucket != "" || accessKeyID != "" || secretAccessKey != "" || pathStyleSet ||
		os.Getenv("SYNVIDEO_MEDIA_STORAGE_TIMEOUT") != "" || os.Getenv("SYNVIDEO_MEDIA_MAX_UPLOAD_BYTES") != ""
	if !configured {
		return MediaStorageConfig{}, nil
	}

	result := MediaStorageConfig{
		Endpoint:        endpoint,
		Region:          region,
		Bucket:          bucket,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Timeout:         defaultMediaStorageTimeout,
		MaxUploadBytes:  DefaultMaxUploadBytes,
	}
	if pathStyleSet {
		value, err := strconv.ParseBool(strings.TrimSpace(pathStyleRaw))
		if err != nil {
			return MediaStorageConfig{}, errors.New("SYNVIDEO_MEDIA_STORAGE_PATH_STYLE must be boolean")
		}
		result.UsePathStyle = value
	}
	if raw := strings.TrimSpace(os.Getenv("SYNVIDEO_MEDIA_STORAGE_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return MediaStorageConfig{}, errors.New("SYNVIDEO_MEDIA_STORAGE_TIMEOUT must be a positive duration")
		}
		result.Timeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("SYNVIDEO_MEDIA_MAX_UPLOAD_BYTES")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return MediaStorageConfig{}, errors.New("SYNVIDEO_MEDIA_MAX_UPLOAD_BYTES must be a positive integer")
		}
		result.MaxUploadBytes = value
	}
	return result, nil
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func getEnvAlias(primary, legacy, fallback string) string {
	if value, ok := os.LookupEnv(primary); ok {
		return value
	}
	return getEnv(legacy, fallback)
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

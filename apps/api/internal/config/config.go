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

type AuthConfig struct {
	OIDCIssuer       string
	OIDCAudience     string
	JWKSURL          string
	JWKSFetchTimeout time.Duration
	JWKSCacheTTL     time.Duration
}

func (c AuthConfig) Configured() bool {
	return strings.TrimSpace(c.OIDCIssuer) != "" && strings.TrimSpace(c.OIDCAudience) != ""
}

func (c AuthConfig) Validate() error {
	if !c.Configured() {
		return errors.New("oidc issuer and audience are required")
	}
	issuer := strings.TrimRight(strings.TrimSpace(c.OIDCIssuer), "/")
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("SYNVIDEO_OIDC_ISSUER must be an origin URL without path: %q", c.OIDCIssuer)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("SYNVIDEO_OIDC_ISSUER must use https in production: %q", c.OIDCIssuer)
	}
	if parsed.User != nil {
		return fmt.Errorf("SYNVIDEO_OIDC_ISSUER must not include credentials: %q", c.OIDCIssuer)
	}
	if strings.TrimSpace(c.OIDCAudience) == "" {
		return errors.New("SYNVIDEO_OIDC_AUDIENCE is required")
	}
	if c.JWKSFetchTimeout <= 0 {
		return errors.New("SYNVIDEO_OIDC_JWKS_FETCH_TIMEOUT must be positive")
	}
	if c.JWKSCacheTTL <= 0 {
		return errors.New("SYNVIDEO_OIDC_JWKS_CACHE_TTL must be positive")
	}
	if jwks := strings.TrimSpace(c.JWKSURL); jwks != "" {
		parsedJWKS, err := url.Parse(jwks)
		if err != nil || parsedJWKS.Scheme == "" || parsedJWKS.Host == "" {
			return fmt.Errorf("SYNVIDEO_OIDC_JWKS_URL must be an absolute https URL: %q", c.JWKSURL)
		}
		if parsedJWKS.Scheme != "https" {
			return fmt.Errorf("SYNVIDEO_OIDC_JWKS_URL must use https in production: %q", c.JWKSURL)
		}
		if parsedJWKS.User != nil || parsedJWKS.Fragment != "" {
			return fmt.Errorf("SYNVIDEO_OIDC_JWKS_URL must not include credentials or fragment: %q", c.JWKSURL)
		}
	}
	return nil
}

type Config struct {
	Addr                    string
	Environment             string
	DatabaseURL             string
	LocalActorID            *uuid.UUID
	Auth                    AuthConfig
	CredentialEncryptionKey string
	CredentialKeyVersion    string
	TextProviderDefinitions string // Deprecated: use ProviderDefinitions
	ProviderDefinitions     string
	CORSAllowedOrigins      []string
	MediaStorage            MediaStorageConfig
}

const (
	defaultMediaStorageTimeout = 30 * time.Second
	defaultJWKSFetchTimeout    = 5 * time.Second
	defaultJWKSCacheTTL        = 5 * time.Minute
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

	authCfg, err := loadAuthConfig()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:                    resolveListenAddr(),
		Environment:             getEnv("SYNVIDEO_ENV", defaultEnvironment),
		DatabaseURL:             getEnv("SYNVIDEO_DATABASE_URL", ""),
		LocalActorID:            localActorID,
		Auth:                    authCfg,
		CredentialEncryptionKey: byokKey,
		CredentialKeyVersion:    getEnv("SYNVIDEO_CREDENTIAL_KEY_VERSION", "v1"),
		TextProviderDefinitions: getEnv("SYNVIDEO_TEXT_PROVIDER_DEFINITIONS", ""),
		ProviderDefinitions:     getEnv("SYNVIDEO_PROVIDER_DEFINITIONS", ""),
		CORSAllowedOrigins:      parseCSVEnv("SYNVIDEO_CORS_ALLOWED_ORIGINS"),
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
	if c.Environment == EnvironmentProduction {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("production auth configuration invalid: %w", err)
		}
		if len(c.CORSAllowedOrigins) == 0 {
			return errors.New("SYNVIDEO_CORS_ALLOWED_ORIGINS is required in production")
		}
		for _, origin := range c.CORSAllowedOrigins {
			if strings.TrimSpace(origin) == "*" {
				return errors.New("SYNVIDEO_CORS_ALLOWED_ORIGINS must not contain wildcard in production")
			}
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				return fmt.Errorf("SYNVIDEO_CORS_ALLOWED_ORIGINS entry must be an origin URL without path: %q", origin)
			}
		}
		if !c.MediaStorage.Configured() {
			return errors.New("media storage configuration is required in production")
		}
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

func resolveListenAddr() string {
	if value, ok := os.LookupEnv("SYNVIDEO_API_ADDR"); ok {
		return value
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return ":" + port
	}
	return defaultAddr
}

func parseCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			origins = append(origins, part)
		}
	}
	return origins
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

func loadAuthConfig() (AuthConfig, error) {
	result := AuthConfig{
		OIDCIssuer:       getEnv("SYNVIDEO_OIDC_ISSUER", ""),
		OIDCAudience:     getEnv("SYNVIDEO_OIDC_AUDIENCE", ""),
		JWKSURL:          getEnv("SYNVIDEO_OIDC_JWKS_URL", ""),
		JWKSFetchTimeout: defaultJWKSFetchTimeout,
		JWKSCacheTTL:     defaultJWKSCacheTTL,
	}
	if raw := strings.TrimSpace(os.Getenv("SYNVIDEO_OIDC_JWKS_FETCH_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return AuthConfig{}, errors.New("SYNVIDEO_OIDC_JWKS_FETCH_TIMEOUT must be a positive duration")
		}
		result.JWKSFetchTimeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("SYNVIDEO_OIDC_JWKS_CACHE_TTL")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return AuthConfig{}, errors.New("SYNVIDEO_OIDC_JWKS_CACHE_TTL must be a positive duration")
		}
		result.JWKSCacheTTL = value
	}
	return result, nil
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

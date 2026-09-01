package openaicompat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = 1 << 20
)

var (
	ErrInvalidConfiguration  = errors.New("invalid OpenAI-compatible provider configuration")
	errCredentialUnavailable = errors.New("credential source unavailable")
)

// SecretSource supplies a credential only at request time.
type SecretSource interface {
	APIKey(ctx context.Context) (string, error)
}

// SecretSourceFunc adapts a function into a SecretSource.
type SecretSourceFunc func(ctx context.Context) (string, error)

func (f SecretSourceFunc) APIKey(ctx context.Context) (string, error) {
	if f == nil {
		return "", errCredentialUnavailable
	}
	return f(ctx)
}

// ModelConfig maps a stable internal model ID to an upstream model name.
type ModelConfig struct {
	ID              providers.ModelID
	DisplayName     string
	ExternalModelID string
}

// Config contains provider-boundary configuration and injected secret access.
type Config struct {
	ProviderID       providers.ProviderID
	DisplayName      string
	BaseURL          string
	CredentialSource SecretSource
	Models           []ModelConfig
	Timeout          time.Duration
	MaxResponseBytes int64
	HTTPClient       *http.Client
}

type modelConfig struct {
	metadata      providers.ModelMetadata
	externalModel string
}

// Adapter implements the provider-neutral text generation boundary.
type Adapter struct {
	providerID       providers.ProviderID
	endpoint         string
	credentialSource SecretSource
	models           map[providers.ModelID]modelConfig
	client           *http.Client
	maxResponseBytes int64
}

// New validates configuration and creates an isolated HTTP adapter.
func New(config Config) (*Adapter, error) {
	if !config.ProviderID.Valid() {
		return nil, fmt.Errorf("%w: provider id is invalid", ErrInvalidConfiguration)
	}
	if strings.TrimSpace(config.DisplayName) == "" {
		return nil, fmt.Errorf("%w: provider display name is required", ErrInvalidConfiguration)
	}
	if config.CredentialSource == nil {
		return nil, fmt.Errorf("%w: credential source is required", ErrInvalidConfiguration)
	}

	endpoint, err := chatCompletionsEndpoint(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	timeout := config.Timeout
	if timeout < 0 {
		return nil, fmt.Errorf("%w: timeout cannot be negative", ErrInvalidConfiguration)
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes < 0 {
		return nil, fmt.Errorf("%w: maximum response size cannot be negative", ErrInvalidConfiguration)
	}
	if maxResponseBytes == 0 {
		maxResponseBytes = DefaultMaxResponseBytes
	}

	models := make(map[providers.ModelID]modelConfig, len(config.Models))
	for _, configured := range config.Models {
		if !configured.ID.Valid() {
			return nil, fmt.Errorf("%w: model id is invalid", ErrInvalidConfiguration)
		}
		if strings.TrimSpace(configured.DisplayName) == "" {
			return nil, fmt.Errorf("%w: model display name is required", ErrInvalidConfiguration)
		}
		if strings.TrimSpace(configured.ExternalModelID) == "" || configured.ExternalModelID != strings.TrimSpace(configured.ExternalModelID) {
			return nil, fmt.Errorf("%w: external model id is invalid", ErrInvalidConfiguration)
		}
		if _, exists := models[configured.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate model id %q", ErrInvalidConfiguration, configured.ID)
		}
		models[configured.ID] = modelConfig{
			metadata: providers.ModelMetadata{
				ProviderID:            config.ProviderID,
				ID:                    configured.ID,
				DisplayName:           configured.DisplayName,
				SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
			},
			externalModel: configured.ExternalModelID,
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%w: at least one model is required", ErrInvalidConfiguration)
	}

	client := &http.Client{}
	if config.HTTPClient != nil {
		copied := *config.HTTPClient
		client = &copied
	}
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Adapter{
		providerID:       config.ProviderID,
		endpoint:         endpoint,
		credentialSource: config.CredentialSource,
		models:           models,
		client:           client,
		maxResponseBytes: maxResponseBytes,
	}, nil
}

// NewRegistration builds deterministic provider metadata and model bindings.
func NewRegistration(config Config) (providers.Registration, error) {
	adapter, err := New(config)
	if err != nil {
		return providers.Registration{}, err
	}

	models := make([]modelConfig, 0, len(adapter.models))
	for _, model := range adapter.models {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].metadata.ID < models[j].metadata.ID
	})

	registration := providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          adapter.providerID,
			DisplayName: config.DisplayName,
		},
		Models: make([]providers.ModelRegistration, len(models)),
	}
	for i, model := range models {
		registration.Models[i] = providers.ModelRegistration{
			Metadata:      model.metadata,
			TextGenerator: adapter,
		}
	}
	return registration, nil
}

func chatCompletionsEndpoint(rawBaseURL string) (string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)
	if baseURL == "" {
		return "", errors.New("base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base URL must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base URL scheme must be http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL cannot contain user info, query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}

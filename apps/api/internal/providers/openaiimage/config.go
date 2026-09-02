// Package openaiimage contains the isolated OpenAI Images HTTP adapter.
package openaiimage

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

const (
	DefaultBaseURL                = "https://api.openai.com/v1"
	DefaultTimeout                = 30 * time.Second
	DefaultMaxResponseBytes       = 1 << 20
	DefaultMaxDecodedImageBytes   = 20 << 20
	DefaultMaxAggregateImageBytes = 80 << 20
)

var (
	ErrInvalidConfiguration  = errors.New("invalid OpenAI image provider configuration")
	ErrUnsupportedParameter  = errors.New("unsupported image generation parameter")
	errCredentialUnavailable = errors.New("credential source unavailable")
)

// SecretSource supplies a credential only when a request is made.
type SecretSource interface {
	APIKey(context.Context) (string, error)
}

// SecretSourceFunc adapts a function into a SecretSource.
type SecretSourceFunc func(context.Context) (string, error)

func (f SecretSourceFunc) APIKey(ctx context.Context) (string, error) {
	if f == nil {
		return "", errCredentialUnavailable
	}
	return f(ctx)
}

// ModelConfig maps a stable internal model identity to an injected OpenAI model name.
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

	// MaxDecodedImageBytes bounds each decoded output.
	MaxDecodedImageBytes int64
	// MaxAggregateImageBytes bounds the sum of all decoded outputs in one response.
	MaxAggregateImageBytes int64
	// MaxOutputCount bounds the number of outputs accepted and requested.
	MaxOutputCount int
	HTTPClient     *http.Client
}

type modelConfig struct {
	metadata      providers.ModelMetadata
	externalModel string
}

// Adapter owns shared HTTP/configuration state. A generator is bound to one model
// through ForModel because the provider-neutral image request has no model field.
type Adapter struct {
	providerID             providers.ProviderID
	endpoint               string
	credentialSource       SecretSource
	models                 map[providers.ModelID]modelConfig
	client                 *http.Client
	maxResponseBytes       int64
	maxDecodedImageBytes   int64
	maxAggregateImageBytes int64
	maxOutputCount         int
}

// New validates configuration and creates an isolated adapter.
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

	endpoint, err := imagesEndpoint(config.BaseURL)
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
	maxDecodedImageBytes := config.MaxDecodedImageBytes
	if maxDecodedImageBytes < 0 {
		return nil, fmt.Errorf("%w: maximum decoded image size cannot be negative", ErrInvalidConfiguration)
	}
	if maxDecodedImageBytes == 0 {
		maxDecodedImageBytes = DefaultMaxDecodedImageBytes
	}
	maxAggregateImageBytes := config.MaxAggregateImageBytes
	if maxAggregateImageBytes < 0 {
		return nil, fmt.Errorf("%w: maximum aggregate image size cannot be negative", ErrInvalidConfiguration)
	}
	if maxAggregateImageBytes == 0 {
		maxAggregateImageBytes = DefaultMaxAggregateImageBytes
	}
	maxOutputCount := config.MaxOutputCount
	if maxOutputCount < 0 || maxOutputCount > providers.MaxImageOutputs {
		return nil, fmt.Errorf("%w: maximum output count is invalid", ErrInvalidConfiguration)
	}
	if maxOutputCount == 0 {
		maxOutputCount = providers.MaxImageOutputs
	}

	models := make(map[providers.ModelID]modelConfig, len(config.Models))
	for _, configured := range config.Models {
		if !configured.ID.Valid() {
			return nil, fmt.Errorf("%w: model id is invalid", ErrInvalidConfiguration)
		}
		if strings.TrimSpace(configured.DisplayName) == "" {
			return nil, fmt.Errorf("%w: model display name is required", ErrInvalidConfiguration)
		}
		if !validExternalModelID(configured.ExternalModelID) {
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
				SupportedCapabilities: []providers.Capability{providers.CapabilityImageGeneration},
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
	// Image generation returns authenticated response bytes. Do not follow a
	// redirect where credentials could be sent to an unvalidated origin.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Adapter{
		providerID:             config.ProviderID,
		endpoint:               endpoint,
		credentialSource:       config.CredentialSource,
		models:                 models,
		client:                 client,
		maxResponseBytes:       maxResponseBytes,
		maxDecodedImageBytes:   maxDecodedImageBytes,
		maxAggregateImageBytes: maxAggregateImageBytes,
		maxOutputCount:         maxOutputCount,
	}, nil
}

// ForModel returns an image generator with one configured external model bound.
func (a *Adapter) ForModel(modelID providers.ModelID) (providers.ImageGenerator, error) {
	model, ok := a.models[modelID]
	if !ok {
		return nil, providers.NewUnknownModelError(a.providerID, modelID)
	}
	return &modelGenerator{adapter: a, model: model}, nil
}

// NewImageGenerator validates config and binds one configured model in one call.
func NewImageGenerator(config Config, modelID providers.ModelID) (providers.ImageGenerator, error) {
	adapter, err := New(config)
	if err != nil {
		return nil, err
	}
	return adapter.ForModel(modelID)
}

// NewRegistration builds deterministic provider metadata and image-only bindings.
func NewRegistration(config Config) (providers.Registration, error) {
	adapter, err := New(config)
	if err != nil {
		return providers.Registration{}, err
	}

	models := make([]modelConfig, 0, len(adapter.models))
	for _, model := range adapter.models {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].metadata.ID < models[j].metadata.ID })

	registration := providers.Registration{
		Provider: providers.ProviderMetadata{ID: adapter.providerID, DisplayName: config.DisplayName},
		Models:   make([]providers.ModelRegistration, len(models)),
	}
	for i, model := range models {
		generator, err := adapter.ForModel(model.metadata.ID)
		if err != nil {
			return providers.Registration{}, err
		}
		registration.Models[i] = providers.ModelRegistration{
			Metadata:       model.metadata,
			ImageGenerator: generator,
		}
	}
	return registration, nil
}

// ValidateBaseURL validates an OpenAI API base URL without exposing credentials.
func ValidateBaseURL(rawBaseURL string) error {
	_, err := imagesEndpoint(rawBaseURL)
	return err
}

func imagesEndpoint(rawBaseURL string) (string, error) {
	if rawBaseURL == "" {
		rawBaseURL = DefaultBaseURL
	}
	if rawBaseURL != strings.TrimSpace(rawBaseURL) {
		return "", errors.New("base URL must not have surrounding whitespace")
	}
	parsed, err := url.Parse(rawBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return "", errors.New("base URL must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base URL scheme must be http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL cannot contain user info, query or fragment")
	}
	if hasUnsafePath(parsed.Path) {
		return "", errors.New("base URL path contains an unsafe segment")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", errors.New("base URL host is required")
	}
	if parsed.Scheme == "http" && !isLocalOrTestHost(host) {
		return "", errors.New("https is required for non-local base URLs")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/images/generations"
	parsed.RawPath = ""
	return parsed.String(), nil
}

func validExternalModelID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func hasUnsafePath(path string) bool {
	unescaped, err := url.PathUnescape(path)
	if err != nil {
		return true
	}
	for _, segment := range strings.Split(unescaped, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

func isLocalOrTestHost(host string) bool {
	if host == "localhost" || host == "test" || strings.HasSuffix(host, ".test") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

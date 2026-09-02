// Package openaitts contains the isolated OpenAI speech HTTP adapter.
package openaitts

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
	DefaultBaseURL          = "https://api.openai.com/v1"
	DefaultTimeout          = 30 * time.Second
	DefaultMaxInputRunes    = 2000
	DefaultMaxInputBytes    = 8000
	DefaultMaxResponseBytes = 64 << 20
)

var (
	ErrInvalidConfiguration  = errors.New("invalid OpenAI speech provider configuration")
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

// VoiceConfig maps a stable internal voice identity to an injected OpenAI voice name.
type VoiceConfig struct {
	ID            providers.VoiceID
	DisplayName   string
	ExternalVoice string
	Locale        string
	Language      string
	Style         string
}

// Config contains provider-boundary configuration and injected secret access.
type Config struct {
	ProviderID       providers.ProviderID
	DisplayName      string
	BaseURL          string
	CredentialSource SecretSource
	Models           []ModelConfig
	Voices           []VoiceConfig
	Timeout          time.Duration
	MaxInputRunes    int
	MaxInputBytes    int64
	MaxResponseBytes int64
	SupportedFormats []providers.AudioFormat
	HTTPClient       *http.Client
}

type modelConfig struct {
	metadata      providers.ModelMetadata
	externalModel string
}

type voiceConfig struct {
	metadata      providers.VoiceMetadata
	externalVoice string
}

// Adapter owns shared HTTP/configuration state. A synthesizer is bound to one
// configured model through ForModel because the speech port has no model field.
type Adapter struct {
	providerID       providers.ProviderID
	endpoint         string
	credentialSource SecretSource
	models           map[providers.ModelID]modelConfig
	voices           map[providers.VoiceID]voiceConfig
	defaultFormat    providers.AudioFormat
	formats          map[providers.AudioFormat]struct{}
	client           *http.Client
	maxInputRunes    int
	maxInputBytes    int64
	maxResponseBytes int64
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
	endpoint, err := speechEndpoint(config.BaseURL)
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
	maxInputRunes := config.MaxInputRunes
	if maxInputRunes < 0 {
		return nil, fmt.Errorf("%w: maximum input runes cannot be negative", ErrInvalidConfiguration)
	}
	if maxInputRunes == 0 {
		maxInputRunes = DefaultMaxInputRunes
	}
	maxInputBytes := config.MaxInputBytes
	if maxInputBytes < 0 {
		return nil, fmt.Errorf("%w: maximum input bytes cannot be negative", ErrInvalidConfiguration)
	}
	if maxInputBytes == 0 {
		maxInputBytes = DefaultMaxInputBytes
	}
	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes < 0 {
		return nil, fmt.Errorf("%w: maximum response size cannot be negative", ErrInvalidConfiguration)
	}
	if maxResponseBytes == 0 {
		maxResponseBytes = DefaultMaxResponseBytes
	}

	formats := make(map[providers.AudioFormat]struct{})
	configuredFormats := config.SupportedFormats
	if len(configuredFormats) == 0 {
		configuredFormats = []providers.AudioFormat{providers.AudioFormatMP3, providers.AudioFormatWAV}
	}
	defaultFormat := configuredFormats[0]
	for _, format := range configuredFormats {
		if !format.Valid() {
			return nil, fmt.Errorf("%w: unsupported output format", ErrInvalidConfiguration)
		}
		if _, exists := formats[format]; exists {
			return nil, fmt.Errorf("%w: duplicate output format", ErrInvalidConfiguration)
		}
		formats[format] = struct{}{}
	}

	models := make(map[providers.ModelID]modelConfig, len(config.Models))
	for _, configured := range config.Models {
		if !configured.ID.Valid() {
			return nil, fmt.Errorf("%w: model id is invalid", ErrInvalidConfiguration)
		}
		if strings.TrimSpace(configured.DisplayName) == "" {
			return nil, fmt.Errorf("%w: model display name is required", ErrInvalidConfiguration)
		}
		if !validExternalName(configured.ExternalModelID) {
			return nil, fmt.Errorf("%w: external model id is invalid", ErrInvalidConfiguration)
		}
		if _, exists := models[configured.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate model id", ErrInvalidConfiguration)
		}
		models[configured.ID] = modelConfig{
			metadata: providers.ModelMetadata{
				ProviderID: config.ProviderID, ID: configured.ID, DisplayName: configured.DisplayName,
				SupportedCapabilities: []providers.Capability{providers.CapabilityTTS},
			},
			externalModel: configured.ExternalModelID,
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%w: at least one model is required", ErrInvalidConfiguration)
	}

	voices := make(map[providers.VoiceID]voiceConfig, len(config.Voices))
	for _, configured := range config.Voices {
		metadata := providers.VoiceMetadata{
			ID: configured.ID, DisplayName: configured.DisplayName, Locale: configured.Locale,
			Language: configured.Language, Style: configured.Style,
		}
		if err := metadata.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
		}
		if !validExternalName(configured.ExternalVoice) {
			return nil, fmt.Errorf("%w: external voice is invalid", ErrInvalidConfiguration)
		}
		if _, exists := voices[configured.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate voice id", ErrInvalidConfiguration)
		}
		voices[configured.ID] = voiceConfig{metadata: metadata, externalVoice: configured.ExternalVoice}
	}
	if len(voices) == 0 {
		return nil, fmt.Errorf("%w: at least one voice is required", ErrInvalidConfiguration)
	}

	client := &http.Client{}
	if config.HTTPClient != nil {
		copied := *config.HTTPClient
		client = &copied
	}
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	return &Adapter{
		providerID: config.ProviderID, endpoint: endpoint, credentialSource: config.CredentialSource,
		models: models, voices: voices, defaultFormat: defaultFormat, formats: formats, client: client,
		maxInputRunes: maxInputRunes, maxInputBytes: maxInputBytes, maxResponseBytes: maxResponseBytes,
	}, nil
}

// ForModel returns a speech synthesizer bound to one configured external model.
func (a *Adapter) ForModel(modelID providers.ModelID) (providers.SpeechSynthesizer, error) {
	model, ok := a.models[modelID]
	if !ok {
		return nil, providers.NewUnknownModelError(a.providerID, modelID)
	}
	return &modelSynthesizer{adapter: a, model: model}, nil
}

// NewSpeechSynthesizer validates config and binds one configured model.
func NewSpeechSynthesizer(config Config, modelID providers.ModelID) (providers.SpeechSynthesizer, error) {
	adapter, err := New(config)
	if err != nil {
		return nil, err
	}
	return adapter.ForModel(modelID)
}

// NewRegistration builds deterministic provider metadata and TTS bindings.
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
		synthesizer, err := adapter.ForModel(model.metadata.ID)
		if err != nil {
			return providers.Registration{}, err
		}
		registration.Models[i] = providers.ModelRegistration{Metadata: model.metadata, SpeechSynthesizer: synthesizer}
	}
	return registration, nil
}

func ValidateBaseURL(rawBaseURL string) error {
	_, err := speechEndpoint(rawBaseURL)
	return err
}

func speechEndpoint(rawBaseURL string) (string, error) {
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
	if parsed.Scheme == "http" && !isLoopbackHost(host) {
		return "", errors.New("https is required for non-local base URLs")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/audio/speech"
	parsed.RawPath = ""
	return parsed.String(), nil
}

func validExternalName(value string) bool {
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

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

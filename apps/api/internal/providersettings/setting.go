package providersettings

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	ErrSettingNotFound     = errors.New("provider setting not found")
	ErrStaleRevision       = errors.New("stale provider setting revision")
	ErrInvalidSettingInput = errors.New("invalid provider setting input")
	ErrDecryptionFailed    = errors.New("credential decryption failed")
	ErrEncryptionFailed    = errors.New("credential encryption failed")
	ErrMasterKeyMissing    = errors.New("credential encryption master key missing or invalid")
	ErrProviderNotFound    = errors.New("provider not found in catalog")
	ErrModelNotFound       = errors.New("model not found in catalog")
	ErrCredentialRequired  = errors.New("api key is required for initial configuration")
	ErrProviderDisabled    = errors.New("provider is disabled")
	ErrModelNotEnabled     = errors.New("model is not enabled for provider")
	ErrUnauthenticated     = errors.New("unauthenticated owner")
)

const (
	MaxAPIKeyLength = 8192
)

// Setting represents an owner's persisted settings and encrypted credential for a provider.
type Setting struct {
	OwnerID              uuid.UUID
	ProviderID           providers.ProviderID
	Revision             int
	Enabled              bool
	EnabledTextModelIDs  []providers.ModelID
	EnabledImageModelIDs []providers.ModelID
	EnabledTTSModelIDs   []providers.ModelID
	EnabledVoiceIDs      []providers.VoiceID
	APIKeyCiphertext     []byte
	APIKeyNonce          []byte
	KeyVersion           string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ProviderSettingView is the safe, non-secret view of a provider's settings.
type ProviderSettingView struct {
	ID          providers.ProviderID `json:"id"`
	DisplayName string               `json:"display_name"`
	Configured  bool                 `json:"configured"`
	Enabled     bool                 `json:"enabled"`
	HasAPIKey   bool                 `json:"has_api_key"`
	Revision    int                  `json:"revision"`
	Models      []ModelSettingView   `json:"models"`
	Voices      []VoiceSettingView   `json:"voices"`
}

// ModelSettingView is the safe view of a model under a provider.
type ModelSettingView struct {
	ID           providers.ModelID `json:"id"`
	DisplayName  string            `json:"display_name"`
	Capabilities []Capability      `json:"capabilities"`
	EnabledText  bool              `json:"enabled_text"`
	EnabledImage bool              `json:"enabled_image"`
	EnabledTTS   bool              `json:"enabled_tts"`
}

// VoiceSettingView is the safe view of a voice under a provider.
type VoiceSettingView struct {
	ID          providers.VoiceID `json:"id"`
	DisplayName string            `json:"display_name"`
	Enabled     bool              `json:"enabled"`
}

// ProviderSettingsListResponse is the response body for GET /api/v1/ai/provider-settings.
type ProviderSettingsListResponse struct {
	Providers []ProviderSettingView `json:"providers"`
}

// TextGenerationOptionModel is a model option for text generation.
type TextGenerationOptionModel struct {
	ID          providers.ModelID `json:"id"`
	DisplayName string            `json:"display_name"`
}

// TextGenerationOptionProvider is a provider option with available models.
type TextGenerationOptionProvider struct {
	ID          providers.ProviderID        `json:"id"`
	DisplayName string                      `json:"display_name"`
	Models      []TextGenerationOptionModel `json:"models"`
}

// TextGenerationOptionsResponse is the response for GET /api/v1/ai/text-generation-options.
type TextGenerationOptionsResponse struct {
	Providers []TextGenerationOptionProvider `json:"providers"`
}

// ImageGenerationOptionModel is a model option for image generation.
type ImageGenerationOptionModel struct {
	ID          providers.ModelID `json:"id"`
	DisplayName string            `json:"display_name"`
}

// ImageGenerationOptionProvider is a provider option with available image models.
type ImageGenerationOptionProvider struct {
	ID          providers.ProviderID         `json:"id"`
	DisplayName string                       `json:"display_name"`
	Models      []ImageGenerationOptionModel `json:"models"`
}

// ImageGenerationOptionsResponse is the response for GET /api/v1/ai/image-generation-options.
type ImageGenerationOptionsResponse struct {
	Providers []ImageGenerationOptionProvider `json:"providers"`
}

// TTSOptionVoice is a voice option for TTS.
type TTSOptionVoice struct {
	ID          providers.VoiceID `json:"id"`
	DisplayName string            `json:"display_name"`
}

// TTSOptionModel is a model option for TTS.
type TTSOptionModel struct {
	ID          providers.ModelID `json:"id"`
	DisplayName string            `json:"display_name"`
}

// TTSOptionProvider is a provider option with available voices.
type TTSOptionProvider struct {
	ID          providers.ProviderID `json:"id"`
	DisplayName string               `json:"display_name"`
	Models      []TTSOptionModel     `json:"models"`
	Voices      []TTSOptionVoice     `json:"voices"`
}

// TTSOptionsResponse is the response for GET /api/v1/ai/tts-options.
type TTSOptionsResponse struct {
	Providers []TTSOptionProvider `json:"providers"`
}

// PutSettingInput is the request body for PUT /api/v1/ai/provider-settings/{provider_id}.
type PutSettingInput struct {
	Revision             *int                `json:"revision,omitempty"`
	Enabled              bool                `json:"enabled"`
	EnabledModelIDs      []providers.ModelID `json:"enabled_model_ids"`      // Legacy TASK-017 field, maps to text
	EnabledTextModelIDs  []providers.ModelID `json:"enabled_text_model_ids"` // New multi-capability field
	EnabledImageModelIDs []providers.ModelID `json:"enabled_image_model_ids"`
	EnabledTTSModelIDs   []providers.ModelID `json:"enabled_tts_model_ids"`
	EnabledVoiceIDs      []providers.VoiceID `json:"enabled_voice_ids"`
	APIKey               *string             `json:"api_key,omitempty"`
}

package providersettings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaicompat"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaiimage"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaitts"
)

// Service provides owner-scoped provider settings management and runtime resolution.
type Service struct {
	catalog    *Catalog
	repo       Repository
	cipher     Cipher
	httpClient *http.Client
}

// NewService constructs a Service with the given catalog, repository, cipher, and HTTP client.
func NewService(catalog *Catalog, repo Repository, cipher Cipher, httpClient *http.Client) *Service {
	return &Service{
		catalog:    catalog,
		repo:       repo,
		cipher:     cipher,
		httpClient: httpClient,
	}
}

// ListSettings returns the safe settings views for all catalog providers for the given owner.
func (s *Service) ListSettings(ctx context.Context, ownerID uuid.UUID) (ProviderSettingsListResponse, error) {
	if s.catalog == nil {
		return ProviderSettingsListResponse{Providers: []ProviderSettingView{}}, nil
	}

	settings, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return ProviderSettingsListResponse{}, fmt.Errorf("list settings: %w", err)
	}

	settingsByProvider := make(map[providers.ProviderID]Setting, len(settings))
	for _, st := range settings {
		settingsByProvider[st.ProviderID] = st
	}

	catalogProviders := s.catalog.Providers()
	views := make([]ProviderSettingView, 0, len(catalogProviders))

	for _, p := range catalogProviders {
		st, configured := settingsByProvider[p.ProviderID]

		enabledTextMap := make(map[providers.ModelID]bool, len(st.EnabledTextModelIDs))
		for _, mID := range st.EnabledTextModelIDs {
			enabledTextMap[mID] = true
		}
		enabledImageMap := make(map[providers.ModelID]bool, len(st.EnabledImageModelIDs))
		for _, mID := range st.EnabledImageModelIDs {
			enabledImageMap[mID] = true
		}
		enabledVideoMap := make(map[providers.ModelID]bool, len(st.EnabledVideoModelIDs))
		for _, mID := range st.EnabledVideoModelIDs {
			enabledVideoMap[mID] = true
		}
		enabledTTSMap := make(map[providers.ModelID]bool, len(st.EnabledTTSModelIDs))
		for _, mID := range st.EnabledTTSModelIDs {
			enabledTTSMap[mID] = true
		}

		enabledVoicesMap := make(map[providers.VoiceID]bool, len(st.EnabledVoiceIDs))
		for _, vID := range st.EnabledVoiceIDs {
			enabledVoicesMap[vID] = true
		}

		modelViews := make([]ModelSettingView, len(p.Models))
		for i, m := range p.Models {
			modelViews[i] = ModelSettingView{
				ID:           m.ModelID,
				DisplayName:  m.DisplayName,
				Capabilities: m.Capabilities,
				EnabledText:  configured && st.Enabled && enabledTextMap[m.ModelID],
				EnabledImage: configured && st.Enabled && enabledImageMap[m.ModelID],
				EnabledVideo: configured && st.Enabled && enabledVideoMap[m.ModelID],
				EnabledTTS:   configured && st.Enabled && enabledTTSMap[m.ModelID],
			}
		}

		voiceViews := make([]VoiceSettingView, len(p.Voices))
		for i, v := range p.Voices {
			voiceViews[i] = VoiceSettingView{
				ID:          v.VoiceID,
				DisplayName: v.DisplayName,
				Enabled:     configured && st.Enabled && enabledVoicesMap[v.VoiceID],
			}
		}

		views = append(views, ProviderSettingView{
			ID:          p.ProviderID,
			DisplayName: p.DisplayName,
			Configured:  configured,
			Enabled:     configured && st.Enabled,
			HasAPIKey:   configured && len(st.APIKeyCiphertext) > 0,
			Revision:    st.Revision,
			Models:      modelViews,
			Voices:      voiceViews,
		})
	}

	return ProviderSettingsListResponse{Providers: views}, nil
}

// PutSetting creates or updates an owner's settings for a provider.
func (s *Service) PutSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, input PutSettingInput) (ProviderSettingView, error) {
	if s.cipher == nil {
		return ProviderSettingView{}, ErrMasterKeyMissing
	}
	if s.catalog == nil {
		return ProviderSettingView{}, ErrProviderNotFound
	}

	provDef, ok := s.catalog.GetProvider(providerID)
	if !ok {
		return ProviderSettingView{}, ErrProviderNotFound
	}

	// Backward compatibility: merge legacy enabled_model_ids into enabled_text_model_ids
	// if explicit text list is empty (TASK-017 clients).
	if len(input.EnabledModelIDs) > 0 && len(input.EnabledTextModelIDs) == 0 {
		input.EnabledTextModelIDs = input.EnabledModelIDs
	}

	if err := s.validateEnabledModels(providerID, input.EnabledTextModelIDs, CapabilityText); err != nil {
		return ProviderSettingView{}, err
	}
	if err := s.validateEnabledModels(providerID, input.EnabledImageModelIDs, CapabilityImage); err != nil {
		return ProviderSettingView{}, err
	}
	if err := s.validateEnabledModels(providerID, input.EnabledVideoModelIDs, CapabilityVideo); err != nil {
		return ProviderSettingView{}, err
	}
	if err := s.validateEnabledModels(providerID, input.EnabledTTSModelIDs, CapabilityTTS); err != nil {
		return ProviderSettingView{}, err
	}
	if len(input.EnabledVoiceIDs) > 0 {
		for _, vID := range input.EnabledVoiceIDs {
			if _, ok := s.catalog.GetVoice(providerID, vID); !ok {
				return ProviderSettingView{}, fmt.Errorf("%w: voice %q under provider %q", ErrModelNotFound, vID, providerID)
			}
		}
	}

	if input.Enabled && len(input.EnabledTextModelIDs) == 0 && len(input.EnabledImageModelIDs) == 0 && len(input.EnabledVideoModelIDs) == 0 && len(input.EnabledTTSModelIDs) == 0 && len(input.EnabledVoiceIDs) == 0 {
		return ProviderSettingView{}, fmt.Errorf("%w: at least one model or voice must be enabled when provider is enabled", ErrInvalidSettingInput)
	}

	existing, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	isExisting := err == nil
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return ProviderSettingView{}, fmt.Errorf("get existing setting: %w", err)
	}

	if !isExisting {
		if input.Revision != nil {
			return ProviderSettingView{}, ErrStaleRevision
		}
		if input.APIKey == nil || strings.TrimSpace(*input.APIKey) == "" {
			return ProviderSettingView{}, ErrCredentialRequired
		}
		if err := validateAPIKey(*input.APIKey); err != nil {
			return ProviderSettingView{}, err
		}

		ciphertext, nonce, err := s.cipher.Encrypt(ownerID, providerID, s.cipher.KeyVersion(), *input.APIKey)
		if err != nil {
			return ProviderSettingView{}, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		setting := Setting{
			OwnerID:              ownerID,
			ProviderID:           providerID,
			Enabled:              input.Enabled,
			EnabledTextModelIDs:  input.EnabledTextModelIDs,
			EnabledImageModelIDs: input.EnabledImageModelIDs,
			EnabledVideoModelIDs: input.EnabledVideoModelIDs,
			EnabledTTSModelIDs:   input.EnabledTTSModelIDs,
			EnabledVoiceIDs:      input.EnabledVoiceIDs,
			APIKeyCiphertext:     ciphertext,
			APIKeyNonce:          nonce,
			KeyVersion:           s.cipher.KeyVersion(),
		}

		saved, err := s.repo.Save(ctx, setting, nil)
		if err != nil {
			return ProviderSettingView{}, err
		}
		return toProviderSettingView(provDef, saved), nil
	}

	if input.Revision == nil || *input.Revision != existing.Revision {
		return ProviderSettingView{}, ErrStaleRevision
	}

	ciphertext := existing.APIKeyCiphertext
	nonce := existing.APIKeyNonce
	keyVersion := existing.KeyVersion

	if input.APIKey != nil && *input.APIKey != "" {
		if err := validateAPIKey(*input.APIKey); err != nil {
			return ProviderSettingView{}, err
		}
		var encErr error
		ciphertext, nonce, encErr = s.cipher.Encrypt(ownerID, providerID, s.cipher.KeyVersion(), *input.APIKey)
		if encErr != nil {
			return ProviderSettingView{}, fmt.Errorf("%w: %v", ErrEncryptionFailed, encErr)
		}
		keyVersion = s.cipher.KeyVersion()
	}

	if input.Enabled && len(ciphertext) == 0 {
		return ProviderSettingView{}, ErrCredentialRequired
	}

	setting := Setting{
		OwnerID:              ownerID,
		ProviderID:           providerID,
		Enabled:              input.Enabled,
		EnabledTextModelIDs:  input.EnabledTextModelIDs,
		EnabledImageModelIDs: input.EnabledImageModelIDs,
		EnabledVideoModelIDs: input.EnabledVideoModelIDs,
		EnabledTTSModelIDs:   input.EnabledTTSModelIDs,
		EnabledVoiceIDs:      input.EnabledVoiceIDs,
		APIKeyCiphertext:     ciphertext,
		APIKeyNonce:          nonce,
		KeyVersion:           keyVersion,
	}

	saved, err := s.repo.Save(ctx, setting, input.Revision)
	if err != nil {
		return ProviderSettingView{}, err
	}

	return toProviderSettingView(provDef, saved), nil
}

func (s *Service) validateEnabledModels(providerID providers.ProviderID, modelIDs []providers.ModelID, capability Capability) error {
	for _, modelID := range modelIDs {
		if _, ok := s.catalog.GetModel(providerID, modelID); !ok {
			return fmt.Errorf("%w: model %q under provider %q", ErrModelNotFound, modelID, providerID)
		}
		if !s.catalog.ModelSupportsCapability(providerID, modelID, capability) {
			return fmt.Errorf("%w: model %q does not support %s capability", ErrInvalidSettingInput, modelID, capability)
		}
	}
	return nil
}

// DeleteSetting removes an owner's settings and encrypted credential for a provider.
func (s *Service) DeleteSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, revision int) error {
	if s.catalog != nil {
		if _, ok := s.catalog.GetProvider(providerID); !ok {
			return ErrProviderNotFound
		}
	}
	return s.repo.Delete(ctx, ownerID, providerID, revision)
}

// GetOwnerTextGenerationOptions returns only the current owner's configured and enabled text models.
func (s *Service) GetOwnerTextGenerationOptions(ctx context.Context, ownerID uuid.UUID) (TextGenerationOptionsResponse, error) {
	if s.catalog == nil {
		return TextGenerationOptionsResponse{Providers: []TextGenerationOptionProvider{}}, nil
	}

	settings, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return TextGenerationOptionsResponse{}, fmt.Errorf("list settings for options: %w", err)
	}

	settingsByProvider := make(map[providers.ProviderID]Setting, len(settings))
	for _, st := range settings {
		if st.Enabled && len(st.APIKeyCiphertext) > 0 {
			settingsByProvider[st.ProviderID] = st
		}
	}

	var resultProviders []TextGenerationOptionProvider
	for _, p := range s.catalog.Providers() {
		st, ok := settingsByProvider[p.ProviderID]
		if !ok {
			continue
		}

		enabledMap := make(map[providers.ModelID]bool, len(st.EnabledTextModelIDs))
		for _, mID := range st.EnabledTextModelIDs {
			enabledMap[mID] = true
		}

		var optionModels []TextGenerationOptionModel
		for _, m := range p.Models {
			if enabledMap[m.ModelID] {
				optionModels = append(optionModels, TextGenerationOptionModel{
					ID:          m.ModelID,
					DisplayName: m.DisplayName,
				})
			}
		}

		if len(optionModels) > 0 {
			resultProviders = append(resultProviders, TextGenerationOptionProvider{
				ID:          p.ProviderID,
				DisplayName: p.DisplayName,
				Models:      optionModels,
			})
		}
	}

	sort.Slice(resultProviders, func(i, j int) bool {
		return resultProviders[i].ID < resultProviders[j].ID
	})

	return TextGenerationOptionsResponse{Providers: resultProviders}, nil
}

// ResolveTextGenerator resolves a provider-neutral TextGenerator configured with the owner's credential.
func (s *Service) ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
	if s.cipher == nil {
		return nil, providers.ErrProviderUnavailable
	}
	if s.catalog == nil {
		return nil, providers.ErrProviderUnavailable
	}

	provDef, ok := s.catalog.GetProvider(providerID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}

	modelDef, ok := s.catalog.GetModel(providerID, modelID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}

	setting, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}
	if !setting.Enabled || len(setting.APIKeyCiphertext) == 0 {
		return nil, providers.ErrProviderUnavailable
	}

	isModelEnabled := false
	for _, mID := range setting.EnabledTextModelIDs {
		if mID == modelID {
			isModelEnabled = true
			break
		}
	}
	if !isModelEnabled {
		return nil, providers.ErrProviderUnavailable
	}

	apiKey, err := s.cipher.Decrypt(ownerID, providerID, setting.KeyVersion, setting.APIKeyCiphertext, setting.APIKeyNonce)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}

	adapter, err := openaicompat.New(openaicompat.Config{
		ProviderID:  provDef.ProviderID,
		DisplayName: provDef.DisplayName,
		BaseURL:     provDef.BaseURL,
		CredentialSource: openaicompat.SecretSourceFunc(func(ctx context.Context) (string, error) {
			return apiKey, nil
		}),
		Models: []openaicompat.ModelConfig{
			{
				ID:              modelDef.ModelID,
				DisplayName:     modelDef.DisplayName,
				ExternalModelID: modelDef.ExternalModelID,
			},
		},
		Timeout:          provDef.Timeout,
		MaxResponseBytes: provDef.MaxResponseBytes,
		HTTPClient:       s.httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrProviderUnavailable, err)
	}

	return adapter, nil
}

// GetOwnerImageGenerationOptions returns only the current owner's configured and enabled image models.
func (s *Service) GetOwnerImageGenerationOptions(ctx context.Context, ownerID uuid.UUID) (ImageGenerationOptionsResponse, error) {
	if s.catalog == nil {
		return ImageGenerationOptionsResponse{Providers: []ImageGenerationOptionProvider{}}, nil
	}

	settings, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return ImageGenerationOptionsResponse{}, fmt.Errorf("list settings for image options: %w", err)
	}

	settingsByProvider := make(map[providers.ProviderID]Setting, len(settings))
	for _, st := range settings {
		if st.Enabled && len(st.APIKeyCiphertext) > 0 {
			settingsByProvider[st.ProviderID] = st
		}
	}

	var resultProviders []ImageGenerationOptionProvider
	for _, p := range s.catalog.Providers() {
		st, ok := settingsByProvider[p.ProviderID]
		if !ok {
			continue
		}

		enabledMap := make(map[providers.ModelID]bool, len(st.EnabledImageModelIDs))
		for _, mID := range st.EnabledImageModelIDs {
			enabledMap[mID] = true
		}

		var optionModels []ImageGenerationOptionModel
		for _, m := range s.catalog.ModelsForCapability(p.ProviderID, CapabilityImage) {
			if enabledMap[m.ModelID] {
				optionModels = append(optionModels, ImageGenerationOptionModel{
					ID:          m.ModelID,
					DisplayName: m.DisplayName,
				})
			}
		}

		if len(optionModels) > 0 {
			resultProviders = append(resultProviders, ImageGenerationOptionProvider{
				ID:          p.ProviderID,
				DisplayName: p.DisplayName,
				Models:      optionModels,
			})
		}
	}

	sort.Slice(resultProviders, func(i, j int) bool {
		return resultProviders[i].ID < resultProviders[j].ID
	})

	return ImageGenerationOptionsResponse{Providers: resultProviders}, nil
}

// GetOwnerTTSOptions returns only the current owner's configured and enabled TTS models and voices.
func (s *Service) GetOwnerTTSOptions(ctx context.Context, ownerID uuid.UUID) (TTSOptionsResponse, error) {
	if s.catalog == nil {
		return TTSOptionsResponse{Providers: []TTSOptionProvider{}}, nil
	}

	settings, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return TTSOptionsResponse{}, fmt.Errorf("list settings for tts options: %w", err)
	}

	settingsByProvider := make(map[providers.ProviderID]Setting, len(settings))
	for _, st := range settings {
		if st.Enabled && len(st.APIKeyCiphertext) > 0 {
			settingsByProvider[st.ProviderID] = st
		}
	}

	var resultProviders []TTSOptionProvider
	for _, p := range s.catalog.Providers() {
		st, ok := settingsByProvider[p.ProviderID]
		if !ok {
			continue
		}

		enabledVoiceMap := make(map[providers.VoiceID]bool, len(st.EnabledVoiceIDs))
		for _, vID := range st.EnabledVoiceIDs {
			enabledVoiceMap[vID] = true
		}

		enabledTTSModelMap := make(map[providers.ModelID]bool, len(st.EnabledTTSModelIDs))
		for _, mID := range st.EnabledTTSModelIDs {
			enabledTTSModelMap[mID] = true
		}

		var optionVoices []TTSOptionVoice
		for _, v := range p.Voices {
			if enabledVoiceMap[v.VoiceID] {
				optionVoices = append(optionVoices, TTSOptionVoice{
					ID:          v.VoiceID,
					DisplayName: v.DisplayName,
				})
			}
		}

		var optionModels []TTSOptionModel
		for _, m := range p.Models {
			if s.catalog.ModelSupportsCapability(p.ProviderID, m.ModelID, CapabilityTTS) && enabledTTSModelMap[m.ModelID] {
				optionModels = append(optionModels, TTSOptionModel{
					ID:          m.ModelID,
					DisplayName: m.DisplayName,
				})
			}
		}

		if len(optionVoices) > 0 || len(optionModels) > 0 {
			resultProviders = append(resultProviders, TTSOptionProvider{
				ID:          p.ProviderID,
				DisplayName: p.DisplayName,
				Models:      optionModels,
				Voices:      optionVoices,
			})
		}
	}

	sort.Slice(resultProviders, func(i, j int) bool {
		return resultProviders[i].ID < resultProviders[j].ID
	})

	return TTSOptionsResponse{Providers: resultProviders}, nil
}

// ResolveImageGenerator resolves a provider-neutral ImageGenerator configured with the owner's credential.
func (s *Service) ResolveImageGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.ImageGenerator, error) {
	if s.cipher == nil {
		return nil, providers.ErrProviderUnavailable
	}
	if s.catalog == nil {
		return nil, providers.ErrProviderUnavailable
	}

	provDef, ok := s.catalog.GetProvider(providerID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}

	modelDef, ok := s.catalog.GetModel(providerID, modelID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}
	if !s.catalog.ModelSupportsCapability(providerID, modelID, CapabilityImage) {
		return nil, providers.ErrProviderUnavailable
	}

	setting, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}
	if !setting.Enabled || len(setting.APIKeyCiphertext) == 0 {
		return nil, providers.ErrProviderUnavailable
	}

	isModelEnabled := false
	for _, mID := range setting.EnabledImageModelIDs {
		if mID == modelID {
			isModelEnabled = true
			break
		}
	}
	if !isModelEnabled {
		return nil, providers.ErrProviderUnavailable
	}

	apiKey, err := s.cipher.Decrypt(ownerID, providerID, setting.KeyVersion, setting.APIKeyCiphertext, setting.APIKeyNonce)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}

	gen, err := openaiimage.NewImageGenerator(openaiimage.Config{
		ProviderID:  provDef.ProviderID,
		DisplayName: provDef.DisplayName,
		BaseURL:     provDef.BaseURL,
		CredentialSource: openaiimage.SecretSourceFunc(func(ctx context.Context) (string, error) {
			return apiKey, nil
		}),
		Models: []openaiimage.ModelConfig{
			{
				ID:              modelDef.ModelID,
				DisplayName:     modelDef.DisplayName,
				ExternalModelID: modelDef.ExternalModelID,
			},
		},
		Timeout:          provDef.Timeout,
		MaxResponseBytes: provDef.MaxResponseBytes,
		HTTPClient:       s.httpClient,
	}, modelID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrProviderUnavailable, err)
	}
	return gen, nil
}

// ResolveSpeechSynthesizer resolves a provider-neutral SpeechSynthesizer configured with the owner's credential.
func (s *Service) ResolveSpeechSynthesizer(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.SpeechSynthesizer, error) {
	if s.cipher == nil {
		return nil, providers.ErrProviderUnavailable
	}
	if s.catalog == nil {
		return nil, providers.ErrProviderUnavailable
	}

	provDef, ok := s.catalog.GetProvider(providerID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}

	modelDef, ok := s.catalog.GetModel(providerID, modelID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}
	if !s.catalog.ModelSupportsCapability(providerID, modelID, CapabilityTTS) {
		return nil, providers.ErrProviderUnavailable
	}

	setting, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}
	if !setting.Enabled || len(setting.APIKeyCiphertext) == 0 {
		return nil, providers.ErrProviderUnavailable
	}

	if len(setting.EnabledTTSModelIDs) == 0 {
		return nil, providers.ErrProviderUnavailable
	}
	ttsModelEnabled := false
	for _, mID := range setting.EnabledTTSModelIDs {
		if mID == modelID {
			ttsModelEnabled = true
			break
		}
	}
	if !ttsModelEnabled {
		return nil, providers.ErrProviderUnavailable
	}

	if len(setting.EnabledVoiceIDs) == 0 {
		return nil, providers.ErrProviderUnavailable
	}

	apiKey, err := s.cipher.Decrypt(ownerID, providerID, setting.KeyVersion, setting.APIKeyCiphertext, setting.APIKeyNonce)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}

	enabledVoiceMap := make(map[providers.VoiceID]bool, len(setting.EnabledVoiceIDs))
	for _, vID := range setting.EnabledVoiceIDs {
		enabledVoiceMap[vID] = true
	}
	filteredVoices := make([]VoiceDefinition, 0, len(provDef.Voices))
	for _, v := range provDef.Voices {
		if enabledVoiceMap[v.VoiceID] {
			filteredVoices = append(filteredVoices, v)
		}
	}

	synth, err := openaitts.NewSpeechSynthesizer(openaitts.Config{
		ProviderID:  provDef.ProviderID,
		DisplayName: provDef.DisplayName,
		BaseURL:     provDef.BaseURL,
		CredentialSource: openaitts.SecretSourceFunc(func(ctx context.Context) (string, error) {
			return apiKey, nil
		}),
		Models: []openaitts.ModelConfig{
			{
				ID:              modelDef.ModelID,
				DisplayName:     modelDef.DisplayName,
				ExternalModelID: modelDef.ExternalModelID,
			},
		},
		Voices:           buildOpenAITTSVoices(filteredVoices),
		Timeout:          provDef.Timeout,
		MaxResponseBytes: provDef.MaxResponseBytes,
		HTTPClient:       s.httpClient,
	}, modelID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrProviderUnavailable, err)
	}
	return synth, nil
}

func buildOpenAITTSVoices(defs []VoiceDefinition) []openaitts.VoiceConfig {
	out := make([]openaitts.VoiceConfig, 0, len(defs))
	for _, v := range defs {
		out = append(out, openaitts.VoiceConfig{
			ID:            v.VoiceID,
			DisplayName:   v.DisplayName,
			ExternalVoice: v.ExternalVoice,
		})
	}
	return out
}

func toProviderSettingView(p ProviderDefinition, s Setting) ProviderSettingView {
	enabledTextMap := make(map[providers.ModelID]bool, len(s.EnabledTextModelIDs))
	for _, mID := range s.EnabledTextModelIDs {
		enabledTextMap[mID] = true
	}
	enabledImageMap := make(map[providers.ModelID]bool, len(s.EnabledImageModelIDs))
	for _, mID := range s.EnabledImageModelIDs {
		enabledImageMap[mID] = true
	}
	enabledVideoMap := make(map[providers.ModelID]bool, len(s.EnabledVideoModelIDs))
	for _, mID := range s.EnabledVideoModelIDs {
		enabledVideoMap[mID] = true
	}
	enabledTTSMap := make(map[providers.ModelID]bool, len(s.EnabledTTSModelIDs))
	for _, mID := range s.EnabledTTSModelIDs {
		enabledTTSMap[mID] = true
	}

	modelViews := make([]ModelSettingView, len(p.Models))
	for i, m := range p.Models {
		modelViews[i] = ModelSettingView{
			ID:           m.ModelID,
			DisplayName:  m.DisplayName,
			Capabilities: m.Capabilities,
			EnabledText:  s.Enabled && enabledTextMap[m.ModelID],
			EnabledImage: s.Enabled && enabledImageMap[m.ModelID],
			EnabledVideo: s.Enabled && enabledVideoMap[m.ModelID],
			EnabledTTS:   s.Enabled && enabledTTSMap[m.ModelID],
		}
	}

	enabledVoiceMap := make(map[providers.VoiceID]bool, len(s.EnabledVoiceIDs))
	for _, vID := range s.EnabledVoiceIDs {
		enabledVoiceMap[vID] = true
	}

	voiceViews := make([]VoiceSettingView, len(p.Voices))
	for i, v := range p.Voices {
		voiceViews[i] = VoiceSettingView{
			ID:          v.VoiceID,
			DisplayName: v.DisplayName,
			Enabled:     s.Enabled && enabledVoiceMap[v.VoiceID],
		}
	}

	return ProviderSettingView{
		ID:          p.ProviderID,
		DisplayName: p.DisplayName,
		Configured:  true,
		Enabled:     s.Enabled,
		HasAPIKey:   len(s.APIKeyCiphertext) > 0,
		Revision:    s.Revision,
		Models:      modelViews,
		Voices:      voiceViews,
	}
}

func validateAPIKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%w: api_key cannot be empty", ErrInvalidSettingInput)
	}
	if key != strings.TrimSpace(key) {
		return fmt.Errorf("%w: api_key must not contain leading or trailing whitespace", ErrInvalidSettingInput)
	}
	if len(key) > MaxAPIKeyLength {
		return fmt.Errorf("%w: api_key length exceeds maximum of %d bytes", ErrInvalidSettingInput, MaxAPIKeyLength)
	}
	return nil
}

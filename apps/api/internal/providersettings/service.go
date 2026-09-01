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

		enabledModelsMap := make(map[providers.ModelID]bool, len(st.EnabledModelIDs))
		for _, mID := range st.EnabledModelIDs {
			enabledModelsMap[mID] = true
		}

		modelViews := make([]ModelSettingView, len(p.Models))
		for i, m := range p.Models {
			modelViews[i] = ModelSettingView{
				ID:          m.ModelID,
				DisplayName: m.DisplayName,
				Enabled:     configured && st.Enabled && enabledModelsMap[m.ModelID],
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

	// Validate model IDs
	if len(input.EnabledModelIDs) > 0 {
		for _, mID := range input.EnabledModelIDs {
			if _, ok := s.catalog.GetModel(providerID, mID); !ok {
				return ProviderSettingView{}, fmt.Errorf("%w: model %q under provider %q", ErrModelNotFound, mID, providerID)
			}
		}
	}

	if input.Enabled && len(input.EnabledModelIDs) == 0 {
		return ProviderSettingView{}, fmt.Errorf("%w: at least one model must be enabled when provider is enabled", ErrInvalidSettingInput)
	}

	existing, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	isExisting := err == nil
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return ProviderSettingView{}, fmt.Errorf("get existing setting: %w", err)
	}

	// First configuration: revision must be omitted (nil), api_key is required
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
			OwnerID:          ownerID,
			ProviderID:       providerID,
			Enabled:          input.Enabled,
			EnabledModelIDs:  input.EnabledModelIDs,
			APIKeyCiphertext: ciphertext,
			APIKeyNonce:      nonce,
			KeyVersion:       s.cipher.KeyVersion(),
		}

		saved, err := s.repo.Save(ctx, setting, nil)
		if err != nil {
			return ProviderSettingView{}, err
		}
		return toProviderSettingView(provDef, saved), nil
	}

	// Update existing configuration
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
		OwnerID:          ownerID,
		ProviderID:       providerID,
		Enabled:          input.Enabled,
		EnabledModelIDs:  input.EnabledModelIDs,
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
		KeyVersion:       keyVersion,
	}

	saved, err := s.repo.Save(ctx, setting, input.Revision)
	if err != nil {
		return ProviderSettingView{}, err
	}

	return toProviderSettingView(provDef, saved), nil
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

		enabledMap := make(map[providers.ModelID]bool, len(st.EnabledModelIDs))
		for _, mID := range st.EnabledModelIDs {
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
	for _, mID := range setting.EnabledModelIDs {
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

func toProviderSettingView(p ProviderDefinition, s Setting) ProviderSettingView {
	enabledMap := make(map[providers.ModelID]bool, len(s.EnabledModelIDs))
	for _, mID := range s.EnabledModelIDs {
		enabledMap[mID] = true
	}

	modelViews := make([]ModelSettingView, len(p.Models))
	for i, m := range p.Models {
		modelViews[i] = ModelSettingView{
			ID:          m.ModelID,
			DisplayName: m.DisplayName,
			Enabled:     s.Enabled && enabledMap[m.ModelID],
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

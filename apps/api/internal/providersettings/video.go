package providersettings

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/runwayvideo"
)

// GetOwnerVideoGenerationOptions returns only configured and enabled video models.
func (s *Service) GetOwnerVideoGenerationOptions(ctx context.Context, ownerID uuid.UUID) (VideoGenerationOptionsResponse, error) {
	if s.catalog == nil {
		return VideoGenerationOptionsResponse{Providers: []VideoGenerationOptionProvider{}}, nil
	}
	settings, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return VideoGenerationOptionsResponse{}, fmt.Errorf("list settings for video options: %w", err)
	}
	settingsByProvider := make(map[providers.ProviderID]Setting, len(settings))
	for _, setting := range settings {
		if setting.Enabled && len(setting.APIKeyCiphertext) > 0 {
			settingsByProvider[setting.ProviderID] = setting
		}
	}
	result := make([]VideoGenerationOptionProvider, 0)
	for _, providerDef := range s.catalog.Providers() {
		setting, ok := settingsByProvider[providerDef.ProviderID]
		if !ok {
			continue
		}
		enabled := make(map[providers.ModelID]bool, len(setting.EnabledVideoModelIDs))
		for _, modelID := range setting.EnabledVideoModelIDs {
			enabled[modelID] = true
		}
		models := make([]VideoGenerationOptionModel, 0)
		for _, model := range providerDef.Models {
			if !enabled[model.ModelID] || !s.catalog.ModelSupportsCapability(providerDef.ProviderID, model.ModelID, CapabilityVideo) {
				continue
			}
			option := VideoGenerationOptionModel{ID: model.ModelID, DisplayName: model.DisplayName}
			if providerDef.ProviderID == providers.ProviderID("runway") && model.ModelID == providers.ModelID("gen4.5") {
				option.MinDurationSeconds = runwayvideo.MinDurationSeconds
				option.MaxDurationSeconds = runwayvideo.MaxDurationSeconds
			}
			models = append(models, option)
		}
		if len(models) > 0 {
			result = append(result, VideoGenerationOptionProvider{ID: providerDef.ProviderID, DisplayName: providerDef.DisplayName, Models: models})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return VideoGenerationOptionsResponse{Providers: result}, nil
}

// ResolveVideoGenerator resolves the first supported video adapter for an enabled owner model.
// V1 intentionally wires Runway as the first adapter while keeping the domain interface provider-neutral.
func (s *Service) ResolveVideoGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.VideoGenerator, error) {
	if s.cipher == nil || s.catalog == nil {
		return nil, providers.ErrProviderUnavailable
	}
	providerDef, ok := s.catalog.GetProvider(providerID)
	if !ok {
		return nil, providers.ErrProviderUnavailable
	}
	modelDef, ok := s.catalog.GetModel(providerID, modelID)
	if !ok || !s.catalog.ModelSupportsCapability(providerID, modelID, CapabilityVideo) {
		return nil, providers.ErrProviderUnavailable
	}
	setting, err := s.repo.GetByOwnerAndProvider(ctx, ownerID, providerID)
	if err != nil || !setting.Enabled || len(setting.APIKeyCiphertext) == 0 {
		return nil, providers.ErrProviderUnavailable
	}
	enabled := false
	for _, candidate := range setting.EnabledVideoModelIDs {
		if candidate == modelID {
			enabled = true
			break
		}
	}
	if !enabled {
		return nil, providers.ErrProviderUnavailable
	}
	apiKey, err := s.cipher.Decrypt(ownerID, providerID, setting.KeyVersion, setting.APIKeyCiphertext, setting.APIKeyNonce)
	if err != nil {
		return nil, providers.ErrProviderUnavailable
	}
	// TASK-032 V1 has one concrete adapter. Other provider IDs remain unavailable until
	// their own adapter is registered rather than being silently treated as Runway-compatible.
	if providerID != providers.ProviderID("runway") {
		return nil, providers.ErrUnsupportedCapability
	}
	adapter, err := runwayvideo.New(runwayvideo.Config{
		ProviderID: providerDef.ProviderID,
		BaseURL:    providerDef.BaseURL,
		CredentialSource: runwayvideo.SecretSourceFunc(func(context.Context) (string, error) {
			return apiKey, nil
		}),
		Model: runwayvideo.ModelConfig{
			ID:              modelDef.ModelID,
			ExternalModelID: modelDef.ExternalModelID,
		},
		Timeout:          providerDef.Timeout,
		MaxResponseBytes: providerDef.MaxResponseBytes,
		HTTPClient:       s.httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrProviderUnavailable, err)
	}
	return adapter, nil
}

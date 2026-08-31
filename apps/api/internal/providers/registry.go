package providers

import (
	"fmt"
	"sort"
	"sync"
)

type modelEntry struct {
	metadata      ModelMetadata
	textGenerator TextGenerator
}

type providerEntry struct {
	metadata ProviderMetadata
	models   map[ModelID]modelEntry
}

// Registry is a deterministic in-memory provider/model catalog.
type Registry struct {
	mu        sync.RWMutex
	providers map[ProviderID]providerEntry
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[ProviderID]providerEntry),
	}
}

func (r *Registry) Register(registration Registration) error {
	registration = cloneRegistration(registration)
	if err := validateRegistration(registration); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[registration.Provider.ID]; exists {
		return NewDuplicateRegistrationError(registration.Provider.ID)
	}

	entry := providerEntry{
		metadata: registration.Provider,
		models:   make(map[ModelID]modelEntry, len(registration.Models)),
	}
	for _, model := range registration.Models {
		if _, exists := entry.models[model.Metadata.ID]; exists {
			return NewDuplicateRegistrationError(registration.Provider.ID)
		}
		entry.models[model.Metadata.ID] = modelEntry{
			metadata:      model.Metadata,
			textGenerator: model.TextGenerator,
		}
	}

	r.providers[registration.Provider.ID] = entry
	return nil
}

func (r *Registry) ResolveTextGenerator(providerID ProviderID, modelID ModelID) (TextGenerator, ModelMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[providerID]
	if !ok {
		return nil, ModelMetadata{}, NewUnknownProviderError(providerID)
	}

	model, ok := provider.models[modelID]
	if !ok {
		return nil, ModelMetadata{}, NewUnknownModelError(providerID, modelID)
	}
	if model.textGenerator == nil || !model.metadata.Supports(CapabilityTextGeneration) {
		return nil, cloneModelMetadata(model.metadata), NewUnsupportedCapabilityError(providerID, modelID, CapabilityTextGeneration)
	}

	return model.textGenerator, cloneModelMetadata(model.metadata), nil
}

func (r *Registry) ListProviders() []ProviderMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]ProviderMetadata, 0, len(r.providers))
	for _, provider := range r.providers {
		items = append(items, cloneProviderMetadata(provider.metadata))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func (r *Registry) ListModels(providerID ProviderID) ([]ModelMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[providerID]
	if !ok {
		return nil, NewUnknownProviderError(providerID)
	}

	items := make([]ModelMetadata, 0, len(provider.models))
	for _, model := range provider.models {
		items = append(items, cloneModelMetadata(model.metadata))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func validateRegistration(registration Registration) error {
	if !registration.Provider.ID.Valid() {
		return fmt.Errorf("provider id is invalid")
	}
	if registration.Provider.DisplayName == "" {
		return fmt.Errorf("provider display name is required")
	}
	if len(registration.Models) == 0 {
		return fmt.Errorf("at least one model registration is required")
	}

	for _, model := range registration.Models {
		if model.Metadata.ProviderID != registration.Provider.ID {
			return fmt.Errorf("model provider id must match registration provider id")
		}
		if !model.Metadata.ID.Valid() {
			return fmt.Errorf("model id is invalid")
		}
		if model.Metadata.DisplayName == "" {
			return fmt.Errorf("model display name is required")
		}
		for _, capability := range model.Metadata.SupportedCapabilities {
			if !capability.Valid() {
				return fmt.Errorf("model capability is invalid")
			}
		}
		if model.Metadata.Supports(CapabilityTextGeneration) && model.TextGenerator == nil {
			return fmt.Errorf("text generation capability requires a text generator binding")
		}
	}

	return nil
}

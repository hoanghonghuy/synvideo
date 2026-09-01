package providersettings

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaicompat"
)

// ModelDefinition specifies the metadata and external upstream model mapping.
type ModelDefinition struct {
	ModelID         providers.ModelID `json:"model_id"`
	DisplayName     string            `json:"display_name"`
	ExternalModelID string            `json:"external_model_id"`
}

// ProviderDefinition contains non-secret deployment configuration for an OpenAI-compatible provider.
type ProviderDefinition struct {
	ProviderID       providers.ProviderID `json:"provider_id"`
	DisplayName      string               `json:"display_name"`
	BaseURL          string               `json:"base_url"`
	Models           []ModelDefinition    `json:"models"`
	Timeout          time.Duration        `json:"timeout,omitempty"`
	MaxResponseBytes int64                `json:"max_response_bytes,omitempty"`
}

// Catalog holds validated deployment provider definitions.
type Catalog struct {
	providers    []ProviderDefinition
	byProviderID map[providers.ProviderID]ProviderDefinition
	byModelID    map[providers.ProviderID]map[providers.ModelID]ModelDefinition
}

// NewCatalogFromJSON parses and validates a JSON array of ProviderDefinitions.
func NewCatalogFromJSON(raw []byte) (*Catalog, error) {
	var defs []ProviderDefinition
	if err := json.Unmarshal(raw, &defs); err != nil {
		return nil, fmt.Errorf("invalid provider definitions json: %w", err)
	}
	return NewCatalog(defs)
}

// NewCatalog validates and builds a Catalog from ProviderDefinitions.
func NewCatalog(defs []ProviderDefinition) (*Catalog, error) {
	if len(defs) == 0 {
		return &Catalog{
			providers:    []ProviderDefinition{},
			byProviderID: make(map[providers.ProviderID]ProviderDefinition),
			byModelID:    make(map[providers.ProviderID]map[providers.ModelID]ModelDefinition),
		}, nil
	}

	byProviderID := make(map[providers.ProviderID]ProviderDefinition, len(defs))
	byModelID := make(map[providers.ProviderID]map[providers.ModelID]ModelDefinition, len(defs))
	validatedProviders := make([]ProviderDefinition, 0, len(defs))

	for _, p := range defs {
		if !p.ProviderID.Valid() {
			return nil, fmt.Errorf("invalid provider_id: %q", p.ProviderID)
		}
		if strings.TrimSpace(p.DisplayName) == "" {
			return nil, fmt.Errorf("provider %q display_name cannot be empty", p.ProviderID)
		}
		if _, exists := byProviderID[p.ProviderID]; exists {
			return nil, fmt.Errorf("duplicate provider_id in catalog: %q", p.ProviderID)
		}

		if err := openaicompat.ValidateBaseURL(p.BaseURL); err != nil {
			return nil, fmt.Errorf("provider %q invalid base_url: %w", p.ProviderID, err)
		}

		if len(p.Models) == 0 {
			return nil, fmt.Errorf("provider %q must define at least one model", p.ProviderID)
		}

		modelsMap := make(map[providers.ModelID]ModelDefinition, len(p.Models))
		validatedModels := make([]ModelDefinition, 0, len(p.Models))

		for _, m := range p.Models {
			if !m.ModelID.Valid() {
				return nil, fmt.Errorf("provider %q invalid model_id: %q", p.ProviderID, m.ModelID)
			}
			if strings.TrimSpace(m.DisplayName) == "" {
				return nil, fmt.Errorf("provider %q model %q display_name cannot be empty", p.ProviderID, m.ModelID)
			}
			if strings.TrimSpace(m.ExternalModelID) == "" {
				return nil, fmt.Errorf("provider %q model %q external_model_id cannot be empty", p.ProviderID, m.ModelID)
			}
			if _, exists := modelsMap[m.ModelID]; exists {
				return nil, fmt.Errorf("provider %q duplicate model_id: %q", p.ProviderID, m.ModelID)
			}
			modelsMap[m.ModelID] = m
			validatedModels = append(validatedModels, m)
		}

		p.Models = validatedModels
		byProviderID[p.ProviderID] = p
		byModelID[p.ProviderID] = modelsMap
		validatedProviders = append(validatedProviders, p)
	}

	sort.Slice(validatedProviders, func(i, j int) bool {
		return validatedProviders[i].ProviderID < validatedProviders[j].ProviderID
	})

	return &Catalog{
		providers:    validatedProviders,
		byProviderID: byProviderID,
		byModelID:    byModelID,
	}, nil
}

func (c *Catalog) Providers() []ProviderDefinition {
	result := make([]ProviderDefinition, len(c.providers))
	copy(result, c.providers)
	return result
}

func (c *Catalog) GetProvider(providerID providers.ProviderID) (ProviderDefinition, bool) {
	p, ok := c.byProviderID[providerID]
	return p, ok
}

func (c *Catalog) GetModel(providerID providers.ProviderID, modelID providers.ModelID) (ModelDefinition, bool) {
	models, ok := c.byModelID[providerID]
	if !ok {
		return ModelDefinition{}, false
	}
	m, ok := models[modelID]
	return m, ok
}

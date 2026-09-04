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

// Capability represents a provider capability.
type Capability string

const (
	CapabilityText  Capability = "text"
	CapabilityImage Capability = "image"
	CapabilityVideo Capability = "video"
	CapabilityTTS   Capability = "tts"
)

// VoiceDefinition specifies a TTS voice.
type VoiceDefinition struct {
	VoiceID       providers.VoiceID `json:"voice_id"`
	DisplayName   string            `json:"display_name"`
	ExternalVoice string            `json:"external_voice"`
}

// ModelDefinition specifies the metadata and external upstream model mapping.
type ModelDefinition struct {
	ModelID         providers.ModelID `json:"model_id"`
	DisplayName     string            `json:"display_name"`
	ExternalModelID string            `json:"external_model_id"`
	Capabilities    []Capability      `json:"capabilities"`
}

// ProviderDefinition contains non-secret deployment configuration for a provider.
type ProviderDefinition struct {
	ProviderID       providers.ProviderID `json:"provider_id"`
	DisplayName      string               `json:"display_name"`
	BaseURL          string               `json:"base_url"`
	Models           []ModelDefinition    `json:"models"`
	Voices           []VoiceDefinition    `json:"voices,omitempty"`
	Timeout          time.Duration        `json:"timeout,omitempty"`
	MaxResponseBytes int64                `json:"max_response_bytes,omitempty"`
}

// Catalog holds validated deployment provider definitions.
type Catalog struct {
	providers    []ProviderDefinition
	byProviderID map[providers.ProviderID]ProviderDefinition
	byModelID    map[providers.ProviderID]map[providers.ModelID]ModelDefinition
	byVoiceID    map[providers.ProviderID]map[providers.VoiceID]VoiceDefinition
}

// NewCatalogFromJSON parses and validates a JSON array of ProviderDefinitions.
func NewCatalogFromJSON(raw []byte) (*Catalog, error) {
	var defs []ProviderDefinition
	if err := json.Unmarshal(raw, &defs); err != nil {
		return nil, fmt.Errorf("invalid provider definitions json: %w", err)
	}
	return NewCatalog(defs)
}

// LegacyProviderDefinition is the TASK-017 schema without capability field.
type LegacyProviderDefinition struct {
	ProviderID       providers.ProviderID    `json:"provider_id"`
	DisplayName      string                  `json:"display_name"`
	BaseURL          string                  `json:"base_url"`
	Models           []LegacyModelDefinition `json:"models"`
	Voices           []VoiceDefinition       `json:"voices,omitempty"`
	Timeout          time.Duration           `json:"timeout,omitempty"`
	MaxResponseBytes int64                   `json:"max_response_bytes,omitempty"`
}

// LegacyModelDefinition is the TASK-017 schema without capability field.
// Defaults to CapabilityText for backward compatibility.
type LegacyModelDefinition struct {
	ModelID         providers.ModelID `json:"model_id"`
	DisplayName     string            `json:"display_name"`
	ExternalModelID string            `json:"external_model_id"`
}

// NewCatalogFromLegacyJSON parses TASK-017 schema and defaults capabilities to [text].
func NewCatalogFromLegacyJSON(raw []byte) (*Catalog, error) {
	var legacy []LegacyProviderDefinition
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("invalid legacy provider definitions json: %w", err)
	}
	defs := make([]ProviderDefinition, len(legacy))
	for i, lp := range legacy {
		models := make([]ModelDefinition, len(lp.Models))
		for j, lm := range lp.Models {
			models[j] = ModelDefinition{
				ModelID:         lm.ModelID,
				DisplayName:     lm.DisplayName,
				ExternalModelID: lm.ExternalModelID,
				Capabilities:    []Capability{CapabilityText},
			}
		}
		defs[i] = ProviderDefinition{
			ProviderID:       lp.ProviderID,
			DisplayName:      lp.DisplayName,
			BaseURL:          lp.BaseURL,
			Models:           models,
			Voices:           lp.Voices,
			Timeout:          lp.Timeout,
			MaxResponseBytes: lp.MaxResponseBytes,
		}
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
			byVoiceID:    make(map[providers.ProviderID]map[providers.VoiceID]VoiceDefinition),
		}, nil
	}

	byProviderID := make(map[providers.ProviderID]ProviderDefinition, len(defs))
	byModelID := make(map[providers.ProviderID]map[providers.ModelID]ModelDefinition, len(defs))
	byVoiceID := make(map[providers.ProviderID]map[providers.VoiceID]VoiceDefinition, len(defs))
	validatedProviders := make([]ProviderDefinition, 0, len(defs))

	for _, p := range defs {
		if !p.ProviderID.Valid() {
			return nil, fmt.Errorf("invalid provider_id: %q", p.ProviderID)
		}
		if strings.TrimSpace(p.DisplayName) == "" {
			return nil, fmt.Errorf("provider %q display_name cannot be empty", p.ProviderID)
		}
		if p.Timeout < 0 {
			return nil, fmt.Errorf("provider %q timeout cannot be negative", p.ProviderID)
		}
		if p.MaxResponseBytes < 0 {
			return nil, fmt.Errorf("provider %q max_response_bytes cannot be negative", p.ProviderID)
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
			if strings.TrimSpace(m.ExternalModelID) == "" || m.ExternalModelID != strings.TrimSpace(m.ExternalModelID) {
				return nil, fmt.Errorf("provider %q model %q external_model_id is invalid: %q", p.ProviderID, m.ModelID, m.ExternalModelID)
			}
			if len(m.Capabilities) == 0 {
				return nil, fmt.Errorf("provider %q model %q must declare at least one capability", p.ProviderID, m.ModelID)
			}
			seenCaps := make(map[Capability]bool, len(m.Capabilities))
			for _, cap := range m.Capabilities {
				if cap != CapabilityText && cap != CapabilityImage && cap != CapabilityVideo && cap != CapabilityTTS {
					return nil, fmt.Errorf("provider %q model %q has invalid capability %q", p.ProviderID, m.ModelID, cap)
				}
				if seenCaps[cap] {
					return nil, fmt.Errorf("provider %q model %q has duplicate capability %q", p.ProviderID, m.ModelID, cap)
				}
				seenCaps[cap] = true
			}
			if _, exists := modelsMap[m.ModelID]; exists {
				return nil, fmt.Errorf("provider %q duplicate model_id: %q", p.ProviderID, m.ModelID)
			}
			modelsMap[m.ModelID] = m
			validatedModels = append(validatedModels, m)
		}

		voicesMap := make(map[providers.VoiceID]VoiceDefinition, len(p.Voices))
		validatedVoices := make([]VoiceDefinition, 0, len(p.Voices))
		for _, v := range p.Voices {
			if !v.VoiceID.Valid() {
				return nil, fmt.Errorf("provider %q invalid voice_id: %q", p.ProviderID, v.VoiceID)
			}
			if strings.TrimSpace(v.DisplayName) == "" {
				return nil, fmt.Errorf("provider %q voice %q display_name cannot be empty", p.ProviderID, v.VoiceID)
			}
			if strings.TrimSpace(v.ExternalVoice) == "" {
				return nil, fmt.Errorf("provider %q voice %q external_voice cannot be empty", p.ProviderID, v.VoiceID)
			}
			if _, exists := voicesMap[v.VoiceID]; exists {
				return nil, fmt.Errorf("provider %q duplicate voice_id: %q", p.ProviderID, v.VoiceID)
			}
			voicesMap[v.VoiceID] = v
			validatedVoices = append(validatedVoices, v)
		}

		p.Models = validatedModels
		p.Voices = validatedVoices
		byProviderID[p.ProviderID] = p
		byModelID[p.ProviderID] = modelsMap
		byVoiceID[p.ProviderID] = voicesMap
		validatedProviders = append(validatedProviders, p)
	}

	sort.Slice(validatedProviders, func(i, j int) bool {
		return validatedProviders[i].ProviderID < validatedProviders[j].ProviderID
	})

	return &Catalog{
		providers:    validatedProviders,
		byProviderID: byProviderID,
		byModelID:    byModelID,
		byVoiceID:    byVoiceID,
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

func (c *Catalog) GetVoice(providerID providers.ProviderID, voiceID providers.VoiceID) (VoiceDefinition, bool) {
	voices, ok := c.byVoiceID[providerID]
	if !ok {
		return VoiceDefinition{}, false
	}
	v, ok := voices[voiceID]
	return v, ok
}

// ModelsForCapability returns models under provider that advertise the given capability.
func (c *Catalog) ModelsForCapability(providerID providers.ProviderID, cap Capability) []ModelDefinition {
	models, ok := c.byModelID[providerID]
	if !ok {
		return nil
	}
	result := make([]ModelDefinition, 0, len(models))
	for _, m := range models {
		for _, c := range m.Capabilities {
			if c == cap {
				result = append(result, m)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ModelID < result[j].ModelID })
	return result
}

// ModelSupportsCapability reports whether a model advertises the given capability.
func (c *Catalog) ModelSupportsCapability(providerID providers.ProviderID, modelID providers.ModelID, cap Capability) bool {
	m, ok := c.GetModel(providerID, modelID)
	if !ok {
		return false
	}
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

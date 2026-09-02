package providers

// ProviderMetadata describes a provider for capability discovery.
type ProviderMetadata struct {
	ID          ProviderID
	DisplayName string
}

// ModelMetadata describes a model and the capabilities it supports.
type ModelMetadata struct {
	ProviderID            ProviderID
	ID                    ModelID
	DisplayName           string
	SupportedCapabilities []Capability
}

func (m ModelMetadata) Supports(capability Capability) bool {
	for _, supported := range m.SupportedCapabilities {
		if supported == capability {
			return true
		}
	}
	return false
}

// ModelRegistration binds a model to optional capability implementations.
type ModelRegistration struct {
	Metadata          ModelMetadata
	TextGenerator     TextGenerator
	ImageGenerator    ImageGenerator
	VideoGenerator    VideoGenerator
	SpeechSynthesizer SpeechSynthesizer
}

// Registration registers one provider and its models in the catalog.
type Registration struct {
	Provider ProviderMetadata
	Models   []ModelRegistration
}

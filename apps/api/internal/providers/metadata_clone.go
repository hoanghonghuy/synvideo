package providers

// cloneCapabilities returns an independent copy of capability metadata.
func cloneCapabilities(values []Capability) []Capability {
	if len(values) == 0 {
		return nil
	}
	copied := make([]Capability, len(values))
	copy(copied, values)
	return copied
}

func cloneModelMetadata(metadata ModelMetadata) ModelMetadata {
	return ModelMetadata{
		ProviderID:            metadata.ProviderID,
		ID:                    metadata.ID,
		DisplayName:           metadata.DisplayName,
		SupportedCapabilities: cloneCapabilities(metadata.SupportedCapabilities),
	}
}

func cloneProviderMetadata(metadata ProviderMetadata) ProviderMetadata {
	return ProviderMetadata{
		ID:          metadata.ID,
		DisplayName: metadata.DisplayName,
	}
}

func cloneRegistration(registration Registration) Registration {
	cloned := Registration{
		Provider: cloneProviderMetadata(registration.Provider),
		Models:   make([]ModelRegistration, len(registration.Models)),
	}
	for i, model := range registration.Models {
		cloned.Models[i] = ModelRegistration{
			Metadata:          cloneModelMetadata(model.Metadata),
			TextGenerator:     model.TextGenerator,
			ImageGenerator:    model.ImageGenerator,
			VideoGenerator:    model.VideoGenerator,
			SpeechSynthesizer: model.SpeechSynthesizer,
		}
	}
	return cloned
}

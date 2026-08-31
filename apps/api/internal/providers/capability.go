package providers

// Capability identifies a provider/model feature without encoding vendor names.
type Capability string

const (
	CapabilityTextGeneration  Capability = "text_generation"
	CapabilityImageGeneration Capability = "image_generation"
	CapabilityVideoGeneration Capability = "video_generation"
	CapabilityTTS             Capability = "tts"
	CapabilityTranscription   Capability = "transcription"
	CapabilityMusic           Capability = "music"
)

func (c Capability) String() string {
	return string(c)
}

func (c Capability) Valid() bool {
	switch c {
	case CapabilityTextGeneration,
		CapabilityImageGeneration,
		CapabilityVideoGeneration,
		CapabilityTTS,
		CapabilityTranscription,
		CapabilityMusic:
		return true
	default:
		return false
	}
}

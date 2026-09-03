package audio

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported audio format")
)

// MeasureDuration calculates the exact audio duration in seconds from the raw audio bytes.
func MeasureDuration(mimeType string, data []byte) (float64, error) {
	if len(data) == 0 {
		return 0, errors.New("empty audio data")
	}
	switch strings.ToLower(mimeType) {
	case "audio/wav", "audio/x-wav":
		return measureWAVDuration(data)
	case "audio/mpeg", "audio/mp3":
		return measureMP3Duration(data)
	default:
		return 0, fmt.Errorf("%w: %s", ErrUnsupportedFormat, mimeType)
	}
}

// StitchAudio stitches multiple ordered audio chunk bytes into a single valid audio binary
// and measures the total duration in seconds.
func StitchAudio(mimeType string, chunks [][]byte) ([]byte, float64, error) {
	if len(chunks) == 0 {
		return nil, 0, errors.New("no audio chunks provided")
	}
	switch strings.ToLower(mimeType) {
	case "audio/wav", "audio/x-wav":
		return stitchWAV(chunks)
	case "audio/mpeg", "audio/mp3":
		return stitchMP3(chunks)
	default:
		return nil, 0, fmt.Errorf("%w: %s", ErrUnsupportedFormat, mimeType)
	}
}

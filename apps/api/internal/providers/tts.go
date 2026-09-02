package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const MaxSpeechTextRunes = 1_000_000

// VoiceID is a stable internal voice identity, separate from provider voice names.
type VoiceID string

func (id VoiceID) String() string { return string(id) }

func (id VoiceID) Valid() bool { return isStableIdentifier(string(id)) }

// VoiceMetadata contains safe, provider-neutral voice discovery metadata.
type VoiceMetadata struct {
	ID          VoiceID
	DisplayName string
	Locale      string
	Language    string
	Style       string
}

func (m VoiceMetadata) Validate() error {
	if !m.ID.Valid() {
		return errors.New("voice id is invalid")
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		return errors.New("voice display name is required")
	}
	for name, value := range map[string]string{"locale": m.Locale, "language": m.Language, "style": m.Style} {
		if value != "" && value != strings.TrimSpace(value) {
			return fmt.Errorf("voice %s must not have surrounding whitespace", name)
		}
	}
	return nil
}

// AudioFormat is a small provider-neutral output format set for speech.
type AudioFormat string

const (
	AudioFormatMP3 AudioFormat = "mp3"
	AudioFormatWAV AudioFormat = "wav"
)

func (f AudioFormat) Valid() bool {
	switch f {
	case AudioFormatMP3, AudioFormatWAV:
		return true
	default:
		return false
	}
}

func (f AudioFormat) MIMEType() string {
	switch f {
	case AudioFormatMP3:
		return "audio/mpeg"
	case AudioFormatWAV:
		return "audio/wav"
	default:
		return ""
	}
}

// SpeechSynthesisRequest contains exact provider-neutral narration input.
type SpeechSynthesisRequest struct {
	Text    string
	VoiceID VoiceID
	Locale  string
	Format  AudioFormat
}

func (r SpeechSynthesisRequest) Validate() error {
	if r.Text == "" {
		return NewInvalidRequestError(errors.New("speech text is required"))
	}
	if !utf8.ValidString(r.Text) {
		return NewInvalidRequestError(errors.New("speech text must be valid UTF-8"))
	}
	if utf8.RuneCountInString(r.Text) > MaxSpeechTextRunes {
		return NewInvalidRequestError(errors.New("speech text exceeds core bound"))
	}
	if !r.VoiceID.Valid() {
		return NewInvalidRequestError(errors.New("voice id is invalid"))
	}
	if r.Locale != "" && r.Locale != strings.TrimSpace(r.Locale) {
		return NewInvalidRequestError(errors.New("speech locale must not have surrounding whitespace"))
	}
	if r.Format != "" && !r.Format.Valid() {
		return NewInvalidRequestError(errors.New("speech format is unsupported"))
	}
	return nil
}

// GeneratedAudio is a provider-neutral streaming generated audio body.
type GeneratedAudio interface {
	MIMEType() string
	// Size returns the known byte size, or -1 when the size is unknown.
	Size() int64
	Open(context.Context) (io.ReadCloser, error)
}

// NewGeneratedAudio creates an immutable in-memory generated audio body.
func NewGeneratedAudio(mime string, data []byte) (GeneratedAudio, error) {
	if !validAudioMIME(mime) {
		return nil, NewInvalidRequestError(errors.New("unsupported generated audio MIME type"))
	}
	return &memoryBinary{mime: mime, data: append([]byte(nil), data...)}, nil
}

// ValidateAudioBinary validates a generated audio body at the provider boundary.
func ValidateAudioBinary(audio GeneratedAudio) error {
	if audio == nil || !validAudioMIME(audio.MIMEType()) || audio.Size() < -1 {
		return NewMalformedResponseError(errors.New("audio output binary is invalid"))
	}
	return nil
}

// SpeechSynthesisResponse contains generated audio and safe selected identities.
type SpeechSynthesisResponse struct {
	Audio   GeneratedAudio
	ModelID ModelID
	Voice   VoiceMetadata
	Usage   UsageMetadata
}

func (r SpeechSynthesisResponse) Validate() error {
	if err := ValidateAudioBinary(r.Audio); err != nil {
		return err
	}
	if r.ModelID != "" && !r.ModelID.Valid() {
		return NewMalformedResponseError(errors.New("speech model id is invalid"))
	}
	if err := r.Voice.Validate(); err != nil {
		return NewMalformedResponseError(err)
	}
	return nil
}

// SpeechSynthesizer is the provider-neutral text-to-speech boundary.
type SpeechSynthesizer interface {
	SynthesizeSpeech(context.Context, SpeechSynthesisRequest) (SpeechSynthesisResponse, error)
}

func validAudioMIME(mime string) bool {
	switch mime {
	case "audio/mpeg", "audio/wav":
		return true
	default:
		return false
	}
}

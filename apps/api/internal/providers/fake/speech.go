package fake

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

// SpeechSynthesizer is a deterministic fake TTS provider for tests.
type SpeechSynthesizer struct {
	audio  []byte
	mime   string
	err    error
	delay  time.Duration
	voices map[providers.VoiceID]providers.VoiceMetadata

	mu       sync.Mutex
	requests []providers.SpeechSynthesisRequest
}

func NewSpeechSynthesizer(audio []byte) *SpeechSynthesizer {
	return &SpeechSynthesizer{audio: append([]byte(nil), audio...), mime: "audio/mpeg", voices: make(map[providers.VoiceID]providers.VoiceMetadata)}
}

func (s *SpeechSynthesizer) WithVoice(voice providers.VoiceMetadata) *SpeechSynthesizer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.voices[voice.ID] = voice
	return s
}

func (s *SpeechSynthesizer) WithVoices(voices ...providers.VoiceMetadata) *SpeechSynthesizer {
	for _, voice := range voices {
		s.WithVoice(voice)
	}
	return s
}

func (s *SpeechSynthesizer) WithMIMEType(mime string) *SpeechSynthesizer {
	s.mime = mime
	return s
}

func (s *SpeechSynthesizer) WithError(err error) *SpeechSynthesizer {
	s.err = err
	return s
}

func (s *SpeechSynthesizer) WithDelay(delay time.Duration) *SpeechSynthesizer {
	s.delay = delay
	return s
}

func (s *SpeechSynthesizer) Requests() []providers.SpeechSynthesisRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]providers.SpeechSynthesisRequest(nil), s.requests...)
}

func (s *SpeechSynthesizer) SynthesizeSpeech(ctx context.Context, req providers.SpeechSynthesisRequest) (providers.SpeechSynthesisResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	if s.delay > 0 {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return providers.SpeechSynthesisResponse{}, ctx.Err()
		case <-timer.C:
		}
	}

	s.mu.Lock()
	voice, configured := s.voices[req.VoiceID]
	if len(s.voices) > 0 && !configured {
		s.mu.Unlock()
		return providers.SpeechSynthesisResponse{}, providers.NewInvalidRequestError(errors.New("voice is not configured"))
	}
	if !configured {
		voice = providers.VoiceMetadata{ID: req.VoiceID, DisplayName: req.VoiceID.String(), Locale: req.Locale}
	}
	s.requests = append(s.requests, req)
	err := s.err
	audio := append([]byte(nil), s.audio...)
	mime := s.mime
	s.mu.Unlock()

	if err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	generated, err := providers.NewGeneratedAudio(mime, audio)
	if err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	return providers.SpeechSynthesisResponse{Audio: generated, Voice: voice}, nil
}

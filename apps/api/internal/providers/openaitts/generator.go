package openaitts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	errTransport       = errors.New("upstream provider transport failure")
	errResponseTooLong = errors.New("upstream provider response exceeded configured limit")
	errEmptyResponse   = errors.New("upstream provider response was empty")
	errWrongMIME       = errors.New("upstream provider returned an unexpected audio MIME type")
	errUnknownVoice    = errors.New("configured voice is unavailable")
)

type modelSynthesizer struct {
	adapter *Adapter
	model   modelConfig
}

type speechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

// SynthesizeSpeech sends one bounded, context-aware Speech API request.
func (s *modelSynthesizer) SynthesizeSpeech(ctx context.Context, req providers.SpeechSynthesisRequest) (providers.SpeechSynthesisResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	if utf8.RuneCountInString(req.Text) > s.adapter.maxInputRunes || (s.adapter.maxInputBytes > 0 && int64(len(req.Text)) > s.adapter.maxInputBytes) {
		return providers.SpeechSynthesisResponse{}, providers.NewSpeechInputTooLongError()
	}

	voice, ok := s.adapter.voices[req.VoiceID]
	if !ok {
		return providers.SpeechSynthesisResponse{}, providers.NewInvalidRequestError(errUnknownVoice)
	}
	format := req.Format
	if format == "" {
		format = s.adapter.defaultFormat
	}
	if _, ok := s.adapter.formats[format]; !ok {
		return providers.SpeechSynthesisResponse{}, providers.NewInvalidRequestError(errors.New("requested speech format is not configured"))
	}

	body, err := json.Marshal(speechRequest{
		Model: s.model.externalModel, Input: req.Text, Voice: voice.externalVoice, ResponseFormat: string(format),
	})
	if err != nil {
		return providers.SpeechSynthesisResponse{}, providers.NewInvalidRequestError(err)
	}
	apiKey, err := s.adapter.credentialSource.APIKey(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.SpeechSynthesisResponse{}, ctxErr
		}
		return providers.SpeechSynthesisResponse{}, providers.NewAuthConfigError(err)
	}
	if apiKey == "" || apiKey != strings.TrimSpace(apiKey) {
		return providers.SpeechSynthesisResponse{}, providers.NewAuthConfigError(errCredentialUnavailable)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.adapter.endpoint, bytes.NewReader(body))
	if err != nil {
		return providers.SpeechSynthesisResponse{}, providers.NewUnavailableError(errTransport)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := s.adapter.client.Do(httpRequest)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.SpeechSynthesisResponse{}, ctxErr
		}
		return providers.SpeechSynthesisResponse{}, providers.NewUnavailableError(errTransport)
	}

	switch {
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewAuthConfigError(upstreamStatus(response.StatusCode))
	case response.StatusCode == http.StatusTooManyRequests:
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewRateLimitedError(upstreamStatus(response.StatusCode))
	case response.StatusCode >= http.StatusInternalServerError:
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewTransientError(upstreamStatus(response.StatusCode))
	case response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices:
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewInvalidRequestError(upstreamStatus(response.StatusCode))
	}

	canonicalMIME, ok := canonicalAudioMIME(response.Header.Get("Content-Type"))
	if !ok || canonicalMIME != format.MIMEType() {
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewMalformedResponseError(errWrongMIME)
	}
	if response.ContentLength > s.adapter.maxResponseBytes {
		response.Body.Close()
		return providers.SpeechSynthesisResponse{}, providers.NewMalformedResponseError(errResponseTooLong)
	}

	audio := &streamingAudio{
		body: response.Body, mime: canonicalMIME, size: response.ContentLength,
		maxBytes: s.adapter.maxResponseBytes, ctx: ctx,
	}
	return providers.SpeechSynthesisResponse{
		Audio: audio, ModelID: s.model.metadata.ID, Voice: voice.metadata,
	}, nil
}

type streamingAudio struct {
	mu       sync.Mutex
	body     io.ReadCloser
	mime     string
	size     int64
	maxBytes int64
	ctx      context.Context
	opened   bool
}

func (a *streamingAudio) MIMEType() string { return a.mime }

func (a *streamingAudio) Size() int64 { return a.size }

func (a *streamingAudio) Open(ctx context.Context) (io.ReadCloser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.opened {
		return nil, errors.New("generated audio stream already opened")
	}
	a.opened = true
	requestContext := a.ctx
	if requestContext == nil {
		requestContext = context.Background()
	}
	readContext, cancel := context.WithCancel(requestContext)
	done := make(chan struct{})
	closer := &audioBodyCloser{body: a.body}
	go func() {
		select {
		case <-ctx.Done():
			cancel()
			_ = closer.Close()
		case <-readContext.Done():
			_ = closer.Close()
		case <-done:
		}
	}()
	return &boundedAudioReadCloser{
		body: a.body, closer: closer, ctx: readContext, maxBytes: a.maxBytes, cancel: cancel, done: done,
	}, nil
}

type audioBodyCloser struct {
	body io.ReadCloser
	once sync.Once
}

func (c *audioBodyCloser) Close() error {
	var err error
	c.once.Do(func() { err = c.body.Close() })
	return err
}

type boundedAudioReadCloser struct {
	body     io.ReadCloser
	closer   *audioBodyCloser
	ctx      context.Context
	maxBytes int64
	read     int64
	closed   atomic.Bool
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
}

func (r *boundedAudioReadCloser) Read(p []byte) (int, error) {
	if r.closed.Load() {
		return 0, errors.New("audio stream is closed")
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	remaining := r.maxBytes - r.read
	if remaining <= 0 {
		var probe [1]byte
		n, err := r.body.Read(probe[:])
		if n > 0 {
			return 0, providers.NewMalformedResponseError(errResponseTooLong)
		}
		return 0, err
	}
	allowed := int64(len(p))
	if allowed > remaining+1 {
		allowed = remaining + 1
	}
	n, err := r.body.Read(p[:allowed])
	if int64(n) > remaining {
		r.read = r.maxBytes
		return int(remaining), providers.NewMalformedResponseError(errResponseTooLong)
	}
	r.read += int64(n)
	if n == 0 && err == io.EOF && r.read == 0 {
		return 0, providers.NewMalformedResponseError(errEmptyResponse)
	}
	return n, err
}

func (r *boundedAudioReadCloser) Close() error {
	var err error
	r.once.Do(func() {
		r.closed.Store(true)
		r.cancel()
		close(r.done)
		err = r.closer.Close()
	})
	return err
}

func canonicalAudioMIME(value string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(mediaType) {
	case "audio/mpeg", "audio/mp3":
		return "audio/mpeg", true
	case "audio/wav", "audio/x-wav":
		return "audio/wav", true
	default:
		return "", false
	}
}

func upstreamStatus(status int) error { return fmt.Errorf("upstream status %d", status) }

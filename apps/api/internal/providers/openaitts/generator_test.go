package openaitts_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/openaitts"
)

func TestSynthesizeSpeechMapsExactModelVoiceTextAndStreamsAudio(t *testing.T) {
	var received struct {
		Model  string `json:"model"`
		Input  string `json:"input"`
		Voice  string `json:"voice"`
		Format string `json:"response_format"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" || r.Header.Get("Authorization") != "Bearer exact-key" {
			t.Fatalf("request endpoint/auth = %s / %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, "audio-bytes")
	}))
	defer server.Close()

	config := validConfig()
	config.BaseURL = server.URL + "/v1"
	config.CredentialSource = openaitts.SecretSourceFunc(func(context.Context) (string, error) { return "exact-key", nil })
	adapter, err := openaitts.New(config)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	synth, err := adapter.ForModel("speech")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	text := "  exact narration — giữ nguyên\n"
	response, err := synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: text, VoiceID: "narrator"})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if received.Model != "gpt-4o-mini-tts" || received.Input != text || received.Voice != "alloy" || received.Format != "mp3" {
		t.Fatalf("upstream request = %#v", received)
	}
	stream, err := response.Audio.Open(context.Background())
	if err != nil {
		t.Fatalf("open response audio: %v", err)
	}
	data, err := io.ReadAll(stream)
	stream.Close()
	if err != nil || string(data) != "audio-bytes" {
		t.Fatalf("audio = %q, err = %v", data, err)
	}
}

func TestSynthesizeSpeechRejectsTooLongBeforeCredentialOrNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	config := validConfig()
	config.BaseURL = server.URL + "/v1"
	config.MaxInputRunes = 4
	config.CredentialSource = openaitts.SecretSourceFunc(func(context.Context) (string, error) {
		t.Fatal("credential source must not be called")
		return "", nil
	})
	adapter, err := openaitts.New(config)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	synth, err := adapter.ForModel("speech")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	tooLong := "12345"
	_, err = synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: tooLong, VoiceID: "narrator"})
	if !errors.Is(err, providers.ErrSpeechInputTooLong) || strings.Contains(err.Error(), tooLong) {
		t.Fatalf("too-long error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("too-long input made a network request")
	}
}

func TestSynthesizeSpeechMapsStatusesAndMalformedSuccessSafely(t *testing.T) {
	secret := "upstream-secret-sentinel"
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: secret, want: providers.ErrAuthenticationUnavailable},
		{name: "forbidden", status: http.StatusForbidden, body: secret, want: providers.ErrAuthenticationUnavailable},
		{name: "rate limited", status: http.StatusTooManyRequests, body: secret, want: providers.ErrRateLimited},
		{name: "server", status: http.StatusBadGateway, body: secret, want: providers.ErrTransientExecution},
		{name: "bad request", status: http.StatusBadRequest, body: secret, want: providers.ErrInvalidRequest},
		{name: "wrong mime", status: http.StatusOK, body: secret, want: providers.ErrMalformedResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Header().Set("Content-Type", "application/json")
				if tc.status == http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			config := validConfig()
			config.BaseURL = server.URL + "/v1"
			adapter, err := openaitts.New(config)
			if err != nil {
				t.Fatalf("new adapter: %v", err)
			}
			synth, err := adapter.ForModel("speech")
			if err != nil {
				t.Fatalf("bind model: %v", err)
			}
			_, err = synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: "hello", VoiceID: "narrator"})
			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), secret) {
				t.Fatalf("error = %v, want safe %v", err, tc.want)
			}
		})
	}
}

func TestSynthesizeSpeechDoesNotFollowRedirectOrLeakCredential(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL+"/v1/audio/speech")
		w.WriteHeader(http.StatusFound)
	}))
	defer source.Close()
	config := validConfig()
	config.BaseURL = source.URL + "/v1"
	config.CredentialSource = openaitts.SecretSourceFunc(func(context.Context) (string, error) { return "secret-key", nil })
	adapter, err := openaitts.New(config)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	synth, err := adapter.ForModel("speech")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	_, err = synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: "hello", VoiceID: "narrator"})
	if err == nil || redirected.Load() != 0 || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("redirect result = %v, target calls = %d", err, redirected.Load())
	}
}

func TestSynthesizeSpeechBoundsStreamingResponseAndRejectsEmptyBody(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		contentLen string
	}{
		{name: "oversize content length", body: "123456", contentLen: "6"},
		{name: "empty body", body: "", contentLen: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "audio/mpeg")
				w.Header().Set("Content-Length", tc.contentLen)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			config := validConfig()
			config.BaseURL = server.URL + "/v1"
			config.MaxResponseBytes = 5
			adapter, err := openaitts.New(config)
			if err != nil {
				t.Fatalf("new adapter: %v", err)
			}
			synth, err := adapter.ForModel("speech")
			if err != nil {
				t.Fatalf("bind model: %v", err)
			}
			response, err := synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: "hello", VoiceID: "narrator"})
			if tc.name == "oversize content length" {
				if !errors.Is(err, providers.ErrMalformedResponse) {
					t.Fatalf("oversize error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("empty response synthesize: %v", err)
			}
			stream, err := response.Audio.Open(context.Background())
			if err != nil {
				t.Fatalf("open empty response: %v", err)
			}
			_, err = io.ReadAll(stream)
			stream.Close()
			if !errors.Is(err, providers.ErrMalformedResponse) {
				t.Fatalf("empty response read error = %v", err)
			}
		})
	}
}

func TestStreamingAudioOpenContextCancelsUnderlyingBody(t *testing.T) {
	body := &blockingBody{closed: make(chan struct{})}
	config := validConfig()
	config.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
			Body:       body,
		}, nil
	})}
	adapter, err := openaitts.New(config)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	synth, err := adapter.ForModel("speech")
	if err != nil {
		t.Fatalf("bind model: %v", err)
	}
	response, err := synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: "hello", VoiceID: "narrator"})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := response.Audio.Open(ctx)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	readDone := make(chan error, 1)
	go func() { _, readErr := stream.Read(make([]byte, 4)); readDone <- readErr }()
	cancel()
	select {
	case err := <-readDone:
		if !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("read cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stream read did not stop after cancellation")
	}
	stream.Close()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type blockingBody struct {
	closed    chan struct{}
	closeOnce sync.Once
}

func (b *blockingBody) Read([]byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

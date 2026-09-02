package providers_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

func TestSpeechRequestPreservesExactNarrationAndValidatesVoiceAndFormat(t *testing.T) {
	request := providers.SpeechSynthesisRequest{
		Text:    "  Xin chào, 世界\n  ",
		VoiceID: "narrator",
		Format:  providers.AudioFormatWAV,
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if request.Text != "  Xin chào, 世界\n  " {
		t.Fatalf("request text was changed: %q", request.Text)
	}

	for _, voice := range []providers.VoiceID{"", " narrator", "NARRATOR", "narrator/1"} {
		if voice.Valid() {
			t.Errorf("voice %q should be invalid", voice)
		}
	}
	for _, format := range []providers.AudioFormat{"", "mp3", "wav"} {
		request.Format = format
		if err := request.Validate(); format == "" && err != nil {
			t.Fatalf("empty format should select adapter default: %v", err)
		} else if format != "" && err != nil {
			t.Fatalf("format %q should be valid: %v", format, err)
		}
	}
}

func TestGeneratedAudioIsClosableAndContextAware(t *testing.T) {
	audio, err := providers.NewGeneratedAudio("audio/wav", []byte("audio"))
	if err != nil {
		t.Fatalf("new audio: %v", err)
	}
	stream, err := audio.Open(context.Background())
	if err != nil {
		t.Fatalf("open audio: %v", err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}
	if string(data) != "audio" {
		t.Fatalf("audio = %q, want audio", data)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := audio.Open(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled open error = %v, want context.Canceled", err)
	}
}

func TestSpeechRequestRejectsInvalidUTF8AndEmptyText(t *testing.T) {
	for _, text := range []string{"", string([]byte{0xff})} {
		request := providers.SpeechSynthesisRequest{Text: text, VoiceID: "narrator"}
		if err := request.Validate(); err == nil {
			t.Errorf("text %q should be rejected", text)
		}
	}
	if err := (providers.SpeechSynthesisRequest{Text: "ok", VoiceID: "narrator", Format: "ogg"}).Validate(); err == nil {
		t.Fatal("unsupported format should be rejected")
	}
}

func TestSpeechResponseRejectsInvalidAudio(t *testing.T) {
	if err := (providers.SpeechSynthesisResponse{}).Validate(); err == nil {
		t.Fatal("missing audio should be rejected")
	}
	if _, err := providers.NewGeneratedAudio("application/octet-stream", []byte("secret")); err == nil {
		t.Fatal("non-audio MIME should be rejected")
	}
}

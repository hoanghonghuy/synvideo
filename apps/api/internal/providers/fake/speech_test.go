package fake_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestSpeechSynthesizerCapturesExactTextAndDeterministicAudio(t *testing.T) {
	voice := providers.VoiceMetadata{ID: "narrator", DisplayName: "Narrator", Locale: "en-US"}
	synth := fake.NewSpeechSynthesizer([]byte("wav-data")).WithVoice(voice).WithMIMEType("audio/wav")
	request := providers.SpeechSynthesisRequest{Text: "  exact narration\n", VoiceID: voice.ID, Format: providers.AudioFormatWAV}
	response, err := synth.SynthesizeSpeech(context.Background(), request)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	stream, err := response.Audio.Open(context.Background())
	if err != nil {
		t.Fatalf("open audio: %v", err)
	}
	data, err := io.ReadAll(stream)
	stream.Close()
	if err != nil || string(data) != "wav-data" {
		t.Fatalf("audio = %q, err = %v", data, err)
	}
	requests := synth.Requests()
	if len(requests) != 1 || requests[0].Text != request.Text {
		t.Fatalf("captured requests = %#v", requests)
	}
	request.Text = "mutated"
	if synth.Requests()[0].Text != "  exact narration\n" {
		t.Fatal("captured request was not cloned")
	}
}

func TestSpeechSynthesizerPropagatesConfiguredErrorAndCancellation(t *testing.T) {
	voice := providers.VoiceMetadata{ID: "narrator", DisplayName: "Narrator"}
	want := errors.New("configured failure")
	synth := fake.NewSpeechSynthesizer([]byte("audio")).WithVoice(voice).WithError(want)
	_, err := synth.SynthesizeSpeech(context.Background(), providers.SpeechSynthesisRequest{Text: "hello", VoiceID: voice.ID})
	if !errors.Is(err, want) {
		t.Fatalf("configured error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = fake.NewSpeechSynthesizer([]byte("audio")).WithVoice(voice).SynthesizeSpeech(ctx, providers.SpeechSynthesisRequest{Text: "hello", VoiceID: voice.ID})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}

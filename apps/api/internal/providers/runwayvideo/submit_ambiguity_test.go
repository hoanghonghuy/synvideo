package runwayvideo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

func TestStartVideoHTTP5xxIsAmbiguousSubmit(t *testing.T) {
	adapter := newTestAdapter(t, "https://runway.invalid", roundTripClient(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioNopCloser{Reader: strings.NewReader(`{"error":"upstream failure"}`)},
			Header:     make(http.Header),
		}, nil
	}))

	_, err := adapter.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "shot", AspectRatio: "16:9"})
	if !errors.Is(err, providers.ErrAmbiguousSubmit) {
		t.Fatalf("expected ambiguous submit for submit HTTP 5xx, got %v", err)
	}
}

type ioNopCloser struct {
	*strings.Reader
}

func (ioNopCloser) Close() error { return nil }

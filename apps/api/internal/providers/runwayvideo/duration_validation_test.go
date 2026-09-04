package runwayvideo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

func TestStartVideoRejectsUnsupportedDurationBeforeSubmit(t *testing.T) {
	calls := 0
	adapter := newTestAdapter(t, "https://runway.invalid", roundTripClient(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected submit")
	}))

	for _, duration := range []int{MinDurationSeconds - 1, MaxDurationSeconds + 1} {
		_, err := adapter.StartVideo(context.Background(), providers.VideoGenerationRequest{
			Prompt:          "shot",
			AspectRatio:     "16:9",
			DurationSeconds: &duration,
		})
		if !errors.Is(err, providers.ErrInvalidRequest) {
			t.Fatalf("duration=%d expected invalid request, got %v", duration, err)
		}
	}
	if calls != 0 {
		t.Fatalf("unsupported durations reached paid submit path: calls=%d", calls)
	}
}

func TestStartVideoAcceptsRunwayDurationBounds(t *testing.T) {
	for _, duration := range []int{MinDurationSeconds, MaxDurationSeconds} {
		serverCalls := 0
		adapter := newTestAdapter(t, "https://runway.invalid", roundTripClient(func(*http.Request) (*http.Response, error) {
			serverCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioNopCloser{Reader: strings.NewReader(`{"id":"task-123"}`)},
				Header:     make(http.Header),
			}, nil
		}))
		_, err := adapter.StartVideo(context.Background(), providers.VideoGenerationRequest{
			Prompt:          "shot",
			AspectRatio:     "16:9",
			DurationSeconds: &duration,
		})
		if err != nil {
			t.Fatalf("duration=%d unexpected error: %v", duration, err)
		}
		if serverCalls != 1 {
			t.Fatalf("duration=%d submit calls=%d", duration, serverCalls)
		}
	}
}

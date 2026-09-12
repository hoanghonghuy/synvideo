package publishing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestYouTubeStatusClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("authorization = %q", got)
		}
		if got := r.URL.Query().Get("id"); got != "video-123" {
			t.Errorf("id = %q", got)
		}
		if got := r.URL.Query().Get("part"); got != "status,processingDetails" {
			t.Errorf("part = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"video-123","status":{"privacyStatus":"private","publishAt":"2026-09-12T04:00:00Z"},"processingDetails":{"processingStatus":"succeeded"}}]}`))
	}))
	defer server.Close()

	client, err := NewYouTubeStatusClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Get(context.Background(), "access-token", "video-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProcessingStatus != "succeeded" || result.PrivacyStatus != "private" {
		t.Fatalf("unexpected status: %+v", result)
	}
	wantPublishAt := time.Date(2026, 9, 12, 4, 0, 0, 0, time.UTC)
	if result.PublishAt == nil || !result.PublishAt.Equal(wantPublishAt) {
		t.Fatalf("publishAt = %v", result.PublishAt)
	}
}

func TestYouTubeStatusClientClassifiesFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		failure    RemoteStatusFailure
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, failure: RemoteStatusFailureReconnectRequired},
		{name: "forbidden", statusCode: http.StatusForbidden, failure: RemoteStatusFailureReconnectRequired},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, failure: RemoteStatusFailureRetryable},
		{name: "server error", statusCode: http.StatusServiceUnavailable, failure: RemoteStatusFailureRetryable},
		{name: "not found", statusCode: http.StatusNotFound, failure: RemoteStatusFailureRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()
			client, err := NewYouTubeStatusClient(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Get(context.Background(), "access-token", "video-123")
			if err != nil {
				t.Fatal(err)
			}
			if result.Failure != tt.failure {
				t.Fatalf("failure = %q, want %q", result.Failure, tt.failure)
			}
		})
	}
}

func TestYouTubeStatusClientRejectsMissingVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	client, err := NewYouTubeStatusClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Get(context.Background(), "access-token", "video-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Failure != RemoteStatusFailureRejected || result.ProviderCode != http.StatusNotFound {
		t.Fatalf("unexpected result: %+v", result)
	}
}

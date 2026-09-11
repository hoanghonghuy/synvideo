package publishing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestYouTubeResumableInitiateUsesPrivateUploadAndReturnsSession(t *testing.T) {
	var gotAuthorization string
	var gotContentLength string
	var gotContentType string
	var gotPart string
	var gotUploadType string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotContentLength = r.Header.Get("X-Upload-Content-Length")
		gotContentType = r.Header.Get("X-Upload-Content-Type")
		gotPart = r.URL.Query().Get("part")
		gotUploadType = r.URL.Query().Get("uploadType")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Location", serverURL(r)+"/session/abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewYouTubeResumableClient(server.URL+"/upload", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Initiate(context.Background(), "access-secret", YouTubeUploadMetadata{Title: "Launch clip", Description: "description"}, "video/mp4", 2048)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionURI == "" || result.Failure != UploadFailureNone {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotAuthorization != "Bearer access-secret" || gotContentLength != "2048" || gotContentType != "video/mp4" {
		t.Fatalf("unexpected upload headers: auth=%q length=%q type=%q", gotAuthorization, gotContentLength, gotContentType)
	}
	if gotPart != "snippet,status" || gotUploadType != "resumable" {
		t.Fatalf("unexpected query: part=%q uploadType=%q", gotPart, gotUploadType)
	}
	if !strings.Contains(gotBody, `"privacyStatus":"private"`) || !strings.Contains(gotBody, `"title":"Launch clip"`) {
		t.Fatalf("unexpected metadata body: %s", gotBody)
	}
})

func TestYouTubeResumableQueryUsesProviderRangeAsSourceOfTruth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("Content-Range") != "bytes */2000000" || r.ContentLength != 0 {
			t.Errorf("unexpected status request: method=%s range=%q length=%d", r.Method, r.Header.Get("Content-Range"), r.ContentLength)
		}
		w.Header().Set("Range", "bytes=0-999999")
		w.Header().Set("Retry-After", "7")
		w.Header().Set("Location", serverURL(r)+"/session/rotated")
		w.WriteHeader(308)
	}))
	defer server.Close()

	client, _ := NewYouTubeResumableClient(server.URL, server.Client())
	result, err := client.Query(context.Background(), "token", server.URL+"/session/original", 2000000)
	if err != nil {
		t.Fatal(err)
	}
	if result.UploadedBytes != 1000000 || result.RetryAfter != 7*time.Second || !strings.HasSuffix(result.SessionURI, "/session/rotated") {
		t.Fatalf("unexpected recovery result: %+v", result)
	}
})

func TestYouTubeResumableQueryWithNoRangeResumesFromZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(308)
	}))
	defer server.Close()

	client, _ := NewYouTubeResumableClient(server.URL, server.Client())
	result, err := client.Query(context.Background(), "token", server.URL+"/session", 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.UploadedBytes != 0 || result.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
})

func TestYouTubeResumableUploadChunkSendsExactRangeAndCompletes(t *testing.T) {
	payload := []byte("world")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Range") != "bytes 5-9/10" || r.ContentLength != 5 || r.Header.Get("Content-Type") != "video/mp4" {
			t.Errorf("unexpected chunk headers: range=%q length=%d type=%q", r.Header.Get("Content-Range"), r.ContentLength, r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, payload) {
			t.Errorf("unexpected chunk body: %q", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"video-123"}`))
	}))
	defer server.Close()

	client, _ := NewYouTubeResumableClient(server.URL, server.Client())
	result, err := client.UploadChunk(context.Background(), "token", server.URL+"/session", "video/mp4", bytes.NewReader(payload), 5, 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Complete || result.RemoteVideoID != "video-123" || result.UploadedBytes != 10 {
		t.Fatalf("unexpected completion: %+v", result)
	}
})

func TestYouTubeResumableClassifiesRecoveryFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   UploadFailure
	}{
		{name: "auth", status: http.StatusUnauthorized, want: UploadFailureReconnectRequired},
		{name: "expired", status: http.StatusNotFound, want: UploadFailureSessionExpired},
		{name: "gone", status: http.StatusGone, want: UploadFailureSessionExpired},
		{name: "internal", status: http.StatusInternalServerError, want: UploadFailureRetryable},
		{name: "bad gateway", status: http.StatusBadGateway, want: UploadFailureRetryable},
		{name: "unavailable", status: http.StatusServiceUnavailable, want: UploadFailureRetryable},
		{name: "gateway timeout", status: http.StatusGatewayTimeout, want: UploadFailureRetryable},
		{name: "bad request", status: http.StatusBadRequest, want: UploadFailureRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			client, _ := NewYouTubeResumableClient(server.URL, server.Client())
			result, err := client.Query(context.Background(), "token", server.URL+"/session", 10)
			if err != nil {
				t.Fatal(err)
			}
			if result.Failure != tt.want || result.ProviderCode != tt.status {
				t.Fatalf("status %d: got %+v want failure %q", tt.status, result, tt.want)
			}
		})
	}
}

func TestYouTubeResumableRejectsInvalidProviderRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Range", "bytes=5-20")
		w.WriteHeader(308)
	}))
	defer server.Close()

	client, _ := NewYouTubeResumableClient(server.URL, server.Client())
	_, err := client.Query(context.Background(), "token", server.URL+"/session", 100)
	if !errorsIs(err, ErrResumableProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

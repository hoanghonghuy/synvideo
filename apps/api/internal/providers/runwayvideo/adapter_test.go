package runwayvideo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

func TestStartVideoUsesTextToVideoModeAndReturnsTaskID(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/image_to_video" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-Runway-Version") != APIVersion {
			t.Fatalf("missing auth/version headers")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-123"})
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL, server.Client())
	duration := 5
	op, err := adapter.StartVideo(context.Background(), providers.VideoGenerationRequest{
		Prompt: "A slow cinematic push through mist",
		AspectRatio: "16:9",
		DurationSeconds: &duration,
	})
	if err != nil {
		t.Fatal(err)
	}
	if op.ID != "task-123" || op.State != providers.VideoOperationQueued {
		t.Fatalf("operation=%#v", op)
	}
	if got["model"] != "gen4.5" || got["promptText"] == "" || got["ratio"] != "1280:720" {
		t.Fatalf("payload=%#v", got)
	}
	if _, exists := got["promptImage"]; exists {
		t.Fatal("text-to-video request must omit promptImage")
	}
}

func TestStartVideoTransportFailureIsAmbiguous(t *testing.T) {
	adapter := newTestAdapter(t, "https://runway.invalid", roundTripClient(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset after write")
	}))
	_, err := adapter.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "shot", AspectRatio: "16:9"})
	if !errors.Is(err, providers.ErrAmbiguousSubmit) {
		t.Fatalf("expected ambiguous submit, got %v", err)
	}
}

func TestGetOperationMapsRunwayLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "task-123", "status": "THROTTLED", "createdAt": "2026-09-03T00:00:00Z",
		})
	}))
	defer server.Close()
	adapter := newTestAdapter(t, server.URL, server.Client())
	op, err := adapter.GetVideoOperation(context.Background(), "task-123")
	if err != nil {
		t.Fatal(err)
	}
	if op.State != providers.VideoOperationQueued {
		t.Fatalf("state=%s", op.State)
	}
}

func TestOpenVideoResultStreamsEphemeralOutput(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tasks/task-123":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "task-123", "status": "SUCCEEDED", "output": []string{server.URL + "/output.mp4"},
			})
		case "/output.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = io.WriteString(w, "video-bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL, server.Client())
	binary, err := adapter.OpenVideoResult(context.Background(), "task-123")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := binary.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if binary.MIMEType() != "video/mp4" || string(data) != "video-bytes" {
		t.Fatalf("mime=%s data=%s", binary.MIMEType(), data)
	}
}

func newTestAdapter(t *testing.T, baseURL string, client *http.Client) *Adapter {
	t.Helper()
	adapter, err := New(Config{
		ProviderID: "runway",
		BaseURL: baseURL,
		CredentialSource: SecretSourceFunc(func(context.Context) (string, error) { return "secret", nil }),
		Model: ModelConfig{ID: "gen4.5", ExternalModelID: "gen4.5"},
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func roundTripClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func TestMapAspectRatioRejectsUnsupportedShape(t *testing.T) {
	_, err := mapAspectRatio("1:1")
	if err == nil || !strings.Contains(err.Error(), "aspect") {
		t.Fatalf("expected aspect error, got %v", err)
	}
}

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

type fakeReadinessProbe struct {
	calls int
	err   error
	block bool
}

func (p *fakeReadinessProbe) Ready(ctx context.Context) error {
	p.calls++
	if p.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return p.err
}

func TestReadinessHandlerDatabaseFailureRecoveryAndTimeout(t *testing.T) {
	oldTimeout := readinessProbeTimeout
	t.Cleanup(func() { readinessProbeTimeout = oldTimeout })
	readinessProbeTimeout = 20 * time.Millisecond

	cfg := config.Config{Environment: config.EnvironmentProduction, DatabaseURL: "postgres://configured"}
	probe := &fakeReadinessProbe{err: errors.New("database unavailable")}

	recorder := httptest.NewRecorder()
	readinessHandler(cfg, probe, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while database is unavailable, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "database unavailable") {
		t.Fatal("readiness response leaked dependency error")
	}

	probe.err = nil
	recorder = httptest.NewRecorder()
	readinessHandler(cfg, probe, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected readiness recovery, got %d", recorder.Code)
	}

	probe.block = true
	started := time.Now()
	recorder = httptest.NewRecorder()
	readinessHandler(cfg, probe, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on probe timeout, got %d", recorder.Code)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("readiness probe was not bounded: %s", elapsed)
	}
}

func TestReadinessHandlerConfiguredStorageFailureRecoveryAndTimeout(t *testing.T) {
	oldTimeout := readinessProbeTimeout
	t.Cleanup(func() { readinessProbeTimeout = oldTimeout })
	readinessProbeTimeout = 20 * time.Millisecond

	cfg := config.Config{Environment: config.EnvironmentProduction}
	cfg.MediaStorage.Endpoint = "http://storage.example.test"
	cfg.MediaStorage.Bucket = "media"
	cfg.MediaStorage.AccessKeyID = "configured"
	cfg.MediaStorage.SecretAccessKey = "configured"
	probe := &fakeReadinessProbe{err: errors.New("bucket unavailable")}

	recorder := httptest.NewRecorder()
	readinessHandler(cfg, nil, probe).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while storage is unavailable, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "bucket unavailable") || strings.Contains(recorder.Body.String(), "media") {
		t.Fatal("readiness response leaked storage detail")
	}

	probe.err = nil
	recorder = httptest.NewRecorder()
	readinessHandler(cfg, nil, probe).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected storage readiness recovery, got %d", recorder.Code)
	}

	probe.block = true
	started := time.Now()
	recorder = httptest.NewRecorder()
	readinessHandler(cfg, nil, probe).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on storage probe timeout, got %d", recorder.Code)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("storage readiness probe was not bounded: %s", elapsed)
	}
}

func TestReadinessHandlerReusesInjectedRuntimeProbes(t *testing.T) {
	cfg := config.Config{Environment: config.EnvironmentProduction, DatabaseURL: "postgres://configured"}
	cfg.MediaStorage.Endpoint = "http://storage.example.test"
	cfg.MediaStorage.Bucket = "media"
	cfg.MediaStorage.AccessKeyID = "configured"
	cfg.MediaStorage.SecretAccessKey = "configured"
	databaseProbe := &fakeReadinessProbe{}
	storageProbe := &fakeReadinessProbe{}
	handler := readinessHandler(cfg, databaseProbe, storageProbe)

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ready response on request %d, got %d", i+1, recorder.Code)
		}
	}
	if databaseProbe.calls != 3 || storageProbe.calls != 3 {
		t.Fatalf("expected the same long-lived probes to serve all checks, got db=%d storage=%d", databaseProbe.calls, storageProbe.calls)
	}
}

func TestReadinessHandlerOptionalStorageIsNotRequired(t *testing.T) {
	storageProbe := &fakeReadinessProbe{err: errors.New("should not run")}
	recorder := httptest.NewRecorder()
	readinessHandler(config.Config{Environment: config.EnvironmentTest}, nil, storageProbe).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unconfigured optional dependencies to stay ready, got %d", recorder.Code)
	}
	if storageProbe.calls != 0 {
		t.Fatalf("expected no optional storage probe, got %d calls", storageProbe.calls)
	}
}

func TestRequestLoggerPropagatesRequestIDAndLogsCompletion(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := requestLogger(logger, mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/secret-project-id", nil)
	request.Header.Set(requestIDHeader, "550e8400-e29b-41d4-a716-446655440000")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(requestIDHeader); got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected response request ID %q", got)
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
		t.Fatalf("decode completion log: %v", err)
	}
	if record["msg"] != "api request completed" || record["status"] != float64(http.StatusCreated) {
		t.Fatalf("unexpected completion log: %#v", record)
	}
	if record["route"] != "GET /api/v1/projects/{id}" {
		t.Fatalf("expected route template, got %#v", record["route"])
	}
	if strings.Contains(logs.String(), "secret-project-id") {
		t.Fatal("completion log leaked raw dynamic path")
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Fatal("completion log missing duration_ms")
	}
}

func TestRequestLoggerGeneratesSafeIDAndCorrelatesPanics(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := requestLogger(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive panic detail")
	}))

	request := httptest.NewRequest(http.MethodGet, "/unmatched/secret", nil)
	request.Header.Set(requestIDHeader, "not-a-valid-request-id")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected panic to map to 500, got %d", recorder.Code)
	}
	requestID := recorder.Header().Get(requestIDHeader)
	if requestID == "" || requestID == "not-a-valid-request-id" {
		t.Fatalf("expected generated safe request ID, got %q", requestID)
	}
	if !strings.Contains(logs.String(), requestID) {
		t.Fatal("panic/completion logs are not correlated by request ID")
	}
	if strings.Contains(logs.String(), "sensitive panic detail") || strings.Contains(logs.String(), "/unmatched/secret") {
		t.Fatal("panic logging leaked sensitive detail or raw unmatched path")
	}
}

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

func TestReadinessHandlerDatabaseFailureRecoveryAndTimeout(t *testing.T) {
	oldDatabaseProbe := readinessDatabaseProbe
	oldStorageProbe := readinessStorageProbe
	oldTimeout := readinessProbeTimeout
	t.Cleanup(func() {
		readinessDatabaseProbe = oldDatabaseProbe
		readinessStorageProbe = oldStorageProbe
		readinessProbeTimeout = oldTimeout
	})
	readinessProbeTimeout = 20 * time.Millisecond
	readinessStorageProbe = func(context.Context, config.MediaStorageConfig) error { return nil }

	cfg := config.Config{Environment: config.EnvironmentProduction, DatabaseURL: "postgres://configured"}
	var databaseErr error
	readinessDatabaseProbe = func(context.Context, string) error { return databaseErr }

	databaseErr = errors.New("database unavailable")
	recorder := httptest.NewRecorder()
	readinessHandler(cfg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while database is unavailable, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "database unavailable") {
		t.Fatal("readiness response leaked dependency error")
	}

	databaseErr = nil
	recorder = httptest.NewRecorder()
	readinessHandler(cfg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected readiness recovery, got %d", recorder.Code)
	}

	readinessDatabaseProbe = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}
	started := time.Now()
	recorder = httptest.NewRecorder()
	readinessHandler(cfg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on probe timeout, got %d", recorder.Code)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("readiness probe was not bounded: %s", elapsed)
	}
}

func TestReadinessHandlerOptionalStorageIsNotRequired(t *testing.T) {
	oldDatabaseProbe := readinessDatabaseProbe
	oldStorageProbe := readinessStorageProbe
	t.Cleanup(func() {
		readinessDatabaseProbe = oldDatabaseProbe
		readinessStorageProbe = oldStorageProbe
	})

	readinessDatabaseProbe = func(context.Context, string) error { return nil }
	storageCalls := 0
	readinessStorageProbe = func(context.Context, config.MediaStorageConfig) error {
		storageCalls++
		return errors.New("should not run")
	}

	recorder := httptest.NewRecorder()
	readinessHandler(config.Config{Environment: config.EnvironmentTest}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unconfigured optional dependencies to stay ready, got %d", recorder.Code)
	}
	if storageCalls != 0 {
		t.Fatalf("expected no optional storage probe, got %d calls", storageCalls)
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

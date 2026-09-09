package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

func TestServerAppliesProductionCORSAllowlist(t *testing.T) {
	server := New(config.Config{
		Addr:               ":0",
		Environment:        config.EnvironmentProduction,
		CORSAllowedOrigins: []string{"https://app.synvideo.example"},
	}, slog.Default(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/healthz", nil)
	allowed.Header.Set("Origin", "https://app.synvideo.example")
	allowed.Header.Set("Access-Control-Request-Method", "GET")
	allowedRecorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(allowedRecorder, allowed)
	if allowedRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected allowed preflight 204, got %d", allowedRecorder.Code)
	}

	disallowed := httptest.NewRequest(http.MethodOptions, "/api/v1/healthz", nil)
	disallowed.Header.Set("Origin", "https://evil.example")
	disallowed.Header.Set("Access-Control-Request-Method", "GET")
	disallowedRecorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(disallowedRecorder, disallowed)
	if disallowedRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected disallowed preflight 403, got %d", disallowedRecorder.Code)
	}
}

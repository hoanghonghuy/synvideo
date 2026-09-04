package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

func TestNewAppliesHTTPResourceBounds(t *testing.T) {
	server := New(config.Config{Addr: ":0"}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if server.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, defaultReadHeaderTimeout)
	}
	if server.IdleTimeout != defaultIdleTimeout {
		t.Fatalf("IdleTimeout = %s, want %s", server.IdleTimeout, defaultIdleTimeout)
	}
	if server.MaxHeaderBytes != defaultMaxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, defaultMaxHeaderBytes)
	}
	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %s, want intentionally unbounded 0", server.ReadTimeout)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %s, want intentionally unbounded 0", server.WriteTimeout)
	}
}

func TestJSONBodyLimitRejectsOversizedRequestBeforeHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limitJSONRequestBody(8, next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"title":"too large"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if called {
		t.Fatal("downstream handler was called for oversized JSON request")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestJSONBodyLimitCannotBeBypassedByMissingContentType(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limitJSONRequestBody(8, next)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/p", bytes.NewBufferString(`{"title":"too large"}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if called {
		t.Fatal("downstream handler was called for oversized body without Content-Type")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestJSONBodyLimitLeavesMediaUploadSemanticsAlone(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limitJSONRequestBody(8, next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/media-assets", bytes.NewBufferString("0123456789abcdef"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("media upload request was incorrectly rejected by JSON body limit")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestJSONBodyLimitUsesFiniteStreamingGuard(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		if err == nil {
			t.Fatal("expected body read to fail after finite limit")
		}
		w.WriteHeader(http.StatusBadRequest)
	})
	handler := limitJSONRequestBody(8, next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", io.NopCloser(bytes.NewBufferString("0123456789abcdef")))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestResourceBoundDefaultsAreNotTiny(t *testing.T) {
	if defaultReadHeaderTimeout < 5*time.Second {
		t.Fatalf("ReadHeaderTimeout %s is too small for normal clients", defaultReadHeaderTimeout)
	}
	if defaultIdleTimeout < 30*time.Second {
		t.Fatalf("IdleTimeout %s is too small for normal keep-alive clients", defaultIdleTimeout)
	}
}

package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimeToolchainEndpointReportsFFmpegVersions(t *testing.T) {
	handler := runtimeToolchainHandler(func() (runtimeToolchainResponse, error) {
		return runtimeToolchainResponse{
			FFmpegVersion:  "ffmpeg version 7.1.1",
			FFprobeVersion: "ffprobe version 7.1.1",
		}, nil
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/toolchain", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response runtimeToolchainResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.FFmpegVersion == "" || response.FFprobeVersion == "" {
		t.Fatalf("expected ffmpeg/ffprobe versions, got %+v", response)
	}
}

func TestRuntimeToolchainEndpointFailsWhenProbeUnavailable(t *testing.T) {
	handler := runtimeToolchainHandler(func() (runtimeToolchainResponse, error) {
		return runtimeToolchainResponse{}, errRuntimeToolchainUnavailable
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/toolchain", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

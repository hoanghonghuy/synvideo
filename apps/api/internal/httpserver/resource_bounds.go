package httpserver

import (
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultMaxHeaderBytes    = 1 << 20 // 1 MiB
	defaultMaxJSONBodyBytes  = 1 << 20 // 1 MiB
)

type requestBodyTooLargeResponse struct {
	Error apiError `json:"error"`
}

func limitJSONRequestBody(maxBytes int64, next http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = defaultMaxJSONBodyBytes
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldLimitJSONRequestBody(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > maxBytes {
			writeProjectJSON(w, http.StatusRequestEntityTooLarge, requestBodyTooLargeResponse{Error: apiError{
				Code:    "request_body_too_large",
				Message: "Request body is too large.",
			}})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

func shouldLimitJSONRequestBody(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}

	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return true
	}
	return mediaType != "multipart/form-data"
}

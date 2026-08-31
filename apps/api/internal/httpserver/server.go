package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

type statusResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment,omitempty"`
}

func New(cfg config.Config, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", healthHandler)
	mux.HandleFunc("GET /api/v1/readyz", readinessHandler(cfg))

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: requestLogger(logger, mux),
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func readinessHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, statusResponse{
			Status:      "ready",
			Environment: cfg.Environment,
		})
	}
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("api request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body statusResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

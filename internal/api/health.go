package api

import (
	"log/slog"
	"net/http"
	"time"
)

// HealthHandler handles GET /api/v1/health requests.
type HealthHandler struct {
	startTime         time.Time
	logger            *slog.Logger
	version           string
	commitSHA         string
	buildTime         string
	schemaVersion     string
	schemaFingerprint string
}

// HealthConfig holds build-time version info for the health endpoint.
type HealthConfig struct {
	SchemaVersion     string
	SchemaFingerprint string
	Version           string
	CommitSHA         string
	BuildTime         string
}

// NewHealthHandler creates a new health check handler.
func NewHealthHandler(logger *slog.Logger, cfg HealthConfig) *HealthHandler {
	if logger == nil {
		logger = slog.Default()
	}
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	return &HealthHandler{
		startTime:         time.Now(),
		logger:            logger,
		version:           version,
		commitSHA:         cfg.CommitSHA,
		buildTime:         cfg.BuildTime,
		schemaVersion:     cfg.SchemaVersion,
		schemaFingerprint: cfg.SchemaFingerprint,
	}
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	SchemaVersion     string `json:"schema_version,omitempty"`
	SchemaFingerprint string `json:"schema_fingerprint,omitempty"`
	Status            string `json:"status"`
	Version           string `json:"version"`
	CommitSHA         string `json:"commit_sha,omitempty"`
	BuildTime         string `json:"build_time,omitempty"`
	UptimeSeconds     int64  `json:"uptime_seconds"`
}

// ServeHTTP implements http.Handler.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:            "healthy",
		SchemaVersion:     h.schemaVersion,
		SchemaFingerprint: h.schemaFingerprint,
		Version:           h.version,
		CommitSHA:         h.commitSHA,
		BuildTime:         h.buildTime,
		UptimeSeconds:     int64(time.Since(h.startTime).Seconds()),
	}

	writeJSON(w, http.StatusOK, resp)
}

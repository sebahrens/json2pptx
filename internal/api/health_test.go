package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var testHealthConfig = HealthConfig{
	Version:   "1.2.3",
	CommitSHA: "abc123",
	BuildTime: "2026-01-01T00:00:00Z",
}

func TestHealthHandler_Healthy(t *testing.T) {
	handler := NewHealthHandler(slog.Default(), testHealthConfig)

	// Wait a moment to ensure uptime > 0
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("content type = %s, want application/json", contentType)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("status = %s, want healthy", resp.Status)
	}

	if resp.Version != "1.2.3" {
		t.Errorf("version = %s, want 1.2.3", resp.Version)
	}

	if resp.CommitSHA != "abc123" {
		t.Errorf("commit_sha = %s, want abc123", resp.CommitSHA)
	}

	if resp.BuildTime != "2026-01-01T00:00:00Z" {
		t.Errorf("build_time = %s, want 2026-01-01T00:00:00Z", resp.BuildTime)
	}

	if resp.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds = %d, want >= 0", resp.UptimeSeconds)
	}
}

func TestHealthHandler_UptimeTracking(t *testing.T) {
	handler := NewHealthHandler(slog.Default(), testHealthConfig)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	var resp1 HealthResponse
	if err := json.NewDecoder(rec1.Body).Decode(&resp1); err != nil {
		t.Fatalf("failed to decode response 1: %v", err)
	}

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Second request
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	var resp2 HealthResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode response 2: %v", err)
	}

	// Second uptime should be greater than or equal to first
	if resp2.UptimeSeconds < resp1.UptimeSeconds {
		t.Errorf("uptime decreased: %d -> %d", resp1.UptimeSeconds, resp2.UptimeSeconds)
	}
}

// TestHealthHandler_AC9 validates AC9: Health Check
// When GET /api/v1/health
// Then returns status and version
func TestHealthHandler_AC9(t *testing.T) {
	handler := NewHealthHandler(slog.Default(), testHealthConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify status code
	if rec.Code != http.StatusOK {
		t.Errorf("AC9: status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("AC9: content type = %s, want application/json", contentType)
	}

	// Verify response structure
	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("AC9: failed to decode response: %v", err)
	}

	// Verify required fields are present
	if resp.Status == "" {
		t.Error("AC9: status field is empty")
	}

	if resp.Version == "" {
		t.Error("AC9: version field is empty")
	}

	if resp.Status != "healthy" {
		t.Errorf("AC9: status = %s, want healthy", resp.Status)
	}
}

func TestHealthHandler_DefaultVersion(t *testing.T) {
	handler := NewHealthHandler(slog.Default(), HealthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "dev" {
		t.Errorf("version = %s, want dev (default)", resp.Version)
	}
}

func TestHealthHandler_NilLogger(t *testing.T) {
	handler := NewHealthHandler(nil, testHealthConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("status = %s, want healthy", resp.Status)
	}
}

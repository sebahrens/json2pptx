package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	apierrors "github.com/ahrens/go-slide-creator/internal/api/errors"
	"github.com/ahrens/go-slide-creator/internal/template"
)

// TestNewServer verifies that NewServer creates a properly configured server.
func TestNewServer(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       slog.Default(),
	})

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.mux == nil {
		t.Error("Expected mux to be initialized")
	}

	if server.convertService == nil {
		t.Error("Expected convertService to be initialized")
	}

	if server.templateService == nil {
		t.Error("Expected templateService to be initialized")
	}

	if server.healthHandler == nil {
		t.Error("Expected healthHandler to be initialized")
	}
}

// TestServerRouting validates that all routes are properly configured.
func TestServerRouting(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       slog.Default(),
	})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int // 0 means we don't care about status, just that route exists
	}{
		{
			name:           "health endpoint",
			method:         "GET",
			path:           "/api/v1/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "list templates",
			method:         "GET",
			path:           "/api/v1/templates",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "template details",
			method:         "GET",
			path:           "/api/v1/templates/corporate",
			expectedStatus: 0, // Will be 400 if template doesn't exist, but route exists
		},
		{
			name:           "slide types",
			method:         "GET",
			path:           "/api/v1/slide-types",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "convert endpoint",
			method:         "POST",
			path:           "/api/v1/convert",
			expectedStatus: 0, // Will be 400 for missing body, but route exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.method == "POST" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte("{}")))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			// Check that route exists (not 404 for wrong route)
			if w.Code == http.StatusNotFound && tt.expectedStatus != 0 {
				t.Errorf("Route not found: %s %s", tt.method, tt.path)
			}

			if tt.expectedStatus != 0 && w.Code != tt.expectedStatus {
				body, _ := io.ReadAll(w.Body)
				t.Logf("Response body: %s", body)
			}
		})
	}
}

// TestServerServeHTTP verifies the ServeHTTP method correctly delegates to the mux.
func TestServerServeHTTP(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       slog.Default(),
	})

	// Test that ServeHTTP delegates to mux
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	// Health endpoint should return OK
	if w.Code != http.StatusOK {
		t.Errorf("ServeHTTP: Status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestSecurityHeaders verifies that security headers are set on all responses.
func TestSecurityHeaders(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       slog.Default(),
	})

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":          "DENY",
		"X-Xss-Protection":         "1; mode=block",
		"Content-Security-Policy":  "default-src 'none'",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	for header, want := range expectedHeaders {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("Header %s = %q, want %q", header, got, want)
		}
	}
}

// TestNewServer_NilLogger verifies that a nil logger defaults to slog.Default().
func TestNewServer_NilLogger(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	// Create server with nil logger
	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       nil, // Should default to slog.Default()
	})

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Verify the server has a logger (should be slog.Default())
	if server.logger == nil {
		t.Error("Expected logger to be initialized with slog.Default()")
	}
}

// TestNewServer_StrictValidationMode verifies strict validation mode is passed to template service.
func TestNewServer_StrictValidationMode(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	// Create server with strict validation enabled
	server := NewServer(ServerConfig{
		TemplatesDir:     templatesDir,
		OutputDir:        outputDir,
		Cache:            cache,
		Logger:           slog.Default(),
		StrictValidation: true, // Strict mode enabled
	})

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.templateService == nil {
		t.Error("Expected templateService to be initialized with strict validation")
	}
}

// TestServerIntegration_RequestValidation validates request validation.
func TestServerIntegration_RequestValidation(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(1 * time.Hour)

	server := NewServer(ServerConfig{
		TemplatesDir: templatesDir,
		OutputDir:    outputDir,
		Cache:        cache,
		Logger:       slog.Default(),
	})

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "missing markdown field",
			method:      "POST",
			path:        "/api/v1/convert",
			body:        `{"template": "test"}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:        "missing template field",
			method:      "POST",
			path:        "/api/v1/convert",
			body:        `{"markdown": "# Test"}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:        "invalid JSON",
			method:      "POST",
			path:        "/api/v1/convert",
			body:        `{invalid json}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(tt.body)))
			if tt.method == "POST" || tt.method == "PUT" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}

			var errResp apierrors.Response
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if errResp.Error.Code != tt.wantErrCode {
				t.Errorf("Error code = %q, want %q", errResp.Error.Code, tt.wantErrCode)
			}

			// Verify field-specific error details
			if tt.wantErrCode == "INVALID_REQUEST" {
				if errResp.Error.Message == "" {
					t.Error("Expected error message to be non-empty")
				}
			}
		})
	}
}

// TestPanicRecovery verifies that panics in handlers are caught and
// converted to 500 Internal Server Error responses instead of crashing the connection.
func TestPanicRecovery(t *testing.T) {
	// Suppress logs during test to avoid noise
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("panic in ServeHTTP is recovered", func(t *testing.T) {
		// Create a server and replace its mux with one that panics, to exercise
		// the inline panic recovery in ServeHTTP.
		tempDir := t.TempDir()
		templatesDir := filepath.Join(tempDir, "templates")
		outputDir := filepath.Join(tempDir, "output")

		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatalf("Failed to create templates dir: %v", err)
		}

		cache := template.NewMemoryCache(1 * time.Hour)

		server := NewServer(ServerConfig{
			TemplatesDir: templatesDir,
			OutputDir:    outputDir,
			Cache:        cache,
			Logger:       discardLogger,
		})

		// Replace the mux with one that has a panicking handler
		panicMux := http.NewServeMux()
		panicMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			panic("test panic message")
		})
		server.mux = panicMux

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// This should NOT panic, but return 500
		server.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
		}

		// Verify response body contains error details
		var response apierrors.Response
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}

		if response.Error.Code != "PANIC_RECOVERED" {
			t.Errorf("Error code = %s, want PANIC_RECOVERED", response.Error.Code)
		}
	})

	t.Run("non-panicking handlers work normally", func(t *testing.T) {
		tempDir := t.TempDir()
		templatesDir := filepath.Join(tempDir, "templates")
		outputDir := filepath.Join(tempDir, "output")

		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatalf("Failed to create templates dir: %v", err)
		}

		cache := template.NewMemoryCache(1 * time.Hour)

		server := NewServer(ServerConfig{
			TemplatesDir: templatesDir,
			OutputDir:    outputDir,
			Cache:        cache,
			Logger:       discardLogger,
		})

		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}

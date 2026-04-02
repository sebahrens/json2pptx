package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
)

func TestDownloadHandler_Success(t *testing.T) {
	outputDir := t.TempDir()

	// Create a fake PPTX file with a valid hex filename
	filename := "0123456789abcdef0123456789abcdef.pptx"
	content := []byte("PK\x03\x04fake-pptx-content")
	if err := os.WriteFile(filepath.Join(outputDir, filename), content, 0644); err != nil {
		t.Fatal(err)
	}

	service := &ConvertService{outputDir: outputDir}
	handler := service.DownloadHandler()

	// Use a mux to get PathValue working
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/download/{filename}", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download/"+filename, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.presentationml.presentation" {
		t.Errorf("Expected PPTX content type, got %s", ct)
	}

	cd := w.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("Expected Content-Disposition header")
	}
}

func TestDownloadHandler_NotFound(t *testing.T) {
	outputDir := t.TempDir()
	service := &ConvertService{outputDir: outputDir}
	handler := service.DownloadHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/download/{filename}", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download/0123456789abcdef0123456789abcdef.pptx", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Code != apierrors.CodeFileNotFound {
		t.Errorf("Expected FILE_NOT_FOUND, got %s", resp.Error.Code)
	}
}

func TestDownloadHandler_InvalidFilename(t *testing.T) {
	outputDir := t.TempDir()
	service := &ConvertService{outputDir: outputDir}
	handler := service.DownloadHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/download/{filename}", handler)

	tests := []struct {
		name     string
		filename string
	}{
		{"wrong extension", "0123456789abcdef0123456789abcdef.exe"},
		{"too short hex", "abcdef.pptx"},
		{"non-hex chars", "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ.pptx"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/download/"+tt.filename, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Should get 400 for invalid filenames (or 404 for empty which won't match route)
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Errorf("Expected 400 or 404 for %q, got %d", tt.filename, w.Code)
			}
		})
	}
}

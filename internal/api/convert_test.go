package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestConvertSuccess validates AC1: Convert Success.
func TestConvertSuccess(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create a minimal test template
	testTemplate := createTestTemplate(t, templatesDir, "test-template")

	// Create service with real pipeline
	cache := template.NewMemoryCache(24 * 60 * 60) // 24 hours
	templateService := NewTemplateService(templatesDir, cache, false)
	conversionPipeline := pipeline.NewPipeline() // No LLM for tests
	service := NewConvertService(templatesDir, outputDir, templateService, conversionPipeline)

	// Valid JSON slide input
	reqBody := ConvertRequest{
		Template: "test-template",
		Slides: []APISlide{
			{Type: "content", Title: "Welcome", Content: APIContent{Body: "This is the first slide."}},
			{Type: "content", Title: "Agenda", Content: APIContent{Bullets: []string{"Point one", "Point two", "Point three"}}},
		},
		Options: &ConvertOptions{
			OutputFormat: "file",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Execute
	service.ConvertHandler()(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var resp ConvertResponseFile
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// AC1: Returns 200 with file_url
	if !resp.Success {
		t.Error("Expected success=true")
	}

	if resp.FileURL == "" {
		t.Error("Expected file_url to be set")
	}

	if !strings.HasPrefix(resp.FileURL, "/api/v1/download/") {
		t.Errorf("Expected file_url to start with /api/v1/download/, got %s", resp.FileURL)
	}

	if resp.ExpiresAt == "" {
		t.Error("Expected expires_at to be set")
	}

	if resp.Stats.SlideCount != 2 {
		t.Errorf("Expected 2 slides, got %d", resp.Stats.SlideCount)
	}

	if resp.Stats.ProcessingTimeMs <= 0 {
		t.Error("Expected positive processing time")
	}

	// Verify output file exists
	filename := strings.TrimPrefix(resp.FileURL, "/api/v1/download/")
	outputPath := filepath.Join(outputDir, filename)
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("Output file does not exist: %s", outputPath)
	}

	// Cleanup
	_ = os.Remove(testTemplate)
	_ = os.Remove(outputPath)
}

// TestConvertBase64 validates AC2: Convert with Base64.
func TestConvertBase64(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	testTemplate := createTestTemplate(t, templatesDir, "test-template")

	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	conversionPipeline := pipeline.NewPipeline()
	service := NewConvertService(templatesDir, outputDir, templateService, conversionPipeline)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides: []APISlide{
			{Type: "content", Title: "Welcome"},
		},
		Options: &ConvertOptions{
			OutputFormat: "base64",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Execute
	service.ConvertHandler()(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var resp ConvertResponseBase64
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// AC2: Returns base64-encoded PPTX in data field
	if !resp.Success {
		t.Error("Expected success=true")
	}

	if resp.Data == "" {
		t.Error("Expected data field to contain base64 content")
	}

	if resp.Filename == "" {
		t.Error("Expected filename to be set")
	}

	// Verify base64 is valid
	decoded, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		t.Errorf("Data is not valid base64: %v", err)
	}

	// Verify it looks like a PPTX file (starts with PK for ZIP)
	if len(decoded) < 2 || decoded[0] != 'P' || decoded[1] != 'K' {
		t.Error("Decoded data does not appear to be a PPTX file")
	}

	if resp.Stats.SlideCount != 1 {
		t.Errorf("Expected 1 slide, got %d", resp.Stats.SlideCount)
	}

	// Verify file was cleaned up (not left in output dir)
	files, _ := os.ReadDir(outputDir)
	if len(files) > 0 {
		t.Errorf("Expected output directory to be empty, found %d files", len(files))
	}

	// Cleanup
	_ = os.Remove(testTemplate)
}

// TestConvertEmptySlides validates AC3: Convert with empty slides returns error.
func TestConvertEmptySlides(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("Expected success=false")
	}

	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("Expected error code INVALID_REQUEST, got %s", resp.Error.Code)
	}
}

// TestConvertInvalidTemplate validates AC4: Convert Invalid Template.
func TestConvertInvalidTemplate(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	conversionPipeline := pipeline.NewPipeline()
	service := NewConvertService(templatesDir, outputDir, templateService, conversionPipeline)

	reqBody := ConvertRequest{
		Template: "nonexistent",
		Slides:   []APISlide{{Type: "content", Title: "Welcome"}},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Execute
	service.ConvertHandler()(w, req)

	// Verify
	// AC4: Returns 404 with TEMPLATE_NOT_FOUND
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("Expected success=false")
	}

	if resp.Error.Code != "TEMPLATE_NOT_FOUND" {
		t.Errorf("Expected error code TEMPLATE_NOT_FOUND, got %s", resp.Error.Code)
	}

	if resp.Error.Message == "" {
		t.Error("Expected error message to be set")
	}

	if !strings.Contains(resp.Error.Message, "nonexistent") {
		t.Errorf("Expected error message to mention template name, got: %s", resp.Error.Message)
	}
}

// TestConvertMissingSlides tests validation when slides are missing.
func TestConvertMissingSlides(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	reqBody := ConvertRequest{
		Slides:   nil,
		Template: "test",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("Expected error code INVALID_REQUEST, got %s", resp.Error.Code)
	}
}

// TestConvertMissingTemplate tests validation when template is missing.
func TestConvertMissingTemplate(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	reqBody := ConvertRequest{
		Slides:   []APISlide{{Type: "content", Title: "Test"}},
		Template: "",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("Expected error code INVALID_REQUEST, got %s", resp.Error.Code)
	}
}

// TestConvertInvalidJSON tests handling of malformed request body.
func TestConvertInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != apierrors.CodeInvalidJSON {
		t.Errorf("Expected error code %s, got %s", apierrors.CodeInvalidJSON, resp.Error.Code)
	}
}

func TestConvertInvalidJSONSyntaxIncludesOffset(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	// Send JSON with a syntax error (invalid token in value position)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(`{"template": x}`))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != apierrors.CodeInvalidJSON {
		t.Errorf("Error code = %q, want %q", resp.Error.Code, apierrors.CodeInvalidJSON)
	}
	if resp.Error.Details == nil {
		t.Fatal("Expected details to be non-nil for syntax errors")
	}
	if _, hasOffset := resp.Error.Details["offset"]; !hasOffset {
		t.Error("Expected details to include 'offset' for syntax errors")
	}
}

// TestConvertInvalidOutputFormat tests validation of output format.
func TestConvertInvalidOutputFormat(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	service := NewConvertService(templatesDir, tempDir, templateService, nil)

	reqBody := ConvertRequest{
		Slides:   []APISlide{{Type: "content", Title: "Slide"}},
		Template: "test",
		Options: &ConvertOptions{
			OutputFormat: "invalid",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("Expected error code INVALID_REQUEST, got %s", resp.Error.Code)
	}

	if !strings.Contains(resp.Error.Message, "output format") {
		t.Errorf("Expected error about output format, got: %s", resp.Error.Message)
	}
}

// TestConvertDefaultOptions tests that default options are applied.
func TestConvertDefaultOptions(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	testTemplate := createTestTemplate(t, templatesDir, "test-template")

	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	conversionPipeline := pipeline.NewPipeline()
	service := NewConvertService(templatesDir, outputDir, templateService, conversionPipeline)

	// Request without options
	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Slide"}},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ConvertResponseFile
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should default to file output
	if resp.FileURL == "" {
		t.Error("Expected file_url for default output format")
	}

	// Cleanup
	_ = os.Remove(testTemplate)
}

// TestGenerateUniqueFilename tests unique filename generation.
func TestGenerateUniqueFilename(t *testing.T) {
	// Generate multiple filenames
	filenames := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name, err := generateUniqueFilename()
		if err != nil {
			t.Errorf("Unexpected error generating filename: %v", err)
			continue
		}
		if name == "" {
			t.Error("Generated empty filename")
		}
		if filenames[name] {
			t.Errorf("Duplicate filename generated: %s", name)
		}
		filenames[name] = true
	}
}

// TestGenerateUniqueFilenameNoPredictableFallback tests LOW-06 security fix:
// generateUniqueFilename should return an error instead of falling back to
// predictable timestamp-based names when crypto/rand fails.
func TestGenerateUniqueFilenameNoPredictableFallback(t *testing.T) {
	// Note: We cannot easily mock crypto/rand.Read failure in Go without
	// dependency injection. However, we can verify the function signature
	// now returns an error, which ensures the caller must handle the failure case.
	//
	// The security fix is that:
	// 1. The function returns (string, error) instead of just string
	// 2. On crypto/rand failure, it returns an error instead of a predictable value
	// 3. Callers must now handle the error case explicitly
	//
	// We verify the success case works correctly:
	name, err := generateUniqueFilename()
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if name == "" {
		t.Error("Expected non-empty filename")
	}
	// Verify it's a hex-encoded 16-byte value (32 hex chars)
	if len(name) != 32 {
		t.Errorf("Expected 32-character hex filename, got %d chars: %s", len(name), name)
	}
	// Verify it's valid hex
	for _, c := range name {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Invalid hex character in filename: %c", c)
		}
	}
}

// TestConvertRequestBodyTooLarge tests HIGH-02: Request body size limits.
func TestConvertRequestBodyTooLarge(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	// Create a request body larger than MaxRequestBodySize (10MB)
	// We'll create an 11MB body
	largeContent := strings.Repeat("x", 11*1024*1024)
	reqBody := ConvertRequest{
		Slides:   []APISlide{{Type: "content", Title: "Test", Content: APIContent{Body: largeContent}}},
		Template: "test",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	// Verify
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	var resp apierrors.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != "REQUEST_TOO_LARGE" {
		t.Errorf("Expected error code REQUEST_TOO_LARGE, got %s", resp.Error.Code)
	}

	if !strings.Contains(resp.Error.Message, "maximum size") {
		t.Errorf("Expected error message to mention maximum size, got: %s", resp.Error.Message)
	}
}

// TestConvertRequestWithinSizeLimit tests that requests within the size limit succeed.
func TestConvertRequestWithinSizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	testTemplate := createTestTemplate(t, templatesDir, "test-template")

	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	conversionPipeline := pipeline.NewPipeline()
	service := NewConvertService(templatesDir, outputDir, templateService, conversionPipeline)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Welcome", Content: APIContent{Body: "Some content"}}},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	// Should succeed - not be rejected due to size
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	// Cleanup
	_ = os.Remove(testTemplate)
}

// mockPipeline is a test implementation of pipeline.Pipeline for testing ConvertService.
type mockPipeline struct {
	convertFunc func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error)
}

func (m *mockPipeline) Convert(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
	if m.convertFunc != nil {
		return m.convertFunc(ctx, req)
	}
	return &pipeline.ConvertResult{
		OutputPath: req.OutputPath,
		SlideCount: 1,
	}, nil
}

// TestConvertServiceWithMockPipeline tests that ConvertService works with injected pipeline.
func TestConvertServiceWithMockPipeline(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create a minimal test template
	testTemplate := createTestTemplate(t, templatesDir, "test-template")

	// Track if pipeline was called
	pipelineCalled := false
	mockPipe := &mockPipeline{
		convertFunc: func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
			pipelineCalled = true

			// Create a minimal valid PPTX file at the output path
			input, err := os.ReadFile(testTemplate)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(req.OutputPath, input, 0644); err != nil {
				return nil, err
			}

			return &pipeline.ConvertResult{
				OutputPath: req.OutputPath,
				SlideCount: 1,
			}, nil
		},
	}

	// Create service with mock pipeline
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	service := NewConvertService(templatesDir, outputDir, templateService, mockPipe)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Welcome", Content: APIContent{Body: "This is the first slide."}}},
		Options:  &ConvertOptions{OutputFormat: "file"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	if !pipelineCalled {
		t.Error("Expected mock pipeline to be called")
	}
}

// TestConvertHandlerTimeoutBehavior tests that context timeout is properly applied (MED-06).
func TestConvertHandlerTimeoutBehavior(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create test template
	testTemplate := createTestTemplate(t, templatesDir, "test-template")
	defer func() { _ = os.Remove(testTemplate) }()

	mockPipe := &mockPipeline{
		convertFunc: func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
			// Verify the context has a deadline (set by DefaultConvertTimeout)
			if _, ok := ctx.Deadline(); !ok {
				t.Error("Expected context to have a deadline from DefaultConvertTimeout")
			}

			// Create a minimal valid PPTX file at the output path
			input, err := os.ReadFile(testTemplate)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(req.OutputPath, input, 0644); err != nil {
				return nil, err
			}

			return &pipeline.ConvertResult{
				OutputPath: req.OutputPath,
				SlideCount: 1,
			}, nil
		},
	}

	// Create service (uses DefaultConvertTimeout internally)
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	service := NewConvertService(templatesDir, outputDir, templateService, mockPipe)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Welcome"}},
		Options:  &ConvertOptions{OutputFormat: "file"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}
}

// createTestTemplate creates a minimal valid PPTX template for testing.
func createTestTemplate(t *testing.T, dir, name string) string {
	t.Helper()

	// Read the existing test template from test fixtures
	fixturesPath := filepath.Join("..", "..", "testdata", "templates", "SimpleMinimal-Consulting.pptx")

	// Check if test fixtures exist
	if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
		// If fixtures don't exist, skip the test
		t.Skip("Test template fixtures not available")
	}

	templatePath := filepath.Join(dir, name+".pptx")

	// Copy the test template
	input, err := os.ReadFile(fixturesPath)
	if err != nil {
		t.Fatalf("Failed to read test template: %v", err)
	}

	if err := os.WriteFile(templatePath, input, 0644); err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	return templatePath
}

// TestSVGScaleValidation tests the svg_scale parameter validation.
func TestSVGScaleValidation(t *testing.T) {
	tests := []struct {
		name       string
		svgScale   float64
		wantStatus int
		wantError  bool
	}{
		{
			name:       "zero value uses default",
			svgScale:   0,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "valid minimum scale",
			svgScale:   0.5,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "valid maximum scale",
			svgScale:   10.0,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "valid mid-range scale",
			svgScale:   2.5,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "below minimum scale",
			svgScale:   0.4,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "above maximum scale",
			svgScale:   10.1,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "negative scale",
			svgScale:   -1.0,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			templatesDir := tempDir + "/templates"
			outputDir := tempDir + "/output"
			if err := os.MkdirAll(templatesDir, 0755); err != nil {
				t.Fatalf("Failed to create templates dir: %v", err)
			}
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				t.Fatalf("Failed to create output dir: %v", err)
			}

			// Create test template
			testTemplate := createTestTemplate(t, templatesDir, "test-template")
			defer func() { _ = os.Remove(testTemplate) }()

			// Create service with mock pipeline
			cache := template.NewMemoryCache(24 * 60 * 60)
			templateService := NewTemplateService(templatesDir, cache, false)
			mockPipe := &mockPipeline{
				convertFunc: func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
					// Create output file
					input, _ := os.ReadFile(testTemplate)
					_ = os.WriteFile(req.OutputPath, input, 0644)
					return &pipeline.ConvertResult{
						OutputPath: req.OutputPath,
						SlideCount: 1,
					}, nil
				},
			}
			service := NewConvertService(templatesDir, outputDir, templateService, mockPipe)

			reqBody := ConvertRequest{
				Template: "test-template",
				Slides:   []APISlide{{Type: "content", Title: "Test Slide", Content: APIContent{Body: "Content here."}}},
				Options: &ConvertOptions{
					OutputFormat: "file",
					SVGScale:     tt.svgScale,
				},
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
			w := httptest.NewRecorder()

			// Execute
			service.ConvertHandler()(w, req)

			// Verify status
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
			}

			// Verify error response if expected
			if tt.wantError {
				var errResp apierrors.Response
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errResp.Error.Code != "INVALID_REQUEST" {
					t.Errorf("Expected error code INVALID_REQUEST, got %s", errResp.Error.Code)
				}
			}
		})
	}
}

// TestSVGScalePassedToPipeline verifies svg_scale is passed through to the pipeline.
func TestSVGScalePassedToPipeline(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create test template
	testTemplate := createTestTemplate(t, templatesDir, "test-template")
	defer func() { _ = os.Remove(testTemplate) }()

	// Track the svg_scale passed to pipeline
	var capturedSVGScale float64
	mockPipe := &mockPipeline{
		convertFunc: func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
			capturedSVGScale = req.SVGScale

			// Create output file
			input, _ := os.ReadFile(testTemplate)
			_ = os.WriteFile(req.OutputPath, input, 0644)
			return &pipeline.ConvertResult{
				OutputPath: req.OutputPath,
				SlideCount: 1,
			}, nil
		},
	}

	// Create service
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	service := NewConvertService(templatesDir, outputDir, templateService, mockPipe)

	wantScale := 3.5
	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Test Slide", Content: APIContent{Body: "Content here."}}},
		Options: &ConvertOptions{
			OutputFormat: "file",
			SVGScale:     wantScale,
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Execute
	service.ConvertHandler()(w, req)

	// Verify success
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify svg_scale was passed correctly
	if capturedSVGScale != wantScale {
		t.Errorf("Expected svg_scale %v to be passed to pipeline, got %v", wantScale, capturedSVGScale)
	}
}

// TestSVGScaleDefaultsToZero verifies that omitting svg_scale passes 0 to pipeline.
func TestSVGScaleDefaultsToZero(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create test template
	testTemplate := createTestTemplate(t, templatesDir, "test-template")
	defer func() { _ = os.Remove(testTemplate) }()

	// Track the svg_scale passed to pipeline
	var capturedSVGScale float64
	mockPipe := &mockPipeline{
		convertFunc: func(ctx context.Context, req pipeline.ConvertRequest) (*pipeline.ConvertResult, error) {
			capturedSVGScale = req.SVGScale

			// Create output file
			input, _ := os.ReadFile(testTemplate)
			_ = os.WriteFile(req.OutputPath, input, 0644)
			return &pipeline.ConvertResult{
				OutputPath: req.OutputPath,
				SlideCount: 1,
			}, nil
		},
	}

	// Create service
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(templatesDir, cache, false)
	service := NewConvertService(templatesDir, outputDir, templateService, mockPipe)

	reqBody := ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Test Slide", Content: APIContent{Body: "Content here."}}},
		Options: &ConvertOptions{
			OutputFormat: "file",
			// SVGScale intentionally not set
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Execute
	service.ConvertHandler()(w, req)

	// Verify success
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify svg_scale defaults to 0 (which tells pipeline/generator to use its own default)
	if capturedSVGScale != 0 {
		t.Errorf("Expected svg_scale to default to 0, got %v", capturedSVGScale)
	}
}

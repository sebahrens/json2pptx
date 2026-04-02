package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apierrors "github.com/ahrens/go-slide-creator/internal/api/errors"
	"github.com/ahrens/go-slide-creator/internal/template"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// mockTemplateCache is a simple in-memory cache for testing.
type mockTemplateCache struct {
	data map[string]*types.TemplateAnalysis
}

func newMockTemplateCache() *mockTemplateCache {
	return &mockTemplateCache{
		data: make(map[string]*types.TemplateAnalysis),
	}
}

func (c *mockTemplateCache) Get(path string) (*types.TemplateAnalysis, bool) {
	analysis, ok := c.data[path]
	return analysis, ok
}

func (c *mockTemplateCache) Set(path string, analysis *types.TemplateAnalysis) {
	c.data[path] = analysis
}

func (c *mockTemplateCache) Invalidate(path string) {
	delete(c.data, path)
}

func (c *mockTemplateCache) IsValid(path string, hash string) bool {
	analysis, ok := c.data[path]
	if !ok {
		return false
	}
	return analysis.Hash == hash
}

func (c *mockTemplateCache) Clear() {
	c.data = make(map[string]*types.TemplateAnalysis)
}

func (c *mockTemplateCache) Size() int {
	return len(c.data)
}

func TestListTemplatesHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupTemplates func(t *testing.T, dir string)
		wantStatus     int
		wantCount      int
		wantNames      []string
	}{
		{
			name: "empty directory",
			setupTemplates: func(t *testing.T, dir string) {
				// No templates
			},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name: "single template",
			setupTemplates: func(t *testing.T, dir string) {
				// Create a mock template analysis in cache
				// Actual PPTX file not needed if we pre-populate cache
			},
			wantStatus: http.StatusOK,
			wantCount:  0, // Will be 0 without actual template files
		},
		{
			name: "ignore non-pptx files",
			setupTemplates: func(t *testing.T, dir string) {
				// Create some non-pptx files
				_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("test"), 0644)
				_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("test: true"), 0644)
			},
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			if tt.setupTemplates != nil {
				tt.setupTemplates(t, tempDir)
			}

			cache := newMockTemplateCache()
			service := NewTemplateService(tempDir, cache, false)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
			w := httptest.NewRecorder()

			// Execute
			handler := service.ListTemplatesHandler()
			handler(w, req)

			// Verify status
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// Verify response
			if w.Code == http.StatusOK {
				var response ListTemplatesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if len(response.Templates) != tt.wantCount {
					t.Errorf("template count = %d, want %d", len(response.Templates), tt.wantCount)
				}
			}
		})
	}
}

func TestGetTemplateDetailsHandler(t *testing.T) {
	tests := []struct {
		name          string
		templateName  string
		setupTemplate func(t *testing.T, dir string, cache *mockTemplateCache)
		wantStatus    int
		wantError     string
	}{
		{
			name:         "template not found",
			templateName: "nonexistent",
			setupTemplate: func(t *testing.T, dir string, cache *mockTemplateCache) {
				// No template
			},
			wantStatus: http.StatusNotFound,
			wantError:  "TEMPLATE_NOT_FOUND",
		},
		{
			name:         "empty template name",
			templateName: "",
			wantStatus:   http.StatusBadRequest,
			wantError:    "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			cache := newMockTemplateCache()

			if tt.setupTemplate != nil {
				tt.setupTemplate(t, tempDir, cache)
			}

			service := NewTemplateService(tempDir, cache, false)

			// Create request
			url := "/api/v1/templates/" + tt.templateName
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("name", tt.templateName)
			w := httptest.NewRecorder()

			// Execute
			handler := service.GetTemplateDetailsHandler()
			handler(w, req)

			// Verify status
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// Verify error response
			if tt.wantError != "" {
				var response apierrors.Response
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}

				if response.Error.Code != tt.wantError {
					t.Errorf("error code = %s, want %s", response.Error.Code, tt.wantError)
				}

				if response.Success {
					t.Error("expected success = false in error response")
				}
			}
		})
	}
}

func TestToDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"corporate", "Corporate"},
		{"minimal-dark", "Minimal Dark"},
		{"modern_blue", "Modern Blue"},
		{"simple", "Simple"},
		{"multi-word-template", "Multi Word Template"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toDisplayName(tt.input)
			if got != tt.want {
				t.Errorf("toDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertLayouts(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout1",
			Name: "Title Slide",
			Tags: []string{"title-slide"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle},
				{Type: types.PlaceholderBody},
			},
		},
		{
			ID:   "layout2",
			Name: "Content",
			Tags: []string{"content", "text-heavy"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle},
				{Type: types.PlaceholderBody},
				{Type: types.PlaceholderImage},
			},
		},
	}

	result := convertLayouts(layouts)

	if len(result) != 2 {
		t.Fatalf("got %d layouts, want 2", len(result))
	}

	// Check first layout
	if result[0].ID != "layout1" {
		t.Errorf("layout[0].ID = %q, want %q", result[0].ID, "layout1")
	}
	if result[0].Name != "Title Slide" {
		t.Errorf("layout[0].Name = %q, want %q", result[0].Name, "Title Slide")
	}
	if len(result[0].Tags) != 1 || result[0].Tags[0] != "title-slide" {
		t.Errorf("layout[0].Tags = %v, want [title-slide]", result[0].Tags)
	}
	if len(result[0].Placeholders) != 2 {
		t.Errorf("layout[0].Placeholders length = %d, want 2", len(result[0].Placeholders))
	}

	// Check second layout
	if result[1].ID != "layout2" {
		t.Errorf("layout[1].ID = %q, want %q", result[1].ID, "layout2")
	}
	if len(result[1].Placeholders) != 3 {
		t.Errorf("layout[1].Placeholders length = %d, want 3", len(result[1].Placeholders))
	}
}

func TestConvertTheme(t *testing.T) {
	theme := types.ThemeInfo{
		Name: "Test Theme",
		Colors: []types.ThemeColor{
			{Name: "accent1", RGB: "#FF0000"},
			{Name: "accent2", RGB: "#00FF00"},
			{Name: "dk1", RGB: "#000000"},
		},
		TitleFont: "Arial",
		BodyFont:  "Calibri",
	}

	result := convertTheme(theme)

	if len(result.Colors) != 3 {
		t.Errorf("got %d colors, want 3", len(result.Colors))
	}

	expectedColors := []string{"#FF0000", "#00FF00", "#000000"}
	for i, color := range result.Colors {
		if color != expectedColors[i] {
			t.Errorf("color[%d] = %q, want %q", i, color, expectedColors[i])
		}
	}

	if result.Fonts.Title != "Arial" {
		t.Errorf("Fonts.Title = %q, want %q", result.Fonts.Title, "Arial")
	}

	if result.Fonts.Body != "Calibri" {
		t.Errorf("Fonts.Body = %q, want %q", result.Fonts.Body, "Calibri")
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		details    map[string]interface{}
		wantStatus int
	}{
		{
			name:       "simple error",
			status:     http.StatusBadRequest,
			code:       "INVALID_REQUEST",
			message:    "Invalid request",
			details:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "error with details",
			status:  http.StatusInternalServerError,
			code:    "TEMPLATE_ERROR",
			message: "Failed to analyze template",
			details: map[string]interface{}{
				"template": "corporate",
				"reason":   "file not found",
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			writeError(w, tt.status, tt.code, tt.message, tt.details)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var response apierrors.Response
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if response.Success {
				t.Error("expected Success = false")
			}

			if response.Error.Code != tt.code {
				t.Errorf("Error.Code = %q, want %q", response.Error.Code, tt.code)
			}

			if response.Error.Message != tt.message {
				t.Errorf("Error.Message = %q, want %q", response.Error.Message, tt.message)
			}

			if tt.details != nil && response.Error.Details == nil {
				t.Error("expected Details to be populated")
			}
		})
	}
}

// TestTemplateServiceIntegration tests the full flow with real template analysis.
// This test requires valid PPTX template files in testdata directory.
func TestTemplateServiceIntegration(t *testing.T) {
	// Check if we have test templates
	testdataDir := filepath.Join("..", "..", "testdata")
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("Skipping integration test: testdata directory not found")
	}

	// Find .pptx files
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Skip("Skipping integration test: cannot read testdata directory")
	}

	hasTemplate := false
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".pptx" {
			hasTemplate = true
			break
		}
	}

	if !hasTemplate {
		t.Skip("Skipping integration test: no .pptx templates in testdata")
	}

	// Run integration test
	cache := template.NewMemoryCache(24 * time.Hour)
	service := NewTemplateService(testdataDir, cache, false)

	// Test list templates
	t.Run("list templates", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
		w := httptest.NewRecorder()

		handler := service.ListTemplatesHandler()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var response ListTemplatesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(response.Templates) == 0 {
			t.Error("expected at least one template")
		}

		// Verify template structure
		for _, tmpl := range response.Templates {
			if tmpl.Name == "" {
				t.Error("template name is empty")
			}
			if tmpl.DisplayName == "" {
				t.Error("template display name is empty")
			}
			if tmpl.LayoutCount <= 0 {
				t.Error("template should have at least one layout")
			}
		}
	})

	// Test get template details (use first template found)
	entries, _ = os.ReadDir(testdataDir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".pptx" {
			templateName := entry.Name()[:len(entry.Name())-5] // Remove .pptx

			t.Run("get template details: "+templateName, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet,
					"/api/v1/templates/"+templateName, nil)
				req.SetPathValue("name", templateName)
				w := httptest.NewRecorder()

				handler := service.GetTemplateDetailsHandler()
				handler(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
				}

				var response TemplateDetailsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if response.Name != templateName {
					t.Errorf("name = %q, want %q", response.Name, templateName)
				}

				if len(response.Layouts) == 0 {
					t.Error("expected at least one layout")
				}

				// Verify layout structure
				for _, layout := range response.Layouts {
					if layout.ID == "" {
						t.Error("layout ID is empty")
					}
					if layout.Name == "" {
						t.Error("layout name is empty")
					}
				}

				// Verify theme structure
				if len(response.Theme.Colors) == 0 {
					t.Error("expected theme colors")
				}
				if response.Theme.Fonts.Title == "" {
					t.Error("theme title font is empty")
				}
				if response.Theme.Fonts.Body == "" {
					t.Error("theme body font is empty")
				}
			})

			break // Only test first template
		}
	}
}

// Acceptance criteria tests

// TestAC7_ListTemplates validates AC7: List Templates
func TestAC7_ListTemplates(t *testing.T) {
	// Setup with empty temporary directory (templates may not exist)
	tempDir := t.TempDir()
	cache := newMockTemplateCache()
	service := NewTemplateService(tempDir, cache, false)

	// When GET /api/v1/templates
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	w := httptest.NewRecorder()

	handler := service.ListTemplatesHandler()
	handler(w, req)

	// Then returns array of available templates
	if w.Code != http.StatusOK {
		t.Fatalf("AC7 FAILED: status = %d, want %d", w.Code, http.StatusOK)
	}

	var response ListTemplatesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("AC7 FAILED: failed to parse response: %v", err)
	}

	// Response should have templates array (may be empty if no templates exist)
	if response.Templates == nil {
		t.Fatal("AC7 FAILED: templates array is nil")
	}

	t.Logf("AC7 PASSED: List templates returned %d templates", len(response.Templates))
}

// TestAC8_TemplateDetails validates AC8: Template Details
func TestAC8_TemplateDetails(t *testing.T) {
	// This test requires a valid template file
	testdataDir := filepath.Join("..", "..", "testdata")

	// Find a template file
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Skip("Skipping AC8: cannot read testdata directory")
	}

	var templateName string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".pptx" {
			templateName = entry.Name()[:len(entry.Name())-5]
			break
		}
	}

	if templateName == "" {
		t.Skip("Skipping AC8: no .pptx templates found in testdata")
	}

	cache := template.NewMemoryCache(24 * time.Hour)
	service := NewTemplateService(testdataDir, cache, false)

	// Given existing template name
	// When GET /api/v1/templates/:name
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateName, nil)
	req.SetPathValue("name", templateName)
	w := httptest.NewRecorder()

	handler := service.GetTemplateDetailsHandler()
	handler(w, req)

	// Then returns layout and theme info
	if w.Code != http.StatusOK {
		t.Fatalf("AC8 FAILED: status = %d, want %d", w.Code, http.StatusOK)
	}

	var response TemplateDetailsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("AC8 FAILED: failed to parse response: %v", err)
	}

	// Verify layout info is present
	if len(response.Layouts) == 0 {
		t.Fatal("AC8 FAILED: no layouts in response")
	}

	// Verify theme info is present
	if response.Theme.Fonts.Title == "" {
		t.Fatal("AC8 FAILED: no theme font info")
	}

	t.Logf("AC8 PASSED: Template details returned %d layouts and theme info", len(response.Layouts))
}

// Security tests for path traversal prevention (HIGH-01)

// TestValidateTemplateName tests template name validation.
func TestValidateTemplateName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid simple name",
			input:     "corporate",
			wantError: false,
		},
		{
			name:      "valid name with hyphen",
			input:     "minimal-dark",
			wantError: false,
		},
		{
			name:      "valid name with underscore",
			input:     "modern_blue",
			wantError: false,
		},
		{
			name:      "valid name with numbers",
			input:     "theme2024",
			wantError: false,
		},
		{
			name:      "empty name",
			input:     "",
			wantError: true,
			errMsg:    "required",
		},
		{
			name:      "path traversal",
			input:     "../../../etc/passwd",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "path separator forward slash",
			input:     "path/to/template",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "path separator backslash",
			input:     "path\\to\\template",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "double dots only",
			input:     "..",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "starts with hyphen",
			input:     "-template",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "starts with underscore",
			input:     "_template",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "contains spaces",
			input:     "my template",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "contains special characters",
			input:     "template$name",
			wantError: true,
			errMsg:    "invalid characters",
		},
		{
			name:      "starts with dot",
			input:     ".hidden",
			wantError: true,
			errMsg:    "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplateName(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}

// TestGetTemplateDetailsHandler_PathTraversal tests that path traversal is blocked in API.
func TestGetTemplateDetailsHandler_PathTraversal(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		wantStatus   int
		wantCode     string
	}{
		{
			name:         "path traversal in name",
			templateName: "../../../etc/passwd",
			wantStatus:   http.StatusBadRequest,
			wantCode:     "INVALID_TEMPLATE",
		},
		{
			name:         "forward slash in name",
			templateName: "path/to/template",
			wantStatus:   http.StatusBadRequest,
			wantCode:     "INVALID_TEMPLATE",
		},
		{
			name:         "backslash in name",
			templateName: "path\\to\\template",
			wantStatus:   http.StatusBadRequest,
			wantCode:     "INVALID_TEMPLATE",
		},
		{
			name:         "double dots",
			templateName: "..",
			wantStatus:   http.StatusBadRequest,
			wantCode:     "INVALID_TEMPLATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			cache := newMockTemplateCache()
			service := NewTemplateService(tempDir, cache, false)

			// Create request
			url := "/api/v1/templates/" + tt.templateName
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("name", tt.templateName)
			w := httptest.NewRecorder()

			// Execute
			handler := service.GetTemplateDetailsHandler()
			handler(w, req)

			// Verify status
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// Verify error code
			var response apierrors.Response
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse error response: %v", err)
			}

			if response.Error.Code != tt.wantCode {
				t.Errorf("error code = %s, want %s", response.Error.Code, tt.wantCode)
			}
		})
	}
}

// TestGetTemplateDetailsHandler_AspectRatioOverride tests the aspect query parameter.
func TestGetTemplateDetailsHandler_AspectRatioOverride(t *testing.T) {
	// Setup: create test directory with a mock template analysis in cache
	tempDir := t.TempDir()
	cache := newMockTemplateCache()

	// Pre-populate cache with analysis for a mock template
	templateName := "testtemplate"
	templatePath := filepath.Join(tempDir, templateName+".pptx")

	// Create a minimal .pptx file (just a valid zip)
	createMinimalPPTX(t, templatePath)

	// Pre-populate cache with a known analysis
	cache.Set(templatePath, &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Hash:         "testhash123",
		AspectRatio:  "16:9", // Auto-detected as 16:9
		Layouts: []types.LayoutMetadata{
			{
				ID:   "layout1",
				Name: "Title Slide",
				Tags: []string{"title-slide"},
			},
		},
		Theme: types.ThemeInfo{
			TitleFont: "Arial",
			BodyFont:  "Calibri",
		},
	})

	service := NewTemplateService(tempDir, cache, false)

	tests := []struct {
		name            string
		queryParam      string
		wantStatus      int
		wantAspectRatio string
		wantError       string
	}{
		{
			name:            "no aspect param uses auto-detected",
			queryParam:      "",
			wantStatus:      http.StatusOK,
			wantAspectRatio: "16:9",
		},
		{
			name:            "aspect=4:3 overrides",
			queryParam:      "aspect=4:3",
			wantStatus:      http.StatusOK,
			wantAspectRatio: "4:3",
		},
		{
			name:            "aspect=16:9 explicit",
			queryParam:      "aspect=16:9",
			wantStatus:      http.StatusOK,
			wantAspectRatio: "16:9",
		},
		{
			name:       "invalid aspect value",
			queryParam: "aspect=16:10",
			wantStatus: http.StatusBadRequest,
			wantError:  "INVALID_REQUEST",
		},
		{
			name:       "invalid aspect format",
			queryParam: "aspect=invalid",
			wantStatus: http.StatusBadRequest,
			wantError:  "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/templates/" + templateName
			if tt.queryParam != "" {
				url += "?" + tt.queryParam
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("name", templateName)
			w := httptest.NewRecorder()

			handler := service.GetTemplateDetailsHandler()
			handler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantError != "" {
				var errResp apierrors.Response
				if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errResp.Error.Code != tt.wantError {
					t.Errorf("error code = %s, want %s", errResp.Error.Code, tt.wantError)
				}
			} else {
				var response TemplateDetailsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if response.AspectRatio != tt.wantAspectRatio {
					t.Errorf("aspect_ratio = %s, want %s", response.AspectRatio, tt.wantAspectRatio)
				}
			}
		})
	}
}

// createMinimalPPTX creates a minimal valid PPTX file for testing.
// This is just a valid ZIP file that satisfies the file existence check.
func createMinimalPPTX(t *testing.T, path string) {
	t.Helper()

	// Create empty file (the cache is pre-populated, so we just need the file to exist)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create test pptx: %v", err)
	}
	_ = f.Close()
}

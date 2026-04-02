package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apierrors "github.com/ahrens/go-slide-creator/internal/api/errors"
	"github.com/ahrens/go-slide-creator/internal/template"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// templateNameRegex validates that template names only contain safe characters.
var templateNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// TemplateAnalyzer provides template analysis functionality.
type TemplateAnalyzer interface {
	GetOrAnalyzeTemplate(templatePath string) (*types.TemplateAnalysis, error)
}

// ValidateTemplateName validates that a template name is safe to use.
// It prevents path traversal attacks by ensuring the name only contains
// alphanumeric characters, hyphens, and underscores.
func ValidateTemplateName(name string) error {
	if name == "" {
		return fmt.Errorf("template name is required")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("template name contains invalid characters")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("template name contains invalid characters")
	}
	if !templateNameRegex.MatchString(name) {
		return fmt.Errorf("template name contains invalid characters: only alphanumeric, hyphens, and underscores are allowed")
	}
	return nil
}

// TemplateService handles template analysis and caching.
type TemplateService struct {
	templatesDir   string
	cache          types.TemplateCache
	strictValidate bool
}

// NewTemplateService creates a new template service.
func NewTemplateService(templatesDir string, cache types.TemplateCache, strictValidate bool) *TemplateService {
	return &TemplateService{
		templatesDir:   templatesDir,
		cache:          cache,
		strictValidate: strictValidate,
	}
}

// ListTemplatesHandler handles GET /api/v1/templates requests.
func (ts *TemplateService) ListTemplatesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(ts.templatesDir)
		if err != nil {
			slog.Error("failed to read templates directory", "path", ts.templatesDir, "error", err)
			writeError(w, http.StatusInternalServerError, apierrors.CodeTemplateError,
				"Failed to read templates directory", nil)
			return
		}

		templates := make([]TemplateInfo, 0)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pptx") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".pptx")
			templatePath := filepath.Join(ts.templatesDir, entry.Name())

			analysis, err := ts.GetOrAnalyzeTemplate(templatePath)
			if err != nil {
				continue
			}

			templates = append(templates, TemplateInfo{
				Name:        name,
				DisplayName: toDisplayName(name),
				AspectRatio: analysis.AspectRatio,
				LayoutCount: len(analysis.Layouts),
			})
		}

		writeJSON(w, http.StatusOK, ListTemplatesResponse{Templates: templates})
	}
}

// GetTemplateDetailsHandler handles GET /api/v1/templates/{name} requests.
func (ts *TemplateService) GetTemplateDetailsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
				"Template name is required", nil)
			return
		}

		if err := ValidateTemplateName(name); err != nil {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidTemplate,
				err.Error(), nil)
			return
		}

		aspectOverride := r.URL.Query().Get("aspect")
		if aspectOverride != "" && aspectOverride != "16:9" && aspectOverride != "4:3" {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
				"Invalid aspect ratio: must be '16:9' or '4:3'", nil)
			return
		}

		templatePath := filepath.Join(ts.templatesDir, name+".pptx")

		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, apierrors.CodeTemplateNotFound,
				fmt.Sprintf("Template '%s' not found", name), nil)
			return
		}

		analysis, err := ts.GetOrAnalyzeTemplate(templatePath)
		if err != nil {
			slog.Error("failed to analyze template", "path", templatePath, "error", err)
			writeError(w, http.StatusInternalServerError, apierrors.CodeTemplateError,
				"Failed to analyze template", nil)
			return
		}

		aspectRatio := analysis.AspectRatio
		if aspectOverride != "" {
			aspectRatio = aspectOverride
		}

		response := TemplateDetailsResponse{
			Name:        name,
			DisplayName: toDisplayName(name),
			AspectRatio: aspectRatio,
			Layouts:     convertLayouts(analysis.Layouts),
			Theme:       convertTheme(analysis.Theme),
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// GetOrAnalyzeTemplate retrieves template analysis from cache or analyzes it.
func (ts *TemplateService) GetOrAnalyzeTemplate(templatePath string) (*types.TemplateAnalysis, error) {
	// Try fast validation if cache supports it
	if fastCache, ok := ts.cache.(types.FastValidationCache); ok {
		if cached, ok := fastCache.GetWithFastValidation(templatePath); ok {
			return cached, nil
		}
	} else {
		if cached, ok := ts.cache.Get(templatePath); ok {
			return cached, nil
		}
	}

	// Cache miss - analyze template
	var modTime time.Time
	if info, err := os.Stat(templatePath); err == nil {
		modTime = info.ModTime()
	}

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts: %w", err)
	}

	theme := template.ParseTheme(reader)

	validationResult := template.ValidateTemplateMetadata(reader, ts.strictValidate)
	if !validationResult.Valid {
		return nil, fmt.Errorf("template validation failed: %v", validationResult.AllIssues())
	}

	metadata := validationResult.Metadata
	template.ApplyMetadataHints(layouts, metadata)

	aspectRatio := "16:9"
	if metadata != nil && metadata.AspectRatio != "" {
		aspectRatio = metadata.AspectRatio
	}

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Hash:         reader.Hash(),
		AspectRatio:  aspectRatio,
		Layouts:      layouts,
		Theme:        theme,
		Metadata:     metadata,
	}

	// Synthesize missing layout capabilities (e.g., two-column)
	template.SynthesizeIfNeeded(reader, analysis)

	// Cache the analysis
	if fastCache, ok := ts.cache.(types.FastValidationCache); ok && !modTime.IsZero() {
		fastCache.SetWithModTime(templatePath, analysis, modTime)
	} else {
		ts.cache.Set(templatePath, analysis)
	}

	return analysis, nil
}

// toDisplayName converts a template name to a display name.
func toDisplayName(name string) string {
	displayName := strings.ReplaceAll(name, "-", " ")
	displayName = strings.ReplaceAll(displayName, "_", " ")

	words := strings.Fields(displayName)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// convertLayouts converts internal layout metadata to API response format.
func convertLayouts(layouts []types.LayoutMetadata) []LayoutSummary {
	result := make([]LayoutSummary, len(layouts))
	for i, layout := range layouts {
		placeholderTypes := make([]string, len(layout.Placeholders))
		for j, ph := range layout.Placeholders {
			placeholderTypes[j] = string(ph.Type)
		}

		result[i] = LayoutSummary{
			ID:           layout.ID,
			Name:         layout.Name,
			Tags:         layout.Tags,
			Placeholders: placeholderTypes,
		}
	}
	return result
}

// convertTheme converts internal theme info to API response format.
func convertTheme(theme types.ThemeInfo) ThemeSummary {
	colors := make([]string, len(theme.Colors))
	for i, color := range theme.Colors {
		colors[i] = color.RGB
	}

	return ThemeSummary{
		Colors: colors,
		Fonts: FontInfo{
			Title: theme.TitleFont,
			Body:  theme.BodyFont,
		},
	}
}

// writeJSON writes a JSON success response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response using the shared error format.
func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	apierrors.Write(w, status, code, message, details)
}

// Response types

// ListTemplatesResponse is the response for GET /api/v1/templates.
type ListTemplatesResponse struct {
	Templates []TemplateInfo `json:"templates"`
}

// TemplateInfo contains basic template metadata.
type TemplateInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	AspectRatio string `json:"aspect_ratio"`
	LayoutCount int    `json:"layout_count"`
}

// TemplateDetailsResponse is the response for GET /api/v1/templates/{name}.
type TemplateDetailsResponse struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	AspectRatio string          `json:"aspect_ratio"`
	Layouts     []LayoutSummary `json:"layouts"`
	Theme       ThemeSummary    `json:"theme"`
}

// LayoutSummary contains layout information for API responses.
type LayoutSummary struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Tags         []string `json:"tags"`
	Placeholders []string `json:"placeholders"`
}

// ThemeSummary contains theme information for API responses.
type ThemeSummary struct {
	Colors []string `json:"colors"`
	Fonts  FontInfo `json:"fonts"`
}

// FontInfo contains font information.
type FontInfo struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

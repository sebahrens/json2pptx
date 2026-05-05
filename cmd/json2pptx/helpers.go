package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// getOrAnalyzeTemplate retrieves cached template analysis or analyzes on demand.
func getOrAnalyzeTemplate(templatePath string, cache types.TemplateCache) (*types.TemplateAnalysis, error) {
	// Try cache first
	if cached, ok := cache.Get(templatePath); ok {
		return cached, nil
	}

	// Open template
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Parse layouts
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts: %w", err)
	}

	// Parse theme
	theme := template.ParseTheme(reader)

	// Validate metadata with soft mode (continue on warnings)
	validationResult := template.ValidateTemplateMetadata(reader, false)
	if !validationResult.Valid {
		return nil, fmt.Errorf("template validation failed: %v", validationResult.AllIssues())
	}

	// Apply metadata hints to layouts
	template.ApplyMetadataHints(layouts, validationResult.Metadata)

	// Determine aspect ratio
	aspectRatio := "16:9"
	if validationResult.Metadata != nil && validationResult.Metadata.AspectRatio != "" {
		aspectRatio = validationResult.Metadata.AspectRatio
	}

	// Extract actual slide dimensions from presentation.xml
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)

	// Extract table styles for validation.
	tblEntries := reader.TableStyles()
	tblStyles := make([]types.TableStyleInfo, len(tblEntries))
	for i, e := range tblEntries {
		tblStyles[i] = types.TableStyleInfo{ID: e.ID, Name: e.Name}
	}

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Hash:         reader.Hash(),
		AspectRatio:  aspectRatio,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        theme,
		Metadata:     validationResult.Metadata,
		TableStyles:  tblStyles,
	}

	// Synthesize missing layout capabilities (e.g., two-column layouts)
	_ = template.SynthesizeIfNeeded(reader, analysis)

	// Normalize placeholder names to canonical form (title, body, body_2, image, etc.)
	// This produces modified layout XML bytes stored in SyntheticFiles so the generator
	// reads normalized shapes. Also updates layout metadata placeholder IDs in-place.
	normalizedFiles, err := template.NormalizeLayoutFiles(reader, analysis.Layouts)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize layouts: %w", err)
	}
	if len(normalizedFiles) > 0 {
		if analysis.Synthesis == nil {
			analysis.Synthesis = &types.SynthesisManifest{
				SyntheticFiles: normalizedFiles,
			}
		} else {
			for path, data := range normalizedFiles {
				analysis.Synthesis.SyntheticFiles[path] = data
			}
		}
	}

	// Update layouts reference in case synthesis added new ones
	layouts = analysis.Layouts
	_ = layouts // used via analysis from here on

	// Cache for future use
	cache.Set(templatePath, analysis)

	return analysis, nil
}

// resolveDataPalette resolves TemplateMetadata.DataPalette scheme color names
// (e.g. "accent1", "accent5") to hex strings using the template's theme colors.
// Returns nil if metadata has no DataPalette or resolution fails.
func resolveDataPalette(metadata *types.TemplateMetadata, themeColors []types.ThemeColor) []string {
	if metadata == nil || len(metadata.DataPalette) == 0 {
		return nil
	}

	// Build lookup map from theme colors
	colorMap := make(map[string]string, len(themeColors))
	for _, tc := range themeColors {
		colorMap[tc.Name] = tc.RGB
	}

	resolved := make([]string, 0, len(metadata.DataPalette))
	for _, name := range metadata.DataPalette {
		if hex, ok := colorMap[name]; ok {
			// Ensure # prefix
			if !strings.HasPrefix(hex, "#") {
				hex = "#" + hex
			}
			resolved = append(resolved, hex)
		}
	}

	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

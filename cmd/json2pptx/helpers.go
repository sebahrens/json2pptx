package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// getOrAnalyzeTemplate retrieves cached template analysis or analyzes on demand.
func getOrAnalyzeTemplate(templatePath string, cache types.TemplateCache) (*types.TemplateAnalysis, error) {
	return template.Analyze(templatePath, cache, false)
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

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ahrens/svggen"
	"github.com/ahrens/go-slide-creator/internal/template"
	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestThemeColorsReachChart(t *testing.T) {
	templates := []struct {
		name       string
		path       string
		expectHex  string // First accent color hex that should appear in SVG
	}{
		{"warm-coral", "../../testdata/templates/warm-coral.pptx", "E64A19"}, // warm-coral red-orange
		{"forest-green", "../../testdata/templates/forest-green.pptx", "2E7D32"}, // forest-green green
	}

	for _, tt := range templates {
		t.Run(tt.name, func(t *testing.T) {
			// Extract theme colors from template
			f, err := os.Open(tt.path)
			if err != nil {
				t.Skipf("template not available: %v", err)
			}
			defer f.Close()
			stat, _ := f.Stat()
			zr, err := zip.NewReader(f, stat.Size())
			if err != nil {
				t.Fatalf("zip reader: %v", err)
			}
			themeInfo := template.ParseThemeFromZip(zr)

			// Convert to svggen ThemeColorInput (same as generator does)
			themeInputs := make([]svggen.ThemeColorInput, len(themeInfo.Colors))
			for i, tc := range themeInfo.Colors {
				themeInputs[i] = svggen.ThemeColorInput{Name: tc.Name, RGB: tc.RGB}
			}

			// Build a bar chart request with theme colors
			req := &svggen.RequestEnvelope{
				Type:  "bar_chart",
				Title: "Test Chart",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{42.0, 45.0, 48.0, 52.0}},
					},
				},
				Style: svggen.StyleSpec{
					ThemeColors: themeInputs,
				},
				Output: svggen.OutputSpec{
					Width:  600,
					Height: 400,
					Format: "svg",
					Scale:  2.0,
				},
			}

			result, err := svggen.RenderMultiFormat(req, "svg")
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			svgContent := string(result.SVG.Content)

			// Check if the template's accent1 color appears in the SVG
			defaultBlue := "4E79A7" // Tableau 10 default
			hasExpected := strings.Contains(strings.ToUpper(svgContent), strings.ToUpper(tt.expectHex))
			hasDefault := strings.Contains(strings.ToUpper(svgContent), strings.ToUpper(defaultBlue))

			t.Logf("Template: %s", tt.name)
			t.Logf("Expected accent1: #%s", tt.expectHex)
			t.Logf("Has expected color: %v", hasExpected)
			t.Logf("Has default blue: %v", hasDefault)

			// Also check what palette the style guide actually produces
			guide := svggen.StyleGuideFromSpec(req.Style)
			t.Logf("StyleGuide Accent1: %s", guide.Palette.Accent1.Hex())
			t.Logf("StyleGuide Accent2: %s", guide.Palette.Accent2.Hex())
			t.Logf("StyleGuide Accent3: %s", guide.Palette.Accent3.Hex())

			// Extract all fill colors from SVG
			for _, c := range []string{tt.expectHex, defaultBlue} {
				count := strings.Count(strings.ToUpper(svgContent), strings.ToUpper(c))
				t.Logf("  #%s appears %d times", c, count)
			}

			if !hasExpected && hasDefault {
				// Dump first 2000 chars of SVG for debug
				snippet := svgContent
				if len(snippet) > 3000 {
					snippet = snippet[:3000]
				}
				t.Logf("SVG snippet:\n%s", snippet)
				t.Errorf("chart uses default palette (#%s) instead of template color (#%s)", defaultBlue, tt.expectHex)
			}

			// Also do a direct render using the chart type's types.DiagramSpec path
			// (simulating what the generator does)
			spec := &types.DiagramSpec{
				Type:   "bar_chart",
				Title:  "Test",
				Width:  600,
				Height: 400,
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{42.0, 45.0, 48.0, 52.0}},
					},
				},
				Style: &types.DiagramStyle{
					ThemeColors: themeInfo.Colors,
				},
			}

			// Use the generator's RenderDiagramSpec path
			// (We can't import generator here without circular deps, so we'll simulate)
			fmt.Printf("[%s] Theme colors injected into DiagramSpec.Style.ThemeColors: %d colors\n",
				tt.name, len(spec.Style.ThemeColors))
		})
	}
}

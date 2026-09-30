package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// chartInsightSpec is a semantic deck whose only chart is the
// chart-insights-split pattern's shape_grid diagram cell.
var chartInsightSpec = map[string]any{
	"meta": map[string]any{"title": "Grid chart strategy", "template": "midnight-blue"},
	"slides": []any{map[string]any{
		"kind":    "chart_insight",
		"title":   "Revenue grew 41% this year, with EMEA driving the Q4 step-up",
		"insight": "Q4 acceleration reflects the EMEA launch landing ahead of schedule.",
		"chart": map[string]any{
			"type":  "bar_chart",
			"title": "Quarterly revenue ($M)",
			"data": map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series":     []any{map[string]any{"name": "Revenue", "values": []any{34, 40, 44, 48}}},
			},
		},
		"insights": []any{
			"Revenue grew 41% across the fiscal year.",
			"The Q4 step-up reflects the EMEA market launch.",
		},
	}},
}

func hasMediaExt(parts map[string][]byte, ext string) bool {
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") && filepath.Ext(name) == ext {
			return true
		}
	}
	return false
}

// maxMediaPNGWidth returns the widest PNG in ppt/media.
func maxMediaPNGWidth(t *testing.T, parts map[string][]byte) int {
	t.Helper()
	widest := 0
	for name, b := range parts {
		if !strings.HasPrefix(name, "ppt/media/") || filepath.Ext(name) != ".png" {
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		widest = max(widest, cfg.Width)
	}
	return widest
}

// go-slide-creator-4c9m7: pattern-embedded shape_grid diagrams obey the
// configured SVG strategy on both the semantic (render_deck_spec) and the raw
// (generate_presentation) MCP path, like placeholder charts do. The default
// native strategy still embeds the SVG.
func TestGridDiagram_HonorsSVGStrategy_SemanticAndRaw(t *testing.T) {
	rawDeck, err := os.ReadFile("../../examples/chart-insights-split.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var rawPresentation map[string]any
	if err := json.Unmarshal(rawDeck, &rawPresentation); err != nil {
		t.Fatalf("parse example: %v", err)
	}
	delete(rawPresentation, "output_filename")

	for _, strategy := range []types.SVGConversionStrategy{types.SVGStrategyPNG, types.SVGStrategyEMF, types.SVGStrategyNative} {
		t.Run(string(strategy), func(t *testing.T) {
			wantSVG := strategy == types.SVGStrategyNative
			mc := semanticTestConfig(t)
			mc.cfg.SVG.Strategy = strategy

			out := renderDeckSpecCall(t, mc, map[string]any{"spec": chartInsightSpec})
			if !out.Success {
				t.Fatalf("render_deck_spec failed: %s %+v", out.Error, out.Diagnostics)
			}
			semanticParts := pptxParts(t, out.PptxPath)
			if got := hasMediaExt(semanticParts, ".svg"); got != wantSVG {
				t.Errorf("semantic %s: svg media present=%v, want %v (media %v)", strategy, got, wantSVG, mediaExtensions(semanticParts))
			}
			if !hasMediaExt(semanticParts, ".png") {
				t.Errorf("semantic %s: chart must still be embedded (media %v)", strategy, mediaExtensions(semanticParts))
			}

			res := mustCall(t, mc.handleGenerate, map[string]any{"presentation": rawPresentation, "output_filename": "raw.pptx"})
			if res.IsError {
				t.Fatalf("generate_presentation failed: %s", textContent(res))
			}
			rawParts := pptxParts(t, filepath.Join(mc.outputDir, "raw.pptx"))
			if got := hasMediaExt(rawParts, ".svg"); got != wantSVG {
				t.Errorf("raw %s: svg media present=%v, want %v (media %v)", strategy, got, wantSVG, mediaExtensions(rawParts))
			}
		})
	}
}

// The raster a non-native strategy embeds for a grid diagram honors the
// configured MaxPNGWidth cap.
func TestGridDiagram_PNGStrategyHonorsMaxPNGWidth(t *testing.T) {
	mc := semanticTestConfig(t)
	mc.cfg.SVG.Strategy = types.SVGStrategyPNG
	mc.cfg.SVG.Scale = 4
	mc.cfg.SVG.MaxPNGWidth = 300
	out := renderDeckSpecCall(t, mc, map[string]any{"spec": chartInsightSpec})
	if !out.Success {
		t.Fatalf("render_deck_spec failed: %s %+v", out.Error, out.Diagnostics)
	}
	parts := pptxParts(t, out.PptxPath)
	if w := maxMediaPNGWidth(t, parts); w == 0 || w > 300 {
		t.Errorf("widest media PNG = %dpx, want 1..300 (MaxPNGWidth)", w)
	}
}

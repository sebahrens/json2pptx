package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDetectContrastPreflight_HexPair(t *testing.T) {
	pairs := []ContrastPreflightPair{
		{
			Path:       "/slides/0/shape_grid/rows/0/cells/0/shape/text",
			Foreground: "#FFFFFF",
			Background: "#FFE8D4", // pale background — white text fails contrast
			Source:     "shape_grid",
		},
	}
	findings := DetectContrastPreflight(pairs, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Code != patterns.ErrCodeContrastPredicted {
		t.Errorf("code = %q, want %q", f.Code, patterns.ErrCodeContrastPredicted)
	}
	if f.Action != "info" {
		t.Errorf("action = %q, want info", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "replace_color" {
		t.Errorf("fix should be replace_color, got %v", f.Fix)
	}
	if f.Fix.Params["source"] != "shape_grid" {
		t.Errorf("expected source=shape_grid, got %v", f.Fix.Params["source"])
	}
	if _, ok := f.Fix.Params["predicted_replacement"]; !ok {
		t.Errorf("fix params should include predicted_replacement")
	}
}

func TestDetectContrastPreflight_SchemeColorResolution(t *testing.T) {
	themeColors := []types.ThemeColor{
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent3", RGB: "#FFE8D4"}, // pale — white text on this fails contrast
	}
	pairs := []ContrastPreflightPair{
		{
			Path:       "/slides/0/shape_grid/rows/0/cells/0/shape/text",
			Foreground: "lt1",
			Background: "accent3",
		},
	}
	findings := DetectContrastPreflight(pairs, themeColors)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for low-contrast scheme pair, got %d", len(findings))
	}
}

func TestDetectContrastPreflight_HighContrastNoFinding(t *testing.T) {
	pairs := []ContrastPreflightPair{
		{
			Foreground: "#000000",
			Background: "#FFFFFF",
		},
	}
	findings := DetectContrastPreflight(pairs, nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for black-on-white, got %d", len(findings))
	}
}

func TestDetectContrastPreflight_UnresolvableSchemeSkipped(t *testing.T) {
	pairs := []ContrastPreflightPair{
		{
			Foreground: "accent99",
			Background: "#FFFFFF",
		},
	}
	findings := DetectContrastPreflight(pairs, nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for unresolved scheme, got %d", len(findings))
	}
}

func TestDetectContrastPreflight_Empty(t *testing.T) {
	if findings := DetectContrastPreflight(nil, nil); findings != nil {
		t.Errorf("expected nil for empty pairs, got %v", findings)
	}
}

// renderTimeShapeGridSwap runs the render-time shape-grid contrast pass on a
// minimal shape with an sRGB text color over an sRGB fill and returns the
// single ContrastSwap it produces (or fails the test).
func renderTimeShapeGridSwap(t *testing.T, fillHex, textHex string, themeColors []types.ThemeColor) ContrastSwap {
	t.Helper()
	shape := []byte(
		`<p:sp><p:spPr><a:solidFill><a:srgbClr val="` + fillHex + `"/></a:solidFill></p:spPr>` +
			`<p:txBody><a:p><a:r><a:rPr><a:solidFill><a:srgbClr val="` + textHex + `"/></a:solidFill></a:rPr>` +
			`<a:t>Hi</a:t></a:r></a:p></p:txBody></p:sp>`)
	_, swaps := fixShapeXMLContrast(shape, themeColors, nil)
	if len(swaps) != 1 {
		t.Fatalf("expected exactly 1 render-time swap, got %d (%v)", len(swaps), swaps)
	}
	return swaps[0]
}

// TestContrastPreflight_WhiteOnLightAccentMatchesRender is the core regression:
// white text on a light accent must be predicted as the same flipped color the
// renderer produces (dk1), not the muddy mid-gray EnsureContrast yields.
func TestContrastPreflight_WhiteOnLightAccentMatchesRender(t *testing.T) {
	themeColors := []types.ThemeColor{
		{Name: "dk1", RGB: "#1A1A1A"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent3", RGB: "#FFE8D4"},
	}

	swap := renderTimeShapeGridSwap(t, "FFE8D4", "FFFFFF", themeColors)

	findings := DetectContrastPreflight([]ContrastPreflightPair{{
		Foreground: "#FFFFFF",
		Background: "#FFE8D4",
		Source:     "shape_grid",
	}}, themeColors)
	if len(findings) != 1 {
		t.Fatalf("expected 1 preflight finding, got %d", len(findings))
	}

	predicted := findings[0].Fix.Params["predicted_replacement"]
	if predicted != swap.ReplacedColor {
		t.Errorf("predicted replacement %v != render-time swap %q", predicted, swap.ReplacedColor)
	}
	// White is a pure neutral, so the renderer flips to dk1 rather than lerping.
	if predicted != "#1A1A1A" {
		t.Errorf("expected flip to dk1 #1A1A1A, got %v", predicted)
	}
	if mode := findings[0].Fix.Params["replacement_mode"]; mode != contrastModeFlip {
		t.Errorf("replacement_mode = %v, want %q", mode, contrastModeFlip)
	}
}

// TestContrastPreflight_NonNeutralLowContrastMatchesRender covers the lerp
// branch: a non-neutral low-contrast foreground is lerped by EnsureContrast in
// both the renderer and the predictor, and they must agree.
func TestContrastPreflight_NonNeutralLowContrastMatchesRender(t *testing.T) {
	themeColors := []types.ThemeColor{
		{Name: "dk1", RGB: "#1A1A1A"},
		{Name: "lt1", RGB: "#FFFFFF"},
	}

	swap := renderTimeShapeGridSwap(t, "FFFFFF", "CCCCCC", themeColors)

	findings := DetectContrastPreflight([]ContrastPreflightPair{{
		Foreground: "#CCCCCC",
		Background: "#FFFFFF",
		Source:     "shape_grid",
	}}, themeColors)
	if len(findings) != 1 {
		t.Fatalf("expected 1 preflight finding, got %d", len(findings))
	}

	predicted := findings[0].Fix.Params["predicted_replacement"]
	if predicted != swap.ReplacedColor {
		t.Errorf("predicted replacement %v != render-time swap %q", predicted, swap.ReplacedColor)
	}
	if mode := findings[0].Fix.Params["replacement_mode"]; mode != contrastModeLerp {
		t.Errorf("replacement_mode = %v, want %q", mode, contrastModeLerp)
	}
}

// TestContrastPreflight_NeutralSchemeForegroundFlips verifies that a pure-neutral
// foreground authored as a scheme name (lt1) is classified as a flip just like
// the literal hex form — the preflight must match the render-time scheme path.
func TestContrastPreflight_NeutralSchemeForegroundFlips(t *testing.T) {
	themeColors := []types.ThemeColor{
		{Name: "dk1", RGB: "#1A1A1A"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent3", RGB: "#FFE8D4"},
	}
	findings := DetectContrastPreflight([]ContrastPreflightPair{{
		Foreground: "lt1",
		Background: "accent3",
		Source:     "shape_grid",
	}}, themeColors)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if mode := findings[0].Fix.Params["replacement_mode"]; mode != contrastModeFlip {
		t.Errorf("replacement_mode = %v, want %q", mode, contrastModeFlip)
	}
	if predicted := findings[0].Fix.Params["predicted_replacement"]; predicted != "#1A1A1A" {
		t.Errorf("expected flip to dk1 #1A1A1A, got %v", predicted)
	}
}

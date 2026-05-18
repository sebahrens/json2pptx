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

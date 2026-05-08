package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGenerateStylishPanelsGroupXML_ThreePanels(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strategy", body: "- Market expansion\n- Digital transformation"},
		{title: "Operations", body: "- Cost reduction\n- Process automation"},
		{title: "People", body: "- Talent acquisition\n- Culture shift"},
	}
	bounds := types.BoundingBox{X: 500000, Y: 1600000, Width: 11000000, Height: 4500000}

	result := generateStylishPanelsGroupXML(panels, bounds, 100)

	// Should be a group shape
	if !strings.HasPrefix(result, "<p:grpSp>") {
		t.Error("should start with <p:grpSp>")
	}
	if !strings.HasSuffix(result, "</p:grpSp>") {
		t.Error("should end with </p:grpSp>")
	}

	// Should contain accent band shapes (3)
	if count := strings.Count(result, `name="Accent Band"`); count != 3 {
		t.Errorf("expected 3 accent bands, got %d", count)
	}

	// Should contain body shapes (3)
	if count := strings.Count(result, `name="Panel Body"`); count != 3 {
		t.Errorf("expected 3 body rects, got %d", count)
	}

	// Should contain one ribbon
	if count := strings.Count(result, `name="Ribbon"`); count != 1 {
		t.Errorf("expected 1 ribbon, got %d", count)
	}

	// Should contain header text boxes (3)
	if count := strings.Count(result, `name="Panel Header"`); count != 3 {
		t.Errorf("expected 3 header text boxes, got %d", count)
	}

	// Should contain panel titles
	for _, panel := range panels {
		if !strings.Contains(result, panel.title) {
			t.Errorf("should contain title %q", panel.title)
		}
	}

	// Should contain bullet text
	if !strings.Contains(result, "Market expansion") {
		t.Error("should contain body bullet text")
	}

	// Should use accent2 for accent band fill
	if !strings.Contains(result, `schemeClr val="accent2"`) {
		t.Error("accent bands should use accent2 scheme color")
	}

	// Should use bg1 for ribbon fill
	if !strings.Contains(result, `schemeClr val="bg1"`) {
		t.Error("ribbon should use bg1 scheme color")
	}

	// Header text should use dk1 color
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("header text should use dk1 scheme color")
	}

	// Should have bullet formatting in body
	if !strings.Contains(result, "buChar") {
		t.Error("body text should contain bullet characters")
	}
}

func TestGenerateStylishPanelsGroupXML_Empty(t *testing.T) {
	result := generateStylishPanelsGroupXML(nil, types.BoundingBox{}, 100)
	if result != "" {
		t.Errorf("empty panels should return empty string, got %q", result)
	}
}

func TestGenerateStylishPanelsGroupXML_SinglePanel(t *testing.T) {
	panels := []nativePanelData{
		{title: "Solo", body: "- Only panel"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 4000000}

	result := generateStylishPanelsGroupXML(panels, bounds, 200)

	// Single panel: should still have all structural elements
	if count := strings.Count(result, `name="Accent Band"`); count != 1 {
		t.Errorf("expected 1 accent band, got %d", count)
	}
	if count := strings.Count(result, `name="Ribbon"`); count != 1 {
		t.Errorf("expected 1 ribbon, got %d", count)
	}
	if count := strings.Count(result, `name="Panel Header"`); count != 1 {
		t.Errorf("expected 1 header, got %d", count)
	}
}

func TestStylishPanelsEstimateShapeCount(t *testing.T) {
	panels := []nativePanelData{
		{title: "A"}, {title: "B"}, {title: "C"},
	}
	// 3 accents + 3 bodies + 1 ribbon + 3 headers + 1 group = 11
	got := stylishPanelsEstimateShapeCount(panels)
	if got != 11 {
		t.Errorf("expected 11 shape IDs for 3 panels, got %d", got)
	}
}

func TestIsPanelNativeLayout_StylishPanels(t *testing.T) {
	spec := &types.DiagramSpec{
		Type: "panel_layout",
		Data: map[string]any{"layout": "stylish_panels"},
	}
	if !isPanelNativeLayout(spec) {
		t.Error("stylish_panels should be recognized as native panel layout")
	}
}

func TestPanelLayoutMode_StylishPanels(t *testing.T) {
	spec := &types.DiagramSpec{
		Type: "panel_layout",
		Data: map[string]any{"layout": "stylish_panels"},
	}
	got := panelLayoutMode(spec)
	if got != "stylish_panels" {
		t.Errorf("expected 'stylish_panels', got %q", got)
	}
}

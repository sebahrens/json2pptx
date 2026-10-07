package generator

import (
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestIsNineBoxDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "nine_box_talent type",
			spec:     &types.DiagramSpec{Type: "nine_box_talent"},
			expected: true,
		},
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: false,
		},
		{
			name:     "panel_layout type",
			spec:     &types.DiagramSpec{Type: "panel_layout"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNineBoxDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isNineBoxDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// The nine cells are one ladder from neutral to accent along the diagonal:
// five score bands, each anti-diagonal one tone, lightness falling as the
// score rises (go-slide-creator-mhc3k).
func TestNineBoxLadderRunsFromNeutralToAccent(t *testing.T) {
	colors := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"},
		{Name: "accent1", RGB: "#FD5108"}, {Name: "accent2", RGB: "#2E7D32"}, {Name: "accent3", RGB: "#7E57C2"},
	}
	tints := nineBoxLadderTints(nativeSurface{colors: colors})
	if len(tints) != 9 {
		t.Fatalf("len(tints) = %d, want 9", len(tints))
	}
	// Row 0 is high potential, column 0 low performance: index 6 is the
	// lowest score, index 2 the highest.
	white := svggen.MustParseColor("#FFFFFF")
	lum := func(i int) float64 {
		base := svggen.MustParseColor(resolveSchemeColorToHex(tints[i].scheme, colors))
		return patterns.EffectiveColorMods(base, tints[i].mods(), white).Luminance()
	}
	for _, same := range [][]int{{3, 7}, {0, 4, 8}, {1, 5}} {
		for _, i := range same[1:] {
			if tints[i] != tints[same[0]] {
				t.Errorf("cells %d and %d share a score and must share a tone: %+v vs %+v", same[0], i, tints[same[0]], tints[i])
			}
		}
	}
	accent := nativeSurface{colors: colors}.accent()
	if tints[6].scheme != patterns.NeutralSurfaceColor {
		t.Errorf("the lowest band is %q, want the neutral surface", tints[6].scheme)
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 7, 8} {
		if tints[i].scheme != accent {
			t.Errorf("cell %d is %q, want a tint of the one accent %s", i, tints[i].scheme, accent)
		}
	}
	// Above the neutral corner every band is darker than the one before.
	order := []int{7, 8, 5, 2}
	for k := 1; k < len(order); k++ {
		if lum(order[k]) >= lum(order[k-1]) {
			t.Errorf("band %d (cell %d) is not darker than band %d (cell %d)", k+1, order[k], k, order[k-1])
		}
	}
	// Dark names stay readable on the deepest band.
	dark := svggen.MustParseColor("#000000")
	top := patterns.EffectiveColorMods(svggen.MustParseColor(resolveSchemeColorToHex(tints[2].scheme, colors)), tints[2].mods(), white)
	if ratio := dark.ContrastWith(top); ratio < svggen.WCAGAANormal {
		t.Errorf("dark text on the top band measures %.2f:1", ratio)
	}
}

func TestScanTemplateLoadsSemanticAccentsForNativeDiagrams(t *testing.T) {
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	ctx := newSinglePassContext(filepath.Join(t.TempDir(), "out.pptx"), nil, nil, true, nil)
	cleanup, err := ctx.initializeContext(templatePath)
	if err != nil {
		t.Fatalf("initializeContext() error = %v", err)
	}
	defer cleanup()
	if err := ctx.scanTemplate(); err != nil {
		t.Fatalf("scanTemplate() error = %v", err)
	}
	want := map[string]string{"positive": "accent4", "negative": "accent2", "neutral": "accent5"}
	for role, scheme := range want {
		if got := ctx.semanticAccents[role]; got != scheme {
			t.Errorf("semanticAccents[%q] = %q, want %q", role, got, scheme)
		}
	}
}

func TestParseNineBoxCells_CellsFormat(t *testing.T) {
	data := map[string]any{
		"cells": []any{
			map[string]any{
				"position": map[string]any{"row": float64(0), "col": float64(2)},
				"label":    "Stars",
				"items": []any{
					map[string]any{"name": "Alice"},
					map[string]any{"name": "Bob"},
				},
			},
			map[string]any{
				"position": map[string]any{"row": float64(1), "col": float64(1)},
				"label":    "Core",
				"items":    []any{"Charlie"},
			},
		},
	}

	cells := parseNineBoxCells(data)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}

	// First cell: Stars at (0,2)
	if cells[0].row != 0 || cells[0].col != 2 {
		t.Errorf("cell 0: expected (0,2), got (%d,%d)", cells[0].row, cells[0].col)
	}
	if cells[0].label != "Stars" {
		t.Errorf("cell 0: expected label 'Stars', got %q", cells[0].label)
	}
	if len(cells[0].items) != 2 {
		t.Errorf("cell 0: expected 2 items, got %d", len(cells[0].items))
	}

	// Second cell: Core at (1,1) with string item
	if cells[1].row != 1 || cells[1].col != 1 {
		t.Errorf("cell 1: expected (1,1), got (%d,%d)", cells[1].row, cells[1].col)
	}
	if len(cells[1].items) != 1 || cells[1].items[0] != "Charlie" {
		t.Errorf("cell 1: expected [Charlie], got %v", cells[1].items)
	}
}

func TestParseNineBoxCells_EmployeesFormat(t *testing.T) {
	data := map[string]any{
		"employees": []any{
			map[string]any{"name": "Alice", "performance": "high", "potential": "high"},
			map[string]any{"name": "Bob", "performance": "high", "potential": "high"},
			map[string]any{"name": "Carol", "performance": "low", "potential": "low"},
		},
	}

	cells := parseNineBoxCells(data)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells (grouped), got %d", len(cells))
	}

	// Find the high/high cell (row=0, col=2)
	found := false
	for _, c := range cells {
		if c.row == 0 && c.col == 2 {
			found = true
			if len(c.items) != 2 {
				t.Errorf("high/high cell: expected 2 items, got %d", len(c.items))
			}
		}
	}
	if !found {
		t.Error("did not find high/high cell at (0,2)")
	}
}

func TestEmployeeToGridPos(t *testing.T) {
	tests := []struct {
		performance, potential string
		wantRow, wantCol       int
	}{
		{"high", "high", 0, 2},
		{"low", "low", 2, 0},
		{"medium", "medium", 1, 1},
		{"high", "low", 2, 2},
		{"low", "high", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.performance+"_"+tt.potential, func(t *testing.T) {
			row, col := employeeToGridPos(tt.performance, tt.potential)
			if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("got (%d,%d), want (%d,%d)", row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestParseNineBoxEmployees_NumericLevels(t *testing.T) {
	// Agents commonly send a numeric 1-3 scale instead of low|medium|high.
	// These must route into the grid rather than collapsing to the center
	// "Core Employee" cell (go-slide-creator-pc2a).
	data := map[string]any{
		"employees": []any{
			map[string]any{"name": "Alice", "performance": float64(3), "potential": float64(3)},
			map[string]any{"name": "Bob", "performance": float64(1), "potential": float64(1)},
			map[string]any{"name": "Carol", "performance": float64(2), "potential": float64(2)},
		},
	}

	cells := parseNineBoxCells(data)

	got := map[string]struct{ row, col int }{}
	for _, c := range cells {
		for _, name := range c.items {
			got[name] = struct{ row, col int }{c.row, c.col}
		}
	}

	// performance->col (low=0,med=1,high=2), potential->row (high=0,med=1,low=2)
	want := map[string]struct{ row, col int }{
		"Alice": {0, 2}, // high/high
		"Bob":   {2, 0}, // low/low
		"Carol": {1, 1}, // medium/medium
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got (%d,%d), want (%d,%d)", name, got[name].row, got[name].col, w.row, w.col)
		}
	}
}

func TestNormalizeNineBoxLevel(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{"high", "high"},
		{"High", "high"},
		{" Medium ", "medium"},
		{"moderate", "medium"},
		{"1", "low"},
		{"2", "medium"},
		{"3", "high"},
		{float64(1), "low"},
		{float64(2), "medium"},
		{float64(3), "high"},
		{int(3), "high"},
		{1.5, "medium"},
		{"unknown", ""},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := normalizeNineBoxLevel(tt.in); got != tt.want {
			t.Errorf("normalizeNineBoxLevel(%v): got %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEncodeDecodeNineBoxAxes(t *testing.T) {
	xTitle := "Performance"
	yTitle := "Potential"
	xLabels := [3]string{"Low", "Medium", "High"}
	yLabels := [3]string{"Limited", "Moderate", "High"}

	encoded := encodeNineBoxAxes(xTitle, yTitle, xLabels, yLabels)
	gotXT, gotYT, gotXL, gotYL := decodeNineBoxAxes(encoded)

	if gotXT != xTitle {
		t.Errorf("xTitle: got %q, want %q", gotXT, xTitle)
	}
	if gotYT != yTitle {
		t.Errorf("yTitle: got %q, want %q", gotYT, yTitle)
	}
	if gotXL != xLabels {
		t.Errorf("xLabels: got %v, want %v", gotXL, xLabels)
	}
	if gotYL != yLabels {
		t.Errorf("yLabels: got %v, want %v", gotYL, yLabels)
	}
}

func TestGenerateNineBoxGroupXML_Basic(t *testing.T) {
	// Build the 10-panel input (1 axis + 9 cells).
	panels := make([]nativePanelData, 10)
	panels[0] = nativePanelData{
		title: "__nine_box_axes__",
		body:  encodeNineBoxAxes("Performance", "Potential", [3]string{"Low", "Medium", "High"}, [3]string{"Limited", "Moderate", "High"}),
	}
	for i := 1; i <= 9; i++ {
		row := (i - 1) / 3
		col := (i - 1) % 3
		panels[i] = nativePanelData{
			title: nineBoxDefaultLabels[row][col],
			body:  "",
		}
	}
	// Add some items to the Star cell (row=0, col=2 -> index 3).
	panels[3] = nativePanelData{
		title: "Star",
		body:  "- Alice\n- Bob",
	}

	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10000000, Height: 6000000}
	result := generateNineBoxGroupXML(panels, bounds, 100, nineBoxLadderTints(nativeSurface{}), nativeDiagramEnv{})

	if result == "" {
		t.Fatal("generateNineBoxGroupXML returned empty string")
	}

	// Should be well-formed XML.
	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Should contain group element.
	if !strings.Contains(result, "p:grpSp") {
		t.Error("should contain p:grpSp group element")
	}

	// Should contain group name.
	if !strings.Contains(result, "Nine Box Talent") {
		t.Error("should contain 'Nine Box Talent' group name")
	}

	// Should contain all 9 default cell labels.
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			label := nineBoxDefaultLabels[row][col]
			if !strings.Contains(result, label) {
				t.Errorf("should contain cell label %q", label)
			}
		}
	}

	// Should contain axis titles.
	if !strings.Contains(result, "Performance") {
		t.Error("should contain x-axis title 'Performance'")
	}
	if !strings.Contains(result, "Potential") {
		t.Error("should contain y-axis title 'Potential'")
	}

	// Should contain axis value labels.
	for _, label := range []string{"Low", "Medium", "High"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain axis label %q", label)
		}
	}

	// Should contain item names.
	if !strings.Contains(result, "Alice") {
		t.Error("should contain item name 'Alice'")
	}
	if !strings.Contains(result, "Bob") {
		t.Error("should contain item name 'Bob'")
	}

	// Should contain scheme color references (theme-aware).
	if !strings.Contains(result, "schemeClr") {
		t.Error("should use scheme colors for theme awareness")
	}

	// One surface style: square corners, as the patterns (go-slide-creator-amtkg).
	if strings.Contains(result, "roundRect") || !strings.Contains(result, `prst="rect"`) {
		t.Error("cards should be square-cornered rects, not roundRect")
	}
}

func TestGenerateNineBoxGroupXML_WrongPanelCount(t *testing.T) {
	panels := []nativePanelData{{title: "only one"}}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateNineBoxGroupXML(panels, bounds, 100, nil, nativeDiagramEnv{})
	if result != "" {
		t.Error("expected empty string for wrong panel count")
	}
}

func TestParseNineBoxAxisLabels(t *testing.T) {
	data := map[string]any{
		"x_axis_labels": []any{"Developing", "Effective", "Outstanding"},
	}

	labels := parseNineBoxAxisLabels(data, "x_axis_labels")
	if labels != [3]string{"Developing", "Effective", "Outstanding"} {
		t.Errorf("unexpected labels: %v", labels)
	}

	// Missing key returns empty.
	empty := parseNineBoxAxisLabels(data, "y_axis_labels")
	if empty != [3]string{} {
		t.Errorf("expected empty labels for missing key, got %v", empty)
	}
}

func TestGenerateNineBoxGroupXML_NoAxes(t *testing.T) {
	// Build panels with empty axis data.
	panels := make([]nativePanelData, 10)
	panels[0] = nativePanelData{
		title: "__nine_box_axes__",
		body:  encodeNineBoxAxes("", "", [3]string{}, [3]string{}),
	}
	for i := 1; i <= 9; i++ {
		panels[i] = nativePanelData{title: "Cell", body: ""}
	}

	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}
	result := generateNineBoxGroupXML(panels, bounds, 100, nineBoxLadderTints(nativeSurface{}), nativeDiagramEnv{})

	if result == "" {
		t.Fatal("should produce valid XML even without axis labels")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

package patterns

import (
	"strings"
	"testing"
)

func TestWaterfallBridge_Registration(t *testing.T) {
	p, ok := Default().Get("waterfall-bridge")
	if !ok {
		t.Fatal("expected waterfall-bridge to be registered in default registry")
	}
	if p.Name() != "waterfall-bridge" {
		t.Errorf("Name() = %q, want %q", p.Name(), "waterfall-bridge")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validWaterfallBridgeValues() *WaterfallBridgeValues {
	return &WaterfallBridgeValues{
		Unit: "$m",
		Columns: []WaterfallBridgeColumn{
			{Label: "Revenue", Value: 120, Type: "total"},
			{Label: "COGS", Value: -45, Type: "delta"},
			{Label: "Gross Profit", Type: "subtotal"},
			{Label: "OpEx", Value: -30, Type: "delta"},
			{Label: "EBITDA", Value: 45, Type: "total"},
		},
	}
}

func TestWaterfallBridge_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	if err := p.Validate(validWaterfallBridgeValues(), nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestWaterfallBridge_Validate_TooFewColumns(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Columns: []WaterfallBridgeColumn{
			{Label: "Start", Value: 100, Type: "total"},
			{Label: "End", Value: 90, Type: "total"},
		},
	}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 columns")
	}
	if !strings.Contains(err.Error(), "kpi-Nup") && !strings.Contains(err.Error(), "stat-hero") {
		t.Errorf("expected sibling hint mentioning kpi-Nup or stat-hero, got: %v", err)
	}
}

func TestWaterfallBridge_Validate_TooManyColumns(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	cols := make([]WaterfallBridgeColumn, 11)
	for i := range cols {
		cols[i] = WaterfallBridgeColumn{Label: "X", Value: 10, Type: "delta"}
	}
	v := &WaterfallBridgeValues{Columns: cols}
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 10 columns")
	}
}

func TestWaterfallBridge_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	v.Columns[1].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestWaterfallBridge_Validate_MissingType(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	v.Columns[2].Type = ""
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for empty type")
	}
}

func TestWaterfallBridge_Validate_UnknownType(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	v.Columns[2].Type = "running" // invalid
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for unknown type")
	}
	if !strings.Contains(err.Error(), "total") {
		t.Errorf("expected error to enumerate allowed types, got: %v", err)
	}
}

func TestWaterfallBridge_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	v.Columns[0].Label = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for label > 40 chars")
	}
}

func TestWaterfallBridge_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	overrides := map[int]any{99: &WaterfallBridgeCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestWaterfallBridge_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 2 {
		t.Fatalf("expected 2 rows (bars + labels), got %d", got)
	}
	if got := len(grid.Rows[0].Cells); got != 5 {
		t.Errorf("expected 5 bar cells, got %d", got)
	}
	if got := len(grid.Rows[1].Cells); got != 5 {
		t.Errorf("expected 5 label cells, got %d", got)
	}
	if string(grid.Columns) != "5" {
		t.Errorf("expected columns=5, got %s", string(grid.Columns))
	}
}

// TestWaterfallBridge_Expand_FloatingBars verifies that delta columns produce
// floating bars (top spacer > 0) while total columns anchor to baseline
// (bottom spacer == 0).
func TestWaterfallBridge_Expand_FloatingBars(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Column 0 = Revenue (total, value 120). Bar should anchor to baseline:
	// the sub-grid should have a top spacer and a bar; no meaningful bottom
	// spacer (yMin = -? Let's check below).
	// Actually with the values {120, -45, 75 subtotal, -30, 45}:
	//   running: 120, 75, 75, 45, 45
	//   yMin = 0, yMax = 120, scale = 120.
	//   Revenue total 0..120: top spacer 0%, bar 100%, bottom 0% → just 1 sub-row.
	revenue := grid.Rows[0].Cells[0]
	if revenue.Grid == nil {
		t.Fatal("expected sub-grid on revenue bar cell")
	}
	if len(revenue.Grid.Rows) != 1 {
		t.Errorf("revenue total 0..max should have 1 sub-row (bar only), got %d", len(revenue.Grid.Rows))
	}

	// Column 1 = COGS (delta, -45). yStart=120, yEnd=75. Bar from 75..120, so:
	//   topPct = (120-120)/120 = 0%, barPct = 45/120 ≈ 37.5%, bottomPct = 75/120 ≈ 62.5%.
	//   → 2 sub-rows: bar + bottom spacer.
	cogs := grid.Rows[0].Cells[1]
	if cogs.Grid == nil {
		t.Fatal("expected sub-grid on COGS bar cell")
	}
	if len(cogs.Grid.Rows) != 2 {
		t.Errorf("COGS delta should have 2 sub-rows (bar + bottom spacer), got %d", len(cogs.Grid.Rows))
	}

	// Column 3 = OpEx (delta, -30). yStart=75, yEnd=45. Bar from 45..75:
	//   topPct = (120-75)/120 = 37.5%, barPct = 30/120 = 25%, bottomPct = 45/120 = 37.5%.
	//   → 3 sub-rows: top spacer + bar + bottom spacer (the floating bar case).
	opex := grid.Rows[0].Cells[3]
	if opex.Grid == nil {
		t.Fatal("expected sub-grid on OpEx bar cell")
	}
	if len(opex.Grid.Rows) != 3 {
		t.Errorf("OpEx floating delta should have 3 sub-rows (top + bar + bottom), got %d", len(opex.Grid.Rows))
	}
}

// TestWaterfallBridge_Expand_SubtotalAutoComputed verifies subtotal columns
// take the running total even when value is omitted in the input.
func TestWaterfallBridge_Expand_SubtotalAutoComputed(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	// v.Columns[2] is "Gross Profit", a subtotal with no Value set.
	if v.Columns[2].Value != 0 {
		t.Fatalf("test precondition: subtotal value should be 0 in input, got %v", v.Columns[2].Value)
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Running total after Revenue (120) + COGS (-45) = 75.
	// The subtotal bar's value label should read "75" (no sign for subtotals).
	gross := grid.Rows[0].Cells[2]
	if gross.Grid == nil {
		t.Fatal("expected sub-grid on subtotal bar cell")
	}
	var subtotalShapeText string
	for _, row := range gross.Grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				subtotalShapeText = string(cell.Shape.Text)
			}
		}
	}
	if !strings.Contains(subtotalShapeText, "75") {
		t.Errorf("expected subtotal label to include running total 75, got %q", subtotalShapeText)
	}
	// Subtotals should NOT carry a + / - sign.
	if strings.Contains(subtotalShapeText, "+75") || strings.Contains(subtotalShapeText, "−75") {
		t.Errorf("subtotal label should not carry a signed prefix, got %q", subtotalShapeText)
	}
}

// TestWaterfallBridge_Expand_NegativeDeltaFill verifies negative-delta bars use
// the negative_accent (default accent2), while positive deltas use the base
// accent.
func TestWaterfallBridge_Expand_NegativeDeltaFill(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Columns: []WaterfallBridgeColumn{
			{Label: "Start", Value: 100, Type: "total"},
			{Label: "Drop", Value: -20, Type: "delta"},
			{Label: "Gain", Value: 30, Type: "delta"},
			{Label: "End", Value: 110, Type: "total"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Helper: pull the bar shape (the cell with bold text) out of a column.
	barFill := func(col int) string {
		c := grid.Rows[0].Cells[col]
		if c.Grid == nil {
			return ""
		}
		for _, row := range c.Grid.Rows {
			for _, cell := range row.Cells {
				if cell.Shape != nil && len(cell.Shape.Text) > 0 {
					return string(cell.Shape.Fill)
				}
			}
		}
		return ""
	}
	dropFill := barFill(1)
	gainFill := barFill(2)
	if !strings.Contains(dropFill, "accent2") {
		t.Errorf("expected negative delta to fill with accent2, got %q", dropFill)
	}
	if !strings.Contains(gainFill, "accent1") {
		t.Errorf("expected positive delta to fill with base accent (accent1), got %q", gainFill)
	}
}

// TestWaterfallBridge_Expand_SubtotalFill verifies subtotal columns use the
// distinct subtotal_accent (default accent3).
func TestWaterfallBridge_Expand_SubtotalFill(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	grid, err := p.Expand(ExpandContext{}, validWaterfallBridgeValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	c := grid.Rows[0].Cells[2] // Gross Profit subtotal
	var fill string
	for _, row := range c.Grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				fill = string(cell.Shape.Fill)
			}
		}
	}
	if !strings.Contains(fill, "accent3") {
		t.Errorf("expected subtotal bar to fill with accent3, got %q", fill)
	}
}

func TestWaterfallBridge_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	overrides := &WaterfallBridgeOverrides{TextOverrides: TextOverrides{Accent: "accent4"}}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Revenue (total) should now be accent4.
	c := grid.Rows[0].Cells[0]
	var fill string
	for _, row := range c.Grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				fill = string(cell.Shape.Fill)
			}
		}
	}
	if !strings.Contains(fill, "accent4") {
		t.Errorf("expected total bar to follow accent override (accent4), got %q", fill)
	}
}

func TestWaterfallBridge_Expand_NegativeAccentOverride(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Columns: []WaterfallBridgeColumn{
			{Label: "Start", Value: 100, Type: "total"},
			{Label: "Drop", Value: -20, Type: "delta"},
			{Label: "End", Value: 80, Type: "total"},
		},
	}
	overrides := &WaterfallBridgeOverrides{NegativeAccent: "accent5"}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	c := grid.Rows[0].Cells[1] // negative delta
	var fill string
	for _, row := range c.Grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				fill = string(cell.Shape.Fill)
			}
		}
	}
	if !strings.Contains(fill, "accent5") {
		t.Errorf("expected negative delta to follow negative_accent override (accent5), got %q", fill)
	}
}

func TestWaterfallBridge_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := validWaterfallBridgeValues()
	cellOverrides := map[int]any{1: &WaterfallBridgeCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[1].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on label cell with cell override")
	}
}

func TestWaterfallBridge_Expand_DeltaValueSign(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Unit: "M",
		Columns: []WaterfallBridgeColumn{
			{Label: "Start", Value: 50, Type: "total"},
			{Label: "Up", Value: 20, Type: "delta"},
			{Label: "Down", Value: -10, Type: "delta"},
			{Label: "End", Value: 60, Type: "total"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	barText := func(col int) string {
		c := grid.Rows[0].Cells[col]
		if c.Grid == nil {
			return ""
		}
		for _, row := range c.Grid.Rows {
			for _, cell := range row.Cells {
				if cell.Shape != nil && len(cell.Shape.Text) > 0 {
					return string(cell.Shape.Text)
				}
			}
		}
		return ""
	}
	upText := barText(1)
	downText := barText(2)
	startText := barText(0)
	if !strings.Contains(upText, "+20M") {
		t.Errorf("expected positive delta label to contain '+20M', got %q", upText)
	}
	if !strings.Contains(downText, "−10M") {
		t.Errorf("expected negative delta label to contain '−10M', got %q", downText)
	}
	// Totals do not carry sign prefixes.
	if strings.Contains(startText, "+50") || strings.Contains(startText, "−50") {
		t.Errorf("total label should not carry signed prefix, got %q", startText)
	}
}

func TestWaterfallBridge_Schema(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestWaterfallBridge_Taxonomy(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	tax := p.Taxonomy()
	if tax.Category != "data-display" {
		t.Errorf("Category = %q, want %q", tax.Category, "data-display")
	}
	if tax.DensityClass != "medium" {
		t.Errorf("DensityClass = %q, want %q", tax.DensityClass, "medium")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("expected NarrativeRole to be non-empty")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("expected PairsWith to be non-empty")
	}
}

func TestWaterfallBridge_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"p&l walk", "P&L walk from revenue to EBITDA", &ContentHints{ItemCount: 5}},
		{"cost bridge", "cost driver bridge showing components of cost variance", &ContentHints{ItemCount: 6}},
		{"gap to target", "gap to target waterfall with subtotals", &ContentHints{ItemCount: 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "waterfall-bridge" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected waterfall-bridge in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}

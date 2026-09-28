package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestWaterfallBridgeTenColumnLabelWarning(t *testing.T) {
	p := &waterfallBridge{}
	v := &WaterfallBridgeValues{}
	for i := 0; i < 10; i++ {
		v.Columns = append(v.Columns, WaterfallBridgeColumn{Label: "Driver", Value: 10, Type: wbTypeDelta})
	}
	v.Columns[0].Type = wbTypeTotal
	v.Columns[0].Value = 100
	v.Columns[9].Type = wbTypeSubtotal
	v.Columns[4].Label = strings.Repeat("L", 11)
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "columns[4].label") || !strings.Contains(got[0], "about 10") {
		t.Fatalf("dense label warning: %v", got)
	}
	v.Columns[4].Label = strings.Repeat("L", 10)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured ten-column target should fit: %v", got)
	}
	v.Columns = v.Columns[:9]
	v.Columns[4].Label = strings.Repeat("L", 12)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "9 columns hold about 11") {
		t.Fatalf("nine-column label warning: %v", got)
	}
	v.Columns[4].Label = strings.Repeat("L", 11)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured nine-column target should fit: %v", got)
	}
	v.Columns = v.Columns[:3]
	v.Columns[1].Label = strings.Repeat("L", wbLabelMax)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("three-column schema maximum should fit: %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

// wbStructuralRows counts a column sub-grid's spacer/bar rows, ignoring the
// hairline bridge-line rows.
func wbStructuralRows(g *jsonschema.ShapeGridInput) int {
	n := 0
	for _, r := range g.Rows {
		if r.Height >= 0.5 {
			n++
		}
	}
	return n
}

// wbIsBridgeLine reports whether a sub-grid row is a bridge-line hairline.
func wbIsBridgeLine(r jsonschema.GridRowInput) bool {
	if r.Height >= 0.5 {
		return false
	}
	for _, c := range r.Cells {
		if c.Shape != nil && strings.Contains(string(c.Shape.Fill), "lumMod") {
			return true
		}
	}
	return false
}

func TestFormatWaterfallBridgeValue_UnitPlacement(t *testing.T) {
	cases := []struct {
		v      float64
		unit   string
		signed bool
		want   string
	}{
		{210, "$m", false, "$210m"},
		{35, "$m", true, "+$35m"},
		{-41, "$m", true, "−$41m"},
		{12.5, "€bn", true, "+€12.5bn"},
		{3, "US$m", false, "US$3m"},
		{4, "%", true, "+4%"},
		{-2, "pts", true, "−2\u2009pts"}, // thin space (go-slide-creator-d6zo)
		{7, "m", false, "7m"},
		{9, "", false, "9"},
	}
	for _, c := range cases {
		if got := formatWaterfallBridgeValue(c.v, c.unit, c.signed); got != c.want {
			t.Errorf("format(%v, %q, %v) = %q, want %q", c.v, c.unit, c.signed, got, c.want)
		}
	}
}

// Thin bars carry their value label outside the bar (in dark text) and every
// adjacent pair of columns is joined by a bridge line at the shared level.
func TestWaterfallBridge_Expand_ThinBarLabelsAndBridgeLines(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Unit: "$m",
		Columns: []WaterfallBridgeColumn{
			{Label: "FY25", Value: 210, Type: "total"},
			{Label: "Volume", Value: 35, Type: "delta"},
			{Label: "Mix", Value: -1, Type: "delta"},
			{Label: "FY26", Value: 244, Type: "total"},
		},
	}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	mix := grid.Rows[0].Cells[2].Grid
	var barText, outsideText string
	for _, r := range mix.Rows {
		for _, c := range r.Cells {
			if c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			if strings.Contains(string(c.Shape.Fill), "accent2") {
				barText = string(c.Shape.Text)
			} else {
				outsideText = string(c.Shape.Text)
			}
		}
	}
	if barText != "" {
		t.Errorf("thin bar should not carry its label inside, got %s", barText)
	}
	if !strings.Contains(outsideText, "−$1m") || !strings.Contains(outsideText, `"dk1"`) {
		t.Errorf("thin bar label should sit outside in dark text, got %q", outsideText)
	}

	// Bridge lines: first column leaves, last column enters, middle both.
	for i, cell := range grid.Rows[0].Cells {
		lines := 0
		for _, r := range cell.Grid.Rows {
			if wbIsBridgeLine(r) {
				lines++
			}
		}
		if lines == 0 {
			t.Errorf("column %d has no bridge line", i)
		}
	}
}

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
	if wbStructuralRows(revenue.Grid) != 1 {
		t.Errorf("revenue total 0..max should have 1 sub-row (bar only), got %d", wbStructuralRows(revenue.Grid))
	}

	// Column 1 = COGS (delta, -45). yStart=120, yEnd=75. Bar from 75..120, so:
	//   topPct = (120-120)/120 = 0%, barPct = 45/120 ≈ 37.5%, bottomPct = 75/120 ≈ 62.5%.
	//   → 2 sub-rows: bar + bottom spacer.
	cogs := grid.Rows[0].Cells[1]
	if cogs.Grid == nil {
		t.Fatal("expected sub-grid on COGS bar cell")
	}
	if wbStructuralRows(cogs.Grid) != 2 {
		t.Errorf("COGS delta should have 2 sub-rows (bar + bottom spacer), got %d", wbStructuralRows(cogs.Grid))
	}

	// Column 3 = OpEx (delta, -30). yStart=75, yEnd=45. Bar from 45..75:
	//   topPct = (120-75)/120 = 37.5%, barPct = 30/120 = 25%, bottomPct = 45/120 = 37.5%.
	//   → 3 sub-rows: top spacer + bar + bottom spacer (the floating bar case).
	opex := grid.Rows[0].Cells[3]
	if opex.Grid == nil {
		t.Fatal("expected sub-grid on OpEx bar cell")
	}
	if wbStructuralRows(opex.Grid) != 3 {
		t.Errorf("OpEx floating delta should have 3 sub-rows (top + bar + bottom), got %d", wbStructuralRows(opex.Grid))
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

// TestWaterfallBridge_Expand_NegativeDeltaFill verifies the sdxii fills: a
// decrease carries the story in accent1, an increase is neutral dk1 at 35%.
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
	if dropFill != `"accent1"` {
		t.Errorf("expected the decrease to fill with accent1, got %q", dropFill)
	}
	if gainFill != `{"color":"dk1","lumMod":35000,"lumOff":65000}` {
		t.Errorf("expected the increase to fill with neutral dk1 at 35%%, got %q", gainFill)
	}
}

// TestWaterfallBridge_Expand_SubtotalFill verifies subtotal columns are
// neutral dk1 at 60%, like totals, unless subtotal_accent is set.
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
	if fill != `{"color":"dk1","lumMod":60000,"lumOff":40000}` {
		t.Errorf("expected subtotal bar to fill with neutral dk1 at 60%%, got %q", fill)
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
	// The accent override recolours the decreases (column 1, COGS); the
	// Revenue total stays neutral.
	if fill := waterfallBarFillForLabel(t, grid, "$120m"); fill == "accent4" {
		t.Errorf("a total must stay neutral under an accent override, got %q", fill)
	}
	c := grid.Rows[0].Cells[1]
	var fill string
	for _, row := range c.Grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				fill = string(cell.Shape.Fill)
			}
		}
	}
	if !strings.Contains(fill, "accent4") {
		t.Errorf("expected the decrease to follow the accent override (accent4), got %q", fill)
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

// go-slide-creator-sdxii: the bridge's story is its decreases, so they take
// accent1 whatever semantic accents the template declares; totals and
// increases stay neutral dk1 tints (supersedes go-slide-creator-noa7).
func TestWaterfallBridge_SemanticAccentsDriveFills(t *testing.T) {
	pat, ok := Default().Get("waterfall-bridge")
	if !ok {
		t.Fatal("waterfall-bridge not registered")
	}

	values := &WaterfallBridgeValues{
		Unit: "EUR m",
		Columns: []WaterfallBridgeColumn{
			{Label: "FY24", Value: 210, Type: "total"},
			{Label: "Price", Value: 31, Type: "delta"},
			{Label: "Volume", Value: -31, Type: "delta"},
			{Label: "FY25", Value: 210, Type: "total"},
		},
	}

	for _, semantic := range []map[string]string{
		{"negative": "accent1", "positive": "accent3", "neutral": "accent4"},
		{"negative": "accent2", "positive": "accent4", "neutral": "accent5"},
		nil,
	} {
		ctx := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
		if semantic != nil {
			ctx.Metadata = &types.TemplateMetadata{SemanticAccents: semantic}
		}
		grid, err := pat.Expand(ctx, values, nil, nil)
		if err != nil {
			t.Fatalf("expand: %v", err)
		}
		if got := waterfallBarFillForLabel(t, grid, "−31"); got != "accent1" {
			t.Errorf("%v: decrease fill = %q, want accent1", semantic, got)
		}
		if got := waterfallBarFillForLabel(t, grid, "+31"); got != `{"color":"dk1","lumMod":35000,"lumOff":65000}` {
			t.Errorf("%v: increase fill = %q, want neutral dk1 at 35%%", semantic, got)
		}
	}
}

// An explicit negative_accent override still wins over the template palette.
func TestWaterfallBridge_ExplicitNegativeAccentWins(t *testing.T) {
	pat, _ := Default().Get("waterfall-bridge")
	values := &WaterfallBridgeValues{
		Columns: []WaterfallBridgeColumn{
			{Label: "FY24", Value: 210, Type: "total"},
			{Label: "Volume", Value: -31, Type: "delta"},
		},
	}
	ctx := ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		Metadata:    &types.TemplateMetadata{SemanticAccents: map[string]string{"negative": "accent1"}},
	}
	grid, err := pat.Expand(ctx, values, &WaterfallBridgeOverrides{NegativeAccent: "accent6"}, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got := waterfallBarFillForLabel(t, grid, "−31"); got != "accent6" {
		t.Errorf("explicit negative_accent override = %q, want accent6", got)
	}
}

// waterfallBarFillForLabel finds the bar shape whose value text contains the
// given label fragment and returns its fill colour string.
func waterfallBarFillForLabel(t *testing.T, grid *jsonschema.ShapeGridInput, labelFragment string) string {
	t.Helper()
	var found string
	var walk func(g *jsonschema.ShapeGridInput)
	walk = func(g *jsonschema.ShapeGridInput) {
		if g == nil || found != "" {
			return
		}
		for _, row := range g.Rows {
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				if cell.Shape != nil && strings.Contains(string(cell.Shape.Text), labelFragment) {
					fill := strings.Trim(string(cell.Shape.Fill), `"`)
					// Skip the transparent outside-label cells.
					if fill != "" && !strings.Contains(fill, "alpha") {
						found = fill
						return
					}
				}
				walk(cell.Grid)
			}
		}
	}
	walk(grid)
	if found == "" {
		t.Fatalf("no bar shape found whose value text contains %q", labelFragment)
	}
	return found
}

// wbColumnRows returns the rows of a column's sub-grid from the bar band.
func wbColumnRows(t *testing.T, grid *jsonschema.ShapeGridInput, barRowIdx, col int) []jsonschema.GridRowInput {
	t.Helper()
	cell := grid.Rows[barRowIdx].Cells[col]
	if cell == nil || cell.Grid == nil {
		t.Fatalf("column %d has no sub-grid", col)
	}
	return cell.Grid.Rows
}

// wbTextObj decodes a shape's text payload.
func wbTextObj(t *testing.T, raw json.RawMessage) waterfallBridgeTextObj {
	t.Helper()
	var obj waterfallBridgeTextObj
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("decode text %s: %v", raw, err)
	}
	return obj
}

// go-slide-creator-2fq1: a bar too thin to hold its value label puts the label
// in the adjacent spacer, anchored to the edge the bar sits on. Like every
// shape it keeps the uniform text margin — no pattern-authored inset_* — and
// the writer clamps that margin when the spacer is too short for it.
func TestWaterfallBridge_ThinBarLabelHugsItsBar(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Unit: "€m",
		Columns: []WaterfallBridgeColumn{
			{Label: "FY24", Type: wbTypeTotal, Value: 186},
			{Label: "Volume", Type: wbTypeDelta, Value: 42},
			{Label: "FX", Type: wbTypeDelta, Value: -4},
			{Label: "FY25", Type: wbTypeTotal, Value: 224},
		},
	}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// The −4 column's label cannot fit its bar, so it lands in a spacer.
	var outside json.RawMessage
	for _, row := range wbColumnRows(t, grid, 0, 2) {
		for _, c := range row.Cells {
			if c == nil || c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			if obj := wbTextObj(t, c.Shape.Text); obj.VerticalAlign == "t" || obj.VerticalAlign == "b" {
				outside = c.Shape.Text
			}
		}
	}
	if outside == nil {
		t.Fatal("the thin bar's value label was not placed in a spacer")
	}
	var raw map[string]any
	if err := json.Unmarshal(outside, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
		if _, has := raw[k]; has {
			t.Errorf("outside label overrides the uniform shape margin with %s", k)
		}
	}
}

// A bridge draws no value axis, so the caption is where the scale is stated.
func TestWaterfallBridge_CaptionStatesTheScale(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	base := func() *WaterfallBridgeValues {
		return &WaterfallBridgeValues{
			Unit: "€m",
			Columns: []WaterfallBridgeColumn{
				{Label: "FY24", Type: wbTypeTotal, Value: 186},
				{Label: "Volume", Type: wbTypeDelta, Value: 42},
				{Label: "FY25", Type: wbTypeTotal, Value: 228},
			},
		}
	}
	ctx := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	// No caption: two bands, exactly as before.
	plain, err := p.Expand(ctx, base(), nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(plain.Rows) != 2 {
		t.Fatalf("without a caption the grid should keep its two bands, got %d rows", len(plain.Rows))
	}
	if plain.Rows[0].Height != wbBarRowPct {
		t.Errorf("bar band = %v, want the unchanged %v", plain.Rows[0].Height, wbBarRowPct)
	}

	// With a caption: a band is taken off the bars, never off the labels.
	v := base()
	v.Caption = "EUR millions, constant FX"
	grid, err := p.Expand(ctx, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("expected caption + bars + labels, got %d rows", len(grid.Rows))
	}
	if grid.Rows[0].Height+grid.Rows[1].Height != wbBarRowPct {
		t.Errorf("caption %v + bars %v should still total the %v bar band",
			grid.Rows[0].Height, grid.Rows[1].Height, wbBarRowPct)
	}
	if grid.Rows[2].Height != wbLabelRowPct {
		t.Errorf("label band = %v, want the unchanged %v", grid.Rows[2].Height, wbLabelRowPct)
	}
	capCell := grid.Rows[0].Cells[0]
	if capCell == nil || capCell.Shape == nil || len(capCell.Shape.Text) == 0 {
		t.Fatal("caption row carries no text")
	}
	if capCell.ColSpan != len(v.Columns) {
		t.Errorf("caption col_span = %d, want %d so it spans the chart", capCell.ColSpan, len(v.Columns))
	}
	obj := wbTextObj(t, capCell.Shape.Text)
	if obj.Paragraphs[0].Content != v.Caption {
		t.Errorf("caption = %q, want %q", obj.Paragraphs[0].Content, v.Caption)
	}
	if obj.Align != "l" {
		t.Errorf("caption align = %q, want left: the unit caption sits top-left of the chart (go-slide-creator-cuszt)", obj.Align)
	}
	// Whitespace-only is not a caption.
	blank := base()
	blank.Caption = "   "
	blankGrid, err := p.Expand(ctx, blank, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(blankGrid.Rows) != 2 {
		t.Errorf("a blank caption should not take a band, got %d rows", len(blankGrid.Rows))
	}
}

// The caption has a budget like every other text field.
func TestWaterfallBridge_CaptionTooLong(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	v := &WaterfallBridgeValues{
		Caption: strings.Repeat("x", wbCaptionMax+1),
		Columns: []WaterfallBridgeColumn{
			{Label: "A", Type: wbTypeTotal, Value: 1},
			{Label: "B", Type: wbTypeDelta, Value: 1},
			{Label: "C", Type: wbTypeTotal, Value: 2},
		},
	}
	err := p.Validate(v, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "caption") {
		t.Errorf("an over-budget caption should be reported, got %v", err)
	}
}

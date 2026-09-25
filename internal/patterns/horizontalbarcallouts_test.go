package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestHorizontalBarCallouts_Registration(t *testing.T) {
	p, ok := Default().Get("horizontal-bar-with-callouts")
	if !ok {
		t.Fatal("expected horizontal-bar-with-callouts to be registered in default registry")
	}
	if p.Name() != "horizontal-bar-with-callouts" {
		t.Errorf("Name() = %q, want %q", p.Name(), "horizontal-bar-with-callouts")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validHBCValues(n int) *HorizontalBarCalloutsValues {
	bars := make([]HorizontalBarCalloutsBar, n)
	labels := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta"}
	values := []float64{95, 87, 72, 64, 51, 40, 28, 15}
	callouts := []string{
		"Strongest on price.",
		"Preferred for enterprise.",
		"Best API ecosystem.",
		"Lags on certifications.",
		"Steady mid-market traction.",
		"Underinvesting in support.",
		"New entrant, watch list.",
		"Limited capability set.",
	}
	for i := 0; i < n; i++ {
		bars[i] = HorizontalBarCalloutsBar{
			Label:   labels[i],
			Value:   values[i],
			Callout: callouts[i],
		}
	}
	return &HorizontalBarCalloutsValues{Bars: bars, Unit: "%"}
}

func TestHorizontalBarCallouts_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	for _, n := range []int{3, 5, 8} {
		v := validHBCValues(n)
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("n=%d: unexpected validation error: %v", n, err)
		}
	}
}

func TestHorizontalBarCallouts_Validate_TooFewBars(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(2)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 bars")
	}
	if !strings.Contains(err.Error(), "kpi-Nup") {
		t.Errorf("expected sibling hint mentioning kpi-Nup, got: %v", err)
	}
}

func TestHorizontalBarCallouts_Validate_TooManyBars(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(8)
	v.Bars = append(v.Bars, HorizontalBarCalloutsBar{Label: "Iota", Value: 5})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 8 bars")
	}
}

func TestHorizontalBarCallouts_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(4)
	v.Bars[1].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestHorizontalBarCallouts_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(4)
	v.Bars[2].Label = strings.Repeat("X", hbcLabelMax+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for label > max length")
	}
}

func TestHorizontalBarCallouts_Validate_NegativeValue(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	v.Bars[0].Value = -1
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for negative value")
	}
}

func TestHorizontalBarCallouts_Validate_NegativeMaxValue(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	v.MaxValue = -10
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for negative max_value")
	}
}

func TestHorizontalBarCallouts_Validate_CalloutTooLong(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	v.Bars[1].Callout = strings.Repeat("X", hbcCalloutMax+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for callout > max length")
	}
}

func TestHorizontalBarCallouts_Validate_UnitTooLong(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	v.Unit = strings.Repeat("X", hbcUnitMax+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for unit > max length")
	}
}

func TestHorizontalBarCallouts_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	overrides := map[int]any{99: &HorizontalBarCalloutsCellOverride{FontSize: 12}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestHorizontalBarCallouts_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(5)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 5 {
		t.Fatalf("expected 5 rows (one per bar), got %d", got)
	}
	for i, row := range grid.Rows {
		if got := len(row.Cells); got != 2 {
			t.Errorf("row %d: expected 2 cells (bar + callout), got %d", i, got)
		}
		if row.Cells[0].Grid == nil {
			t.Errorf("row %d: expected bar cell to host a sub-grid", i)
		}
	}
	// Outer columns must be [60, 40].
	if !strings.Contains(string(grid.Columns), "60") || !strings.Contains(string(grid.Columns), "40") {
		t.Errorf("expected outer columns [60, 40], got %s", string(grid.Columns))
	}
}

func TestHorizontalBarCallouts_Expand_BoundaryBarCounts(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	for _, n := range []int{3, 8} {
		v := validHBCValues(n)
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: Expand failed: %v", n, err)
		}
		if got := len(grid.Rows); got != n {
			t.Errorf("n=%d: expected %d rows, got %d", n, n, got)
		}
	}
}

func TestHorizontalBarCallouts_Expand_BarFillProportional(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := &HorizontalBarCalloutsValues{
		MaxValue: 100,
		Bars: []HorizontalBarCalloutsBar{
			{Label: "Full", Value: 100, Callout: "Maxed."},
			{Label: "Half", Value: 50, Callout: "Mid."},
			{Label: "Quarter", Value: 25, Callout: "Quarter."},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Each bar row's left cell is a sub-grid with columns [label, fill, (rest?)].
	// For value=100 there is no "rest" column. For half/quarter there is one.
	row0SubCols := string(grid.Rows[0].Cells[0].Grid.Columns)
	if !strings.Contains(row0SubCols, "78") {
		t.Errorf("100%% bar: expected fill ~78, got cols %s", row0SubCols)
	}

	row1SubCols := string(grid.Rows[1].Cells[0].Grid.Columns)
	// 50% of 78 = 39
	if !strings.Contains(row1SubCols, "39") {
		t.Errorf("50%% bar: expected fill ~39, got cols %s", row1SubCols)
	}

	row2SubCols := string(grid.Rows[2].Cells[0].Grid.Columns)
	// 25% of 78 = 19.5
	if !strings.Contains(row2SubCols, "19.5") {
		t.Errorf("25%% bar: expected fill ~19.5, got cols %s", row2SubCols)
	}
}

func TestHorizontalBarCallouts_Expand_MaxValueDefaultsToMax(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := &HorizontalBarCalloutsValues{
		Bars: []HorizontalBarCalloutsBar{
			{Label: "Top", Value: 87, Callout: "Top."},
			{Label: "Mid", Value: 43.5, Callout: "Mid."},
			{Label: "Bot", Value: 21.75, Callout: "Bot."},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Top bar should fill the full available bar area (~78%) since maxValue
	// defaults to the highest bar value.
	row0SubGrid := grid.Rows[0].Cells[0].Grid
	if row0SubGrid == nil {
		t.Fatal("expected sub-grid on first bar row")
	}
	cols := string(row0SubGrid.Columns)
	if !strings.Contains(cols, "78") {
		t.Errorf("top bar should fill ~78 (full bar area), got cols %s", cols)
	}
}

func TestHorizontalBarCallouts_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(4)
	overrides := &HorizontalBarCalloutsOverrides{TextOverrides: TextOverrides{Accent: "accent4"}}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Bar fill on row 0 should use accent4.
	fillCell := grid.Rows[0].Cells[0].Grid.Rows[0].Cells[1]
	if !strings.Contains(string(fillCell.Shape.Fill), "accent4") {
		t.Errorf("expected bar fill to include accent4, got %q", string(fillCell.Shape.Fill))
	}
	// Callout accent bar on row 0 should use accent4.
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Fatal("expected callout accent bar")
	}
	if grid.Rows[0].Cells[1].AccentBar.Color != "accent4" {
		t.Errorf("expected callout accent color accent4, got %q", grid.Rows[0].Cells[1].AccentBar.Color)
	}
}

func TestHorizontalBarCallouts_Expand_CalloutAccentBarPresent(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := validHBCValues(3)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, row := range grid.Rows {
		if row.Cells[1].AccentBar == nil {
			t.Errorf("row %d: expected callout accent bar", i)
			continue
		}
		if row.Cells[1].AccentBar.Position != "left" {
			t.Errorf("row %d: expected accent bar position 'left', got %q", i, row.Cells[1].AccentBar.Position)
		}
	}
}

func TestHorizontalBarCallouts_Expand_ValueLabelFormatting(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := &HorizontalBarCalloutsValues{
		Unit: "%",
		Bars: []HorizontalBarCalloutsBar{
			{Label: "A", Value: 50, Callout: "x"},
			{Label: "B", Value: 33.5, Callout: "x"},
			{Label: "C", Value: 12, Callout: "x"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Inspect the fill cell text (value label) on each row.
	expected := []string{"50%", "33.5%", "12%"}
	for i, want := range expected {
		fillCell := grid.Rows[i].Cells[0].Grid.Rows[0].Cells[1]
		if !strings.Contains(string(fillCell.Shape.Text), want) {
			t.Errorf("row %d: expected value text to contain %q, got %q", i, want, string(fillCell.Shape.Text))
		}
	}
}

func TestHorizontalBarCallouts_Schema(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestHorizontalBarCallouts_Taxonomy(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
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

func TestHorizontalBarCallouts_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"vendor scoring", "ranked vendor scoring with per-vendor insight", &ContentHints{ItemCount: 4}},
		{"opportunity sizing", "opportunity sizing with per-bar callouts", &ContentHints{ItemCount: 5}},
		{"driver list", "ranked driver list horizontal bars with insight callouts", &ContentHints{ItemCount: 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "horizontal-bar-with-callouts" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected horizontal-bar-with-callouts in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}

// go-slide-creator-d6zo: bar values concatenated the raw number and the unit,
// so a currency/magnitude unit — the consulting default — produced
// "1240.5EUR M". Values now go through the shared formatter.
func TestFormatHorizontalBarValue_UnitPlacement(t *testing.T) {
	cases := []struct {
		v    float64
		unit string
		want string
	}{
		{1240.5, "EUR M", "1,240.5 EUR M"},
		{96.44, "", "96.4"},
		{3.8, "%", "3.8%"},
		{210, "$m", "$210m"},
		{1234567, "", "1,234,567"},
		{12, "x", "12x"},
		{0, "%", "0%"},
	}
	for _, c := range cases {
		if got := formatHorizontalBarValue(c.v, c.unit); got != c.want {
			t.Errorf("formatHorizontalBarValue(%v, %q) = %q, want %q", c.v, c.unit, got, c.want)
		}
	}
	// The concrete regression: no separator at all before a word unit.
	if got := formatHorizontalBarValue(1240.5, "EUR M"); got == "1240.5EUR M" {
		t.Error("value and unit are still concatenated with no separator")
	}
}

// A bar too short to hold its value label must carry the label OUTSIDE (in the
// transparent remainder cell, dark text) instead of clipping it inside.
func TestValueLabelFitsInBar(t *testing.T) {
	const size = 12.0
	cases := []struct {
		name       string
		label      string
		barWidthPt float64
		want       bool
	}{
		{"wide bar holds a short label", "195", 200, true},
		{"narrow bar rejects a long label", "1,240.5 EUR M", 30, false},
		{"very short bar rejects even a short label", "3.8", 8, false},
		{"unknown bar width keeps the label inside", "3.8", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := valueLabelFitsInBar(c.label, size, c.barWidthPt); got != c.want {
				t.Errorf("valueLabelFitsInBar(%q, %v, %v) = %v, want %v", c.label, size, c.barWidthPt, got, c.want)
			}
		})
	}
}

// End-to-end: a deck whose smallest bar is a sliver of the largest must put
// that bar's value label in the remainder cell, and the largest bar's inside.
func TestHorizontalBarCallouts_ShortBarLabelMovesOutside(t *testing.T) {
	pat, ok := Default().Get("horizontal-bar-with-callouts")
	if !ok {
		t.Fatal("pattern not registered")
	}
	vals := &HorizontalBarCalloutsValues{
		Unit: "EUR M",
		Bars: []HorizontalBarCalloutsBar{
			{Label: "Enterprise", Value: 1240.5, Callout: "Largest segment"},
			{Label: "Mid-market", Value: 96.4, Callout: "Steady"},
			{Label: "SMB", Value: 3.8, Callout: "Long tail"},
		},
	}
	ctx := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, err := pat.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}

	// barSubGrid returns the [label, fill, rest] sub-grid cells of a row.
	barSubGrid := func(rowIdx int) []*jsonschema.GridCellInput {
		row := grid.Rows[rowIdx]
		if len(row.Cells) == 0 || row.Cells[0].Grid == nil || len(row.Cells[0].Grid.Rows) == 0 {
			t.Fatalf("row %d has no bar sub-grid", rowIdx)
		}
		return row.Cells[0].Grid.Rows[0].Cells
	}

	hasText := func(c *jsonschema.GridCellInput) bool {
		return c != nil && c.Shape != nil && len(c.Shape.Text) > 0
	}

	// Largest bar: label inside the fill cell.
	big := barSubGrid(0)
	if !hasText(big[1]) {
		t.Error("the longest bar should carry its value label inside the fill")
	}

	// Smallest bar: fill cell carries no text; the remainder cell does.
	small := barSubGrid(2)
	if hasText(small[1]) {
		t.Error("a sliver bar must not carry its value label inside the fill (it clips)")
	}
	if len(small) < 3 || !hasText(small[2]) {
		t.Fatal("a sliver bar must carry its value label in the remainder cell to the right")
	}
	if !strings.Contains(string(small[2].Shape.Text), "3.8") {
		t.Errorf("outside label should carry the value, got %s", small[2].Shape.Text)
	}
	if !strings.Contains(string(small[2].Shape.Text), "dk1") {
		t.Errorf("outside label should be dark text on the transparent remainder, got %s", small[2].Shape.Text)
	}
}

// go-slide-creator-i0x0: bars with nothing to say used to keep a 40%-wide
// callout column and a row of accent ticks anchored to empty cells, which read
// as a column of floating coloured dashes beside bars squeezed into 60% of the
// slide.
func TestHorizontalBarCallouts_NoCalloutsDropsTheCalloutColumn(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := &HorizontalBarCalloutsValues{
		Bars: []HorizontalBarCalloutsBar{
			{Label: "A", Value: 3},
			{Label: "B", Value: 2},
			{Label: "C", Value: 1},
		},
	}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if got := string(grid.Columns); got != "[100]" {
		t.Errorf("outer columns = %s, want [100] — the callout column should be gone", got)
	}
	for i, row := range grid.Rows {
		if got := len(row.Cells); got != 1 {
			t.Fatalf("row %d: expected only the bar cell, got %d cells", i, got)
		}
		if row.Cells[0].AccentBar != nil {
			t.Errorf("row %d: bar cell carries an accent bar with no callout to anchor", i)
		}
		if row.Cells[0].Grid == nil {
			t.Fatalf("row %d: expected the bar cell to host a sub-grid", i)
		}
	}

	// The labels did not get longer, so the gutter keeps its absolute width:
	// 22% of the old 60% column is 13.2% of the full width. Every reclaimed
	// point goes to the bars.
	cols := hbcSubGridColumns(t, grid, 0)
	if diff := cols[0] - hbcLabelColPct*hbcLeftColPct/100; diff > 0.01 || diff < -0.01 {
		t.Errorf("label column = %.2f%% of full width, want %.2f%%", cols[0], hbcLabelColPct*hbcLeftColPct/100)
	}
	// The longest bar (value == derived max_value) must reach the right edge.
	if rest := cols[len(cols)-1]; len(cols) > 2 && rest > 0.01 {
		t.Errorf("longest bar leaves a %.2f%% remainder; it should reach the axis", rest)
	}
	if fill := cols[1]; fill < 100-cols[0]-0.01 {
		t.Errorf("longest bar fill = %.2f%%, want the whole %.2f%% bar area", fill, 100-cols[0])
	}
}

// A single callout still earns the column — but only that row gets a tick.
func TestHorizontalBarCallouts_TickOnlyWhereThereIsACallout(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	v := &HorizontalBarCalloutsValues{
		Unit: "%",
		Bars: []HorizontalBarCalloutsBar{
			{Label: "Vendor A", Value: 87, Callout: "Strongest on price."},
			{Label: "Vendor B", Value: 72},
			{Label: "Vendor C", Value: 64, Callout: "   "},
		},
	}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if !strings.Contains(string(grid.Columns), "60") {
		t.Fatalf("one callout should keep the two-column split, got %s", string(grid.Columns))
	}
	want := []bool{true, false, false}
	for i, row := range grid.Rows {
		if len(row.Cells) != 2 {
			t.Fatalf("row %d: expected bar + callout cells, got %d", i, len(row.Cells))
		}
		if got := row.Cells[1].AccentBar != nil; got != want[i] {
			t.Errorf("row %d: accent bar present = %v, want %v", i, got, want[i])
		}
	}
	// The bar geometry is untouched by the per-row tick decision.
	cols := hbcSubGridColumns(t, grid, 0)
	if diff := cols[0] - hbcLabelColPct; diff > 0.01 || diff < -0.01 {
		t.Errorf("label column = %.2f%%, want the two-column %.2f%%", cols[0], hbcLabelColPct)
	}
}

// hbcSubGridColumns returns the [label, fill, rest?] column percentages of a
// bar row's sub-grid.
func hbcSubGridColumns(t *testing.T, grid *jsonschema.ShapeGridInput, rowIdx int) []float64 {
	t.Helper()
	row := grid.Rows[rowIdx]
	if len(row.Cells) == 0 || row.Cells[0].Grid == nil {
		t.Fatalf("row %d has no bar sub-grid", rowIdx)
	}
	var cols []float64
	if err := json.Unmarshal(row.Cells[0].Grid.Columns, &cols); err != nil {
		t.Fatalf("row %d sub-grid columns %s: %v", rowIdx, row.Cells[0].Grid.Columns, err)
	}
	return cols
}

// go-slide-creator-s1uvj.37: any cell override on a callout row used to remove
// its accent bar, because the bar was only added when no override existed.
// Only an explicit accent_bar:false may suppress it.
func TestHorizontalBarCallouts_CellOverrideKeepsAccentBarUnlessFalse(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	raw := map[int]string{
		0: `{"accent_bar":true}`,
		1: `{"font_size":14}`,
		2: `{"accent_bar":false}`,
	}
	cellOverrides := map[int]any{}
	for i, js := range raw {
		co := p.NewCellOverride()
		if err := json.Unmarshal([]byte(js), co); err != nil {
			t.Fatalf("decode cell_overrides[%d]: %v", i, err)
		}
		cellOverrides[i] = co
	}
	v := validHBCValues(4)
	if err := p.Validate(v, nil, cellOverrides); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	want := []bool{true, true, false, true}
	for i, row := range grid.Rows {
		if got := row.Cells[1].AccentBar != nil; got != want[i] {
			t.Errorf("row %d: accent bar present = %v, want %v", i, got, want[i])
		}
	}
}

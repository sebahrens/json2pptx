package patterns

import (
	"strings"
	"testing"
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
	overrides := map[int]any{99: &HorizontalBarCalloutsCellOverride{AccentBar: true}}
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

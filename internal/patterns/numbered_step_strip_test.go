package patterns

import (
	"strings"
	"testing"
)

func validNumberedStepStripValues(style string, n int) *NumberedStepStripValues {
	labels := []string{"Discover", "Design", "Deliver", "Sustain", "Scale", "Renew"}
	bodies := []string{
		"Map the current state and surface the constraints.",
		"Shape the target operating model and the roadmap.",
		"Stand up the capability and migrate the workload.",
		"Embed the run model and track the value captured.",
		"Extend the platform across the wider estate.",
		"Refresh the portfolio against the next horizon.",
	}
	steps := make([]NumberedStepStripStep, n)
	for i := 0; i < n; i++ {
		steps[i] = NumberedStepStripStep{Label: labels[i], Body: bodies[i]}
	}
	return &NumberedStepStripValues{Style: style, Steps: steps}
}

func TestNumberedStepStrip_Registration(t *testing.T) {
	p, ok := Default().Get("numbered-step-strip")
	if !ok {
		t.Fatal("expected numbered-step-strip to be registered")
	}
	if p.Name() != "numbered-step-strip" {
		t.Errorf("Name() = %q, want numbered-step-strip", p.Name())
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Error("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func TestNumberedStepStrip_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	for _, style := range []string{"", "chevron", "stacked-box", "toc"} {
		for _, n := range []int{3, 4, 6} {
			if err := p.Validate(validNumberedStepStripValues(style, n), nil, nil); err != nil {
				t.Errorf("style=%q n=%d: unexpected validation error: %v", style, n, err)
			}
		}
	}
}

func TestNumberedStepStrip_Validate_BadStyle(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("swimlane", 4)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for unknown style")
	}
	if !strings.Contains(err.Error(), "style") {
		t.Errorf("expected error to mention style, got: %v", err)
	}
}

func TestNumberedStepStrip_Validate_TooFewSteps(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	if err := p.Validate(validNumberedStepStripValues("chevron", 2), nil, nil); err == nil {
		t.Fatal("expected validation error for fewer than 3 steps")
	}
}

func TestNumberedStepStrip_Validate_TooManySteps(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("chevron", 6)
	v.Steps = append(v.Steps, NumberedStepStripStep{Label: "Seventh"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 6 steps")
	}
}

func TestNumberedStepStrip_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("chevron", 4)
	v.Steps[1].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestNumberedStepStrip_Validate_BodyTooLong(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 4)
	v.Steps[0].Body = strings.Repeat("X", 181)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for body > 180 chars")
	}
}

func TestNumberedStepStrip_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 4)
	overrides := map[int]any{99: &NumberedStepStripCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

// containsNoDiamond walks every cell shape and fails if any geometry is a
// decision diamond — the defining contract of this pattern vs process-flow.
func assertNoDiamonds(t *testing.T, p Pattern, v *NumberedStepStripValues) {
	t.Helper()
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell.Shape != nil && cell.Shape.Geometry == "diamond" {
				t.Errorf("style=%q: numbered-step-strip must never emit a decision diamond", v.Style)
			}
		}
	}
}

func TestNumberedStepStrip_Expand_NoDiamonds(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	for _, style := range []string{"chevron", "stacked-box", "toc"} {
		for _, n := range []int{3, 6} {
			assertNoDiamonds(t, p, validNumberedStepStripValues(style, n))
		}
	}
}

func TestNumberedStepStrip_Expand_ChevronCompactNoBody(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := &NumberedStepStripValues{
		Style: "chevron",
		Steps: []NumberedStepStripStep{
			{Label: "Plan"}, {Label: "Build"}, {Label: "Ship"}, {Label: "Learn"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows) != 1 {
		t.Fatalf("expected a single chevron row when no body present, got %d rows", len(grid.Rows))
	}
	if grid.Bounds == nil {
		t.Fatal("expected compact bounds when no body present")
	}
	if grid.Bounds.Height > 35 {
		t.Errorf("expected compact height <= 35%%, got %v", grid.Bounds.Height)
	}
	for _, cell := range grid.Rows[0].Cells {
		if cell.Shape.Geometry != "chevron" {
			t.Errorf("expected chevron geometry, got %q", cell.Shape.Geometry)
		}
	}
}

func TestNumberedStepStrip_Expand_ChevronWithBodyAddsDetailRow(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("chevron", 4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Fatalf("expected chevron row + detail row when body present, got %d rows", len(grid.Rows))
	}
	if grid.Bounds != nil {
		t.Error("expected full-height layout (no compact bounds) when body present")
	}
}

func TestNumberedStepStrip_Expand_StackedBoxShape(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("expected one row per step, got %d", len(grid.Rows))
	}
	for i, row := range grid.Rows {
		if len(row.Cells) != 2 {
			t.Fatalf("row %d: expected 2 cells (number + body), got %d", i, len(row.Cells))
		}
		number := row.Cells[0]
		body := row.Cells[1]
		// Number/tip lane must carry a colored fill (not "none").
		if strings.Contains(string(number.Shape.Fill), "none") {
			t.Errorf("row %d: number/tip lane should be colored, got %q", i, string(number.Shape.Fill))
		}
		// Body column must be borderless / no-fill.
		if !strings.Contains(string(body.Shape.Fill), "none") {
			t.Errorf("row %d: body column should be no-border (none fill), got %q", i, string(body.Shape.Fill))
		}
	}
}

func TestNumberedStepStrip_Expand_StackedBoxTipColor(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 3)
	v.Steps[1].TipColor = "accent4"
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if !strings.Contains(string(grid.Rows[1].Cells[0].Shape.Fill), "accent4") {
		t.Errorf("expected per-step tip_color accent4 on number lane, got %q", string(grid.Rows[1].Cells[0].Shape.Fill))
	}
}

func TestNumberedStepStrip_Expand_NumberOverride(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 3)
	v.Steps[0].Number = "A"
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if !strings.Contains(string(grid.Rows[0].Cells[0].Shape.Text), "\"A\"") {
		t.Errorf("expected explicit number override 'A' in number lane, got %q", string(grid.Rows[0].Cells[0].Shape.Text))
	}
	// Auto-numbered second step should be zero-padded "02".
	if !strings.Contains(string(grid.Rows[1].Cells[0].Shape.Text), "02") {
		t.Errorf("expected auto number 02 on step 2, got %q", string(grid.Rows[1].Cells[0].Shape.Text))
	}
}

func TestNumberedStepStrip_Expand_TOCMirrorsAgenda(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("toc", 4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("expected one row per item, got %d", len(grid.Rows))
	}
	badge := grid.Rows[0].Cells[0]
	title := grid.Rows[0].Cells[1]
	if badge.Shape.Geometry != "roundRect" {
		t.Errorf("expected rounded number badge (like agenda), got %q", badge.Shape.Geometry)
	}
	if !strings.Contains(string(title.Shape.Fill), "none") {
		t.Errorf("expected no-fill title cell (like agenda), got %q", string(title.Shape.Fill))
	}
}

func TestNumberedStepStrip_Expand_DefaultStyleIsStackedBox(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("", 4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// stacked-box is one row per step with 2 cells each.
	if len(grid.Rows) != 4 || len(grid.Rows[0].Cells) != 2 {
		t.Errorf("expected default stacked-box layout (4 rows × 2 cells), got %d rows", len(grid.Rows))
	}
}

func TestNumberedStepStrip_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	v := validNumberedStepStripValues("stacked-box", 4)
	cellOverrides := map[int]any{2: &NumberedStepStripCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[2].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on body cell with override")
	}
}

func TestNumberedStepStrip_Schema(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	if p.Schema() == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestNumberedStepStrip_Taxonomy(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want structural", tax.Category)
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("expected non-empty NarrativeRole")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("expected non-empty PairsWith")
	}
}

func TestNumberedStepStrip_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"ordered steps", "our approach in four ordered steps", &ContentHints{ItemCount: 4}},
		{"numbered steps", "the process in numbered steps", &ContentHints{ItemCount: 5}},
		{"toc with detail", "annotated agenda with descriptions", &ContentHints{ItemCount: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "numbered-step-strip" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected numbered-step-strip in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}

package patterns

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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
	// The strip is sized to its content: chevrons + a text-sized detail row
	// directly beneath them, never a flex row that fills the slide.
	if grid.Bounds == nil || grid.Bounds.Height >= 100 {
		t.Fatalf("expected content-sized bounds when body present, got %+v", grid.Bounds)
	}
	if grid.Rows[0].Height <= 0 || grid.Rows[0].Height >= 100 {
		t.Errorf("chevron row should have a fixed share of the strip, got %v", grid.Rows[0].Height)
	}
}

func TestNumberedStepStrip_Chevron_NotchInsetAndSizes(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	for _, n := range []int{3, 4, 5, 6} {
		ctx := testThemeCtx()
		v := validNumberedStepStripValues("chevron", n)
		grid, err := p.Expand(ctx, v, nil, nil)
		if err != nil {
			t.Fatalf("n=%d Expand failed: %v", n, err)
		}
		geo := chevronStripGeometry(ctx, n)
		if geo.stepWPt < 2*geo.chevHPt-0.01 {
			t.Errorf("n=%d: chevron %.1fx%.1fpt must be at least twice as wide as tall", n, geo.stepWPt, geo.chevHPt)
		}
		for i, cell := range grid.Rows[0].Cells {
			if cell.Shape.Geometry != "chevron" {
				t.Fatalf("n=%d cell %d: geometry %q", n, i, cell.Shape.Geometry)
			}
			adj := cell.Shape.Adjustments["adj"]
			if adj <= 0 || adj > 50000 {
				t.Errorf("n=%d cell %d: adj %d out of range", n, i, adj)
			}
			notchPt := float64(adj) / 100000 * geo.chevHPt
			var text struct {
				InsetLeft  float64 `json:"inset_left"`
				InsetRight float64 `json:"inset_right"`
			}
			if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
				t.Fatalf("text: %v", err)
			}
			if text.InsetLeft < notchPt {
				t.Errorf("n=%d cell %d: lIns %.1fpt < notch depth %.1fpt", n, i, text.InsetLeft, notchPt)
			}
			if text.InsetRight < notchPt {
				t.Errorf("n=%d cell %d: rIns %.1fpt < point depth %.1fpt", n, i, text.InsetRight, notchPt)
			}
		}
		for i, cell := range grid.Rows[1].Cells {
			var text struct {
				VerticalAlign string `json:"vertical_align"`
				Paragraphs    []struct {
					Size float64 `json:"size"`
				} `json:"paragraphs"`
			}
			if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
				t.Fatalf("desc text: %v", err)
			}
			if text.Paragraphs[0].Size < 12 {
				t.Errorf("n=%d desc %d: size %.1f < 12pt", n, i, text.Paragraphs[0].Size)
			}
			if text.VerticalAlign != "t" {
				t.Errorf("n=%d desc %d: want top-anchored description, got %q", n, i, text.VerticalAlign)
			}
		}
	}
}

func TestEstimateWrappedLines(t *testing.T) {
	if got := estimateWrappedLines("short", 12, 200); got != 1 {
		t.Errorf("short text: %d lines", got)
	}
	long := strings.Repeat("word ", 40)
	if got := estimateWrappedLines(long, 12, 120); got < 5 {
		t.Errorf("long text wrapped to only %d lines", got)
	}
	if got := estimateWrappedLines("a\nb", 12, 200); got != 2 {
		t.Errorf("explicit newline: %d lines", got)
	}
	if got := estimateWrappedLines("x", 12, 0); got != 1 {
		t.Errorf("zero width: %d", got)
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

// chevronLabelSizes reads the label paragraph size and the adjustment value out
// of an expanded chevron row.
func chevronLabelSizes(t *testing.T, grid *jsonschema.ShapeGridInput) (adj int64, sizePt float64) {
	t.Helper()
	cell := grid.Rows[0].Cells[0]
	var text struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
		t.Fatalf("label text: %v", err)
	}
	if len(text.Paragraphs) < 2 {
		t.Fatalf("want number + label paragraphs, got %d", len(text.Paragraphs))
	}
	return cell.Shape.Adjustments["adj"], text.Paragraphs[1].Size
}

// TestNumberedStepStrip_Chevron_LongLabelsGiveUpNotchDepth pins the fix for
// go-slide-creator-e97v: at 6 steps a 13pt "Qualification" had 51pt of usable
// chevron and rendered as "Qualific / ation". The strip now trades notch depth
// for label width — and only shallows the notch when it has to, so short
// labels keep the full arrow.
func TestNumberedStepStrip_Chevron_LongLabelsGiveUpNotchDepth(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	ctx := testThemeCtx()

	short := &NumberedStepStripValues{Style: "chevron", Steps: []NumberedStepStripStep{
		{Label: "Attract"}, {Label: "Close"}, {Label: "Renew"},
	}}
	shortGrid, err := p.Expand(ctx, short, nil, nil)
	if err != nil {
		t.Fatalf("Expand short: %v", err)
	}
	if adj, size := chevronLabelSizes(t, shortGrid); adj != chevronAdj || size != 13 {
		t.Errorf("short labels: adj=%d size=%.0f, want the full notch (%d) at 13pt", adj, size, chevronAdj)
	}

	long := &NumberedStepStripValues{Style: "chevron", Steps: []NumberedStepStripStep{
		{Label: "Attract"}, {Label: "Qualification"}, {Label: "Proposal"},
		{Label: "Negotiate"}, {Label: "Onboarding"}, {Label: "Expansion"},
	}}
	longGrid, err := p.Expand(ctx, long, nil, nil)
	if err != nil {
		t.Fatalf("Expand long: %v", err)
	}
	adj, size := chevronLabelSizes(t, longGrid)
	if adj >= chevronAdj {
		t.Errorf("long labels: adj=%d, want a shallower notch than %d", adj, chevronAdj)
	}
	if adj < chevronMinAdj {
		t.Errorf("long labels: adj=%d below the floor %d", adj, chevronMinAdj)
	}
	if size != 13 {
		t.Errorf("long labels: size=%.1f, want 13pt — the notch should pay before the type does", size)
	}

	// Every label fits on one line at the size and inset that were chosen.
	geo := chevronStripGeometry(ctx, len(long.Steps))
	for _, step := range long.Steps {
		if got := measuredLines(step.Label, ctx.Theme.BodyFont, true, size, chevronLabelWidthPt(geo, adj)); got > 1 {
			t.Errorf("label %q still wraps to %d lines", step.Label, got)
		}
	}
}

// TestNumberedStepStrip_Chevron_ShrinksToTheReadableFloor checks the second
// remedy: when the shallowest notch is still not enough the label shrinks, and
// never below the readable floor.
func TestNumberedStepStrip_Chevron_ShrinksToTheReadableFloor(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	ctx := testThemeCtx()
	vals := &NumberedStepStripValues{Style: "chevron", Steps: []NumberedStepStripStep{
		{Label: "Internationalisation"}, {Label: "Standardisation"}, {Label: "Commercialisation"},
		{Label: "Operationalisation"}, {Label: "Professionalisation"}, {Label: "Decommissioning"},
	}}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	adj, size := chevronLabelSizes(t, grid)
	if adj != chevronMinAdj {
		t.Errorf("adj=%d, want the shallowest notch %d before shrinking type", adj, chevronMinAdj)
	}
	if size < chevronMinLabelPt {
		t.Errorf("size=%.1f is below the readable floor %.1f", size, chevronMinLabelPt)
	}
	if size >= 13 {
		t.Errorf("size=%.1f, want the label shrunk once the notch had nothing left to give", size)
	}
}

// TestNumberedStepStrip_Chevron_WarnsWhenEvenTheFloorWraps checks the signal an
// agent gets when the labels are simply too long for the step count: the
// pattern renders its best attempt and says so.
func TestNumberedStepStrip_Chevron_WarnsWhenEvenTheFloorWraps(t *testing.T) {
	p, _ := Default().Get("numbered-step-strip")
	warner, ok := p.(PostExpandWarner)
	if !ok {
		t.Fatal("numbered-step-strip must implement PostExpandWarner")
	}
	ctx := testThemeCtx()

	fine := &NumberedStepStripValues{Style: "chevron", Steps: []NumberedStepStripStep{
		{Label: "Attract"}, {Label: "Qualification"}, {Label: "Proposal"},
		{Label: "Negotiate"}, {Label: "Onboarding"}, {Label: "Expansion"},
	}}
	if w := warner.PostExpandWarnings(ctx, fine, nil); len(w) != 0 {
		t.Errorf("labels that fit should not warn, got %v", w)
	}

	tooLong := &NumberedStepStripValues{Style: "chevron", Steps: []NumberedStepStripStep{
		{Label: "Internationalisation"}, {Label: "Standardisation"}, {Label: "Commercialisation"},
		{Label: "Operationalisation"}, {Label: "Professionalisation"}, {Label: "Decommissioning"},
	}}
	warnings := warner.PostExpandWarnings(ctx, tooLong, nil)
	if len(warnings) != 1 {
		t.Fatalf("want one warning, got %v", warnings)
	}
	if !strings.HasPrefix(warnings[0], ErrCodeBodyTooLong+": ") {
		t.Errorf("warning must carry the parseable code prefix: %q", warnings[0])
	}
	for _, want := range []string{"Internationalisation", "mid-word", "fewer steps"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning missing %q: %q", want, warnings[0])
		}
	}

	// Only the chevron style paints into a pointed shape.
	stacked := &NumberedStepStripValues{Style: "stacked-box", Steps: tooLong.Steps}
	if w := warner.PostExpandWarnings(ctx, stacked, nil); len(w) != 0 {
		t.Errorf("stacked-box has no notch to overflow, got %v", w)
	}
}

// TestChevronStripGeometry_SubtractsTheColumnGap pins the measurement bug the
// fit pass depends on: the pattern asks for gap 0, the grid DTO reads that as
// "unset" and applies its 8pt default, so a chevron is narrower than its share
// of the content area.
func TestChevronStripGeometry_SubtractsTheColumnGap(t *testing.T) {
	ctx := testThemeCtx()
	const count = 6
	geo := chevronStripGeometry(ctx, count)

	w, _ := expandContentSize(ctx)
	share := float64(w) / 12700 / count
	if geo.stepWPt >= share {
		t.Errorf("stepW %.2fpt should be narrower than the %.2fpt column share", geo.stepWPt, share)
	}
	want := (float64(w)/12700 - chevronColGapPt*(count-1)) / count
	if math.Abs(geo.stepWPt-want) > 0.01 {
		t.Errorf("stepW = %.2f, want %.2f", geo.stepWPt, want)
	}
}

package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestValueChainDenseUnbrokenDescriptionWarning(t *testing.T) {
	p := &valueChain{}
	v := &ValueChainValues{Steps: make([]ValueChainStep, 10)}
	for i := range v.Steps {
		v.Steps[i] = ValueChainStep{Label: "Stage", Description: "Brief description"}
	}
	v.Steps[3].Description = strings.Repeat("W", 119)
	got := p.PostExpandWarnings(testThemeCtx(), v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "steps[3].description") || !strings.Contains(got[0], "about 118") {
		t.Fatalf("dense description warning: %v", got)
	}
	v.Steps[3].Description = strings.Repeat("W", 118)
	if got := p.PostExpandWarnings(testThemeCtx(), v, nil); len(got) != 0 {
		t.Fatalf("measured ten-step target should fit: %v", got)
	}
	v.Steps[3].Description = strings.Repeat("word ", 36)
	if got := p.PostExpandWarnings(testThemeCtx(), v, nil); len(got) != 0 {
		t.Fatalf("word-like schema maximum should fit: %v", got)
	}
	v.Steps = v.Steps[:7]
	v.Steps[3].Description = strings.Repeat("W", 180)
	if got := p.PostExpandWarnings(testThemeCtx(), v, nil); len(got) != 0 {
		t.Fatalf("seven-step schema maximum should fit: %v", got)
	}
}

func TestValueChain_Registration(t *testing.T) {
	p, ok := Default().Get("value-chain")
	if !ok {
		t.Fatal("expected value-chain to be registered in default registry")
	}
	if p.Name() != "value-chain" {
		t.Errorf("Name() = %q, want %q", p.Name(), "value-chain")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validValueChainValues(n int) *ValueChainValues {
	steps := make([]ValueChainStep, n)
	labels := []string{"Extraction", "Processing", "Manufacturing", "Distribution", "Retail", "Service", "Recycling", "Disposal", "Recovery", "Renewal"}
	descs := []string{
		"Mining raw materials and managing EPC contracts.",
		"Refining ore into intermediate inputs.",
		"Converting inputs into finished goods.",
		"Moving product through wholesale channels.",
		"Reaching end customers via partner stores.",
		"Supporting customers throughout product lifecycle.",
		"Recovering materials for reuse.",
		"Decommissioning end-of-life inventory.",
		"Returning value back into supply.",
		"Closing the loop on the chain.",
	}
	for i := 0; i < n; i++ {
		steps[i] = ValueChainStep{Label: labels[i], Description: descs[i]}
	}
	return &ValueChainValues{Steps: steps}
}

func TestValueChain_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("value-chain")
	for _, n := range []int{4, 5, 8, 10} {
		t.Run(t.Name(), func(t *testing.T) {
			if err := p.Validate(validValueChainValues(n), nil, nil); err != nil {
				t.Errorf("n=%d: unexpected validation error: %v", n, err)
			}
		})
	}
}

func TestValueChain_Validate_TooFewSteps(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(3)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 4 steps")
	}
	if !strings.Contains(err.Error(), "process-flow") {
		t.Errorf("expected sibling hint mentioning process-flow, got: %v", err)
	}
}

func TestValueChain_Validate_TooManySteps(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(10)
	v.Steps = append(v.Steps, ValueChainStep{Label: "Eleventh"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 10 steps")
	}
}

func TestValueChain_Validate_MissingLabel(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[2].Label = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank label")
	}
}

func TestValueChain_Validate_LabelTooLong(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[1].Label = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for label > 40 chars")
	}
}

func TestValueChain_Validate_DescriptionTooLong(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[1].Description = strings.Repeat("X", 181)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for description > 180 chars")
	}
}

func TestValueChain_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	overrides := map[int]any{99: &ValueChainCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestValueChain_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(8)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 2 {
		t.Fatalf("expected 2 rows (label + description), got %d", got)
	}
	if got := len(grid.Rows[0].Cells); got != 8 {
		t.Errorf("expected 8 label cells, got %d", got)
	}
	if got := len(grid.Rows[1].Cells); got != 8 {
		t.Errorf("expected 8 description cells, got %d", got)
	}
	if string(grid.Columns) != "8" {
		t.Errorf("expected columns=8, got %s", string(grid.Columns))
	}
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the label row (arrows between steps)")
	}
}

func TestValueChain_Expand_BoundaryStepCounts(t *testing.T) {
	p, _ := Default().Get("value-chain")
	for _, n := range []int{4, 10} {
		t.Run(t.Name(), func(t *testing.T) {
			v := validValueChainValues(n)
			grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
			if err != nil {
				t.Fatalf("n=%d: Expand failed: %v", n, err)
			}
			if got := len(grid.Rows[0].Cells); got != n {
				t.Errorf("n=%d: expected %d label cells, got %d", n, n, got)
			}
			if got := len(grid.Rows[1].Cells); got != n {
				t.Errorf("n=%d: expected %d description cells, got %d", n, n, got)
			}
		})
	}
}

func TestValueChain_Expand_HighlightDefaultColor(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(5)
	v.Steps[2].Highlight = true
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Step 2 (highlighted) should fill with accent2 (default highlight color).
	highlighted := grid.Rows[0].Cells[2]
	if highlighted.Shape == nil {
		t.Fatal("expected shape on highlighted label cell")
	}
	if !strings.Contains(string(highlighted.Shape.Fill), "accent2") {
		t.Errorf("expected highlighted label fill to include accent2, got %q", string(highlighted.Shape.Fill))
	}
	// Step 0 (not highlighted) should fill with dk2 (never dk1 black).
	plain := grid.Rows[0].Cells[0]
	if !strings.Contains(string(plain.Shape.Fill), "dk2") {
		t.Errorf("expected non-highlighted label fill to include dk2, got %q", string(plain.Shape.Fill))
	}
}

func TestValueChain_Expand_HighlightCustomColor(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	v.HighlightColor = "accent3"
	v.Steps[0].Highlight = true
	v.Steps[3].Highlight = true
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for _, idx := range []int{0, 3} {
		cell := grid.Rows[0].Cells[idx]
		if !strings.Contains(string(cell.Shape.Fill), "accent3") {
			t.Errorf("step %d: expected fill to include accent3, got %q", idx, string(cell.Shape.Fill))
		}
	}
}

func TestValueChain_Expand_DescriptionFillIsBg1(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, cell := range grid.Rows[1].Cells {
		if cell.Shape == nil {
			t.Fatalf("description cell %d: expected shape", i)
		}
		if !strings.Contains(string(cell.Shape.Fill), "bg1") {
			t.Errorf("description cell %d: expected bg1 fill, got %q", i, string(cell.Shape.Fill))
		}
	}
}

func TestValueChain_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	overrides := &ValueChainOverrides{Accent: "accent4"}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Connector color should follow the accent override.
	if grid.Rows[0].Connector == nil {
		t.Fatal("expected connector on label row")
	}
	if grid.Rows[0].Connector.Color != "accent4" {
		t.Errorf("expected connector color accent4, got %q", grid.Rows[0].Connector.Color)
	}
}

func TestValueChain_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("value-chain")
	v := validValueChainValues(4)
	cellOverrides := map[int]any{1: &ValueChainCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on label cell with override")
	}
}

func TestValueChain_Schema(t *testing.T) {
	p, _ := Default().Get("value-chain")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestValueChain_Taxonomy(t *testing.T) {
	p, _ := Default().Get("value-chain")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want %q", tax.Category, "structural")
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

func TestValueChain_Recommend(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *ContentHints
	}{
		{"porter value chain", "Porter value chain across primary activities", &ContentHints{ItemCount: 6}},
		{"supply chain mapping", "supply chain mapping with 5 steps", &ContentHints{ItemCount: 5}},
		{"operations chain", "operations chain from raw to retail", &ContentHints{ItemCount: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Recommend(reg, tc.intent, tc.hints, 5)
			found := false
			for _, c := range result.Candidates {
				if c.PatternName == "value-chain" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected value-chain in recommendations for intent %q; got %+v", tc.intent, result.Candidates)
			}
		})
	}
}

// TestValueChain_DescriptionRowIsContentSized pins the fix for
// go-slide-creator-pr3g: the description row was uncapped, so it took the
// remaining 75% of the content area and centred the descriptions inside it —
// a full-width empty stripe ran between the step boxes and their descriptions.
func TestValueChain_DescriptionRowIsContentSized(t *testing.T) {
	p, _ := Default().Get("value-chain")
	ctx := testThemeCtx()
	vals := &ValueChainValues{Steps: []ValueChainStep{
		{Label: "Extraction", Description: "Mining raw materials and managing EPC contracts."},
		{Label: "Processing", Description: "Refining ore into intermediate inputs."},
		{Label: "Manufacturing", Description: "Converting inputs into finished goods."},
		{Label: "Distribution", Description: "Moving product through wholesale channels."},
	}}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if grid.VerticalAlign != GridVerticalAlignDefault {
		t.Errorf("vertical_align = %q, want %q", grid.VerticalAlign, GridVerticalAlignDefault)
	}
	if len(grid.Rows) != 2 {
		t.Fatalf("got %d rows, want label + description", len(grid.Rows))
	}
	desc := grid.Rows[1]
	if desc.MaxHeight <= 0 {
		t.Fatal("the description row has no max_height: it stretches to the rest of the zone")
	}

	// A longer description needs a taller row: the cap is measured, not fixed.
	long := &ValueChainValues{Steps: append([]ValueChainStep(nil), vals.Steps...)}
	long.Steps[1].Description = "Refining ore into intermediate inputs across three sites, with the quality gate moved upstream to the concentrator."
	longGrid, err := p.Expand(ctx, long, nil, nil)
	if err != nil {
		t.Fatalf("Expand long: %v", err)
	}
	if longGrid.Rows[1].MaxHeight <= desc.MaxHeight {
		t.Errorf("a longer description should grow the row: short=%.0f long=%.0f", desc.MaxHeight, longGrid.Rows[1].MaxHeight)
	}
}

// TestValueChainFitsStepLabels is go-slide-creator-vo0j1: the label size was
// fixed at 12pt regardless of the step count, so a ten-step chain rendered
// "Manufactur / ing".
func TestValueChainFitsStepLabels(t *testing.T) {
	ctx := testThemeCtx()
	long := []ValueChainStep{
		{Label: "Extraction"}, {Label: "Processing"}, {Label: "Manufacturing"}, {Label: "Distribution"},
		{Label: "Retail"}, {Label: "Service"}, {Label: "Recovery"}, {Label: "Disposal"},
		{Label: "Renewal"}, {Label: "Closure"},
	}

	// Six short labels leave the authored size alone.
	if size, unfit := fitValueChainLabels(ctx, long[:6], 12.0); size != 12.0 || len(unfit) != 0 {
		t.Errorf("six short labels: size=%.1f unfit=%v, want 12 and none", size, unfit)
	}

	// The floor is the RENDERER's floor. Shrinking below it would be undone by
	// the shape_grid renderer, which raises any authored size back to 12pt.
	if valueChainMinLabelPt != shapegrid.MinTextSizePt {
		t.Errorf("label floor %.1f must be the renderer's %.1f, or the fit promises a size the render will not honour",
			valueChainMinLabelPt, shapegrid.MinTextSizePt)
	}

	// Ten steps: the two labels that cannot fit are named, and they are the two
	// the render breaks.
	size, unfit := fitValueChainLabels(ctx, long, 12.0)
	if size < valueChainMinLabelPt {
		t.Errorf("fitted size %.1f is below the renderer's floor", size)
	}
	got := map[string]bool{}
	for _, u := range unfit {
		got[u] = true
	}
	for _, want := range []string{"Manufacturing", "Distribution"} {
		if !got[want] {
			t.Errorf("%q does not fit a ten-step column but was not reported (%v)", want, unfit)
		}
	}
	for _, fits := range []string{"Retail", "Service", "Closure"} {
		if got[fits] {
			t.Errorf("%q fits but was reported as unfit", fits)
		}
	}
}

// TestValueChainWarnsAboutUnfittableLabels: the warning is the blocking
// TEXT_EXCEEDS_SHAPE code, and it stacks with the highlight-contrast warning
// rather than replacing it.
func TestValueChainWarnsAboutUnfittableLabels(t *testing.T) {
	p, _ := Default().Get("value-chain")
	warner, ok := p.(PostExpandWarner)
	if !ok {
		t.Fatal("value-chain must implement PostExpandWarner")
	}
	ctx := testThemeCtx()
	vals := &ValueChainValues{Steps: []ValueChainStep{
		{Label: "Extraction"}, {Label: "Processing"}, {Label: "Manufacturing"}, {Label: "Distribution"},
		{Label: "Retail"}, {Label: "Service"}, {Label: "Recovery"}, {Label: "Disposal"},
		{Label: "Renewal"}, {Label: "Closure"},
	}}
	warnings := warner.PostExpandWarnings(ctx, vals, nil)
	if len(warnings) != 1 {
		t.Fatalf("want one warning, got %v", warnings)
	}
	if !strings.HasPrefix(warnings[0], ErrCodeTextExceedsShape+": ") {
		t.Errorf("warning must carry the blocking code prefix: %q", warnings[0])
	}
	for _, want := range []string{"Manufacturing", "mid-word", "fewer steps"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning missing %q: %q", want, warnings[0])
		}
	}

	// Short labels say nothing.
	short := &ValueChainValues{Steps: []ValueChainStep{
		{Label: "Make"}, {Label: "Move"}, {Label: "Sell"}, {Label: "Serve"},
	}}
	if w := warner.PostExpandWarnings(ctx, short, nil); len(w) != 0 {
		t.Errorf("labels that fit should not warn, got %v", w)
	}
}

package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// bundledThemeColors are the four bundled templates' theme palettes, read from
// templates/*.pptx. They are duplicated here because internal/patterns has no
// template reader; TestValueChainHighlightOnRealTemplates in cmd/json2pptx
// runs the same rule against the real files, so a drift between the two shows
// up there rather than silently here.
var bundledThemeColors = map[string][]types.ThemeColor{
	"forest-green": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "1A3C34"}, {Name: "lt2", RGB: "EDF5F0"},
		{Name: "accent1", RGB: "2E7D32"}, {Name: "accent2", RGB: "FF8F00"},
		{Name: "accent3", RGB: "1565C0"}, {Name: "accent4", RGB: "6A1B9A"},
		{Name: "accent5", RGB: "00838F"}, {Name: "accent6", RGB: "C62828"},
	},
	"midnight-blue": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "1B2A4A"}, {Name: "lt2", RGB: "E8ECF1"},
		{Name: "accent1", RGB: "2E5090"}, {Name: "accent2", RGB: "D4463A"},
		{Name: "accent3", RGB: "E8A838"}, {Name: "accent4", RGB: "43A047"},
		{Name: "accent5", RGB: "5C6BC0"}, {Name: "accent6", RGB: "26A69A"},
	},
	"modern-template": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "2C3932"}, {Name: "lt2", RGB: "FDF6EA"},
		{Name: "accent1", RGB: "B5485A"}, {Name: "accent2", RGB: "4A7AB5"},
		{Name: "accent3", RGB: "3A7D5C"}, {Name: "accent4", RGB: "6B5B8D"},
		{Name: "accent5", RGB: "C06B3F"}, {Name: "accent6", RGB: "9E8520"},
	},
	"warm-coral": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "3E2723"}, {Name: "lt2", RGB: "FBE9E7"},
		{Name: "accent1", RGB: "E64A19"}, {Name: "accent2", RGB: "5D4037"},
		{Name: "accent3", RGB: "FF8A65"}, {Name: "accent4", RGB: "0097A7"},
		{Name: "accent5", RGB: "7B1FA2"}, {Name: "accent6", RGB: "689F38"},
	},
}

func valueChainSteps() []ValueChainStep {
	return []ValueChainStep{
		{Label: "Extraction", Description: "Ore out of the ground."},
		{Label: "Processing", Description: "Concentrate and refine."},
		{Label: "Manufacturing", Description: "Converting inputs into finished goods.", Highlight: true},
		{Label: "Distribution", Description: "To the customer."},
	}
}

// fillOf reads the fill colour name out of an expanded cell.
func fillOf(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("cannot read fill %s: %v", raw, err)
	}
	return obj.Color
}

// The highlight is the pattern's one semantic signal. Painted from a fixed
// scheme slot it was only as visible as the gap between two slots in whatever
// template the deck landed on: on warm-coral dk2 #3E2723 against accent2
// #5D4037 is 1.48:1, so the highlighted step was indistinguishable from its
// neighbours (go-slide-creator-ah5s).
func TestValueChainHighlightIsDistinctOnEveryBundledTheme(t *testing.T) {
	vc := &valueChain{}
	for name, colors := range bundledThemeColors {
		t.Run(name, func(t *testing.T) {
			ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
			vals := &ValueChainValues{Steps: valueChainSteps()}
			grid, err := vc.Expand(ctx, vals, nil, nil)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}

			step := valueChainStepTone(ctx, ctx.DefaultAccent())
			if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != string(step.fillJSON()) {
				t.Fatalf("plain step fill = %s, want the accent's content swatch %s", got, step.fillJSON())
			}
			base := valueChainLabelFillName
			highlight := fillOf(t, grid.Rows[0].Cells[2].Shape.Fill)
			ratio, ok := fillContrast(ctx, step, fillTone{Color: highlight})
			if !ok {
				t.Fatalf("cannot measure %s vs %s", base, highlight)
			}
			if ratio < valueChainHighlightMin {
				t.Errorf("highlight %s on %s reads at %.2f:1, want >= %.1f", highlight, base, ratio, valueChainHighlightMin)
			}
			t.Logf("%s: %s on %s = %.2f:1", name, highlight, base, ratio)
		})
	}
}

// Without a theme there is nothing to measure, so the historical default
// stands — an expansion with no template must not change.
func TestValueChainHighlightDefaultWithoutTheme(t *testing.T) {
	vc := &valueChain{}
	grid, err := vc.Expand(ExpandContext{}, &ValueChainValues{Steps: valueChainSteps()}, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := fillOf(t, grid.Rows[0].Cells[2].Shape.Fill); got != valueChainDefaultHighlight {
		t.Errorf("highlight = %q, want the historical default %q", got, valueChainDefaultHighlight)
	}
}

// An authored highlight_color is honoured — the author may know something the
// measurement does not — but a highlight that cannot be seen is reported.
func TestValueChainAuthoredHighlightIsHonouredAndReported(t *testing.T) {
	vc := &valueChain{}
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: bundledThemeColors["midnight-blue"]}}
	// midnight-blue accent3 #E8A838 is 1.48:1 against the step fill, accent1's
	// Lighter 80% swatch — under the 1.6:1 bar.
	vals := &ValueChainValues{Steps: valueChainSteps(), HighlightColor: "accent3"}

	grid, err := vc.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := fillOf(t, grid.Rows[0].Cells[2].Shape.Fill); got != "accent3" {
		t.Errorf("authored highlight_color was overridden: got %q", got)
	}

	warnings := vc.PostExpandWarnings(ctx, vals, nil)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.HasPrefix(warnings[0], ErrCodeLowContrastHighlight+":") {
		t.Errorf("warning %q does not carry the finding code", warnings[0])
	}
	if !strings.Contains(warnings[0], "1.48:1") {
		t.Errorf("warning should quote the measured ratio, got %q", warnings[0])
	}
	// The finding names what the highlight was measured against and the bar.
	if want := "against the step fill (the accent's Lighter 80% swatch)"; !strings.Contains(warnings[0], want) ||
		!strings.Contains(warnings[0], "("+valueChainLabelFillName+")") {
		t.Errorf("warning should name the step fill %q, got %q", want, warnings[0])
	}
	if !strings.Contains(warnings[0], "below 1.6:1") || !strings.Contains(warnings[0], fmt.Sprintf("below %.1f:1", tonalEmphasisMin)) {
		t.Errorf("warning should quote the 1.6:1 bar (tonalEmphasisMin), got %q", warnings[0])
	}
	// The plain steps it is measured against are that swatch.
	if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `{"color":"accent1","lumMod":20000,"lumOff":80000}` {
		t.Errorf("plain step fill = %s, want accent1's Lighter 80%% swatch", got)
	}

	// The colour that used to be reported on warm-coral — accent3 #FF8A65,
	// 1.59:1 against the old neutral 16% step — now reads at 1.64:1 against
	// warm-coral's own accent swatch and clears the bar.
	coral := ExpandContext{Theme: types.ThemeInfo{Colors: bundledThemeColors["warm-coral"]}}
	if ratio, ok := fillContrast(coral, valueChainStepTone(coral, coral.DefaultAccent()), fillTone{Color: "accent3"}); !ok || ratio < valueChainHighlightMin || ratio > 1.7 {
		t.Errorf("warm-coral accent3 on the step fill = %.2f:1 (ok=%t), want just above the %.1f:1 bar", ratio, ok, valueChainHighlightMin)
	}
	if w := vc.PostExpandWarnings(coral, vals, nil); len(w) != 0 {
		t.Errorf("warm-coral accent3 clears the bar but drew %v", w)
	}

	// A highlight that does read draws nothing.
	vals.HighlightColor = "accent2"
	if w := vc.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
		t.Errorf("a distinct authored highlight drew %v", w)
	}
	// Neither does a chain with no highlighted step.
	vals.HighlightColor = "accent3"
	for i := range vals.Steps {
		vals.Steps[i].Highlight = false
	}
	if w := vc.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
		t.Errorf("a chain with no highlighted step drew %v", w)
	}
}

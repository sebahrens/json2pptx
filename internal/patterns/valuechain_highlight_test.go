package patterns

import (
	"encoding/json"
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

			base := fillOf(t, grid.Rows[0].Cells[0].Shape.Fill)
			highlight := fillOf(t, grid.Rows[0].Cells[2].Shape.Fill)
			if highlight == base {
				t.Fatalf("highlighted step uses the same fill as its neighbours (%s)", base)
			}
			ratio, ok := fillContrast(ctx, fillTone{Color: base}, fillTone{Color: highlight})
			if !ok {
				t.Fatalf("cannot measure %s vs %s", base, highlight)
			}
			if ratio < fillDistinctnessMin {
				t.Errorf("highlight %s on %s reads at %.2f:1, want >= %.1f", highlight, base, ratio, fillDistinctnessMin)
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
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: bundledThemeColors["warm-coral"]}}
	vals := &ValueChainValues{Steps: valueChainSteps(), HighlightColor: "accent2"}

	grid, err := vc.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := fillOf(t, grid.Rows[0].Cells[2].Shape.Fill); got != "accent2" {
		t.Errorf("authored highlight_color was overridden: got %q", got)
	}

	warnings := vc.PostExpandWarnings(ctx, vals, nil)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.HasPrefix(warnings[0], ErrCodeLowContrastHighlight+":") {
		t.Errorf("warning %q does not carry the finding code", warnings[0])
	}
	if !strings.Contains(warnings[0], "1.48") {
		t.Errorf("warning should quote the measured ratio, got %q", warnings[0])
	}

	// A highlight that does read draws nothing.
	vals.HighlightColor = "accent3"
	if w := vc.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
		t.Errorf("a distinct authored highlight drew %v", w)
	}
	// Neither does a chain with no highlighted step.
	vals.HighlightColor = "accent2"
	for i := range vals.Steps {
		vals.Steps[i].Highlight = false
	}
	if w := vc.PostExpandWarnings(ctx, vals, nil); len(w) != 0 {
		t.Errorf("a chain with no highlighted step drew %v", w)
	}
}

package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// On business-template accent1 (#AD84C6) and accent2 (#8784C7) are both mid
// purples, so negative deltas painted accent2 were indistinguishable from the
// accent1 totals (go-slide-creator-csclk.97).
func TestWaterfallNegativeAccentDistinctFromTotals(t *testing.T) {
	business := []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "373545"}, {Name: "lt2", RGB: "DCD8DC"},
		{Name: "accent1", RGB: "AD84C6"}, {Name: "accent2", RGB: "8784C7"},
		{Name: "accent3", RGB: "5D739A"}, {Name: "accent4", RGB: "6997AF"},
		{Name: "accent5", RGB: "84ACB6"}, {Name: "accent6", RGB: "6F8183"},
	}
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: business}}
	if !waterfallFillsCollide(ctx, "accent1", "accent2") {
		t.Fatal("business-template accent1/accent2 should collide")
	}
	pick, ok := pickDistinctFill(ctx, fillTone{Color: "accent1"}, fillDistinctnessMin, waterfallNegativeFallbacks...)
	if !ok || waterfallFillsCollide(ctx, "accent1", pick) {
		t.Fatalf("no distinct negative fill picked: %q ok=%v", pick, ok)
	}
	// The bundled palettes whose accent1/accent2 already differ keep accent2.
	for name, colors := range bundledThemeColors {
		c := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		if waterfallFillsCollide(c, "accent1", "accent2") {
			t.Errorf("%s: accent1/accent2 unexpectedly collide", name)
		}
	}
}

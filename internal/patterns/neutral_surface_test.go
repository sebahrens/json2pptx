package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Neutral surface fills as the expanders emit them (go-slide-creator-pgdkp,
// go-slide-creator-8xsj3).
const (
	neutral4JSON  = `{"color":"dk1","lumMod":4000,"lumOff":96000}`
	neutral8JSON  = `{"color":"dk1","lumMod":8000,"lumOff":92000}`
	neutral16JSON = `{"color":"dk1","lumMod":16000,"lumOff":84000}`
)

func TestNeutralToneResolvesToGreyOnBlackInk(t *testing.T) {
	ctx := fullThemeCtx()
	for pct, want := range map[int]string{4: "#F5F5F5", 8: "#EBEBEB", 16: "#D6D6D6", 60: "#666666"} {
		got, ok := effectiveFillColor(ctx, neutralTone(pct))
		if !ok {
			t.Fatalf("neutral %d%% did not resolve", pct)
		}
		if got.Hex() != want {
			t.Errorf("neutral %d%% = %s, want %s", pct, got.Hex(), want)
		}
	}
}

func TestStructuralDarkToneAvoidsBlackDk2(t *testing.T) {
	black := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"}, {Name: "dk2", RGB: "000000"},
	}}}
	if got := structuralDarkTone(black); got != neutralTone(NeutralTint60) {
		t.Errorf("black dk2: structural fill = %+v, want neutral 60%%", got)
	}
	navy := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"}, {Name: "dk2", RGB: "1B2A4A"},
	}}}
	if got := structuralDarkTone(navy); got != (fillTone{Color: "dk2"}) {
		t.Errorf("navy dk2: structural fill = %+v, want dk2", got)
	}
}

func TestSurfacePairNeverOutlinesOrRepeatsPageColour(t *testing.T) {
	// No metadata: the two neutral steps.
	a, b := surfacePairJSON(ExpandContext{})
	if string(a) != neutral4JSON || string(b) != neutral8JSON {
		t.Errorf("undeclared pair = %s / %s, want neutral 4%% / 8%%", a, b)
	}
	// Shipped metadata (subtle lt2, paper lt1): the declared lt2 is kept and
	// the page-coloured paper becomes the 4% step, never a white card.
	ctx := ExpandContext{Metadata: &types.TemplateMetadata{SurfaceTints: map[string]string{"subtle": "lt2", "paper": "lt1"}}}
	a, b = surfacePairJSON(ctx)
	if string(a) != `"lt2"` || string(b) != neutral4JSON {
		t.Errorf("declared pair = %s / %s, want lt2 / neutral 4%%", a, b)
	}
}

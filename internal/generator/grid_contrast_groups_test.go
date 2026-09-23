package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// tintTheme mirrors the reported template: one accent whose tints fill the
// tiers, a near-black dk2 that misses AA on the darkest tint, and a pure-black
// dk1 the fix should not have to reach for.
func tintTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#373545"},
		{Name: "accent1", RGB: "#AD84C6"},
	}
}

// tintedCell builds one shape-grid cell: a solid fill with a white label.
func tintedCell(fillHex, textHex string) []byte {
	return []byte(fmt.Sprintf(
		`<p:sp><p:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></p:spPr>`+
			`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1400" b="1">`+
			`<a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr><a:t>Tier</a:t></a:r></a:p></p:txBody></p:sp>`,
		strings.TrimPrefix(fillHex, "#"), strings.TrimPrefix(textHex, "#")))
}

// textColorsOf returns the text colours a shape carries, uppercased.
func textColorsOf(t *testing.T, shape []byte, theme []types.ThemeColor) []string {
	t.Helper()
	return textColorsIn(shapeTextBody(shape), theme)
}

// TestGridSiblingsShareOneTextColor is the go-slide-creator-tnx3e acceptance
// test: four tiers of progressive tints, all labelled white, must come out in
// ONE colour rather than white / black / dk2 across four sibling cells.
func TestGridSiblingsShareOneTextColor(t *testing.T) {
	theme := tintTheme()
	shapes := [][]byte{
		tintedCell("#AD84C6", "#FFFFFF"),
		tintedCell("#B590CC", "#FFFFFF"),
		tintedCell("#BD9DD1", "#FFFFFF"),
		tintedCell("#C6A9D7", "#FFFFFF"),
	}

	fixed, swaps := enforceShapeGridContrast(shapes, theme, nil, 4)

	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1 for the group: %+v", len(swaps), swaps)
	}
	if swaps[0].Cells != 4 {
		t.Errorf("swap covers %d cells, want 4", swaps[0].Cells)
	}
	if swaps[0].Source != gridGroupSwapSource {
		t.Errorf("swap source = %q, want %q", swaps[0].Source, gridGroupSwapSource)
	}

	colors := map[string]bool{}
	for i, shape := range fixed {
		for _, c := range textColorsOf(t, shape, theme) {
			colors[c] = true
		}
		if len(textColorsOf(t, shape, theme)) == 0 {
			t.Fatalf("cell %d lost its text colour", i)
		}
	}
	if len(colors) != 1 {
		t.Errorf("four sibling tiers came out in %d colours (%v) — they should share one", len(colors), colors)
	}

	// And the one colour has to read on every tier, including the darkest.
	var chosen svggen.Color
	for c := range colors {
		parsed, err := svggen.ParseColor(c)
		if err != nil {
			t.Fatalf("unparseable colour %q", c)
		}
		chosen = parsed
	}
	for _, fillHex := range []string{"#AD84C6", "#B590CC", "#BD9DD1", "#C6A9D7"} {
		fill, err := svggen.ParseColor(fillHex)
		if err != nil {
			t.Fatal(err)
		}
		if r := chosen.ContrastWith(fill); r < svggen.WCAGAALarge {
			t.Errorf("the group colour reads %.2f on %s, below the large-text bar", r, fillHex)
		}
	}
}

// The reported modern-yellow KPI row contains a bold 36pt value and an
// unmarked 14pt caption in each cell. The caption needs 4.5:1 on every fill,
// even where white already clears the 3:1 large-text threshold.
func TestKPIGroupContrastUsesRegularCaptionThreshold(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk1", RGB: "#000000"},
		{Name: "dk2", RGB: "#2D4C69"},
		{Name: "accent1", RGB: "#FFC000"},
		{Name: "accent2", RGB: "#5B9BD5"},
		{Name: "accent3", RGB: "#ED7D31"},
		{Name: "accent4", RGB: "#A5A5A5"},
	}
	fills := []string{"#FFC000", "#5B9BD5", "#ED7D31", "#A5A5A5"}
	shapes := make([][]byte, len(fills))
	for i, fill := range fills {
		shapes[i] = []byte(fmt.Sprintf(`<p:sp><p:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></p:spPr>`+
			`<p:txBody><a:bodyPr/><a:lstStyle/><a:p>`+
			`<a:r><a:rPr sz="3600" b="1"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:rPr><a:t>42</a:t></a:r>`+
			`<a:r><a:rPr sz="1400"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:rPr><a:t>Caption</a:t></a:r>`+
			`</a:p></p:txBody></p:sp>`, strings.TrimPrefix(fill, "#")))
	}
	fixed, _ := enforceShapeGridContrast(shapes, theme, computeWhiteTextSafeHex(theme), 0)
	colors := map[string]bool{}
	for i, shape := range fixed {
		inkHexes := textColorsIn(shapeTextBody(shape), theme)
		if len(inkHexes) != 1 {
			t.Fatalf("cell %d has %v inks, want one for value and caption", i, inkHexes)
		}
		colors[inkHexes[0]] = true
		ink := svggen.MustParseColor(inkHexes[0])
		if ratio := ink.ContrastWith(svggen.MustParseColor(fills[i])); ratio < svggen.WCAGAANormal {
			t.Errorf("cell %d: %s on %s has %.2f contrast, want >= 4.5 for 14pt caption", i, inkHexes[0], fills[i], ratio)
		}
	}
	if len(colors) != 1 {
		t.Errorf("sibling KPI cards use %d text colors (%v), want one", len(colors), colors)
	}
}

// TestGroupPrefersATonalShadeOverPureBlack pins the reported preference: when
// the palette's dk2 misses the bar on one tint, the fix darkens the fill's own
// hue rather than dropping the whole diagram to #000000.
func TestGroupPrefersATonalShadeOverPureBlack(t *testing.T) {
	theme := tintTheme()
	shapes := [][]byte{
		tintedCell("#AD84C6", "#FFFFFF"),
		tintedCell("#B590CC", "#FFFFFF"),
		tintedCell("#BD9DD1", "#FFFFFF"),
		tintedCell("#C6A9D7", "#FFFFFF"),
	}
	_, swaps := enforceShapeGridContrast(shapes, theme, nil, 0)
	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1", len(swaps))
	}
	if swaps[0].ReplacedColor == "#000000" {
		t.Error("the group fell back to pure black; a tonal shade of the fill should have been preferred")
	}
	chosen, err := svggen.ParseColor(swaps[0].ReplacedColor)
	if err != nil {
		t.Fatalf("unparseable replacement %q", swaps[0].ReplacedColor)
	}
	// A tonal shade keeps the hue: its red and blue channels stay apart, the way
	// the purple fill's do, instead of collapsing to a neutral.
	r, _, b := chosen.R, chosen.G, chosen.B
	if r == b {
		t.Errorf("replacement %s is neutral; expected a shade of the fill's hue", swaps[0].ReplacedColor)
	}
	_ = r
	_ = b
}

// TestGroupLeavesMixedFillsToThePerShapePass pins the guard: dark cards and a
// pale card can share a white label and have NO single readable colour between
// them. Forcing one would make the dark cards worse to help the pale one, so the
// group decision stands down.
func TestGroupLeavesMixedFillsToThePerShapePass(t *testing.T) {
	theme := tintTheme()
	shapes := [][]byte{
		tintedCell("#1B2A4A", "#FFFFFF"), // white reads fine here
		tintedCell("#F2EFF6", "#FFFFFF"), // and not at all here
	}
	fixed, swaps := enforceShapeGridContrast(shapes, theme, nil, 0)

	for _, s := range swaps {
		if s.Source == gridGroupSwapSource {
			t.Errorf("a group decision was forced on incompatible fills: %+v", s)
		}
	}
	// The dark card keeps its white label; the pale one is fixed per-shape.
	if !strings.Contains(string(fixed[0]), "FFFFFF") {
		t.Errorf("the readable dark card lost its white label: %s", fixed[0])
	}
	if strings.Contains(string(fixed[1]), "FFFFFF") {
		t.Errorf("the pale card kept its unreadable white label: %s", fixed[1])
	}
}

// TestGroupLeavesReadableSiblingsAlone pins that a group already reading well is
// not touched: no swap, no rewritten XML.
func TestGroupLeavesReadableSiblingsAlone(t *testing.T) {
	theme := tintTheme()
	shapes := [][]byte{
		tintedCell("#1B2A4A", "#FFFFFF"),
		tintedCell("#22335C", "#FFFFFF"),
	}
	before := []string{string(shapes[0]), string(shapes[1])}
	fixed, swaps := enforceShapeGridContrast(shapes, theme, nil, 0)
	if len(swaps) != 0 {
		t.Errorf("readable siblings should produce no swap: %+v", swaps)
	}
	for i := range fixed {
		if string(fixed[i]) != before[i] {
			t.Errorf("cell %d was rewritten although it read fine", i)
		}
	}
}

// TestGroupHandlesSchemeSpelledText pins that a label written as schemeClr joins
// the group of the colour it RESOLVES to — the reported deck spells the same
// colour both ways across cells.
func TestGroupHandlesSchemeSpelledText(t *testing.T) {
	theme := tintTheme()
	schemeCell := []byte(`<p:sp><p:spPr><a:solidFill><a:srgbClr val="C6A9D7"/></a:solidFill></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1400" b="1">` +
		`<a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>Tier</a:t></a:r></a:p></p:txBody></p:sp>`)
	shapes := [][]byte{tintedCell("#AD84C6", "#FFFFFF"), schemeCell}

	fixed, swaps := enforceShapeGridContrast(shapes, theme, nil, 0)
	if len(swaps) != 1 || swaps[0].Cells != 2 {
		t.Fatalf("scheme-spelled white should join the group: %+v", swaps)
	}
	for i, shape := range fixed {
		if strings.Contains(string(shape), `val="lt1"`) {
			t.Errorf("cell %d still names lt1; the group colour must be pinned explicitly", i)
		}
	}
}

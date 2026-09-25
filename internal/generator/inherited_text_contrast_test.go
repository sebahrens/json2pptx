package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// modernLikeTheme mirrors the palette that produced the bug: a white lt1, a
// black dk1 and a dark accent (dk2) the fix can reach for.
func modernLikeTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#2C3932"},
		{Name: "lt2", RGB: "#FDF6EA"},
		{Name: "accent1", RGB: "#6C5CE7"},
	}
}

// invertedSectionLayout is modern-template's section divider, reduced to what
// matters: a color map override that swaps the light and dark slots, a
// background painted with the (now light) tx1, a title that names its own color
// and a body placeholder that names none.
const invertedSectionLayout = `<?xml version="1.0"?>
<p:sldLayout xmlns:p="p" xmlns:a="a">
 <p:cSld>
  <p:bg><p:bgPr><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></p:bgPr></p:bg>
  <p:spTree>
   <p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
    <p:txBody><a:lstStyle><a:lvl1pPr><a:defRPr sz="6500"><a:solidFill><a:schemeClr val="bg1"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle></p:txBody></p:sp>
   <p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
    <p:txBody><a:lstStyle/></p:txBody></p:sp>
  </p:spTree>
 </p:cSld>
 <p:clrMapOvr><a:overrideClrMapping bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2" accent1="accent1"/></p:clrMapOvr>
</p:sldLayout>`

// masterWithTx1Body is the master those placeholders fall back to: body text
// coloured schemeClr tx1, which the layout's override turns into lt1.
const masterWithTx1Body = `<?xml version="1.0"?>
<p:sldMaster xmlns:p="p" xmlns:a="a">
 <p:txStyles>
  <p:titleStyle><a:lvl1pPr><a:defRPr sz="4500"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr></p:titleStyle>
  <p:bodyStyle><a:lvl1pPr><a:defRPr sz="1400"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr></p:bodyStyle>
  <p:otherStyle><a:lvl1pPr><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr></p:otherStyle>
 </p:txStyles>
</p:sldMaster>`

// bodySlide builds a slide carrying one body placeholder whose runs state no
// color — the shape the old pass could not see.
func bodySlide(runs ...string) *slideXML {
	idx := 1
	paras := make([]paragraphXML, 0, len(runs))
	for _, text := range runs {
		paras = append(paras, paragraphXML{Runs: []runXML{{
			RunProperties: &runPropertiesXML{Lang: "en-US"},
			Text:          text,
		}}})
	}
	return &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
			NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: &idx}},
		},
		TextBody: &textBodyXML{ListStyle: &listStyleXML{}, Paragraphs: paras},
	}}}}}
}

func slideRunFills(slide *slideXML) []string {
	var out []string
	for _, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		if shape.TextBody == nil {
			continue
		}
		for _, p := range shape.TextBody.Paragraphs {
			for _, r := range p.Runs {
				if r.RunProperties != nil {
					out = append(out, r.RunProperties.Inner)
				}
			}
		}
	}
	return out
}

// TestInheritedTextContrast_WhiteOnWhite is the go-slide-creator-ucmgr repro:
// bullets that state no color inherit tx1 from the master, the layout's override
// turns that into white, and the layout's background is white too. The old pass
// saw no color to rewrite and emitted neither a fix nor a finding.
func TestInheritedTextContrast_WhiteOnWhite(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)
	bgHex := extractLayoutBackgroundColor(layout, theme)
	if bgHex != "#FFFFFF" {
		t.Fatalf("layout background resolved to %q, want #FFFFFF (tx1 through the override)", bgHex)
	}

	slide := bodySlide("Financial Performance Summary", "Customer Growth Metrics")
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), bgHex, theme, 1, override)

	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1 (one per shape, not one per run)", len(swaps))
	}
	swap := swaps[0]
	if swap.OriginalColor != "#FFFFFF" {
		t.Errorf("swap.OriginalColor = %q, want the inherited #FFFFFF", swap.OriginalColor)
	}
	if swap.BackgroundColor != "#FFFFFF" {
		t.Errorf("swap.BackgroundColor = %q, want #FFFFFF", swap.BackgroundColor)
	}
	if swap.RatioBefore > 1.01 {
		t.Errorf("swap.RatioBefore = %.2f, want ~1.0 (white on white)", swap.RatioBefore)
	}
	if swap.RatioAfter < 4.5 {
		t.Errorf("swap.RatioAfter = %.2f, want >= 4.5 (WCAG AA normal)", swap.RatioAfter)
	}
	if swap.SlideIndex != 1 || swap.Path != "/slides/1" {
		t.Errorf("swap location = slide %d %q, want slide 1 /slides/1", swap.SlideIndex, swap.Path)
	}
	if swap.Source != inheritedSourceMaster {
		t.Errorf("swap.Source = %q, want %q — the color came from the master", swap.Source, inheritedSourceMaster)
	}

	fills := slideRunFills(slide)
	if len(fills) != 2 {
		t.Fatalf("got %d runs, want 2", len(fills))
	}
	for i, inner := range fills {
		if !strings.Contains(inner, "<a:solidFill><a:srgbClr val=") {
			t.Errorf("run %d was not given an explicit color: %q", i, inner)
		}
		if !strings.Contains(inner, strings.TrimPrefix(swap.ReplacedColor, "#")) {
			t.Errorf("run %d color does not match the recorded swap %q: %q", i, swap.ReplacedColor, inner)
		}
	}
}

// TestInheritedTextContrast_ReadableIsLeftAlone pins that the pass is silent
// when the inherited color already reads: no swap, no rewritten XML, so a deck
// on a template without an inverted layout is byte-identical.
func TestInheritedTextContrast_ReadableIsLeftAlone(t *testing.T) {
	theme := modernLikeTheme()
	// No override: tx1 stays dk1 (black) and the background is lt1 (white).
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:schemeClr val="bg1"/></a:solidFill></p:bgPr></p:bg><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:lstStyle/></p:txBody></p:sp>
	</p:spTree></p:cSld></p:sldLayout>`)
	bgHex := extractLayoutBackgroundColor(layout, theme)

	slide := bodySlide("Readable already")
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), bgHex, theme, 0, nil)
	if len(swaps) != 0 {
		t.Fatalf("readable text should be left alone, got swaps=%+v", swaps)
	}
	for _, inner := range slideRunFills(slide) {
		if strings.Contains(inner, "solidFill") {
			t.Errorf("run XML was rewritten: %q", inner)
		}
	}
}

// TestInheritedTextContrast_LayoutStyleWins pins the resolution order: a color
// stated by the LAYOUT's placeholder beats the master's default, and the swap
// says so.
func TestInheritedTextContrast_LayoutStyleWins(t *testing.T) {
	theme := modernLikeTheme()
	// The layout paints the body white itself; the master would have said tx1.
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:bgPr></p:bg><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		 <p:txBody><a:lstStyle><a:lvl1pPr><a:defRPr sz="1400"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle></p:txBody></p:sp>
	</p:spTree></p:cSld></p:sldLayout>`)

	slide := bodySlide("White on white from the layout")
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), "#FFFFFF", theme, 0, nil)
	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1", len(swaps))
	}
	if swaps[0].Source != inheritedSourceLayout {
		t.Errorf("swap.Source = %q, want %q", swaps[0].Source, inheritedSourceLayout)
	}
}

func TestInheritedTextContrast_UsesPlaceholderSolidFillBeforeSlideCanvas(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:bgPr></p:bg><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		<p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr>
		<p:txBody><a:lstStyle/></p:txBody></p:sp>
	</p:spTree></p:cSld></p:sldLayout>`)
	slide := bodySlide("Black text on a dark placeholder")
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), "#FFFFFF", modernLikeTheme(), 0, nil)
	if len(swaps) != 1 || swaps[0].BackgroundColor != "#1B2A4A" || swaps[0].RatioAfter < 4.5 {
		t.Fatalf("layout placeholder fill must supersede white canvas: %+v", swaps)
	}
}

func TestPlaceholderSolidFillAlphaUsesVisibleSlideCanvas(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:spTree><p:sp><p:nvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr><a:solidFill><a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr></a:solidFill></p:spPr></p:sp></p:spTree></p:cSld></p:sldLayout>`)
	idx := 1
	ph := &placeholderXML{Type: "body", Index: &idx}
	for _, tc := range []struct{ canvas, want string }{{"#000000", "#800000"}, {"#FFFFFF", "#FF8080"}, {"", ""}} {
		if got := layoutPlaceholderSolidFill(layout, ph, modernLikeTheme(), tc.canvas, nil); got != tc.want {
			t.Errorf("render fill on %s = %s, want %s", tc.canvas, got, tc.want)
		}
	}
}

func TestExplicitTextContrast_UsesPlaceholderSolidFillWithoutSlideCanvas(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		<p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr></p:sp>
	</p:spTree></p:cSld></p:sldLayout>`)
	slide := bodySlide("Black run on dark placeholder")
	slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.Paragraphs[0].Runs[0].RunProperties.Inner = `<a:solidFill><a:schemeClr val="dk1"/></a:solidFill>`
	swaps := enforceTextContrastInSlide(slide, "", modernLikeTheme(), 0, nil, false, layout)
	if len(swaps) != 1 || swaps[0].BackgroundColor != "#1B2A4A" || swaps[0].RatioAfter < 4.5 {
		t.Fatalf("explicit text must use its layout placeholder fill even without p:bg: %+v", swaps)
	}
}

func TestInheritedTextContrast_MapsPlaceholderFillThroughLayoutColorMap(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		<p:spPr><a:solidFill><a:schemeClr val="bg1"/></a:solidFill></p:spPr><p:txBody><a:lstStyle/></p:txBody></p:sp>
	</p:spTree></p:cSld><p:clrMapOvr><a:overrideClrMapping bg1="dk1"/></p:clrMapOvr></p:sldLayout>`)
	slide := bodySlide("Black text on mapped dark fill")
	bg := "#FFFFFF"
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), bg, modernLikeTheme(), 0, parseLayoutColorMapOverride(layout))
	if len(swaps) != 1 || swaps[0].BackgroundColor != "#000000" {
		t.Fatalf("layout color map was not applied to placeholder fill: %+v", swaps)
	}
}

func TestInheritedTextContrast_ColorlessLayoutInheritsMasterColor(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		<p:txBody><a:lstStyle><a:lvl1pPr><a:defRPr sz="1200"/></a:lvl1pPr></a:lstStyle></p:txBody></p:sp>
		</p:spTree></p:cSld><p:clrMapOvr><a:overrideClrMapping tx1="lt1"/></p:clrMapOvr></p:sldLayout>`)
	slide := bodySlide("Inherited white text")
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), "#FFFFFF", modernLikeTheme(), 0,
		parseLayoutColorMapOverride(layout))
	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want one master-color repair: %+v", len(swaps), swaps)
	}
	if swaps[0].Source != inheritedSourceMaster || swaps[0].OriginalColor != "#FFFFFF" || swaps[0].RatioAfter < 4.5 {
		t.Errorf("master color was not repaired at the layout's 12pt threshold: %+v", swaps[0])
	}
	if !strings.Contains(slideRunFills(slide)[0], strings.TrimPrefix(swaps[0].ReplacedColor, "#")) {
		t.Errorf("repair was not pinned onto the slide's run: %q", slideRunFills(slide)[0])
	}
	withoutMaster := bodySlide("Unknown inherited color")
	if missing := enforceInheritedTextContrast(withoutMaster, layout, nil, "#FFFFFF", modernLikeTheme(), 0, nil); len(missing) != 0 {
		t.Errorf("colorless layout without master cannot justify a swap: %+v", missing)
	}
}

// TestInheritedTextContrast_SkipsStatedColors pins the division of labour: a
// shape that names a color anywhere belongs to the existing pass, and fixing it
// here too would record two swaps for one change.
func TestInheritedTextContrast_SkipsStatedColors(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)

	t.Run("a run states its color", func(t *testing.T) {
		slide := bodySlide("Stated")
		slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.Paragraphs[0].Runs[0].RunProperties.Inner =
			`<a:solidFill><a:schemeClr val="tx1"/></a:solidFill>`
		swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), "#FFFFFF", theme, 0, override)
		if len(swaps) != 0 {
			t.Errorf("a shape with a stated run color is the other pass's job, got %+v", swaps)
		}
	})

	t.Run("the slide's own lstStyle states it", func(t *testing.T) {
		slide := bodySlide("Stated")
		slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner =
			`<a:lvl1pPr><a:defRPr><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr>`
		swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), "#FFFFFF", theme, 0, override)
		if len(swaps) != 0 {
			t.Errorf("a shape with a stated lstStyle color is the other pass's job, got %+v", swaps)
		}
	})
}

// TestInheritedTextContrast_NonPlaceholdersAndEmptyText pins what the pass does
// not touch: a shape with no placeholder identity has no inheritance chain to
// resolve, and a shape with no text has nothing to colour.
func TestInheritedTextContrast_NonPlaceholdersAndEmptyText(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)

	noPlaceholder := bodySlide("Free text box")
	noPlaceholder.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.NvPr.Placeholder = nil
	if swaps := enforceInheritedTextContrast(noPlaceholder, layout, []byte(masterWithTx1Body), "#FFFFFF", theme, 0, override); len(swaps) != 0 {
		t.Errorf("non-placeholder shape should be skipped, got %+v", swaps)
	}

	empty := bodySlide("   ")
	if swaps := enforceInheritedTextContrast(empty, layout, []byte(masterWithTx1Body), "#FFFFFF", theme, 0, override); len(swaps) != 0 {
		t.Errorf("shape with no text should be skipped, got %+v", swaps)
	}
}

// TestInheritedTextContrast_HostilePaletteStillReads pins that the fix does not
// depend on the template having a usable color: with nothing but mid-grey in the
// palette, the pass falls back to a tonal shade of the background and still
// clears WCAG AA.
func TestInheritedTextContrast_HostilePaletteStillReads(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "dk1", RGB: "#767676"},
		{Name: "lt1", RGB: "#767676"},
		{Name: "dk2", RGB: "#767676"},
		{Name: "lt2", RGB: "#767676"},
	}
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="767676"/></a:solidFill></p:bgPr></p:bg><p:spTree>
		<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
		 <p:txBody><a:lstStyle><a:lvl1pPr><a:defRPr sz="1400"><a:solidFill><a:srgbClr val="767676"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle></p:txBody></p:sp>
	</p:spTree></p:cSld></p:sldLayout>`)

	slide := bodySlide("Grey on grey")
	swaps := enforceInheritedTextContrast(slide, layout, nil, "#767676", theme, 2, nil)
	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1", len(swaps))
	}
	if swaps[0].RatioAfter < 4.5 {
		t.Errorf("ratio after = %.2f, want >= 4.5 even with nothing usable in the palette", swaps[0].RatioAfter)
	}
	if swaps[0].SlideIndex != 2 {
		t.Errorf("swap.SlideIndex = %d, want 2", swaps[0].SlideIndex)
	}
}

// TestInheritedTextThreshold pins that big text is held to the large-text bar
// and body text to the normal one, read out of the inherited fragment.
func TestInheritedTextThreshold(t *testing.T) {
	cases := []struct {
		name     string
		fragment string
		want     float64
	}{
		{"body size", `<a:defRPr sz="1400"/>`, 4.5},
		{"large size", `<a:defRPr sz="2400"/>`, 3.0},
		{"bold at 14pt", `<a:defRPr sz="1400" b="1"/>`, 3.0},
		{"no size stated", `<a:defRPr/>`, 4.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inheritedTextThreshold(tc.fragment); got != tc.want {
				t.Errorf("threshold = %.1f, want %.1f", got, tc.want)
			}
		})
	}
}

// TestPlaceholderMatches pins the layout lookup: idx identifies a placeholder
// when it has one, type when it does not, and a title is spelled either way.
func TestPlaceholderMatches(t *testing.T) {
	idx1, idx2 := 1, 2
	cases := []struct {
		name  string
		shape string
		ph    *placeholderXML
		want  bool
	}{
		{"same idx", `<p:sp><p:ph type="body" idx="1"/></p:sp>`, &placeholderXML{Type: "body", Index: &idx1}, true},
		{"different idx", `<p:sp><p:ph type="body" idx="2"/></p:sp>`, &placeholderXML{Type: "body", Index: &idx1}, false},
		{"idx wins over type", `<p:sp><p:ph type="title" idx="2"/></p:sp>`, &placeholderXML{Type: "body", Index: &idx2}, true},
		{"title spelled ctrTitle", `<p:sp><p:ph type="ctrTitle"/></p:sp>`, &placeholderXML{Type: "title"}, true},
		{"body without idx", `<p:sp><p:ph type="body"/></p:sp>`, &placeholderXML{Type: "body"}, true},
		{"indexed layout body, unindexed slide body", `<p:sp><p:ph type="body" idx="1"/></p:sp>`, &placeholderXML{Type: "body"}, false},
		{"no placeholder at all", `<p:sp><p:nvSpPr/></p:sp>`, &placeholderXML{Type: "body"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := placeholderMatches(tc.shape, tc.ph); got != tc.want {
				t.Errorf("placeholderMatches = %v, want %v", got, tc.want)
			}
		})
	}
}

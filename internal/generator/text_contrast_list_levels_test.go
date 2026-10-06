package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// A section-number placeholder as a template writes it: the level its text
// uses (lvl1) in the accent, and a muted tint on the levels a section number
// never reaches (go-slide-creator-6s2w4).
const (
	sectionNumberLvl1 = `<a:lvl1pPr marL="0" indent="0" algn="r"><a:buNone/><a:defRPr sz="35000" b="0"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>`
	sectionNumberLvl2 = `<a:lvl2pPr marL="457200" indent="0"><a:buNone/><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="tx1"><a:tint val="82000"/></a:schemeClr></a:solidFill></a:defRPr></a:lvl2pPr>`
	sectionNumberLvl3 = `<a:lvl3pPr marL="914400" indent="0"><a:buNone/><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="tx1"><a:tint val="82000"/></a:schemeClr></a:solidFill></a:defRPr></a:lvl3pPr>`
	sectionNumberBg   = "#FFE8D4"
)

func sectionNumberTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#2E353A"}, {Name: "lt2", RGB: "#FFE8D4"},
		{Name: "accent1", RGB: "#FD5108"},
	}
}

func sectionNumberSlide(paragraphs ...paragraphXML) *slideXML {
	return &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
		TextBody: &textBodyXML{
			ListStyle:  &listStyleXML{Inner: sectionNumberLvl1 + sectionNumberLvl2 + sectionNumberLvl3},
			Paragraphs: paragraphs,
		},
	}}}}}
}

func paragraphAtLevel(level *int, text string) paragraphXML {
	p := paragraphXML{Runs: []runXML{{Text: text}}}
	if level != nil {
		p.Properties = &paragraphPropertiesXML{Level: level}
	}
	return p
}

// TestListStyleContrastFixesOnlyUsedLevels: a low-contrast colour on a level no
// paragraph uses is neither rewritten nor reported; the same colour on a level
// a paragraph does use is.
func TestListStyleContrastFixesOnlyUsedLevels(t *testing.T) {
	tests := []struct {
		name       string
		paragraphs []paragraphXML
		// wantSwapped names the levels whose colour must be replaced; every
		// other level must come back byte for byte.
		wantSwapped map[int]bool
	}{
		{name: "no pPr is level 1", paragraphs: []paragraphXML{paragraphAtLevel(nil, "01")}, wantSwapped: map[int]bool{1: true}},
		{name: `lvl="0" is lvl1pPr`, paragraphs: []paragraphXML{paragraphAtLevel(intPtr(0), "01")}, wantSwapped: map[int]bool{1: true}},
		{name: `lvl="1" is lvl2pPr`, paragraphs: []paragraphXML{paragraphAtLevel(intPtr(1), "detail")}, wantSwapped: map[int]bool{2: true}},
		{name: "two levels in use", paragraphs: []paragraphXML{paragraphAtLevel(nil, "01"), paragraphAtLevel(intPtr(2), "deep")}, wantSwapped: map[int]bool{1: true, 3: true}},
		{name: "a blank paragraph uses no level", paragraphs: []paragraphXML{paragraphAtLevel(nil, "01"), paragraphAtLevel(intPtr(1), "  ")}, wantSwapped: map[int]bool{1: true}},
	}
	levels := map[int]string{1: sectionNumberLvl1, 2: sectionNumberLvl2, 3: sectionNumberLvl3}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide := sectionNumberSlide(tt.paragraphs...)
			swaps := enforceTextContrastInSlide(slide, sectionNumberBg, sectionNumberTheme(), 2, nil, false)
			if len(swaps) != len(tt.wantSwapped) {
				t.Fatalf("recorded %d swaps, want %d (one per used level): %+v", len(swaps), len(tt.wantSwapped), swaps)
			}
			for _, swap := range swaps {
				if swap.Source != "lstStyle" || swap.SlideIndex != 2 {
					t.Errorf("swap not attributed to the slide's lstStyle: %+v", swap)
				}
			}
			got := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner
			for level, original := range levels {
				unchanged := strings.Contains(got, original)
				if tt.wantSwapped[level] && unchanged {
					t.Errorf("lvl%dpPr is used and low-contrast but was left as written", level)
				}
				if !tt.wantSwapped[level] && !unchanged {
					t.Errorf("lvl%dpPr is not used by any paragraph but was rewritten:\n got %s\nwant it to contain %s", level, got, original)
				}
			}
		})
	}
}

// Each used level is judged at its own size: the 350pt section number needs
// 3:1, and the 16pt level beside it must not drag it to 4.5:1 when no text
// sits on that level.
func TestListStyleContrastThresholdIgnoresUnusedLevelSizes(t *testing.T) {
	// tx1 at an 82% tint on #FFE8D4 reads about 3.8:1 — fine for large text,
	// short of the body bar.
	muted := `<a:solidFill><a:schemeClr val="tx1"><a:tint val="82000"/></a:schemeClr></a:solidFill>`
	lvl1 := `<a:lvl1pPr><a:defRPr sz="35000">` + muted + `</a:defRPr></a:lvl1pPr>`
	lvl2 := `<a:lvl2pPr><a:defRPr sz="1600">` + muted + `</a:defRPr></a:lvl2pPr>`
	theme := sectionNumberTheme()
	build := func(level *int) *slideXML {
		return &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
			TextBody: &textBodyXML{ListStyle: &listStyleXML{Inner: lvl1 + lvl2}, Paragraphs: []paragraphXML{paragraphAtLevel(level, "01")}},
		}}}}}
	}
	if swaps := enforceTextContrastInSlide(build(nil), sectionNumberBg, theme, 0, nil, false); len(swaps) != 0 {
		t.Errorf("large level-1 text was held to the unused 16pt level's threshold: %+v", swaps)
	}
	if swaps := enforceTextContrastInSlide(build(intPtr(1)), sectionNumberBg, theme, 0, nil, false); len(swaps) != 1 {
		t.Errorf("16pt level-2 text below 4.5:1 recorded %d swaps, want 1: %+v", len(swaps), swaps)
	}
}

// Colours outside the level elements apply to every level, so they stay in
// the pass whatever levels are used; a self-closing level is left alone.
func TestRewriteUsedListLevelsKeepsSharedFragments(t *testing.T) {
	inner := `<a:defPPr><a:defRPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:defPPr>` +
		`<a:lvl1pPr algn="r"/>` + sectionNumberLvl2 + `<a:extLst/>`
	var seen []string
	got := rewriteUsedListLevels(inner, map[int]bool{1: true}, func(fragment string) string {
		seen = append(seen, fragment)
		return fragment
	})
	if got != inner {
		t.Errorf("identity rewrite changed the list style:\n got %s\nwant %s", got, inner)
	}
	want := []string{
		`<a:defPPr><a:defRPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:defPPr>`,
		`<a:lvl1pPr algn="r"/>`,
		`<a:extLst/>`,
	}
	if strings.Join(seen, "|") != strings.Join(want, "|") {
		t.Errorf("fragments passed to the fixer = %q, want %q", seen, want)
	}
}

func TestUsedListLevelsClampsOutOfRangeLevels(t *testing.T) {
	shape := &sectionNumberSlide(paragraphAtLevel(intPtr(-3), "a"), paragraphAtLevel(intPtr(40), "b")).CommonSlideData.ShapeTree.Shapes[0]
	used := usedListLevels(shape)
	if len(used) != 2 || !used[1] || !used[9] {
		t.Errorf("used levels = %v, want {1, 9}", used)
	}
}

// A colour stated only on a level no paragraph uses is not the text's colour:
// the lstStyle pass skips it, so the inherited pass must still resolve and fix
// what the text actually inherits rather than treat the shape as coloured.
func TestInheritedTextContrastSeesPastUnusedLevelColours(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)
	bgHex := extractLayoutBackgroundColor(layout, theme)

	slide := bodySlide("Financial Performance Summary")
	shape := &slide.CommonSlideData.ShapeTree.Shapes[0]
	shape.TextBody.ListStyle.Inner = `<a:lvl2pPr><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl2pPr>`
	if !levelNamesNoColor(shape, 1) {
		t.Fatal("a colour on an unused level made the shape count as naming its text colour")
	}
	if swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), bgHex, theme, 1, override); len(swaps) != 1 {
		t.Errorf("white-on-white level-1 text behind an unused lvl2 colour drew %d swaps, want 1", len(swaps))
	}

	level := 1
	shape = &bodySlide("Detail").CommonSlideData.ShapeTree.Shapes[0]
	shape.TextBody.ListStyle.Inner = `<a:lvl2pPr><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl2pPr>`
	shape.TextBody.Paragraphs[0].Properties = &paragraphPropertiesXML{Level: &level}
	if levelNamesNoColor(shape, 2) {
		t.Error("a colour on the level the paragraph uses is the lstStyle pass's to fix")
	}
}

// masterWithLightSecondLevel styles first-level body text in a colour that
// reads on the inverted section layout (bg1 -> dk1) and second-level text in
// one that does not (tx1 -> lt1, white on white).
const masterWithLightSecondLevel = `<?xml version="1.0"?>
<p:sldMaster xmlns:p="p" xmlns:a="a">
 <p:txStyles>
  <p:bodyStyle><a:lvl1pPr><a:defRPr sz="2000"><a:solidFill><a:schemeClr val="bg1"/></a:solidFill></a:defRPr></a:lvl1pPr><a:lvl2pPr marL="457200"><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl2pPr></p:bodyStyle>
 </p:txStyles>
</p:sldMaster>`

func runFill(run *runXML) string {
	if run.RunProperties == nil {
		return ""
	}
	return run.RunProperties.Inner
}

// The inherited pass resolves each level a paragraph sits on from that level's
// own fragment: a second-level bullet whose master colour is white on white is
// fixed, and the first-level text beside it, which reads, is left as it is.
// The pass used to resolve the first level only and pin every run to one
// colour, so the bullet stayed invisible (go-slide-creator-50xzk).
func TestInheritedTextContrastResolvesEachUsedLevel(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)
	bgHex := extractLayoutBackgroundColor(layout, theme)
	level := 1

	slide := bodySlide("Revenue grew", "driven by renewals")
	shape := &slide.CommonSlideData.ShapeTree.Shapes[0]
	shape.TextBody.Paragraphs[1].Properties = &paragraphPropertiesXML{Level: &level}
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithLightSecondLevel), bgHex, theme, 3, override)
	if len(swaps) != 1 {
		t.Fatalf("got %d swaps, want 1 (the second level): %+v", len(swaps), swaps)
	}
	if swaps[0].OriginalColor != "#FFFFFF" || swaps[0].Source != inheritedSourceMaster || swaps[0].SlideIndex != 3 {
		t.Errorf("swap = %+v, want the master's white second-level colour on slide 3", swaps[0])
	}
	if fill := runFill(&shape.TextBody.Paragraphs[0].Runs[0]); strings.Contains(fill, "<a:solidFill>") {
		t.Errorf("first-level run, whose colour reads, was pinned: %s", fill)
	}
	if fill := runFill(&shape.TextBody.Paragraphs[1].Runs[0]); !strings.Contains(fill, "<a:solidFill>") {
		t.Errorf("second-level run was not given a readable colour: %q", fill)
	}

	// The same deck with no second-level paragraph: nothing uses the failing
	// level, so nothing is changed or reported.
	flat := bodySlide("Revenue grew", "driven by renewals")
	if swaps := enforceInheritedTextContrast(flat, layout, []byte(masterWithLightSecondLevel), bgHex, theme, 3, override); len(swaps) != 0 {
		t.Errorf("an unused failing level drew swaps: %+v", swaps)
	}
}

// A list style that states a colour on one used level leaves the other used
// levels to the layout and master: the lstStyle pass owns the first, and the
// inherited pass still resolves the rest instead of skipping the whole shape.
func TestInheritedTextContrastFixesLevelsTheListStyleLeavesAlone(t *testing.T) {
	theme := modernLikeTheme()
	layout := []byte(invertedSectionLayout)
	override := parseLayoutColorMapOverride(layout)
	bgHex := extractLayoutBackgroundColor(layout, theme)
	level := 1

	slide := bodySlide("Financial Performance Summary", "Detail")
	shape := &slide.CommonSlideData.ShapeTree.Shapes[0]
	shape.TextBody.ListStyle.Inner = `<a:lvl2pPr><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl2pPr>`
	shape.TextBody.Paragraphs[1].Properties = &paragraphPropertiesXML{Level: &level}
	swaps := enforceInheritedTextContrast(slide, layout, []byte(masterWithTx1Body), bgHex, theme, 1, override)
	if len(swaps) != 1 {
		t.Fatalf("white-on-white first-level text beside a coloured second level drew %d swaps, want 1: %+v", len(swaps), swaps)
	}
	if fill := runFill(&shape.TextBody.Paragraphs[1].Runs[0]); strings.Contains(fill, "<a:solidFill>") {
		t.Errorf("the level the list style colours was pinned as well: %s", fill)
	}
}

// A layout list style that styles only a deeper level says nothing about the
// first level's colour; that level's colour is the master's.
func TestLayoutLevelStyleDoesNotLendDeeperLevelsToTheFirst(t *testing.T) {
	layout := []byte(strings.Replace(invertedSectionLayout,
		`<p:txBody><a:lstStyle/></p:txBody></p:sp>`,
		`<p:txBody><a:lstStyle><a:lvl2pPr><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl2pPr></a:lstStyle></p:txBody></p:sp>`, 1))
	idx := 1
	ph := &placeholderXML{Type: "body", Index: &idx}
	if got := layoutPlaceholderLevelStyle(layout, ph, 1); got != "" {
		t.Errorf("level 1 fragment = %q, want none", got)
	}
	if got := layoutPlaceholderLevelStyle(layout, ph, 2); !strings.Contains(got, `val="accent1"`) {
		t.Errorf("level 2 fragment = %q, want the layout's lvl2pPr", got)
	}
	fragment, source := inheritedTextStyleFragment(layout, []byte(masterWithTx1Body), ph, 1)
	if source != inheritedSourceMaster || !strings.Contains(fragment, `val="tx1"`) {
		t.Errorf("level 1 resolved from %q (%s), want the master's first level", source, fragment)
	}
}

// ContentListLevels answers with the levels the population code writes: a
// top-level bullet at the layout's first bulleted level and an indented
// sub-bullet one below it, body text at the first level.
func TestContentListLevels(t *testing.T) {
	tests := []struct {
		name string
		item ContentItem
		base int
		want []int
	}{
		{"flat bullets", ContentItem{PlaceholderID: "body", Type: ContentBullets, Value: []string{"One", "Two"}}, 0, []int{0}},
		{"nested bullets", ContentItem{PlaceholderID: "body", Type: ContentBullets, Value: []string{"One", "  Sub", "Two"}}, 0, []int{0, 1}},
		{"nested bullets on a master whose first level is unmarked", ContentItem{PlaceholderID: "body", Type: ContentBullets, Value: []string{"One", "  Sub"}}, 1, []int{1, 2}},
		{"text", ContentItem{PlaceholderID: "body", Type: ContentText, Value: "A sentence."}, 1, []int{0}},
		{"body and nested bullets", ContentItem{PlaceholderID: "body", Type: ContentBodyAndBullets,
			Value: BodyAndBulletsContent{Body: "Lead", Bullets: []string{"One", "  Sub"}}}, 1, []int{0, 1, 2}},
		{"no text", ContentItem{PlaceholderID: "body", Type: ContentText, Value: ""}, 0, nil},
		{"not text content", ContentItem{PlaceholderID: "body", Type: ContentImage}, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentListLevels(tt.item, tt.base)
			if len(got) != len(tt.want) {
				t.Fatalf("levels = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("levels = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

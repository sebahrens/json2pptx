package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// s7wmhTheme is a template whose title colour is a deep navy — not a neutral,
// so the old code lerped it. midnight-blue's real values.
var s7wmhTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"},
	{Name: "dk2", RGB: "#1B2A4A"},
	{Name: "lt1", RGB: "#FFFFFF"},
	{Name: "lt2", RGB: "#F5F7FA"},
	{Name: "accent1", RGB: "#2E5090"},
}

// titleSlideXML is one title shape carrying the scheme colour the generator
// copies onto the slide from the layout's title placeholder.
func titleSlideXML(scheme string) *slideXML {
	return &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
					},
					TextBody: &textBodyXML{
						Paragraphs: []paragraphXML{{
							Runs: []runXML{{
								RunProperties: &runPropertiesXML{
									Inner: `<a:solidFill><a:schemeClr val="` + scheme + `"/></a:solidFill>`,
								},
								Text: "Where we stand",
							}},
						}},
					},
				}},
			},
		},
	}
}

// TestAuthorBackgroundSnapsToPalette is the bead's headline case: a slide that
// sets background.color dk1 under a template title drawn in dk2 must end up
// with the template's own light text, not a mid-tone that merely clears the
// WCAG floor. Before go-slide-creator-s7wmh the navy was lerped to #4E5A72 —
// ratio exactly 3.0 and barely visible on black.
func TestAuthorBackgroundSnapsToPalette(t *testing.T) {
	slide := titleSlideXML("dk2")
	swaps := enforceTextContrastInSlide(slide, "#000000", s7wmhTheme, 0, nil, true)
	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d (%v)", len(swaps), swaps)
	}
	if swaps[0].ReplacedColor != "#FFFFFF" {
		t.Errorf("replacement = %s, want the template's lt1 #FFFFFF", swaps[0].ReplacedColor)
	}
	if swaps[0].RatioAfter < 7 {
		t.Errorf("ratio after = %.2f, want a palette colour well clear of the floor (>= 7)", swaps[0].RatioAfter)
	}
}

// TestLayoutBackgroundStillLerps pins the other half of the rule: when the
// background is the TEMPLATE's, the text colour was chosen against it and the
// hue is intentional, so the fix stays a lerp toward the WCAG floor. A deck
// whose background comes from its layout must not change at all.
func TestLayoutBackgroundStillLerps(t *testing.T) {
	slide := titleSlideXML("dk2")
	swaps := enforceTextContrastInSlide(slide, "#000000", s7wmhTheme, 0, nil, false)
	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d (%v)", len(swaps), swaps)
	}
	if swaps[0].ReplacedColor == "#FFFFFF" {
		t.Errorf("layout background snapped to %s; only an author background may snap", swaps[0].ReplacedColor)
	}
	// The lerp stops as soon as it clears the floor for this text size; a
	// palette snap would land far above it (lt1 on black is 21:1).
	if swaps[0].RatioAfter > svggen.WCAGAANormal+0.5 {
		t.Errorf("ratio after = %.2f, want the lerp's minimal lift just past %.1f, not a palette jump",
			swaps[0].RatioAfter, svggen.WCAGAANormal)
	}
}

// TestShapeGridCellKeepsLerp guards the exemption: a cell's fill and its text
// colour were authored together, so the author's hue intent survives even
// though the fill is, literally, an author-introduced background.
func TestShapeGridCellKeepsLerp(t *testing.T) {
	fg := svggen.MustParseColor("#1B2A4A")
	bg := svggen.MustParseColor("#000000")

	cell, mode, _ := contrastReplacementScheme("#1B2A4A", fg, bg, s7wmhTheme, svggen.WCAGAANormal, false)
	if mode != contrastModeLerp {
		t.Errorf("shape_grid cell mode = %q, want %q", mode, contrastModeLerp)
	}
	if cell.Hex() == "#FFFFFF" {
		t.Error("shape_grid cell snapped to lt1; the cell's authored hue must be preserved")
	}

	authored, mode, scheme := contrastReplacementScheme("#1B2A4A", fg, bg, s7wmhTheme, svggen.WCAGAANormal, true)
	if mode != contrastModeFlip || scheme != "lt1" {
		t.Errorf("author background mode = %q scheme = %q, want %q / lt1", mode, scheme, contrastModeFlip)
	}
	if authored.Hex() != "#FFFFFF" {
		t.Errorf("author background replacement = %s, want #FFFFFF", authored.Hex())
	}
}

// TestAuthorBackgroundPreflightMatchesRender is the validate/generate parity
// the bead requires: the contrast_predicted finding validate emits must name
// the same colour the render-time swap writes.
func TestAuthorBackgroundPreflightMatchesRender(t *testing.T) {
	slide := titleSlideXML("dk2")
	swaps := enforceTextContrastInSlide(slide, "#000000", s7wmhTheme, 0, nil, true)
	if len(swaps) != 1 {
		t.Fatalf("expected 1 render-time swap, got %d", len(swaps))
	}

	findings := DetectContrastPreflight([]ContrastPreflightPair{{
		Path:             "/slides/0/content/title",
		Foreground:       "dk2",
		Background:       "dk1",
		Source:           "slide_background",
		TextPt:           60,
		AuthorBackground: true,
	}}, s7wmhTheme)
	if len(findings) != 1 {
		t.Fatalf("expected 1 preflight finding, got %d", len(findings))
	}
	if predicted := findings[0].Fix.Params["predicted_replacement"]; predicted != swaps[0].ReplacedColor {
		t.Errorf("predicted %v != render-time swap %q", predicted, swaps[0].ReplacedColor)
	}
	if mode := findings[0].Fix.Params["replacement_mode"]; mode != contrastModeFlip {
		t.Errorf("replacement_mode = %v, want %q", mode, contrastModeFlip)
	}
}

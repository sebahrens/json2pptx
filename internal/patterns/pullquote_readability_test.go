package patterns

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// The quote row was capped at 80% of the area and a long quote was left to the
// writer's autofit, so a 500-character quote was stored at 36pt with a ~45%
// fontScale — a shrink PowerPoint and LibreOffice each apply their own way —
// and the attribution row was pinned by the theme-font model alone. Both rows
// now hold the written fit of their text: the default quote size steps down
// (36 → 20pt) until the quote is written unshrunk, and only a quote that
// overflows even at 20pt is left to autofit, which the readability floor still
// polices (go-slide-creator-n1muf).
func TestPullQuoteRowsHoldWrittenFit(t *testing.T) {
	p := &pullQuote{}
	for _, font := range []string{"", "Tenorite", "Segoe UI"} {
		for _, a := range writtenFloorAreas {
			for chars := 50; chars <= 500; chars += 50 {
				for _, attr := range []int{10, 60} {
					for _, photo := range []bool{false, true} {
						name := fmt.Sprintf("%q/%s/quote%d/attr%d/photo=%v", font, a.name, chars, attr, photo)
						t.Run(name, func(t *testing.T) {
							v := &PullQuoteValues{
								Quote:       wordsOfLength("invention matters", chars),
								Attribution: wordsOfLength("Alexander", attr),
								Role:        wordsOfLength("Chief", attr),
							}
							var ovr any
							if photo {
								v.Image = &jsonschema.GridImageInput{Path: "headshot.png"}
								ovr = &PullQuoteOverrides{ImageWidthPct: pullQuoteImageMaxPct, AttrSize: 20}
							}
							ctx := ExpandContext{}
							ctx.Theme.BodyFont = font
							below, warned := patternWrittenBelowFloor(t, p, ctx, a.w, a.h, v, ovr)
							if len(below) > 0 && len(warned) == 0 {
								t.Errorf("written below the floor without BODY_TOO_LONG: %v", below)
							}
							ctx.LayoutBounds = LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}
							grid, err := p.Expand(ctx, v, ovr, nil)
							if err != nil {
								t.Fatal(err)
							}
							ApplyGridDefaults(grid)
							res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
							for _, c := range res.Cells {
								if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
									continue
								}
								tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
								if err != nil || tb == nil {
									continue
								}
								scale := writtenScaleIn(ctx, tb, c.Bounds)
								if scale < 1 && largestRunPt(tb) > pullQuoteMinQuotePt {
									t.Errorf("%q at %.0fpt is written at %.0f%% in a %.0f×%.0fpt row",
										firstText(tb), largestRunPt(tb), scale*100, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700)
								}
							}
						})
					}
				}
			}
		}
	}
}

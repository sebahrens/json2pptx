package patterns

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// An option matrix with four criteria rendered "Cost per tonne" and "Energy
// use" visibly larger than "Deployment speed" and "Commercial traction"
// (go-slide-creator-fr538). The grid's type scale grows sparse cell text one
// cell at a time: a short label had room to grow from 12pt to 14pt, while a
// label whose longest word would no longer fit its column kept 12pt. Header
// cells opt out of that growth, so the header row is written at the one header
// size whatever the labels are.
func TestTableHighlight_HeadersShareOneSize(t *testing.T) {
	labels := []string{"Cost per tonne", "Energy use", "Deployment speed", "Commercial traction", "Regulatory readiness", "Customer impact"}
	pat := &tableHighlight{}
	for _, area := range writtenFitAreas {
		for _, mode := range []string{"", "comfortable", "presentation"} {
			for nCrit := 2; nCrit <= 6; nCrit++ {
				t.Run(fmt.Sprintf("%s/%s/%d criteria", area.name, mode, nCrit), func(t *testing.T) {
					v := &TableHighlightValues{Scale: "harvey"}
					for _, label := range labels[:nCrit] {
						v.Criteria = append(v.Criteria, TableHighlightCriterion{Label: label})
					}
					for _, name := range []string{"Parcel automation", "Hub consolidation", "Partner network"} {
						o := TableHighlightOption{Name: name, Detail: "Close two hubs, expand one"}
						for j := 0; j < nCrit; j++ {
							o.Scores = append(o.Scores, "3")
						}
						v.Options = append(v.Options, o)
					}
					ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(area.w * 12700), Height: int64(area.h * 12700)}}
					ctx.Theme.BodyFont = area.font
					grid, err := pat.Expand(ctx, v, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					ApplyGridDefaults(grid)
					// The semantic compiler sets the deck's type scale on the grid.
					grid.TypeScale = mode
					res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})

					sizes := map[int][]string{}
					headers := 0
					for _, c := range res.Cells {
						if c.RowIdx != 0 || c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
							continue
						}
						tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
						if err != nil || tb == nil || len(tb.Paragraphs) == 0 || len(tb.Paragraphs[0].Runs) == 0 {
							t.Fatalf("header cell text: %v", err)
						}
						run := tb.Paragraphs[0].Runs[0]
						// The size a reader sees: the written size times the
						// row's shared autofit shrink.
						scale := c.AutofitScale
						if scale == 0 {
							tb.ThemeFonts = ctx.themeFonts()
							for j := range tb.Insets {
								tb.Insets[j] += c.TextInsets[j]
							}
							scale = pptx.AutofitScaleFor(tb, c.Bounds)
						}
						shown := int(float64(run.FontSize)*scale + 0.5)
						sizes[shown] = append(sizes[shown], run.Text)
						headers++
					}
					if headers != nCrit+1 {
						t.Fatalf("found %d header cells, want %d", headers, nCrit+1)
					}
					if len(sizes) != 1 {
						t.Errorf("header labels render at %d sizes (hundredths of a point → labels): %v", len(sizes), sizes)
					}
				})
			}
		}
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Density findings on slides whose content is a pattern nested in a grid
// cell, or a ring / hub figure (go-slide-creator-2ym44).

// densityCodes are the findings that say a slide is mostly empty.
var densityCodes = map[string]bool{
	patterns.ErrCodeSparseLayout:   true,
	patterns.ErrCodeSlideUnderused: true,
}

func densityTemplates(t *testing.T) []string {
	t.Helper()
	if testing.Short() {
		return []string{"business-template", "midnight-blue"}
	}
	return testutil.AllTestTemplateNames()
}

func densitySlide(t *testing.T, body string) SlideInput {
	t.Helper()
	var slide SlideInput
	if err := json.Unmarshal([]byte(`{"slide_type":"content","layout_id":"blank-title","content":[{"placeholder_id":"title","type":"text","text_value":"One governed platform replaces six private copies"}],`+body+`}`), &slide); err != nil {
		t.Fatal(err)
	}
	return slide
}

// densityFindings returns the slide's density findings and its left-edge
// grid_violation, if any.
func densityFindings(t *testing.T, tpl string, slide SlideInput) (density []patterns.FitFinding, leftEdge *patterns.FitFinding) {
	t.Helper()
	a := loadTemplateAnalysis(t, tpl)
	deck := PresentationInput{Template: tpl, Slides: []SlideInput{slide}}
	for _, f := range collectFitFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		switch {
		case densityCodes[f.Code]:
			density = append(density, f)
		case f.Code == "grid_violation" && strings.Contains(f.Message, "content left"):
			leftEdge = &f
		}
	}
	return density, leftEdge
}

const (
	densityRingValues  = `{"phases":[{"label":"Forecast"},{"label":"Review results"},{"label":"Decide"},{"label":"Commit funding"},{"label":"Execute"},{"label":"Track benefits"}]}`
	densityCardsValues = `{"columns":2,"rows":2,"cells":[{"header":"Billing","body":"Revenue assurance and the month-end close for every region"},{"header":"Marketing","body":"Audiences and campaign attribution across channels"},{"header":"Service","body":"The full history of a customer at every contact"},{"header":"Sales","body":"Account insight and the next best action per account"}]}`
	densityHubValues   = `{"center":{"label":"Customer platform"},"spokes":[{"label":"Marketing"},{"label":"Sales"},{"label":"Service"},{"label":"Finance"},{"label":"Risk"},{"label":"Product"}]}`
	densityRowsValues  = `{"rows":[{"label":"WHY","body":"Each function keeps its own copy of the customer today."},{"label":"WHAT","body":"One governed platform that every function reads from."},{"label":"HOW","body":"Connect risk and compliance first, then sales."}]}`
	densityCardCell    = `{"shape":{"geometry":"rect","fill":"lt2","text":{"content":"One governed platform that every function reads from, reviewed each month by the leadership team and funded from the savings of the six copies it retires."}}}`
	densityNoteCell    = `{"shape":{"geometry":"rect","fill":"none","text":{"content":"One governed platform."}}}`
)

// A pattern nested in a grid cell is content. Unexpanded, its cell read as
// empty: a slide half filled by a ring or by four cards reported "visible
// content covers 3 percent of grid area", SLIDE_UNDERUSED at 5 percent and a
// content left edge half a slide in. The ring is one of the circular family;
// card-grid shows the fault was never theirs.
func TestNestedCellPatternCountsAsContentAcrossTemplates(t *testing.T) {
	left := map[string]string{
		"cycle-ring": `{"pattern":{"name":"cycle-ring","values":` + densityRingValues + `}}`,
		"card-grid":  `{"pattern":{"name":"card-grid","values":` + densityCardsValues + `}}`,
		"radial-hub": `{"pattern":{"name":"radial-hub","values":` + densityHubValues + `}}`,
	}
	grid := func(cells ...string) string {
		return `"shape_grid":{"columns":[50,50],"rows":[{"cells":[` + strings.Join(cells, ",") + `]}]}`
	}
	rows := `{"pattern":{"name":"labeled-rows","values":` + densityRowsValues + `}}`
	for _, tpl := range densityTemplates(t) {
		for name, cell := range left {
			// Beside an authored card and beside a second nested pattern the
			// slide is full.
			for _, right := range []struct{ name, cell string }{{"card", densityCardCell}, {"nested rows", rows}} {
				t.Run(fmt.Sprintf("%s/%s beside %s", tpl, name, right.name), func(t *testing.T) {
					density, leftEdge := densityFindings(t, tpl, densitySlide(t, grid(cell, right.cell)))
					for _, f := range density {
						// Where the block sits is another rule under the same
						// code (slideLowerBandMaxFrac): content-sized cards
						// beside content-sized rows do end above the lower
						// third of p-style's tall content area.
						if strings.Contains(f.Message, "the lower ") {
							continue
						}
						t.Errorf("%s: %s", f.Code, f.Message)
					}
					if leftEdge != nil {
						t.Errorf("grid_violation: %s", leftEdge.Message)
					}
				})
			}
			// Beside three words the slide is sparse and says so, with the
			// pattern counted: well over the 3–5 percent of an empty cell.
			t.Run(fmt.Sprintf("%s/%s beside a note", tpl, name), func(t *testing.T) {
				density, leftEdge := densityFindings(t, tpl, densitySlide(t, grid(cell, densityNoteCell)))
				if leftEdge != nil {
					t.Errorf("grid_violation: %s", leftEdge.Message)
				}
				var sparse *patterns.FitFinding
				for i, f := range density {
					if f.Code == patterns.ErrCodeSparseLayout {
						sparse = &density[i]
					}
				}
				if sparse == nil {
					t.Fatalf("a pattern beside three words must still report sparse_layout; got %v", density)
				}
				if sparse.OverflowRatio < 0.12 {
					t.Errorf("sparse_layout measured %.0f%% of the grid: the nested %s was not counted", 100*sparse.OverflowRatio, name)
				}
			})
		}
	}
}

// A ring or a hub beside a text column fills its slide: the figure is the
// size of its circle and its labels stand in rows beside it. Counted disc by
// disc and glyph by glyph these slides read 23–37 percent and were called
// mostly empty.
func TestRingBesideTextIsNotUnderusedAcrossTemplates(t *testing.T) {
	segment := func(pct int, name, values string) string {
		return fmt.Sprintf(`{"size_pct":%d,"pattern":{"name":%q,"values":%s}}`, pct, name, values)
	}
	rows := func(pct int) string { return segment(pct, "labeled-rows", densityRowsValues) }
	nodes := `{"steps":[{"label":"Pick one loss"},{"label":"Trial the fix"},{"label":"Measure the gap"},{"label":"Set the standard"},{"label":"Share it"}]}`
	intake := `{"intake":[{"label":"Qualify"},{"label":"Onboard"}],"phases":[{"label":"Activate"},{"label":"Engage"},{"label":"Renew"},{"label":"Expand"},{"label":"Refer"}]}`
	layers := `{"layers":[{"label":"Core business"},{"label":"Adjacent markets"},{"label":"Partner ecosystem"},{"label":"New ventures"}]}`
	cases := map[string]string{
		"radial-hub 60":       `"compose":{"direction":"horizontal","segments":[` + segment(60, "radial-hub", densityHubValues) + `,` + rows(40) + `]}`,
		"radial-hub 50":       `"compose":{"direction":"horizontal","segments":[` + segment(50, "radial-hub", densityHubValues) + `,` + rows(50) + `]}`,
		"cycle-ring 50":       `"compose":{"direction":"horizontal","segments":[` + segment(50, "cycle-ring", densityRingValues) + `,` + rows(50) + `]}`,
		"cycle-nodes 60":      `"compose":{"direction":"horizontal","segments":[` + segment(60, "cycle-nodes", nodes) + `,` + rows(40) + `]}`,
		"cycle-intake 60":     `"compose":{"direction":"horizontal","segments":[` + rows(40) + `,` + segment(60, "cycle-intake", intake) + `]}`,
		"concentric-rings 50": `"compose":{"direction":"horizontal","segments":[` + segment(50, "concentric-rings", layers) + `,` + rows(50) + `]}`,
		"radial-hub alone":    `"pattern":{"name":"radial-hub","values":` + densityHubValues + `}`,
		"radial-hub inside":   `"pattern":{"name":"radial-hub","overrides":{"labels":"inside"},"values":` + densityHubValues + `}`,
	}
	for _, tpl := range densityTemplates(t) {
		for name, body := range cases {
			t.Run(tpl+"/"+name, func(t *testing.T) {
				density, _ := densityFindings(t, tpl, densitySlide(t, body))
				for _, f := range density {
					t.Errorf("%s: %s", f.Code, f.Message)
				}
			})
		}
	}
}

// The showcase decks of the family report no slide of theirs as mostly empty.
func TestCircularExamplesHaveNoDensityFindingsAcrossTemplates(t *testing.T) {
	for _, file := range []string{"radial-hub.json", "circular-layouts.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "examples", file))
		if err != nil {
			t.Fatal(err)
		}
		for _, tpl := range densityTemplates(t) {
			t.Run(file+"/"+tpl, func(t *testing.T) {
				var deck PresentationInput
				if err := json.Unmarshal(raw, &deck); err != nil {
					t.Fatal(err)
				}
				deck.Template = tpl
				a := loadTemplateAnalysis(t, tpl)
				for _, f := range collectFitFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
					if densityCodes[f.Code] {
						t.Errorf("%s %s: %s", f.Path, f.Code, f.Message)
					}
				}
			})
		}
	}
}

// The figure counts as large as it is drawn. A ring capped to a strip of the
// content area, and a small hub beside one figure, are still small blocks on
// a mostly empty slide.
func TestSmallRingAloneStillReportsUnderused(t *testing.T) {
	steps := `{"steps":[{"label":"Plan"},{"label":"Do"},{"label":"Check"},{"label":"Act"}]}`
	phases := `{"phases":[{"label":"Plan"},{"label":"Do"},{"label":"Check"},{"label":"Act"}]}`
	hub := `{"center":{"label":"Core"},"spokes":[{"label":"Plan"},{"label":"Do"},{"label":"Check"},{"label":"Act"}]}`
	cases := map[string]string{
		"cycle-nodes capped": `"pattern":{"name":"cycle-nodes","max_height_pct":45,"values":` + steps + `}`,
		"cycle-ring bounds":  `"pattern":{"name":"cycle-ring","bounds":{"x":10,"y":30,"width":80,"height":30},"values":` + phases + `}`,
		"radial-hub capped":  `"pattern":{"name":"radial-hub","max_height_pct":45,"overrides":{"labels":"inside"},"values":` + hub + `}`,
		"hub beside a figure": `"compose":{"direction":"horizontal","segments":[{"size_pct":30,"pattern":{"name":"radial-hub","overrides":{"labels":"inside"},"values":` + hub + `}},` +
			`{"size_pct":70,"pattern":{"name":"stat-hero","values":{"value":"4","label":"steps"}}}]}`,
	}
	for _, tpl := range []string{"business-template", "midnight-blue"} {
		for name, body := range cases {
			t.Run(tpl+"/"+name, func(t *testing.T) {
				density, _ := densityFindings(t, tpl, densitySlide(t, body))
				for _, f := range density {
					if f.Code == patterns.ErrCodeSlideUnderused {
						return
					}
				}
				t.Errorf("a small figure on a mostly empty slide must report SLIDE_UNDERUSED; got %v", density)
			})
		}
	}
}

// figureCellBounds joins the layers of each cell and leaves plain cells out.
func TestFigureCellBounds(t *testing.T) {
	layer := func(row, col int, x, y, cx, cy int64) shapegrid.ResolvedCell {
		return shapegrid.ResolvedCell{Kind: shapegrid.CellKindShape, Layer: true, RowIdx: row, ColIdx: col, Bounds: pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy}}
	}
	got := figureCellBounds([]shapegrid.ResolvedCell{
		layer(0, 0, 100, 100, 50, 50),
		{Kind: shapegrid.CellKindShape, RowIdx: 0, ColIdx: 1, Bounds: pptx.RectEmu{X: 900, Y: 0, CX: 100, CY: 100}},
		layer(0, 0, 300, 250, 50, 50),
		layer(1, 0, 0, 500, 40, 40),
		layer(0, 0, 200, 0, 0, 0),
	})
	want := []pptx.RectEmu{{X: 100, Y: 100, CX: 250, CY: 200}, {X: 0, Y: 500, CX: 40, CY: 40}}
	if len(got) != len(want) {
		t.Fatalf("figureCellBounds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("figure %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

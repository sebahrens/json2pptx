package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// sparseExhibit is a pattern with typical short content alone on a slide and
// what the slide must then show: the largest and the smallest text size (the
// attribution rule and other one-glyph marks aside) and the least share of
// the content area's height the block covers.
type sparseExhibit struct {
	name, pattern, values string
	headPt, bodyPt        float64
	minShare              float64
}

// sparseExhibits are the three patterns the shared zoom first gained nothing
// on (go-slide-creator-cyyiy): the open card-grid, whose one-line body at the
// stepped size came within a renderer's slack of its column and needed a
// second line the grown row did not hold; quote-cluster, whose one-paragraph
// cell held the type at 14pt; and agenda, which already sets itself at 18pt.
var sparseExhibits = []sparseExhibit{
	{
		name: "card-grid of two", pattern: "card-grid",
		values: `{"cells":[{"header":"Starter","body":"For individuals and small teams getting started"},{"header":"Professional","body":"For growing teams that need more power"}]}`,
		headPt: 24, bodyPt: 18, minShare: 0.27,
	},
	{
		name: "card-grid of four", pattern: "card-grid",
		values: `{"cells":[{"header":"Starter","body":"For individuals and small teams getting started"},{"header":"Professional","body":"For growing teams that need more power"},{"header":"Enterprise","body":"For large organizations with custom needs"},{"header":"API Access","body":"Programmatic access to all platform features"}]}`,
		headPt: 24, bodyPt: 18, minShare: 0.65,
	},
	{
		name: "quote-cluster of three", pattern: "quote-cluster",
		values: `{"quotes":[{"text":"The new platform cut our cycle time in half.","name":"J. Lin","title":"Head of Operations"},{"text":"Our analysts finally trust the numbers they see.","name":"P. Reyes","title":"Director of Finance"},{"text":"Adoption was easier than any tool we have rolled out.","name":"K. Müller","title":"CTO"}]}`,
		headPt: 18, bodyPt: 14, minShare: 0.5,
	},
	{
		name: "agenda of four", pattern: "agenda",
		values: `{"items":["Market context","Options assessed","Recommended path","Decisions and next steps"]}`,
		headPt: 18, bodyPt: 18, minShare: 0.65,
	},
}

// A sparse open exhibit alone on a slide is set in confident type and uses
// the body: the engine zoom (shapegrid.Grid.ComposeZoom) takes its type step
// on each of these, on every template.
func TestSparseExhibitsTakeTheZoomTypeStepAcrossTemplates(t *testing.T) {
	templates := []string{"midnight-blue", "p-style", "abstract", "warm-coral"}
	if testing.Short() {
		templates = templates[:2]
	}
	for _, tpl := range templates {
		if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		a := loadTemplateAnalysis(t, tpl)
		for _, ex := range sparseExhibits {
			slide := titledPatternSlide(ex.pattern, json.RawMessage(ex.values))
			grid := expandSlidePatternGrid(&slide, 0, a.SlideWidth, a.SlideHeight, &a.Theme)
			if grid == nil {
				t.Fatalf("%s on %s: the pattern did not expand", ex.name, tpl)
			}
			slide.ShapeGrid = grid
			geom := resolveGridGeometry(slide, a.Layouts, a.SlideWidth, a.SlideHeight)
			res := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)
			if res == nil {
				t.Fatalf("%s on %s: the grid did not resolve", ex.name, tpl)
			}
			area := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)
			largest, smallest := 0.0, math.Inf(1)
			top, bottom := int64(math.MaxInt64), int64(0)
			for _, cell := range res.Cells {
				if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil {
					continue
				}
				for _, pt := range sparseExhibitWordSizes(cell.ShapeSpec.Text) {
					largest, smallest = math.Max(largest, pt), math.Min(smallest, pt)
					if cell.Bounds.Y < top {
						top = cell.Bounds.Y
					}
					if end := cell.Bounds.Y + cell.Bounds.CY; end > bottom {
						bottom = end
					}
				}
			}
			if largest != ex.headPt || smallest != ex.bodyPt {
				t.Errorf("%s on %s: type is %v / %vpt, want %v / %vpt", ex.name, tpl, largest, smallest, ex.headPt, ex.bodyPt)
			}
			if share := float64(bottom-top) / float64(area.CY); share < ex.minShare {
				t.Errorf("%s on %s: the block covers %.0f%% of the content area's height, want at least %.0f%%", ex.name, tpl, 100*share, 100*ex.minShare)
			}
		}
	}
}

// sparseExhibitWordSizes lists the sizes of a shape's worded paragraphs: a
// paragraph without a letter or digit (a quote mark, a rule) and a numeral in
// the heading face are display marks, not the type the slide is read in.
func sparseExhibitWordSizes(raw json.RawMessage) []float64 {
	var obj struct {
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
			Font    string  `json:"font"`
		} `json:"paragraphs"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	var out []float64
	for _, p := range obj.Paragraphs {
		worded := false
		for _, r := range p.Content {
			worded = worded || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		}
		if worded && p.Font == "" && p.Size > 0 {
			out = append(out, p.Size)
		}
	}
	return out
}

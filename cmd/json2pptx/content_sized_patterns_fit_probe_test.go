//go:build probe

package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestContentSizedPatterns_DebugDeck prints every fit finding for the slides
// of the deck named by J2P_FIT_DECK (a render-review aid; skipped otherwise).
func TestContentSizedPatterns_DebugDeck(t *testing.T) {
	path := os.Getenv("J2P_FIT_DECK")
	if path == "" {
		t.Skip("set J2P_FIT_DECK=<deck.json> to print fit findings for a deck")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var in PresentationInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	a := loadTemplateAnalysis(t, in.Template)
	for i, s := range in.Slides {
		if s.Pattern != nil && os.Getenv("J2P_FIT_GEOM") != "" {
			ctx := patterns.ExpandContext{Theme: a.Theme, SlideWidth: a.SlideWidth, SlideHeight: a.SlideHeight}
			grid, _, err := expandPattern(s.Pattern, ctx, patterns.Default())
			if err == nil {
				s2 := s
				s2.ShapeGrid = grid
				geom := resolveGridGeometry(s2, a.Layouts, a.SlideWidth, a.SlideHeight)
				if res := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight); res != nil {
					for _, c := range res.Cells {
						t.Logf("slide %d cell r%d c%d: %.1f x %.1f pt at y=%.1f", i+1, c.RowIdx, c.ColIdx, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700, float64(c.Bounds.Y)/12700)
					}
				}
			}
		}
		for _, f := range fitFindingsForSlide(t, a, s) {
			t.Logf("slide %d: %s %s — %s", i+1, f.Code, f.Path, f.Message)
		}
	}
}

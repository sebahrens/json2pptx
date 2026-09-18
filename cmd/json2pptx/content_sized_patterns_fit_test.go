package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// contentSizedPatterns are the patterns that size their own rows from their
// text (go-slide-creator-xyph / e53n / ycbn / pzrs). Their exemplar values
// must render without the geometry / readability findings that flag stretched,
// mostly-empty or unreadable layouts.
var contentSizedPatterns = []string{"exec-summary"}

// forbiddenExemplarFindings are the codes a content-sized pattern must not
// emit on its own exemplar values.
var forbiddenExemplarFindings = map[string]bool{
	patterns.ErrCodeSparseFill:           true,
	patterns.ErrCodeSlideUnderused:       true,
	patterns.ErrCodeTextExceedsShape:     true,
	patterns.ErrCodeTextBelowReadableMin: true,
}

// fitFindingsForSlide runs the full fit-finding collector twice: once on the
// pattern slide (geometry detectors expand the pattern themselves) and once on
// the pre-expanded shape_grid (the form generation renders, which the
// readability detector measures).
func fitFindingsForSlide(t *testing.T, a *types.TemplateAnalysis, slide SlideInput) []patterns.FitFinding {
	t.Helper()
	in := PresentationInput{Slides: []SlideInput{slide}}
	findings := collectFitFindings(&in, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme)

	if slide.Pattern != nil {
		ctx := patterns.ExpandContext{Theme: a.Theme, SlideWidth: a.SlideWidth, SlideHeight: a.SlideHeight}
		grid, _, err := expandPattern(slide.Pattern, ctx, patterns.Default())
		if err != nil {
			t.Fatalf("expand %s: %v", slide.Pattern.Name, err)
		}
		expanded := slide
		expanded.Pattern = nil
		expanded.ShapeGrid = grid
		in2 := PresentationInput{Slides: []SlideInput{expanded}}
		findings = append(findings, collectFitFindings(&in2, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme)...)
	}
	return findings
}

func titledPatternSlide(name string, values json.RawMessage) SlideInput {
	title := "Content-sized pattern probe"
	return SlideInput{
		LayoutID: "content",
		Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		Pattern:  &PatternInput{Name: name, Values: values},
	}
}

func TestContentSizedPatterns_ExemplarHasNoLayoutFindings(t *testing.T) {
	for _, tpl := range []string{"midnight-blue", "warm-coral"} {
		a := loadTemplateAnalysis(t, tpl)
		for _, name := range contentSizedPatterns {
			p, ok := patterns.Default().Get(name)
			if !ok {
				t.Fatalf("pattern %q not registered", name)
			}
			ex, ok := p.(patterns.Exemplar)
			if !ok {
				t.Fatalf("pattern %q has no exemplar values", name)
			}
			values, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				t.Fatalf("marshal exemplar %s: %v", name, err)
			}
			t.Run(name+"/"+tpl, func(t *testing.T) {
				for _, f := range fitFindingsForSlide(t, a, titledPatternSlide(name, values)) {
					if forbiddenExemplarFindings[f.Code] {
						t.Errorf("%s: %s at %s: %s", name, f.Code, f.Path, f.Message)
					}
				}
			})
		}
	}
}

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

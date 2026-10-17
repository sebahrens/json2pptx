package main

import (
	"encoding/json"

	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// contentSizedPatterns are the patterns that size their own rows from their
// text (go-slide-creator-xyph / e53n / ycbn / pzrs). Their exemplar values
// must render without the geometry / readability findings that flag stretched,
// mostly-empty or unreadable layouts.
var contentSizedPatterns = []string{"exec-summary", "table-highlight", "image-text-split", "chart-insights-split"}

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

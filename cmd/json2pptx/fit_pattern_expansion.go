// fit_pattern_expansion.go lets the text-capacity and readability detectors see
// pattern slides (go-slide-creator-adur).
//
// A slide-level named pattern only gets a ShapeGrid at generation time, and both
// generateFitReport and collectReadabilityFindings walked `slide.ShapeGrid`
// directly — so every text-density, autofit and readability check silently
// skipped the surface the skill tells agents to author through. The geometry
// detectors already expanded patterns (expandSlidePatternGrid), which is why
// TEXT_EXCEEDS_SHAPE and SPARSE_FILL fired on pattern slides while
// fit_overflow, cell_underfilled and TEXT_BELOW_READABLE_MIN never did: the same
// content authored as a raw shape_grid produced four error-severity findings and
// as a pattern produced none, on a deck whose body text renders at ~8pt.
package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// expandPatternsForFit returns a working copy of input in which every
// slide-level pattern (including its nested cell patterns) has been expanded
// into ShapeGrid, plus the set of slide indices that were expanded so their
// finding paths can be rerooted at /slides/N/pattern.
//
// input is returned unchanged when no slide needs expansion.
func expandPatternsForFit(input *PresentationInput, slideWidth, slideHeight int64, theme *types.ThemeInfo, layoutSets ...types.LayoutMetadata) (*PresentationInput, map[int]bool) {
	if input == nil {
		return nil, nil
	}
	needs := false
	for i := range input.Slides {
		if input.Slides[i].Pattern != nil && input.Slides[i].ShapeGrid == nil {
			needs = true
			break
		}
	}
	if !needs {
		return input, nil
	}

	expanded := *input
	expanded.Slides = make([]SlideInput, len(input.Slides))
	copy(expanded.Slides, input.Slides)

	fromPattern := make(map[int]bool)
	for i := range expanded.Slides {
		s := &expanded.Slides[i]
		if s.Pattern == nil || s.ShapeGrid != nil {
			continue
		}
		var bounds patterns.LayoutBounds
		if len(layoutSets) > 0 {
			var rhythm *resolvedGrid
			if input.Grid != nil && validateGridConfig(input.Grid) == nil {
				rhythm = resolveGrid(input.Grid, layoutSets, slideWidth, slideHeight)
			}
			_, b := patternExpansionGeometry(*s, layoutSets, slideWidth, slideHeight, rhythm)
			bounds = patterns.LayoutBounds{X: b.X, Y: b.Y, Width: b.CX, Height: b.CY}
		}
		grid, _ := expandSlidePatternGridWithWarningsAtBounds(s, i, slideWidth, slideHeight, theme, bounds)
		if grid == nil {
			// Invalid pattern values: the pattern validator reports those.
			continue
		}
		s.ShapeGrid = grid
		fromPattern[i] = true
	}
	if len(fromPattern) == 0 {
		return input, nil
	}
	return &expanded, fromPattern
}

// collectPatternPostExpandFindings converts every slide-level pattern's own
// PostExpandWarnings into fit findings.
//
// A pattern knows things about the content it was just handed that no geometric
// detector can see: a bio over its two-line budget, a chart panel with no chart.
// Those warnings used to be read only by preview_presentation_plan, so validate
// and generate both reported "no issues" on a deck that rendered 75% empty and
// whose own pattern had already objected (go-slide-creator-wn4v).
//
// It expands the pattern regardless of whether the slide already carries a
// shape_grid: on the generate path the grid IS the expansion of that pattern,
// so skipping those slides would silence exactly the surface this fixes.
func collectPatternPostExpandFindings(input *PresentationInput, slideWidth, slideHeight int64, theme *types.ThemeInfo, layoutSets ...types.LayoutMetadata) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var out []patterns.FitFinding
	for i := range input.Slides {
		p := input.Slides[i].Pattern
		if p == nil {
			continue
		}
		probe := SlideInput{Pattern: p}
		var bounds patterns.LayoutBounds
		if len(layoutSets) > 0 {
			var rhythm *resolvedGrid
			if input.Grid != nil && validateGridConfig(input.Grid) == nil {
				rhythm = resolveGrid(input.Grid, layoutSets, slideWidth, slideHeight)
			}
			_, b := patternExpansionGeometry(input.Slides[i], layoutSets, slideWidth, slideHeight, rhythm)
			bounds = patterns.LayoutBounds{X: b.X, Y: b.Y, Width: b.CX, Height: b.CY}
		}
		_, warnings := expandSlidePatternGridWithWarningsAtBounds(&probe, i, slideWidth, slideHeight, theme, bounds)
		for _, w := range warnings {
			if f := patternWarningAsFinding(i, p.Name, w); f != nil {
				out = append(out, *f)
			}
		}
	}
	return out
}

// rerootPatternPath rewrites a finding path on an expanded pattern slide from
// the synthetic /slides/N/shape_grid root to /slides/N/pattern, matching what
// the geometry detectors already emit. The deck has no shape_grid at that
// index, so the shape_grid form would point at nothing.
func rerootPatternPath(path string, fromPattern map[int]bool) string {
	if len(fromPattern) == 0 || path == "" {
		return path
	}
	idx := slidepath.SlideIndex(path)
	if idx < 0 || !fromPattern[idx] {
		return path
	}
	gridRoot := slidepath.ShapeGrid(idx)
	if !strings.HasPrefix(path, gridRoot) {
		return path
	}
	return slidepath.SlideField(idx, "pattern") + strings.TrimPrefix(path, gridRoot)
}

// patternCellFix annotates a cell-text fix on a pattern slide.
//
// It used to swap the fix for the advisory rewrite_field, because
// reduce_cell_text refused with "slide has no shape_grid" on a pattern slide.
// repair_slide now resolves such a cell_path back to the pattern value that
// produced it (go-slide-creator-qnrb), so the executable directive stays — an
// agent that submits it gets the edit, and the one case that cannot be resolved
// (text composed at expansion) refuses with did_you_mean: replace_value.
func patternCellFix(fix *patterns.FixSuggestion, patternName string) *patterns.FixSuggestion {
	if fix == nil || patternName == "" {
		return fix
	}
	switch fix.Kind {
	case "reduce_cell_text", "reduce_text":
	default:
		return fix
	}
	params := map[string]any{}
	for k, v := range fix.Params {
		params[k] = v
	}
	params["pattern"] = patternName
	return &patterns.FixSuggestion{Kind: fix.Kind, Params: params}
}

// patternNameForSlide returns the pattern name of the slide a finding path
// points at, for findings whose emitter did not set it.
func patternNameForSlide(input *PresentationInput, path string) string {
	if input == nil {
		return ""
	}
	idx := slidepath.SlideIndex(path)
	if idx < 0 || idx >= len(input.Slides) || input.Slides[idx].Pattern == nil {
		return ""
	}
	return input.Slides[idx].Pattern.Name
}

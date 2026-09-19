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
func expandPatternsForFit(input *PresentationInput, slideWidth, slideHeight int64, theme *types.ThemeInfo) (*PresentationInput, map[int]bool) {
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
		grid := expandSlidePatternGrid(s, i, slideWidth, slideHeight, theme)
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

// patternCellFix replaces a fix that only reaches a raw shape_grid cell with one
// an agent can act on for a pattern slide: the cell does not exist in the deck
// JSON, so reduce_cell_text has nothing to edit. rewrite_field is the registered
// advisory for "shorten this text yourself"; the measured budget travels in
// params so the agent knows how much to cut, and the pattern name says where.
func patternCellFix(fix *patterns.FixSuggestion, patternName string) *patterns.FixSuggestion {
	if fix == nil {
		return nil
	}
	switch fix.Kind {
	case "reduce_cell_text", "reduce_text":
	default:
		return fix
	}
	params := map[string]any{}
	for k, v := range fix.Params {
		if k == "cell_path" {
			continue
		}
		params[k] = v
	}
	if patternName != "" {
		params["pattern"] = patternName
	}
	params["surface"] = "pattern_values"
	return &patterns.FixSuggestion{Kind: "rewrite_field", Params: params}
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

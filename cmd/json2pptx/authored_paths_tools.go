package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Tools that take a path back (go-slide-creator-9564b, -2v8me, -o45pn).
//
// A finding's path is a pointer into the authored deck; its debug.locator is
// the engine's own. repair_slide, repair_slides_batch and propose_repairs
// accept either in path / cell_path: enginePathParams turns what the caller
// sent into the locator the fix code works on, for the slide the call names.
// slide_index itself always counts rendered slides — finding.slide_number - 1.

// pathParamKeys are the fix params that carry a path into the deck.
var pathParamKeys = []string{"path", "cell_path"}

// enginePathParams returns params with every path param as the engine's
// locator on slide slideIdx. params is not modified.
func (a *authoredPaths) enginePathParams(params map[string]any, slideIdx int) map[string]any {
	if a == nil || len(params) == 0 || !a.hasOrigins() {
		return params
	}
	var out map[string]any
	for _, key := range pathParamKeys {
		p, ok := params[key].(string)
		if !ok || p == "" {
			continue
		}
		engine := a.engine(p, slideIdx)
		if engine == p {
			continue
		}
		if out == nil {
			out = copyFacts(params)
		}
		out[key] = engine
	}
	if out == nil {
		return params
	}
	return out
}

// engineFinding rewrites a finding a caller hands back (propose_repairs) to
// the engine's terms: the slide is the rendered one (slide_number, else
// where.slide, else the slide the authored path is on), and the path is the
// engine's locator — debug.locator when the finding still carries it.
func (a *authoredPaths) engineFinding(f *proposeRepairsFinding) {
	if f.SlideIndex == nil && f.SlideNumber != nil && *f.SlideNumber > 0 {
		idx := *f.SlideNumber - 1
		f.SlideIndex = &idx
	}
	hint := -1
	if f.SlideIndex != nil {
		hint = *f.SlideIndex
	}
	if locator, _ := f.Debug[locatorDebugKey].(string); locator != "" {
		f.Path = locator
	} else if f.Path != "" {
		f.Path = a.engine(f.Path, hint)
	}
	if f.Fix != nil {
		fix := *f.Fix
		fix.Params = a.enginePathParams(fix.Params, hint)
		f.Fix = &fix
	}
}

// editPattern applies edit to the pattern object at a pointer below slide
// (tokens: "pattern"; "compose","segments","1","pattern";
// "shape_grid","rows","0","cells","2","pattern", through nested grids and
// envelopes) and stores the result. It reports whether there is one.
func editPattern(slide *SlideInput, tokens []string, edit func(*PatternInput) error) (bool, error) {
	if len(tokens) == 0 {
		return false, nil
	}
	switch tokens[0] {
	case "pattern":
		if len(tokens) != 1 || slide.Pattern == nil {
			return false, nil
		}
		if err := edit(slide.Pattern); err != nil {
			return true, err
		}
		slide.ShapeGrid = nil
		return true, nil
	case "compose":
		return editComposePattern(slide.Compose, tokens[1:], edit)
	case "shape_grid":
		return editGridPattern(slide.ShapeGrid, tokens[1:], edit)
	}
	return false, nil
}

func editComposePattern(c *ComposeInput, tokens []string, edit func(*PatternInput) error) (bool, error) {
	if c == nil || len(tokens) < 3 || tokens[0] != "segments" {
		return false, nil
	}
	i, err := strconv.Atoi(tokens[1])
	if err != nil || i < 0 || i >= len(c.Segments) {
		return false, nil
	}
	seg := &c.Segments[i]
	switch tokens[2] {
	case "pattern":
		if len(tokens) != 3 || !seg.HasPattern() {
			return false, nil
		}
		return true, edit(&seg.Pattern)
	case "compose":
		return editComposePattern(seg.Compose, tokens[3:], edit)
	}
	return false, nil
}

func editGridPattern(grid *ShapeGridInput, tokens []string, edit func(*PatternInput) error) (bool, error) {
	if grid == nil || len(tokens) < 5 || tokens[0] != "rows" || tokens[2] != "cells" {
		return false, nil
	}
	r, rerr := strconv.Atoi(tokens[1])
	c, cerr := strconv.Atoi(tokens[3])
	if rerr != nil || cerr != nil || r < 0 || r >= len(grid.Rows) || c < 0 || c >= len(grid.Rows[r].Cells) || grid.Rows[r].Cells[c] == nil {
		return false, nil
	}
	cell := grid.Rows[r].Cells[c]
	switch tokens[4] {
	case "grid":
		return editGridPattern(cell.Grid, tokens[5:], edit)
	case "pattern":
		if len(tokens) != 5 || len(cell.Pattern) == 0 {
			return false, nil
		}
		var p PatternInput
		if err := json.Unmarshal(cell.Pattern, &p); err != nil {
			return true, err
		}
		if err := edit(&p); err != nil {
			return true, err
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return true, err
		}
		cell.Pattern = raw
		return true, nil
	}
	return false, nil
}

// patternValuePointer splits the pointer of a pattern value below a slide
// ("/pattern/values/2/small") into the pattern object's tokens and the
// value's tokens below values. ok is false for any other path.
func patternValuePointer(rest string) (object, value []string, ok bool) {
	const marker = "/pattern/values/"
	at := strings.LastIndex(rest, marker)
	if at < 0 {
		return nil, nil, false
	}
	return pointerTokens(rest[:at+len("/pattern")]), pointerTokens(rest[at+len("/pattern/values"):]), true
}

// reducePatternValueText is reduce_cell_text for a cell_path that names the
// authored pattern value a cell shows (/slides/2/pattern/values/2/small) —
// the address a finding reports — rather than the cell of the expanded grid:
// the value is shortened in place.
func reducePatternValueText(input *PresentationInput, slideIdx int, pointer string, maxChars int, params map[string]any) (appliedFix, bool) {
	const kind = "reduce_cell_text"
	rest, under := cutPrefixPath(pointer, slidepath.Slide(slideIdx))
	object, valueTokens, ok := patternValuePointer(rest)
	if !under || !ok {
		return appliedFix{}, false
	}
	var result appliedFix
	found, err := editPattern(&input.Slides[slideIdx], object, func(p *PatternInput) error {
		var values any
		if err := json.Unmarshal(p.Values, &values); err != nil {
			return fmt.Errorf("failed to parse pattern values: %w", err)
		}
		parent, _, whole := descend(values, valueTokens[:len(valueTokens)-1])
		last := valueTokens[len(valueTokens)-1]
		text, isText := "", false
		if whole {
			switch t := parent.(type) {
			case map[string]any:
				text, isText = t[last].(string)
			case []any:
				if i, err := strconv.Atoi(last); err == nil && i >= 0 && i < len(t) {
					text, isText = t[i].(string)
				}
			}
		}
		if !isText {
			return fmt.Errorf("%s is not a text value of pattern %q", pointer, p.Name)
		}
		truncated := truncateWithEllipsis(text, maxChars)
		if truncated == text {
			result = appliedFix{Kind: kind, Applied: false, Message: "text already within max_chars"}
			return nil
		}
		if !boolParam(params, "confirm_semantic_change", false) && losesProtectedFacts(text, truncated) {
			result = appliedFix{Kind: kind, Applied: false, Code: "semantic_review_required", Message: "truncation would remove a number, unit, negation, or qualifier; shorten the pattern value yourself or split the slide"}
			return nil
		}
		switch t := parent.(type) {
		case map[string]any:
			t[last] = truncated
		case []any:
			i, _ := strconv.Atoi(last)
			t[i] = truncated
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("failed to marshal pattern values: %w", err)
		}
		p.Values = encoded
		result = appliedFix{Kind: kind, Applied: true, Message: fmt.Sprintf("shortened pattern value %s to %d chars", joinPointer(valueTokens), maxChars)}
		return nil
	})
	if !found {
		return appliedFix{}, false
	}
	if err != nil {
		return appliedFix{Kind: kind, Applied: false, Message: err.Error()}, true
	}
	return result, true
}

package main

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Field-level repair budgets for shared-cell readability (go-slide-creator-ifcng).
//
// A pattern often writes several authored fields into one grid cell (an
// eyebrow, heading, body and bullets; a step label over its detail). Autofit
// shrinks every paragraph of that cell by one factor, so the paragraph that
// lands below its floor (a 12-char eyebrow) is rarely the one whose length
// caused the shrink. The cell's max_chars budget is measured for the whole
// cell, and handed to the failing paragraph's field it asked an author to
// rewrite "PILOT DESIGN" in at most 532 characters — no edit at all.
//
// fieldRepairBudget instead finds the field whose shortening actually clears
// the finding: it cuts each candidate paragraph (longest first) in the
// authored slide, re-runs this check on that slide — re-expanding its
// pattern, since a content-sized pattern re-lays itself out around shorter
// text — and keeps the longest length that leaves the cell clear. The fix
// then names that paragraph (edit_text) and its own budget
// (field_max_chars). When no single field can absorb the overflow within
// half its length, the repair is composition-level (repair: "composition"):
// fewer rows / bullets, shorter detail, more room or the kind's native
// layout — not a character budget on one field.

// fieldRepairMinKeep is the share of a field a single-field cut may keep at
// the least; a deeper cut is no longer "shorten this field".
const fieldRepairMinKeep = 0.5

// fieldRepairMaxCandidates bounds how many of a cell's paragraphs are tried.
const fieldRepairMaxCandidates = 3

// fieldRepairScope is the deck a finding was measured in.
type fieldRepairScope struct {
	authored                *PresentationInput
	slideIdx                int
	layouts                 []types.LayoutMetadata
	slideWidth, slideHeight int64
	theme                   *types.ThemeInfo
}

// fieldRepairBudget annotates a predicted (autofit-shrink) readability fix
// with the field to shorten and its own budget, or marks the repair as
// composition-level when no single field can clear the finding.
func fieldRepairBudget(f *patterns.FitFinding, scope fieldRepairScope, paras []cellParagraph) {
	if f == nil || f.Fix == nil || scope.authored == nil || scope.slideIdx < 0 || scope.slideIdx >= len(scope.authored.Slides) {
		return
	}
	if src, _ := f.Fix.Params["measurement_source"].(string); src != "predicted" {
		// Authored below its floor: no amount of shortening raises the size.
		return
	}
	cellPath, _ := f.Fix.Params["cell_path"].(string)
	if cellPath == "" {
		return
	}
	cands := fieldRepairCandidates(paras)
	for i, cand := range cands {
		if i == fieldRepairMaxCandidates {
			break
		}
		if n, ok := fieldCutThatClears(scope, cellPath, cand); ok {
			f.Fix.Params["edit_text"] = cand
			f.Fix.Params["field_max_chars"] = n
			return
		}
	}
	f.Fix.Params["repair"] = "composition"
	if len(cands) > 0 {
		// The longest field is still the one a composition repair most likely
		// trims; it names an exact authored path without promising a budget.
		f.Fix.Params["edit_text"] = cands[0]
	}
}

// fieldRepairCandidates returns the cell's distinct populated paragraphs,
// longest first.
func fieldRepairCandidates(paras []cellParagraph) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paras {
		t := strings.TrimSpace(p.text)
		if utf8.RuneCountInString(t) < 2 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return utf8.RuneCountInString(out[i]) > utf8.RuneCountInString(out[j])
	})
	return out
}

// fieldCutThatClears returns the longest length (in runes, shorter than the
// field and at least fieldRepairMinKeep of it) to which cutting the field
// that authored text clears every readability finding on cellPath.
func fieldCutThatClears(scope fieldRepairScope, cellPath, text string) (int, bool) {
	length := utf8.RuneCountInString(text)
	lo := int(float64(length)*fieldRepairMinKeep + 0.5)
	if lo < 1 {
		lo = 1
	}
	hi := length - 1
	if hi < lo || !cutClears(scope, cellPath, text, lo) {
		return 0, false
	}
	// Shorter text never shrinks more, so the clearing lengths are a prefix
	// of [lo, hi]: find its end.
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if cutClears(scope, cellPath, text, mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, true
}

// cutClears reports whether the slide, with the field that authored text cut
// to n runes, leaves no readability finding on cellPath.
func cutClears(scope fieldRepairScope, cellPath, text string, n int) bool {
	slide, ok := cutAuthoredField(scope.authored.Slides[scope.slideIdx], text, n)
	if !ok {
		return false
	}
	single := *scope.authored
	single.Slides = []SlideInput{slide}
	want := slideRelativePath(cellPath)
	for _, f := range collectReadability(&single, scope.layouts, scope.slideWidth, scope.slideHeight, scope.theme, false) {
		if f.Fix == nil {
			continue
		}
		if p, _ := f.Fix.Params["cell_path"].(string); slideRelativePath(p) == want {
			return false
		}
	}
	return true
}

// slideRelativePath drops the /slides/N prefix of a JSON pointer.
func slideRelativePath(p string) string {
	parts := strings.SplitN(p, "/", 4)
	if len(parts) == 4 && parts[1] == "slides" {
		return parts[3]
	}
	return p
}

// cutAtWord shortens text to at most n runes, at a word boundary when one
// falls in the second half of the cut.
func cutAtWord(text string, n int) string {
	r := []rune(text)
	if n >= len(r) {
		return text
	}
	out := string(r[:n])
	if i := strings.LastIndexByte(out, ' '); i > len(out)/2 {
		out = out[:i]
	}
	return strings.TrimSpace(out)
}

// cutAuthoredField returns a copy of slide whose first string line matching
// text (case-insensitively: patterns upper-case eyebrows) is cut to n runes.
func cutAuthoredField(slide SlideInput, text string, n int) (SlideInput, bool) {
	raw, err := json.Marshal(slide)
	if err != nil {
		return slide, false
	}
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return slide, false
	}
	replaced := false
	var walk func(v any) any
	walk = func(v any) any {
		if replaced {
			return v
		}
		switch t := v.(type) {
		case string:
			lines := strings.Split(t, "\n")
			for i, line := range lines {
				if strings.EqualFold(strings.TrimSpace(line), text) {
					lines[i] = cutAtWord(strings.TrimSpace(line), n)
					replaced = true
					return strings.Join(lines, "\n")
				}
			}
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t[k] = walk(t[k])
			}
		case []any:
			for i := range t {
				t[i] = walk(t[i])
			}
		}
		return v
	}
	doc = walk(doc)
	if !replaced {
		return slide, false
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return slide, false
	}
	var cut SlideInput
	if json.Unmarshal(out, &cut) != nil {
		return slide, false
	}
	return cut, true
}

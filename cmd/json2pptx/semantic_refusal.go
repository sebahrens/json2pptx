package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Semantic generation refusals (go-slide-creator-b7qqg.4).
//
// When the generator refuses a compiled DeckSpec (text below its readable
// floor, or source it would lose), render_deck_spec used to hand back only the
// error string, whose locator was a generated shape id such as
// /slides/0/rendered_shapes/200/paragraphs/1. Nothing in it names the DeckSpec
// field to edit. semanticRefusalDiagnostic keeps the refusal's code, severity
// and measurement, traces it back to the authored field, and supplies a patch
// the agent can run.

// semanticRefusalDiagnostic converts a generation refusal carried by err into
// a semantic diagnostic, or returns nil when err is not a refusal.
func semanticRefusalDiagnostic(cr *semantic.CompileResult, err error) *semanticDiagnostic {
	loss := generationRefusal(err)
	if loss == nil {
		return nil
	}
	var sm *semantic.SourceMap
	var ir *semantic.DeckIR
	if cr != nil {
		sm, ir = cr.SourceMap, cr.IR
	}
	d := semanticDiagFromFit(sm, patterns.FitFinding{ValidationError: *loss, Action: "refuse"})
	setDiagnosticSeverity(&d, diagnostics.SeverityError)
	d.Action = "refuse"

	var evidence generator.ReadabilityEvidence
	var refusal *generator.ReadabilityRefusal
	if errors.As(err, &refusal) && refusal != nil {
		evidence = refusal.Evidence
	}
	if ev := refusalEvidenceMap(evidence); ev != nil {
		d.Evidence = ev
	}

	rawIdx := slidepath.SlideIndex(loss.Path)
	// A rendered shape id means nothing to the author; the paragraph's text
	// says which field it came from.
	if strings.Contains(loss.Path, "/rendered_shapes/") || d.SemanticPath == "" || d.SemanticPath == slideSemanticPath(sm, rawIdx) {
		if field := semanticFieldForText(ir, rawIdx, evidence.Text); field != "" {
			d.SemanticPath = field
		} else if d.SemanticPath == "" && rawIdx >= 0 {
			d.SemanticPath = slideSemanticPath(sm, rawIdx)
		}
	}
	if d.SlideIndex == nil && rawIdx >= 0 {
		idx := rawIdx
		d.SlideIndex = &idx
	}
	if d.RecommendedEdit == nil {
		d.RecommendedEdit = &semantic.SemanticEdit{
			Kind: semantic.EditShortenText,
			Hint: "Preserve the meaning at a readable size: shorten this field, move detail to another slide, or switch the slide to a composition with more room (list_slide_kinds lists each kind's compositions).",
		}
	}
	if evidence.MinPt > 0 {
		params := map[string]any{"min_pt": evidence.MinPt, "actual_pt": evidence.ActualPt}
		if evidence.Role != "" {
			params["role"] = evidence.Role
		}
		for k, v := range d.RecommendedEdit.Params {
			params[k] = v
		}
		edit := *d.RecommendedEdit
		edit.Params = params
		d.RecommendedEdit = &edit
	}
	d.fallbackPatch = compositionSwitchPatch(ir, rawIdx, slideSemanticPath(sm, rawIdx))
	return &d
}

// generatedCellSemanticPath names the DeckSpec field behind a readability
// finding on a pattern-generated grid cell. The source map stops at the
// pattern's values, so a cell path (/slides/0/pattern/rows/0/cells/0/…) has no
// entry; the paragraph's text identifies the authored field instead.
func generatedCellSemanticPath(ir *semantic.DeckIR, f patterns.FitFinding) string {
	if ir == nil || f.Code != patterns.ErrCodeTextBelowReadableMin || f.Fix == nil {
		return ""
	}
	text, _ := f.Fix.Params["paragraph_text"].(string)
	return semanticFieldForText(ir, slidepath.SlideIndex(f.Path), text)
}

// compositionPatchDetail is the Details / evidence key a validate diagnostic
// uses to hand semanticizeFinding its composition-switch fallback patch.
const compositionPatchDetail = "composition_patch"

// editPathDetail is the Details / evidence key naming the DeckSpec field the
// repair edits when it differs from the field whose text is too small: in a
// shared cell, shortening the body is what lifts a 12-char eyebrow back to
// its floor (go-slide-creator-ifcng).
const editPathDetail = "edit_path"

// decorateReadabilityRefusal gives a validate_deck_spec readability refusal
// the same evidence and fallback a render refusal carries: measured against
// allowed size, and the composition switch to suggest when the authored field
// is a list no single rewrite can fix.
func decorateReadabilityRefusal(d *diagnostics.Diagnostic, ir *semantic.DeckIR, sm *semantic.SourceMap, f patterns.FitFinding) {
	if ir == nil || f.Code != patterns.ErrCodeTextBelowReadableMin || f.Action != "refuse" {
		return
	}
	if d.Details == nil {
		d.Details = map[string]any{}
	}
	if f.Fix != nil {
		if actual, ok := f.Fix.Params["actual_pt"].(float64); ok {
			d.Details["measured"] = map[string]any{"font_pt": actual}
		}
		if minPt, ok := f.Fix.Params["min_pt"].(float64); ok {
			d.Details["allowed"] = map[string]any{"min_font_pt": minPt}
		}
	}
	rawIdx := slidepath.SlideIndex(f.Path)
	if f.Fix != nil {
		if text, _ := f.Fix.Params["edit_text"].(string); text != "" {
			if field := semanticFieldForText(ir, rawIdx, text); field != "" && field != d.Path {
				d.Details[editPathDetail] = field
			}
		}
	}
	if patch := compositionSwitchPatch(ir, rawIdx, slideSemanticPath(sm, rawIdx)); patch != nil {
		d.Details[compositionPatchDetail] = patch
	}
}

// refusalEvidenceMap renders the refusal's measurement as the diagnostic's
// evidence: measured (the size the text renders at) against allowed (its
// floor), plus the role, viewing mode and paragraph text.
func refusalEvidenceMap(ev generator.ReadabilityEvidence) map[string]any {
	if ev.MinPt <= 0 && ev.Text == "" {
		return nil
	}
	out := map[string]any{}
	if ev.MinPt > 0 {
		out["measured"] = map[string]any{"font_pt": ev.ActualPt}
		out["allowed"] = map[string]any{"min_font_pt": ev.MinPt}
	}
	if ev.Role != "" {
		out["role"] = ev.Role
	}
	if ev.ViewingMode != "" {
		out["viewing_mode"] = ev.ViewingMode
	}
	if ev.MeasurementSource != "" {
		out["measurement_source"] = ev.MeasurementSource
	}
	if ev.Text != "" {
		out["text"] = ev.Text
	}
	return out
}

// slideSemanticPath is the DeckSpec locator of raw slide rawIdx, or "".
func slideSemanticPath(sm *semantic.SourceMap, rawIdx int) string {
	if rawIdx < 0 {
		return ""
	}
	return sm.SlidePath(rawIdx)
}

// compositionSwitchPatch returns the DeckSpec patch that moves slide rawIdx
// off its generated pattern onto the kind's native-layout alternative, which
// lays the same source out as template text instead of fixed grid cells. It
// is the source-preserving fallback when the refused text lives in a list and
// no single field can be rewritten. nil when the kind offers no such layout.
func compositionSwitchPatch(ir *semantic.DeckIR, rawIdx int, slidePath string) []any {
	if ir == nil || rawIdx < 0 || rawIdx >= len(ir.Slides) || slidePath == "" {
		return nil
	}
	layout := semantic.NativeLayoutAlternative(ir.Slides[rawIdx])
	if layout == "" {
		return nil
	}
	pointer, ok := semanticPointer(slidePath)
	if !ok {
		return nil
	}
	return []any{map[string]any{"op": "add", "path": pointer + "/layout", "value": layout}}
}

// minTextMatchRunes is the shortest authored string matched by containment;
// shorter strings ("A", "Q1") occur inside unrelated text too often.
const minTextMatchRunes = 3

// semanticFieldForText finds the DeckSpec field on raw slide rawIdx that
// authored text: the one string field equal to it, else the closest common
// ancestor of the string fields it contains (a tier's items joined into one
// paragraph resolve to that tier's items list). "" when nothing, or only the
// whole slide, matches.
func semanticFieldForText(ir *semantic.DeckIR, rawIdx int, text string) string {
	text = normalizeMatchText(text)
	if ir == nil || rawIdx < 0 || rawIdx >= len(ir.Slides) || text == "" {
		return ""
	}
	slide := ir.Slides[rawIdx]
	if slide.SourcePath == "" {
		return ""
	}
	leaves := map[string]string{}
	collectStringLeaves(slide.Body, "", leaves)
	var exact, contained []string
	for path, value := range leaves {
		v := normalizeMatchText(value)
		switch {
		case v == "":
		case v == text:
			exact = append(exact, path)
		case utf8.RuneCountInString(v) >= minTextMatchRunes && strings.Contains(text, v):
			contained = append(contained, path)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = contained
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	common := commonSemanticAncestor(matches)
	if common == "" {
		return ""
	}
	return slide.SourcePath + "." + strings.TrimPrefix(common, ".")
}

func normalizeMatchText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// collectStringLeaves records every string value under v by its DeckSpec
// path relative to the slide ("tiers[0].items[1]"). Composition overrides
// and the kind discriminator are not authored text.
func collectStringLeaves(v any, path string, out map[string]string) {
	switch t := v.(type) {
	case string:
		if path != "" {
			out[path] = t
		}
	case map[string]any:
		for k, child := range t {
			if path == "" && (k == "kind" || k == "pattern" || k == "layout") {
				continue
			}
			p := k
			if path != "" {
				p = path + "." + k
			}
			collectStringLeaves(child, p, out)
		}
	case []any:
		for i, child := range t {
			collectStringLeaves(child, fmt.Sprintf("%s[%d]", path, i), out)
		}
	case []string:
		for i, child := range t {
			collectStringLeaves(child, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

// commonSemanticAncestor returns the longest path every input shares at a
// segment boundary ("tiers[0].items[0]", "tiers[0].items[2]" share
// "tiers[0].items"), or "" when they share no segment.
func commonSemanticAncestor(paths []string) string {
	common := splitSemanticPath(paths[0])
	for _, p := range paths[1:] {
		segs := splitSemanticPath(p)
		n := 0
		for n < len(common) && n < len(segs) && common[n] == segs[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "")
}

// splitSemanticPath splits "tiers[0].items" into "tiers", "[0]", ".items".
func splitSemanticPath(p string) []string {
	var segs []string
	start := 0
	for i := 1; i < len(p); i++ {
		if p[i] == '.' || p[i] == '[' {
			segs = append(segs, p[start:i])
			start = i
		}
	}
	return append(segs, p[start:])
}

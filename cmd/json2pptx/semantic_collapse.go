package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// One entry per root cause (go-slide-creator-c2j5b).
//
// The agent journey review got four identical SEMANTIC_UNKNOWN_FIELD errors
// for one misnamed key on four list entries, three "label is N characters"
// findings for three options of one decision, and — on a slide that had
// fallen back to a bullet list — BODY_TOO_LONG, SLIDE_TEXT_DENSE and
// TEXT_SIZE_OFF_TARGET about that bullet list beside the finding that said why
// it was one. Each was a full finding with its own remediation block; one
// response was 21.5 KB.
//
// Two rules now give one entry per cause:
//
//   - a slide that lost its visual (SEMANTIC_PATTERN_DEGRADED) or is over a
//     row limit (SEMANTIC_DENSITY) carries the fit findings about the result
//     as its symptoms: they describe what the fallback looks like, and go
//     when the cause is fixed (foldFallbackSymptoms);
//   - findings with the same code and the same cause are one entry: it keeps
//     the first finding's path and message, counts the rest in occurrences and
//     lists every affected path in paths. Facts that differ per item (the
//     measured length) become lists in the order of paths, and the suggested
//     patch covers every item (collapseDiagnostics).

// diagMember is one of the findings a collapsed diagnostic stands for.
type diagMember struct {
	// Path is the member's dotted DeckSpec path.
	Path       string
	SlideIndex *int
	// fixParams are the member's own DeckSpec-safe fix params (its budget).
	fixParams map[string]any
	// facts are the member's measured / allowed evidence.
	facts map[string]any
}

var (
	// numberRE matches the figures that differ between findings of one cause.
	numberRE = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
	// quotedRE matches the authored text a message quotes.
	quotedRE = regexp.MustCompile(`"[^"]*"`)

	kindNameOnce sync.Once
	kindNameRE   *regexp.Regexp
)

// kindNames matches a registered slide kind named in a message.
func kindNames() *regexp.Regexp {
	kindNameOnce.Do(func() {
		names := make([]string, 0, 32)
		for _, k := range semantic.AllSlideKinds() {
			names = append(names, regexp.QuoteMeta(string(k)))
		}
		kindNameRE = regexp.MustCompile(`\b(?:` + strings.Join(names, "|") + `)\b`)
	})
	return kindNameRE
}

// causeKey identifies a finding's cause: its code, severity and action, and
// its message with what names the instance taken out. For every finding that
// is the figures (which item, what was measured). An advisory may also differ
// in the text it quotes and the kind of slide it sits on — six topic titles
// are one piece of advice — but a blocking finding may not: two different
// misspelled keys are two things to fix.
func causeKey(d semanticDiagnostic) string {
	msg := numberRE.ReplaceAllString(firstNonEmpty(d.baseMessage, d.Message), "#")
	if !diagnosticBlocks(d) {
		msg = kindNames().ReplaceAllString(quotedRE.ReplaceAllString(msg, `"…"`), "<kind>")
	}
	return d.Code + "\x00" + d.Severity + "\x00" + d.Action + "\x00" + d.Waived + "\x00" + msg
}

// collapsible reports whether a diagnostic may be folded with others of its
// cause: it must sit at an authored path, and a root cause that already
// carries its symptoms stays as it is.
func collapsible(d semanticDiagnostic) bool {
	return d.SemanticPath != "" && !strings.HasPrefix(d.SemanticPath, "/") && len(d.Symptoms) == 0
}

// collapseDiagnostics folds findings that share a code and a cause into the
// first of them, then lists each slide's fallback symptoms under their cause.
// It keeps the order of first occurrence and is idempotent: an entry that
// already stands for several findings takes in the ones that join it later.
func collapseDiagnostics(diags []semanticDiagnostic) []semanticDiagnostic {
	return foldFallbackSymptoms(collapseSameCause(diags))
}

// collapseSameCause is the same-cause rule of collapseDiagnostics.
func collapseSameCause(diags []semanticDiagnostic) []semanticDiagnostic {
	fallback := fallbackRoots(diags)
	first := map[string]int{}
	grown := map[int]bool{}
	out := make([]semanticDiagnostic, 0, len(diags))
	for i, d := range diags {
		// A fit finding about a slide's fallback belongs to that slide's cause,
		// not to the like finding on another slide.
		if _, symptom := fallbackSymptomOf(i, d, fallback); symptom || !collapsible(d) {
			out = append(out, d)
			continue
		}
		key := causeKey(d)
		at, seen := first[key]
		if !seen {
			first[key] = len(out)
			out = append(out, d)
			continue
		}
		root := &out[at]
		if root.SemanticPath == d.SemanticPath && len(d.members) == 0 {
			continue // the same finding reported twice
		}
		if len(root.members) == 0 {
			root.members = []diagMember{memberOf(*root)}
			root.baseMessage = root.Message
		}
		joining := d.members
		if len(joining) == 0 {
			joining = []diagMember{memberOf(d)}
		}
		for _, m := range joining {
			if !hasMember(root.members, m.Path) {
				root.members = append(root.members, m)
				grown[at] = true
			}
		}
	}
	for i := range out {
		if grown[i] {
			finishCollapsed(&out[i])
		}
	}
	return out
}

func hasMember(members []diagMember, path string) bool {
	for _, m := range members {
		if m.Path == path {
			return true
		}
	}
	return false
}

// memberOf records what a finding contributes to the entry it is folded into.
func memberOf(d semanticDiagnostic) diagMember {
	m := diagMember{Path: d.SemanticPath, SlideIndex: d.SlideIndex}
	if d.RecommendedEdit != nil {
		m.fixParams = d.RecommendedEdit.Params
	}
	if d.diag != nil {
		if d.diag.Fix != nil && m.fixParams == nil {
			m.fixParams = semanticFixParams(d.diag.Fix.Kind, d.diag.Fix.Params)
		}
		for _, k := range []string{"measured", "allowed"} {
			if v, ok := d.diag.Details[k]; ok {
				if m.facts == nil {
					m.facts = map[string]any{}
				}
				m.facts[k] = v
			}
		}
	}
	return m
}

// spansSlides reports whether a collapsed entry's members sit on more than one
// slide; such an entry has no single slide_number.
func spansSlides(members []diagMember) bool {
	for _, m := range members[1:] {
		if slideContainer(m.Path) != slideContainer(members[0].Path) {
			return true
		}
	}
	return false
}

// finishCollapsed writes the collapsed entry's message and its per-item facts.
func finishCollapsed(d *semanticDiagnostic) {
	n := len(d.members)
	where := "on this slide"
	if spansSlides(d.members) {
		where = "in the deck"
	}
	d.Message = d.baseMessage + fmt.Sprintf(" — %d like this %s (see paths)", n, where)
	if d.diag == nil {
		return
	}
	source := *d.diag
	source.Message = d.Message
	source.Details = map[string]any{}
	for k, v := range d.diag.Details {
		source.Details[k] = v
	}
	for _, k := range []string{"measured", "allowed"} {
		values := make([]any, 0, n)
		differ := false
		for _, m := range d.members {
			v, ok := m.facts[k]
			if !ok {
				values = nil
				break
			}
			if len(values) > 0 && !reflect.DeepEqual(values[0], v) {
				differ = true
			}
			values = append(values, v)
		}
		if differ {
			source.Details[k] = values
		}
	}
	if d.Evidence == nil {
		for _, k := range []string{"measured", "allowed"} {
			if v, ok := source.Details[k]; ok {
				if d.Evidence == nil {
					d.Evidence = map[string]any{}
				}
				d.Evidence[k] = v
			}
		}
	}
	d.diag = &source
}

// collapsedPatchOps builds the DeckSpec patch for every member of a collapsed
// finding, each with its own budget, so applying the suggestion clears the
// whole entry rather than its first item.
func collapsedPatchOps(data []byte, d semanticDiagnostic) []any {
	if len(d.members) < 2 {
		return nil
	}
	var ops []any
	for _, m := range d.members {
		ops = append(ops, semanticPatchOps(data, m.Path, d.Code, m.fixParams)...)
	}
	return ops
}

// expandCollapsedPatches replaces the single-item patch of each collapsed
// envelope finding with the patch for all of its items. envelope and diags
// are index-aligned.
func expandCollapsedPatches(envelope *diagnostics.FindingEnvelope, diags []semanticDiagnostic, data []byte, deckID string) {
	if deckID == "" {
		return
	}
	for i := range envelope.Findings {
		if i >= len(diags) {
			return
		}
		f := &envelope.Findings[i]
		ops := collapsedPatchOps(data, diags[i])
		if len(ops) < 2 || f.NextToolCall == nil || f.NextToolCall.Tool != "validate_deck_spec" {
			continue
		}
		f.NextToolCall = semanticPatchSuggestion(deckID, ops)
		if f.Remediation != nil && f.Remediation.Primary != nil && f.Remediation.Primary.Params != nil {
			f.Remediation.Primary.Params["ops"] = ops
		}
	}
}

// fallbackSymptomCodes are the fit findings that measure how much text a slide
// carries. On a slide that fell back from its visual they measure the
// fallback.
var fallbackSymptomCodes = map[string]bool{
	patterns.ErrCodeBodyTooLong:          true,
	patterns.ErrCodeSlideTextDense:       true,
	patterns.ErrCodeTextSizeOffTarget:    true,
	patterns.ErrCodeTextBelowReadableMin: true,
	patterns.ErrCodeDensityExceeded:      true,
}

// fallbackRootCodes are the spec-level findings that say a slide renders as
// something other than what its kind promised, or holds more rows than it can
// lay out.
var fallbackRootCodes = map[string]bool{
	diagnostics.CodeSemanticPatternDegraded: true,
	diagnostics.CodeSemanticDensity:         true,
}

// fallbackRoots maps each slide that has a fallback cause to the index of
// that cause in diags.
func fallbackRoots(diags []semanticDiagnostic) map[string]int {
	roots := map[string]int{}
	for i, d := range diags {
		if !fallbackRootCodes[d.Code] || d.Waived != "" || d.Action != "" {
			continue
		}
		if slide := slideContainer(d.SemanticPath); slide != "" {
			if _, seen := roots[slide]; !seen {
				roots[slide] = i
			}
		}
	}
	return roots
}

// fallbackSymptomOf returns the index of the cause diagnostic d (at index i)
// is a symptom of.
func fallbackSymptomOf(i int, d semanticDiagnostic, roots map[string]int) (int, bool) {
	if len(roots) == 0 || !fallbackSymptomCodes[d.Code] || d.Action == "" || d.Waived != "" {
		return 0, false
	}
	slide := slideContainer(d.SemanticPath)
	if slide == "" && d.SlideIndex != nil {
		slide = fmt.Sprintf("slides[%d]", *d.SlideIndex)
	}
	at, ok := roots[slide]
	return at, ok && at != i
}

// foldFallbackSymptoms lists the text-volume fit findings of a slide under the
// finding that explains them. A summary with six points degrades to a bullet
// list, and the bullet list is then too long, too dense and too small: three
// findings about a slide the author never asked for. They are that slide's
// symptoms; if one of them blocks, the cause blocks.
func foldFallbackSymptoms(diags []semanticDiagnostic) []semanticDiagnostic {
	roots := fallbackRoots(diags)
	if len(roots) == 0 {
		return diags
	}
	drop := map[int]bool{}
	blocked := map[int]bool{}
	for i, d := range diags {
		at, ok := fallbackSymptomOf(i, d, roots)
		if !ok {
			continue
		}
		root := &diags[at]
		// A symptom that blocks keeps its sentence (it is why the cause now
		// blocks); the rest are named by code, since they describe a rendering
		// the author did not ask for.
		symptom := findingSymptom{Code: d.Code}
		if diagnosticBlocks(d) {
			blocked[at] = true
			// A row limit and its fit twin say the same thing; one sentence.
			if root.Code != diagnostics.CodeSemanticDensity {
				symptom.Message = d.Message
			}
		}
		root.Symptoms = appendSymptom(root.Symptoms, symptom)
		// A cause that names no fix of its own keeps the symptom's (a table
		// over its row limit keeps the row to split at).
		if root.diag != nil && root.diag.Fix == nil && d.diag != nil && d.diag.Fix != nil {
			source := *root.diag
			source.Fix = d.diag.Fix
			root.diag = &source
			if root.RecommendedEdit == nil {
				root.RecommendedEdit = d.RecommendedEdit
			}
		}
		for _, nested := range d.Symptoms {
			root.Symptoms = appendSymptom(root.Symptoms, nested)
		}
		drop[i] = true
	}
	if len(drop) == 0 {
		return diags
	}
	out := make([]semanticDiagnostic, 0, len(diags)-len(drop))
	for i := range diags {
		if drop[i] {
			continue
		}
		d := diags[i]
		if len(d.Symptoms) > 0 {
			if blocked[i] && !diagnosticBlocks(d) {
				setDiagnosticSeverity(&d, diagnostics.SeverityError)
				d.Message += " — and what renders instead does not fit (see symptoms)"
			}
			if d.diag != nil {
				source := *d.diag
				source.Severity = diagnostics.Severity(d.Severity)
				source.Message = d.Message
				source.Details = map[string]any{}
				for k, v := range d.diag.Details {
					source.Details[k] = v
				}
				source.Details[symptomsDetail] = symptomsEvidence(d.Symptoms)
				d.diag = &source
			}
		}
		out = append(out, d)
	}
	return out
}

// appendSymptom adds a symptom unless the list already has it.
func appendSymptom(list []findingSymptom, s findingSymptom) []findingSymptom {
	for _, have := range list {
		if have.Code == s.Code && have.Path == s.Path && have.Message == s.Message {
			return list
		}
	}
	return append(list, s)
}

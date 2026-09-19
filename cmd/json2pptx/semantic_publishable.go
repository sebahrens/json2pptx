package main

import (
	"fmt"
	"sort"
)

// Publishability of a rendered deck (go-slide-creator-swak).
//
// render_deck_spec's ok/success answer one question — "was the .pptx written?"
// — and the closure audit found a deck that answered yes while carrying an
// action:refuse diagnostic and a quality gate that had already failed. An agent
// keying on ok (which the server instructions tell it to use as the
// precondition) shipped the broken slide; only reading every diagnostic's
// severity saved it.
//
// Rather than overload ok — other tools' parity depends on it meaning "the call
// produced its artifact" — the response gained a second, explicit verdict.
// publishable answers "is it fit to ship?", and blocking_reasons say why not.

// blockingDiagnosticReasons returns one human-readable reason per diagnostic
// that blocks publication: an error-severity finding, or one whose action is
// refuse. Reasons are deduplicated by code and sorted, so the list is stable
// and short even when a finding repeats across slides.
func blockingDiagnosticReasons(diags []semanticDiagnostic) []string {
	byCode := map[string]int{}
	paths := map[string]string{}
	for _, d := range diags {
		if d.Severity != "error" && d.Action != "refuse" {
			continue
		}
		byCode[d.Code]++
		if _, seen := paths[d.Code]; !seen {
			paths[d.Code] = firstNonEmpty(d.SemanticPath, d.RawPath)
		}
	}
	if len(byCode) == 0 {
		return nil
	}
	codes := make([]string, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	reasons := make([]string, 0, len(codes))
	for _, code := range codes {
		reason := code
		if n := byCode[code]; n > 1 {
			reason = fmt.Sprintf("%s (%d slides)", code, n)
		} else if p := paths[code]; p != "" {
			reason = fmt.Sprintf("%s at %s", code, p)
		}
		reasons = append(reasons, reason)
	}
	return reasons
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// publishabilityOf decides whether a rendered deck is fit to ship and why not.
// A deck is publishable when nothing in its diagnostics blocks and the
// deterministic quality gate — the same gate score_deck applies — passed.
func publishabilityOf(diags []semanticDiagnostic, q *QualityScore) (publishable bool, reasons []string) {
	reasons = blockingDiagnosticReasons(diags)
	if q != nil && q.QualityGate != nil && !q.QualityGate.Passed {
		for _, r := range q.QualityGate.Reasons {
			reasons = append(reasons, "quality gate: "+r)
		}
		if len(q.QualityGate.Reasons) == 0 {
			reasons = append(reasons, "quality gate: failed")
		}
	}
	return len(reasons) == 0, reasons
}

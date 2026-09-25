package main

import (
	"fmt"
	"sort"
)

// Publication status of a rendered semantic deck (go-slide-creator-swak,
// go-slide-creator-uxfx8.1).
//
// render_deck_spec's ok/success answer one question — "was the .pptx written?"
// — and the closure audit found a deck that answered yes while carrying an
// action:refuse diagnostic and a quality gate that had already failed. An agent
// keying on ok (which the server instructions tell it to use as the
// precondition) shipped the broken slide; only reading every diagnostic's
// severity saved it.
//
// Rather than overload ok — other tools' parity depends on it meaning "the call
// produced its artifact" — the response carries deterministic_ready and
// publishable separately. A render cannot approve pixels it has not inspected.

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

// semanticPublicationStatus combines the deterministic verdict with evidence
// for the *current* artifact. It reuses the facade's two-stage contract:
// deterministic_ready can be reached on the render call, while publishable
// additionally requires an approved all-slide review of matching bytes.
func semanticPublicationStatus(diags []semanticDiagnostic, q *QualityScore, currentHash string) facadeStatus {
	diagnosticReasons := blockingDiagnosticReasons(diags)
	reasons := append([]string(nil), diagnosticReasons...)
	gatePassed := q != nil && q.QualityGate != nil && q.QualityGate.Passed
	if q == nil || q.QualityGate == nil {
		reasons = append(reasons, "quality gate: missing")
	} else if !q.QualityGate.Passed {
		for _, r := range q.QualityGate.Reasons {
			reasons = append(reasons, "quality gate: "+r)
		}
		if len(q.QualityGate.Reasons) == 0 {
			reasons = append(reasons, "quality gate: failed")
		}
	}
	gatePassed = gatePassed && len(diagnosticReasons) == 0

	evidenceComplete, outputValid, visuallyApproved := false, false, false
	if q != nil && q.Evidence != nil {
		e := q.Evidence
		outputValid = e.StructuralValid
		evidenceComplete = e.SchemaValid && e.Generated && e.FitChecked && e.StructuralValid && currentHash != "" && e.ArtifactSHA256 == currentHash
		visuallyApproved = e.Approved && currentHash != "" && e.ArtifactSHA256 == currentHash
	}
	return deriveFacadeStatus(gatePassed, evidenceComplete, outputValid, visuallyApproved, reasons, contentProvenanceAuthorSupplied)
}

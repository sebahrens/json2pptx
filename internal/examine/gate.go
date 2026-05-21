package examine

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

// CanonicalTitleID is the single portable on-disk name every title-typed
// placeholder must use, so JSON authors and agents can rely on one
// placeholder_id for the title slot regardless of which template they target.
// It mirrors the canonical name harmonised across the bundled templates
// (go-slide-creator-jtlb).
const CanonicalTitleID = "title"

// SectionNumberID is the exact on-disk name a section-divider layout's
// auto-numbering frame must carry, so the normalizer preserves it and the
// engine writes the section ordinal into the right shape
// (go-slide-creator-5mf7, -sjzb).
const SectionNumberID = "Section Number"

// Gate-violation codes. These are CI-gate findings, distinct from the
// report's FindingEnvelope codes: the gate promotes a fixed set of structural
// template requirements into hard build failures so a non-conformant template
// cannot merge. They are intentionally separate from examine's agent-facing
// findings (which stay informational) — the gate reads those findings (see
// GateCodeErrorFinding) but adds the structural checks the report only
// surfaces as warnings or not at all.
const (
	// GateCodeEmptyTags fires when a layout carries no classification tags, so
	// tag-based selection can never reach it.
	GateCodeEmptyTags = "GATE.LAYOUT_EMPTY_TAGS"
	// GateCodeCanonicalCoverage fires when a content-bearing canonical family
	// (title-slide, section-divider, one-content, qa-closing) has no layout.
	GateCodeCanonicalCoverage = "GATE.CANONICAL_COVERAGE_INCOMPLETE"
	// GateCodeTitleNaming fires when a title-typed placeholder is named
	// anything other than exactly "title" on disk.
	GateCodeTitleNaming = "GATE.TITLE_PLACEHOLDER_NAMING"
	// GateCodeSectionNumber fires when a section-divider layout lacks a
	// placeholder named exactly "Section Number".
	GateCodeSectionNumber = "GATE.SECTION_NUMBER_MISSING"
	// GateCodeErrorFinding fires once per error-severity finding the
	// examination already emitted.
	GateCodeErrorFinding = "GATE.ERROR_FINDING"
)

// GateViolation is one CI-gate failure: a precise, human-readable reason a
// template is not fit to ship, tagged with a stable code so the workflow log
// and reviewers can identify the failure class at a glance.
type GateViolation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Gate applies the template CI gate to a built report and returns every
// violation found (empty when the template is conformant). The checks are, in
// order:
//
//  1. every layout has at least one classification tag;
//  2. canonical_coverage is complete across the four content-bearing families;
//  3. every title-typed placeholder is named exactly "title";
//  4. every section-divider layout has a "Section Number" placeholder;
//  5. the examination emitted no error-severity finding.
//
// Gate is pure and side-effect-free; the CLI's --gate flag prints the result
// and exits non-zero when the slice is non-empty.
func Gate(report *Report) []GateViolation {
	if report == nil {
		return nil
	}
	var violations []GateViolation
	violations = append(violations, gateCheckTags(report)...)
	violations = append(violations, gateCheckCoverage(report)...)
	violations = append(violations, gateCheckTitleNaming(report)...)
	violations = append(violations, gateCheckSectionNumber(report)...)
	violations = append(violations, gateCheckErrorFindings(report)...)
	return violations
}

// gateCheckTags flags any layout with an empty tag set.
func gateCheckTags(report *Report) []GateViolation {
	var v []GateViolation
	for i := range report.Layouts {
		l := &report.Layouts[i]
		if len(l.Tags) == 0 {
			v = append(v, GateViolation{
				Code: GateCodeEmptyTags,
				Message: fmt.Sprintf(
					"layout %q (%s) has no classification tags; tag-based selection can never reach it — "+
						"give it at least one tag (canonical role tag or a name-based tag like \"statement\")",
					l.Name, l.ID,
				),
			})
		}
	}
	return v
}

// gateCheckCoverage flags any content-bearing canonical family with no layout.
func gateCheckCoverage(report *Report) []GateViolation {
	var v []GateViolation
	for _, fam := range canonicalFamilies {
		c, ok := report.CanonicalCoverage[string(fam)]
		if !ok || !c.Present {
			v = append(v, GateViolation{
				Code: GateCodeCanonicalCoverage,
				Message: fmt.Sprintf(
					"canonical coverage incomplete: no %s layout (the four required content-bearing families are "+
						"title-slide, section-divider, one-content, qa-closing)",
					string(fam),
				),
			})
		}
	}
	return v
}

// gateCheckTitleNaming flags title-typed placeholders not named exactly "title".
func gateCheckTitleNaming(report *Report) []GateViolation {
	var v []GateViolation
	for i := range report.Layouts {
		l := &report.Layouts[i]
		for j := range l.Placeholders {
			ph := &l.Placeholders[j]
			if ph.Type == string(types.PlaceholderTitle) && ph.ID != CanonicalTitleID {
				v = append(v, GateViolation{
					Code: GateCodeTitleNaming,
					Message: fmt.Sprintf(
						"layout %q: title-typed placeholder named %q on disk, must be exactly %q — "+
							"normalize the placeholder name before committing the template",
						l.Name, ph.ID, CanonicalTitleID,
					),
				})
			}
		}
	}
	return v
}

// gateCheckSectionNumber flags section-divider layouts without a "Section
// Number" placeholder.
func gateCheckSectionNumber(report *Report) []GateViolation {
	var v []GateViolation
	for i := range report.Layouts {
		l := &report.Layouts[i]
		if l.CanonicalFamily != string(types.LayoutFamilySectionDivider) {
			continue
		}
		if !hasSectionNumberPlaceholder(l) {
			v = append(v, GateViolation{
				Code: GateCodeSectionNumber,
				Message: fmt.Sprintf(
					"section-divider layout %q lacks a placeholder named exactly %q; section auto-numbering has "+
						"no frame to write the ordinal into",
					l.Name, SectionNumberID,
				),
			})
		}
	}
	return v
}

// hasSectionNumberPlaceholder reports whether a layout carries a placeholder
// named exactly "Section Number".
func hasSectionNumberPlaceholder(l *LayoutReport) bool {
	for i := range l.Placeholders {
		if l.Placeholders[i].ID == SectionNumberID {
			return true
		}
	}
	return false
}

// gateCheckErrorFindings flags every error-severity finding the examination
// already produced.
func gateCheckErrorFindings(report *Report) []GateViolation {
	var v []GateViolation
	for i := range report.Findings.Findings {
		f := &report.Findings.Findings[i]
		if f.Severity == diagnostics.SeverityError {
			v = append(v, GateViolation{
				Code: GateCodeErrorFinding,
				Message: fmt.Sprintf("examination emitted an error finding [%s]: %s", f.Code, f.Message),
			})
		}
	}
	return v
}

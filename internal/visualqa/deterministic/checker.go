// Package deterministic provides rule-based visual quality checks that produce
// zero false positives. Unlike the heuristic (vision-model) layer in the parent
// visualqa package, these checks use geometry math, theme color resolution, and
// computed measurements — no LLM calls.
package deterministic

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// SeverityWeight maps FitFinding action strings to point deductions.
// Higher values = more severe. The score formula is:
//
//	100 - sum(weight(action) for each finding)
//
// clamped to [0, 100].
var SeverityWeight = map[string]int{
	"refuse":          25,
	"shrink_or_split": 15,
	"review":          5,
	"info":            0,
}

// ScoreFinding is a single deterministic finding in the score_deck output.
// It reuses the FitFinding envelope for consistency with the fit-report path.
type ScoreFinding struct {
	Code     string                  `json:"code"`
	Severity string                  `json:"severity"` // "error", "warning", "info"
	Message  string                  `json:"message"`
	Fix      *patterns.FixSuggestion `json:"fix,omitempty"`
	// Class separates a poor pattern choice ("pattern_choice") from a rendering
	// problem ("rendering") or a content-authoring problem ("content"). It is
	// derived from the finding code via patterns.FindingClass so a QA report can
	// tell whether a finding is a layout mistake to swap away from or a
	// rendering issue to fix.
	Class string `json:"class"`
}

// SlideScore holds the score and findings for a single slide.
type SlideScore struct {
	Index    int            `json:"index"`
	Score    int            `json:"score"`
	Findings []ScoreFinding `json:"findings"`
}

// CompositionDiagnostic is a single composition-level finding (deck rhythm).
type CompositionDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // "warning" or "info"
	Message  string `json:"message"`
}

// CompositionResult holds the composition axis of the deck score.
type CompositionResult struct {
	Score       int                     `json:"score"`       // 0–100
	Diagnostics []CompositionDiagnostic `json:"diagnostics"` // individual composition issues
}

// ScoreBasisStructural labels a score computed by deterministic rules over the
// GENERATED deck structure (fit findings, pagination, autofit, contrast swaps,
// deck rhythm). It never inspects rendered pixels.
const ScoreBasisStructural = "structural"

// DeckScore is the top-level score_deck response.
type DeckScore struct {
	// OverallScore is on the shared 0-100 scale.
	OverallScore int `json:"overall_score"`
	// Basis states what the score measured: always ScoreBasisStructural here.
	Basis       string             `json:"basis"`
	PerSlide    []SlideScore       `json:"per_slide"`
	Composition *CompositionResult `json:"composition,omitempty"`
	Summary     DeckSummary        `json:"summary"`
	QualityGate *QualityGate       `json:"quality_gate,omitempty"`
	ModeUsed    string             `json:"mode_used"`
	// RenderEvidence is present only when the render-time finding pass that
	// backs the score did NOT complete (slide conversion, temp-dir creation, or
	// generation failed). Its presence is the unambiguous signal that the score
	// and quality gate reflect static analysis only — the absence of render-time
	// findings is the result of an internal failure, not a clean render. When it
	// is present an explicit RENDER_EVIDENCE_INCOMPLETE finding also appears in
	// the findings set / quality gate.
	RenderEvidence *RenderEvidence `json:"render_evidence,omitempty"`
}

// RenderEvidence reports whether the render-time finding pass behind a
// deterministic score ran to completion. Render-time findings (contrast swaps,
// autofit shrink, pagination, clamping) only materialize when the deck is
// actually generated; when that generation step fails, an empty render-finding
// set must NOT be read as evidence of a clean render. RenderEvidence makes the
// failure machine-readable so callers (and quality gates) refuse to treat
// absence-of-findings as a pass.
type RenderEvidence struct {
	// Complete is true when slide conversion, temp-dir creation, and the
	// generation pass all succeeded.
	Complete bool `json:"complete"`
	// Stage names the failed step when Complete is false: "convert", "tempdir",
	// or "generate". Empty when Complete is true.
	Stage string `json:"stage,omitempty"`
	// Detail carries the underlying error text for the failed stage.
	Detail string `json:"detail,omitempty"`
	// Degraded is true when the caller explicitly permitted scoring to proceed
	// on incomplete render evidence (allow_degraded_scoring). The incompleteness
	// is then advisory rather than blocking, but the score is still labeled
	// degraded so it can never be confused with a clean pass.
	Degraded bool `json:"degraded,omitempty"`
}

// QualityGate is the machine-readable "definition of done" for a deck. Passed
// is true iff every criterion in Criteria is satisfied; Reasons enumerates the
// unmet ones (empty when Passed). Surfacing the Criteria block in the response
// lets agents reason about why the gate did or didn't pass and pin a known set
// of thresholds in their own tests.
type QualityGate struct {
	Passed   bool                `json:"passed"`
	Reasons  []string            `json:"reasons"`
	Criteria QualityGateCriteria `json:"criteria"`
}

// QualityGateCriteria documents the thresholds applied. The score_deck handler
// uses the fixed defaults exposed by DefaultQualityGateCriteria — auto_repair
// has its own tunable gate elsewhere (different defaults to give the repair
// loop room to converge).
type QualityGateCriteria struct {
	MinScore                int  `json:"min_score"`
	MaxP0Findings           int  `json:"max_p0_findings"`
	MaxP1Findings           int  `json:"max_p1_findings"`
	RequireTakeawayOnCharts bool `json:"require_takeaway_on_charts"`
	AllowAccentOverload     bool `json:"allow_accent_overload"`
	// MaxProblemSlidesPct caps the share of slides carrying at least one
	// finding. Without it a deck reached 95-100 with an unresolved review
	// finding on every slide — five slides each holding a single one-word
	// bullet scored 100 and passed (go-slide-creator-q7ar). 0 disables it.
	MaxProblemSlidesPct int `json:"max_problem_slides_pct"`
	// MinCompositionScore is the floor for the deck's composition (rhythm)
	// score. The composition axis was computed and then discarded: eight
	// consecutive identical kpi-3up slides scored 100 and PASSED the gate with
	// composition 55 and diagnostics [pattern_run, missing_emphasis]. The single
	// most common LLM deck failure — everything looks the same — was already
	// measured in the response and had no consequence (go-slide-creator-xx9i).
	// 0 disables it, which is what the subset (slide_indices) path uses: a
	// composition score over a few slides is not a deck's rhythm.
	MinCompositionScore int `json:"min_composition_score"`
}

// Default thresholds for the score_deck quality gate — the numeric definition
// of done. Tighter than the auto_repair convergence gate (min_score 75,
// max_p1 2) because score_deck's gate is the ship-quality check, not the
// repair-loop continue/stop signal.
const (
	DefaultQualityGateMinScore      = 80
	DefaultQualityGateMaxP0Findings = 0
	DefaultQualityGateMaxP1Findings = 0
	// DefaultQualityGateMaxProblemSlidesPct allows a minority of slides to
	// carry an open advisory finding — real decks always have a few — but not
	// a deck where most slides do.
	DefaultQualityGateMaxProblemSlidesPct = 40

	// DefaultQualityGateMinCompositionScore is the composition floor. 65 sits
	// above the graded monotony decks (55 for eight identical kpi-3up slides,
	// 50 for seven identical bullet slides, 60 for four title-less ones) and
	// below every deck built to be good, which score 100
	// (go-slide-creator-xx9i).
	DefaultQualityGateMinCompositionScore = 65
	// minProblemSlidesForShare is the fewest blemished slides that can trip the
	// share criterion.
	minProblemSlidesForShare = 3
)

// DefaultQualityGateCriteria returns the fixed ship-quality thresholds used by
// score_deck. Returning a value (not a global) prevents accidental mutation
// by callers and keeps the criteria block self-documenting in every response.
func DefaultQualityGateCriteria() QualityGateCriteria {
	return QualityGateCriteria{
		MinScore:                DefaultQualityGateMinScore,
		MaxP0Findings:           DefaultQualityGateMaxP0Findings,
		MaxP1Findings:           DefaultQualityGateMaxP1Findings,
		RequireTakeawayOnCharts: true,
		AllowAccentOverload:     false,
		MaxProblemSlidesPct:     DefaultQualityGateMaxProblemSlidesPct,
		MinCompositionScore:     DefaultQualityGateMinCompositionScore,
	}
}

// EvaluateQualityGate computes a QualityGate verdict against the given score
// and findings using the supplied criteria. Reason order is deterministic:
// score → P0 → P1 → takeaway → accent_overload → composition → problem-slide
// share, so agents can pattern-match on the leading reason.
//
// findings should be the same slice that produced the score (i.e. already
// scoped to the slides being evaluated when slide_indices is set); the gate
// counts a finding once per occurrence regardless of action ranking.
func EvaluateQualityGate(ds *DeckScore, findings []patterns.FitFinding, criteria QualityGateCriteria) *QualityGate {
	gate := &QualityGate{
		Passed:   false,
		Reasons:  []string{},
		Criteria: criteria,
	}
	if ds == nil {
		return gate
	}

	if ds.OverallScore < criteria.MinScore {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("score %d < min_score %d", ds.OverallScore, criteria.MinScore))
	}

	var p0, p1, takeawayMissing, accentOverload int
	for _, f := range findings {
		switch f.Action {
		case "refuse":
			p0++
		case "shrink_or_split":
			p1++
		}
		switch f.Code {
		case patterns.ErrCodeTakeawayMissing:
			takeawayMissing++
		case patterns.ErrCodeAccentOverload:
			accentOverload++
		}
	}

	if p0 > criteria.MaxP0Findings {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d P0 (refuse) finding(s) exceeds max_p0_findings %d", p0, criteria.MaxP0Findings))
	}
	if p1 > criteria.MaxP1Findings {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d P1 (shrink_or_split) finding(s) exceeds max_p1_findings %d", p1, criteria.MaxP1Findings))
	}
	if criteria.RequireTakeawayOnCharts && takeawayMissing > 0 {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d chart/matrix slide(s) missing takeaway", takeawayMissing))
	}
	if !criteria.AllowAccentOverload && accentOverload > 0 {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d slide(s) emit accent_overload (too many distinct accents)", accentOverload))
	}
	// Composition (deck rhythm). Only when the score was computed over the whole
	// deck — ds.Composition is nil on the slide_indices path, where a rhythm
	// verdict would be meaningless.
	if criteria.MinCompositionScore > 0 && ds.Composition != nil && ds.Composition.Score < criteria.MinCompositionScore {
		reason := fmt.Sprintf("composition %d < min_composition_score %d", ds.Composition.Score, criteria.MinCompositionScore)
		if codes := compositionCodes(ds.Composition); codes != "" {
			reason += " (" + codes + ")"
		}
		gate.Reasons = append(gate.Reasons, reason)
	}

	// The share criterion needs a minimum absolute count: on a four-slide deck
	// two blemished slides are 50%, which is not the "most of this deck is
	// wrong" signal the criterion exists to catch.
	if criteria.MaxProblemSlidesPct > 0 && len(ds.PerSlide) > 0 && ds.Summary.ProblemSlidesCount >= minProblemSlidesForShare {
		pct := ds.Summary.ProblemSlidesCount * 100 / len(ds.PerSlide)
		if pct > criteria.MaxProblemSlidesPct {
			gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d of %d slides carry findings (%d%%) — exceeds max_problem_slides_pct %d",
				ds.Summary.ProblemSlidesCount, len(ds.PerSlide), pct, criteria.MaxProblemSlidesPct))
		}
	}

	gate.Passed = len(gate.Reasons) == 0
	return gate
}

// compositionCodes joins a composition result's diagnostic codes, so the gate
// reason names WHICH rhythm problem dropped the score rather than only the
// number.
func compositionCodes(c *CompositionResult) string {
	if c == nil || len(c.Diagnostics) == 0 {
		return ""
	}
	seen := map[string]bool{}
	codes := make([]string, 0, len(c.Diagnostics))
	for _, d := range c.Diagnostics {
		if d.Code == "" || seen[d.Code] {
			continue
		}
		seen[d.Code] = true
		codes = append(codes, d.Code)
	}
	sort.Strings(codes)
	return strings.Join(codes, ", ")
}

// DeckSummary provides aggregate stats.
type DeckSummary struct {
	TopCodes           []CodeCount `json:"top_codes"`
	SlideCount         int         `json:"slide_count"`
	ProblemSlidesCount int         `json:"problem_slides_count"`
}

// CodeCount pairs a finding code with its occurrence count.
type CodeCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// actionToSeverity maps a FitFinding action to its severity label, through the
// one shared mapping. It used to keep its own copy, which read "review" as a
// warning while every envelope read it as info: the same code came back at two
// severities depending on which tool the agent asked (go-slide-creator-7xyy).
// The score itself is unaffected — that reads SeverityWeight[action], not this
// label.
func actionToSeverity(action string) string {
	return string(diagnostics.SeverityForAction(action))
}

// breadthExemptCodes are advisory codes about AIRINESS rather than defect: a
// KPI slide with three big numbers is supposed to be visually sparse, and the
// underfill family fires on exactly those slides. They still cost their per-slide
// points, but they do not make a slide "a problem slide" for the breadth penalty
// or the gate's problem-slide criterion — otherwise breadth turns the project's
// own showcase decks into gate failures, which is the noise this work removes
// rather than adds (go-slide-creator-q7ar).
var breadthExemptCodes = map[string]bool{
	patterns.ErrCodeSparseLayout:       true,
	patterns.ErrCodeSparseFill:         true,
	patterns.ErrCodeSlideUnderused:     true,
	patterns.ErrCodeCellUnderfilled:    true,
	patterns.ErrCodePatternUnderfilled: true,
	// A 44pt title running to two lines in an 11.5in placeholder is ordinary
	// design. The finding is useful advice ("this headline is long"); it is not
	// evidence that the slide is broken.
	patterns.ErrCodeTitleWraps: true,
}

// isBreadthProblem reports whether a finding makes its slide count as a problem
// slide.
func isBreadthProblem(f patterns.FitFinding) bool {
	if SeverityWeight[f.Action] <= 0 {
		return false
	}
	return !breadthExemptCodes[f.Code]
}

// slideHasBreadthProblem reports whether any of a slide's findings counts
// toward the problem-slide share.
func slideHasBreadthProblem(findings []patterns.FitFinding) bool {
	for _, f := range findings {
		if isBreadthProblem(f) {
			return true
		}
	}
	return false
}

// breadthPenaltyWeight scales how much the SHARE of affected slides pulls the
// deck score down. The mean of per-slide scores saturates: five slides each
// carrying one advisory finding averaged 95, so a deck where every slide is
// nearly empty, titleless or full of exemplar copy scored the same as a good
// deck with one blemish. Breadth is the signal a human reads first — "most of
// this deck has something wrong" — and folding it in moved the correlation with
// blind human grades from +0.33 to +0.76 across the 16-deck calibration set
// (go-slide-creator-q7ar).
const breadthPenaltyWeight = 1.0

// breadthPenaltyTolerance is the share of blemished slides a deck is allowed
// before breadth counts against it. Every real deck has one slide with an open
// advisory on it; a quarter of them having one is a different deck. Calibrated
// against the blind-graded set: at 0 tolerance the correlation is +0.86 but the
// good decks fall below the ship threshold on two advisory findings, at 0.15
// it is +0.82 and all three good decks pass.
const breadthPenaltyTolerance = 0.15

// breadthAdjustedScore applies the breadth penalty to a mean per-slide score.
func breadthAdjustedScore(mean, problemSlides, slides int) int {
	// Same floor as the gate's share criterion: on a four-slide deck two
	// blemished slides are 50% but not a pattern, and penalising that turned
	// short showcase decks into failures.
	if slides <= 0 || problemSlides < minProblemSlidesForShare {
		return mean
	}
	share := float64(problemSlides) / float64(slides)
	excess := share - breadthPenaltyTolerance
	if excess <= 0 {
		return mean
	}
	adjusted := float64(mean) * (1 - breadthPenaltyWeight*excess/(1-breadthPenaltyTolerance))
	if adjusted < 0 {
		adjusted = 0
	}
	return int(math.Round(adjusted))
}

// ScoreFromFindingsForIndices computes a DeckScore where PerSlide only contains
// entries for the slides listed in includedIndices. Findings on slides not in
// that set are ignored. The summary.slide_count reflects the full-deck size
// (slideCount); problem_slides_count and overall_score are computed across the
// included slides only.
//
// The returned PerSlide slice is ordered by the values in includedIndices
// (callers that want sorted order should sort the slice before passing it in).
// Duplicate indices are dropped.
func ScoreFromFindingsForIndices(findings []patterns.FitFinding, slideCount int, includedIndices []int) *DeckScore {
	// Build the included set, preserving caller order on first occurrence.
	seen := make(map[int]bool, len(includedIndices))
	order := make([]int, 0, len(includedIndices))
	for _, i := range includedIndices {
		if i < 0 || i >= slideCount || seen[i] {
			continue
		}
		seen[i] = true
		order = append(order, i)
	}

	// Group findings by slide index, but only those whose slide is included.
	bySlide := map[int][]patterns.FitFinding{}
	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		if !seen[si] {
			continue
		}
		bySlide[si] = append(bySlide[si], f)
	}

	perSlide := make([]SlideScore, 0, len(order))
	codeCounts := map[string]int{}
	problemSlides := 0
	total := 0

	for _, i := range order {
		ffs := bySlide[i]
		slideScore := 100
		// Always non-nil so JSON marshals as [] (not null) when no findings.
		scoreFindings := []ScoreFinding{}

		for _, f := range ffs {
			w := SeverityWeight[f.Action]
			slideScore -= w
			codeCounts[f.Code]++

			scoreFindings = append(scoreFindings, ScoreFinding{
				Code:     f.Code,
				Severity: actionToSeverity(f.Action),
				Message:  f.Message,
				Fix:      f.Fix,
				Class:    patterns.FindingClass(f.Code),
			})
		}

		if slideScore < 0 {
			slideScore = 0
		}
		if slideHasBreadthProblem(ffs) {
			problemSlides++
		}
		total += slideScore

		perSlide = append(perSlide, SlideScore{
			Index:    i,
			Score:    slideScore,
			Findings: scoreFindings,
		})
	}

	overall := 100
	if len(perSlide) > 0 {
		overall = breadthAdjustedScore(total/len(perSlide), problemSlides, len(perSlide))
	}

	topCodes := make([]CodeCount, 0, len(codeCounts))
	for code, count := range codeCounts {
		topCodes = append(topCodes, CodeCount{Code: code, Count: count})
	}
	sort.Slice(topCodes, func(i, j int) bool {
		return topCodes[i].Count > topCodes[j].Count
	})
	if len(topCodes) > 10 {
		topCodes = topCodes[:10]
	}

	return &DeckScore{
		OverallScore: overall,
		Basis:        ScoreBasisStructural,
		PerSlide:     perSlide,
		Summary: DeckSummary{
			TopCodes:           topCodes,
			SlideCount:         slideCount,
			ProblemSlidesCount: problemSlides,
		},
		ModeUsed: "deterministic",
	}
}

// ScoreFromFindings computes a DeckScore from a slice of FitFindings.
// slideCount is the total number of slides in the deck.
func ScoreFromFindings(findings []patterns.FitFinding, slideCount int) *DeckScore {
	// Group findings by slide index.
	bySlide := map[int][]patterns.FitFinding{}
	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		bySlide[si] = append(bySlide[si], f)
	}

	perSlide := make([]SlideScore, slideCount)
	codeCounts := map[string]int{}
	problemSlides := 0

	for i := 0; i < slideCount; i++ {
		ffs := bySlide[i]
		slideScore := 100
		// Always non-nil so JSON marshals as [] (not null) when no findings.
		scoreFindings := []ScoreFinding{}

		for _, f := range ffs {
			w := SeverityWeight[f.Action]
			slideScore -= w
			codeCounts[f.Code]++

			scoreFindings = append(scoreFindings, ScoreFinding{
				Code:     f.Code,
				Severity: actionToSeverity(f.Action),
				Message:  f.Message,
				Fix:      f.Fix,
				Class:    patterns.FindingClass(f.Code),
			})
		}

		if slideScore < 0 {
			slideScore = 0
		}
		if slideHasBreadthProblem(ffs) {
			problemSlides++
		}

		perSlide[i] = SlideScore{
			Index:    i,
			Score:    slideScore,
			Findings: scoreFindings,
		}
	}

	// Compute overall score as the average of per-slide scores, pulled down by
	// the share of slides carrying findings (see breadthAdjustedScore).
	overall := 100
	if slideCount > 0 {
		total := 0
		for _, ss := range perSlide {
			total += ss.Score
		}
		overall = breadthAdjustedScore(total/slideCount, problemSlides, slideCount)
	}

	// Build top_codes sorted by count descending.
	topCodes := make([]CodeCount, 0, len(codeCounts))
	for code, count := range codeCounts {
		topCodes = append(topCodes, CodeCount{Code: code, Count: count})
	}
	sort.Slice(topCodes, func(i, j int) bool {
		return topCodes[i].Count > topCodes[j].Count
	})
	// Cap to top 10.
	if len(topCodes) > 10 {
		topCodes = topCodes[:10]
	}

	return &DeckScore{
		OverallScore: overall,
		Basis:        ScoreBasisStructural,
		PerSlide:     perSlide,
		Summary: DeckSummary{
			TopCodes:           topCodes,
			SlideCount:         slideCount,
			ProblemSlidesCount: problemSlides,
		},
		ModeUsed: "deterministic",
	}
}

// FormatTopCodes returns a concise human-readable summary of the top finding codes.
func FormatTopCodes(codes []CodeCount) string {
	if len(codes) == 0 {
		return "no issues"
	}
	s := ""
	for i, c := range codes {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%s(%d)", c.Code, c.Count)
	}
	return s
}

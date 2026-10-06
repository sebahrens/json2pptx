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

// Some review findings describe visibly broken content, not optional polish.
// Keep the action's default weight for ordinary advisories (for example a
// wrapped title), but let these specific defects move the quality gate.
var substantiveReviewWeight = map[string]int{
	patterns.ErrCodeBodyTooLong:          20,
	patterns.ErrCodeNodeLabelTooLong:     20,
	patterns.ErrCodeTextBelowReadableMin: 20,
	patterns.ErrCodeSlideNearlyEmpty:     20,
	patterns.ErrCodeLowContrastHighlight: 15,
}

// CompositionFaultWeight is what a visible composition fault costs its slide:
// content in one half of the slide over an empty other half, a box whose text
// wraps into a column of fragments, peer headers at two sizes. Each is an
// ordinary review advisory by action — it does not block by itself, a deck
// may carry one — but at review weight (5) a slide with three of them scored
// 85 and a deck of them scored 100, so agents learned to ignore the score
// (go-slide-creator-wwmod). 25 puts a slide carrying one below the gate's
// min_score (80), so a deck of weak slides fails the score floor. Sparseness
// alone (SLIDE_UNDERUSED, SPARSE_FILL) is not in this set: a centred hero
// number is sparse by design.
//
// All four also count toward the problem-slide share, so a deck where the
// fault is widespread fails the gate's share criterion as well. The two
// whitespace faults were held out of the share while content-sized blocks
// hung from the body line by default (the calibration corpus's good deck G2
// carried three); the placement policy now composes such blocks
// (go-slide-creator-yhzxt), so one that still reports is a fault
// (TestCalibrationRanking).
var CompositionFaultWeight = map[string]int{
	patterns.ErrCodeVerticalImbalance:   25,
	patterns.ErrCodeHorizontalImbalance: 25,
	patterns.ErrCodeTextWrapsNarrow:     25,
	patterns.ErrCodeSiblingSizeMismatch: 25,
}

func findingWeight(f patterns.FitFinding) int {
	if f.Action == "review" && substantiveReviewWeight[f.Code] > 0 {
		return substantiveReviewWeight[f.Code]
	}
	if f.Action == "review" && CompositionFaultWeight[f.Code] > 0 {
		return CompositionFaultWeight[f.Code]
	}
	return SeverityWeight[f.Action]
}

// IsSubstantiveReview identifies review findings that must block convergence.
// Predicted autofit can overstate shrinkage on pattern cells, so unreadable
// text blocks only when its effective size is authored or generated.
func IsSubstantiveReview(f patterns.FitFinding) bool {
	if f.Action != "review" || substantiveReviewWeight[f.Code] == 0 {
		return false
	}
	if f.Code != patterns.ErrCodeTextBelowReadableMin {
		return true
	}
	if f.Fix == nil {
		return false
	}
	source, _ := f.Fix.Params["measurement_source"].(string)
	return source == "authored" || source == "generated"
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

// DeckFinding is a finding that belongs to no single slide: its path is not
// under /slides/N (CHROME_TRUNCATED at /chrome, a template synthesis note,
// RENDER_EVIDENCE_INCOMPLETE). It is listed once and costs its weight once,
// on the deck's overall score.
type DeckFinding struct {
	ScoreFinding
	// Path is the finding's JSON Pointer ("/chrome"); empty when it has none.
	Path string `json:"path,omitempty"`
	// Slides are the 0-based slides the finding names (fix.params.slides),
	// the indices per_slide uses; omitted when it names none.
	Slides []int `json:"slides,omitempty"`
	// Points is what the finding costs the overall score.
	Points int `json:"points"`
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
	Basis    string       `json:"basis"`
	PerSlide []SlideScore `json:"per_slide"`
	// DeckFindings lists the findings no slide owns (see DeckFinding). They
	// used to be dropped: a footer line cut on every slide appeared in
	// validate and generate but not here, and cost nothing
	// (go-slide-creator-qhgm8). Omitted when there are none.
	DeckFindings []DeckFinding      `json:"deck_findings,omitempty"`
	Composition  *CompositionResult `json:"composition,omitempty"`
	Summary      DeckSummary        `json:"summary"`
	QualityGate  *QualityGate       `json:"quality_gate,omitempty"`
	ModeUsed     string             `json:"mode_used"`
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
	// MaxTopicTitlePct caps the share of scored slides whose title is not an
	// action title (TITLE_NOT_ACTION: a topic, a stock label or over 15
	// words). The skill states the rule; at info weight a deck of topic titles
	// scored 99 and passed (go-slide-creator-kuurd). A share rather than a
	// count, because the verb test is a heuristic and one misread title must
	// not block a sound deck; at least minTopicTitlesForShare are needed to
	// trip it. 0 disables it. Unsourced data slides (DATA_WITHOUT_SOURCE) are
	// review-weighted in the score but are not a gate criterion: the
	// calibration corpus's good decks carry them.
	MaxTopicTitlePct int `json:"max_topic_title_pct"`
	// RequireStoryline fails the gate on the deck-structure findings
	// NO_EXECUTIVE_SUMMARY and CLOSING_WITHOUT_NEXT_STEPS
	// (go-slide-creator-kuurd).
	RequireStoryline    bool `json:"require_storyline"`
	AllowAccentOverload bool `json:"allow_accent_overload"`
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

// Default thresholds for the score_deck quality gate. auto_repair uses these
// same configurable thresholds by default; substantive review defects are
// fixed blockers for both gates.
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

	// DefaultQualityGateMaxTopicTitlePct is the action-title ceiling: a deck
	// may carry a stray label, not a storyline of them. The review's weak deck
	// had topic titles on 5 of 9 slides (55%); the calibration corpus's good
	// decks have none (go-slide-creator-kuurd).
	DefaultQualityGateMaxTopicTitlePct = 25
	// minTopicTitlesForShare is the fewest topic titles that can trip the
	// action-title criterion, so a two-slide showcase with one label passes.
	minTopicTitlesForShare = 2
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
		MaxTopicTitlePct:        DefaultQualityGateMaxTopicTitlePct,
		RequireStoryline:        true,
		AllowAccentOverload:     false,
		MaxProblemSlidesPct:     DefaultQualityGateMaxProblemSlidesPct,
		MinCompositionScore:     DefaultQualityGateMinCompositionScore,
	}
}

// EvaluateQualityGate computes a QualityGate verdict against the given score
// and findings using the supplied criteria. Reason order is deterministic:
// score → P0 → P1 → substantive review → takeaway → action titles → storyline
// → accent_overload → composition → problem-slide share, so agents can
// pattern-match on the leading reason.
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

	var p0, p1, substantiveReviews, takeawayMissing, accentOverload int
	var topicTitles int
	storyline := map[string]int{}
	for _, f := range findings {
		switch f.Action {
		case "refuse":
			p0++
		case "shrink_or_split":
			p1++
		}
		if IsSubstantiveReview(f) {
			substantiveReviews++
		}
		switch f.Code {
		case patterns.ErrCodeTakeawayMissing:
			takeawayMissing++
		case patterns.ErrCodeAccentOverload:
			accentOverload++
		case patterns.ErrCodeTitleNotAction:
			topicTitles++
		case patterns.ErrCodeNoExecutiveSummary, patterns.ErrCodeClosingWithoutNextSteps:
			storyline[f.Code]++
		}
	}

	if p0 > criteria.MaxP0Findings {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d P0 (refuse) finding(s) exceeds max_p0_findings %d%s", p0, criteria.MaxP0Findings, gateActionCodes(findings, "refuse")))
	}
	if p1 > criteria.MaxP1Findings {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d P1 (shrink_or_split) finding(s) exceeds max_p1_findings %d%s", p1, criteria.MaxP1Findings, gateActionCodes(findings, "shrink_or_split")))
	}
	if substantiveReviews > 0 {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d substantive review finding(s) remain (readability, empty content, or contrast)", substantiveReviews))
	}
	if criteria.RequireTakeawayOnCharts && takeawayMissing > 0 {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d chart/matrix slide(s) missing takeaway (require_takeaway_on_charts)", takeawayMissing))
	}
	gate.Reasons = append(gate.Reasons, storylineGateReasons(ds, criteria, topicTitles, storyline)...)
	if !criteria.AllowAccentOverload && accentOverload > 0 {
		gate.Reasons = append(gate.Reasons, fmt.Sprintf("%d slide(s) emit accent_overload (too many distinct accents)", accentOverload))
	}
	if reason := compositionGateReason(ds, criteria); reason != "" {
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

// BlocksGate reports whether one finding fails the quality gate by itself
// under criteria: a refuse or shrink_or_split action, a substantive review
// defect, a chart or matrix without its takeaway, a storyline gap, or accent
// overload. It is the per-finding half of EvaluateQualityGate, exported so a
// surface can give exactly these findings the blocking severity instead of
// leaving an "info" to stop the gate (go-slide-creator-x9rhq). The aggregate
// criteria (score floor, action-title share, composition floor, problem-slide
// share) have no single finding behind them and stay with the gate's reasons.
func BlocksGate(f patterns.FitFinding, criteria QualityGateCriteria) bool {
	switch f.Action {
	case "refuse":
		return criteria.MaxP0Findings <= 0
	case "shrink_or_split":
		return criteria.MaxP1Findings <= 0
	}
	if IsSubstantiveReview(f) {
		return true
	}
	switch f.Code {
	case patterns.ErrCodeTakeawayMissing:
		return criteria.RequireTakeawayOnCharts
	case patterns.ErrCodeAccentOverload:
		return !criteria.AllowAccentOverload
	case patterns.ErrCodeNoExecutiveSummary, patterns.ErrCodeClosingWithoutNextSteps:
		return criteria.RequireStoryline
	}
	return false
}

// Blocking profiles: whether a finding code blocks the gate whenever it is
// emitted, only under a stated condition, or never.
const (
	BlocksAlways    = "always"
	BlocksSometimes = "sometimes"
	BlocksNever     = "never"
)

// BlockingProfile describes, for one finding code and the action it declares
// by default, whether the default quality gate blocks on it and when. It is
// the static description of BlocksGate that describe_finding reports, so a
// code's documentation and the severity an agent sees on a finding are derived
// from the same rule (go-slide-creator-x9rhq).
func BlockingProfile(code, declaredAction string) (blocks, when string) {
	switch declaredAction {
	case "refuse", "shrink_or_split":
		return BlocksAlways, "Always blocks: reported with severity error and blocking:true."
	}
	switch code {
	case patterns.ErrCodeNoExecutiveSummary:
		return BlocksSometimes, "Blocks (severity error) unless waived: meta.waivers names it with a reason, or meta.archetype is one that does not call for an executive summary (sales_pitch, project_roadmap, market_analysis). Waived, it is an info."
	case patterns.ErrCodeClosingWithoutNextSteps, patterns.ErrCodeTakeawayMissing:
		return BlocksSometimes, "Blocks (severity error) unless meta.waivers names it with a reason; waived, it is an info."
	case patterns.ErrCodeAccentOverload:
		return BlocksAlways, "Always blocks: reported with severity error and blocking:true."
	case patterns.ErrCodeTextBelowReadableMin:
		return BlocksSometimes, "Blocks (severity error) when the size was measured on authored or generated text, or generation refused it; a predicted shrink is an info."
	case "SEMANTIC_WEAK_CONTENT":
		return BlocksSometimes, "Blocks (severity error) when the text is placeholder copy the product itself emitted: a plan draft's __FILL__, a recommend_visual recipe's title, alt text or sample source, a suggested patch's \"<…>\" hint. Authored filler (TBD, lorem ipsum) is a warning."
	case "SEMANTIC_PATTERN_DEGRADED", "SEMANTIC_DENSITY":
		return BlocksSometimes, "A warning on its own. Blocks (severity error) when what the slide renders as instead does not fit: the fit findings about that fallback are listed under symptoms."
	case patterns.ErrCodeTitleNotAction:
		return BlocksSometimes, "An info on its own slide. When topic titles exceed max_topic_title_pct of the deck the gate fails and one QUALITY_GATE error reports it, unless meta.waivers names TITLE_NOT_ACTION."
	}
	if substantiveReviewWeight[code] > 0 {
		return BlocksSometimes, "Blocks (severity error) when emitted with action review, shrink_or_split or refuse; an instance emitted as info is an advisory."
	}
	return BlocksNever, "Advisory: never blocks by itself. Review-level advisories cost score points; a deck whose score or share of affected slides fails the gate reports one QUALITY_GATE error naming them."
}

// AggregateGateReasons returns the gate reasons that no single finding
// accounts for: the score floor, the action-title share, the composition floor
// and the problem-slide share. Every other reason EvaluateQualityGate gives is
// the sum of findings BlocksGate reports.
func AggregateGateReasons(gate *QualityGate) []string {
	if gate == nil {
		return nil
	}
	var out []string
	for _, r := range gate.Reasons {
		switch {
		case strings.HasPrefix(r, "score "),
			strings.HasPrefix(r, "composition "),
			strings.Contains(r, "lack an action title"),
			strings.Contains(r, "slides carry findings"):
			out = append(out, r)
		}
	}
	return out
}

// compositionGateReason applies the deck-rhythm floor. Only when the score was
// computed over the whole deck — ds.Composition is nil on the slide_indices
// path, where a rhythm verdict would be meaningless.
func compositionGateReason(ds *DeckScore, criteria QualityGateCriteria) string {
	if criteria.MinCompositionScore <= 0 || ds.Composition == nil || ds.Composition.Score >= criteria.MinCompositionScore {
		return ""
	}
	reason := fmt.Sprintf("composition %d < min_composition_score %d", ds.Composition.Score, criteria.MinCompositionScore)
	if codes := compositionCodes(ds.Composition); codes != "" {
		reason += " (" + codes + ")"
	}
	return reason
}

// storylineGateReasons applies the action-title share and require_storyline
// criteria (go-slide-creator-kuurd).
func storylineGateReasons(ds *DeckScore, criteria QualityGateCriteria, topicTitles int, storyline map[string]int) []string {
	var reasons []string
	if criteria.MaxTopicTitlePct > 0 && len(ds.PerSlide) > 0 && topicTitles >= minTopicTitlesForShare {
		if pct := topicTitles * 100 / len(ds.PerSlide); pct > criteria.MaxTopicTitlePct {
			reasons = append(reasons, fmt.Sprintf("%d of %d slides lack an action title (%d%%, TITLE_NOT_ACTION) — exceeds max_topic_title_pct %d",
				topicTitles, len(ds.PerSlide), pct, criteria.MaxTopicTitlePct))
		}
	}
	if criteria.RequireStoryline && len(storyline) > 0 {
		codes := make([]string, 0, len(storyline))
		for code := range storyline {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		reasons = append(reasons, fmt.Sprintf("deck storyline incomplete (require_storyline: %s)", strings.Join(codes, ", ")))
	}
	return reasons
}

// gateActionCodes names the distinct findings behind an action-based gate
// reason, so the reason can be matched to an actionable diagnostic. Sorting
// keeps the explanation stable regardless of finding collection order.
func gateActionCodes(findings []patterns.FitFinding, action string) string {
	codes := make(map[string]bool)
	for _, finding := range findings {
		if finding.Action == action && finding.Code != "" {
			codes[finding.Code] = true
		}
	}
	if len(codes) == 0 {
		return ""
	}
	names := make([]string, 0, len(codes))
	for code := range codes {
		names = append(names, code)
	}
	sort.Strings(names)
	return " (codes: " + strings.Join(names, ", ") + ")"
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
	// Info-severity advisories that describe polish or deck-level rhythm, not a
	// broken slide. Each is still reported and still costs its per-slide points,
	// but counting them made an honest 28-slide deck read as "57% problem
	// slides" and fail max_problem_slides_pct on advice alone
	// (go-slide-creator-fabz4). accent_overload and DECK_MONOTONY keep their own
	// gate criteria (allow_accent_overload, min_composition_score, refuse at 6+);
	// DUPLICATE_TITLE feeds the composition score.
	patterns.ErrCodeMissingAltText:     true,
	patterns.ErrCodeAccentOverload:     true,
	patterns.ErrCodeDuplicateTitle:     true,
	patterns.ErrCodeDeckMonotony:       true,
	patterns.ErrCodeTableFontScaled:    true,
	patterns.ErrCodeChartShapeInferred: true,
	patterns.ErrCodeTitleNotAction:     true,
	patterns.ErrCodeTitleTooLong:       true,
	// Storyline findings carry their own gate criteria (require_action_titles,
	// require_sources, require_takeaway_on_charts, require_storyline); counting
	// them toward the share too would charge one defect twice and collapse the
	// score of an otherwise sound deck (go-slide-creator-kuurd).
	patterns.ErrCodeDataWithoutSource:       true,
	patterns.ErrCodeTakeawayMissing:         true,
	patterns.ErrCodeNoExecutiveSummary:      true,
	patterns.ErrCodeClosingWithoutNextSteps: true,
	// A few lines at the top of a body placeholder is airiness
	// (go-slide-creator-u9xfy). The two imbalance codes are not in this set:
	// with the placement policy composing sparse blocks
	// (go-slide-creator-yhzxt), content left in one half of the slide is a
	// composition fault like the others in CompositionFaultWeight and counts
	// toward the problem-slide share (go-slide-creator-wwmod).
	patterns.ErrCodeSparsePlaceholder: true,
}

// isBreadthProblem reports whether a finding makes its slide count as a problem
// slide.
func isBreadthProblem(f patterns.FitFinding) bool {
	if findingWeight(f) <= 0 {
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

// namedSlides returns the 0-based slides a finding names in
// fix.params.slides, which carries 1-based slide numbers ([]int from the
// engine, []any of float64 after a JSON round trip).
func namedSlides(f patterns.FitFinding) []int {
	if f.Fix == nil {
		return nil
	}
	var out []int
	add := func(n float64) {
		if n >= 1 && n == math.Trunc(n) {
			out = append(out, int(n)-1)
		}
	}
	switch v := f.Fix.Params["slides"].(type) {
	case []int:
		for _, n := range v {
			add(float64(n))
		}
	case []float64:
		for _, n := range v {
			add(n)
		}
	case []any:
		for _, e := range v {
			switch n := e.(type) {
			case float64:
				add(n)
			case int:
				add(float64(n))
			}
		}
	}
	return out
}

// groupFindings sorts findings into the slides that own them and the
// deck-level rest. A finding whose path is under /slides/N belongs to slide N.
// One whose path is not, and which names exactly one slide in
// fix.params.slides, belongs to that slide. Any other is deck-level: a footer
// line cut on twelve slides is one fact about the deck, and attributed to each
// slide it would make every slide a problem slide and fail the gate's share
// criterion on a single advisory. included, when set, limits the result to
// those slides: a finding of an excluded slide is dropped, and a deck-level
// finding that names slides is kept only when it names an included one.
// Findings that repeat are listed, and charged, once, on a slide as in the
// deck: preflight and render report the same footer line, and the same
// diagram note at the field and at the content item that holds it
// (patterns.DedupeFindings).
func groupFindings(findings []patterns.FitFinding, slideCount int, included map[int]bool) (bySlide map[int][]patterns.FitFinding, deck []patterns.FitFinding) {
	bySlide = map[int][]patterns.FitFinding{}
	in := func(i int) bool {
		return i >= 0 && i < slideCount && (included == nil || included[i])
	}
	for _, f := range patterns.DedupeFindings(findings) {
		if si := slidepath.SlideIndex(f.Path); si >= 0 {
			if in(si) {
				bySlide[si] = append(bySlide[si], f)
			}
			continue
		}
		named := namedSlides(f)
		if len(named) == 1 {
			if in(named[0]) {
				bySlide[named[0]] = append(bySlide[named[0]], f)
			}
			continue
		}
		if len(named) > 0 && included != nil {
			any := false
			for _, n := range named {
				any = any || in(n)
			}
			if !any {
				continue
			}
		}
		deck = append(deck, f)
	}
	return bySlide, deck
}

// applyDeckFindings lists the deck-level findings on ds, takes each one's
// weight off the overall score and counts its code.
func applyDeckFindings(ds *DeckScore, deck []patterns.FitFinding, slideCount int, codeCounts map[string]int) {
	for _, f := range deck {
		w := findingWeight(f)
		ds.OverallScore -= w
		codeCounts[f.Code]++
		var slides []int
		for _, n := range namedSlides(f) {
			if n < slideCount {
				slides = append(slides, n)
			}
		}
		ds.DeckFindings = append(ds.DeckFindings, DeckFinding{
			ScoreFinding: ScoreFinding{
				Code:     f.Code,
				Severity: actionToSeverity(f.Action),
				Message:  f.Message,
				Fix:      f.Fix,
				Class:    patterns.FindingClass(f.Code),
			},
			Path:   f.Path,
			Slides: slides,
			Points: w,
		})
	}
	if ds.OverallScore < 0 {
		ds.OverallScore = 0
	}
}

// topCodesOf returns the ten most frequent codes, most frequent first (ties
// by code, so the order is stable).
func topCodesOf(codeCounts map[string]int) []CodeCount {
	topCodes := make([]CodeCount, 0, len(codeCounts))
	for code, count := range codeCounts {
		topCodes = append(topCodes, CodeCount{Code: code, Count: count})
	}
	sort.Slice(topCodes, func(i, j int) bool {
		if topCodes[i].Count != topCodes[j].Count {
			return topCodes[i].Count > topCodes[j].Count
		}
		return topCodes[i].Code < topCodes[j].Code
	})
	if len(topCodes) > 10 {
		topCodes = topCodes[:10]
	}
	return topCodes
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

	// Group findings by slide, keeping only the included slides' and the
	// deck-level ones that concern them.
	bySlide, deck := groupFindings(findings, slideCount, seen)

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
			w := findingWeight(f)
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

	ds := &DeckScore{
		OverallScore: overall,
		Basis:        ScoreBasisStructural,
		PerSlide:     perSlide,
		ModeUsed:     "deterministic",
	}
	applyDeckFindings(ds, deck, slideCount, codeCounts)
	ds.Summary = DeckSummary{
		TopCodes:           topCodesOf(codeCounts),
		SlideCount:         slideCount,
		ProblemSlidesCount: problemSlides,
	}
	return ds
}

// ScoreFromFindings computes a DeckScore from a slice of FitFindings.
// slideCount is the total number of slides in the deck.
func ScoreFromFindings(findings []patterns.FitFinding, slideCount int) *DeckScore {
	// Group findings by slide; the rest are deck-level.
	bySlide, deck := groupFindings(findings, slideCount, nil)

	perSlide := make([]SlideScore, slideCount)
	codeCounts := map[string]int{}
	problemSlides := 0

	for i := 0; i < slideCount; i++ {
		ffs := bySlide[i]
		slideScore := 100
		// Always non-nil so JSON marshals as [] (not null) when no findings.
		scoreFindings := []ScoreFinding{}

		for _, f := range ffs {
			w := findingWeight(f)
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

	ds := &DeckScore{
		OverallScore: overall,
		Basis:        ScoreBasisStructural,
		PerSlide:     perSlide,
		ModeUsed:     "deterministic",
	}
	applyDeckFindings(ds, deck, slideCount, codeCounts)
	ds.Summary = DeckSummary{
		TopCodes:           topCodesOf(codeCounts),
		SlideCount:         slideCount,
		ProblemSlidesCount: problemSlides,
	}
	return ds
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

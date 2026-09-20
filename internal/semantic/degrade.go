package semantic

import "github.com/sebahrens/json2pptx/internal/diagnostics"

// Degrade advisories (go-slide-creator-kjc8l).
//
// Eight slide kinds report the same thing — "you asked for a visual, you are
// getting bullets" — and they all used to report it as SEMANTIC_DENSITY,
// sharing the code with plain count-range advice. That is a misnomer for the
// budget cases (a KPI value twenty characters too long is not a density
// problem) and it left an agent parsing prose to tell a fallback from a
// suggestion. SEMANTIC_PATTERN_DEGRADED says it in the code; the fix params say
// it in a form an agent can branch on without reading the message.
//
// SEMANTIC_DENSITY keeps the advisories that only recommend a count range
// without changing what the slide renders as.

// Degrade targets — what the slide renders as once the visual is refused.
const (
	degradeToBullets      = "content-bullets"
	degradeToContent      = "content-slide"
	degradeToTwoColumn    = "native-two-column"
	degradeToInsightsOnly = "insights-only"
)

// Degrade reasons — why the visual was refused. Coarser than the message on
// purpose: these are the branches an agent can act on.
const (
	degradeCountOutOfRange   = "count_out_of_range"
	degradeBudgetExceeded    = "budget_exceeded"
	degradeColumnsUnbalanced = "columns_unbalanced"
	degradeScoresIncomplete  = "scores_incomplete"
	degradeScoreUnreadable   = "score_unreadable"
	degradeChartDataMissing  = "chart_data_missing"
)

// degrade reports an advisory that announces a fallback: the slide still
// renders, but as something weaker than the visual its kind promised. from is
// the pattern that was refused (empty when the kind has not committed to one),
// to is what the compiler emits instead, and reason is why.
func (s *semDiags) degrade(path, msg, from, to, reason string) {
	s.degradeFix(path, msg, &diagnostics.Fix{Kind: "restore_visual"}, from, to, reason)
}

// degradeFix is degrade with a caller-supplied fix, for the sites that have a
// better remedy to offer than "decide what to cut" — a chart with no data wants
// provide_value, not a restore_visual nudge. The from/to/reason params are
// merged into whatever the fix already carries, so every finding on this code
// answers the same three questions however it was raised.
func (s *semDiags) degradeFix(path, msg string, fix *diagnostics.Fix, from, to, reason string) {
	if fix.Params == nil {
		fix.Params = make(map[string]any, 3)
	}
	fix.Params["from"] = from
	fix.Params["to"] = to
	fix.Params["reason"] = reason
	s.advisoryFix(path, diagnostics.CodeSemanticPatternDegraded, msg, fix)
}

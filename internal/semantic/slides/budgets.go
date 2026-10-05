package slides

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Budget is one authored field's fixed text budget: the limit the compiler and
// validation hold the field to before the slide degrades or is refused. It is
// the same on every template; the budgets that depend on the template (title,
// takeaway) are measured by the caller (go-slide-creator-iubjb).
type Budget struct {
	// Field is the payload path under the slide; "[]" marks a list entry
	// ("options[].label") and "+" joins fields that share one budget.
	Field string
	// MaxChars is the most characters the field holds, 0 when it has none.
	MaxChars int
	// MinItems / MaxItems bound a list field, 0 when unbounded.
	MinItems, MaxItems int
	// Note says when the budget applies, for a kind with more than one visual
	// or a budget that tightens with the item count: MaxChars is then the most
	// the field ever holds and the note states the tighter lengths, which are
	// the ones a finding quotes for a slide that has them.
	Note string
}

// kindBudgets is the fixed budget table, keyed by slide kind. Every number is
// the constant the compiler enforces, so the catalogue cannot drift from it;
// TestKindBudgetsAgreeWithSchemaAndFindings holds the schema descriptions and
// the findings to the same numbers.
var kindBudgets = map[string][]Budget{
	"executive_summary": {
		{Field: "points", MinItems: execSummaryMinPoints, MaxItems: execSummaryMaxPoints},
		{Field: "points[].lead", MaxChars: patterns.ExecSummaryLeadMax, Note: "3 points, or 4 without a bottom line; 57 with 4 points and a bottom line, 52 with 5 points"},
		{Field: "points[].support", MaxChars: patterns.ExecSummarySupportMax, Note: "3 points and no bottom line; 193 beside a bottom line (182 when it is over 40 characters), 128 with 4 points (65 with a bottom line), 60 with 5 points"},
		{Field: "bottom_line", MaxChars: patterns.ExecSummaryBottomLineMax, Note: "3 points; 102 with 4 or 5 points"},
	},
	"kpi_snapshot": {
		{Field: "kpis", MinItems: 2, MaxItems: 6},
		{Field: "kpis[].value", MaxChars: patterns.KPIValueMax, Note: fmt.Sprintf("2–4 KPIs; on one line a value holds about %d digits with 5 KPIs and %d with 6 on the narrowest shipped template (measured per template)", patterns.KPIValueFiveMax, patterns.KPIValueSixMax)},
		{Field: "kpis[].delta", MaxChars: patterns.KPIDeltaMax},
		{Field: "kpis[].comparator", MaxChars: patterns.KPIComparatorMax},
	},
	"chart_insight": {
		{Field: "insights", MinItems: 1, MaxItems: ChartInsightMaxInsights},
	},
	"option_matrix": {
		{Field: "criteria", MinItems: optionMatrixMinCriteria, MaxItems: optionMatrixMaxCriteria},
		{Field: "options", MinItems: optionMatrixMinOptions, MaxItems: optionMatrixMaxOptions},
		{Field: "options[].detail", MaxChars: patterns.TableHighlightDetailLimit(optionMatrixMinOptions, optionMatrixMinCriteria), Note: "up to 4 options × 4 criteria; 60 in a denser matrix"},
		{Field: "highlight_label", MaxChars: optionMatrixLabelMax},
		{Field: "corner_label", MaxChars: optionMatrixLabelMax},
	},
	"architecture": {
		{Field: "tiers", MinItems: archStackMinTiers, MaxItems: archStackMaxTiers},
		{Field: "tiers[].label", MaxChars: archStackLabelMax},
		{Field: "tiers[].description", MaxChars: archStackDescMax},
		{Field: "rails", MaxItems: archStackMaxRails},
		{Field: "rails[]", MaxChars: archStackRailMax},
	},
	"agenda": {
		{Field: "sections", MinItems: agendaMinItems, MaxItems: agendaMaxItems},
		{Field: "sections[].title", MaxChars: agendaItemMax},
		{Field: "sections[].subtitle", MaxChars: agendaSubtitleMax, Note: "120 in the numbered list; a longer one uses agenda-with-images rows (3–6 sections, title ≤80)"},
	},
	"bridge": {
		{Field: "columns", MinItems: bridgeMinColumns, MaxItems: bridgeMaxColumns},
		{Field: "columns[].label", MaxChars: bridgeLabelMax},
		{Field: "unit", MaxChars: bridgeUnitMax},
		{Field: "caption", MaxChars: bridgeCaptionMax},
	},
	"team": {
		{Field: "members", MinItems: 1, MaxItems: teamMaxMembers},
		{Field: "members[].name", MaxChars: teamNameMax},
		{Field: "members[].role", MaxChars: teamRoleMax},
		{Field: "members[].bio", MaxChars: teamBioMax},
	},
	"stat": {
		{Field: "value", MaxChars: statValueMax},
		{Field: "unit", MaxChars: statUnitMax},
		{Field: "label", MaxChars: statLabelMax},
		{Field: "context", MaxChars: statContextMax},
		{Field: "source", MaxChars: statSourceMax},
		{Field: "label+context+source", MaxChars: patterns.StatHeroStackBudget, Note: "combined: the stack beneath the number"},
	},
	"timeline": {
		{Field: "milestones", MinItems: timelineMinStops, MaxItems: timelineMaxStops},
		{Field: "milestones[].label", MaxChars: timelineLabelMax},
		{Field: "milestones[].date", MaxChars: timelineDateMax},
		{Field: "milestones[].body", MaxChars: timelineBodyMax},
	},
	"matrix_2x2": {
		{Field: "quadrants", MinItems: 4, MaxItems: 4},
		{Field: "quadrants[].header", MaxChars: matrixHeaderMax},
		{Field: "quadrants[].body", MaxChars: matrixBodyMax},
		{Field: "x_axis", MaxChars: matrixXAxisMax},
		{Field: "y_axis", MaxChars: matrixYAxisMax},
		{Field: "x_low", MaxChars: matrixAxisEndMax},
		{Field: "x_high", MaxChars: matrixAxisEndMax},
		{Field: "y_low", MaxChars: matrixAxisEndMax},
		{Field: "y_high", MaxChars: matrixAxisEndMax},
	},
	"framework": {
		{Field: "sections.<part>", MaxItems: frameworkSectionItemsMax},
		{Field: "sections.<part>[]", MaxChars: frameworkItemMax},
	},
	"image_case": {
		{Field: "eyebrow", MaxChars: imageCaseEyebrowMax},
		{Field: "heading", MaxChars: imageCaseHeadingMax},
		{Field: "body", MaxChars: imageCaseBodyMax},
		{Field: "bullets", MaxItems: imageCaseMaxBullets},
		{Field: "bullets[]", MaxChars: imageCaseBulletMax},
		{Field: "metrics", MaxItems: imageCaseMaxMetrics},
		{Field: "metrics[].value", MaxChars: imageCaseMetricValueMax},
		{Field: "metrics[].label", MaxChars: imageCaseMetricLabelMax},
		{Field: "caption", MaxChars: imageCaseCaptionMax},
		{Field: "image_label", MaxChars: imageCaseLabelMax},
		{Field: "callouts", MaxItems: ImageCaseMaxCallouts},
		{Field: "callouts[].label", MaxChars: ImageCaseCalloutLabelMax},
	},
	"roadmap": {
		{Field: "phases", MinItems: roadmapMinPhases, MaxItems: roadmapMaxPhases},
		{Field: "phases[].name", MaxChars: patterns.PhaseRoadmapNameMax},
		{Field: "phases[].date_label", MaxChars: patterns.PhaseRoadmapDateLabelMax},
		{Field: "phases[].description", MaxChars: patterns.PhaseRoadmapDescriptionMax, Note: "together with the phase's items, which render as bullets under it"},
		{Field: "phases[].milestone", MaxChars: patterns.PhaseRoadmapMilestoneMax},
		{Field: "parallel_tracks", MaxItems: patterns.PhaseRoadmapMaxTracks},
		{Field: "parallel_tracks[]", MaxChars: patterns.PhaseRoadmapTrackMax},
		{Field: "parallel_label", MaxChars: patterns.PhaseRoadmapLabelMax},
	},
	"process": {
		{Field: "steps", MinItems: processStripMin, MaxItems: processStripMax, Note: "numbered rows (steps with a description); the flow diagram takes 3–8 bare labels"},
		{Field: "steps[].label", MaxChars: processStripLabelMax, Note: "numbered rows; a flow box holds 80 for label and description together (7–8 steps are on two rows)"},
		{Field: "steps[].description", MaxChars: processStripBodyMax, Note: "numbered rows"},
	},
	"decision": {
		{Field: "options", MinItems: decisionMinSteps, MaxItems: decisionMaxSteps, Note: "numbered boxes; 2 or 7–12 options, each with a detail, become cards"},
		{Field: "options[].label", MaxChars: decisionLabelMax, Note: "numbered boxes; a card holds 80"},
		{Field: "options[].detail", MaxChars: decisionDetailMax, Note: "numbered boxes; a card holds 300 in a pair, 160 in a grid of 7–12"},
	},
	"next_steps": {
		{Field: "actions", MinItems: 2, MaxItems: 6},
		{Field: "actions[].action", MaxChars: patterns.NextStepsActionMax},
		{Field: "actions[].owner", MaxChars: patterns.NextStepsOwnerMax},
		{Field: "actions[].date", MaxChars: patterns.NextStepsDateMax},
		{Field: "decisions", MaxItems: 3},
		{Field: "decisions[]", MaxChars: patterns.NextStepsDecisionMax},
	},
}

// Budgets returns the fixed text budgets of a slide kind, or nil when the kind
// states none.
func Budgets(kind string) []Budget {
	return append([]Budget(nil), kindBudgets[kind]...)
}

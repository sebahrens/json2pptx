package core

// Finding codes for structured render-time findings emitted by svggen.
// All finding codes live in this single file for audit and telemetry.
//
// Each code follows the pattern "chart.<specific_issue>". Findings carry
// a FixSuggestion with Kind and Params that agents can act on.
const (
	// FindingInvalidNumeric is emitted when NaN, Inf, or values exceeding
	// the safe range are clamped to finite values during rendering.
	FindingInvalidNumeric = "chart.invalid_numeric"

	// FindingZeroSumPie is emitted when a pie/donut chart has all-zero or
	// all-negative values, producing a blank or misleading chart.
	FindingZeroSumPie = "chart.zero_sum_pie"

	// FindingNegativeOnLog is emitted when negative values appear in a
	// log-scale chart and are silently dropped or clamped.
	FindingNegativeOnLog = "chart.negative_on_log"

	// FindingAllZeroSeries is emitted when all series values are zero,
	// producing a flat/blank chart.
	FindingAllZeroSeries = "chart.all_zero_series"

	// FindingCapacityExceeded is emitted when the number of series, points,
	// or categories exceeds the renderer's safe capacity limit.
	FindingCapacityExceeded = "chart.capacity_exceeded"

	// FindingInvalidTimeFormat is emitted when a time-series string cannot
	// be parsed as a valid time value.
	FindingInvalidTimeFormat = "chart.invalid_time_format"

	// FindingAutoLogScaleApplied is emitted when the renderer automatically
	// switches to log scale based on data range.
	FindingAutoLogScaleApplied = "chart.auto_log_scale_applied"

	// FindingTickThinned is emitted when axis tick labels are thinned
	// (skipped) to prevent overlap.
	FindingTickThinned = "chart.tick_thinned"

	// FindingScatterLabelSkipped is emitted when a scatter plot label is
	// skipped due to collision with another label.
	FindingScatterLabelSkipped = "chart.scatter_label_skipped"

	// FindingLabelTruncated is emitted when a label is truncated to fit
	// the available space.
	FindingLabelTruncated = "chart.label_truncated"

	// FindingLabelEllipsized is emitted when a label is shortened with
	// an ellipsis to fit the available space.
	FindingLabelEllipsized = "chart.label_ellipsized"

	// FindingLabelClipped is emitted when a label is hard-clipped at the
	// boundary of its container.
	FindingLabelClipped = "chart.label_clipped"

	// FindingLegendOverflowDropped is emitted when legend entries are
	// dropped because they exceed the available legend area.
	FindingLegendOverflowDropped = "chart.legend_overflow_dropped"

	// FindingOverflowSuppressed is emitted when overflow content (e.g.
	// "+N more" indicators) is suppressed or truncated.
	FindingOverflowSuppressed = "chart.overflow_suppressed"
)

// FixKind constants for the Kind field of FixSuggestion.
const (
	FixKindReplaceValue    = "replace_value"
	FixKindTruncateOrSplit = "truncate_or_split"
	FixKindAlignSeries     = "align_series"
	FixKindExplicitScale   = "explicit_scale"
	FixKindReduceItems     = "reduce_items"
	FixKindIncreaseCanvas  = "increase_canvas"
)

// FixSuggestion is a structured remediation hint attached to a finding.
// Kind identifies the class of fix; Params carries kind-specific data
// (e.g. {"field": "data.values[0]", "original": NaN, "clamped_to": 0}).
type FixSuggestion struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params,omitempty"`
}

// Finding is a structured render-time diagnostic. It extends ValidationError
// with a Severity level and an optional FixSuggestion for agent consumption.
type Finding struct {
	// Field is the JSON path to the relevant data (e.g., "data.values[2]").
	Field string `json:"field,omitempty"`

	// Code is a machine-readable finding code (e.g., "chart.invalid_numeric").
	Code string `json:"code"`

	// Message is a human-readable description of the finding.
	Message string `json:"message"`

	// Severity is "warning" by default; promoted to "error" by strict-fit.
	Severity string `json:"severity"`

	// Fix is an optional structured remediation suggestion.
	Fix *FixSuggestion `json:"fix,omitempty"`
}

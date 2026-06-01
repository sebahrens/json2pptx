package patterns

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Error codes for structured validation errors.
const (
	ErrCodeRequired            = "required"
	ErrCodeMaxLength           = "max_length"
	ErrCodeOutOfRange          = "out_of_range"
	ErrCodeCountMismatch       = "count_mismatch"
	ErrCodeUnknownKey          = "unknown_key"
	ErrCodeMinItems            = "min_items"
	ErrCodeMaxItems            = "max_items"
	ErrCodeEmptyValue          = "empty_value"
	ErrCodeHexFillNonBrand     = "hex_fill_non_brand"
	ErrCodeUnknownLayoutID     = "unknown_layout_id"
	ErrCodeCalloutUnsupported  = "callout_unsupported"
	ErrCodeUnknownEnum         = "UNKNOWN_ENUM"
	ErrCodePlaceholderNotFound = "placeholder_not_found"
	ErrCodeUnknownTableStyleID = "unknown_table_style_id"
	ErrCodeWrongPattern        = "wrong_pattern"
	ErrCodeInvalidShape        = "invalid_shape"

	// Fit-report error codes.
	ErrCodeFitOverflow         = "fit_overflow"
	ErrCodeDensityExceeded     = "density_exceeded"
	ErrCodeStackedTables       = "stacked_tables"
	ErrCodeDividerTooThin      = "divider_too_thin"
	ErrCodeMixedFillScheme     = "mixed_fill_scheme"
	ErrCodePlaceholderOverflow = "placeholder_overflow"
	ErrCodeSlideBoundsOverflow = "slide_bounds_overflow"
	ErrCodeFooterCollision     = "footer_collision"
	ErrCodeTitleCollision      = "title_collision"
	ErrCodeTitleWraps          = "title_wraps"
	ErrCodeSparseLayout        = "sparse_layout"
	ErrCodePatternUnderfilled  = "pattern_underfilled"
	ErrCodePatternOvercrowded  = "pattern_overcrowded"
	ErrCodeCellUnderfilled     = "cell_underfilled"
	ErrCodeTakeawayMissing     = "takeaway_missing"
	ErrCodeAccentOverload      = "accent_overload"

	// Layout-guard code — emitted when a single-row sequence pattern
	// (process-flow / timeline-horizontal "dots") with sparse per-cell text is
	// left to fill the whole slide (no bounds / max_height_pct cap), so its
	// boxes stretch vertically. Advisory; never blocks render.
	ErrCodeSparseSingleRowFlow = "SPARSE_SINGLE_ROW_FLOW"

	// Pattern-choice QA heuristics (J2P-VQA-009) — visual-quality smells that a
	// rendered deck review caught but static validation passed. They flag a
	// "poor pattern choice" rather than a rendering bug (see FindingClass).
	// All advisory (action "review"); none block render.
	//
	//   - OvertallFlowLane: a single-row process-flow / timeline-horizontal lane
	//     occupies more than half the content height with short labels, in cases
	//     SPARSE_SINGLE_ROW_FLOW does not cover (a height cap that is still too
	//     tall, or a 7–8 step row whose boxes still stretch vertically).
	//   - FlowDiamondNoContent: a standalone process-flow carries a decision
	//     diamond but has no supporting content zone to explain the branch.
	//   - TocFlowchartVocab: an agenda / table-of-contents slide is rendered with
	//     sequential flowchart vocabulary (process-flow / swimlane / timeline)
	//     instead of a list / agenda layout.
	ErrCodeOvertallFlowLane     = "OVERTALL_FLOW_LANE"
	ErrCodeFlowDiamondNoContent = "FLOW_DIAMOND_NO_CONTENT"
	ErrCodeTocFlowchartVocab    = "TOC_FLOWCHART_VOCAB"

	// Rendering-geometry QA heuristic (J2P-VQA-009) — a rotated text band (an
	// axis label rotated ~90°/270°) that spans rows/columns renders wide-short
	// or tall-narrow and intrudes into the adjacent quadrants/cells. This is a
	// rendering-geometry smell (see FindingClass → "rendering"), the J2P-MATRIX-005
	// anti-pattern; matrix-2x2 now uses vert270 text in an unrotated band, so the
	// check guards against regressions and hand-authored rotated bands. Advisory.
	ErrCodeMatrixAxisImbalance = "MATRIX_AXIS_IMBALANCE"

	// Content lint codes — emitted when slide text exceeds readability budgets
	// or bullet lists nest more than two levels. Advisory; never block render.
	ErrCodeHeadlineTooLong   = "HEADLINE_TOO_LONG"
	ErrCodeBodyTooLong       = "BODY_TOO_LONG"
	ErrCodeBulletNestingDeep = "BULLET_NESTING_DEEP"
	ErrCodeMissingAltText    = "MISSING_ALT_TEXT"
	ErrCodeDuplicateTitle    = "DUPLICATE_TITLE"

	// Shared content-drop diagnostic — emitted from any path that fails to place
	// author-provided content (a dropped slide, an unplaced content block, a
	// truncated column, etc.). Turns silent content loss into one consistent,
	// machine-actionable signal. Advisory; never blocks render.
	ErrCodeContentDropped = "CONTENT_DROPPED"

	// Custom-color drop diagnostic — emitted in constrained design mode when a
	// diagram's data payload carries raw hex colors (e.g. pyramid levels[].color)
	// that the engine ignores in favor of the template scheme. Advisory (info);
	// never blocks render. Tells the author to rerun with design_mode "free" to
	// honor the custom colors.
	ErrCodeCustomColorDropped = "CUSTOM_COLOR_DROPPED"

	// Chart data diagnostic codes (emitted during chart data validation).
	ErrCodeChartValueCoerced     = "chart_value_coerced"
	ErrCodeChartShapeInferred    = "chart_shape_inferred"
	ErrCodeChartDataEmpty        = "chart_data_empty"
	ErrCodeChartPlaceholderEmpty = "CHART_PLACEHOLDER_EMPTY"

	// Grid visual cell finding codes (emitted for diagram/icon/image grid cells).
	ErrCodeGridDiagramNarrow     = "grid_diagram_narrow"
	ErrCodeDiagramAspectMismatch = "diagram_aspect_mismatch"
	ErrCodeDiagramAspectConflict = "diagram_aspect_conflict"

	// Render-time finding codes (emitted during generation, not pre-flight).
	ErrCodePlaceholderRemapped = "placeholder_remapped"
	ErrCodeTextTrimmed         = "text_trimmed"
	ErrCodeTextOverflow        = "text_overflow"
	ErrCodeReadabilityTrimmed  = "readability_trimmed"
	ErrCodeNoAutofitOverflow   = "no_autofit_overflow"
	ErrCodeTableRowsTruncated  = "table_rows_truncated"
	ErrCodeTableFontScaled     = "table_font_scaled"
	ErrCodeDiagramClamped      = "diagram_clamped"
	ErrCodeDiagramRenderFailed = "diagram_render_failed"
	ErrCodePaginationDefault   = "pagination_default_threshold"
	ErrCodeColumnWidthDeficit  = "column_width_deficit"

	// Preflight predictions for render-time behaviour. These mirror the
	// render-time codes but are emitted from collectFitFindings using only
	// template geometry, theme colors, and JSON content — no rendering.
	ErrCodeContrastPredicted = "contrast_predicted"
)

// Sentinel errors for matching with errors.Is. Each ValidationError wraps the
// sentinel corresponding to its Code, so callers can write:
//
//	if errors.Is(err, patterns.ErrRequired) { ... }
var (
	ErrRequired            = errors.New("required field missing")
	ErrMaxLength           = errors.New("value exceeds maximum length")
	ErrOutOfRange          = errors.New("value out of range")
	ErrCountMismatch       = errors.New("item count mismatch")
	ErrUnknownKey          = errors.New("unknown key")
	ErrMinItems            = errors.New("too few items")
	ErrMaxItems            = errors.New("too many items")
	ErrEmptyValue          = errors.New("empty value")
	ErrHexFillNonBrand     = errors.New("hex fill color is not in brand allowlist")
	ErrUnknownLayoutID     = errors.New("layout_id not found in template")
	ErrCalloutUnsupported  = errors.New("pattern does not support callout")
	ErrUnknownEnum         = errors.New("unknown enum value")
	ErrPlaceholderNotFound = errors.New("placeholder_id not found in layout")
	ErrUnknownTableStyleID = errors.New("style_id not found in template table styles")
	ErrWrongPattern        = errors.New("content shape matches a different pattern")
	ErrInvalidShape        = errors.New("value has wrong structure")

	ErrFitOverflow         = errors.New("text exceeds cell dimensions")
	ErrDensityExceeded     = errors.New("table density exceeds TDR ceiling")
	ErrStackedTables       = errors.New("stacked tables with insufficient gap")
	ErrDividerTooThin      = errors.New("divider shape too thin")
	ErrMixedFillScheme     = errors.New("slide mixes hex and semantic fill colors")
	ErrPlaceholderOverflow = errors.New("placeholder text overflows frame")
	ErrSlideBoundsOverflow = errors.New("shape center falls outside slide bounds")
	ErrFooterCollision     = errors.New("shape intrudes into footer reserved area")
	ErrTitleWraps          = errors.New("title text wraps to multiple lines")
	ErrSparseLayout        = errors.New("content occupies less than 40% of bounds height")
	ErrPatternUnderfilled  = errors.New("pattern grid less than 50% filled")
	ErrPatternOvercrowded  = errors.New("pattern grid exceeds recommended cell count")
	ErrCellUnderfilled     = errors.New("cell content is well below capacity")
	ErrTakeawayMissing     = errors.New("slide is missing a takeaway / so-what headline")
	ErrAccentOverload      = errors.New("slide uses more than two distinct accent hues")
	ErrSparseSingleRowFlow = errors.New("single-row flow pattern stretched to fill slide with sparse per-cell text")
	ErrOvertallFlowLane     = errors.New("single-row flow lane occupies more than half the content height with short labels")
	ErrFlowDiamondNoContent = errors.New("process-flow decision diamond has no supporting content zone")
	ErrTocFlowchartVocab    = errors.New("agenda / table-of-contents slide uses sequential flowchart vocabulary")
	ErrMatrixAxisImbalance  = errors.New("rotated axis band spans rows/columns and intrudes after rotation")

	ErrHeadlineTooLong   = errors.New("headline exceeds word count budget")
	ErrBodyTooLong       = errors.New("body text block exceeds word count budget")
	ErrBulletNestingDeep = errors.New("bullet list nests more than two levels deep")
	ErrMissingAltText    = errors.New("image or icon asset is missing alt text")
	ErrDuplicateTitle    = errors.New("slide title duplicates another content slide's title")
	ErrContentDropped    = errors.New("author-provided content was dropped without being placed")

	ErrChartValueCoerced     = errors.New("non-numeric chart value coerced to zero")
	ErrChartShapeInferred    = errors.New("chart data shape inferred from flat input")
	ErrChartDataEmpty        = errors.New("chart data is empty; output will be blank")
	ErrChartPlaceholderEmpty = errors.New("chart placeholder rendered without chart spec")

	ErrPlaceholderRemapped = errors.New("placeholder remapped to fallback target")
	ErrTextTrimmed         = errors.New("trailing paragraphs trimmed to fit placeholder")
	ErrTextOverflow        = errors.New("text overflows placeholder even after trimming")
	ErrReadabilityTrimmed  = errors.New("paragraphs trimmed for readability")
	ErrNoAutofitOverflow   = errors.New("text overflows placeholder with noAutofit")
	ErrTableRowsTruncated  = errors.New("table rows truncated to fit height")
	ErrTableFontScaled     = errors.New("table font scaled to minimum floor")
	ErrDiagramClamped      = errors.New("diagram placeholder dimensions clamped to minimum")
	ErrDiagramRenderFailed = errors.New("diagram rendering failed, placeholder image inserted")
	ErrPaginationDefault   = errors.New("pagination using default threshold, no template capacity")
	ErrColumnWidthDeficit  = errors.New("column widths fell back to global floor")

	ErrContrastPredicted = errors.New("text color is predicted to be auto-replaced for WCAG AA contrast")

	ErrDiagramAspectMismatch = errors.New("diagram cell aspect differs from rendered SVG aspect")
	ErrDiagramAspectConflict = errors.New("diagram cell aspect conflicts with diagram type's natural aspect")
)

// codeSentinel maps error code strings to their sentinel errors.
var codeSentinel = map[string]error{
	ErrCodeRequired:              ErrRequired,
	ErrCodeMaxLength:             ErrMaxLength,
	ErrCodeOutOfRange:            ErrOutOfRange,
	ErrCodeCountMismatch:         ErrCountMismatch,
	ErrCodeUnknownKey:            ErrUnknownKey,
	ErrCodeMinItems:              ErrMinItems,
	ErrCodeMaxItems:              ErrMaxItems,
	ErrCodeEmptyValue:            ErrEmptyValue,
	ErrCodeHexFillNonBrand:       ErrHexFillNonBrand,
	ErrCodeUnknownLayoutID:       ErrUnknownLayoutID,
	ErrCodeCalloutUnsupported:    ErrCalloutUnsupported,
	ErrCodeUnknownEnum:           ErrUnknownEnum,
	ErrCodePlaceholderNotFound:   ErrPlaceholderNotFound,
	ErrCodeUnknownTableStyleID:   ErrUnknownTableStyleID,
	ErrCodeWrongPattern:          ErrWrongPattern,
	ErrCodeInvalidShape:          ErrInvalidShape,
	ErrCodeFitOverflow:           ErrFitOverflow,
	ErrCodeDensityExceeded:       ErrDensityExceeded,
	ErrCodeStackedTables:         ErrStackedTables,
	ErrCodeDividerTooThin:        ErrDividerTooThin,
	ErrCodeMixedFillScheme:       ErrMixedFillScheme,
	ErrCodePlaceholderOverflow:   ErrPlaceholderOverflow,
	ErrCodeSlideBoundsOverflow:   ErrSlideBoundsOverflow,
	ErrCodeFooterCollision:       ErrFooterCollision,
	ErrCodeTitleWraps:            ErrTitleWraps,
	ErrCodeSparseLayout:          ErrSparseLayout,
	ErrCodePatternUnderfilled:    ErrPatternUnderfilled,
	ErrCodePatternOvercrowded:    ErrPatternOvercrowded,
	ErrCodeCellUnderfilled:       ErrCellUnderfilled,
	ErrCodeTakeawayMissing:       ErrTakeawayMissing,
	ErrCodeAccentOverload:        ErrAccentOverload,
	ErrCodeSparseSingleRowFlow:   ErrSparseSingleRowFlow,
	ErrCodeOvertallFlowLane:      ErrOvertallFlowLane,
	ErrCodeFlowDiamondNoContent:  ErrFlowDiamondNoContent,
	ErrCodeTocFlowchartVocab:     ErrTocFlowchartVocab,
	ErrCodeMatrixAxisImbalance:   ErrMatrixAxisImbalance,
	ErrCodeHeadlineTooLong:       ErrHeadlineTooLong,
	ErrCodeBodyTooLong:           ErrBodyTooLong,
	ErrCodeBulletNestingDeep:     ErrBulletNestingDeep,
	ErrCodeMissingAltText:        ErrMissingAltText,
	ErrCodeDuplicateTitle:        ErrDuplicateTitle,
	ErrCodeContentDropped:        ErrContentDropped,
	ErrCodeChartValueCoerced:     ErrChartValueCoerced,
	ErrCodeChartShapeInferred:    ErrChartShapeInferred,
	ErrCodeChartDataEmpty:        ErrChartDataEmpty,
	ErrCodeChartPlaceholderEmpty: ErrChartPlaceholderEmpty,
	ErrCodePlaceholderRemapped:   ErrPlaceholderRemapped,
	ErrCodeTextTrimmed:           ErrTextTrimmed,
	ErrCodeTextOverflow:          ErrTextOverflow,
	ErrCodeReadabilityTrimmed:    ErrReadabilityTrimmed,
	ErrCodeNoAutofitOverflow:     ErrNoAutofitOverflow,
	ErrCodeTableRowsTruncated:    ErrTableRowsTruncated,
	ErrCodeTableFontScaled:       ErrTableFontScaled,
	ErrCodeDiagramClamped:        ErrDiagramClamped,
	ErrCodeDiagramRenderFailed:   ErrDiagramRenderFailed,
	ErrCodePaginationDefault:     ErrPaginationDefault,
	ErrCodeColumnWidthDeficit:    ErrColumnWidthDeficit,
	ErrCodeContrastPredicted:     ErrContrastPredicted,
	ErrCodeDiagramAspectMismatch: ErrDiagramAspectMismatch,
	ErrCodeDiagramAspectConflict: ErrDiagramAspectConflict,
}

// AllFitFindingCodes returns the sorted list of all fit-finding error codes.
// Used by get_capabilities to expose the vocabulary programmatically.
func AllFitFindingCodes() []string {
	codes := make([]string, 0, len(codeSentinel))
	for code := range codeSentinel {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// FixSuggestion is a structured fix suggestion with a machine-readable kind
// and optional parameters. The kind identifies the category of remediation
// (e.g. "split_at_row", "shrink_text"), and params carry the specifics.
type FixSuggestion struct {
	Kind   string         `json:"kind"`             // e.g. "split_at_row", "shrink_text", "provide_value"
	Params map[string]any `json:"params,omitempty"` // kind-specific parameters
}

// TextFix creates a FixSuggestion with kind "text" wrapping a free-form message.
// Deprecated: use structured fix kinds (provide_value, reduce_items, etc.) instead.
// Retained only for external callers; internal constructors use structured kinds.
func TextFix(msg string) *FixSuggestion {
	return &FixSuggestion{Kind: "text", Params: map[string]any{"message": msg}}
}

// ProvideValueFix creates a fix suggestion telling the agent to supply a value.
func ProvideValueFix(path string) *FixSuggestion {
	return &FixSuggestion{Kind: "provide_value", Params: map[string]any{"path": path}}
}

// ReduceTextFix creates a fix suggestion to shorten text to a max length.
func ReduceTextFix(path string, maxLength int) *FixSuggestion {
	return &FixSuggestion{Kind: "reduce_text", Params: map[string]any{"path": path, "max_length": maxLength}}
}

// ReplaceValueFix creates a fix suggestion to set a field within bounds.
func ReplaceValueFix(path string, min, max int) *FixSuggestion {
	return &FixSuggestion{Kind: "replace_value", Params: map[string]any{"path": path, "min": min, "max": max}}
}

// RemoveKeyFix creates a fix suggestion to remove an unknown key.
func RemoveKeyFix(key, path string, allowed []string) *FixSuggestion {
	return &FixSuggestion{Kind: "remove_key", Params: map[string]any{"key": key, "path": path, "allowed": allowed}}
}

// ResizeListFix creates a fix suggestion to set a list to an exact count.
func ResizeListFix(path string, count int) *FixSuggestion {
	return &FixSuggestion{Kind: "resize_list", Params: map[string]any{"path": path, "count": count}}
}

// AddItemsFix creates a fix suggestion to add items to meet a minimum.
func AddItemsFix(path string, minItems int) *FixSuggestion {
	return &FixSuggestion{Kind: "add_items", Params: map[string]any{"path": path, "min_items": minItems}}
}

// ReduceItemsFix creates a fix suggestion to remove items to meet a maximum.
func ReduceItemsFix(path string, maxItems int) *FixSuggestion {
	return &FixSuggestion{Kind: "reduce_items", Params: map[string]any{"path": path, "max_items": maxItems}}
}

// UseOneOfFix creates a fix suggestion to replace a value with one of the allowed options.
func UseOneOfFix(path string, allowed []string) *FixSuggestion {
	return &FixSuggestion{Kind: "use_one_of", Params: map[string]any{"path": path, "allowed": allowed}}
}

// ReshapeValueFix creates a fix suggestion to restructure a value to the expected shape.
func ReshapeValueFix(path, expectedShape, example string) *FixSuggestion {
	return &FixSuggestion{Kind: "reshape_value", Params: map[string]any{
		"path":           path,
		"expected_shape": expectedShape,
		"example":        example,
	}}
}

// RemoveFieldFix creates a fix suggestion to remove a field entirely.
func RemoveFieldFix(path string) *FixSuggestion {
	return &FixSuggestion{Kind: "remove_field", Params: map[string]any{"path": path}}
}

// ValidationError is a structured validation error with a JSON path, error
// code, human-readable message, and optional fix suggestion. It implements the
// error interface so it can be used with errors.Join alongside plain errors.
type ValidationError struct {
	Pattern string         `json:"pattern"`       // e.g. "card-grid"
	Path    string         `json:"path"`          // JSON path, e.g. "cells[2].header"
	Code    string         `json:"code"`          // machine-readable code, e.g. "required"
	Message string         `json:"message"`       // human-readable, e.g. "card-grid: cells[2].header is required"
	Fix     *FixSuggestion `json:"fix,omitempty"` // optional structured fix suggestion
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Unwrap returns the sentinel error for this validation error's Code,
// enabling errors.Is matching (e.g. errors.Is(err, patterns.ErrRequired)).
func (e *ValidationError) Unwrap() error {
	return codeSentinel[e.Code]
}

// --- Constructors ---

// errRequired creates a "required" validation error.
func errRequired(pattern, path string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeRequired,
		Message: fmt.Sprintf("%s: %s is required", pattern, path),
		Fix:     ProvideValueFix(path),
	}
}

// errMaxLength creates a "max_length" validation error.
func errMaxLength(pattern, path string, maxLen, actualLen int) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMaxLength,
		Message: fmt.Sprintf("%s: %s exceeds maxLength %d (%d chars)", pattern, path, maxLen, actualLen),
		Fix:     ReduceTextFix(path, maxLen),
	}
}

// errOutOfRange creates an "out_of_range" validation error for integer bounds.
func errOutOfRange(pattern, path string, min, max, actual int) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeOutOfRange,
		Message: fmt.Sprintf("%s: %s must be %d–%d, got %d", pattern, path, min, max, actual),
		Fix:     ReplaceValueFix(path, min, max),
	}
}

// errUnknownKey creates an "unknown_key" validation error for cell_overrides.
func errUnknownKey(pattern, path, key, allowedList string) *ValidationError {
	allowed := strings.Split(allowedList, ", ")
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeUnknownKey,
		Message: fmt.Sprintf("%s: %s contains unknown key %q; allowed keys per D15: %s", pattern, path, key, allowedList),
		Fix:     RemoveKeyFix(key, path, allowed),
	}
}

// errEmptyValue creates an "empty_value" validation error.
func errEmptyValue(pattern, path string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeEmptyValue,
		Message: fmt.Sprintf("%s: %s must not be empty", pattern, path),
		Fix:     ProvideValueFix(path),
	}
}

// errCellOverrideOutOfRange creates an "out_of_range" error for cell_overrides keys.
func errCellOverrideOutOfRange(pattern string, idx, maxIdx int, hint string) *ValidationError {
	path := fmt.Sprintf("cell_overrides[%d]", idx)
	msg := fmt.Sprintf("%s: cell_overrides key %d out of range [0,%d]", pattern, idx, maxIdx)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeOutOfRange,
		Message: msg,
		Fix:     ReplaceValueFix(path, 0, maxIdx),
	}
}

// errCountMismatch creates a "count_mismatch" validation error.
func errCountMismatch(pattern, path string, expected, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain exactly %d items, got %d", pattern, path, expected, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeCountMismatch,
		Message: msg,
		Fix:     ResizeListFix(path, expected),
	}
}

// errMinItems creates a "min_items" validation error.
func errMinItems(pattern, path string, minCount, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain at least %d items, got %d", pattern, path, minCount, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMinItems,
		Message: msg,
		Fix:     AddItemsFix(path, minCount),
	}
}

// errMaxItems creates a "max_items" validation error.
func errMaxItems(pattern, path string, maxCount, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain at most %d items, got %d", pattern, path, maxCount, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMaxItems,
		Message: msg,
		Fix:     ReduceItemsFix(path, maxCount),
	}
}

// ErrCalloutUnsupportedFor creates a "callout_unsupported" validation error
// that names the pattern and suggests patterns that do support callout.
func ErrCalloutUnsupportedFor(pattern string, supportedPatterns []string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    "pattern.callout",
		Code:    ErrCodeCalloutUnsupported,
		Message: fmt.Sprintf("%s: does not support callout", pattern),
		Fix: &FixSuggestion{
			Kind: "remove_field_or_switch_pattern",
			Params: map[string]any{
				"supports_callout_patterns": supportedPatterns,
			},
		},
	}
}

// newValidationError creates a ValidationError with explicit message and fix.
// Use this when the canned constructors don't match the required message format.
func newValidationError(pattern, path, code, message string, fix *FixSuggestion) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    code,
		Message: message,
		Fix:     fix,
	}
}

// ErrWrongPatternFor creates a "wrong_pattern" validation error with a
// swap_pattern fix suggestion. suggestions carries the alternative patterns
// that accept the user's content shape.
func ErrWrongPatternFor(pattern string, itemCount int, suggestions []SwapSuggestion) *ValidationError {
	// Build target list for human-readable message.
	names := make([]string, len(suggestions))
	for i, s := range suggestions {
		names[i] = s.To
	}

	msg := fmt.Sprintf("%s: content shape (%d items) matches a different pattern; consider %s",
		pattern, itemCount, strings.Join(names, " or "))

	// Convert suggestions to []any for JSON serialisation.
	suggested := make([]any, len(suggestions))
	for i, s := range suggestions {
		m := map[string]any{"from": s.From, "to": s.To}
		if len(s.FieldMapping) > 0 {
			m["field_mapping"] = s.FieldMapping
		}
		if s.Rationale != "" {
			m["rationale"] = s.Rationale
		}
		suggested[i] = m
	}

	return &ValidationError{
		Pattern: pattern,
		Path:    "pattern",
		Code:    ErrCodeWrongPattern,
		Message: msg,
		Fix: &FixSuggestion{
			Kind: "swap_pattern",
			Params: map[string]any{
				"suggested": suggested,
			},
		},
	}
}

// SwapSuggestion describes an alternative pattern and how to map fields.
type SwapSuggestion struct {
	From         string            `json:"from"`
	To           string            `json:"to"`
	Rationale    string            `json:"rationale,omitempty"`
	FieldMapping map[string]string `json:"field_mapping,omitempty"`
}

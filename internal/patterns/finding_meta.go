package patterns

import "sort"

// FindingMeta is the agent-facing description of a single finding code. It
// answers, for one code, what an agent needs to fix the underlying problem
// in one extra tool call: a one-line summary, the severity action, when the
// engine emits it, ranked remediation steps, illustrative before/after
// snippets, and related codes the agent should consider together.
//
// The registry below is the single source of truth for these descriptions.
// TestFindingMetaCoversAllCodes asserts that every code emitted by the
// engine has an entry, so the data cannot silently drift away from the
// codes in errors.go.
type FindingMeta struct {
	Code             string   `json:"code"`
	Summary          string   `json:"summary"`
	Severity         string   `json:"severity"`
	WhenEmitted      string   `json:"when_emitted"`
	RemediationSteps []string `json:"remediation_steps"`
	ExampleBefore    string   `json:"example_before,omitempty"`
	ExampleAfter     string   `json:"example_after,omitempty"`
	RelatedCodes     []string `json:"related_codes,omitempty"`
}

// GetFindingMeta returns the FindingMeta for the given code, or (nil, false)
// when no entry exists. Callers that need to suggest alternatives can use
// AllFindingMetaCodes() to enumerate the known codes.
func GetFindingMeta(code string) (*FindingMeta, bool) {
	m, ok := findingMetaRegistry[code]
	if !ok {
		return nil, false
	}
	// Return a copy to keep the registry immutable from caller mutations.
	out := m
	return &out, true
}

// AllFindingMetaCodes returns the sorted list of codes that have metadata.
// Used by describe_finding to advertise the allowed code vocabulary on the
// unknown-code error path.
func AllFindingMetaCodes() []string {
	out := make([]string, 0, len(findingMetaRegistry))
	for code := range findingMetaRegistry {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// findingMetaRegistry holds the metadata for every finding code the engine
// emits. Entries are grouped by category for readability. When a new code is
// added in errors.go or anywhere else, add an entry here too — the drift
// test will fail otherwise.
var findingMetaRegistry = map[string]FindingMeta{
	// ---- Input validation codes (emitted by pattern.Validate / value unmarshalling) ----

	ErrCodeRequired: {
		Code:        ErrCodeRequired,
		Summary:     "A required field is missing from the value object.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate detects a missing required field on the values object.",
		RemediationSteps: []string{
			"Supply a value for the field at the reported path.",
			"Run show_pattern <name> to see which fields are required for the pattern.",
		},
		ExampleBefore: `{"pattern":{"name":"kpi-3up","values":{"kpis":[{"value":"$1.2M"}]}}}  // missing required "label"`,
		ExampleAfter:  `{"pattern":{"name":"kpi-3up","values":{"kpis":[{"value":"$1.2M","label":"ARR"}]}}}`,
		RelatedCodes:  []string{ErrCodeEmptyValue, ErrCodeInvalidShape},
	},
	ErrCodeMaxLength: {
		Code:        ErrCodeMaxLength,
		Summary:     "A string value exceeds the pattern's maxLength budget.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate measures a string length against the pattern's per-field char budget and finds it exceeds maxLength.",
		RemediationSteps: []string{
			"Shorten the text at the reported path to fit within maxLength.",
			"Consult show_pattern <name> for the field's max_length (or text_budget_guide).",
		},
		ExampleBefore: `{"label":"This pattern card label is way too long for the available cell width"}`,
		ExampleAfter:  `{"label":"Card label"}`,
		RelatedCodes:  []string{ErrCodeBodyTooLong, ErrCodeHeadlineTooLong},
	},
	ErrCodeOutOfRange: {
		Code:        ErrCodeOutOfRange,
		Summary:     "A numeric value is outside the pattern's allowed [min,max] range.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate finds an integer field whose value falls outside the documented bounds for that field.",
		RemediationSteps: []string{
			"Replace the value at the reported path with a value inside [min,max] from the message.",
			"For cell_overrides keys, ensure the index references an existing cell.",
		},
		RelatedCodes: []string{ErrCodeCountMismatch, ErrCodeMaxItems, ErrCodeMinItems},
	},
	ErrCodeCountMismatch: {
		Code:        ErrCodeCountMismatch,
		Summary:     "A list has a different number of items than the pattern requires.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate counts a fixed-arity list and finds the wrong number of items (e.g., bmc-canvas requires exactly 9 cells).",
		RemediationSteps: []string{
			"Resize the list at the reported path to the exact item count noted in the message.",
			"If a different count is intended, swap to a different pattern via recommend_pattern.",
		},
		RelatedCodes: []string{ErrCodeMinItems, ErrCodeMaxItems, ErrCodeWrongPattern},
	},
	ErrCodeUnknownKey: {
		Code:        ErrCodeUnknownKey,
		Summary:     "A cell_overrides or values object contains an unknown key.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate (or strict unknown-key checking) finds a key not in the pattern's schema.",
		RemediationSteps: []string{
			"Remove the unknown key at the reported path.",
			"Check the message for the allowed key list; pick the closest legal key or drop the field.",
		},
		RelatedCodes: []string{ErrCodeInvalidShape},
	},
	ErrCodeMinItems: {
		Code:        ErrCodeMinItems,
		Summary:     "A list has fewer items than the pattern's minimum.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate detects a list shorter than the documented minimum (e.g., kpi-3up requires 3 kpis).",
		RemediationSteps: []string{
			"Append items at the reported path until the list reaches the minimum count.",
			"If the content does not justify the minimum, swap to a smaller pattern via recommend_pattern.",
		},
		RelatedCodes: []string{ErrCodeCountMismatch, ErrCodePatternUnderfilled},
	},
	ErrCodeMaxItems: {
		Code:        ErrCodeMaxItems,
		Summary:     "A list has more items than the pattern's maximum.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate detects a list longer than the documented maximum (e.g., card-grid caps cells at 8).",
		RemediationSteps: []string{
			"Trim items at the reported path to the documented maximum.",
			"If the content needs more items, split across two slides or swap to a denser pattern.",
		},
		RelatedCodes: []string{ErrCodeCountMismatch, ErrCodePatternOvercrowded},
	},
	ErrCodeEmptyValue: {
		Code:        ErrCodeEmptyValue,
		Summary:     "A required value is present but empty (empty string or whitespace).",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate detects a required field that is an empty/whitespace-only string.",
		RemediationSteps: []string{
			"Supply non-empty content at the reported path.",
			"If the field is genuinely optional, remove it; otherwise replace whitespace with real text.",
		},
		RelatedCodes: []string{ErrCodeRequired},
	},
	ErrCodeHexFillNonBrand: {
		Code:        ErrCodeHexFillNonBrand,
		Summary:     "A shape's hex fill color is outside the template's brand allowlist.",
		Severity:    "review",
		WhenEmitted: "Validation finds a `#RRGGBB` shape fill that does not match a theme color or brand-approved palette entry.",
		RemediationSteps: []string{
			"Replace the hex color with a semantic scheme name (accent1..accent6, lt1/dk1, lt2/dk2).",
			"Or add the hex to the template's brand allowlist via register_template_setting.",
		},
		RelatedCodes: []string{ErrCodeMixedFillScheme, ErrCodeAccentOverload},
	},
	ErrCodeUnknownLayoutID: {
		Code:        ErrCodeUnknownLayoutID,
		Summary:     "A slide references a layout_id that the template does not define.",
		Severity:    "refuse",
		WhenEmitted: "Validation cannot find the slide's `layout_id` in the resolved template's `canonical_layout_ids`.",
		RemediationSteps: []string{
			"Call list_templates to discover the template's canonical_layout_ids.",
			"Set slide.layout_id to one of the listed values.",
		},
		RelatedCodes: []string{ErrCodePlaceholderNotFound},
	},
	ErrCodeCalloutUnsupported: {
		Code:        ErrCodeCalloutUnsupported,
		Summary:     "A pattern's callout band was used on a pattern that does not support it.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate sees `pattern.callout` populated on a pattern whose taxonomy does not list callout support.",
		RemediationSteps: []string{
			"Remove the callout from the pattern values.",
			"Or switch to one of the patterns listed in the fix params (e.g., card-grid, comparison-2col).",
		},
	},
	ErrCodeUnknownEnum: {
		Code:        ErrCodeUnknownEnum,
		Summary:     "An enum field received a value outside the allowed set.",
		Severity:    "refuse",
		WhenEmitted: "Validation receives a string for an enum-typed field that does not match any allowed value.",
		RemediationSteps: []string{
			"Use one of the values listed in the fix.params.allowed array.",
			"Inspect get_capabilities vocabularies for the canonical enum set.",
		},
	},
	ErrCodePlaceholderNotFound: {
		Code:        ErrCodePlaceholderNotFound,
		Summary:     "A content item references a placeholder_id that the layout does not declare.",
		Severity:    "refuse",
		WhenEmitted: "Validation cannot match `content[].placeholder_id` to any placeholder on the slide's resolved layout.",
		RemediationSteps: []string{
			"Pick a placeholder_id from the layout — see list_templates → layout_summaries.",
			"Use portable aliases (title, subtitle, body, body_2) when possible.",
		},
		RelatedCodes: []string{ErrCodePlaceholderRemapped},
	},
	ErrCodeUnknownTableStyleID: {
		Code:        ErrCodeUnknownTableStyleID,
		Summary:     "A table references a style_id the template does not define.",
		Severity:    "refuse",
		WhenEmitted: "Validation cannot find the requested style_id in the template's table_styles registry.",
		RemediationSteps: []string{
			"Pick a style_id from list_templates → table_styles for the chosen template.",
			"Or drop the style_id field to fall back to the template default.",
		},
	},
	ErrCodeWrongPattern: {
		Code:        ErrCodeWrongPattern,
		Summary:     "The content's shape matches a different pattern than the one chosen.",
		Severity:    "refuse",
		WhenEmitted: "Pattern.Validate detects a content shape (e.g., item count) that fits another pattern better — the fix suggests swap targets.",
		RemediationSteps: []string{
			"Use recommend_pattern with the actual item count to confirm an alternative.",
			"Apply the suggested field_mapping from fix.params.suggested to remap fields.",
			"Or restructure the content to match the original pattern's contract.",
		},
		RelatedCodes: []string{ErrCodePatternOvercrowded, ErrCodePatternUnderfilled},
	},
	ErrCodeInvalidShape: {
		Code:        ErrCodeInvalidShape,
		Summary:     "A value has the wrong structural shape (e.g., array where object expected).",
		Severity:    "refuse",
		WhenEmitted: "A pattern's custom UnmarshalJSON detects a value whose JSON type does not match the schema.",
		RemediationSteps: []string{
			"Reshape the value at the reported path to the expected_shape from fix.params.",
			"Compare against show_pattern.example_values for a working shape.",
		},
		RelatedCodes: []string{ErrCodeUnknownKey},
	},

	// ---- Fit-finding codes (emitted by collectFitFindings / preflight checks) ----

	ErrCodeFitOverflow: {
		Code:        ErrCodeFitOverflow,
		Summary:     "Text in a table cell exceeds the cell's available height.",
		Severity:    "shrink_or_split",
		WhenEmitted: "textfit.Calculate reports that a table cell's text would not fit at the resolved font size; emitted for headers and data cells.",
		RemediationSteps: []string{
			"For data cells: split the table at the suggested row using repair_slide(kind=split_at_row, params.row=<row>).",
			"For headers: shorten the header text via repair_slide(kind=reduce_text).",
		},
		ExampleBefore: `{"pattern":"table","path":"/slides/0/content/0/rows/3/1","code":"fit_overflow","fix":{"kind":"split_at_row","params":{"row":4}}}`,
		ExampleAfter:  `{"slides":[{"content":[{"type":"table","table_value":{"rows":[[..3 rows..]]}}]},{"title":"... (continued)","content":[{"type":"table","table_value":{"rows":[[..rest..]]}}]}]}`,
		RelatedCodes:  []string{ErrCodeDensityExceeded, ErrCodePlaceholderOverflow, ErrCodeTableRowsTruncated},
	},
	ErrCodeDensityExceeded: {
		Code:        ErrCodeDensityExceeded,
		Summary:     "A table has more cells than the TDR ceiling allows at the resolved font size.",
		Severity:    "shrink_or_split",
		WhenEmitted: "The pre-flight Table Density Ratio check sees a cell count exceeding the per-font-size ceiling (60@18pt, 80@14pt, 100@12pt, 120@10pt).",
		RemediationSteps: []string{
			"Split the table at the suggested row via repair_slide(kind=split_at_row, params.row=<row>).",
			"Or shrink the font size if the template allows.",
			"Or trim columns/rows to bring the cell count under the ceiling.",
		},
		ExampleBefore: `{"code":"density_exceeded","message":"table has 72 cells (8 rows × 9 cols) at 12pt; TDR ceiling is 60"}`,
		ExampleAfter:  `Splitting at row 4 yields two tables with ≤60 cells each.`,
		RelatedCodes:  []string{ErrCodeFitOverflow, ErrCodeTableRowsTruncated, ErrCodeTableFontScaled},
	},
	ErrCodeStackedTables: {
		Code:        ErrCodeStackedTables,
		Summary:     "Two tables are stacked vertically with insufficient gap between them.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds two tables in the same slide whose vertical gap is below the minimum readable spacing.",
		RemediationSteps: []string{
			"Move one table to a separate slide.",
			"Or merge the two tables into one.",
			"Or increase the vertical gap by resizing one table.",
		},
		RelatedCodes: []string{ErrCodeDensityExceeded, ErrCodeFitOverflow},
	},
	ErrCodeDividerTooThin: {
		Code:        ErrCodeDividerTooThin,
		Summary:     "A divider shape's stroke is below the minimum readable thickness.",
		Severity:    "review",
		WhenEmitted: "Pre-flight detects a divider shape (line / thin rectangle) below ~1pt at the rendered size.",
		RemediationSteps: []string{
			"Increase the divider's stroke width or shape height.",
			"Or remove the divider if it adds no structural information.",
		},
	},
	ErrCodeMixedFillScheme: {
		Code:        ErrCodeMixedFillScheme,
		Summary:     "The slide mixes raw hex fills with semantic scheme color fills.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds at least one hex fill and at least one semantic scheme fill on the same slide, which breaks theme/template overrides.",
		RemediationSteps: []string{
			"Convert hex fills to semantic scheme names (accent1..accent6, lt1, dk1).",
			"Or convert all semantic fills to hex if the slide must lock to a custom palette.",
		},
		RelatedCodes: []string{ErrCodeHexFillNonBrand, ErrCodeAccentOverload},
	},
	ErrCodePlaceholderOverflow: {
		Code:        ErrCodePlaceholderOverflow,
		Summary:     "Text in a body/content placeholder overflows its frame even at minimum autofit scale.",
		Severity:    "shrink_or_split",
		WhenEmitted: "Pre-flight finds significant overshoot (>15%), no autofit available, and overflow persists at the minimum font scale.",
		RemediationSteps: []string{
			"Apply repair_slide(kind=reduce_text) to shorten the body text.",
			"Or split the slide content across two slides.",
			"Or change the placeholder's autofit mode to normAutofit so PowerPoint shrinks text.",
		},
		ExampleBefore: `{"pattern":"placeholder","path":"/slides/0/content/body","code":"placeholder_overflow","overflow_ratio":1.42}`,
		ExampleAfter:  `Trim body_value or bullets_value so the measured height fits within the placeholder.`,
		RelatedCodes:  []string{ErrCodeTextOverflow, ErrCodeBodyTooLong, ErrCodeReadabilityTrimmed, ErrCodeNoAutofitOverflow},
	},
	ErrCodeSlideBoundsOverflow: {
		Code:        ErrCodeSlideBoundsOverflow,
		Summary:     "A shape's center falls outside the slide rectangle.",
		Severity:    "shrink_or_split",
		WhenEmitted: "Pre-flight finds a JSON-authored shape whose center coordinate is outside the slide bounds (decorative role shapes are excluded).",
		RemediationSteps: []string{
			"Reposition the shape so its center is inside the slide rectangle.",
			"Or mark it role='decor' if the off-slide placement is intentional.",
		},
		RelatedCodes: []string{ErrCodeFooterCollision},
	},
	ErrCodeFooterCollision: {
		Code:        ErrCodeFooterCollision,
		Summary:     "A shape intrudes into the layout's footer-reserved area.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds the bottom edge of a JSON-authored shape inside the layout footer band (only fires when the layout declares a footer placeholder).",
		RemediationSteps: []string{
			"Move or shrink the shape so its bottom edge clears the footer area.",
			"Or swap to a layout without a footer placeholder.",
		},
		RelatedCodes: []string{ErrCodeSlideBoundsOverflow},
	},
	ErrCodeTitleWraps: {
		Code:        ErrCodeTitleWraps,
		Summary:     "Title text wraps to multiple lines inside its placeholder.",
		Severity:    "review",
		WhenEmitted: "Pre-flight measures the title's rendered height and finds it exceeds a single line at the resolved font size.",
		RemediationSteps: []string{
			"Apply repair_slide(kind=shorten_title) to trim the title to fit one line.",
			"Or accept the wrap if the title genuinely needs the additional words.",
		},
		ExampleBefore: `{"code":"title_wraps","message":"title wraps to multiple lines (36pt font, 9.1\" wide placeholder)"}`,
		ExampleAfter:  `Set the title text_value to ≤ ~50 characters.`,
		RelatedCodes:  []string{ErrCodeHeadlineTooLong},
	},
	ErrCodeSparseLayout: {
		Code:        ErrCodeSparseLayout,
		Summary:     "Slide content occupies less than 40% of the available bounds height.",
		Severity:    "review",
		WhenEmitted: "Pre-flight estimates that the rendered content height is under 40% of the grid bounds — the slide reads as mostly empty.",
		RemediationSteps: []string{
			"Add more content to the grid (more cells, longer text, supporting bullets).",
			"Or swap to a smaller pattern via recommend_pattern with the current item count.",
			"Or set bounds.height to shrink the allocated region.",
		},
		RelatedCodes: []string{ErrCodePatternUnderfilled, ErrCodeCellUnderfilled},
	},
	ErrCodePatternUnderfilled: {
		Code:        ErrCodePatternUnderfilled,
		Summary:     "A pattern grid has less than 50% of its slots populated.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds the pattern's grid populated below 50% of total slots (e.g., 1 of 3 KPIs).",
		RemediationSteps: []string{
			"Add items to reach at least 50% fill.",
			"Or swap to a smaller pattern via recommend_pattern.",
		},
		RelatedCodes: []string{ErrCodeSparseLayout, ErrCodeCellUnderfilled},
	},
	ErrCodePatternOvercrowded: {
		Code:        ErrCodePatternOvercrowded,
		Summary:     "A pattern grid exceeds its recommended maximum cell count.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds the pattern's grid populated above its recommended max (e.g., 12 cards in card-grid where max is 8).",
		RemediationSteps: []string{
			"Apply repair_slide(kind=split_pattern, params.first=<n>) to split across two slides.",
			"Or swap to a denser pattern via recommend_pattern.",
		},
		RelatedCodes: []string{ErrCodeMaxItems, ErrCodeDensityExceeded},
	},
	ErrCodeCellUnderfilled: {
		Code:        ErrCodeCellUnderfilled,
		Summary:     "A shape-grid cell's text uses less than 60% of its character capacity.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a cell whose text length is under 60% of the textcapacity.MaxChars estimate (info at 40–59%, warning <40%).",
		RemediationSteps: []string{
			"Add detail to the cell's text content.",
			"Or use a smaller grid pattern that better fits the content density.",
		},
		RelatedCodes: []string{ErrCodeSparseLayout, ErrCodePatternUnderfilled},
	},
	ErrCodeTakeawayMissing: {
		Code:        ErrCodeTakeawayMissing,
		Summary:     "A chart or matrix slide lacks a `takeaway` headline.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide with a chart content item, chart-shaped diagram, or a matrix-* pattern whose `slide.takeaway` field is empty.",
		RemediationSteps: []string{
			"Add a one-sentence `takeaway` to the slide stating the 'so what' of the data.",
			"Apply via repair_slide(kind=provide_value, params.field='takeaway').",
		},
		ExampleBefore: `{"slides":[{"layout_id":"content","content":[{"type":"chart","chart_value":{...}}]}]}  // takeaway omitted`,
		ExampleAfter:  `{"slides":[{"layout_id":"content","takeaway":"Q4 revenue grew 18% YoY, driven by enterprise.","content":[{"type":"chart","chart_value":{...}}]}]}`,
	},
	ErrCodeAccentOverload: {
		Code:        ErrCodeAccentOverload,
		Summary:     "A slide's shape_grid uses more than two distinct accent hues.",
		Severity:    "review",
		WhenEmitted: "DetectStructuralSmells counts the distinct accent1..accent6 fills on a slide's shape_grid and finds three or more.",
		RemediationSteps: []string{
			"Pick one base accent and use cell_accent_mode (alternate/progressive) for within-slide variety.",
			"Reserve a second accent only for paired comparisons (current vs proposed, before vs after).",
		},
		ExampleBefore: `cells use accent1, accent2, accent3, accent4 — 4 distinct hues`,
		ExampleAfter:  `cells all use accent1 with cell_accent_mode='progressive' (tint ladder)`,
		RelatedCodes:  []string{ErrCodeMixedFillScheme, ErrCodeHexFillNonBrand},
	},

	// ---- Content-lint codes (advisory text-budget checks) ----

	ErrCodeHeadlineTooLong: {
		Code:        ErrCodeHeadlineTooLong,
		Summary:     "A title placeholder carries more than 12 whitespace-separated words.",
		Severity:    "review",
		WhenEmitted: "Pre-flight word-counts the title text and finds >12 words — single-line titles at 36-40pt fit roughly 12 words.",
		RemediationSteps: []string{
			"Trim the title to 12 or fewer words.",
			"Apply via repair_slide(kind=shorten_title).",
		},
		RelatedCodes: []string{ErrCodeTitleWraps, ErrCodeBodyTooLong},
	},
	ErrCodeBodyTooLong: {
		Code:        ErrCodeBodyTooLong,
		Summary:     "A body text block exceeds 80 whitespace-separated words.",
		Severity:    "review",
		WhenEmitted: "Pre-flight word-counts the body text (or aggregated bullets) and finds >80 words — audiences read at most ~5 lines per slide.",
		RemediationSteps: []string{
			"Trim the body to 80 or fewer words.",
			"Apply via repair_slide(kind=reduce_text).",
			"Or split the content across two slides.",
		},
		RelatedCodes: []string{ErrCodePlaceholderOverflow, ErrCodeBulletNestingDeep},
	},
	ErrCodeBulletNestingDeep: {
		Code:        ErrCodeBulletNestingDeep,
		Summary:     "A bullet list nests more than two levels deep.",
		Severity:    "review",
		WhenEmitted: "Pre-flight measures per-bullet indent depth and finds at least one bullet at depth ≥3.",
		RemediationSteps: []string{
			"Flatten the bullet list to two levels or fewer.",
			"Apply via repair_slide(kind=reduce_text).",
		},
		RelatedCodes: []string{ErrCodeBodyTooLong},
	},
	ErrCodeMissingAltText: {
		Code:        ErrCodeMissingAltText,
		Summary:     "An image or icon asset sourced from path/url/svg_data is missing alt text.",
		Severity:    "review",
		WhenEmitted: "Accessibility lint pass detects an image_value, shape_grid image, or icon (cell- or shape-overlay) whose source is a file path, URL, or inline SVG markup but whose alt field is empty. Bundled built-in icons referenced by name are exempt because the name itself supplies an implicit caption.",
		RemediationSteps: []string{
			"Set the alt field on the affected image_value, grid image, or icon entry to a short caption describing the visual content.",
			"For decorative-only marks, supply a brief description (e.g., \"divider\") rather than leaving alt empty.",
			"Switch icons to a bundled name when possible — bundled icons carry implicit alt text from their qualified_name.",
		},
		ExampleBefore: `{"image_value": {"path": "team.png"}}`,
		ExampleAfter:  `{"image_value": {"path": "team.png", "alt": "Leadership team standing on stage"}}`,
		RelatedCodes:  []string{ErrCodeBodyTooLong},
	},
	ErrCodeDuplicateTitle: {
		Code:        ErrCodeDuplicateTitle,
		Summary:     "Two or more content slides share the same title (case-insensitive, whitespace-normalized).",
		Severity:    "review",
		WhenEmitted: "Pre-flight duplicate-title pass groups title-placeholder text across non-title/section slides; emits the finding on the second and later occurrences of any normalized title. Title and section-divider slides are exempt because cover / Q&A / closing slides legitimately repeat phrasing.",
		RemediationSteps: []string{
			"Rename the headline so each content slide announces a distinct point.",
			"Apply via repair_slide(kind=shorten_title) with a new title text, or hand-edit the slide's title content item.",
			"If a section genuinely covers the same topic across multiple slides, prefix titles with subtopic differentiators (e.g., \"Pricing — Plans\", \"Pricing — Margins\").",
		},
		ExampleBefore: `slide 3 and slide 5 both titled "Next Steps"`,
		ExampleAfter:  `slide 3 titled "Next Steps — Q3 Pilot", slide 5 titled "Next Steps — Q4 Rollout"`,
		RelatedCodes:  []string{ErrCodeHeadlineTooLong, ErrCodeTitleWraps},
	},

	// ---- Chart data diagnostic codes ----

	ErrCodeChartValueCoerced: {
		Code:        ErrCodeChartValueCoerced,
		Summary:     "A non-numeric value in the chart data map was coerced to zero.",
		Severity:    "review",
		WhenEmitted: "Chart data validation finds a non-numeric value (string, null, bool) in a numeric column and coerces it to 0.",
		RemediationSteps: []string{
			"Replace the value in chart_value.data with a numeric value.",
			"For genuinely missing values, decide whether 0, null-as-gap, or omitting the row is intended.",
		},
		RelatedCodes: []string{ErrCodeChartShapeInferred, ErrCodeChartDataEmpty},
	},
	ErrCodeChartShapeInferred: {
		Code:        ErrCodeChartShapeInferred,
		Summary:     "Chart received flat data; engine inferred a structured shape (series, gauge, etc.).",
		Severity:    "review",
		WhenEmitted: "Chart validation receives flat key:value data for a chart type that expects structured input (multi-series, gauge, etc.) and infers a shape.",
		RemediationSteps: []string{
			"Restructure chart_value.data into the native format (e.g., series array, gauge {value, min, max}).",
			"See get_chart_capabilities for the expected shape per chart type.",
		},
		RelatedCodes: []string{ErrCodeChartValueCoerced, ErrCodeChartDataEmpty},
	},
	ErrCodeChartDataEmpty: {
		Code:        ErrCodeChartDataEmpty,
		Summary:     "The chart's data map is empty — output would be a blank chart.",
		Severity:    "refuse",
		WhenEmitted: "Chart validation receives an empty data map; rendering would produce a blank placeholder.",
		RemediationSteps: []string{
			"Populate chart_value.data with at least one numeric value.",
			"Or remove the chart content item entirely.",
		},
		RelatedCodes: []string{ErrCodeChartPlaceholderEmpty},
	},
	ErrCodeChartPlaceholderEmpty: {
		Code:        ErrCodeChartPlaceholderEmpty,
		Summary:     "chart-insights-split expanded without a chart spec — left panel collapses.",
		Severity:    "review",
		WhenEmitted: "The chart-insights-split pattern's PostExpandWarnings hook fires when no chart spec was supplied.",
		RemediationSteps: []string{
			"Supply a chart spec in the pattern values.",
			"Or switch to an insights-only pattern (card-grid, pull-quote) via recommend_pattern.",
		},
		RelatedCodes: []string{ErrCodeChartDataEmpty},
	},

	// ---- Grid visual cell codes ----

	ErrCodeGridDiagramNarrow: {
		Code:        ErrCodeGridDiagramNarrow,
		Summary:     "A complex diagram is placed in a narrow grid cell (<50% of slide width).",
		Severity:    "review",
		WhenEmitted: "Generation-time check finds a complex diagram type (org_chart, fishbone, swot, heatmap, ...) in a grid cell whose width is below 50% of slide width.",
		RemediationSteps: []string{
			"Reshape the grid so the diagram cell spans ≥50% of slide width.",
			"Or move the diagram to a full-width layout via repair_slide(kind=reshape_grid).",
		},
		RelatedCodes: []string{ErrCodeDiagramAspectMismatch, ErrCodeDiagramAspectConflict},
	},
	ErrCodeDiagramAspectMismatch: {
		Code:        ErrCodeDiagramAspectMismatch,
		Summary:     "A diagram with explicit width and height has a (post-fit) cell aspect that differs from those pinned dimensions by more than 25%.",
		Severity:    "review",
		WhenEmitted: "Pre-flight or render-time finds a diagram with BOTH explicit diagram.width and diagram.height whose post-fit cell aspect diverges from that pinned aspect by >25%. Unset or single-axis (width-only / height-only) diagrams adapt to the cell aspect and are not flagged (natural-aspect types are covered by diagram_aspect_conflict).",
		RemediationSteps: []string{
			"Reshape the cell to match the diagram's pinned aspect ratio (apply repair_slide(kind=reshape_grid)).",
			"Or set cell.fit to 'contain' / 'fit-width' / 'fit-height'.",
			"Or change the explicit diagram.width / diagram.height to match the cell.",
		},
		RelatedCodes: []string{ErrCodeDiagramAspectConflict, ErrCodeGridDiagramNarrow},
	},
	ErrCodeDiagramAspectConflict: {
		Code:        ErrCodeDiagramAspectConflict,
		Summary:     "A non-chart diagram cell's aspect conflicts with the diagram type's natural aspect by >30%.",
		Severity:    "review",
		WhenEmitted: "Pre-flight or render-time finds a non-chart diagram (timeline, gantt, org_chart) whose cell aspect diverges from svggen.NaturalAspect by >30%.",
		RemediationSteps: []string{
			"Reshape the cell to match the diagram's natural aspect ratio.",
			"Or set cell.fit, or supply explicit diagram.width / diagram.height.",
		},
		RelatedCodes: []string{ErrCodeDiagramAspectMismatch, ErrCodeGridDiagramNarrow},
	},

	// ---- Render-time codes ----

	ErrCodePlaceholderRemapped: {
		Code:        ErrCodePlaceholderRemapped,
		Summary:     "A placeholder_id was implicitly remapped to a different layout placeholder.",
		Severity:    "info",
		WhenEmitted: "Generation resolves an input placeholder_id that the layout does not declare to a fallback placeholder (e.g., subtitle → body on a section layout).",
		RemediationSteps: []string{
			"Author the resolved placeholder_id directly (see fix.params.to) to avoid the implicit remap.",
			"Or swap to a layout that declares the original placeholder_id.",
		},
		RelatedCodes: []string{ErrCodePlaceholderNotFound},
	},
	ErrCodeTextTrimmed: {
		Code:        ErrCodeTextTrimmed,
		Summary:     "Trailing paragraphs were trimmed to fit inside the placeholder.",
		Severity:    "review",
		WhenEmitted: "Render-time text fitting drops trailing paragraphs because they would not fit at the minimum font scale.",
		RemediationSteps: []string{
			"Trim the text upstream so all paragraphs fit.",
			"Or split the content across two slides.",
		},
		RelatedCodes: []string{ErrCodePlaceholderOverflow, ErrCodeTextOverflow, ErrCodeReadabilityTrimmed},
	},
	ErrCodeTextOverflow: {
		Code:        ErrCodeTextOverflow,
		Summary:     "Text overflows the placeholder even after trimming.",
		Severity:    "review",
		WhenEmitted: "Render-time text fitting cannot fit the text even after trimming trailing paragraphs.",
		RemediationSteps: []string{
			"Shorten the content via repair_slide(kind=reduce_text).",
			"Or split the slide across two slides.",
		},
		RelatedCodes: []string{ErrCodePlaceholderOverflow, ErrCodeTextTrimmed, ErrCodeNoAutofitOverflow},
	},
	ErrCodeReadabilityTrimmed: {
		Code:        ErrCodeReadabilityTrimmed,
		Summary:     "Paragraphs were trimmed to keep the resulting font size readable.",
		Severity:    "review",
		WhenEmitted: "Render-time text fitting trims paragraphs to avoid shrinking text below the readability floor.",
		RemediationSteps: []string{
			"Trim or split the content so all paragraphs fit at a readable font size.",
		},
		RelatedCodes: []string{ErrCodeTextTrimmed, ErrCodePlaceholderOverflow},
	},
	ErrCodeNoAutofitOverflow: {
		Code:        ErrCodeNoAutofitOverflow,
		Summary:     "Text overflows a placeholder whose autofit mode is `noAutofit`.",
		Severity:    "review",
		WhenEmitted: "Render-time finds overflow on a placeholder where PowerPoint cannot shrink text because autofit is off.",
		RemediationSteps: []string{
			"Shorten the text via repair_slide(kind=reduce_text).",
			"Or change the layout to use a placeholder with normAutofit.",
		},
		RelatedCodes: []string{ErrCodePlaceholderOverflow, ErrCodeTextOverflow},
	},
	ErrCodeTableRowsTruncated: {
		Code:        ErrCodeTableRowsTruncated,
		Summary:     "Table rows were truncated at render time to fit the available height.",
		Severity:    "review",
		WhenEmitted: "Render-time table layout drops trailing rows because they would exceed the placeholder height at the minimum font scale.",
		RemediationSteps: []string{
			"Split the table at the suggested row via repair_slide(kind=split_at_row).",
			"Or reduce row count upstream.",
		},
		RelatedCodes: []string{ErrCodeDensityExceeded, ErrCodeFitOverflow, ErrCodeTableFontScaled},
	},
	ErrCodeTableFontScaled: {
		Code:        ErrCodeTableFontScaled,
		Summary:     "Table font was scaled to its minimum floor to fit content.",
		Severity:    "review",
		WhenEmitted: "Render-time table layout reduces font size to the minimum readable floor to make rows fit.",
		RemediationSteps: []string{
			"Trim columns or rows so the table fits at a larger font.",
			"Or accept the small font if readability is acceptable for the audience.",
		},
		RelatedCodes: []string{ErrCodeTableRowsTruncated, ErrCodeDensityExceeded},
	},
	ErrCodeDiagramClamped: {
		Code:        ErrCodeDiagramClamped,
		Summary:     "A diagram's width or height was below the minimum and was clamped up.",
		Severity:    "review",
		WhenEmitted: "Render-time finds a diagram placeholder dimension below the engine's minimum and clamps it to the floor.",
		RemediationSteps: []string{
			"Switch to a wider layout via repair_slide(kind=swap_layout).",
			"Or supply explicit diagram.width / diagram.height matching a wider cell.",
		},
		RelatedCodes: []string{ErrCodeGridDiagramNarrow, ErrCodeDiagramAspectMismatch},
	},
	ErrCodeDiagramRenderFailed: {
		Code:        ErrCodeDiagramRenderFailed,
		Summary:     "Diagram rendering failed; a placeholder image was inserted instead.",
		Severity:    "review",
		WhenEmitted: "Render-time svggen call returns an error; the engine inserts a placeholder rather than failing the deck.",
		RemediationSteps: []string{
			"Inspect the diagram data shape against get_diagram_capabilities.",
			"Try a simpler diagram type, or regenerate with svggen-mcp.validate_diagram to surface the underlying error.",
		},
	},
	ErrCodePaginationDefault: {
		Code:        ErrCodePaginationDefault,
		Summary:     "Pagination fell back to a default threshold because the template lacks capacity hints.",
		Severity:    "info",
		WhenEmitted: "Render-time auto-pagination cannot derive a per-template threshold and uses a hard-coded default.",
		RemediationSteps: []string{
			"Register a template capacity hint via register_template_setting.",
			"Or split the content manually to control pagination.",
		},
	},
	ErrCodeColumnWidthDeficit: {
		Code:        ErrCodeColumnWidthDeficit,
		Summary:     "Column widths fell back to the global floor because authored widths summed to less than the available space.",
		Severity:    "review",
		WhenEmitted: "Render-time table layout cannot distribute authored column widths into the available width and falls back to a uniform global floor.",
		RemediationSteps: []string{
			"Set column widths whose sum matches the table's available width.",
			"Or omit column widths to let the engine distribute them.",
		},
	},

	// ---- Preflight prediction codes ----

	ErrCodeContrastPredicted: {
		Code:        ErrCodeContrastPredicted,
		Summary:     "Pre-flight predicts the renderer's auto-fix will replace a text color for WCAG AA contrast.",
		Severity:    "info",
		WhenEmitted: "Pre-flight contrast detector walks shape-grid cells with author-specified text and fill colors and predicts a swap using the same replacement algorithm the renderer applies, so predicted_replacement equals the contrast_autofixed color. fix.params.replacement_mode is 'flip' (pure-neutral white/black snapped to dk1/lt1) or 'lerp' (darkened/lightened via EnsureContrast).",
		RemediationSteps: []string{
			"Adjust the text color upstream to clear the WCAG AA threshold (≥3:1 against the fill).",
			"Or apply repair_slide(kind=replace_color, params.from/to) to lock in the predicted replacement.",
			"Or accept the auto-fix — it is non-destructive and matches the render-time contrast_autofixed finding.",
		},
		RelatedCodes: []string{"contrast_autofixed"},
	},

	// ---- String-literal codes (not in errors.go const block) ----

	"unresolved_placeholder": {
		Code:        "unresolved_placeholder",
		Summary:     "A user-visible string still holds the __FILL__ skeleton placeholder.",
		Severity:    "review",
		WhenEmitted: "validate_input / generate_presentation scan the deck and find a plan_deck skeleton token (__FILL__) that was never replaced with real content. Warning by default; an error when placeholder_policy=strict.",
		RemediationSteps: []string{
			"Replace the __FILL__ token at the reported path with the slide's real content.",
			"plan_deck skeletons are draft scaffolding — overwrite every __FILL__ before publishable generation.",
			"For publishable/gated output, pass placeholder_policy=strict so unresolved tokens block instead of warn.",
		},
		ExampleBefore: `{"placeholder_id":"title","type":"text","text_value":"__FILL__"}`,
		ExampleAfter:  `{"placeholder_id":"title","type":"text","text_value":"Q3 Revenue Growth"}`,
	},
	"contrast_autofixed": {
		Code:        "contrast_autofixed",
		Summary:     "Render-time text color was auto-replaced to meet WCAG AA contrast.",
		Severity:    "info",
		WhenEmitted: "Generator's contrast pass detects low-contrast text against the resolved background and swaps the color; finding records the before/after colors and ratios.",
		RemediationSteps: []string{
			"Author the replacement color directly upstream to make the swap explicit.",
			"Or accept the swap — it is informational and the deck rendered correctly.",
		},
		RelatedCodes: []string{ErrCodeContrastPredicted},
	},
	"findings_truncated": {
		Code:        "findings_truncated",
		Summary:     "Per-slide finding budget was hit; additional findings on this slide were suppressed.",
		Severity:    "info",
		WhenEmitted: "Fit-finding collection caps findings at 5 per slide; the truncation marker reports how many were dropped.",
		RemediationSteps: []string{
			"Re-run with verbose_fit=true (MCP) or --verbose-fit (CLI) to see all findings.",
			"Or fix the highest-severity findings first; many lower-severity ones may share a root cause.",
		},
	},

	// ---- Chart finding codes (chart.* — emitted by svggen during dry-render) ----

	"chart.invalid_numeric": {
		Code:        "chart.invalid_numeric",
		Summary:     "Chart data contains a value that cannot be parsed as a number.",
		Severity:    "review",
		WhenEmitted: "svggen dry-render finds a non-numeric value in a numeric column.",
		RemediationSteps: []string{
			"Replace the value with a numeric one in chart_value.data.",
			"For missing values, decide between 0, null gap, or omitting the entry.",
		},
		RelatedCodes: []string{ErrCodeChartValueCoerced},
	},
	"chart.zero_sum_pie": {
		Code:        "chart.zero_sum_pie",
		Summary:     "A pie/donut chart's slices sum to zero, producing a blank chart.",
		Severity:    "refuse",
		WhenEmitted: "svggen dry-render finds the sum of all pie slice values equals zero.",
		RemediationSteps: []string{
			"Ensure at least one slice has a non-zero value.",
			"Or switch chart type if all-zero is intended.",
		},
		RelatedCodes: []string{ErrCodeChartDataEmpty},
	},
	"chart.negative_on_log": {
		Code:        "chart.negative_on_log",
		Summary:     "A log-scale chart received a negative or zero value, which has no logarithm.",
		Severity:    "review",
		WhenEmitted: "svggen dry-render finds a non-positive value plotted on a logarithmic axis.",
		RemediationSteps: []string{
			"Remove non-positive values from the data.",
			"Or switch to a linear axis via the chart style.",
		},
	},
	"chart.all_zero_series": {
		Code:        "chart.all_zero_series",
		Summary:     "A chart series contains only zero values and would render as a flat line.",
		Severity:    "review",
		WhenEmitted: "svggen dry-render finds a series whose values are all zero.",
		RemediationSteps: []string{
			"Verify the data — all-zero series often signals a column mismatch.",
			"Or drop the series from the chart.",
		},
	},
	"chart.capacity_exceeded": {
		Code:        "chart.capacity_exceeded",
		Summary:     "Chart data exceeds svggen's rendering capacity (too many points/series).",
		Severity:    "review",
		WhenEmitted: "svggen dry-render finds the data exceeds the chart type's max series/points capacity.",
		RemediationSteps: []string{
			"Aggregate or sample data to reduce the point count.",
			"Or switch to a chart type with higher capacity (e.g., line over bar for time series).",
		},
		RelatedCodes: []string{ErrCodeDensityExceeded},
	},
	"chart.invalid_time_format": {
		Code:        "chart.invalid_time_format",
		Summary:     "A time-axis value is not in a recognized format.",
		Severity:    "review",
		WhenEmitted: "svggen dry-render cannot parse a time-axis value (expects ISO 8601 / Y-m-d).",
		RemediationSteps: []string{
			"Reformat time values as ISO 8601 (e.g., 2026-01-01 or 2026-01-01T00:00:00Z).",
		},
	},
	"chart.auto_log_scale_applied": {
		Code:        "chart.auto_log_scale_applied",
		Summary:     "svggen auto-applied a log scale because the value range spans multiple orders of magnitude.",
		Severity:    "info",
		WhenEmitted: "svggen dry-render detects an extreme value range and switches to a logarithmic axis automatically.",
		RemediationSteps: []string{
			"Accept the log scale — it preserves readability.",
			"Or set scale='linear' in the chart style if a linear axis is required.",
		},
	},
	"chart.tick_thinned": {
		Code:        "chart.tick_thinned",
		Summary:     "Axis ticks were thinned because labels would overlap.",
		Severity:    "info",
		WhenEmitted: "svggen layout pass thins axis ticks to avoid label collisions.",
		RemediationSteps: []string{
			"Accept the thinning — it preserves readability.",
			"Or reduce the data point count if denser ticks are required.",
		},
	},
	"chart.scatter_label_skipped": {
		Code:        "chart.scatter_label_skipped",
		Summary:     "Scatter-plot point labels were skipped because they would overlap.",
		Severity:    "info",
		WhenEmitted: "svggen layout pass omits scatter point labels to avoid overlap.",
		RemediationSteps: []string{
			"Accept the omission, or remove duplicates so fewer labels collide.",
		},
	},
	"chart.label_truncated": {
		Code:        "chart.label_truncated",
		Summary:     "An axis or data label was truncated to fit available width.",
		Severity:    "review",
		WhenEmitted: "svggen layout pass shortens a label string that would exceed available width.",
		RemediationSteps: []string{
			"Shorten the source label.",
			"Or widen the chart cell so the full label fits.",
		},
		RelatedCodes: []string{"chart.label_ellipsized", "chart.label_clipped"},
	},
	"chart.label_ellipsized": {
		Code:        "chart.label_ellipsized",
		Summary:     "A label was ellipsized (…) because truncation alone was insufficient.",
		Severity:    "review",
		WhenEmitted: "svggen layout pass applies an ellipsis after truncation cannot make the label fit.",
		RemediationSteps: []string{
			"Shorten the source label.",
			"Or widen the chart cell.",
		},
		RelatedCodes: []string{"chart.label_truncated", "chart.label_clipped"},
	},
	"chart.label_clipped": {
		Code:        "chart.label_clipped",
		Summary:     "A label remains clipped at the cell boundary even after truncation/ellipsis.",
		Severity:    "review",
		WhenEmitted: "svggen layout pass cannot make the label fit; it is rendered clipped at the cell edge.",
		RemediationSteps: []string{
			"Widen the chart cell.",
			"Or shorten the source label drastically.",
		},
		RelatedCodes: []string{"chart.label_truncated", "chart.label_ellipsized"},
	},
	"chart.legend_overflow_dropped": {
		Code:        "chart.legend_overflow_dropped",
		Summary:     "Legend entries were dropped because the legend overflowed available space.",
		Severity:    "review",
		WhenEmitted: "svggen layout pass drops trailing legend entries that would not fit.",
		RemediationSteps: []string{
			"Reduce the number of series.",
			"Or widen the chart cell to give the legend more room.",
		},
	},
	"chart.overflow_suppressed": {
		Code:        "chart.overflow_suppressed",
		Summary:     "Chart elements that would overflow the cell were suppressed.",
		Severity:    "review",
		WhenEmitted: "svggen layout pass drops or clips elements that would extend beyond the cell.",
		RemediationSteps: []string{
			"Widen the chart cell.",
			"Or reduce data/series count so the chart fits.",
		},
	},

	// ---- Template-validation codes (emitted by validate-template; TPL.* namespace) ----

	"TEMPLATE_METADATA_PARSE": {
		Code:        "TEMPLATE_METADATA_PARSE",
		Summary:     "The template's embedded metadata file could not be read or parsed.",
		Severity:    "review",
		WhenEmitted: "validate-template reads ppt/go-slide-creator-metadata.json and finds it missing, unreadable, or not valid JSON. The template still works using inferred defaults.",
		RemediationSteps: []string{
			"Regenerate the template's metadata with mktemplate, or fix the malformed JSON in ppt/go-slide-creator-metadata.json.",
			"If the template was hand-authored without metadata, ignore this warning — layout capabilities are inferred from the slide layouts.",
		},
		RelatedCodes: []string{"TEMPLATE_METADATA_VERSION"},
	},
	"TEMPLATE_METADATA_VERSION": {
		Code:        "TEMPLATE_METADATA_VERSION",
		Summary:     "The template metadata declares a version outside the supported range.",
		Severity:    "review",
		WhenEmitted: "validate-template parses the metadata version and finds it below the minimum or above the maximum supported by this build.",
		RemediationSteps: []string{
			"Re-export the template with a compatible tool version, or edit the \"version\" field in the metadata to a supported value.",
			"Upgrade json2pptx if the template was produced by a newer release.",
		},
		RelatedCodes: []string{"TEMPLATE_METADATA_PARSE"},
	},
	"TEMPLATE_ASPECT_RATIO_INVALID": {
		Code:        "TEMPLATE_ASPECT_RATIO_INVALID",
		Summary:     "The metadata aspect_ratio is not in WIDTH:HEIGHT form.",
		Severity:    "review",
		WhenEmitted: "validate-template finds an aspect_ratio that does not match a numeric WIDTH:HEIGHT pattern (e.g. \"16:9\"). The default 16:9 is used instead.",
		RemediationSteps: []string{
			"Set aspect_ratio to a numeric ratio like \"16:9\" or \"4:3\" in the template metadata.",
		},
		ExampleBefore: `{"aspect_ratio":"16x9"}`,
		ExampleAfter:  `{"aspect_ratio":"16:9"}`,
	},
	"TEMPLATE_LAYOUT_HINT_INVALID": {
		Code:        "TEMPLATE_LAYOUT_HINT_INVALID",
		Summary:     "A layout hint in the metadata is malformed (empty key or a negative budget).",
		Severity:    "review",
		WhenEmitted: "validate-template finds a layout_hints entry with an empty key, a negative max_bullets, or a negative max_chars. The malformed hint is ignored.",
		RemediationSteps: []string{
			"Give every layout_hints entry a non-empty layout key.",
			"Set max_bullets and max_chars to non-negative integers (0 means \"no hint\").",
		},
		ExampleBefore: `{"layout_hints":{"content":{"max_bullets":-1}}}`,
		ExampleAfter:  `{"layout_hints":{"content":{"max_bullets":6}}}`,
	},
	"TEMPLATE_SECTION_NUMBER_NAMING": {
		Code:        "TEMPLATE_SECTION_NUMBER_NAMING",
		Summary:     "A section-header layout has a decorative number placeholder that is not named \"Section Number\".",
		Severity:    "review",
		WhenEmitted: "validate-template finds, on a section-header layout, a small high-position body placeholder with a large font (the signature of a decorative number frame) that is not named \"Section Number\". The engine's normalizer skips shapes named \"Section Number\"; a misnamed one is treated as body text and corrupted.",
		RemediationSteps: []string{
			"Rename the placeholder to \"Section Number\" in the template's slide layout so the engine preserves it.",
			"If the placeholder is genuinely body text, ignore this warning.",
		},
	},
}

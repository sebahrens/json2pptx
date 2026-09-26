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
	ErrCodePatternUnknownField: {
		Code:        ErrCodePatternUnknownField,
		Summary:     "A key in a pattern's values / overrides is never read, so its content is dropped.",
		Severity:    "refuse",
		WhenEmitted: "Pattern input inspection decodes values with the pattern's own decoder and finds text the caller wrote that is absent from the decoded value — a misnamed field (members[].title where the pattern reads role) or a wrapper key. Tolerated aliases survive the decode and are never reported.",
		RemediationSteps: []string{
			"Rename the key at the reported path to fix.params.did_you_mean.",
			"When no rename is suggested, pick a key from fix.params.allowed or show_pattern <name>.",
			"Or remove the key if the content belongs somewhere else on the slide.",
		},
		ExampleBefore: `{"members":[{"name":"Dana","title":"VP Finance"}]}`,
		ExampleAfter:  `{"members":[{"name":"Dana","role":"VP Finance"}]}`,
		RelatedCodes:  []string{ErrCodeInvalidShape, ErrCodeUnknownKey, ErrCodeRequired},
	},
	ErrCodeTextOverImageUnverified: {
		Code:        ErrCodeTextOverImageUnverified,
		Summary:     "Text may overlap an image whose pixel contrast has not been verified.",
		Severity:    "review",
		WhenEmitted: "A slide sets background.image/url without an overlay, or populated text overlaps an authored native image frame without an opaque text fill. Frame overlap is conservative: contain may leave whitespace. Canvas color cannot establish contrast against image pixels.",
		RemediationSteps: []string{
			"Inspect the current rendered slide at readable resolution; check actual pixels under every affected text region.",
			"For required diagrams/screenshots, prefer separate text and image regions. Do not dim or cover required source labels with a scrim.",
			"For background photos, an overlay may help, but its color/alpha alone does not prove composite contrast. Re-render and inspect it.",
			"Use contrast_check: false only after verifying image contrast; it suppresses checks, not the underlying risk.",
		},
		ExampleBefore: `{"background":{"image":"hero.jpg"}}`,
		ExampleAfter:  `{"background":{"image":"hero.jpg","overlay":{"color":"dk1","alpha":0.45}}}`,
		RelatedCodes:  []string{ErrCodeContrastPredicted},
	},
	ErrCodeInvalidShape: {
		Code:        ErrCodeInvalidShape,
		Summary:     "A value has the wrong structural shape (e.g., array where object expected).",
		Severity:    "refuse",
		WhenEmitted: "Pattern input inspection finds a JSON type the pattern cannot read at the reported path (a string where an object belongs, an object wrapping the array values IS), or a pattern's custom UnmarshalJSON rejects a value's shape. The expected shape is named in schema terms — never a Go type — with a copy-ready fix.params.example built from the value you sent, or fix.params.unwrap_key when the payload wraps the array the pattern wants.",
		RemediationSteps: []string{
			"Apply fix.params.example at the reported path, or unwrap fix.params.unwrap_key.",
			"Reshape the value to the expected shape named in the message.",
			"Compare against show_pattern.example_values for a working shape.",
		},
		RelatedCodes: []string{ErrCodePatternUnknownField, ErrCodeUnknownKey},
	},

	// ---- Fit-finding codes (emitted by collectFitFindings / preflight checks) ----

	ErrCodeFitOverflow: {
		Code:        ErrCodeFitOverflow,
		Summary:     "Text exceeds the height available to it — in a table cell, a shape_grid cell, or a grid row.",
		Severity:    "shrink_or_split",
		WhenEmitted: "textfit.Calculate reports that a table cell's text would not fit at the resolved font size (headers and data cells), or the shape_grid density pass finds a cell over its character capacity, or a grid row's content exceeds its max_height.",
		RemediationSteps: []string{
			"For data cells: split the table at the suggested row using repair_slide(kind=split_at_row, params.row=<row>).",
			"For headers: shorten the header text via repair_slide(kind=reduce_text, params.max_chars=<budget>).",
			"For a shape_grid cell: apply the finding's fix verbatim — repair_slide(kind=reduce_cell_text, params={cell_path, max_chars}). reduce_text cannot reach grid text and answers with code wrong_kind_for_target and the corrected directive.",
			"For a grid row: trim its cells (each carries its own reduce_cell_text fix) or reshape the grid; the row itself is not a text target.",
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
		WhenEmitted: "Validate/generate fit collection finds two consecutive rows of an authored shape_grid that both hold a table while the grid's row gap is below 4pt.",
		RemediationSteps: []string{
			"Raise the grid's row_gap (or gap) to at least 4pt.",
			"Or merge the two tables into one.",
			"Or move one table to a separate slide.",
		},
		RelatedCodes: []string{ErrCodeDensityExceeded, ErrCodeFitOverflow},
	},
	ErrCodeDividerTooThin: {
		Code:        ErrCodeDividerTooThin,
		Summary:     "Rows of an authored shape_grid are crushed together or a divider row is too thin.",
		Severity:    "review",
		WhenEmitted: "Validate/generate fit collection finds an authored shape_grid whose row gap is below 3pt, or a row whose explicit height is below 4% of the slide.",
		RemediationSteps: []string{
			"Raise the grid's row_gap (or gap) to at least 3pt.",
			"Or give the thin row at least 4% of the slide height, or drop it if it adds no structure.",
		},
	},
	ErrCodeMixedFillScheme: {
		Code:        ErrCodeMixedFillScheme,
		Summary:     "The slide mixes raw hex fills with semantic scheme color fills.",
		Severity:    "review",
		WhenEmitted: "Validate/generate fit collection finds at least one non-black/white hex fill and at least one semantic scheme fill in the same authored shape_grid, which breaks theme/template overrides.",
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
	ErrCodeChromeCollision: {
		Code:        ErrCodeChromeCollision,
		Summary:     "Authored content overlaps opaque artwork inherited from the template.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a populated content placeholder inside a filled master or layout decoration; rendering reserves tall side artwork but may narrow the content area.",
		RemediationSteps: []string{
			"Choose a layout whose content placeholder clears the artwork.",
			"Or repair the template so its body placeholder does not overlap its own decorative shapes.",
		},
		RelatedCodes: []string{ErrCodeFooterCollision},
	},
	ErrCodeTitleWraps: {
		Code:        ErrCodeTitleWraps,
		Summary:     "Title needs more lines than the layout comfortably expects.",
		Severity:    "info",
		WhenEmitted: "Pre-flight finds three or more title lines, two lines in a one-line title box, or a measured title that needs a reduced comfort size. Two lines in a roomy box are silent.",
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
		Summary:     "Visible slide content covers less than 40% of the resolved grid area.",
		Severity:    "review",
		WhenEmitted: "Pre-flight measures filled cells, non-text visuals, and wrapped text ink against resolved grid bounds; native framework rendering measures diagram text ink against its allocated region. Both emit below 40% coverage.",
		RemediationSteps: []string{
			"Add more content to the grid (more cells, longer text, supporting bullets).",
			"Or swap to a smaller pattern via recommend_pattern with the current item count.",
			"Or set bounds.height to shrink the allocated region.",
			"For a native SWOT, PESTEL, KPI-dashboard or house diagram, add supporting detail or use a shorter diagram region; its sparse finding recommends add_detail_or_resize.",
		},
		RelatedCodes: []string{ErrCodePatternUnderfilled, ErrCodeCellUnderfilled},
	},
	ErrCodePatternUnderfilled: {
		Code:        ErrCodePatternUnderfilled,
		Summary:     "A pattern grid has too few populated slots or too little measured content height.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds the pattern's grid populated below 50% of total slots (e.g., 1 of 3 KPIs), or validate_pattern finds text and visuals filling under 40% of the available height in a multi-cell pattern.",
		RemediationSteps: []string{
			"Add items or detail to make better use of the available space.",
			"Or reduce the pattern height with max_height_pct, or swap to a smaller pattern via recommend_pattern.",
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
		Summary:     "An authored raw shape grid's text cells use well under their character capacity — reported once per slide, not once per cell.",
		Severity:    "info",
		WhenEmitted: "Pre-flight finds authored raw-grid cells whose wrapped text fills under 35% of their box height (textcapacity measures each paragraph at its own font size) and folds them into ONE finding per slide, listing each in fix.params.cells[]. Named patterns and compose segments use visible-ink under-fill checks instead of expander-owned cell capacity. Metric values and short labels/captions are exempt: a KPI card holding \"$12.4M\" is correct, not underfilled. The finding is informational unless the grid as a whole carries under 30% of its text capacity across 3+ cells, in which case it escalates to review.",
		RemediationSteps: []string{
			"Check fix.params.slide_mostly_empty first — when false this is advisory and a deliberately airy layout is a fine reason to ignore it.",
			"When it is true, add detail to the cells listed in fix.params.cells[], or swap to a pattern with fewer/smaller cells.",
			"fix.params.slide_fill_pct is the grid's overall fill; min_density_pct locates the sparsest cell.",
		},
		RelatedCodes: []string{ErrCodeSparseLayout, ErrCodePatternUnderfilled},
	},
	ErrCodeTakeawayMissing: {
		Code:        ErrCodeTakeawayMissing,
		Summary:     "A slide that argues from data lacks a `takeaway` headline.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide with a chart content item, a chart-shaped diagram, or a pattern marked `data_visual` in its taxonomy, whose `slide.takeaway` is empty and whose title is not itself a full-sentence takeaway.",
		RemediationSteps: []string{
			"Add a one-sentence `takeaway` to the slide stating the 'so what' of the data.",
			"Or make the slide title the argument — a six-word-or-longer sentence with a verb suppresses the finding.",
			"Apply via repair_slide(kind=provide_value, params.field='takeaway').",
		},
		ExampleBefore: `{"slides":[{"layout_id":"content","content":[{"type":"chart","chart_value":{...}}]}]}  // takeaway omitted`,
		ExampleAfter:  `{"slides":[{"layout_id":"content","takeaway":"Q4 revenue grew 18% YoY, driven by enterprise.","content":[{"type":"chart","chart_value":{...}}]}]}`,
	},
	ErrCodeChromeBandNoFit: {
		Code:        ErrCodeChromeBandNoFit,
		Summary:     "The slide's takeaway/source band cannot be placed on its layout without overlapping chrome.",
		Severity:    "review",
		WhenEmitted: "Pre-flight resolves the layout-derived band stack (body column x-range, stacked above the dt/ftr/sldNum footer placeholders) and finds it would climb into the title or leave under 20% of the slide height for the layout's content placeholders. The band is skipped at render instead of overlapping.",
		RemediationSteps: []string{
			"Move the slide to a layout with more vertical room (e.g. One Content instead of a section divider).",
			"Or drop the `source` note, or fold the takeaway into the title / body text.",
		},
		RelatedCodes: []string{ErrCodeTakeawayMissing, ErrCodeFooterCollision},
	},
	ErrCodeAccentOverload: {
		Code:        ErrCodeAccentOverload,
		Summary:     "A slide's shape_grid uses more than two distinct accent hues.",
		Severity:    "review",
		WhenEmitted: "Validate/generate fit collection counts the distinct accent1..accent6 fills on an authored (non-pattern, non-compose) shape_grid and finds three or more.",
		RemediationSteps: []string{
			"Pick one base accent and use cell_accent_mode (alternate/progressive) for within-slide variety.",
			"Reserve a second accent only for paired comparisons (current vs proposed, before vs after).",
		},
		ExampleBefore: `cells use accent1, accent2, accent3, accent4 — 4 distinct hues`,
		ExampleAfter:  `cells all use accent1 with cell_accent_mode='progressive' (tint ladder)`,
		RelatedCodes:  []string{ErrCodeMixedFillScheme, ErrCodeHexFillNonBrand},
	},

	ErrCodeSparseSingleRowFlow: {
		Code:        ErrCodeSparseSingleRowFlow,
		Summary:     "A single-row sequence pattern with sparse per-cell text fills the whole slide, so its boxes stretch vertically.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide-level process-flow (or single-row \"dots\" timeline-horizontal) of 3–6 cells whose average per-cell text is below the sparse threshold, with no bounds / max_height_pct cap. Compose segments and nested cell patterns are exempt (a second zone already absorbs the height).",
		RemediationSteps: []string{
			"Swap to numbered-step-strip via recommend_pattern — its per-step detail zone fills the vertical space.",
			"Or swap to process-grid-2row when the steps split into two parallel tracks, or phase-roadmap for dated milestones with descriptions.",
			"Or keep the pattern and set max_height_pct (e.g. 35) so the row no longer stretches to fill the slide.",
		},
		ExampleBefore: `{"pattern":{"name":"process-flow","values":{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}}}`,
		ExampleAfter:  `{"pattern":{"name":"numbered-step-strip","values":{"steps":[{"title":"Plan","detail":"…"},{"title":"Build","detail":"…"},{"title":"Ship","detail":"…"}]}}}`,
		RelatedCodes:  []string{ErrCodeSparseLayout, ErrCodeCellUnderfilled, ErrCodeWrongPattern},
	},

	// ---- Pattern-choice / rendering QA heuristics (J2P-VQA-009) ----

	ErrCodeOvertallFlowLane: {
		Code:        ErrCodeOvertallFlowLane,
		Summary:     "A single-row flow lane occupies more than half the content height with short labels, so its boxes stretch vertically.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide-level process-flow / timeline-horizontal whose estimated lane height exceeds ~50% of the content area with short average per-cell text, in cases SPARSE_SINGLE_ROW_FLOW does not cover (a max_height_pct cap that is still too tall, or a 7–8 step row). The two never fire on the same slide.",
		RemediationSteps: []string{
			"Swap to numbered-step-strip — its per-step detail zone fills the vertical space instead of stretching the boxes.",
			"Or swap to process-grid-2row when the steps split into two parallel tracks.",
			"Or set max_height_pct to ~35 so the lane no longer occupies half the slide.",
		},
		ExampleBefore: `{"pattern":{"name":"process-flow","max_height_pct":60,"values":{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Scale"},{"label":"Review"},{"label":"Iterate"},{"label":"Ship"}]}}}`,
		ExampleAfter:  `{"pattern":{"name":"numbered-step-strip","values":{"steps":[{"title":"Plan","detail":"…"}]}}}`,
		RelatedCodes:  []string{ErrCodeSparseSingleRowFlow, ErrCodeSparseLayout, ErrCodeWrongPattern},
	},
	ErrCodeFlowDiamondNoContent: {
		Code:        ErrCodeFlowDiamondNoContent,
		Summary:     "A standalone process-flow carries a decision diamond but has no supporting zone to explain the branch outcomes.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide-level process-flow with at least one step of type \"decision\" (a diamond) and no second content zone. A lone single-row flow cannot show the yes/no paths a decision implies. Compose envelopes and nested cell patterns are exempt (a second zone carries the explanation).",
		RemediationSteps: []string{
			"Switch to numbered-step-strip with per-step detail so each decision outcome is explained.",
			"Or pair the flow with an explanatory panel using a compose envelope so the branch outcomes are visible.",
			"Or remove the decision diamond if the step is not actually a branch.",
		},
		ExampleBefore: `{"pattern":{"name":"process-flow","values":{"steps":[{"label":"Request"},{"label":"Review","type":"decision"},{"label":"Deploy"}]}}}`,
		ExampleAfter:  `{"compose":{"direction":"vertical","segments":[{"pattern":{"name":"process-flow","values":{"steps":[…]}}},{"pattern":{"name":"card-grid","values":{…branch outcomes…}}}]}}`,
		RelatedCodes:  []string{ErrCodeSparseSingleRowFlow, ErrCodeWrongPattern},
	},
	ErrCodeTocFlowchartVocab: {
		Code:        ErrCodeTocFlowchartVocab,
		Summary:     "An agenda / table-of-contents slide is drawn with sequential flowchart vocabulary (process-flow / swimlane / timeline).",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a slide whose title matches agenda / table-of-contents vocabulary while the slide's pattern is a sequential flow (process-flow, process-flow-compact, swimlane, timeline-horizontal). A contents list is not a sequence with arrows.",
		RemediationSteps: []string{
			"Switch to the agenda pattern — a numbered section list is the canonical table-of-contents layout.",
			"Or use numbered-step-strip in 'toc' style for a contents list without flowchart arrows.",
		},
		ExampleBefore: `{"content":[{"placeholder_id":"title","type":"text","text_value":"Agenda"}],"pattern":{"name":"process-flow","values":{"steps":[…]}}}`,
		ExampleAfter:  `{"content":[{"placeholder_id":"title","type":"text","text_value":"Agenda"}],"pattern":{"name":"agenda","values":{"items":[…]}}}`,
		RelatedCodes:  []string{ErrCodeSparseSingleRowFlow, ErrCodeWrongPattern},
	},
	ErrCodeMatrixAxisImbalance: {
		Code:        ErrCodeMatrixAxisImbalance,
		Summary:     "A spanning text band is rotated ~90°/270°, so it renders wide-short (or tall-narrow) and intrudes into adjacent cells.",
		Severity:    "review",
		WhenEmitted: "Pre-flight finds a shape_grid cell whose text-bearing shape is rotated within ~15° of 90° or 270° and spans rows or columns (an axis band). Rotating the whole band flips its width/height about its center (the J2P-MATRIX-005 anti-pattern). matrix-2x2 now uses vert270 text in an unrotated band, so the check guards against regressions and hand-authored rotated bands.",
		RemediationSteps: []string{
			"Set the band shape's rotation to 0 and rotate only the text via vert=\"vert270\" (vertical text direction) so the fill geometry is never transformed.",
			"Or use the matrix-2x2 pattern, which renders axis labels correctly.",
		},
		ExampleBefore: `{"shape":{"geometry":"rect","rotation":270,"text":{"content":"Market Growth"}}}  // row_span 2 band`,
		ExampleAfter:  `{"shape":{"geometry":"rect","text":{"content":"Market Growth","vert":"vert270"}}}`,
		RelatedCodes:  []string{ErrCodeDividerTooThin},
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
	ErrCodeWeakContent: {
		Code:        ErrCodeWeakContent,
		Summary:     "A slide still carries exemplar copy instead of the deck's content.",
		Severity:    "refuse",
		WhenEmitted: "A slide's text matches known placeholder copy — lorem ipsum, \"Click to add title\", \"Presentation Title\", a masked number like \"XX%\" or \"$X.XM\", a whole value of \"TBD\" / \"N/A\", or a pattern's own exemplar label (\"Card 1\", \"Description 2\"). Structural strings (placeholder IDs, geometry names) are exempt. It is the raw-deck twin of the semantic compiler's SEMANTIC_WEAK_CONTENT, which only ever saw DeckSpec input.",
		RemediationSteps: []string{
			"Replace the named strings with the deck's real message — the finding lists up to three samples per slide.",
			"A make_deck skeleton is exemplar by design: fill it in with repair_slide before shipping (it reports content_status: exemplar_skeleton for the same reason).",
		},
		ExampleBefore: `{"placeholder_id":"title","type":"text","text_value":"Click to add title"}`,
		ExampleAfter:  `{"placeholder_id":"title","type":"text","text_value":"Margin recovered to 68% in Q3"}`,
		RelatedCodes:  []string{ErrCodeSlideNearlyEmpty, ErrCodeMissingTitle},
	},
	ErrCodeMissingTitle: {
		Code:        ErrCodeMissingTitle,
		Summary:     "A content slide has no title.",
		Severity:    "review",
		WhenEmitted: "A slide expected to make a point (not a title, section or blank slide) carries no non-empty title placeholder and no pattern title value.",
		RemediationSteps: []string{
			"Give the slide the one-line point it makes — a title is what the audience reads first and what makes the deck navigable.",
			"If the slide is deliberately chrome (a divider, a full-bleed image), set slide_type to \"section\" or \"blank\" so it is not judged as an argument slide.",
		},
		RelatedCodes: []string{ErrCodeDuplicateTitle, ErrCodeHeadlineTooLong},
	},
	ErrCodeSectionNumberSequenceMismatch: {
		Code:        ErrCodeSectionNumberSequenceMismatch,
		Summary:     "An authored numeric section label contradicts the automatic divider sequence.",
		Severity:    "refuse",
		WhenEmitted: "A section-divider slide explicitly populates its section-number slot with digits whose numeric value differs from that divider's 1-based sequence. Leading zeroes are ignored for comparison; non-numeric labels remain supported as custom numbering.",
		RemediationSteps: []string{
			"Replace the authored label with the expected value reported in the finding.",
			"Or remove the authored section-number content and let json2pptx inject the sequence automatically.",
		},
	},
	ErrCodeSectionNumberRenumbered: {
		Code:        ErrCodeSectionNumberRenumbered,
		Summary:     "Warn-mode generation corrected an authored numeric section label to the divider sequence.",
		Severity:    "info",
		WhenEmitted: "A section-divider label conflicts with its 1-based sequence and generation runs with strict_fit=warn.",
		RemediationSteps: []string{
			"Update the authored label to the reported value so validation and the source deck agree.",
		},
	},
	ErrCodeSlideNearlyEmpty: {
		Code:        ErrCodeSlideNearlyEmpty,
		Summary:     "A content slide carries almost no content.",
		Severity:    "review",
		WhenEmitted: "A text-only content slide carries fewer than 8 words of body content, or a deck's slides carry nothing the engine recognises at all (usually a DeckSpec handed to a PresentationInput tool).",
		RemediationSteps: []string{
			"Add the substance the slide promises, or merge it into a neighbouring slide.",
			"If the JSON is a DeckSpec (kind / points / takeaway), compile it with validate_deck_spec + render_deck_spec — those tools report on the spec itself.",
		},
		RelatedCodes: []string{ErrCodeSlideUnderused, ErrCodeWeakContent},
	},
	ErrCodeDeckMonotony: {
		Code:        ErrCodeDeckMonotony,
		Summary:     "Several consecutive slides are built the same way.",
		Severity:    "review",
		WhenEmitted: "Four or more consecutive argument slides share one shape (the same pattern, or the same content types); six or more raises it to refuse. Title, section and blank slides never join a run.",
		RemediationSteps: []string{
			"Vary the visual family: call recommend_visual for the middle slides of the run and take a different pattern for some of them.",
			"Or merge the run — eight monthly KPI slides are usually one table or one trend chart.",
		},
		RelatedCodes: []string{ErrCodeDuplicateTitle},
	},
	ErrCodePatternContentMismatch: {
		Code:        ErrCodePatternContentMismatch,
		Summary:     "The pattern is not the shape of the content in it.",
		Severity:    "review",
		WhenEmitted: "Every geometric check asks whether content fits; this one asks whether it belongs. Emitted when a timeline's stops are all measurements against labels that are not periods (metrics drawn as a sequence), when all four quadrants of a 2x2 are named after points in time (a plan drawn as a matrix), or when every step of a flow / tier of a pyramid is a short label carrying a figure (numbers drawn as an order of operations). Each rule needs EVERY item to match, so one date among three metrics does not trip it.",
		RemediationSteps: []string{
			"Take the swap_pattern fix: it names the pattern whose shape the content already has.",
			"Or keep the pattern and give it the content it is for — a timeline needs periods, a 2x2 needs two crossing dimensions, a flow needs stages that follow one another.",
		},
		RelatedCodes: []string{ErrCodeDeckMonotony},
	},
	ErrCodeChartOverloaded: {
		Code:        ErrCodeChartOverloaded,
		Summary:     "A chart has more categories than a reader can follow.",
		Severity:    "review",
		WhenEmitted: "A pie/donut carries more than 7 slices, or any other chart more than 12 categories — 8 when the labels average 24+ characters, because the renderer then rotates and truncates them into an unreadable band.",
		RemediationSteps: []string{
			"Keep the top N categories and group the rest into \"Other\".",
			"Or split the chart across two slides, or switch to a table when every row matters.",
		},
		RelatedCodes: []string{ErrCodeDensityExceeded},
	},
	ErrCodeBodyTooLong: {
		Code:        ErrCodeBodyTooLong,
		Summary:     "Body copy exceeds either the 80-word content limit or a pattern's shape-specific text budget.",
		Severity:    "review",
		WhenEmitted: "Content lint finds more than 80 whitespace-separated words in one text/bullet block, or a pattern's post-expand check finds text beyond its geometry-specific character, line, or measured-fit budget (card-grid reports its body-only limit per cell).",
		RemediationSteps: []string{
			"Read the finding's path and message for the actual unit and target; do not assume that every BODY_TOO_LONG finding has an 80-word limit.",
			"For content-lint findings with fix.params.max_words, trim to that word limit or use repair_slide(kind=reduce_text, params={max_words}); preserve numbers, units, negations, and qualifiers.",
			"For pattern warnings, shorten the named field to the reported character/line budget, reduce the pattern's item density, or split the content across slides. A pattern warning does not supply max_words.",
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
			"Apply via repair_slide(kind=reduce_text, params={max_items}) to drop the deepest items, or restructure them into bullet_groups.",
		},
		RelatedCodes: []string{ErrCodeBodyTooLong},
	},
	ErrCodeNumberedListNotApplied: {
		Code:        ErrCodeNumberedListNotApplied,
		Summary:     "Bullets carry typed \"N. \" prefixes the renderer cannot turn into auto-numbering, so they print beside the layout's bullet glyph.",
		Severity:    "review",
		WhenEmitted: "A bullets list has at least one entry starting \"1. \" / \"2. \" but is not a complete ordered list numbered from 1 with no gaps — a partially numbered list, a list starting at another number, or one with a repeated or skipped number. A complete list is auto-numbered by OOXML and the typed prefixes are removed; anything else keeps the author's text verbatim, which reads as a double marker.",
		RemediationSteps: []string{
			"Number every bullet from 1 with no gaps, and the renderer supplies the numbers (the typed prefixes are stripped).",
			"Or remove the \"N. \" prefixes entirely and let the list render as unordered bullets.",
			"A single line that merely starts with a number (\"2024. A big year\") is prose and is left alone — this finding does not apply to it.",
		},
		ExampleBefore: `{"bullets_value":["1. Freeze the schema","Replay the log","3. Cut traffic over"]}`,
		ExampleAfter:  `{"bullets_value":["1. Freeze the schema","2. Replay the log","3. Cut traffic over"]}`,
		RelatedCodes:  []string{ErrCodeBulletNestingDeep},
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

	ErrCodeContentDropped: {
		Code:        ErrCodeContentDropped,
		Summary:     "Author-provided content could not be placed and was dropped (a skipped slide, an unplaced content block, a truncated column, or a dropped payload field).",
		Severity:    "review",
		WhenEmitted: "Any engine path that fails to place author-supplied content emits this shared signal instead of dropping silently — e.g. a slide skipped in partial mode, a content block with no available placeholder, or a column that did not fit. The drop has already happened; the finding makes it visible and repairable.",
		RemediationSteps: []string{
			"Read the fix.params.locator and fix.params.reason to identify exactly what was dropped and why.",
			"Restructure the slide so the content fits: split it across two slides, move the dropped item to its own slide, or reduce the surrounding content.",
			"If the drop was caused by an invalid slide spec (partial mode), fix the underlying error so the slide is no longer skipped.",
		},
		ExampleBefore: `slide 4 silently omitted because its layout_id was invalid`,
		ExampleAfter:  `slide 4 fixed (or split out) so all author content renders`,
		RelatedCodes:  []string{ErrCodePlaceholderRemapped, ErrCodeTextTrimmed, ErrCodeTableRowsTruncated},
	},

	ErrCodeLayoutDerived: {
		Code:        ErrCodeLayoutDerived,
		Summary:     "An asymmetric two-column slide was derived from a template layout.",
		Severity:    "info",
		WhenEmitted: "A slide requests two-column-wide-narrow or two-column-narrow-wide. Generation keeps the native Two Content layout's styling and outer bounds but resizes its body columns to 65/35 or 35/65 with at least a 0.3-inch gutter.",
		RemediationSteps: []string{
			"No action is needed if the derived geometry fits the content.",
			"Use two-column for the template's native balanced column widths.",
		},
	},
	ErrCodeLayoutUnresolvable: {
		Code:        ErrCodeLayoutUnresolvable,
		Summary:     "No layout in this template can host the slide's declared slide_type.",
		Severity:    "review",
		WhenEmitted: "Layout auto-selection finds no layout matching the slide's type — the template is missing that role entirely (e.g. only Title Slide and Blank are present). A pattern / shape_grid / compose slide then falls back to the template's blank canvas with a warning, since those need only a canvas; any other slide refuses, because there is nowhere to put its placeholders.",
		RemediationSteps: []string{
			"Set slide.layout_id explicitly to one of fix.params.candidates — the layouts this template actually declares.",
			"Or pick a template that declares the role: call list_templates and check canonical_layout_ids, or examine_template for the missing-role findings.",
			"For a pattern or shape_grid slide the canvas fallback is usually acceptable; the finding tells you the placement was not the engine's first choice.",
		},
		RelatedCodes: []string{ErrCodeContentDropped},
	},

	// ---- Design-mode codes ----

	ErrCodeDesignModeViolation: {
		Code:        ErrCodeDesignModeViolation,
		Summary:     "The slide uses a free-mode-only construct — a raw hex color or an absolute font size — while the deck is in constrained design mode (the default), which blocks generation.",
		Severity:    "refuse",
		WhenEmitted: "validate_input / generate_presentation find a raw hex color on the documented override surface (diagram_value.style.colors, shape fills, text colors) or an explicit font size, and the deck's design_mode is \"constrained\" (the default when the field is absent). Unlike CUSTOM_COLOR_DROPPED — which is advisory and describes colors the engine ignores — this one refuses the deck.",
		RemediationSteps: []string{
			"Replace the raw hex color with a scheme color name (accent1-accent6, dk1, dk2, lt1, lt2). fix.params.value carries the nearest scheme color to the hex you used.",
			"Remove the explicit size field and let the template manage type scale; fix.kind \"remove_field\" names the exact path.",
			"If the raw value is genuinely required (a brand color the template does not carry), set the DECK-level \"design_mode\": \"free\" — it is a top-level field, not a slide field — or pass --design-mode=free to the CLI.",
		},
		ExampleBefore: `{"shape": {"fill": "#1F4E79"}}`,
		ExampleAfter:  `{"shape": {"fill": "accent1"}}`,
		RelatedCodes:  []string{ErrCodeCustomColorDropped},
	},

	ErrCodeCustomColorDropped: {
		Code:        ErrCodeCustomColorDropped,
		Summary:     "A diagram's data payload embedded raw hex colors, which constrained design mode ignores — the diagram rendered with the template scheme instead.",
		Severity:    "info",
		WhenEmitted: "A diagram data payload carries raw hex colors in per-item fields (e.g. pyramid levels[].color) that are not part of the validated override surface. In constrained mode (the default) they are dropped rather than refused, so this finding makes the drop visible.",
		RemediationSteps: []string{
			"If the template scheme is acceptable, no action is needed — the drop is intended behaviour in constrained mode.",
			"To honor the custom colors, rerun with the deck-level \"design_mode\": \"free\".",
			"To keep constrained mode and still vary the colors, use scheme color names (accent1-accent6) in the data payload instead of hex; those are allowed and never dropped.",
		},
		ExampleBefore: `{"type": "pyramid", "data": {"levels": [{"label": "Vision", "color": "#FF0000"}]}}`,
		ExampleAfter:  `{"type": "pyramid", "data": {"levels": [{"label": "Vision", "color": "accent1"}]}}`,
		RelatedCodes:  []string{ErrCodeDesignModeViolation},
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

	ErrCodeLowContrastHighlight: {
		Code:        ErrCodeLowContrastHighlight,
		Summary:     "An authored pattern highlight does not read as a highlight against the structure it sits in.",
		Severity:    "review",
		WhenEmitted: "A pattern whose one semantic signal is a highlighted cell (value-chain) measures the authored highlight_color against the resolved fill of the other cells and finds under 3:1 — the WCAG non-text bar, below which the two fills are one block of colour. Only an AUTHORED highlight draws this: the default is chosen by the same measurement, so it cannot fail.",
		RemediationSteps: []string{
			"Omit highlight_color and let the engine pick the first accent that clears 3:1 against the step fill for this template.",
			"Or choose a different accent: contrast is template-dependent, so the slot that works on one template can vanish on another.",
		},
		ExampleBefore: `{"highlight_color": "accent2", "steps": [{"label": "Manufacturing", "highlight": true}]}`,
		ExampleAfter:  `{"steps": [{"label": "Manufacturing", "highlight": true}]}`,
	},
	ErrCodeRotatedAccentUnreadable: {
		Code:        ErrCodeRotatedAccentUnreadable,
		Summary:     "The requested rotating accent was unreadable or reserved for negative meaning; a safe fill was substituted.",
		Severity:    "review",
		WhenEmitted: "A pattern using accent_strategy=rotate would otherwise place normal-size lt1 text on a low-contrast accent or use the template's negative semantic accent for neutral content.",
		RemediationSteps: []string{
			"Keep the substituted accent, or supply an explicit accent/semantic_accent if the intended meaning differs.",
			"Choose a template whose accents provide more 4.5:1-safe options for light body text.",
		},
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
		RelatedCodes: []string{ErrCodeDiagramAspectMismatch},
	},
	ErrCodeDiagramAspectMismatch: {
		Code:        ErrCodeDiagramAspectMismatch,
		Summary:     "A diagram's authored (explicit width×height) aspect differs from the post-fit render frame it is sized into by more than 25%.",
		Severity:    "review",
		WhenEmitted: "Pre-flight or render-time finds a diagram with BOTH explicit diagram.width and diagram.height whose authored aspect diverges from the post-fit render frame aspect by >25%. Unset or single-axis (width-only / height-only) diagrams adapt to the frame aspect and are not flagged (a natural-aspect type adapts too, because an explicit output box wins over its natural aspect). fix.params carry authored_*, effective_* (+ dimension_source), cell_* (pre-fit), and render_* (post-fit) aspect evidence plus fit_adjusted so an authoring mistake is distinguishable from a fit-driven mismatch.",
		RemediationSteps: []string{
			"Reshape the cell to match the diagram's pinned aspect ratio (apply repair_slide(kind=reshape_grid)).",
			"Or set cell.fit to 'contain' / 'fit-width' / 'fit-height'.",
			"Or change the explicit diagram.width / diagram.height to match the render frame.",
		},
		RelatedCodes: []string{ErrCodeGridDiagramNarrow},
	},
	// ---- Render-time codes ----

	ErrCodePlaceholderRemapped: {
		Code:        ErrCodePlaceholderRemapped,
		Summary:     "A placeholder_id was implicitly remapped to a different layout placeholder.",
		Severity:    "info",
		WhenEmitted: "Generation resolves an input placeholder_id that the layout does not declare to a usable fallback placeholder. Section subtitles with no real subtitle slot are rejected instead: body slots may hold decorative numbers.",
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
		Summary:     "Table rows were dropped to fit the available height — the hidden rows are absent from the deck.",
		Severity:    "refuse",
		WhenEmitted: "Table layout cannot fit every row in the placeholder height even at the minimum font scale, so trailing rows are replaced by an \"…and N more rows\" cell. This is content loss, not a styling nit: the rows' data is simply not in the deck, so it blocks rather than warns.",
		RemediationSteps: []string{
			"Split the table at the row named in fix.params.split_at_row via repair_slide(kind=split_at_row) — the split point is already computed for you.",
			"Or use the split_slide envelope (by: \"table.rows\") so the engine paginates the rows across slides.",
			"Or reduce the row count upstream, if the trailing rows are genuinely not needed.",
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
		Summary:     "The chart or diagram did not render — the slide carries a grey \"Data unavailable\" placeholder where the visual should be.",
		Severity:    "refuse",
		WhenEmitted: "The render-time svggen call returns an error (an unregistered type, a rejected optional key, or data the type cannot accept) and the engine substitutes a slide-sized placeholder image rather than failing the deck. The visual is lost, so this blocks rather than warns.",
		RemediationSteps: []string{
			"Read the finding's fix.params.reason — for an unrecognised type it names the closest registered type and lists every allowed one.",
			"If the reason names the grid-cell PNG fallback, install rsvg-convert or resvg and retry validation and generation.",
			"Fix the type or the data shape; check it against get_diagram_capabilities or svggen-mcp.validate_diagram, which surfaces the underlying error without a full render.",
			"If the visual you want has no registered type (a combo chart, a sankey, a choropleth), pick a supported type that carries the same argument, or supply the visual as an image.",
		},
		RelatedCodes: []string{ErrCodeChartDataEmpty, ErrCodeContentDropped},
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
		WhenEmitted: "Pre-flight contrast detector walks shape-grid cells with author-specified text and fill colors, and placeholder text on a background the slide itself sets, and predicts a swap using the same replacement algorithm the renderer applies. For authored shape-grid text the executable fix uses replace_color with target=text, a cell path, and from/to matching the authored foreground and predicted replacement. Inherited placeholder text and derived pattern/compose grids have no directly editable source text color, so they have no executable fix; the predicted replacement remains in the message. The required ratio depends on the TEXT SIZE: 4.5:1 for normal text, 3:1 only for genuinely large text (>=18pt, or >=14pt bold).",
		RemediationSteps: []string{
			"Adjust the text color upstream to clear the WCAG AA threshold (≥3:1 against the fill).",
			"For authored shape-grid text, apply the finding's repair_slide replace_color fix with target=text and its cell path to lock in the predicted replacement.",
			"For template-inherited placeholder text, adjust the authored slide background or template; there is no source text color for repair_slide to change.",
			"For pattern or compose slides, adjust the pattern values, overrides, or template; the derived grid cannot be edited by repair_slide.",
			"Or accept the auto-fix — it is non-destructive and matches the render-time contrast_autofixed finding.",
		},
		RelatedCodes: []string{"contrast_autofixed"},
	},
	ErrCodeContrastUnresolved: {
		Code:        ErrCodeContrastUnresolved,
		Summary:     "Placeholder text contrast fails across a gradient or its own fill cannot be resolved; no safe automatic replacement has been applied.",
		Severity:    "refuse",
		WhenEmitted: "Preflight finds text below its size-dependent WCAG AA threshold at a placeholder gradient color, or cannot resolve the placeholder's own solid/gradient fill against the visible canvas.",
		RemediationSteps: []string{
			"Add a sufficiently opaque local backdrop behind the text or darken the full gradient in the template.",
			"Re-run validation and inspect the rendered slide; a single text-color change may make a different stop unreadable.",
		},
		RelatedCodes: []string{ErrCodeContrastPredicted},
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
		WhenEmitted: "Generator's contrast pass detects low-contrast text against the resolved background and swaps the color; a background the author set (background.color or an opaque scrim) snaps to a template text color, a template background is lerped. The finding records before/after colors, ratios, and the rendered path. A replace_color fix with target=text is offered only when a simple raw shape-grid cell can be mapped exactly to authored text; inherited placeholder, pattern, grouped-grid, and ambiguous render surfaces have no executable source-level fix.",
		RemediationSteps: []string{
			"When the finding carries a replace_color fix, apply its target=text and cell path to author the replacement color upstream.",
			"For inherited or generated text without a fix, adjust the source template, pattern values, or background as appropriate.",
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
	"chart.percent_scale_ambiguous": {
		Code:        "chart.percent_scale_ambiguous",
		Summary:     "Percent formatting received values above 1, so their scale is ambiguous.",
		Severity:    "review",
		WhenEmitted: "svggen receives style.value_format.style=percent and at least one formatted value has magnitude above 1; those values are preserved as already-scaled percentage points.",
		RemediationSteps: []string{
			"Use fractional values in [0,1] when percent should multiply by 100 (0.412 → 41.2%).",
			"Or keep already-scaled values and review the rendered labels to confirm that percentage points are intended.",
		},
	},
	"chart.currency_prefix_defaulted": {
		Code:        "chart.currency_prefix_defaulted",
		Summary:     "Currency formatting omitted the currency symbol or code.",
		Severity:    "review",
		WhenEmitted: "svggen receives style.value_format.style=currency without prefix and renders the generic ¤ marker instead of guessing a currency.",
		RemediationSteps: []string{
			"Set style.value_format.prefix to the intended symbol or code, such as $, €, £, or USD.",
		},
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
		Summary:     "Legacy svggen finding for an automatically applied log scale (no longer emitted).",
		Severity:    "info",
		WhenEmitted: "Only in reports generated before the bar-chart linear-default change.",
		RemediationSteps: []string{
			"Regenerate the chart: current svggen keeps bar lengths linear unless style.scale is explicitly log.",
		},
	},
	"chart.wide_range_linear": {
		Code:        "chart.wide_range_linear",
		Summary:     "A bar chart has a wide value range; the linear axis preserves bar-length encoding and value labels are enabled.",
		Severity:    "review",
		WhenEmitted: "Positive values in a non-stacked bar chart span at least 1000x and style.scale is not log.",
		RemediationSteps: []string{
			"Keep the linear axis and visible value labels if exact magnitudes matter.",
			"Split the data into separate charts when the small bars need to be compared visually.",
			"Set style.scale to log only if a logarithmic comparison is intentional; the axis will be labelled.",
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
		Summary:     "Legend entries were dropped because the legend overflowed available space; the chart shows a \"+N more\" row in their place.",
		Severity:    "review",
		WhenEmitted: "svggen's layout pass cannot fit every legend entry. The last row is given to a \"+N more\" marker rather than to one more entry, so the chart itself says something is missing (go-slide-creator-p142). Raised at render AND by the dry-render preflight.",
		RemediationSteps: []string{
			"Reduce the number of series, or group the tail into an \"Other\" category.",
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
	"diagram.items_dropped": {
		Code:        "diagram.items_dropped",
		Summary:     "Authored diagram items did not fit and were omitted from the rendered visual.",
		Severity:    "refuse",
		WhenEmitted: "A Venn circle's item list exceeds the space available in its resolved template or grid-cell frame.",
		RemediationSteps: []string{
			"Reduce or shorten the items in the affected circle.",
			"Widen or heighten the diagram frame, or move the detail to a text column beside it.",
		},
	},
	"diagram.quadrant_position_defaulted": {
		Code:        "diagram.quadrant_position_defaulted",
		Summary:     "A matrix quadrant had no valid position and was placed by its list index.",
		Severity:    "review",
		WhenEmitted: "A matrix_2x2 quadrant omits position or provides a value other than top-left, top-right, bottom-left, or bottom-right (underscore aliases are accepted).",
		RemediationSteps: []string{
			"Set data.quadrants[i].position explicitly to the intended quadrant name.",
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

package patterns

import "errors"

const (
	ErrCodeBaselineMisalign         = "BASELINE_MISALIGN"
	ErrCodeTypeHierarchyFlat        = "TYPE_HIERARCHY_FLAT"
	ErrCodePaneFillImbalance        = "PANE_FILL_IMBALANCE"
	ErrCodeExhibitTooSmall          = "EXHIBIT_TOO_SMALL"
	ErrCodeGutterInconsistent       = "GUTTER_INCONSISTENT"
	ErrCodeNumberFormatInconsistent = "NUMBER_FORMAT_INCONSISTENT"
	ErrCodeTitleOrphan              = "TITLE_ORPHAN"
	ErrCodeDetachedLabel            = "DETACHED_LABEL"
	ErrCodeContinuationUnderfilled  = "CONTINUATION_UNDERFILLED"
	ErrCodeStatusColorsIndistinct   = "STATUS_COLORS_INDISTINCT"
)

func init() {
	fixKindRegistry["align_top_insets"] = FixKindInfo{Kind: "align_top_insets", Class: FixClassAdvisory, Guidance: "Align same-role peer text anchors and top insets using the reported cell paths. Revalidate the composition after editing.", Alternatives: []string{"reshape_grid"}}
	fixKindRegistry["review_layout"] = FixKindInfo{Kind: "review_layout", Class: FixClassAdvisory, Guidance: "Use fix.params.hint and the measured cell/slide evidence to adjust the layout or formatting; this requires authoring judgement."}

	for _, item := range []struct{ code, summary, when, fix, severity string }{
		{ErrCodeBaselineMisalign, "Peer text baselines differ by more than 2pt.", "Measured text in same-role cells of one row starts on different baselines.", "Align top insets and text anchoring across the peer cells.", "review"},
		{ErrCodeTypeHierarchyFlat, "The title/body/caption type hierarchy is flat or inverted.", "Resolved title is within 2pt of body, or a known caption is no smaller than body.", "Restore distinct template-relative title, body and caption sizes.", "review"},
		{ErrCodePaneFillImbalance, "Sibling text panes have substantially different measured fill.", "Comparable sibling panes differ by more than 0.4 in content fill ratio.", "Rebalance pane shares or move supporting text closer to its exhibit.", "review"},
		{ErrCodeExhibitTooSmall, "The only evidence visual occupies less than 35% of the content area.", "Resolved content geometry gives an image or chart a small share of the content area.", "Enlarge the visual or give it a dedicated slide.", "review"},
		{ErrCodeGutterInconsistent, "Unauthored gutters vary across comparable content slides.", "Measured automatic gaps differ by more than 4pt from the deck median.", "Use consistent margins and gutters, or author a deliberate gap.", "info"},
		{ErrCodeNumberFormatInconsistent, "Comparable numbers use conflicting formats across the deck.", "The same numeric/unit context uses conflicting decimal/thousands separators or currency positions.", "Choose one formatter convention per unit and locale.", "info"},
		{ErrCodeTitleOrphan, "A title or heading ends with an orphan or separates a number from its unit.", "Measured wrapping leaves one word on the final line or divides a numeric unit phrase.", "Rewrite the heading or join the number and unit with a non-breaking space.", "info"},
		{ErrCodeDetachedLabel, "A known caption is detached from its parent.", "A comparator or sublabel sits farther from its parent than 1.5 times the grid gap or closer to a different element.", "Move the caption toward its parent and preserve its grouping.", "review"},
		{ErrCodeContinuationUnderfilled, "A split table continuation fills less than half its content frame.", "The actual native table row height leaves most of a continuation empty.", "Merge or rebalance the table pages while preserving merged row groups.", "review"},
		{ErrCodeStatusColorsIndistinct, "Two used status colours are difficult to distinguish.", "Resolved RAG status colours used on one slide have CIE76 deltaE below 20.", "Use distinct status colours and retain text labels or symbols for status meaning.", "review"},
	} {
		codeSentinel[item.code] = errors.New(item.summary)
		findingMetaRegistry[item.code] = FindingMeta{Code: item.code, Summary: item.summary, WhenEmitted: item.when, Severity: item.severity, RemediationSteps: []string{item.fix}}
	}
}

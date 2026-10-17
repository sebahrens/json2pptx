package patterns

// SegmentFit describes structural placement constraints, not a guarantee that
// arbitrary content fits. Omitted minimums mean no fixed minimum is declared;
// expand_pattern still measures the actual content, template and segment.
type SegmentFit struct {
	MinContentWidthPt  float64 `json:"min_content_width_pt,omitempty"`
	MinContentHeightPt float64 `json:"min_content_height_pt,omitempty"`
	// NarrowFallback is legend, stacked, refused, or none. None means no
	// automatic alternate layout is declared, not that every width fits.
	NarrowFallback string `json:"narrow_fallback"`
	// SegmentOnly identifies supporting bands intended for a compose segment
	// or region. This is placement guidance, not a standalone render refusal.
	SegmentOnly bool `json:"segment_only"`
	// FallbackBelowWidthPt is a default-layout transition, not a hard minimum.
	FallbackBelowWidthPt float64 `json:"fallback_below_width_pt,omitempty"`
	// MinLabelColumnWidthPt applies to the remaining label column after the
	// ring, gaps and number cues; it is not the width of the whole segment.
	MinLabelColumnWidthPt float64 `json:"min_label_column_width_pt,omitempty"`
	// ReferenceValueDigitBudget describes full-width default KPI cards on the
	// narrowest shipped template. Actual fonts, glyphs, icons and width matter.
	ReferenceValueDigitBudget int    `json:"reference_value_digit_budget,omitempty"`
	FitNotes                  string `json:"fit_notes,omitempty"`
}

// SegmentFitFor exposes the builders' own thresholds to discovery consumers.
// Keep geometry-dependent transitions conditional rather than inventing one
// minimum slide width for every count, font and label mode.
func SegmentFitFor(p Pattern) *SegmentFit {
	fit := &SegmentFit{NarrowFallback: "none"}
	switch p.Name() {
	case "cycle-figure-eight":
		fit.MinContentWidthPt = cfeMinWidthPt
		fit.MinContentHeightPt = cfeMinHeightPt
		fit.NarrowFallback = "refused"
		fit.FitNotes = "Below the minimum area, use cycle-ring or a full-width segment; actual text still needs fit validation."
	case "cycle-ring":
		fit.FallbackBelowWidthPt = 2*cycleRingReservePt + cycleRingOutsideSidePt
		fit.NarrowFallback = cycleRingLabelsLegend
		fit.FitNotes = "With labels unset, widths below the threshold use a legend. Explicit labels override this transition; descriptions may be omitted with a warning when the legend is too short."
	case "cycle-nodes":
		fit.MinLabelColumnWidthPt = cycleNodesMinLabelColPt
		fit.NarrowFallback = cycleNodesLabelsLegend
		fit.FitNotes = "With labels unset, use a legend when either label column, after the ring, gap and number cue, is narrower than the minimum. Ring size depends on both width and height; explicit labels override this transition."
	case "radial-hub":
		fit.MinLabelColumnWidthPt = rhMinLabelPt
		fit.NarrowFallback = rhLabelsLegend
		fit.FitNotes = "Outside labels fall back to a legend when the ring and two label columns cannot fit, or the hub label does not fit. Counts, words, gaps and height also affect this choice."
	case "cycle-intake":
		fit.MinLabelColumnWidthPt = cycleRingLegendMinPt
		fit.NarrowFallback = "stacked"
		fit.FitNotes = "The intake lane stacks above the loop when the width left for labels after the intake steps, ring and gaps is below the minimum. Intake count and height affect this transition; stacked intake descriptions are omitted with a warning."
	case "kpi-inline", "before-after-compact", "process-flow-compact":
		fit.SegmentOnly = true
		fit.FitNotes = "Supporting band intended for a compose segment or region; alone on a slide it can report SLIDE_UNDERUSED."
	}
	if kpi, ok := p.(*kpiNup); ok && kpi.cfg.Count >= kpiNupTightFromCount {
		fit.ReferenceValueDigitBudget = kpiNupNarrowestBudget[kpi.cfg.Count]
		fit.FitNotes = "Digit budget is for full-width default cards on the narrowest shipped template, not a segment fit guarantee. Actual glyphs, font, icons and card width change the budget; use fewer KPIs in narrow segments and check expand_pattern."
	}
	return fit
}

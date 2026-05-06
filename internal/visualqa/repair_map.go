package visualqa

// categoryFixMap maps visual QA finding categories to candidate repair_slide
// fix kinds, ordered by preference (most targeted first). When the visual QA
// agent detects a category, these are the fix kinds an agent should try via
// repair_slide's autofix_visual kind.
var categoryFixMap = map[string][]string{
	"text_overflow":     {"reduce_cell_text", "split_at_row", "reshape_grid"},
	"text_truncation":   {"reduce_cell_text", "split_at_row", "reshape_grid"},
	"contrast":          {"replace_color", "use_semantic_color"},
	"alignment":         {"reshape_grid", "swap_layout"},
	"spacing":           {"reshape_grid"},
	"overlap":           {"reshape_grid", "swap_layout"},
	"font_size":         {"reduce_text", "reshape_grid"},
	"missing_content":   {"provide_value"},
	"table_readability": {"split_at_row", "reshape_grid"},
	"layout_balance":    {"reshape_grid", "swap_layout"},
	"color_consistency": {"replace_color", "use_semantic_color"},
	"visual_hierarchy":  {"swap_layout", "reshape_grid"},
	"chart_readability": {"reshape_grid"},
	"footer_clearance":  {"reshape_grid"},
}

// SuggestedFixesForCategory returns the candidate repair_slide fix kinds for
// a visual QA finding category. Returns nil for categories with no mapped
// fixes (e.g. image_quality, aspect_ratio, border_style).
func SuggestedFixesForCategory(category string) []SuggestedFix {
	kinds, ok := categoryFixMap[category]
	if !ok {
		return nil
	}
	fixes := make([]SuggestedFix, len(kinds))
	for i, k := range kinds {
		fixes[i] = SuggestedFix{Kind: k}
	}
	return fixes
}

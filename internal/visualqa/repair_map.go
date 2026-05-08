package visualqa

// categoryFixMap maps visual QA finding categories to candidate repair_slide
// fix kinds, ordered by preference (most targeted first). When the visual QA
// agent detects a category, these are the fix kinds an agent should try via
// repair_slide's autofix_visual kind.
//
// Categories intentionally absent from this map have no deterministic
// source-side auto-fix and are review-only:
//   - image_quality: requires human judgement on image replacement
//   - aspect_ratio: requires human decision on cropping or source replacement
//   - border_style: cosmetic; no safe deterministic mutation
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

// ReviewOnlyCategories lists visual QA categories that have no deterministic
// auto-fix mapping. Agents receiving findings in these categories should
// present them for human review rather than attempting repair_slide.
var ReviewOnlyCategories = []string{
	"image_quality",
	"aspect_ratio",
	"border_style",
}

// SuggestedFixesForCategory returns the candidate repair_slide fix kinds for
// a visual QA finding category. Returns nil for categories with no mapped
// fixes (review-only categories like image_quality, aspect_ratio, border_style).
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

// IsReviewOnly reports whether a visual QA category has no deterministic
// auto-fix and should be presented for human review only.
func IsReviewOnly(category string) bool {
	if _, ok := categoryFixMap[category]; ok {
		return false
	}
	// Valid category with no mapping → review-only.
	return ValidCategory(category)
}

package main

// repairFixKindParams is the per-kind parameter catalogue for repair_slide's
// executable fix kinds. It used to be ~5KB of repair_slide's tool description,
// paid on every tools/list whether or not the agent ever repaired a raw slide
// (go-slide-creator-fa3k8); it is now served on demand by
// get_capabilities sections:["vocabularies"] as repair_fix_kind_params.
// TestRepairFixKindParamsCoverExecutableKinds keeps it in step with the
// registry.
var repairFixKindParams = map[string]string{
	// Text / title fits
	"reduce_text":      "Truncate bullets/body text. Params: path (optional), max_items (int, bullets), max_length (int, text).",
	"shorten_title":    "Truncate a title. Params: path (optional), max_length or max_chars (int; max_chars is emitted by measured title-fit findings).",
	"reduce_cell_text": "Truncate a shape_grid cell, preserving markdown emphasis and ending with an ellipsis. Params: cell_path (JSON Pointer), max_chars (int).",
	"renumber_bullets": "Number bullets sequentially, or remove typed numbers. Params: path (optional), strip (bool, optional).",
	// Layout / pagination
	"split_at_row":  "Split a table across pages (split_slide envelope). Params: path (optional), row (int, rows per page), title_suffix (optional), repeat_headers (bool, optional).",
	"split_bullets": "Split plain bullet columns verbatim across sibling slides, keeping nested children with parents. Params: max_items (positive int, per-column page budget). Requires equal nonempty column lengths; no path targeting or compound bullet content; refuses decks with numeric internal slide links. Other content repeats; notes/source stay on page one. Render every resulting page.",
	"swap_layout":   "Change the slide's layout_id. Params: layout_id (string, required).",
	// Color / theme
	"use_one_of":         "Replace a field value with a valid option. Params: path (string), value (string).",
	"replace_color":      "Replace one color in shape_grid fills or text. Params: from, to (colors), target (\"fill\" default or \"text\"), path (optional grid-cell JSON Pointer). Also accepts original_color/replacement_color from contrast findings.",
	"use_semantic_color": "Replace a hex fill with a scheme color. Params: path (JSON Pointer, e.g. /slides/0/shape_grid/rows/0/cells/0/shape/fill), value (scheme name, e.g. accent1).",
	// Pattern shape
	"split_pattern":      "Split a pattern slide. Params: first (count on slide 1; default half), title_part_2 (suffix), path (optional values-array key).",
	"swap_pattern":       "Replace the slide's pattern. Params: to (required pattern name), values, overrides, cell_overrides (objects, optional).",
	"reshape_grid":       "Adjust rows/columns. Params: rows (int), columns (int or []int); at least one required.",
	"set_pattern_style":  "Change the style variant in a pattern's overrides (e.g. timeline-horizontal dots to chevron). Params: style (string, required).",
	"set_max_height_pct": "Cap pattern height to avoid overtall lanes. Params: max_height_pct (0 < n <= 100; ~35 for a sparse row).",
	// Pattern values (field-level edits to slide.pattern.values)
	"rename_field":  "Rename a top-level key in pattern values (or slide-level fields). Params: from, to (strings, required).",
	"reshape_value": "Replace an existing pattern-values field with a restructured value (e.g. array to object). Params: path (key in pattern.values), value (any), both required.",
	"provide_value": "Set a pattern-values field, creating the key if missing. Params: path (key in pattern.values), value (any), both required.",
	"replace_value": "Replace an existing pattern-values field (typically to bring it within bounds). Params: path (key in pattern.values), value (any), both required.",
	"reduce_items":  "Truncate an array. Params: path (array key), max_items (>0), confirm_semantic_change (bool). Fact loss requires confirmation or split_pattern.",
	"add_items":     "Append items to a pattern-values array (created if missing). Params: path (array key), items (array), both required.",
	"resize_list":   "Resize an array. Params: path (array key), count (>0), confirm_semantic_change (bool). Too few items requires add_items; fact loss is guarded.",
	"remove_key":    "Remove a key from pattern overrides or values (overrides checked first). Params: key (string, required).",
	"remove_field":  "Remove a top-level field from pattern values or slide-level fields. Params: path (field name, required).",
	// Heuristic
	"autofix_visual": "Apply a heuristic fix for a visual QA category, trying each candidate fix kind in order until one succeeds. Params: category (required, e.g. text_overflow, contrast); other params are forwarded.",
}

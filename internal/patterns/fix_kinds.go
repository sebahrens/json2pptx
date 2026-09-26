// fix_kinds.go is the single registry of fix kinds (go-slide-creator-ui4c).
//
// Every finding that carries a Fix names a kind, and agents were told — by
// get_capabilities.vocabularies.repair_fix_kinds and by the documented loop
// score_deck → propose_repairs → repair_slide — that a fix is something
// repair_slide executes. It is not: 506 of 814 fix-carrying findings across a
// 50-deck corpus named a kind repair_slide cannot apply, and they were exactly
// the findings that fire on the *good* decks (cell_underfilled,
// SPARSE_FILL, SLIDE_UNDERUSED). repair_slide answered
// "kind_not_supported" and propose_repairs filed them under
// unmapped/"fix_kind_not_repairable", so the loop terminated with zero
// directives and the only remaining signal was prose in fix.params.hint.
//
// The vocabulary is therefore explicitly two-class:
//
//   - executable: repair_slide applies it. These are the kinds
//     get_capabilities.vocabularies.repair_fix_kinds advertises.
//   - advisory: the remedy needs authoring judgement (add detail, merge
//     slides, pick a different pattern) or a human eye. repair_slide cannot
//     apply it and never pretends to; the registry carries the instruction and
//     the executable kinds that address the same problem, so an advisory
//     finding still chains forward.
//
// Adding a fix kind anywhere in the codebase means adding it here.
// TestEveryEmittedFixKindIsRegistered fails otherwise.
package patterns

import "sort"

// FixKindClass is how a fix kind reaches the deck.
type FixKindClass string

const (
	// FixClassExecutable: repair_slide applies the directive directly.
	FixClassExecutable FixKindClass = "executable"
	// FixClassAdvisory: the remedy is authoring judgement, not a mechanical
	// edit. Guidance says what to do; Alternatives name executable kinds that
	// address the same problem.
	FixClassAdvisory FixKindClass = "advisory"
)

// FixKindInfo describes one fix kind.
type FixKindInfo struct {
	Kind  string       `json:"kind"`
	Class FixKindClass `json:"class"`
	// Guidance is the instruction for an advisory kind: what the agent or
	// author has to decide. Empty for executable kinds, whose params are the
	// instruction.
	Guidance string `json:"guidance,omitempty"`
	// Alternatives are executable kinds that address the same defect, for an
	// agent that wants a mechanical edit instead of an authoring decision.
	Alternatives []string `json:"alternatives,omitempty"`
}

// fixKindRegistry is the authoritative vocabulary. Keep it alphabetical.
var fixKindRegistry = map[string]FixKindInfo{
	// ---- executable: repair_slide applies these ----
	"add_items":          {Kind: "add_items", Class: FixClassExecutable},
	"autofix_visual":     {Kind: "autofix_visual", Class: FixClassExecutable},
	"provide_value":      {Kind: "provide_value", Class: FixClassExecutable},
	"reduce_cell_text":   {Kind: "reduce_cell_text", Class: FixClassExecutable},
	"reduce_items":       {Kind: "reduce_items", Class: FixClassExecutable},
	"reduce_text":        {Kind: "reduce_text", Class: FixClassExecutable},
	"remove_field":       {Kind: "remove_field", Class: FixClassExecutable},
	"renumber_bullets":   {Kind: "renumber_bullets", Class: FixClassExecutable},
	"remove_key":         {Kind: "remove_key", Class: FixClassExecutable},
	"rename_field":       {Kind: "rename_field", Class: FixClassExecutable},
	"replace_color":      {Kind: "replace_color", Class: FixClassExecutable},
	"replace_value":      {Kind: "replace_value", Class: FixClassExecutable},
	"reshape_grid":       {Kind: "reshape_grid", Class: FixClassExecutable},
	"reshape_value":      {Kind: "reshape_value", Class: FixClassExecutable},
	"resize_list":        {Kind: "resize_list", Class: FixClassExecutable},
	"set_max_height_pct": {Kind: "set_max_height_pct", Class: FixClassExecutable},
	"set_pattern_style":  {Kind: "set_pattern_style", Class: FixClassExecutable},
	"shorten_title":      {Kind: "shorten_title", Class: FixClassExecutable},
	"split_at_row":       {Kind: "split_at_row", Class: FixClassExecutable},
	"split_bullets":      {Kind: "split_bullets", Class: FixClassExecutable},
	"split_pattern":      {Kind: "split_pattern", Class: FixClassExecutable},
	"swap_layout":        {Kind: "swap_layout", Class: FixClassExecutable},
	"swap_pattern":       {Kind: "swap_pattern", Class: FixClassExecutable},
	"use_one_of":         {Kind: "use_one_of", Class: FixClassExecutable},
	"use_semantic_color": {Kind: "use_semantic_color", Class: FixClassExecutable},

	// ---- advisory: the remedy is a decision, not an edit ----
	"add_detail_or_resize": {
		Kind:         "add_detail_or_resize",
		Class:        FixClassAdvisory,
		Guidance:     "The shape is much larger than its text. Add the supporting detail the box was sized for, cap the grid height (pattern.max_height_pct or explicit bounds) so it shrinks to its content, or use a compact pattern variant. No mechanical edit can invent the missing content.",
		Alternatives: []string{"set_max_height_pct", "reshape_grid", "swap_pattern"},
	},
	"adopt_pattern": {
		Kind:         "adopt_pattern",
		Class:        FixClassAdvisory,
		Guidance:     "A raw shape_grid is doing work a named pattern does better. Call recommend_visual with the slide's intent and item_count, then author the winning pattern.",
		Alternatives: []string{"swap_pattern"},
	},
	"choose_template": {
		Kind:     "choose_template",
		Class:    FixClassAdvisory,
		Guidance: "The requested template cannot be resolved or parsed. Call list_templates, choose a listed template, and replace meta.template before compiling or rendering.",
	},
	"choose_kind": {
		Kind:     "choose_kind",
		Class:    FixClassAdvisory,
		Guidance: "The slide kind is not recognized. Call list_slide_kinds, choose a supported kind, and replace the slide's kind in the semantic spec.",
	},
	"choose_template_or_remove_requirement": {
		Kind:     "choose_template_or_remove_requirement",
		Class:    FixClassAdvisory,
		Guidance: "The selected template does not provide a required native layout. Choose a template that provides it, or remove that layout from meta.required_layouts if it is not essential to the brief.",
	},
	"consolidate_accents": {
		Kind:         "consolidate_accents",
		Class:        FixClassAdvisory,
		Guidance:     "Too many accent colors compete on one slide. Decide which single element deserves the accent and set the rest to a neutral role.",
		Alternatives: []string{"replace_color", "use_semantic_color"},
	},
	"differentiate_title": {
		Kind:     "differentiate_title",
		Class:    FixClassAdvisory,
		Guidance: "Rewrite the title at the finding's path so this slide announces a distinct point. Preserve the slide's actual claim; do not merely truncate the shared title or append a slide number. If the slides make the same point, merge them instead.",
	},
	"fix_structure": {
		Kind:     "fix_structure",
		Class:    FixClassAdvisory,
		Guidance: "The deck's structure (missing opener, no closing, section imbalance) needs an editorial decision about which slides to add, merge, or drop.",
	},
	"grow_pattern": {
		Kind:         "grow_pattern",
		Class:        FixClassAdvisory,
		Guidance:     "The grid's content is far shorter than the space it was given. Add content, reshape the grid to fewer/denser cells, or cap its height so the boxes stop stretching.",
		Alternatives: []string{"reshape_grid", "set_max_height_pct"},
	},
	"increase_gap": {
		Kind:         "increase_gap",
		Class:        FixClassAdvisory,
		Guidance:     "Elements sit too close to read as separate. Widen the gap in the pattern's overrides or move to a layout with more room.",
		Alternatives: []string{"reshape_grid", "swap_layout"},
	},
	"increase_row_height": {
		Kind:         "increase_row_height",
		Class:        FixClassAdvisory,
		Guidance:     "A row is too short for its content. Raise that row's height in the grid, or move content out of it.",
		Alternatives: []string{"reshape_grid", "reduce_cell_text"},
	},
	"provide_data": {
		Kind:     "provide_data",
		Class:    FixClassAdvisory,
		Guidance: "The chart or table has no data to plot. Supply the real series/rows — the engine will not invent numbers.",
	},
	"provide_native_format": {
		Kind:     "provide_native_format",
		Class:    FixClassAdvisory,
		Guidance: "The value is in a shape this content type cannot read. Re-author it in the type's native format (see get_data_format_hints / list_templates).",
	},
	"provide_numeric_value": {
		Kind:     "provide_numeric_value",
		Class:    FixClassAdvisory,
		Guidance: "A numeric field carries text. Supply the number (the unit belongs in its own field or the label).",
	},
	"reduce_columns": {
		Kind:         "reduce_columns",
		Class:        FixClassAdvisory,
		Guidance:     "The table has more columns than the slide can render legibly. Decide which columns carry the argument and drop the rest, or split the table across slides — dropping columns silently would lose data.",
		Alternatives: []string{"split_at_row", "swap_layout"},
	},
	"remap_placeholder": {
		Kind:         "remap_placeholder",
		Class:        FixClassAdvisory,
		Guidance:     "The requested placeholder does not exist on the chosen layout. Pick a placeholder the layout provides (list_templates layout_summaries) or a layout that has this one.",
		Alternatives: []string{"swap_layout", "rename_field"},
	},
	"remove_emoji": {
		Kind:         "remove_emoji",
		Class:        FixClassAdvisory,
		Guidance:     "Emoji do not render reliably in PowerPoint text. Rewrite the string without them, or use a bundled icon for the same signal.",
		Alternatives: []string{"replace_value"},
	},
	"remove_field_or_switch_pattern": {
		Kind:         "remove_field_or_switch_pattern",
		Class:        FixClassAdvisory,
		Guidance:     "The field is not part of this pattern's contract. Drop it, or move to a pattern that models it (show_pattern lists each pattern's fields).",
		Alternatives: []string{"remove_field", "swap_pattern"},
	},
	"replace_placeholder": {
		Kind:     "replace_placeholder",
		Class:    FixClassAdvisory,
		Guidance: "The text is exemplar/placeholder copy, not content. Replace it with the deck's real message.",
	},
	"reposition_shape": {
		Kind:         "reposition_shape",
		Class:        FixClassAdvisory,
		Guidance:     "A shape overruns the slide, the footer band, or the title. Move or resize it within the content area, or move to a layout that fits it.",
		Alternatives: []string{"reshape_grid", "swap_layout", "set_max_height_pct"},
	},
	"restore_visual": {
		Kind:         "restore_visual",
		Class:        FixClassAdvisory,
		Guidance:     "The slide's content will not fit the visual its kind promised, so it renders as bullets (or a plain content slide) instead. params.to says what you get and params.reason why; bring the count into range or shorten the over-budget text to keep params.from, or accept the fallback.",
		Alternatives: []string{"reduce_items", "add_items", "reduce_text", "split_pattern"},
	},
	"review": {
		Kind:     "review",
		Class:    FixClassAdvisory,
		Guidance: "Look at the rendered slide and judge it: the engine has flagged something it cannot decide for you (a scaled table font, a chart that rendered thin, a headline that reads long).",
	},
	"review_layout": {
		Kind:         "review_layout",
		Class:        FixClassAdvisory,
		Guidance:     "The template lacks a real layout for this slide, so one was synthesized. Check the rendered slide, or author the layout in the template.",
		Alternatives: []string{"swap_layout"},
	},
	"rewrite_field": {
		Kind:         "rewrite_field",
		Class:        FixClassAdvisory,
		Guidance:     "The text does not fit and cutting it would lose the point. Rewrite it shorter yourself.",
		Alternatives: []string{"replace_value", "reduce_text"},
	},
	"set_design_mode_free": {
		Kind:     "set_design_mode_free",
		Class:    FixClassAdvisory,
		Guidance: "The deck asked for something the current design mode forbids. Set design_mode to \"free\" deliberately, or stay inside the mode's constraints.",
	},
	"simplify_or_enlarge_diagram": {
		Kind:         "simplify_or_enlarge_diagram",
		Class:        FixClassAdvisory,
		Guidance:     "The embedded diagram has text below the viewing-mode readability floor at its actual slide size. Remove low-priority labels or nodes, give the diagram a larger cell, or use a slide layout with more room.",
		Alternatives: []string{"reshape_grid", "swap_layout"},
	},
	"shrink_text": {
		Kind:         "shrink_text",
		Class:        FixClassAdvisory,
		Guidance:     "The text needs less of it or a smaller size. Decide whether to cut words (reduce_text) or accept a smaller font on this slide.",
		Alternatives: []string{"reduce_text", "reduce_cell_text"},
	},
	"text": {
		Kind:     "text",
		Class:    FixClassAdvisory,
		Guidance: "Free-form guidance in params.message — read it and decide. (Legacy shape; new findings use a structured kind.)",
	},
	"truncation_summary": {
		Kind:         "truncation_summary",
		Class:        FixClassAdvisory,
		Guidance:     "Content was dropped or truncated to fit. Decide what to cut or where to split — accepting silent truncation loses author content.",
		Alternatives: []string{"split_at_row", "split_pattern", "reduce_text"},
	},
	"widen_shape_text_area": {
		Kind:         "widen_shape_text_area",
		Class:        FixClassAdvisory,
		Guidance:     "The shape geometry and insets leave too little width for a word. Patch the pattern's height or step type, or widen/change the raw shape. Truncating content is not a mechanical geometry fix.",
		Alternatives: []string{"set_max_height_pct", "reshape_grid", "swap_pattern"},
	},
}

// FixKind returns the registry entry for a kind.
func FixKind(kind string) (FixKindInfo, bool) {
	info, ok := fixKindRegistry[kind]
	return info, ok
}

// KnownFixKind reports whether the kind is in the vocabulary at all. An unknown
// kind is a bug (a finding invented a kind), not an advisory.
func KnownFixKind(kind string) bool {
	_, ok := fixKindRegistry[kind]
	return ok
}

// FixKindIsExecutable reports whether repair_slide can apply the kind.
func FixKindIsExecutable(kind string) bool {
	info, ok := fixKindRegistry[kind]
	return ok && info.Class == FixClassExecutable
}

// FixKindIsAdvisory reports whether the kind is a registered advisory: known,
// but not something repair_slide executes.
func FixKindIsAdvisory(kind string) bool {
	info, ok := fixKindRegistry[kind]
	return ok && info.Class == FixClassAdvisory
}

// ExecutableFixKinds returns the sorted executable vocabulary — the authoritative
// source for get_capabilities.vocabularies.repair_fix_kinds and for
// repair_slide's supported_kinds.
func ExecutableFixKinds() []string { return fixKindsOfClass(FixClassExecutable) }

// AdvisoryFixKinds returns the sorted advisory vocabulary.
func AdvisoryFixKinds() []string { return fixKindsOfClass(FixClassAdvisory) }

func fixKindsOfClass(class FixKindClass) []string {
	out := make([]string, 0, len(fixKindRegistry))
	for kind, info := range fixKindRegistry {
		if info.Class == class {
			out = append(out, kind)
		}
	}
	sort.Strings(out)
	return out
}

// AllFixKinds returns every registered kind, sorted.
func AllFixKinds() []string {
	out := make([]string, 0, len(fixKindRegistry))
	for kind := range fixKindRegistry {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

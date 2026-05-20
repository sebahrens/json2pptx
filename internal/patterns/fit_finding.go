package patterns

import "sort"

// Extent represents a measured or allowed dimension in EMU.
type Extent struct {
	WidthEMU  int64 `json:"width_emu"`
	HeightEMU int64 `json:"height_emu"`
}

// ToolCallSuggestion is a machine-readable next-step for an agent: the exact
// MCP tool to call and a template for its arguments. Agents can invoke the
// tool directly without inferring the protocol from prose.
type ToolCallSuggestion struct {
	Tool         string         `json:"tool"`
	ArgsTemplate map[string]any `json:"args_template"`
}

// FitFinding is a fit-report finding that embeds ValidationError for
// unification with the existing validation envelope. The embedding flattens
// all ValidationError fields to the top level in JSON output.
type FitFinding struct {
	ValidationError

	// Action is the recommended remediation: "refuse", "shrink_or_split",
	// "review", or "info", ranked from most to least severe.
	Action string `json:"action"`

	// Measured is the actual extent of the content (nil when not applicable).
	Measured *Extent `json:"measured,omitempty"`

	// Allowed is the available extent for the content (nil when not applicable).
	Allowed *Extent `json:"allowed,omitempty"`

	// OverflowRatio is measured/allowed as a fraction (e.g. 1.25 means 25%
	// over). Zero when extents are not available.
	OverflowRatio float64 `json:"overflow_ratio,omitempty"`

	// NextToolCall is a machine-readable suggestion for the next MCP tool call
	// that would resolve this finding. Populated for findings with action
	// "refuse", "shrink_or_split", or "review".
	NextToolCall *ToolCallSuggestion `json:"next_tool_call,omitempty"`

	// SegmentIndex is the 0-based child segment index inside a compose envelope
	// that the finding is attributable to. Nil when the finding is not
	// segment-scoped (i.e. the slide is not a compose slide or the finding is
	// emitted against the merged grid rather than a specific segment).
	SegmentIndex *int `json:"segment_index,omitempty"`
}

// actionRanks maps action strings to severity ranks. Higher rank = more severe.
var actionRanks = map[string]int{
	"info":           0,
	"review":         1,
	"shrink_or_split": 2,
	"refuse":         3,
}

// ActionRank returns the severity rank for the given action string.
// Unknown actions return -1.
func ActionRank(action string) int {
	rank, ok := actionRanks[action]
	if !ok {
		return -1
	}
	return rank
}

// repairFixKinds lists fix kinds that map to the repair_slide tool.
var repairFixKinds = map[string]bool{
	"reduce_text":         true,
	"split_at_row":        true,
	"shorten_title":       true,
	"replace_color":       true,
	"use_semantic_color":  true,
	"split_pattern":       true,
	"swap_layout":         true,
	"swap_pattern":        true,
	"use_one_of":          true,
	"rename_field":        true,
	"reshape_grid":        true,
	"reshape_value":       true,
	"set_pattern_style":   true,
	"provide_value":       true,
	"replace_value":       true,
	"reduce_items":        true,
	"add_items":           true,
	"resize_list":         true,
	"remove_key":          true,
	"remove_field":        true,
	"autofix_visual":      true,
}

// RepairToolCall builds a ToolCallSuggestion for the repair_slide tool from a
// fix suggestion and slide index. Returns nil if the fix kind is not a
// repair_slide kind.
func RepairToolCall(slideIdx int, fix *FixSuggestion) *ToolCallSuggestion {
	if fix == nil {
		return nil
	}
	if !repairFixKinds[fix.Kind] {
		return nil
	}

	fixDirective := map[string]any{"kind": fix.Kind}
	if len(fix.Params) > 0 {
		fixDirective["params"] = fix.Params
	}

	return &ToolCallSuggestion{
		Tool: "repair_slide",
		ArgsTemplate: map[string]any{
			"slide_index": slideIdx,
			"fixes":       []any{fixDirective},
		},
	}
}

// RecommendToolCall builds a ToolCallSuggestion for the recommend_pattern tool.
// Used when a finding suggests adopting or switching patterns.
func RecommendToolCall(itemCount int) *ToolCallSuggestion {
	return &ToolCallSuggestion{
		Tool: "recommend_pattern",
		ArgsTemplate: map[string]any{
			"item_count": itemCount,
		},
	}
}

// SortCanonical sorts findings in place into the canonical serialization order:
//
//  1. severity descending (highest ActionRank first — refuse > shrink_or_split > review > info)
//  2. slide index ascending (path-derived; -1 for deck-level findings sorts before slide 0)
//  3. code ascending (lexicographic, stable tiebreaker)
//
// This is the invariant every fit_report / findings array must satisfy before
// it crosses a serialization boundary so agents can rely on findings[0] being
// the most important fix, with deterministic ordering across runs and tools.
//
// slideIndexFn extracts the 0-based slide index from a finding's path; pass
// slidepath.SlideIndex (or an equivalent extractor) from the caller.
func SortCanonical(findings []FitFinding, slideIndexFn func(string) int) {
	if len(findings) <= 1 {
		return
	}
	sort.Slice(findings, func(i, j int) bool {
		ri := ActionRank(findings[i].Action)
		rj := ActionRank(findings[j].Action)
		if ri != rj {
			return ri > rj
		}
		si := slideIndexFn(findings[i].Path)
		sj := slideIndexFn(findings[j].Path)
		if si != sj {
			return si < sj
		}
		return findings[i].Code < findings[j].Code
	})
}

// AttachNextToolCalls populates NextToolCall on each finding whose action is
// not "info", using the fix kind and path to determine the appropriate tool.
// slideIndexFn extracts the 0-based slide index from a finding's path.
func AttachNextToolCalls(findings []FitFinding, slideIndexFn func(string) int) {
	for i := range findings {
		f := &findings[i]
		if f.Action == "info" || f.NextToolCall != nil {
			continue
		}
		if f.Fix == nil {
			continue
		}

		slideIdx := slideIndexFn(f.Path)

		switch f.Fix.Kind {
		case "adopt_pattern":
			// Extract item count from params if available.
			itemCount := 0
			if n, ok := f.Fix.Params["filled_slots"].(int); ok {
				itemCount = n
			}
			f.NextToolCall = RecommendToolCall(itemCount)
		case "swap_pattern":
			// swap_pattern suggests switching to a different pattern — point
			// to recommend_pattern for the agent to pick the right one.
			itemCount := 0
			f.NextToolCall = RecommendToolCall(itemCount)
		default:
			f.NextToolCall = RepairToolCall(slideIdx, f.Fix)
		}
	}
}

// mcp_next_call_profile.go keeps next_tool_call suggestions callable under the
// active tool profile (go-slide-creator-mvny).
//
// A suggestion naming a tool the profile does not advertise is worse than no
// suggestion at all: a client model cannot emit a call to a tool that never
// appeared in tools/list, so the chain dead-ends exactly where the agent needs
// help. Every suggestion that reaches an agent therefore passes through
// substituteUnadvertised, which swaps in the in-profile tool that does the same
// job (translating the args, since the substitute's schema differs) or drops the
// suggestion when the profile has no equivalent.
package main

import "github.com/sebahrens/json2pptx/internal/patterns"

// suggestionSubstitutes maps a tool that the core profile hides to a builder for
// the in-profile tool that serves the same purpose. The builder receives the
// original args_template so it can carry over what still applies.
//
// Only tools that a next_tool_call actually names need an entry; a tool with no
// entry is dropped rather than guessed at.
var suggestionSubstitutes = map[string]func(args map[string]any) *patterns.ToolCallSuggestion{
	// recommend_pattern ranks named patterns only; recommend_visual ranks
	// patterns, layouts, charts and diagrams through one intent string.
	"recommend_pattern": func(args map[string]any) *patterns.ToolCallSuggestion {
		hints := map[string]any{}
		if n, ok := args["item_count"]; ok {
			hints["item_count"] = n
		}
		return &patterns.ToolCallSuggestion{
			Tool: "recommend_visual",
			ArgsTemplate: map[string]any{
				"intent":        "<one sentence: what this slide should show>",
				"content_hints": hints,
			},
		}
	},
	// read_presentation reports what a PPTX contains; rendering it to pixels is
	// the in-profile way to see the same thing.
	"read_presentation": func(args map[string]any) *patterns.ToolCallSuggestion {
		return renderThumbnailsSuggestion(args, "pptx_path", "path")
	},
	// validate_presentation_output explains why a package is malformed; in core,
	// a render that fails on the same file confirms it is not openable.
	"validate_presentation_output": func(args map[string]any) *patterns.ToolCallSuggestion {
		return renderThumbnailsSuggestion(args, "path", "pptx_path")
	},
	// Folded aliases (go-slide-creator-fa3k8): the successor does the job.
	"render_slide_image": func(args map[string]any) *patterns.ToolCallSuggestion {
		s := renderThumbnailsSuggestion(args, "pptx_path")
		if idx, ok := args["slide_index"]; ok {
			s.ArgsTemplate["slide_indices"] = []any{idx}
		}
		return s
	},
	// repair_slide takes one slide's fixes; carry the batch's first slide.
	"repair_slides_batch": func(args map[string]any) *patterns.ToolCallSuggestion {
		out := map[string]any{"slide_index": 0}
		for _, k := range []string{"presentation", "deck_id"} {
			if v, ok := args[k]; ok {
				out[k] = v
			}
		}
		var fixes []any
		list, _ := args["fixes"].([]any)
		for i, raw := range list {
			f, _ := raw.(map[string]any)
			if f == nil {
				continue
			}
			if i == 0 {
				if idx, ok := f["slide_index"]; ok {
					out["slide_index"] = idx
				}
			}
			if f["slide_index"] != out["slide_index"] {
				continue
			}
			fix := map[string]any{"kind": f["kind"]}
			if p, ok := f["params"]; ok {
				fix["params"] = p
			}
			fixes = append(fixes, fix)
		}
		if len(fixes) == 0 {
			fixes = []any{map[string]any{"kind": "<fix kind>", "params": map[string]any{}}}
		}
		out["fixes"] = fixes
		return &patterns.ToolCallSuggestion{Tool: "repair_slide", ArgsTemplate: out}
	},
	"get_chart_capabilities":   fullTemplatesSuggestion,
	"get_diagram_capabilities": fullTemplatesSuggestion,
}

// fullTemplatesSuggestion points at list_templates fields="full", whose
// supported_types carries chart_capabilities and diagram_capabilities.
func fullTemplatesSuggestion(map[string]any) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool:         "list_templates",
		ArgsTemplate: map[string]any{"fields": "full", "page_size": 1},
	}
}

// renderThumbnailsSuggestion builds a render_deck_thumbnails suggestion, reusing
// the original file path from whichever arg key carried it.
func renderThumbnailsSuggestion(args map[string]any, keys ...string) *patterns.ToolCallSuggestion {
	path := any("<path to the .pptx>")
	for _, k := range keys {
		if v, ok := args[k]; ok {
			path = v
			break
		}
	}
	return &patterns.ToolCallSuggestion{
		Tool:         "render_deck_thumbnails",
		ArgsTemplate: map[string]any{"pptx_path": path},
	}
}

// substituteUnadvertised returns s unchanged when the active profile advertises
// s.Tool, the in-profile equivalent when one is defined, or nil when the profile
// offers nothing that does the job.
func substituteUnadvertised(s *patterns.ToolCallSuggestion) *patterns.ToolCallSuggestion {
	if s == nil || toolIsAdvertised(s.Tool) {
		return s
	}
	build, ok := suggestionSubstitutes[s.Tool]
	if !ok {
		return nil
	}
	sub := build(s.ArgsTemplate)
	if sub == nil || !toolIsAdvertised(sub.Tool) {
		return nil
	}
	return sub
}

// retargetUnadvertisedSuggestions applies substituteUnadvertised to every
// finding's next_tool_call in place. Call it after the suggestions are attached,
// on any path that serializes findings to an agent.
func retargetUnadvertisedSuggestions(findings []patterns.FitFinding) {
	for i := range findings {
		findings[i].NextToolCall = substituteUnadvertised(findings[i].NextToolCall)
	}
}

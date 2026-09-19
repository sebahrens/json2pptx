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
				"intent": "<one sentence: what this slide should show>",
				"hints":  hints,
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

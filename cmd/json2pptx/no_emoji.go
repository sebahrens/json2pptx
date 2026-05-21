package main

import (
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// noEmojiDiagnostics converts no-emoji FitFindings into Diagnostics with a
// next_tool_call hint that points authors back to validate_input so they can
// re-check after stripping emoji.
//
// The emoji-detection predicate, sanitizer, and FitFinding builder live in
// internal/policy/emoji so every producer (this CLI, internal/testrand, and
// future generators) and this enforcer share one implementation. This file is
// the thin diagnostics-layer adapter that the engine's generate/validate
// boundary uses; call emoji.ValidateNoEmojiInText to produce the findings.
func noEmojiDiagnostics(violations []patterns.FitFinding) []diagnostics.Diagnostic {
	diags := make([]diagnostics.Diagnostic, 0, len(violations))
	for _, v := range violations {
		d := diagnostics.Diagnostic{
			Code:     "no_emoji_violation",
			Path:     v.Path,
			Message:  v.Message,
			Severity: diagnostics.SeverityError,
			NextToolCall: &patterns.ToolCallSuggestion{
				Tool: "validate_input",
				ArgsTemplate: map[string]any{
					"presentation": "<re-submit after removing emoji codepoints>",
				},
			},
		}
		if v.Fix != nil {
			d.Fix = &diagnostics.Fix{Kind: v.Fix.Kind, Params: v.Fix.Params}
		}
		diags = append(diags, d)
	}
	return diags
}

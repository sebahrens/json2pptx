package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
)

// placeholderPolicyDefault is the standing policy when the caller does not set
// placeholder_policy: surface unresolved __FILL__ tokens as warnings (with JSON
// paths) so the agent knows which fields still need real content, but do not
// block. Publishable/gated callers opt into "strict" to refuse the deck until
// every token is replaced. This mirrors the strict_fit graduated default.
const placeholderPolicyDefault = "warn"

// placeholderPolicyFromRequest reads the placeholder_policy argument
// (off|warn|strict). Unset or unrecognized values fall back to the default
// (warn) so the scan still surfaces unfinished content. Returns the effective
// policy.
func placeholderPolicyFromRequest(request mcp.CallToolRequest) string {
	if v, err := request.RequireString("placeholder_policy"); err == nil && v != "" {
		return normalizePlaceholderPolicy(v)
	}
	return placeholderPolicyDefault
}

// normalizePlaceholderPolicy clamps an arbitrary policy string to one of the
// three supported modes, defaulting unknown values to warn.
func normalizePlaceholderPolicy(policy string) string {
	switch policy {
	case "off", "warn", "strict":
		return policy
	default:
		return placeholderPolicyDefault
	}
}

// placeholderDiagnostics converts unresolved-placeholder violations into
// Diagnostics. When blocking is true (placeholder_policy=strict) the findings
// are error-severity and fail validation / refuse generation; otherwise they
// are warnings that surface the offending paths without blocking.
//
// Each diagnostic carries the JSON path, a structured replace_placeholder fix,
// and a next_tool_call that re-runs validate_input after the tokens are
// replaced — the "suggested next action" the acceptance criteria require.
func placeholderDiagnostics(violations []placeholder.Violation, blocking bool) []diagnostics.Diagnostic {
	if len(violations) == 0 {
		return nil
	}
	severity := diagnostics.SeverityWarning
	if blocking {
		severity = diagnostics.SeverityError
	}
	diags := make([]diagnostics.Diagnostic, 0, len(violations))
	for _, v := range violations {
		diags = append(diags, diagnostics.Diagnostic{
			Code:     placeholder.FindingCode,
			Path:     v.Path,
			Message:  placeholderMessage(v, blocking),
			Severity: severity,
			Fix: &diagnostics.Fix{
				Kind: "replace_placeholder",
				Params: map[string]any{
					"path":  v.Path,
					"token": v.Token,
					"hint":  "overwrite the " + placeholder.Token + " token with the slide's real content — plan_deck skeletons are scaffolding, not finished text",
				},
			},
			NextToolCall: &patterns.ToolCallSuggestion{
				Tool: "validate_input",
				ArgsTemplate: map[string]any{
					"presentation": "<re-submit after replacing every " + placeholder.Token + " token>",
				},
			},
		})
	}
	return diags
}

// placeholderMessage builds the human-readable finding message. The strict
// variant explains that the deck was refused; the warn variant explains how to
// escalate to a blocking gate for publishable output.
func placeholderMessage(v placeholder.Violation, blocking bool) string {
	if blocking {
		return fmt.Sprintf(
			"%s still holds the unresolved skeleton placeholder %q — replace it with real content (placeholder_policy=strict refuses unfinished decks)",
			placeholder.DisplayPath(v.Path), v.Token)
	}
	return fmt.Sprintf(
		"%s still holds the unresolved skeleton placeholder %q — replace it with real content before publishable generation (pass placeholder_policy=strict to block on it)",
		placeholder.DisplayPath(v.Path), v.Token)
}

// scanPlaceholderDiagnostics runs the placeholder scan under the given policy
// and returns the resulting diagnostics. It returns nil when the policy is
// "off" or no unresolved tokens remain. blocking is reported alongside so
// callers can decide whether the diagnostics gate the operation.
func scanPlaceholderDiagnostics(input any, policy string) (diags []diagnostics.Diagnostic, blocking bool) {
	if policy == "off" {
		return nil, false
	}
	violations := placeholder.Scan(input)
	if len(violations) == 0 {
		return nil, false
	}
	blocking = policy == "strict"
	return placeholderDiagnostics(violations, blocking), blocking
}

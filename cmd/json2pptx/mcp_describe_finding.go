package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
)

// ---------------------------------------------------------------------------
// describe_finding — agent-facing dictionary for finding codes
//
// Given a single finding code emitted anywhere in the pipeline (fit findings,
// chart diagnostics, validation errors, render-time auto-fixes), returns the
// machine-readable record an agent needs to resolve the underlying problem
// without reading docs/FIT_FINDINGS.md or the SKILL.md tables.
//
// Sourced from diagnostics.Describe, which unifies the pattern registry with
// pipeline, output-validation, and visual-QA code metadata. Source-scan tests
// guard against new emitted codes silently drifting from the catalogue.
// ---------------------------------------------------------------------------

func mcpDescribeFindingTool() mcp.Tool {
	return mcp.NewTool("describe_finding",
		mcp.WithDescription(`Look up an agent-facing description for a single finding code. Returns {code, summary, severity, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}. Use after any tool returns a finding/error you do not recognize — resolves the meaning in one extra tool call without scanning docs/FIT_FINDINGS.md or SKILL.md.

Covers every code emitted across the pipeline: the fit/pattern codes from get_capabilities.vocabularies.fit_finding_codes, chart.* and string-literal codes (contrast_autofixed, findings_truncated), and every diagnostics taxonomy code (MISSING_PARAMETER, TEMPLATE_NOT_FOUND, RENDER_FAILED, INTERNAL, …). Accepts either the bare legacy code or the dotted namespaced code from a finding envelope (INPUT.MISSING_PARAMETER, FIT.placeholder_overflow) — the namespace prefix is stripped before lookup, so a finding's describe_command runs verbatim. Unknown codes return a structured error carrying fix.params.did_you_mean (the closest known code) — or fix.params.allowed with the full vocabulary when nothing is close.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaDescribeFinding)),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The finding code to describe (e.g., \"placeholder_overflow\", \"accent_overload\", \"chart.zero_sum_pie\")."),
		),
	)
}

func handleDescribeFinding(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return argRequired(request, "describe_finding", "code", "string", "placeholder_overflow", nil), nil
	}
	if code == "" {
		return argRequired(request, "describe_finding", "code", "string", "placeholder_overflow", nil), nil
	}

	meta, ok := diagnostics.Describe(code)
	if !ok {
		// Lead with a did_you_mean rather than the whole vocabulary: inlining
		// all 150+ codes cost ~3.8KB on every typo and buried the one thing the
		// agent needed (go-slide-creator-7zrt). The full list is still offered,
		// but only when there is no close match to point at.
		allowed := diagnostics.AllDescribableCodes()
		params := map[string]any{}
		msg := fmt.Sprintf("unknown finding code %q", code)
		if match, _ := generator.ClosestMatch(code, allowed, 4); match != "" {
			params["did_you_mean"] = match
			msg += fmt.Sprintf("; did you mean %q?", match)
		} else {
			params["allowed"] = allowed
			msg += "; see fix.params.allowed for the known vocabulary"
		}
		return mcpParseErrorWithFix(
			"UNKNOWN_FINDING_CODE",
			"code",
			msg,
			&diagnostics.Fix{Kind: "use_one_of", Params: params},
		), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, meta)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal describe_finding response: %v", err)), nil
	}
	return mcpResult, nil
}

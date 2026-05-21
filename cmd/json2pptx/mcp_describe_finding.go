package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// ---------------------------------------------------------------------------
// describe_finding — agent-facing dictionary for finding codes
//
// Given a single finding code emitted anywhere in the pipeline (fit findings,
// chart diagnostics, validation errors, render-time auto-fixes), returns the
// machine-readable record an agent needs to resolve the underlying problem
// without reading docs/FIT_FINDINGS.md or the SKILL.md tables.
//
// Sourced from patterns.FindingMeta — a single registry whose entries are
// asserted to cover every emitted code by TestFindingMetaCoversAllCodes,
// so the data cannot silently drift from the engine.
// ---------------------------------------------------------------------------

func mcpDescribeFindingTool() mcp.Tool {
	return mcp.NewTool("describe_finding",
		mcp.WithDescription(`Look up an agent-facing description for a single finding code. Returns {code, summary, severity, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}. Use after any tool returns a finding/error you do not recognize — resolves the meaning in one extra tool call without scanning docs/FIT_FINDINGS.md or SKILL.md.

Covers every code emitted across the pipeline: the fit/pattern codes from get_capabilities.vocabularies.fit_finding_codes, chart.* and string-literal codes (contrast_autofixed, findings_truncated), and every diagnostics taxonomy code (MISSING_PARAMETER, TEMPLATE_NOT_FOUND, RENDER_FAILED, INTERNAL, …). Accepts either the bare legacy code or the dotted namespaced code from a finding envelope (INPUT.MISSING_PARAMETER, FIT.placeholder_overflow) — the namespace prefix is stripped before lookup, so a finding's describe_command runs verbatim. Unknown codes return a structured error whose fix.params.allowed enumerates the known vocabulary.`),
		mcp.WithRawOutputSchema(outputSchemaDescribeFinding),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The finding code to describe (e.g., \"placeholder_overflow\", \"accent_overload\", \"chart.zero_sum_pie\")."),
		),
	)
}

func handleDescribeFinding(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return argMissing("describe_finding", "code", "string", "placeholder_overflow", nil), nil
	}
	if code == "" {
		return argMissing("describe_finding", "code", "string", "placeholder_overflow", nil), nil
	}

	meta, ok := diagnostics.Describe(code)
	if !ok {
		allowed := diagnostics.AllDescribableCodes()
		fix := &diagnostics.Fix{
			Kind:   "use_one_of",
			Params: map[string]any{"allowed": allowed},
		}
		return mcpParseErrorWithFix(
			"UNKNOWN_FINDING_CODE",
			"code",
			fmt.Sprintf("unknown finding code %q; see fix.params.allowed for the known vocabulary", code),
			fix,
		), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, meta)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal describe_finding response: %v", err)), nil
	}
	return mcpResult, nil
}

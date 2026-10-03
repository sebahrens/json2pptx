package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
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
		mcp.WithDescription(`Explain one finding code: {code, summary, severity, blocks (always | sometimes | never), blocks_when, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}. Call it for a finding or error you do not recognize.

Covers every code a tool emits: fit and pattern codes, chart.*, SEMANTIC_*, and the input / template / render errors (MISSING_PARAMETER, TEMPLATE_NOT_FOUND, RENDER_FAILED, …). A namespaced code from a finding envelope (FIT.placeholder_overflow) is accepted. An unknown code returns the closest known one (fix.params.did_you_mean), or the vocabulary (fix.params.allowed) when nothing is close. hidden_tools names tools the remediation mentions that this server's tools/list does not carry.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaDescribeFinding)),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The finding code, e.g. \"BODY_TOO_LONG\", \"chart.zero_sum_pie\"."),
		),
	)
}

// describeFindingResponse is a finding's catalogue entry plus its blocking
// profile on the DeckSpec tools.
type describeFindingResponse struct {
	*patterns.FindingMeta
	// Blocks is "always", "sometimes" or "never": whether validate_deck_spec /
	// render_deck_spec report this code with severity error and blocking:true.
	Blocks string `json:"blocks"`
	// BlocksWhen states the condition.
	BlocksWhen string `json:"blocks_when"`
	// HiddenTools names the tools the entry mentions that the active profile's
	// tools/list does not carry (go-slide-creator-7bdn6).
	HiddenTools *hiddenToolsNote `json:"hidden_tools,omitempty"`
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

	// One severity model across the DeckSpec tools: say whether this code
	// blocks, in the words the gate uses (go-slide-creator-x9rhq). "severity"
	// stays the action rank raw fit_findings carry.
	blocks, when := deterministic.BlockingProfile(meta.Code, meta.Severity)
	resp := describeFindingResponse{FindingMeta: meta, Blocks: blocks, BlocksWhen: when}
	if body, err := json.Marshal(resp); err == nil {
		resp.HiddenTools = hiddenToolsIn(string(body))
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal describe_finding response: %v", err)), nil
	}
	return mcpResult, nil
}

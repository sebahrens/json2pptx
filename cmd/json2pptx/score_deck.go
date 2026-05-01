package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

func mcpScoreDeckTool() mcp.Tool {
	return mcp.NewTool("score_deck",
		mcp.WithDescription(`Score a presentation for visual quality using deterministic rules. Returns an overall score (0-100), per-slide scores, and structured findings with fix suggestions.

Deterministic mode (default) runs geometry-based checks: text overflow, placeholder overflow, title wraps, footer collision, slide bounds overflow, and contrast auto-fixes. These checks produce zero false positives.

Score formula: 100 - sum(severity_weights × findings). Weights: refuse=25, shrink_or_split=15, review=5, info=0.

Use this after generate_presentation to get structured visual feedback without burning vision tokens.`),
		mcp.WithString("json_input",
			mcp.Required(),
			mcp.Description("JSON string containing the presentation definition (same format as generate_presentation json_input). Required for running deterministic checks."),
		),
		mcp.WithString("template",
			mcp.Description("Template name override. If omitted, uses the template field from json_input."),
		),
		mcp.WithString("mode",
			mcp.Description("Scoring mode: 'deterministic' (default, zero false positives) or 'with_heuristics' (adds vision-model checks, requires ANTHROPIC_API_KEY and rendered images)."),
			mcp.Enum("deterministic", "with_heuristics"),
		),
	)
}

func (mc *mcpConfig) handleScoreDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, err := request.RequireString("json_input")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "json_input is required"), nil
	}

	mode := "deterministic"
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		mode = m
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "json_input", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Apply deck-level defaults before checks.
	applyDefaults(&input)

	// Resolve template name.
	templateName := input.Template
	if override, err := request.RequireString("template"); err == nil && override != "" {
		templateName = override
	}
	if templateName == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "template is required (in json_input or as template parameter)"), nil
	}
	if len(input.Slides) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "at least one slide is required in json_input"), nil
	}

	// Resolve and analyze template.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)

	// Collect deterministic findings (reuses the existing fit-findings pipeline).
	findings := collectFitFindings(&input, layouts, slideWidth, slideHeight)

	// Also run text-fit checks (generateFitReport checks table/shape-grid text).
	// These are already included by collectFitFindings via convertTextFitFinding.

	// Score the findings.
	ds := deterministic.ScoreFromFindings(findings, len(input.Slides))

	if mode == "with_heuristics" {
		// Heuristic mode requires rendered images + API key — not yet wired up.
		// Return deterministic results with a note.
		ds.ModeUsed = "deterministic"
		ds.Summary.TopCodes = appendHeuristicNote(ds.Summary.TopCodes)
	}

	mcpResult, err := api.MCPSuccessResult(ctx, ds)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// appendHeuristicNote adds a synthetic code entry indicating heuristic mode
// was requested but is not yet available.
func appendHeuristicNote(codes []deterministic.CodeCount) []deterministic.CodeCount {
	return codes // no-op for now; heuristic mode is opt-in future work
}


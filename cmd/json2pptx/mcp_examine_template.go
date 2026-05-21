// mcp_examine_template.go implements the examine_template MCP tool — the
// inline counterpart to the `json2pptx examine-template` CLI subcommand. Where
// the CLI materialises a directory of artifacts (report.json, per-layout XML /
// SVG / PNG, conformance.json, …), the MCP tool returns the full
// examine.Report as StructuredContent: the same report.json shape (nested
// FindingEnvelope under "findings", slide dims, theme, canonical_coverage,
// derivable_layouts, and layouts[] with font-aware max_chars, bounds in inches,
// z-order, and derived content zones), with no on-disk side effects.
//
// MCP mode is deliberately side-effect-free: it never writes a directory and
// accepts no out param for asset materialisation. Agents that need the rendered
// artifacts shell out to the CLI; agents that only need the capability facts
// read them straight off this response.
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/template"
)

func mcpExamineTemplateTool() mcp.Tool {
	return mcp.NewTool("examine_template",
		mcp.WithDescription(`Examine a PPTX template and return a full capability report inline. Mirrors the `+"`json2pptx examine-template`"+` CLI but writes no files: the response is the report.json shape returned as structured content.

Report fields: template, sha256, aspect_ratio, slide (dimensions in EMU + inches), theme (name, title_font, body_font, colors scheme→hex map), masters[], canonical_coverage (the four content-bearing families title-slide / section-divider / one-content / qa-closing, each {family, present, layouts[]}), derivable_layouts[] ({name, ready, missing[]}), layouts[] (per-layout canonical type/family + confidence, asset_base, xml_path, derived content_zone, and placeholders[] with role, font-aware font_pt + max_chars, exact bounds in EMU + inches, and z_index), and a findings envelope folding every diagnostic — including TPL.LAYOUT.MISSING_ROLE when a canonical family is absent.

Use this before authoring to learn exactly what layouts, placeholder roles, character budgets, and theme colors a user-provided template supports. For the rendered SVG/PNG artifact tree, use the CLI subcommand instead.`),
		mcp.WithRawOutputSchema(outputSchemaExamineTemplate),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue). Use list_templates to discover available names."),
		),
		mcp.WithBoolean("strict",
			mcp.Description("When true, metadata validation fails on warnings, not just errors (surfaced in the findings envelope). Default: false."),
			mcp.DefaultBool(false),
		),
	)
}

func (mc *mcpConfig) handleExamineTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateName, err := request.RequireString("template_name")
	if err != nil {
		return argMissing("examine_template", "template_name", "string", "midnight-blue", nextCallListTemplates()), nil
	}

	strict := false
	if v, err := request.RequireBool("strict"); err == nil {
		strict = v
	}

	// Resolve template path using the shared resolution path (flag → env →
	// user home → ./templates → embedded), so examination succeeds wherever
	// generation does.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("failed to open template: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	report, err := examine.Examine(reader, examine.Options{TemplatePath: templatePath, Strict: strict})
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("examine-template: %v", err)), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, report)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

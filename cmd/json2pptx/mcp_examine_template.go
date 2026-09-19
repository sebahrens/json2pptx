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
//
// The tool accepts a template in one of two mutually-exclusive forms:
//
//   - template_name — a registered/embedded template resolved through the shared
//     lookup path (flag → env → user home → ./templates → embedded). This is the
//     default and unchanged historical behaviour.
//   - template_path — a guarded local .pptx path. It lets an MCP-only agent
//     inspect a not-yet-registered template file. The path is validated against
//     the allowed root (base_dir, falling back to the server CWD): it must,
//     after ~/$ENV expansion and symlink resolution, be a regular .pptx file
//     contained within base_dir. Anything escaping that root (a ".." traversal
//     or a symlink/absolute path pointing outside) is rejected with a clear
//     INVALID_PATH forbidden-path diagnostic instead of being inspected.
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/template"
)

func mcpExamineTemplateTool() mcp.Tool {
	return mcp.NewTool("examine_template",
		mcp.WithDescription(`Examine a PPTX template and return a full capability report inline. Mirrors the `+"`json2pptx examine-template`"+` CLI but writes no files: the response is the report.json shape returned as structured content.

Report fields: template, sha256, aspect_ratio, slide (dimensions in EMU + inches), theme (name, title_font, body_font, colors scheme→hex map), masters[], canonical_coverage (the four content-bearing families title-slide / section-divider / one-content / qa-closing, each {family, present, layouts[]}), derivable_layouts[] ({name, ready, missing[]}), layouts[] (per-layout canonical type/family + confidence, asset_base, xml_path, derived content_zone, and placeholders[] with role, font-aware font_pt + max_chars, exact bounds in EMU + inches, and z_index), and a findings envelope folding every diagnostic — including TPL.LAYOUT.MISSING_ROLE when a canonical family is absent. The findings envelope already folds the validate-template metadata diagnostics; the merged conformance.json bundle (the validate-template capabilities verdict plus the template-check conformance report) remains CLI-only.

Supply EXACTLY ONE of:
  - template_name — a registered/embedded template (use list_templates to discover names). Default form.
  - template_path — a local .pptx path for a not-yet-registered template. The path is resolved against base_dir (or the server CWD when base_dir is absent) and MUST stay inside that allowed root after ~/$ENV expansion and symlink evaluation; a path escaping it returns an INVALID_PATH forbidden-path diagnostic.

Use this before authoring to learn exactly what layouts, placeholder roles, character budgets, and theme colors a user-provided template supports. For the rendered SVG/PNG artifact tree, use the CLI subcommand instead.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaExamineTemplate)),
		mcp.WithString("template_name",
			mcp.Description("Registered/embedded template name (e.g., midnight-blue). Use list_templates to discover available names. Provide this OR template_path, not both."),
		),
		mcp.WithString("template_path",
			mcp.Description("Path to a local .pptx template file to inspect, resolved against base_dir (or the server CWD when base_dir is absent). Must stay within that allowed root. Provide this OR template_name, not both."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory that bounds template_path resolution (the allowed root). Relative template_path values resolve against it; the resolved file must stay inside it. When absent, the server falls back to its process CWD."),
		),
		mcp.WithBoolean("strict",
			mcp.Description("When true, metadata validation fails on warnings, not just errors (surfaced in the findings envelope). Default: false."),
			mcp.DefaultBool(false),
		),
	)
}

func (mc *mcpConfig) handleExamineTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	strict := false
	if v, err := request.RequireBool("strict"); err == nil {
		strict = v
	}

	args := request.GetArguments()
	templateName, _ := args["template_name"].(string)
	rawTemplatePath, _ := args["template_path"].(string)

	templatePath, cleanup, errResult := mc.resolveExamineTemplateSource(request, templateName, rawTemplatePath)
	if errResult != nil {
		return errResult, nil
	}
	defer cleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeTemplateError, fmt.Sprintf("failed to open template: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	report, err := examine.Examine(reader, examine.Options{TemplatePath: templatePath, Strict: strict})
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeTemplateError, fmt.Sprintf("examine-template: %v", err)), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, report)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeInternal, fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// resolveExamineTemplateSource resolves the template source from exactly one of
// template_name (registered/embedded lookup) or template_path (guarded local
// path). It returns the resolved path plus a cleanup function (a no-op except
// for embedded templates extracted to a temp file), or a structured error
// result the caller must return unchanged. The cleanup function is always
// non-nil and safe to defer even on the error path.
func (mc *mcpConfig) resolveExamineTemplateSource(request mcp.CallToolRequest, templateName, rawTemplatePath string) (string, func(), *mcp.CallToolResult) {
	path, cleanup, d := mc.resolveTemplateSource(request, "examine_template", "template_name", "template_path", templateName, rawTemplatePath)
	if d != nil {
		return "", func() {}, api.MCPDiagnosticsError([]diagnostics.Diagnostic{*d})
	}
	return path, cleanup, nil
}

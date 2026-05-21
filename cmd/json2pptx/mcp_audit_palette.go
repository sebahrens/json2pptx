// mcp_audit_palette.go exposes the deterministic palette-diff audit (the same
// core that backs the `json2pptx audit-palette` CLI subcommand) as the MCP
// audit_palette tool. Both surfaces call the shared auditPalettePPTX core in
// audit_palette.go, so the report an agent gets over MCP is identical to the
// CLI report for the same PPTX and options.
//
// Unlike the CLI, the MCP tool deliberately exposes no -output / -keep / -tmp
// parameters: render artifacts (PDF + per-slide PNGs) are written only to an
// auto-removed OS temp directory and cleaned up before the handler returns, so
// the tool never writes to an agent-controlled path. The pptx_path input is
// validated with api.ValidatePptxPath (rejecting traversal and non-.pptx paths)
// before any work begins.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func mcpAuditPaletteTool() mcp.Tool {
	return mcp.NewTool("audit_palette",
		mcp.WithDescription(`Render a PPTX to PNG and report the CIE76 ΔE between every embedded chart/picture region and every native solid-filled shape region on each slide. This is the deterministic palette-diff the vision QA agent cannot do: it catches silent drift where a native shape fill diverges from a chart embedded next to it, even when both came from the "same" template palette.

Requires libreoffice + pdftoppm on PATH; returns AUDIT_FAILED when either is missing. Render artifacts are written only to an auto-removed temp directory — there is no parameter to redirect output to disk.

Response shape: the full audit report (pptx, slide_count, violations, per-slide pic/shape regions and (pic, shape) pairs with delta_e + pass) promoted to the top level, plus a "findings" FindingEnvelope where each pair that exceeds max_delta_e becomes one RENDER.palette_drift error finding. Branch on findings.ok: it is false (and violations > 0) when any pair drifts past the threshold.`),
		mcp.WithRawOutputSchema(outputSchemaAuditPalette),
		mcp.WithString("pptx_path",
			mcp.Required(),
			mcp.Description("Path to the PPTX file to audit. Must be an absolute or relative .pptx path with no traversal segments."),
		),
		mcp.WithNumber("max_delta_e",
			mcp.Description("Maximum allowed CIE76 ΔE for a (pic, shape) pair before it is counted as a violation. Default: 5.0."),
			mcp.DefaultNumber(5.0),
		),
		mcp.WithNumber("chroma_min",
			mcp.Description("Minimum chroma (max-min channel, 0..255) for a pixel to count toward the dominant color; filters white/black/gray chrome. Default: 25."),
			mcp.DefaultNumber(25),
		),
		mcp.WithNumber("density",
			mcp.Description("DPI for pdftoppm rasterization. Higher density samples more pixels per region at the cost of render time. Default: 150."),
			mcp.DefaultNumber(150),
		),
	)
}

// auditPaletteOutput is the audit_palette response. It promotes the full
// auditReport to the top level and adds a FindingEnvelope projection of every
// ΔE violation under "findings", so an agent can branch on findings.ok without
// losing the per-pair detail. The envelope is always present; findings.findings
// is empty when no pair exceeds the threshold. See docs/AGENT_DIAGNOSTICS.md.
type auditPaletteOutput struct {
	*auditReport
	Findings diagnostics.FindingEnvelope `json:"findings"`
}

// handleAuditPalette validates pptx_path, runs the shared palette audit, and
// returns the report plus a FindingEnvelope projection of every ΔE violation.
func handleAuditPalette(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil || pptxPath == "" {
		return argMissing("audit_palette", "pptx_path", "string", "/tmp/out/deck.pptx", nil), nil
	}

	if err := api.ValidatePptxPath(pptxPath); err != nil {
		return argInvalidValue("audit_palette", "INVALID_PATH", "pptx_path", err.Error(), "string", "/tmp/out/deck.pptx", nil), nil
	}

	if _, statErr := os.Stat(pptxPath); os.IsNotExist(statErr) {
		return api.MCPSimpleError("FILE_NOT_FOUND", fmt.Sprintf("pptx file not found: %s", pptxPath)), nil
	}

	// Optional knobs — MCP delivers numbers as float64. Defaults mirror the CLI.
	maxDelta := 5.0
	if v, ok := request.GetArguments()["max_delta_e"].(float64); ok {
		maxDelta = v
	}
	chromaMin := 25
	if v, ok := request.GetArguments()["chroma_min"].(float64); ok {
		chromaMin = int(v)
	}
	if uint8Overflows(chromaMin) {
		return argInvalidValue("audit_palette", "INVALID_PARAMETER", "chroma_min",
			fmt.Sprintf("chroma_min must be in 0..255 (got %d)", chromaMin), "integer", 25, nil), nil
	}
	density := 150
	if v, ok := request.GetArguments()["density"].(float64); ok {
		density = int(v)
	}
	if density <= 0 {
		return argInvalidValue("audit_palette", "INVALID_PARAMETER", "density",
			fmt.Sprintf("density must be a positive DPI (got %d)", density), "integer", 150, nil), nil
	}

	// TmpDir="" + Keep=false make the core create its own OS temp directory and
	// remove it (and every render artifact) before returning — the MCP tool
	// never writes to an agent-controlled path.
	report, auditErr := auditPalettePPTX(pptxPath, auditOptions{
		MaxDeltaE: maxDelta,
		ChromaMin: uint8(chromaMin),
		Density:   density,
		TmpDir:    "",
		Keep:      false,
	})
	if auditErr != nil {
		// The audit's failure modes — missing libreoffice/pdftoppm, PPTX open,
		// rasterization, PNG decode — are all render-class failures.
		return api.MCPSimpleError(diagnostics.CodeRenderFailed, fmt.Sprintf("palette audit failed: %v", auditErr)), nil
	}

	// The render PNGs live under the temp dir the core just removed, so the
	// render_image paths now point at deleted files. Drop them rather than
	// hand the agent dangling paths.
	for i := range report.Slides {
		report.Slides[i].RenderImage = ""
	}

	output := auditPaletteOutput{
		auditReport: report,
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand: "audit_palette",
		}, diagnosticsFromAuditReport(report)),
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// diagnosticsFromAuditReport flattens every (pic, shape) pair whose ΔE exceeds
// the threshold into a transport-neutral RENDER.palette_drift diagnostic, in
// report order (by slide, then pair). Passing pairs are omitted so the envelope
// OK flag mirrors the CLI exit code: ok=true iff there are zero violations.
func diagnosticsFromAuditReport(report *auditReport) []diagnostics.Diagnostic {
	if report == nil {
		return nil
	}
	var ds []diagnostics.Diagnostic
	for _, s := range report.Slides {
		for _, p := range s.Pairs {
			if p.Pass {
				continue
			}
			d := diagnostics.Diagnostic{
				Code:     diagnostics.DottedCode(diagnostics.NamespaceRender, "palette_drift"),
				Severity: diagnostics.SeverityError,
				Path:     fmt.Sprintf("slides[%d]", p.Slide-1),
				Message: fmt.Sprintf("palette drift on slide %d: chart pic %q (#%s) vs shape %q (#%s) ΔE=%.3f exceeds threshold %.3f",
					p.Slide, p.Pic.Name, p.Pic.Hex, p.Shape.Name, p.Shape.Hex, p.DeltaE, report.MaxDeltaEAllowed),
				Details: map[string]any{
					"slide":      p.Slide,
					"delta_e":    p.DeltaE,
					"threshold":  report.MaxDeltaEAllowed,
					"pic_name":   p.Pic.Name,
					"pic_hex":    p.Pic.Hex,
					"shape_name": p.Shape.Name,
					"shape_hex":  p.Shape.Hex,
				},
			}
			if p.Shape.DeclaredHex != "" {
				d.Details["declared_hex"] = p.Shape.DeclaredHex
			}
			ds = append(ds, d)
		}
	}
	return ds
}

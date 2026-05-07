package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/pptxread"
)

func mcpReadPresentationTool() mcp.Tool {
	return mcp.NewTool("read_presentation",
		mcp.WithDescription(`Read a PPTX file and return its content as structured JSON. Best-effort extraction of placeholders, shapes, tables, and speaker notes — entirely deterministic, no LibreOffice dependency.

Use this tool to introspect what generate_presentation actually produced: verify text placement, check which placeholders were populated, detect silent trimming, and confirm idempotency.

Response shape: {slide_count, slides: [{index, layout_id, placeholders: [{id, type, text, bounds}], shapes: [{name, geometry, text, bounds}], tables: [{name, rows, cols, headers, data, bounds}], speaker_notes}]}`),
		mcp.WithString("pptx_path",
			mcp.Description("Path to the PPTX file to read."),
			mcp.Required(),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("Extract a single slide by 0-based index. Omit to read all slides."),
		),
		mcp.WithRawOutputSchema(outputSchemaReadPresentation),
	)
}

func handleReadPresentation(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "pptx_path is required"), nil
	}

	if err := api.ValidatePptxPath(pptxPath); err != nil {
		return api.MCPSimpleError("INVALID_PATH", err.Error()), nil
	}

	if _, err := os.Stat(pptxPath); os.IsNotExist(err) {
		return api.MCPSimpleError("FILE_NOT_FOUND", fmt.Sprintf("pptx file not found: %s", pptxPath)), nil
	}

	pres, err := pptxread.ReadFile(pptxPath)
	if err != nil {
		return api.MCPSimpleError("READ_FAILED", fmt.Sprintf("failed to read presentation: %v", err)), nil
	}

	// Filter to a single slide if requested.
	if v, ok := request.GetArguments()["slide_index"].(float64); ok {
		idx := int(v)
		if idx < 0 || idx >= len(pres.Slides) {
			return api.MCPSimpleError("INVALID_SLIDE_INDEX",
				fmt.Sprintf("slide_index %d out of range (presentation has %d slides)", idx, len(pres.Slides))), nil
		}
		pres.Slides = []pptxread.Slide{pres.Slides[idx]}
	}

	ctx := context.Background()
	mcpResult, err := api.MCPSuccessResult(ctx, pres)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// render_from_json.go implements the render_slide_image_from_json MCP tool —
// a single-slide design-feedback loop that skips the cost of rendering the
// entire deck. Agents iterating on one slide can send the slide JSON + a
// template name and receive a PNG without first calling generate_presentation
// on the whole deck.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/render"
)

// mcpRenderSlideImageFromJSONTool is the tool-definition constructor.
func mcpRenderSlideImageFromJSONTool() mcp.Tool {
	return mcp.NewTool("render_slide_image_from_json",
		mcp.WithDescription(`Render a single slide directly from its JSON definition + a template name, without first generating the full deck. Returns the same image envelope as render_slide_image (base64 PNG or path reference if >200KB).

Use this for tight single-slide design iteration loops: edit one slide's JSON, see the rendered PNG, repeat. Avoids the cost of rendering N-1 unchanged slides.

Requires LibreOffice and ImageMagick (magick) on PATH. Behind the scenes, this builds a one-slide PPTX in a temp directory, renders it, and discards the intermediate.

Results are cached by (slide JSON content + template content + density) — repeated calls with identical inputs return instantly. Pass force=true to re-render.`),
		mcp.WithRawOutputSchema(outputSchemaRenderSlideImage),
		mcp.WithObject("slide",
			mcp.Required(),
			mcp.Description("A single slide JSON object. Same schema as one entry in presentation.slides for generate_presentation. Use get_input_schema to discover the slide shape."),
		),
		mcp.WithString("template",
			mcp.Required(),
			mcp.Description("Template name (use list_templates to discover available names)."),
		),
		mcp.WithNumber("density",
			mcp.Description("DPI for rendering. Higher = sharper but larger. Default: 100. Range: 50-300."),
		),
		mcp.WithBoolean("force",
			mcp.Description("If true, bypass the render cache and re-convert even if a cached result exists. Default: false."),
		),
	)
}

// handleRenderSlideImageFromJSON is the MCP handler for render_slide_image_from_json.
func (mc *mcpConfig) handleRenderSlideImageFromJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Required: slide object.
	slideJSON, paramErr := objectParamAsJSON(request, "slide")
	if paramErr != nil {
		return paramErr, nil
	}
	if slideJSON == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "slide is required"), nil
	}

	// Required: template name.
	templateName, err := request.RequireString("template")
	if err != nil || templateName == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "template is required"), nil
	}

	// Optional: density (clamped 50-300, default 100).
	density := 100
	if v, ok := request.GetArguments()["density"].(float64); ok {
		d := int(v)
		if d < 50 {
			d = 50
		} else if d > 300 {
			d = 300
		}
		density = d
	}

	// Optional: force.
	force := false
	if v, ok := request.GetArguments()["force"].(bool); ok {
		force = v
	}

	// Resolve the template path so we can hash it for the cache key.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	tplHash, err := render.HashFile(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("hash template: %v", err)), nil
	}

	// Cache key: sha256(slide JSON || template content hash). Density is
	// folded in by RenderSlideWithCacheKey.
	h := sha256.New()
	h.Write([]byte(slideJSON))
	h.Write([]byte{0})
	h.Write([]byte(tplHash))
	jsonCacheKey := hex.EncodeToString(h.Sum(nil))

	// Fast path: if a cached PNG exists for this (json+template+density)
	// combination, return it without spinning up LibreOffice.
	if !force {
		if cached := render.LookupCachedSlide(jsonCacheKey, 0, density); cached != nil {
			mcpResult, marshalErr := api.MCPSuccessResult(ctx, cached)
			if marshalErr != nil {
				return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", marshalErr)), nil
			}
			return mcpResult, nil
		}
	}

	// Slow path: generate a single-slide PPTX into a temp dir, then render.
	tempOutDir, err := os.MkdirTemp("", "render-from-json-out-*")
	if err != nil {
		return api.MCPSimpleError("OUTPUT_DIR", fmt.Sprintf("failed to create temp dir: %v", err)), nil
	}
	defer os.RemoveAll(tempOutDir)

	// Build a single-slide presentation by wrapping the caller's slide.
	var slideObj any
	if err := json.Unmarshal([]byte(slideJSON), &slideObj); err != nil {
		return mcpParseError("INVALID_JSON", "slide", fmt.Sprintf("invalid JSON: %v", err)), nil
	}
	presentation := map[string]any{
		"template":        templateName,
		"output_filename": "single-slide.pptx",
		"slides":          []any{slideObj},
	}

	// Delegate to handleGenerate via a temp-mc whose outputDir is the temp dir.
	tempMC := &mcpConfig{
		templatesDir: mc.templatesDir,
		outputDir:    tempOutDir,
		cfg:          mc.cfg,
		cache:        mc.cache,
	}
	genReq := mcpRequestWithArgs(map[string]any{
		"presentation": presentation,
	})
	genResult, err := tempMC.handleGenerate(ctx, genReq)
	if err != nil {
		return api.MCPSimpleError("GENERATION_FAILED", fmt.Sprintf("generation failed: %v", err)), nil
	}
	if genResult == nil {
		return api.MCPSimpleError("GENERATION_FAILED", "generation returned no result"), nil
	}
	if genResult.IsError {
		// Forward the underlying error to the caller — its diagnostics are
		// more actionable than a wrapped INTERNAL.
		return genResult, nil
	}

	// Extract the generated PPTX path from the JSONOutput envelope.
	var genOut JSONOutput
	rawText := extractMCPTextContent(genResult)
	if rawText == "" {
		return api.MCPSimpleError("GENERATION_FAILED", "generation result missing text content"), nil
	}
	if err := json.Unmarshal([]byte(rawText), &genOut); err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to parse generation result: %v", err)), nil
	}
	if genOut.OutputPath == "" {
		return api.MCPSimpleError("GENERATION_FAILED", "generation result missing output_path"), nil
	}

	// Render the single slide (index 0), caching under the JSON-derived key.
	img, err := render.RenderSlideWithCacheKey(genOut.OutputPath, 0, density, force, jsonCacheKey)
	if err != nil {
		code := "RENDER_FAILED"
		if strings.Contains(err.Error(), "not found on PATH") {
			if strings.Contains(err.Error(), "libreoffice") {
				code = "LIBREOFFICE_UNAVAILABLE"
			} else {
				code = "IMAGEMAGICK_UNAVAILABLE"
			}
		}
		return api.MCPSimpleError(code, err.Error()), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, img)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// extractMCPTextContent pulls the first TextContent block from an MCP
// CallToolResult. Returns "" if none is present.
func extractMCPTextContent(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

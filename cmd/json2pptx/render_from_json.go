// render_from_json.go implements the render_slide_image_from_json MCP tool —
// a single-slide design-feedback loop that skips the cost of rendering the
// entire deck. Agents iterating on one slide can send the slide JSON + a
// template name and receive a PNG without first calling generate_presentation
// on the whole deck.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/svggen"
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
		mcp.WithBoolean("overlay",
			mcp.Description(`If true, composite a diagnostic overlay on top of the rendered PNG: shape_grid cell rectangles, density-band tints (info=blue, review=amber, shrink_or_split=orange, refuse=red), and fit-finding badges. Lets agents "see" the diagnostic without cross-referencing finding coordinates against the image manually. Default: false.`),
		),
	)
}

// handleRenderSlideImageFromJSON is the MCP handler for render_slide_image_from_json.
//
//nolint:gocognit,gocyclo // straight-line param parsing + cache short-circuit +
// render + optional overlay; each branch returns early. Splitting further would
// obscure the dependency order between template hashing, cache lookup, and generation.
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

	// Optional: overlay. When true, post-process the rendered PNG to
	// composite shape_grid cell bounds + fit-finding badges on top of the
	// LibreOffice raster so agents can see the diagnostic visually.
	overlay := false
	if v, ok := request.GetArguments()["overlay"].(bool); ok {
		overlay = v
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
	// combination, return it without spinning up LibreOffice. When overlay
	// is requested we still need plan resolution + compositing, so fall
	// through to the per-call overlay step below using the cached bytes.
	if !force {
		if cached := render.LookupCachedSlide(jsonCacheKey, 0, density); cached != nil {
			if overlay {
				if applied, applyErr := applyRenderOverlay(cached, slideJSON, templateName, templatePath, mc.templatesDir, jsonCacheKey); applyErr == nil {
					cached = applied
				} else {
					return api.MCPSimpleError("OVERLAY_FAILED", applyErr.Error()), nil
				}
			}
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

	if overlay {
		if applied, applyErr := applyRenderOverlay(img, slideJSON, templateName, templatePath, mc.templatesDir, jsonCacheKey); applyErr == nil {
			img = applied
		} else {
			return api.MCPSimpleError("OVERLAY_FAILED", applyErr.Error()), nil
		}
	}

	mcpResult, err := api.MCPSuccessResult(ctx, img)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// applyRenderOverlay composites a wireframe overlay (cell bounds, fit
// findings, density tints) on top of the raster PNG carried by img. The
// returned SlideImage shares Index/Width/Height with the input but
// carries the composited PNG (inline base64 when small enough, otherwise
// written to a stable path keyed off the cache key). On failure the
// input is left untouched.
func applyRenderOverlay(img *render.SlideImage, slideJSON, templateName, templatePath, templatesDir, cacheKey string) (*render.SlideImage, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}

	basePNG, err := readSlideImageBytes(img)
	if err != nil {
		return nil, fmt.Errorf("read base image: %w", err)
	}
	baseImg, err := png.Decode(bytes.NewReader(basePNG))
	if err != nil {
		return nil, fmt.Errorf("decode base PNG: %w", err)
	}

	// Resolve the slide plan so we know cell geometry + findings.
	wfReq, err := buildOverlayWireframeRequest(slideJSON, templateName, templatePath, templatesDir, baseImg.Bounds().Dx())
	if err != nil {
		return nil, err
	}
	if wfReq == nil {
		// No cells/findings — nothing to draw. Return original unchanged.
		return img, nil
	}

	out, err := svggen.RenderWireframe(wfReq, svggen.RenderWireframeOptions{
		IncludePNG:  true,
		PNGScale:    1.0,
		OverlayOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("render overlay: %w", err)
	}
	overlayImg, err := png.Decode(bytes.NewReader(out.PNG))
	if err != nil {
		return nil, fmt.Errorf("decode overlay PNG: %w", err)
	}

	composed := compositeImages(baseImg, overlayImg)
	var buf bytes.Buffer
	if err := png.Encode(&buf, composed); err != nil {
		return nil, fmt.Errorf("encode composite PNG: %w", err)
	}

	// Mirror the size-based fan-out from render.SlideImage: large
	// composites get a stable on-disk path, small ones inline as base64.
	composedBytes := buf.Bytes()
	result := &render.SlideImage{
		Index:  img.Index,
		Width:  baseImg.Bounds().Dx(),
		Height: baseImg.Bounds().Dy(),
	}
	const maxInlineBytes = 200 * 1024
	if len(composedBytes) > maxInlineBytes {
		safeKey := cacheKey
		if len(safeKey) > 16 {
			safeKey = safeKey[:16]
		}
		stablePath := filepath.Join(os.TempDir(), fmt.Sprintf("json2pptx-slide-overlay-%s.png", safeKey))
		if writeErr := os.WriteFile(stablePath, composedBytes, 0644); writeErr != nil {
			return nil, fmt.Errorf("write composite file: %w", writeErr)
		}
		result.Path = stablePath
	} else {
		result.PNG64 = base64.StdEncoding.EncodeToString(composedBytes)
	}
	return result, nil
}

// readSlideImageBytes returns the raw PNG bytes for a SlideImage carrying
// either inline base64 or a file path.
func readSlideImageBytes(img *render.SlideImage) ([]byte, error) {
	if img.PNG64 != "" {
		return base64.StdEncoding.DecodeString(img.PNG64)
	}
	if img.Path != "" {
		return os.ReadFile(img.Path)
	}
	return nil, fmt.Errorf("SlideImage has neither PNG64 nor Path")
}

// compositeImages draws overlay on top of base. When the overlay
// dimensions match base, stdlib draw.Over does the work. When they
// differ (rounding between EMU→points→pixels can produce a 1-px
// mismatch), we resize the overlay to base dimensions via
// nearest-neighbour and then composite.
func compositeImages(base, overlay image.Image) *image.RGBA {
	bounds := base.Bounds()
	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, base, bounds.Min, draw.Src)

	ob := overlay.Bounds()
	if ob.Dx() == bounds.Dx() && ob.Dy() == bounds.Dy() {
		draw.Draw(out, bounds, overlay, ob.Min, draw.Over)
		return out
	}
	resized := resizeNearest(overlay, bounds.Dx(), bounds.Dy())
	draw.Draw(out, bounds, resized, resized.Bounds().Min, draw.Over)
	return out
}

// resizeNearest produces an RGBA image of (w,h) by nearest-neighbour
// sampling from src.
func resizeNearest(src image.Image, w, h int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	if sb.Dx() == 0 || sb.Dy() == 0 {
		return out
	}
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sb.Dy()/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sb.Dx()/w
			out.Set(x, y, src.At(sx, sy))
		}
	}
	return out
}

// buildOverlayWireframeRequest resolves the same plan that
// preview_slide_wireframe would, but for the single-slide presentation
// implied by render_slide_image_from_json. Returns nil if the slide has
// no cells and no findings (nothing to overlay).
func buildOverlayWireframeRequest(slideJSON, templateName, templatePath, templatesDir string, baseWidthPx int) (*svggen.WireframeRequest, error) {
	var slideObj any
	if err := json.Unmarshal([]byte(slideJSON), &slideObj); err != nil {
		return nil, fmt.Errorf("parse slide JSON: %w", err)
	}
	presentation := map[string]any{
		"template": templateName,
		"slides":   []any{slideObj},
	}
	presJSON, err := json.Marshal(presentation)
	if err != nil {
		return nil, fmt.Errorf("encode presentation envelope: %w", err)
	}

	var input PresentationInput
	if err := strictUnmarshalJSON(presJSON, &input); err != nil {
		return nil, fmt.Errorf("unmarshal presentation: %w", err)
	}
	applyDefaults(&input)

	tctx, err := loadPreviewTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}
	defer func() { _ = tctx.reader.Close() }()
	_ = templatesDir // reserved for future cross-template overrides

	resolveCanonicalLayoutIDs(input.Slides, tctx.layouts)
	plan := resolvePreviewSlides(&input, tctx)
	findings := computePreviewFitFindings(&input, &plan, tctx, true /*verbose*/)

	if len(plan.ResolvedSlides) == 0 {
		return nil, fmt.Errorf("plan produced no resolved slides")
	}
	rs := plan.ResolvedSlides[0]

	// Pick canvas width matching the base raster so the overlay aligns 1:1
	// when composited. svggen's overlay path emits a canvas equal to the
	// slide aspect ratio at this width.
	wf := &svggen.WireframeRequest{
		SlideIndex:    0,
		LayoutID:      rs.LayoutID,
		LayoutName:    rs.LayoutName,
		SlideType:     rs.SlideType,
		TemplateName:  input.Template,
		SlideWidth:    float64(tctx.slideWidth),
		SlideHeight:   float64(tctx.slideHeight),
		OutputWidthPx: float64(baseWidthPx),
	}
	if rs.Occupancy != nil {
		wf.Occupancy = &svggen.WireframeOccupancy{
			FilledPct:   rs.Occupancy.FilledPct,
			FilledSlots: rs.Occupancy.FilledSlots,
			TotalSlots:  rs.Occupancy.TotalSlots,
		}
	}
	if rs.ShapeGridResolution != nil {
		for _, c := range rs.ShapeGridResolution.Cells {
			wf.Cells = append(wf.Cells, svggen.WireframeCell{
				Row:  c.Row,
				Col:  c.Col,
				Kind: c.Kind,
				Rect: svggen.WireframeRect{X: float64(c.X), Y: float64(c.Y), W: float64(c.W), H: float64(c.H)},
			})
		}
	}
	for _, f := range findings {
		if slidepath.SlideIndex(f.Path) != 0 {
			continue
		}
		wfd := svggen.WireframeFinding{Code: f.Code, Action: f.Action, Message: f.Message}
		if _, r, c, ok := slidepath.ParseGridCell(f.Path); ok {
			wfd.HasCell = true
			wfd.Row = r
			wfd.Col = c
		}
		wf.Findings = append(wf.Findings, wfd)
	}

	if len(wf.Cells) == 0 && len(wf.Findings) == 0 {
		return nil, nil
	}
	return wf, nil
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

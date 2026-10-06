package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// Recipe previews (go-slide-creator-r1uy7, go-slide-creator-ueopl).
//
// A preview is a recipe rendered: the one-slide DeckSpec an agent would send
// to render_deck_spec (a kind's example, or a recommend_visual candidate's
// next_tool_call) goes through the compiler and runner render_deck_spec uses,
// and the slide is converted to an image through the render cache. There is
// no second drawing path, so a preview cannot show a slide generation would
// not produce — the defect of the old preview-patterns gallery, which resolved
// a pattern's grid on its own and drew solid tiles, no title and no chrome.
//
// list_slide_kinds(preview:true), recommend_visual(preview:true) and the
// preview-patterns CLI all call renderSpecPreview.

const (
	// previewDensity is the DPI of a preview image: 960px wide on a 16:9
	// slide, enough to read the layout and small enough to travel inline.
	previewDensity = 72
	// maxPreviewImages bounds the images one discovery call returns; each one
	// not yet cached costs a LibreOffice conversion.
	maxPreviewImages = 4
	// maxStylePreviewImages is the ceiling of one call: the cycle kind asked
	// for alone previews each of its styles, one more than there are today.
	maxStylePreviewImages = 7
)

// previewSpecTemplate returns the template a preview spec pins.
func previewSpecTemplate(spec map[string]any) string {
	meta, _ := spec["meta"].(map[string]any)
	name, _ := meta["template"].(string)
	return name
}

// buildSpecPreviewDeck renders a DeckSpec to a .pptx in outDir through the
// runner render_deck_spec uses, without storing a deck handle or recording a
// gate. It returns the deck path.
func (mc *mcpConfig) buildSpecPreviewDeck(ctx context.Context, data []byte, outDir string) (string, error) {
	spec, parseDiags := semantic.Parse("preview.json", data)
	if parseDiags.HasErrors() {
		return "", fmt.Errorf("preview spec does not parse: %s", parseDiags[0].Message)
	}
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		return "", fmt.Errorf("preview spec does not compile: %w", err)
	}
	if violations := compiledDesignModeDiagnostics(input); len(violations) > 0 {
		return "", blockingDesignModeError(violations)
	}
	run := mc.runCompiledDeckSpec(ctx, "render_deck_spec", mcpRequestWithArgs(map[string]any{}), specSource{}, spec, input, compileResult, specRunOptions{
		OutputDir:        outDir,
		OutputFilename:   "preview.pptx",
		OutputValidation: "strict",
		Start:            time.Now(),
	})
	if len(run.EarlyDiagnostics) > 0 {
		return "", errors.New(run.EarlyDiagnostics[0].Message)
	}
	if !run.Result.OK {
		return "", errors.New(firstNonEmpty(run.Result.Error, "the recipe did not render"))
	}
	return run.Result.OutputPath, nil
}

// renderSpecPreview returns the image of the first slide of a DeckSpec, as
// render_deck_spec followed by a render of that slide would produce it.
//
// The deck is always built (generation is fast and has no external
// dependency); the image is cached under the slide's visible identity, the
// digest of everything it renders with. So a repeated preview costs no
// LibreOffice conversion, a preview and the deck an agent then renders from
// the same recipe share one image, and an engine or template change that
// alters the slide cannot return a stale picture.
func (mc *mcpConfig) renderSpecPreview(ctx context.Context, spec map[string]any, density int) (*render.SlideImage, error) {
	if err := render.CheckDependencies(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	outDir, err := os.MkdirTemp("", "json2pptx-preview-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(outDir) }()
	pptxPath, err := mc.buildSpecPreviewDeck(ctx, data, outDir)
	if err != nil {
		return nil, err
	}
	keys, err := render.VisibleSlideKeys(pptxPath)
	if err != nil {
		return nil, fmt.Errorf("read preview deck: %w", err)
	}
	if len(keys) == 0 {
		return nil, errors.New("preview deck has no slide to render")
	}
	key := "preview-" + keys[0]
	if cached := render.LookupCachedSlide(key, 0, density); cached != nil {
		return cached, nil
	}
	return render.RenderSlideWithCacheKeyContext(ctx, pptxPath, 0, density, false, key)
}

// previewRequest names one spec to preview.
type previewRequest struct {
	Name string
	Spec map[string]any
}

// renderPreviews renders up to maxStylePreviewImages recipes (callers hold
// kinds and candidates to maxPreviewImages) and returns one
// record per request plus the images, in content order after the JSON text.
// A failed preview is reported on its record and never fails the call.
func (mc *mcpConfig) renderPreviews(ctx context.Context, reqs []previewRequest) ([]patterns.VisualPreview, []api.MCPImage) {
	if len(reqs) > maxStylePreviewImages {
		reqs = reqs[:maxStylePreviewImages]
	}
	out := make([]patterns.VisualPreview, 0, len(reqs))
	var images []api.MCPImage
	for _, r := range reqs {
		p := patterns.VisualPreview{Name: r.Name, Template: previewSpecTemplate(r.Spec)}
		img, err := mc.renderSpecPreview(ctx, r.Spec, previewDensity)
		if err == nil {
			var enc api.MCPImage
			var meta renderedSlideMeta
			if meta, enc, err = slideImageToMCP(*img, len(images)+1); err == nil {
				p.ImageContentIndex = meta.ImageContentIndex
				p.ContentHash = meta.ContentHash
				images = append(images, enc)
			}
		}
		if err != nil {
			p.Error = err.Error()
		}
		out = append(out, p)
	}
	return out, images
}

// previewArg reads the boolean preview argument of a discovery tool.
func previewArg(tool string, request mcp.CallToolRequest) (bool, *mcp.CallToolResult) {
	raw, present := request.GetArguments()["preview"]
	if !present || raw == nil {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, argInvalidValue(tool, "INVALID_PARAMETER", "preview", "preview must be true or false", "boolean", true, nil)
	}
	return b, nil
}

// candidatePreviewRequests are the runnable recipes of the leading candidates.
func candidatePreviewRequests(rec *patterns.RecommendVisualResult) []previewRequest {
	var reqs []previewRequest
	for i := range rec.Candidates {
		c := &rec.Candidates[i]
		if c.NextToolCall == nil || c.NextToolCall.Tool != "render_deck_spec" {
			continue
		}
		spec, ok := c.NextToolCall.ArgsTemplate["spec"].(map[string]any)
		if !ok {
			continue
		}
		reqs = append(reqs, previewRequest{Name: c.Name, Spec: spec})
		if len(reqs) == maxPreviewImages {
			break
		}
	}
	return reqs
}

// previewCallFor is the call that returns pictures of the leading candidates
// of a recommend_visual response that was asked for none.
func previewCallFor(intent, templateName string, rec *patterns.RecommendVisualResult) *patterns.ToolCallSuggestion {
	reqs := candidatePreviewRequests(rec)
	if len(reqs) == 0 {
		return nil
	}
	names := make([]any, len(reqs))
	for i, r := range reqs {
		names[i] = r.Name
	}
	args := map[string]any{"intent": intent, "candidates": names, "preview": true}
	if templateName != "" {
		args["template"] = templateName
	}
	return &patterns.ToolCallSuggestion{Tool: "recommend_visual", ArgsTemplate: args}
}

// kindPreviewSpec wraps a kind's copy-ready example in a one-slide DeckSpec.
func kindPreviewSpec(k semantic.SlideKind, templateName string) map[string]any {
	return recipeSpec(strings.ReplaceAll(string(k), "_", " "), templateName, semantic.KindExample(k))
}

// patternPreviewSpec is the one-slide DeckSpec that shows a pattern with its
// exemplar values under a title: what preview-patterns renders.
func patternPreviewSpec(reg *patterns.Registry, name, templateName string) (map[string]any, error) {
	slide, _, err := rawPatternSlide(reg, name)
	if err != nil {
		return nil, err
	}
	return recipeSpec(strings.ReplaceAll(name, "-", " "), templateName, slide), nil
}

// previewableResult is a discovery tool's result: its JSON, followed by the
// preview images when it rendered any.
func previewableResult(ctx context.Context, v any, images []api.MCPImage) (*mcp.CallToolResult, error) {
	if len(images) == 0 {
		return api.MCPSuccessResult(ctx, v)
	}
	return api.MCPImageResult(ctx, v, images)
}

// kindPreviewRequests are the examples of the kinds a list_slide_kinds call
// selected, in catalogue order.
func kindPreviewRequests(kinds []string, templateName string) []previewRequest {
	if len(kinds) == 1 && kinds[0] == string(semantic.KindCycle) {
		// The cycle kind is six pictures behind one style field: asked for
		// alone, its preview shows each style (go-slide-creator-53v5u).
		reqs := make([]previewRequest, 0, len(slides.CycleStyles))
		for _, style := range slides.CycleStyles {
			reqs = append(reqs, previewRequest{
				Name: "cycle style " + style,
				Spec: recipeSpec("cycle "+strings.ReplaceAll(style, "_", " "), templateName, semantic.CycleStyleExample(style)),
			})
		}
		return reqs
	}
	reqs := make([]previewRequest, 0, len(kinds))
	for _, k := range kinds {
		reqs = append(reqs, previewRequest{Name: k, Spec: kindPreviewSpec(semantic.SlideKind(k), templateName)})
	}
	return reqs
}

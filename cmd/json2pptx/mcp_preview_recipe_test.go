package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// previewTestTemplates are the templates the preview tests run on: two tracked
// ones, plus the local p-style when it is present.
func previewTestTemplates() []string {
	names := []string{"midnight-blue", "warm-coral"}
	if _, err := os.Stat("../../templates/p-style.pptx"); err == nil {
		names = append(names, "p-style")
	}
	return names
}

// slideParts returns the parts of a deck that draw its slides: slide XML,
// slide relationships and media, by name.
func slideParts(t *testing.T, pptxPath string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open %s: %v", pptxPath, err)
	}
	defer func() { _ = zr.Close() }()
	out := map[string][]byte{}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/") && !strings.HasPrefix(f.Name, "ppt/media/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = data
	}
	return out
}

// renderDeckSpecPath renders a spec through the render_deck_spec tool and
// returns the written deck.
func renderDeckSpecPath(t *testing.T, mc *mcpConfig, spec map[string]any) string {
	t.Helper()
	res := renderDeckSpecCall(t, mc, map[string]any{"spec": spec})
	if !res.Success || res.PptxPath == "" {
		t.Fatalf("render_deck_spec did not write a deck: %+v", res)
	}
	return res.PptxPath
}

// TestPatternPreviewIsTheGeneratedSlide is the go-slide-creator-r1uy7 gate
// that needs no render toolchain: for every registered pattern, on every test
// template, the deck a preview is rendered from holds the same slide (XML,
// relationships, media) as the deck render_deck_spec writes for the pattern's
// exemplar values. A preview that drew the pattern any other way would differ
// here before it differed in pixels.
//
// Each pair is two generations, minutes for the whole matrix under -race. The
// short run takes every pattern on one template, the templates in turn, so
// every pattern and every template is still compared; the run without -short
// and the integration corpus job's TestShortReducedMatricesCorpus compare
// every pattern on every template (go-slide-creator-q7cpq).
func TestPatternPreviewIsTheGeneratedSlide(t *testing.T) {
	assertPatternPreviewIsTheGeneratedSlide(t, testing.Short())
}

// assertPatternPreviewIsTheGeneratedSlide compares the preview deck with the
// generated deck for every pattern: on every preview template, or, when
// oneTemplateEach, on one template per pattern with the templates in turn.
func assertPatternPreviewIsTheGeneratedSlide(t *testing.T, oneTemplateEach bool) {
	t.Helper()
	reg := patterns.Default()
	templates := previewTestTemplates()
	for ti, tpl := range templates {
		mc := semanticTestConfig(t)
		for pi, pat := range reg.List() {
			if oneTemplateEach && pi%len(templates) != ti {
				continue
			}
			name := pat.Name()
			t.Run(tpl+"/"+name, func(t *testing.T) {
				spec, err := patternPreviewSpec(reg, name, tpl)
				if err != nil {
					t.Fatalf("every registered pattern must have a preview recipe: %v", err)
				}
				data, err := json.Marshal(spec)
				if err != nil {
					t.Fatal(err)
				}
				previewDeck, err := mc.buildSpecPreviewDeck(context.Background(), data, t.TempDir())
				if err != nil {
					t.Fatalf("preview deck: %v", err)
				}
				got, want := slideParts(t, previewDeck), slideParts(t, renderDeckSpecPath(t, mc, spec))
				if _, ok := want["ppt/slides/slide1.xml"]; !ok || len(want) == 0 {
					t.Fatal("generated deck has no slide1.xml")
				}
				if !bytes.Contains(want["ppt/slides/slide1.xml"], []byte("Replace with the action title")) {
					t.Error("the previewed slide carries no title")
				}
				var names []string
				for n := range want {
					names = append(names, n)
				}
				sort.Strings(names)
				if len(got) != len(want) {
					t.Errorf("preview deck has %d slide parts, generated deck %d", len(got), len(want))
				}
				for _, n := range names {
					if !bytes.Equal(got[n], want[n]) {
						t.Errorf("%s differs between the preview deck and the generated deck", n)
					}
				}
			})
		}
	}
}

// decodePreviewPNG decodes a rendered slide image.
func decodePreviewPNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

// differingPixelShare is the share of pixels whose colour differs by more
// than a rendering-noise threshold on any channel. Images of different size
// differ entirely.
func differingPixelShare(a, b image.Image) float64 {
	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		return 1
	}
	const channelTolerance = 24 << 8
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	diff := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, _ := a.At(a.Bounds().Min.X+x, a.Bounds().Min.Y+y).RGBA()
			br, bg, bb, _ := b.At(b.Bounds().Min.X+x, b.Bounds().Min.Y+y).RGBA()
			if absDiff(ar, br) > channelTolerance || absDiff(ag, bg) > channelTolerance || absDiff(ab, bb) > channelTolerance {
				diff++
			}
		}
	}
	return float64(diff) / float64(w*h)
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// previewPixelTolerance is the share of pixels a preview may differ from an
// independent conversion of the generated deck: LibreOffice does not convert
// one slide identically every time (an italic face substituted differently
// moves a line of text), and nothing else is allowed to differ.
const previewPixelTolerance = 0.005

// previewPixelSample is the default pattern sample of the pixel test: the
// patterns the stale gallery misdrew (go-slide-creator-r1uy7's evidence) and
// two structural ones. PREVIEW_PIXEL_FULL=1 runs every registered pattern.
var previewPixelSample = []string{"card-grid", "kpi-3up", "icon-row", "before-after-compact", "metric-list", "strategy-house"}

// TestPatternPreviewPixelsMatchGeneratedSlide is the go-slide-creator-r1uy7
// pixel gate: the PNG preview-patterns writes for a pattern matches, within
// previewPixelTolerance, an independent conversion of the one-slide deck
// render_deck_spec generates for the same exemplar values on the same
// template. It runs on two tracked templates (and p-style when present).
func TestPatternPreviewPixelsMatchGeneratedSlide(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	reg := patterns.Default()
	names := previewPixelSample
	if os.Getenv("PREVIEW_PIXEL_FULL") == "1" {
		names = nil
		for _, p := range reg.List() {
			names = append(names, p.Name())
		}
	}
	const density = 60
	for _, tpl := range previewTestTemplates() {
		mc := semanticTestConfig(t)
		for _, name := range names {
			t.Run(tpl+"/"+name, func(t *testing.T) {
				png := filepath.Join(t.TempDir(), name+".png")
				if err := mc.writePatternPreview(context.Background(), reg, name, tpl, png, density); err != nil {
					t.Fatalf("preview: %v", err)
				}
				previewData, err := os.ReadFile(png)
				if err != nil {
					t.Fatal(err)
				}
				spec, err := patternPreviewSpec(reg, name, tpl)
				if err != nil {
					t.Fatal(err)
				}
				// force converts the generated deck afresh instead of reusing
				// the image stored for this slide, so the two sides are two
				// conversions.
				generated, err := render.RenderSlideOpts(renderDeckSpecPath(t, mc, spec), 0, density, true)
				if err != nil {
					t.Fatalf("render generated deck: %v", err)
				}
				generatedData, err := readSlideImageBytes(generated)
				if err != nil {
					t.Fatal(err)
				}
				if share := differingPixelShare(decodePreviewPNG(t, previewData), decodePreviewPNG(t, generatedData)); share > previewPixelTolerance {
					t.Errorf("preview differs from the generated slide in %.2f%% of pixels (tolerance %.2f%%)", share*100, previewPixelTolerance*100)
				}
			})
		}
	}
}

// imageBlocks returns the image content blocks of a tool result.
func imageBlocks(res *mcp.CallToolResult) []mcp.ImageContent {
	var out []mcp.ImageContent
	for _, c := range res.Content {
		if img, ok := c.(mcp.ImageContent); ok {
			out = append(out, img)
		}
	}
	return out
}

// previewsOf decodes the previews[] of a discovery result.
func previewsOf(t *testing.T, res *mcp.CallToolResult) []patterns.VisualPreview {
	t.Helper()
	var body struct {
		Previews []patterns.VisualPreview `json:"previews"`
	}
	if err := json.Unmarshal([]byte(textContent(res)), &body); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return body.Previews
}

// TestListSlideKindsPreviewMatchesRenderDeckSpec is the go-slide-creator-ueopl
// gate: list_slide_kinds(preview:true) returns an MCP image per named kind,
// and that image is the picture render_deck_spec + render_deck_thumbnails
// produce for the kind's example on the same template (same content hash).
func TestListSlideKindsPreviewMatchesRenderDeckSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	for _, tpl := range previewTestTemplates() {
		t.Run(tpl, func(t *testing.T) {
			mc := semanticTestConfig(t)
			kinds := []any{"kpi_snapshot", "chart_insight"}
			res, err := mc.handleListSlideKinds(context.Background(), makeRequest(map[string]any{"kinds": kinds, "template": tpl, "preview": true}))
			if err != nil || res.IsError {
				t.Fatalf("list_slide_kinds: %v %s", err, textContent(res))
			}
			previews := previewsOf(t, res)
			images := imageBlocks(res)
			if len(previews) != len(kinds) || len(images) != len(kinds) {
				t.Fatalf("got %d previews and %d images for %d kinds: %+v", len(previews), len(images), len(kinds), previews)
			}
			for i, p := range previews {
				if p.Error != "" || p.ContentHash == "" || p.ImageContentIndex != i+1 || p.Template != tpl {
					t.Fatalf("preview %d = %+v", i, p)
				}
				if _, ok := res.Content[p.ImageContentIndex].(mcp.ImageContent); !ok {
					t.Errorf("content[%d] is not the image of %s", p.ImageContentIndex, p.Name)
				}
				if strings.Contains(textContent(res), "/"+p.Name+".png") {
					t.Errorf("preview of %s is referenced by a file path", p.Name)
				}
				// The same example through the authoring path.
				deck := renderDeckSpecPath(t, mc, kindPreviewSpec(semantic.SlideKind(p.Name), tpl))
				thumbs, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": deck, "density": float64(previewDensity)}))
				if err != nil || thumbs.IsError {
					t.Fatalf("render_deck_thumbnails: %v %s", err, textContent(thumbs))
				}
				var deckMeta renderedDeckThumbnailsResponse
				if err := json.Unmarshal([]byte(textContent(thumbs)), &deckMeta); err != nil || len(deckMeta.Slides) != 1 {
					t.Fatalf("thumbnails = %s (%v)", textContent(thumbs), err)
				}
				if deckMeta.Slides[0].ContentHash != p.ContentHash {
					t.Errorf("%s: preview content_hash %s, render_deck_spec slide %s", p.Name, p.ContentHash, deckMeta.Slides[0].ContentHash)
				}
			}
		})
	}
}

// TestRecommendVisualPreviewReturnsImages covers recommend_visual(preview:
// true) for a kind-backed pattern, a raw pattern, a chart and a diagram: each
// candidate's next_tool_call comes back rendered, as an image.
func TestRecommendVisualPreviewReturnsImages(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	mc := semanticTestConfig(t)
	names := []any{"kpi-3up", "labeled-rows", "gantt", "waterfall"}
	res, err := mc.handleRecommendVisual(context.Background(), makeRequest(map[string]any{
		"intent": "show the plan", "template": "midnight-blue", "candidates": names, "preview": true,
	}))
	if err != nil || res.IsError {
		t.Fatalf("recommend_visual: %v %s", err, textContent(res))
	}
	previews := previewsOf(t, res)
	if len(previews) != len(names) || len(imageBlocks(res)) != len(names) {
		t.Fatalf("got %d previews and %d images for %d candidates: %+v", len(previews), len(imageBlocks(res)), len(names), previews)
	}
	seen := map[string]bool{}
	for _, p := range previews {
		if p.Error != "" || p.ContentHash == "" || p.Template != "midnight-blue" {
			t.Errorf("preview = %+v", p)
		}
		seen[p.Name] = true
	}
	for _, n := range names {
		if !seen[n.(string)] {
			t.Errorf("no preview for %s", n)
		}
	}
	if strings.Contains(textContent(res), "preview_call") {
		t.Error("a response that carries previews should not also point at the preview call")
	}
}

// TestRecommendVisualPreviewReferences covers what a response hands out when
// no preview was asked for: a runnable preview_call instead of a file path to
// a pattern gallery image (go-slide-creator-ueopl), and no path to an image
// that does not exist (go-slide-creator-r1uy7).
func TestRecommendVisualPreviewReferences(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent": "three headline metrics", "template": "midnight-blue", "candidates": []any{"kpi-3up", "card-grid", "bar"},
	})
	if rec.PreviewCall == nil || rec.PreviewCall.Tool != "recommend_visual" || rec.PreviewCall.ArgsTemplate["preview"] != true {
		t.Fatalf("preview_call = %+v", rec.PreviewCall)
	}
	if rec.PreviewCall.ArgsTemplate["template"] != "midnight-blue" || rec.PreviewCall.ArgsTemplate["intent"] != "three headline metrics" {
		t.Errorf("preview_call does not repeat the call's intent and template: %+v", rec.PreviewCall.ArgsTemplate)
	}
	if len(rec.Previews) != 0 {
		t.Errorf("previews without preview:true: %+v", rec.Previews)
	}
	for _, c := range rec.Candidates {
		if c.Example == nil {
			continue
		}
		if c.Category != patterns.VisualCategoryPlaceholder && c.Example.PreviewPNGPath != "" {
			t.Errorf("%s: preview_png_path %q names a gallery image; previews are images now", c.Name, c.Example.PreviewPNGPath)
		}
		for _, path := range []string{c.Example.PreviewPNGPath, c.Example.LayoutPreviewPNGPath} {
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s: preview path %q does not exist", c.Name, path)
			}
		}
	}
}

// TestPreviewArgumentContract covers the preview argument before any render.
func TestPreviewArgumentContract(t *testing.T) {
	mc := testMCPConfig(t)
	ctx := context.Background()
	for name, args := range map[string]map[string]any{
		"needs kinds":    {"preview": true},
		"too many kinds": {"preview": true, "kinds": []any{"title", "section", "kpi_snapshot", "chart_insight", "table"}},
		"not a boolean":  {"preview": "yes", "kinds": []any{"title"}},
	} {
		res, err := mc.handleListSlideKinds(ctx, makeRequest(args))
		if err != nil || !res.IsError || !strings.Contains(textContent(res), "INVALID_PARAMETER") {
			t.Errorf("list_slide_kinds %s: want INVALID_PARAMETER, got %v %s", name, err, textContent(res))
		}
	}
	res, err := mc.handleRecommendVisual(ctx, makeRequest(map[string]any{"intent": "kpis", "preview": "yes"}))
	if err != nil || !res.IsError || !strings.Contains(textContent(res), "INVALID_PARAMETER") {
		t.Errorf("recommend_visual preview:\"yes\": want INVALID_PARAMETER, got %v %s", err, textContent(res))
	}
	// preview:false is the default call.
	res, err = mc.handleListSlideKinds(ctx, makeRequest(map[string]any{"preview": false}))
	if err != nil || res.IsError || len(imageBlocks(res)) != 0 || strings.Contains(textContent(res), `"previews"`) {
		t.Errorf("list_slide_kinds preview:false changed the response: %v", err)
	}
}

// TestPreviewRecipesCoverEveryKind keeps list_slide_kinds(preview:true)
// honest without a render toolchain: every kind's example compiles into a
// deck whose first slide is that example.
func TestPreviewRecipesCoverEveryKind(t *testing.T) {
	mc := semanticTestConfig(t)
	for _, k := range semantic.AllSlideKinds() {
		data, err := json.Marshal(kindPreviewSpec(k, "midnight-blue"))
		if err != nil {
			t.Fatal(err)
		}
		deck, err := mc.buildSpecPreviewDeck(context.Background(), data, t.TempDir())
		if err != nil {
			t.Errorf("%s: example does not build a preview deck: %v", k, err)
			continue
		}
		keys, err := render.VisibleSlideKeys(deck)
		if err != nil || len(keys) != 1 {
			t.Errorf("%s: preview deck has %d slides (%v), want the example alone", k, len(keys), err)
		}
	}
}

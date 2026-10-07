package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/types"
)

// distinctPNG encodes a small PNG whose bytes are unique to this test, so the
// assertion "the deck embeds exactly this file" cannot pass by accident.
func distinctPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 37, 23))
	for x := 0; x < 37; x++ {
		for y := 0; y < 23; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 11), B: 123, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func imageCaseSpec(image map[string]any) map[string]any {
	return map[string]any{
		"meta": map[string]any{"title": "Asset root test", "template": "midnight-blue"},
		"slides": []any{map[string]any{
			"kind": "image_case", "title": "Asset root test", "image": image,
			"body": "A relative image belongs to the supplied authoring directory.",
		}},
	}
}

// pptxParts reads every part of a rendered deck.
func pptxParts(t *testing.T, path string) map[string][]byte {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = z.Close() }()
	out := map[string][]byte{}
	for _, f := range z.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open part %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read part %s: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

func embedsBytes(parts map[string][]byte, want []byte) bool {
	for name, b := range parts {
		if strings.HasPrefix(name, "ppt/media/") && bytes.Equal(b, want) {
			return true
		}
	}
	return false
}

func renderDeckSpecCall(t *testing.T, mc *mcpConfig, args map[string]any) renderDeckSpecResponse {
	t.Helper()
	res := mustCall(t, mc.handleRenderDeckSpec, args)
	var out renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &out)
	if res.IsError && out.PptxPath == "" && out.Error == "" && len(out.Diagnostics) == 0 {
		t.Fatalf("render_deck_spec error result: %s", textContent(res))
	}
	return out
}

// go-slide-creator-b7qqg.1: a relative image in a DeckSpec resolves against
// base_dir and its bytes land in the deck, instead of a "file not found"
// warning on a success:true render.
func TestRenderDeckSpec_RelativeImageResolvesAgainstBaseDir(t *testing.T) {
	dir := t.TempDir()
	pngBytes := distinctPNG(t)
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	mc := semanticTestConfig(t)
	out := renderDeckSpecCall(t, mc, map[string]any{
		"spec":     imageCaseSpec(map[string]any{"path": "photo.png"}),
		"base_dir": dir,
	})
	if !out.Success {
		t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
	}
	for _, w := range out.Warnings {
		if strings.Contains(w, "not found") {
			t.Errorf("relative image was not resolved: warning %q", w)
		}
	}
	if !embedsBytes(pptxParts(t, out.PptxPath), pngBytes) {
		t.Error("the deck does not embed the relative image's bytes")
	}
}

// A missing relative image is refused with a structured, semantic-path
// diagnostic — the same guard generate_presentation applies.
func TestRenderDeckSpec_MissingRelativeImageIsRefused(t *testing.T) {
	mc := semanticTestConfig(t)
	out := renderDeckSpecCall(t, mc, map[string]any{
		"spec":     imageCaseSpec(map[string]any{"path": "absent.png"}),
		"base_dir": t.TempDir(),
	})
	if out.Success {
		t.Fatal("a missing relative image must not render as success")
	}
	found := false
	for _, d := range out.Diagnostics {
		if d.RawPath != "" && strings.Contains(d.Message, "absent.png") {
			found = true
			if !strings.HasPrefix(d.SemanticPath, "slides[0]") {
				t.Errorf("asset diagnostic not mapped to a DeckSpec path: %+v", d)
			}
		}
	}
	if !found {
		t.Errorf("no asset diagnostic for the missing image; diags=%+v error=%q", out.Diagnostics, out.Error)
	}
}

// URL images go through the server's resource resolver (mc.resolverOpts), and
// its cache must outlive generation for the bytes to be embedded.
func TestRenderDeckSpec_ImageURLResolvesThroughResolver(t *testing.T) {
	pngBytes := distinctPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	t.Cleanup(server.Close)

	mc := semanticTestConfig(t)
	mc.resolverOpts = resource.ResolverOptions{HTTPClient: server.Client()}
	out := renderDeckSpecCall(t, mc, map[string]any{
		"spec": imageCaseSpec(map[string]any{"url": server.URL + "/photo.png"}),
	})
	if !out.Success {
		t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
	}
	if !embedsBytes(pptxParts(t, out.PptxPath), pngBytes) {
		t.Error("the deck does not embed the URL image's bytes")
	}
}

// go-slide-creator-b7qqg.11: every SVG knob comes from the server config.
func TestWithServerSVGConfig(t *testing.T) {
	cases := []types.SVGConfig{
		{Strategy: types.SVGStrategyPNG, Scale: 3.5, NativeCompatibility: types.SVGCompatWarn, MaxPNGWidth: 1234},
		{Strategy: types.SVGStrategyEMF, Scale: 1.25, NativeCompatibility: types.SVGCompatFallback, MaxPNGWidth: 800},
		{Strategy: types.SVGStrategyNative, Scale: 2, NativeCompatibility: types.SVGCompatStrict, MaxPNGWidth: 4096},
	}
	for _, c := range cases {
		mc := &mcpConfig{}
		mc.cfg.SVG = c
		got := mc.withServerSVGConfig(RenderOptions{OutputDir: "x"})
		if got.SVGStrategy != string(c.Strategy) || got.SVGScale != c.Scale ||
			got.SVGNativeCompat != string(c.NativeCompatibility) || got.MaxPNGWidth != c.MaxPNGWidth {
			t.Errorf("config %+v produced options %+v", c, got)
		}
		if got.OutputDir != "x" {
			t.Error("withServerSVGConfig must keep the other options")
		}
	}
}

// svgConfigSpec renders a placeholder chart, the media the SVG strategy
// governs. (Pattern-embedded shape_grid diagrams are covered by
// grid_diagram_svg_strategy_test.go, go-slide-creator-4c9m7.)
var svgConfigSpec = map[string]any{
	"meta": map[string]any{"title": "Config parity", "template": "midnight-blue"},
	"slides": []any{map[string]any{
		"kind": "raw_json2pptx",
		"slide": map[string]any{
			"slide_type": "chart",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Revenue rises every quarter"},
				map[string]any{"placeholder_id": "body", "type": "chart", "chart_value": map[string]any{
					"type": "bar", "title": "Revenue by quarter ($M)",
					"alt":  "Revenue rises from $12M in Q1 to $18M in Q3.",
					"data": map[string]any{"Q1": 12.0, "Q2": 14.5, "Q3": 18.0},
				}},
			},
		},
	}},
}

func mediaExtensions(parts map[string][]byte) []string {
	var out []string
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") {
			out = append(out, filepath.Ext(name))
		}
	}
	sort.Strings(out)
	return out
}

// An operator's PNG strategy reaches the semantic render (no native SVG part),
// and the semantic and raw paths embed the same media for the same deck.
func TestRenderDeckSpec_HonorsServerSVGStrategyLikeRaw(t *testing.T) {
	for _, strategy := range []types.SVGConversionStrategy{types.SVGStrategyPNG, types.SVGStrategyNative} {
		t.Run(string(strategy), func(t *testing.T) {
			mc := semanticTestConfig(t)
			mc.cfg.SVG.Strategy = strategy
			out := renderDeckSpecCall(t, mc, map[string]any{"spec": svgConfigSpec})
			if !out.Success {
				t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
			}
			semanticParts := pptxParts(t, out.PptxPath)
			semanticMedia := mediaExtensions(semanticParts)
			for name := range semanticParts {
				if strings.HasPrefix(name, "ppt/media/") {
					t.Logf("semantic media: %s", name)
				}
			}
			hasSVG := false
			for _, ext := range semanticMedia {
				hasSVG = hasSVG || ext == ".svg"
			}
			if hasSVG != (strategy == types.SVGStrategyNative) {
				t.Errorf("strategy %s: media %v", strategy, semanticMedia)
			}

			// Raw parity: generate_presentation over the compiled deck.
			specJSON, _ := json.Marshal(svgConfigSpec)
			spec, pd := semantic.Parse("deck.json", specJSON)
			if pd.HasErrors() {
				t.Fatalf("parse: %v", pd)
			}
			input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			raw, _ := json.Marshal(input)
			var presentation map[string]any
			_ = json.Unmarshal(raw, &presentation)
			res := mustCall(t, mc.handleGenerate, map[string]any{"presentation": presentation, "output_filename": "raw.pptx"})
			if res.IsError {
				t.Fatalf("generate_presentation failed: %s", textContent(res))
			}
			rawMedia := mediaExtensions(pptxParts(t, filepath.Join(mc.outputDir, "raw.pptx")))
			if strings.Join(rawMedia, ",") != strings.Join(semanticMedia, ",") {
				t.Errorf("semantic media %v != raw media %v", semanticMedia, rawMedia)
			}
		})
	}
}

// resolvedDir is dir with symlinks resolved, so two spellings of one
// directory compare equal; a directory that cannot be resolved is returned
// cleaned.
func resolvedDir(t *testing.T, dir string) string {
	t.Helper()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return filepath.Clean(dir)
}

const fourByThreeSldSz = `<p:sldSz cx="9144000" cy="6858000"`

// go-slide-creator-b7qqg.8: a bring-your-own template rendered through
// template_path survives on the deck_id: re-render, patch, validate, score and
// generate by deck_id alone all use the same 4:3 file.
func TestRenderDeckSpec_BYOTemplateSurvivesDeckID(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	mc := handleTestConfig(t)
	first := renderDeckSpecCall(t, mc, map[string]any{
		"spec": map[string]any{
			"meta":   map[string]any{"title": "Bring your own template review"},
			"slides": []any{map[string]any{"kind": "title", "title": "Bring your own template review", "subtitle": "Native four by three layout"}},
		},
		"template_path":   "tests/quality/fixtures/portability/templates/portability-4x3.pptx",
		"base_dir":        repoRoot,
		"output_filename": "byo-initial.pptx",
	})
	if !first.Success || first.DeckID == "" {
		t.Fatalf("initial BYO render failed: %s %+v", first.Error, first.Diagnostics)
	}
	assert4x3 := func(label, path string) {
		t.Helper()
		if !bytes.Contains(pptxParts(t, path)["ppt/presentation.xml"], []byte(fourByThreeSldSz)) {
			t.Errorf("%s: deck is not the 4:3 BYO template", label)
		}
	}
	assert4x3("initial", first.PptxPath)

	// Re-render with a patch, by deck_id alone.
	second := renderDeckSpecCall(t, mc, map[string]any{
		"deck_id":         first.DeckID,
		"patch":           []any{map[string]any{"op": "replace", "path": "/slides/0/subtitle", "value": "Patched by handle"}},
		"output_filename": "byo-followup.pptx",
	})
	if !second.Success {
		t.Fatalf("deck_id re-render failed: %s %+v", second.Error, second.Diagnostics)
	}
	assert4x3("follow-up", second.PptxPath)

	// Validate by deck_id: the template's layouts are checked, not skipped.
	// The 4:3 fixture has no quote layout, which only the template can tell.
	vres := mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"deck_id": first.DeckID,
		"patch": []any{
			map[string]any{"op": "add", "path": "/slides/-", "value": semantic.KindExample(semantic.KindQuote)},
			map[string]any{"op": "add", "path": "/meta/required_layouts", "value": []any{"quote"}},
		},
	})
	env := deckSpecEnvelope(t, vres)
	sawTemplateCheck := false
	for _, f := range env.Findings {
		if strings.Contains(f.Message, "unavailable in template") {
			sawTemplateCheck = true
		}
	}
	if !sawTemplateCheck {
		t.Errorf("validate by deck_id did not check the BYO template's layouts: %+v", env.Findings)
	}
	// Drop the requirement again; validate must not have lost the template.
	mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"deck_id": first.DeckID,
		"patch": []any{
			map[string]any{"op": "remove", "path": "/meta/required_layouts"},
			map[string]any{"op": "remove", "path": "/slides/1"},
		},
	})
	// Compared with symlinks resolved on both sides: the handle may store the
	// resolved directory while the test reached the checkout through a link
	// (/tmp is /private/tmp on macOS; go-slide-creator-jequz).
	h, _ := mc.deckHandles.Load(first.DeckID)
	if h == nil || h.TemplatePath == "" || resolvedDir(t, h.BaseDir) != resolvedDir(t, repoRoot) {
		t.Fatalf("handle lost its BYO template after validate (want base_dir %s): %+v", repoRoot, h)
	}

	// Score by deck_id alone.
	sres := mustCall(t, mc.handleScoreDeck, map[string]any{"deck_id": first.DeckID})
	if sres.IsError {
		t.Fatalf("score_deck by BYO deck_id failed: %s", textContent(sres))
	}

	// Generate the raw deck by deck_id alone.
	gres := mustCall(t, mc.handleGenerate, map[string]any{"deck_id": first.DeckID, "output_filename": "byo-raw.pptx"})
	if gres.IsError {
		t.Fatalf("generate_presentation by BYO deck_id failed: %s", textContent(gres))
	}
	assert4x3("raw", filepath.Join(mc.outputDir, "byo-raw.pptx"))

	// An explicit registered template on the call renders THIS call on it and
	// says so; the deck_id stays bound to its BYO file
	// (go-slide-creator-2dit4).
	third := renderDeckSpecCall(t, mc, map[string]any{"deck_id": first.DeckID, "template": "midnight-blue", "output_filename": "byo-switch.pptx"})
	if !third.Success {
		t.Fatalf("one-off template render failed: %s", third.Error)
	}
	if bytes.Contains(pptxParts(t, third.PptxPath)["ppt/presentation.xml"], []byte(fourByThreeSldSz)) {
		t.Error("template=midnight-blue still rendered the 4:3 BYO file")
	}
	if len(third.Warnings) == 0 || !strings.Contains(third.Warnings[0], "stays bound") {
		t.Errorf("one-off template render did not say the deck_id keeps its template: %v", third.Warnings)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h == nil || h.TemplatePath == "" || h.Template != "" || resolvedDir(t, h.BaseDir) != resolvedDir(t, repoRoot) {
		t.Errorf("a template argument rebound the handle: %+v", h)
	}

	// The deck's template changes by patch.
	fourth := renderDeckSpecCall(t, mc, map[string]any{
		"deck_id":         first.DeckID,
		"patch":           []any{map[string]any{"op": "add", "path": "/meta/template", "value": "midnight-blue"}},
		"output_filename": "byo-patched.pptx",
	})
	if !fourth.Success || fourth.Template != "midnight-blue" {
		t.Fatalf("patching /meta/template failed: %s (template %q)", fourth.Error, fourth.Template)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h == nil || h.TemplatePath != "" || h.Template != "midnight-blue" {
		t.Errorf("handle after patching /meta/template: %+v", h)
	}
}

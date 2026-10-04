package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

// go-slide-creator-1w3uo: the image tools accept a slide id wherever they
// accept an index. The id is resolved against the exact file render_deck_spec
// wrote, follows the slide through an insert, and an id the deck does not have
// is refused with the ids it does have — before anything is rendered.
func TestRenderToolsResolveSlideIDs(t *testing.T) {
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": revisionTestSpec})
	if !first.Success || first.PptxPath == "" {
		t.Fatalf("first render failed: %+v", first)
	}
	if idx, err := slideIDIndex(first.PptxPath, "costs"); err != nil || idx != 2 {
		t.Fatalf("slideIDIndex(costs) = %d, %v; want 2", idx, err)
	}
	if idx, err := slideIDIndex(first.PptxPath, "s1"); err != nil || idx != 0 {
		t.Errorf("slideIDIndex(s1) = %d, %v; want the assigned id of the first slide", idx, err)
	}

	// Mixed ids and indices become indices the shared parser reads.
	req, errRes := withResolvedSlideIDs("render_deck_thumbnails", makeRequest(map[string]any{
		"pptx_path": first.PptxPath, "slide_indices": []any{"costs", float64(0)},
	}), first.PptxPath)
	if errRes != nil {
		t.Fatalf("resolving [costs, 0] was refused: %s", resultText(errRes))
	}
	if indices, present, bad := slideIndicesArg(req); bad != nil || !present || !slices.Equal(indices, []int{0, 2}) {
		t.Errorf("slide_indices [costs, 0] resolved to %v (present=%v), want [0 2]", indices, present)
	}

	// The response names each slide by id beside its index.
	metas := []renderedSlideMeta{{Index: 2}, {Index: 3}}
	stampSlideIDs(first.PptxPath, metas)
	if metas[0].ID != "costs" || metas[1].ID != "s3" {
		t.Errorf("stamped ids = %q, %q; want costs, s3", metas[0].ID, metas[1].ID)
	}

	// An unknown id is refused by both tools, naming the ids the deck has.
	for name, call := range map[string]func() (string, bool){
		"render_deck_thumbnails": func() (string, bool) {
			res := mustCall(t, mc.handleRenderDeckThumbnails, map[string]any{"pptx_path": first.PptxPath, "slide_indices": []any{"risks"}})
			return resultText(res), res.IsError
		},
		"render_slide_image": func() (string, bool) {
			res := mustCall(t, mc.handleRenderSlideImage, map[string]any{"pptx_path": first.PptxPath, "slide_id": "risks"})
			return resultText(res), res.IsError
		},
	} {
		text, isErr := call()
		if !isErr || !strings.Contains(text, `no slide has id \"risks\"`) || !strings.Contains(text, "s1, s2, costs, s3") {
			t.Errorf("%s: an unknown id should be refused with the deck's ids, got:\n%s", name, text)
		}
	}
	both := mustCall(t, mc.handleRenderSlideImage, map[string]any{"pptx_path": first.PptxPath, "slide_id": "costs", "slide_index": float64(1)})
	if !both.IsError || !strings.Contains(resultText(both), "AMBIGUOUS_INPUT") {
		t.Errorf("slide_id with slide_index should be refused as ambiguous, got:\n%s", resultText(both))
	}

	// After an insert the id addresses the same slide at its new index.
	newSlide := map[string]any{"kind": "kpi_snapshot", "title": "Churn is flat", "kpis": []any{map[string]any{"value": "2%", "label": "Churn"}, map[string]any{"value": "0", "label": "Change"}}}
	second := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID,
		"patch": []any{map[string]any{"op": "add", "path": "/slides/costs", "value": newSlide}}})
	if !second.Success {
		t.Fatalf("patched render failed: %+v", second)
	}
	if idx, err := slideIDIndex(second.PptxPath, "costs"); err != nil || idx != 3 {
		t.Errorf("after an insert slideIDIndex(costs) = %d, %v; want 3", idx, err)
	}

	// A file nothing is known about has no ids, and says where they come from.
	stranger := filepath.Join(t.TempDir(), "other.pptx")
	data, err := os.ReadFile(second.PptxPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stranger, append(data, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := slideIDIndex(stranger, "costs"); err == nil || !strings.Contains(err.Error(), "no slide ids are known") {
		t.Errorf("an unknown file should have no ids, got %v", err)
	}
}

// TestRenderThumbnailsBySlideIDIntegration renders by id through LibreOffice.
func TestRenderThumbnailsBySlideIDIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": revisionTestSpec})

	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path": first.PptxPath, "slide_indices": []any{"costs", "s1"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	deck := assertImageContentDeck(t, res, 2)
	if deck.Slides[0].Index != 0 || deck.Slides[0].ID != "s1" || deck.Slides[1].Index != 2 || deck.Slides[1].ID != "costs" {
		t.Errorf("slides = %+v; want index 0 (s1) and index 2 (costs)", deck.Slides)
	}
	if !slices.Equal(deck.Selected, []int{0, 2}) {
		t.Errorf("selected = %v, want [0 2]", deck.Selected)
	}

	one, err := mc.handleRenderSlideImage(context.Background(), makeRequest(map[string]any{"pptx_path": first.PptxPath, "slide_id": "costs"}))
	if err != nil || one.IsError {
		t.Fatalf("render_slide_image slide_id: %v %s", err, resultText(one))
	}
	var meta renderedSlideImageResponse
	if err := json.Unmarshal([]byte(textContent(one)), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Index != 2 || meta.ID != "costs" {
		t.Errorf("render_slide_image returned index %d id %q; want 2, costs", meta.Index, meta.ID)
	}
}

// TestCLIRenderBySlideID: the CLI spells the same thing --slides <id> and
// --slide-id <id>, reading the ids from the sidecar `semantic render` wrote.
func TestCLIRenderBySlideID(t *testing.T) {
	work := t.TempDir()
	spec := filepath.Join(work, "deck.json")
	if err := os.WriteFile(spec, []byte(revisionTestSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	deck := filepath.Join(work, "deck.pptx")
	// The spec carries a blocking review finding (it closes on "Questions?"),
	// so the exit status is 1; the deck and its sidecar are written all the same.
	stdout, stderr, _ := cliRun(t, nil, "semantic", "render", spec, "--out", deck, "--templates-dir", "../../templates")
	if _, err := os.Stat(deck + authoringManifestSuffix); err != nil {
		t.Fatalf("semantic render wrote no sidecar: %v\n%.400s\n%s", err, stdout, stderr)
	}

	// go-slide-creator-cmwmg: the CLI assigns the ids MCP render_deck_spec
	// assigns to the same spec, lists them in its result and records them in
	// the sidecar, so an assigned id resolves like an authored one.
	var rendered semanticRenderResult
	decodeOneJSON(t, stdout, &rendered)
	viaMCP := renderDeckSpec(t, handleTestConfig(t), map[string]any{"spec": revisionTestSpec})
	var ids, mcpIDs, recorded []string
	for _, s := range rendered.Slides {
		ids = append(ids, s.ID)
	}
	for _, s := range viaMCP.Slides {
		mcpIDs = append(mcpIDs, s.ID)
	}
	for _, s := range slideRefsForPptx(deck) {
		recorded = append(recorded, s.ID)
	}
	if len(ids) == 0 || !slices.Equal(ids, mcpIDs) || !slices.Equal(recorded, mcpIDs) {
		t.Fatalf("slide ids: CLI result %v, sidecar %v, MCP render_deck_spec %v; want all three equal", ids, recorded, mcpIDs)
	}
	if !slices.Contains(ids, "costs") || !slices.Contains(ids, "s1") {
		t.Fatalf("ids %v: want the authored id kept and the others assigned", ids)
	}
	if idx, err := slideIDIndex(deck, "s1"); err != nil || idx != 0 {
		t.Errorf("slideIDIndex(s1) = %d, %v; want the assigned id of the first slide", idx, err)
	}
	known := "(ids: " + strings.Join(ids, ", ") + ")"

	// Refused before rendering, so this half needs no LibreOffice.
	for _, args := range [][]string{
		{"render-slide", deck, "--slide-id", "risks"},
		{"render-thumbnails", deck, "--slides", "0,risks"},
	} {
		stdout, _, code := cliRun(t, nil, args...)
		var env cliEnvelope
		decodeOneJSON(t, stdout, &env)
		if code == 0 || len(env.Findings) != 1 || !strings.Contains(env.Findings[0].Message, `no slide has id "risks" `+known) {
			t.Errorf("%v: exit=%d envelope=%s; want the unknown id refused with the deck's ids", args, code, strings.TrimSpace(stdout))
		}
	}

	if testing.Short() || !cliRenderToolsAvailable() {
		return
	}
	env := []string{"TMPDIR=" + filepath.Join(work, "tmp")}
	if err := os.MkdirAll(filepath.Join(work, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	slides := filepath.Join(work, "slides")
	var code int
	stdout, stderr, code = cliRun(t, env, "render-thumbnails", deck, "--slides", "costs", "--out-dir", slides)
	if code != 0 {
		t.Fatalf("render-thumbnails --slides costs exited %d: %s", code, stderr)
	}
	var manifest cliRenderManifest
	decodeOneJSON(t, stdout, &manifest)
	if len(manifest.Slides) != 1 || manifest.Slides[0].Index != 2 || manifest.Slides[0].ID != "costs" {
		t.Fatalf("manifest slides = %+v; want index 2 with id costs", manifest.Slides)
	}
	if _, err := os.Stat(filepath.Join(slides, "slide-2.png")); err != nil {
		t.Errorf("slide-2.png not written: %v", err)
	}
	one := filepath.Join(work, "costs.png")
	stdout, stderr, code = cliRun(t, env, "render-slide", deck, "--slide-id", "costs", "--out", one)
	if code != 0 {
		t.Fatalf("render-slide --slide-id exited %d: %s", code, stderr)
	}
	decodeOneJSON(t, stdout, &manifest)
	if len(manifest.Slides) != 1 || manifest.Slides[0].Index != 2 || manifest.Slides[0].ID != "costs" {
		t.Errorf("render-slide manifest = %+v; want index 2 with id costs", manifest.Slides)
	}
}

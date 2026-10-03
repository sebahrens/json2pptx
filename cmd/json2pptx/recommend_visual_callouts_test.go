package main

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-n3j96: "screenshot with callouts" is answered with the
// image_case kind and a recipe that carries callouts on a real picture, and
// that recipe validates and renders as it stands — callouts drawn, none
// cropped.
func TestRecommendVisualScreenshotCallouts(t *testing.T) {
	mc := handleTestConfig(t)
	for _, tmpl := range recommendVisualEvalTemplates() {
		for _, intent := range []string{
			"screenshot with callouts",
			"annotated screenshot",
			"a product screenshot with callouts pointing at parts of the UI",
			"annotate a screenshot of the dashboard",
		} {
			rec := recommendVisualFor(t, mc, map[string]any{"intent": intent, "template": tmpl})
			if len(rec.Candidates) == 0 {
				t.Fatalf("%s / %q: no candidates", tmpl, intent)
			}
			top := rec.Candidates[0]
			if top.DeckSpec == nil || top.DeckSpec.Kind != string(semantic.KindImageCase) {
				t.Errorf("%s / %q: top is %s %q (deckspec %+v), want the image_case kind", tmpl, intent, top.Category, top.Name, top.DeckSpec)
				continue
			}
			optional := strings.Join(top.DeckSpec.Fields.Optional, ",")
			if !strings.Contains(optional, "callouts") {
				t.Errorf("%s / %q: deckspec.fields does not list callouts: %s", tmpl, intent, optional)
			}
			if top.DataContract == nil || !strings.Contains(top.DataContract.Description, "callouts [{label") {
				t.Errorf("%s / %q: the data contract does not describe callouts", tmpl, intent)
			}
			slide := recipeArgs(t, top)["spec"].(map[string]any)["slides"].([]any)[0].(map[string]any)
			callouts, _ := slide["callouts"].([]any)
			if len(callouts) < 2 {
				t.Errorf("%s / %q: recipe carries %d callouts, want at least 2", tmpl, intent, len(callouts))
				continue
			}
			for i, c := range callouts {
				m, _ := c.(map[string]any)
				_, hasX := m["x"].(float64)
				_, hasY := m["y"].(float64)
				if label, _ := m["label"].(string); label == "" || !hasX || !hasY {
					t.Errorf("%s / %q: callouts[%d] = %v, want {label, x, y}", tmpl, intent, i, c)
				}
			}

			// The recipe is blocked only by its placeholders, validates once
			// they are written over ...
			assertRecipeValidates(t, mc, top)
			// ... and renders, with every callout on the slide and none cropped.
			verdict := renderFilledRecipe(t, mc, top)
			if !verdict.Success {
				t.Errorf("%s / %q: the recipe did not render: %s %+v", tmpl, intent, verdict.Error, verdict.Diagnostics)
				continue
			}
			for _, d := range verdict.Diagnostics {
				if d.Severity == "error" || d.Code == "OVERLAY_TARGET_CROPPED" {
					t.Errorf("%s / %q: recipe raised %s %s at %s", tmpl, intent, d.Severity, d.Code, d.SemanticPath)
				}
			}
			xml := readZipEntry(t, verdict.PptxPath, "ppt/slides/slide1.xml")
			leaders := strings.Count(xml, `name="Overlay callout leader `)
			labels := strings.Count(xml, `name="Overlay callout `) - leaders
			if labels != len(callouts) || leaders != len(callouts) {
				t.Errorf("%s / %q: slide draws %d labels and %d leaders for %d callouts", tmpl, intent, labels, leaders, len(callouts))
			}
		}
	}

	// An image intent that asks for no callouts keeps the plain case-study recipe.
	plain := recommendVisualFor(t, mc, map[string]any{"intent": "customer case study with a photo", "template": "midnight-blue"})
	slide := recipeArgs(t, plain.Candidates[0])["spec"].(map[string]any)["slides"].([]any)[0].(map[string]any)
	if _, has := slide["callouts"]; has {
		t.Errorf("a case-study intent got callouts: %v", slide)
	}
}

func TestSampleScreenshotIsAReadablePNG(t *testing.T) {
	path, err := sampleScreenshot()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 1280 || cfg.Height != 720 {
		t.Fatalf("sample screenshot: %dx%d, %v; want a 1280x720 PNG", cfg.Width, cfg.Height, err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("sample path %q has no image extension", path)
	}
	for intent, want := range map[string]bool{
		"screenshot with callouts":     true,
		"an annotated photo":           true,
		"labels pointing at the chart": true,
		"customer case study":          false,
		"a bar chart of revenue":       false,
	} {
		if got := calloutIntent(`intent="` + intent + `"`); got != want {
			t.Errorf("calloutIntent(%q) = %v, want %v", intent, got, want)
		}
	}
}

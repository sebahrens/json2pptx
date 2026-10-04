package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
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

// go-slide-creator-bcefj: with ALLOWED_IMAGE_PATHS configured the recipe's
// sample screenshot sits outside every allowed root, and the recipe was
// refused as it stood. The server's own sample is admitted — the file, while
// it holds the bytes this binary draws — and nothing beside it.
func TestScreenshotCalloutRecipeRendersUnderAllowList(t *testing.T) {
	mc := handleTestConfig(t)
	mc.cfg = config.DefaultConfig()
	mc.cfg.Images.AllowedBasePaths = []string{t.TempDir()}

	rec := recommendVisualFor(t, mc, map[string]any{"intent": "screenshot with callouts", "template": "midnight-blue"})
	top := rec.Candidates[0]
	slide := recipeArgs(t, top)["spec"].(map[string]any)["slides"].([]any)[0].(map[string]any)
	sample, _ := slide["image"].(map[string]any)["path"].(string)
	if sample != sampleScreenshotPath() {
		t.Fatalf("recipe image.path = %q, want the server's sample %q", sample, sampleScreenshotPath())
	}
	verdict := renderFilledRecipe(t, mc, top)
	if !verdict.Success {
		t.Fatalf("the recipe was refused under a restricted allow-list: %s %+v", verdict.Error, verdict.Diagnostics)
	}
	for _, d := range verdict.Diagnostics {
		if d.Severity == "error" || strings.Contains(d.Code, "IMAGE") {
			t.Errorf("recipe raised %s %s at %s: %s", d.Severity, d.Code, d.SemanticPath, d.Message)
		}
	}
	if rels := readZipEntry(t, verdict.PptxPath, "ppt/slides/_rels/slide1.xml.rels"); !strings.Contains(rels, "/image") {
		t.Errorf("the sample screenshot was not embedded:\n%s", rels)
	}

	// The restriction still holds for everything else: a picture next to the
	// sample is outside the allow-list, and so is a file at a sample's path
	// that is not the sample.
	allowed := imageAllowList(mc.cfg.Images.AllowedBasePaths)
	neighbour := filepath.Join(filepath.Dir(sample), "other.png")
	if err := generator.ValidateImagePathWithConfig(neighbour, allowed); err == nil {
		t.Errorf("%s is allowed; only the sample itself should be", neighbour)
	}
	if err := generator.ValidateImagePathWithConfig(sample, allowed); err != nil {
		t.Errorf("the sample is not allowed: %v", err)
	}
	data, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	same, other, link := filepath.Join(dir, "same.png"), filepath.Join(dir, "other.png"), filepath.Join(dir, "link.png")
	if err := os.WriteFile(same, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, append([]byte(nil), data[:len(data)-1]...), 0o600); err != nil {
		t.Fatal(err)
	}
	if !sampleScreenshotIntact(same) || sampleScreenshotIntact(other) || sampleScreenshotIntact(filepath.Join(dir, "missing.png")) {
		t.Error("sampleScreenshotIntact must accept exactly the bytes this binary draws")
	}
	if err := os.Symlink(same, link); err == nil && sampleScreenshotIntact(link) {
		t.Error("a link at the sample path must not be admitted")
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

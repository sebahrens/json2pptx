package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestImageHeroBuiltInPreviewsBindIllustrativeAsset(t *testing.T) {
	patternSpec, err := patternPreviewSpec(patterns.Default(), "image-hero", "midnight-blue")
	if err != nil {
		t.Fatal(err)
	}
	kindSpec, err := kindPreviewSpec(semantic.KindImageHero, "midnight-blue")
	if err != nil {
		t.Fatal(err)
	}
	for name, spec := range map[string]map[string]any{"kind": kindSpec, "pattern_gallery": patternSpec} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(`"path":"hero.jpg"`)) || !bytes.Contains(data, []byte("Illustrative")) {
				t.Fatal("built-in preview did not identify and bind illustrative image")
			}
			deck, err := semanticTestConfig(t).buildSpecPreviewDeck(context.Background(), data, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			want, err := sampleScreenshotPNG()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for name, part := range slideParts(t, deck) {
				if strings.HasPrefix(name, "ppt/media/") && bytes.Equal(part, want) {
					found = true
				}
			}
			if !found {
				t.Fatal("preview deck omitted the explicit illustrative image")
			}
		})
	}
	fresh, err := json.Marshal(semantic.KindExample(semantic.KindImageHero))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(fresh, []byte(`"path":"hero.jpg"`)) {
		t.Fatal("preview mutated kind example")
	}
	// Binding a recipe must not mutate the registry's canonical authoring data.
	pat, _ := patterns.Default().Get("image-hero")
	raw, err := json.Marshal(pat.(patterns.Exemplar).ExemplarValues())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"path":"hero.jpg"`)) {
		t.Fatal("preview mutated registered exemplar")
	}
}
func TestImageHeroPreviewDoesNotRepairAuthoredMissingAssets(t *testing.T) {
	mc := semanticTestConfig(t)
	var err error
	mc.templatesDir, err = filepath.Abs(mc.templatesDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	for _, path := range []string{"hero.jpg", filepath.Join(t.TempDir(), "missing-authored.png")} {
		slide := semantic.KindExample(semantic.KindImageHero)
		slide["image"] = map[string]any{"path": path, "alt": "Authored image"}
		data, err := json.Marshal(recipeSpec("Authored hero", "midnight-blue", slide))
		if err != nil {
			t.Fatal(err)
		}
		_, err = mc.buildSpecPreviewDeck(context.Background(), data, t.TempDir())
		if err == nil || (!strings.Contains(err.Error(), "image") && !strings.Contains(err.Error(), "asset")) {
			t.Fatalf("missing authored image %s did not refuse: %v", path, err)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("missing authored image was created: %v", statErr)
		}
	}
}

func TestImageHeroIllustrationBindingIsExactAndReportsWriteErrors(t *testing.T) {
	for _, image := range []map[string]any{{"path": "authored.png"}, {"url": "https://example.com/hero.jpg"}, {"path": "hero.jpg", "url": "https://example.com/hero.jpg"}} {
		raw, err := json.Marshal(image)
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]any{"image": image}
		if err := bindIllustrativeHeroImage(values); err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(values["image"])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, after) {
			t.Fatal("noncanonical authored image was rebound")
		}
	}
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", blocker)
	requests := kindPreviewRequests([]string{string(semantic.KindImageHero)}, "midnight-blue")
	records, images := (&mcpConfig{}).renderPreviews(context.Background(), requests)
	if len(records) != 1 || !strings.Contains(records[0].Error, "illustration") || len(images) != 0 {
		t.Fatalf("preview record lost constructor error: %+v", records)
	}

	if _, err := kindPreviewSpec(semantic.KindImageHero, "midnight-blue"); err == nil || !strings.Contains(err.Error(), "illustration") {
		t.Fatalf("kind lost sample write error: %v", err)
	}
	if _, err := patternPreviewSpec(patterns.Default(), "image-hero", "midnight-blue"); err == nil || !strings.Contains(err.Error(), "illustration") {
		t.Fatalf("pattern lost sample write error: %v", err)
	}
	if _, err := buildPatternInputForSlide(patterns.Default(), "image-hero"); err == nil || !strings.Contains(err.Error(), "illustration") {
		t.Fatalf("planned exemplar lost sample write error: %v", err)
	}
	comp := &patterns.VisualComposition{Direction: "vertical", Regions: []patterns.VisualRegion{{Category: patterns.VisualCategoryPattern, Name: "image-hero", SizePct: 100}}}
	if _, _, err := composeRecipeEnvelope(comp, "slides[0].slide.compose", patterns.Default(), nil); err == nil || !strings.Contains(err.Error(), "illustration") {
		t.Fatalf("composition lost sample write error: %v", err)
	}
	candidate := patterns.VisualCandidate{Category: patterns.VisualCategoryCompose, Composition: comp, NextToolCall: &patterns.ToolCallSuggestion{Tool: "render_deck_spec", ArgsTemplate: map[string]any{"spec": "stale recipe"}}}
	attachComposeRecipe(&candidate, "midnight-blue", patterns.Default(), nil)
	if candidate.NextToolCall != nil || !strings.Contains(comp.Instructions, "illustration") {
		t.Fatalf("composition hid sample write failure: %+v", candidate)
	}
}

// Positive render fixtures own their illustrative assets; discovery examples
// and every authored payload keep the canonical path contract.
func illustrativeKindExample(t *testing.T, kind semantic.SlideKind) map[string]any {
	t.Helper()
	example := semantic.KindExample(kind)
	if kind == semantic.KindImageHero {
		if err := bindIllustrativeHeroImage(example); err != nil {
			t.Fatal(err)
		}
	}
	return example
}

// Test-owned raw exemplar fixtures opt into the same explicit illustration as
// discovery recipes; authored assets never pass through this constructor.
func bindTestExemplarImage(t *testing.T, name string, raw json.RawMessage) json.RawMessage {
	t.Helper()
	if name != "image-hero" {
		return raw
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	if err := bindIllustrativeHeroImage(values); err != nil {
		t.Fatal(err)
	}
	bound, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func TestImageHeroRecommendationIllustrationIsRunnable(t *testing.T) {
	rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: patterns.VisualCategoryPattern, Name: "image-hero"}}}
	attachVisualRecipes(&rec, "midnight-blue")
	if len(rec.Candidates) != 1 {
		t.Fatal("lost hero recommendation")
	}
	candidate := rec.Candidates[0]
	if candidate.NextToolCall == nil || candidate.DeckSpec == nil || candidate.DeckSpec.Kind != "image_hero" {
		t.Fatalf("hero has no native authoring recipe: %+v", candidate)
	}
	assertRecipeValidates(t, semanticTestConfig(t), candidate)
	assertRecipeRenders(t, semanticTestConfig(t), candidate)
}

func TestImageHeroIllustrativePlanAndComposeGenerate(t *testing.T) {
	reg := patterns.Default()
	plan := deckplan.BuildDeckPlan(reg, deckplan.Params{Brief: "Three slides introducing the operating platform", SlideBudget: 3, BudgetExplicit: true, MustInclude: []string{"image-hero"}}, nil)
	input, err := buildPresentationFromPlan(reg, plan, "midnight-blue", "Operating platform", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, slide := range input.Slides {
		found = found || (slide.Pattern != nil && slide.Pattern.Name == "image-hero")
	}
	if !found {
		t.Fatal("MustInclude did not reach the illustrative slide constructor")
	}
	t.Run("MustInclude", func(t *testing.T) {
		result, cleanup, err := RunPresentation(t.Context(), input, RenderOptions{OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off"})
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			t.Fatal(err)
		}
		assertHeroIllustrationEmbedded(t, result.OutputPath)
	})
	t.Run("compose", func(t *testing.T) {
		comp := &patterns.VisualComposition{Direction: "vertical", Regions: []patterns.VisualRegion{{Category: patterns.VisualCategoryPattern, Name: "image-hero", SizePct: 80}}}
		envelope, _, err := composeRecipeEnvelope(comp, "slides[0].slide.compose", reg, nil)
		if err != nil {
			t.Fatal(err)
		}
		envelope["segments"] = append(envelope["segments"].([]any), map[string]any{"size_pct": 20, "text": map[string]any{"body": "The operating platform links the team to the client."}})
		spec := recipeSpec("Operating platform", "midnight-blue", map[string]any{"kind": "raw_json2pptx", "slide": map[string]any{"slide_type": "content", "layout_id": "blank", "compose": envelope}})
		raw, err := json.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		deck, err := semanticTestConfig(t).buildSpecPreviewDeck(t.Context(), raw, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		assertHeroIllustrationEmbedded(t, deck)
		// Replacing the owned sample with an authored missing path never
		// re-enters the constructor and must still fail actual generation.
		segments := envelope["segments"].([]any)
		values := segments[0].(map[string]any)["pattern"].(map[string]any)["values"].(map[string]any)
		values["image"] = map[string]any{"path": filepath.Join(t.TempDir(), "custom-missing.png"), "alt": "Authored photograph"}
		raw, err = json.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := semanticTestConfig(t).buildSpecPreviewDeck(t.Context(), raw, t.TempDir()); err == nil {
			t.Fatal("authored missing composition asset was silently replaced")
		}
	})
}

func assertHeroIllustrationEmbedded(t *testing.T, deck string) {
	t.Helper()
	want, err := sampleScreenshotPNG()
	if err != nil {
		t.Fatal(err)
	}
	for name, part := range slideParts(t, deck) {
		if strings.HasPrefix(name, "ppt/media/") && bytes.Equal(part, want) {
			return
		}
	}
	t.Fatal("generated deck omitted the owned illustrative image")
}

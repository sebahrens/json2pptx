package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
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
}

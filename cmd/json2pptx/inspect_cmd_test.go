package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeInspectImages(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("image"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func inspectImageNames(t *testing.T, images []map[string]any) []string {
	t.Helper()
	names := make([]string, len(images))
	for i, image := range images {
		path, ok := image["path"].(string)
		if !ok {
			t.Fatalf("images[%d].path = %T, want string", i, image["path"])
		}
		names[i] = filepath.Base(path)
	}
	return names
}

func TestCollectInspectImagesPrefersDirectoryNamedSlideGroup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deck-a")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInspectImages(t, dir,
		"deck-a-slide-10.png", "deck-a-slide-2.png", "deck-a-slide-1.png",
		"slide-0.png", "slide-1.png", "final-slide-0.png", "final-slide-1.png",
		"contact-sheet.png", "crop-slide-3-footer.png", "row-0.png",
	)

	images, err := collectInspectImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"deck-a-slide-1.png", "deck-a-slide-2.png", "deck-a-slide-10.png"}
	got := inspectImageNames(t, images)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("images = %v, want %v", got, want)
	}
}

func TestCollectInspectImagesPrefersCanonicalSlideGroup(t *testing.T) {
	dir := t.TempDir()
	writeInspectImages(t, dir,
		"slide-2.jpg", "slide-0.jpg", "slide-1.jpg",
		"final-slide-0.png", "final-slide-1.png", "contact-sheet.png",
	)

	images, err := collectInspectImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"slide-0.jpg", "slide-1.jpg", "slide-2.jpg"}
	got := inspectImageNames(t, images)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("images = %v, want %v", got, want)
	}
}

func TestCollectInspectImagesAcceptsSoleNumberedGroup(t *testing.T) {
	dir := t.TempDir()
	writeInspectImages(t, dir, "render-slide-2.png", "render-slide-1.png", "contact-sheet.png")

	images, err := collectInspectImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"render-slide-1.png", "render-slide-2.png"}
	got := inspectImageNames(t, images)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("images = %v, want %v", got, want)
	}
}

func TestCollectInspectImagesRejectsAmbiguousNumberedGroups(t *testing.T) {
	dir := t.TempDir()
	writeInspectImages(t, dir, "draft-slide-0.png", "final-slide-0.png")

	_, err := collectInspectImages(dir)
	if err == nil || !strings.Contains(err.Error(), "ambiguous slide image groups") {
		t.Fatalf("error = %v, want ambiguous slide image groups", err)
	}
}

func TestCollectInspectImagesRetainsArbitraryImageFallback(t *testing.T) {
	dir := t.TempDir()
	writeInspectImages(t, dir, "overview.png", "detail.jpg", "notes.txt")

	images, err := collectInspectImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"detail.jpg", "overview.png"}
	got := inspectImageNames(t, images)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("images = %v, want %v", got, want)
	}
}

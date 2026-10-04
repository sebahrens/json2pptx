package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// TestCLIRenderManifestWritesNamedFiles covers the file-writing half of
// render-thumbnails without LibreOffice: given a render result that points at
// cached PNGs, the slides are copied to slide-<index>.png, the manifest hashes
// the bytes written, and slide files the run did not write are removed
// (go-slide-creator-92gox).
func TestCLIRenderManifestWritesNamedFiles(t *testing.T) {
	cache := t.TempDir()
	out := filepath.Join(t.TempDir(), "slides")
	var metas []renderedSlideMeta
	for idx := 0; idx < 3; idx++ {
		src := filepath.Join(cache, fmt.Sprintf("artifact-%d.png", idx))
		if err := os.WriteFile(src, []byte(fmt.Sprintf("\x89PNG slide %d", idx)), 0o600); err != nil {
			t.Fatal(err)
		}
		metas = append(metas, renderedSlideMeta{Index: idx, Path: src, Width: 667, Height: 375})
	}
	meta, err := json.Marshal(renderedDeckThumbnailsResponse{Slides: metas, SlideCount: 3, SourceHash: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(out, "slide-7.png")
	unrelated := filepath.Join(out, "notes.txt")
	for _, p := range []string{stale, unrelated} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var printErr error
	stdout := captureStdout(t, func() {
		printErr = cliPrintRenderManifest(mcpgo.NewToolResultText(string(meta)), out, func(i int) string {
			return filepath.Join(out, fmt.Sprintf("slide-%d.png", i))
		}, true)
	})
	if printErr != nil {
		t.Fatal(printErr)
	}
	var manifest cliRenderManifest
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v\n%s", err, stdout)
	}
	if len(manifest.Slides) != 3 || manifest.SlideCount != 3 || manifest.SourceHash != "abc" || manifest.OutDir != out {
		t.Fatalf("manifest = %+v", manifest)
	}
	for i, s := range manifest.Slides {
		data, err := os.ReadFile(s.Path)
		if err != nil {
			t.Fatalf("slide %d not written: %v", i, err)
		}
		sum := sha256.Sum256(data)
		if filepath.Base(s.Path) != fmt.Sprintf("slide-%d.png", i) || s.SHA256 != hex.EncodeToString(sum[:]) || s.Bytes != int64(len(data)) {
			t.Errorf("slide %d: %+v does not describe the file written", i, s)
		}
		if !numberedSlideImageRE.MatchString(filepath.Base(s.Path)) {
			t.Errorf("%s is not a name `inspect` reads", filepath.Base(s.Path))
		}
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("stale slide-7.png was left behind")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Error("a file that is not a slide image was removed")
	}

	// Without a destination the manifest reports the cache paths and says so.
	stdout = captureStdout(t, func() {
		printErr = cliPrintRenderManifest(mcpgo.NewToolResultText(string(meta)), "", nil, false)
	})
	if printErr != nil {
		t.Fatal(printErr)
	}
	manifest = cliRenderManifest{}
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Slides[0].Path != metas[0].Path || !strings.Contains(manifest.Note, "--out-dir") {
		t.Errorf("cache-path manifest = %+v", manifest)
	}

	// A single-slide result (the slide's fields at top level) works too.
	single, err := json.Marshal(renderedSlideImageResponse{renderedSlideMeta: metas[1], Delivery: "image_content"})
	if err != nil {
		t.Fatal(err)
	}
	one := filepath.Join(t.TempDir(), "one.png")
	stdout = captureStdout(t, func() {
		printErr = cliPrintRenderManifest(mcpgo.NewToolResultText(string(single)), "", singleSlideDest(one), false)
	})
	if printErr != nil {
		t.Fatal(printErr)
	}
	if _, err := os.Stat(one); err != nil || !strings.Contains(stdout, one) {
		t.Errorf("single-slide render did not write %s: %v\n%s", one, err, stdout)
	}
}

// TestCLIRenderManifestKeepsUnchangedSlides is the CLI half of
// go-slide-creator-yosa8: a slide named by --known-hashes is listed as
// unchanged, writes nothing, and the file an earlier run wrote for it survives
// the sweep a deck render does.
func TestCLIRenderManifestKeepsUnchangedSlides(t *testing.T) {
	out := t.TempDir()
	src := filepath.Join(t.TempDir(), "artifact.png")
	kept := filepath.Join(out, "slide-0.png")
	for _, p := range []string{src, kept, filepath.Join(out, "slide-5.png")} {
		if err := os.WriteFile(p, []byte("\x89PNG "+filepath.Base(p)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	meta, err := json.Marshal(renderedDeckThumbnailsResponse{SlideCount: 2, Slides: []renderedSlideMeta{
		{Index: 0, ID: "intro", ContentHash: "aa", Unchanged: true},
		{Index: 1, Path: src, ContentHash: "bb"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var printErr error
	stdout := captureStdout(t, func() {
		printErr = cliPrintRenderManifest(mcpgo.NewToolResultText(string(meta)), out, func(i int) string {
			return filepath.Join(out, fmt.Sprintf("slide-%d.png", i))
		}, true)
	})
	if printErr != nil {
		t.Fatal(printErr)
	}
	var manifest cliRenderManifest
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v\n%s", err, stdout)
	}
	if len(manifest.Slides) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if got := manifest.Slides[0]; !got.Unchanged || got.Path != "" || got.SHA256 != "" || got.ContentHash != "aa" || got.ID != "intro" {
		t.Errorf("unchanged slide entry = %+v, want index, id and content_hash only", got)
	}
	if got := manifest.Slides[1]; got.Unchanged || got.ContentHash != "bb" || filepath.Base(got.Path) != "slide-1.png" {
		t.Errorf("changed slide entry = %+v", got)
	}
	if data, err := os.ReadFile(kept); err != nil || string(data) != "\x89PNG slide-0.png" {
		t.Errorf("the unchanged slide's earlier file must be left as it was (err=%v, %q)", err, data)
	}
	if _, err := os.Stat(filepath.Join(out, "slide-5.png")); err == nil {
		t.Error("a slide file past the deck's end was left behind")
	}
}

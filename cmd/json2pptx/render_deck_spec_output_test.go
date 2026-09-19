package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// go-slide-creator-tngh. render_deck_spec had no output_filename and every
// render wrote <output_dir>/output.pptx, so two renders in one session silently
// destroyed each other: both returned success with different content_hash
// values and the same pptx_path, and only one deck was on disk. The reported
// hash then matched nothing, which makes submit_visual_review (pptx_revision
// must equal the artifact sha256) and the thumbnail cache unsatisfiable.

// renderSpecWithTitle returns a valid two-slide spec with a given deck title.
func renderSpecWithTitle(title string) string {
	return "meta:\n  title: " + title + "\n  template: midnight-blue\nslides:\n" +
		"  - kind: title\n    title: " + title + "\n  - kind: closing\n    title: Questions?\n"
}

func sha256OfFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func renderDeckSpec(t *testing.T, mc *mcpConfig, args map[string]any) renderDeckSpecResponse {
	t.Helper()
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("render_deck_spec returned go error: %v", err)
	}
	var out renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &out)
	if !out.Success {
		t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
	}
	return out
}

// TestRenderDeckSpecDistinctOutputNames is the bead's named test: two different
// specs rendered in one session must not land on the same file, and each
// response's content_hash must be true of the bytes at its own path.
func TestRenderDeckSpecDistinctOutputNames(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)

	a := renderDeckSpec(t, mc, map[string]any{"spec": renderSpecWithTitle("Nordwind Alpha")})
	b := renderDeckSpec(t, mc, map[string]any{"spec": renderSpecWithTitle("Nordwind Beta")})

	if a.PptxPath == b.PptxPath {
		t.Fatalf("both renders wrote %s — the second destroyed the first", a.PptxPath)
	}
	for _, r := range []renderDeckSpecResponse{a, b} {
		if got := sha256OfFile(t, r.PptxPath); got != r.ContentHash {
			t.Errorf("%s: content_hash %s does not match the file (%s)", r.PptxPath, r.ContentHash, got)
		}
	}
	// The default name should be recognisable, not a bare digest.
	if !strings.Contains(filepath.Base(a.PptxPath), "nordwind-alpha") {
		t.Errorf("default name %q does not carry the deck title", filepath.Base(a.PptxPath))
	}
}

// TestRenderDeckSpecHonoursOutputFilename covers the explicit argument, and
// that it is sanitised the same way generate_presentation's is.
func TestRenderDeckSpecHonoursOutputFilename(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)

	r := renderDeckSpec(t, mc, map[string]any{
		"spec": renderSpecWithTitle("Nordwind"), "output_filename": "nordwind-v2.pptx",
	})
	if filepath.Base(r.PptxPath) != "nordwind-v2.pptx" {
		t.Errorf("pptx_path = %q, want it to end in nordwind-v2.pptx", r.PptxPath)
	}
	if _, err := os.Stat(r.PptxPath); err != nil {
		t.Errorf("no file at %s: %v", r.PptxPath, err)
	}

	// Path traversal is stripped, and the file lands inside the output dir.
	esc := renderDeckSpec(t, mc, map[string]any{
		"spec": renderSpecWithTitle("Nordwind"), "output_filename": "../../escape.pptx",
	})
	if filepath.Base(esc.PptxPath) != "escape.pptx" {
		t.Errorf("sanitised name = %q, want escape.pptx", filepath.Base(esc.PptxPath))
	}
	if filepath.Dir(esc.PptxPath) != mc.outputDir {
		t.Errorf("escaped the output dir: %q is not in %q", esc.PptxPath, mc.outputDir)
	}

	// A name without the extension still produces a .pptx.
	ext := renderDeckSpec(t, mc, map[string]any{
		"spec": renderSpecWithTitle("Nordwind"), "output_filename": "no-extension",
	})
	if filepath.Base(ext.PptxPath) != "no-extension.pptx" {
		t.Errorf("pptx_path = %q, want no-extension.pptx", filepath.Base(ext.PptxPath))
	}
}

// TestRenderDeckSpecReportsOverwrote: re-rendering onto an existing file is
// allowed, but the caller is told it replaced something.
func TestRenderDeckSpecReportsOverwrote(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)
	args := map[string]any{"spec": renderSpecWithTitle("Nordwind"), "output_filename": "same.pptx"}

	first := renderDeckSpec(t, mc, args)
	if first.Overwrote {
		t.Error("first render reported overwrote=true on an empty directory")
	}
	second := renderDeckSpec(t, mc, args)
	if !second.Overwrote {
		t.Error("second render onto the same name did not report overwrote=true")
	}
}

// TestConcurrentRenderDeckSpecHashMatchesFile is the bead's other named test.
// Independent concurrent renders must each end up with a file whose bytes hash
// to the value that render reported — the guarantee submit_visual_review and
// the thumbnail cache are built on.
func TestConcurrentRenderDeckSpecHashMatchesFile(t *testing.T) {
	if testing.Short() {
		t.Skip("renders several decks")
	}
	mc := semanticTestConfig(t)

	const n = 4
	results := make([]renderDeckSpecResponse, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := mc.handleRenderDeckSpec(context.Background(),
				makeRequest(map[string]any{"spec": renderSpecWithTitle(specTitles[i])}))
			if err != nil {
				t.Errorf("render %d: %v", i, err)
				return
			}
			var out renderDeckSpecResponse
			structuredInto(t, res.StructuredContent, &out)
			results[i] = out
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i, r := range results {
		if !r.Success {
			t.Errorf("render %d failed: %s", i, r.Error)
			continue
		}
		if seen[r.PptxPath] {
			t.Errorf("render %d reused path %s — a concurrent render destroyed it", i, r.PptxPath)
		}
		seen[r.PptxPath] = true
		if got := sha256OfFile(t, r.PptxPath); got != r.ContentHash {
			t.Errorf("render %d: reported content_hash %s, file is %s — the agent's revision matches nothing on disk",
				i, r.ContentHash, got)
		}
	}
}

var specTitles = [4]string{"Deck Alpha", "Deck Beta", "Deck Gamma", "Deck Delta"}

func TestDeckSpecOutputFilename(t *testing.T) {
	tests := []struct {
		name, title string
		spec        string
		want        string
	}{
		{"slugs the title", "FY26 Growth Plan", "a", "fy26-growth-plan-"},
		{"punctuation collapses", "Q3 — Review: Part 2!", "a", "q3-review-part-2-"},
		{"no title falls back", "", "a", "deck-"},
		{"non-ascii title falls back", "日本語", "a", "deck-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deckSpecOutputFilename(tt.title, []byte(tt.spec))
			if !strings.HasPrefix(got, tt.want) || !strings.HasSuffix(got, ".pptx") {
				t.Errorf("deckSpecOutputFilename(%q) = %q, want %q… .pptx", tt.title, got, tt.want)
			}
		})
	}

	// Same spec, same name: re-rendering an unedited spec is idempotent.
	if a, b := deckSpecOutputFilename("Deck", []byte("x")), deckSpecOutputFilename("Deck", []byte("x")); a != b {
		t.Errorf("same spec produced %q then %q", a, b)
	}
	// Different spec, different name, even with the same title.
	if a, b := deckSpecOutputFilename("Deck", []byte("x")), deckSpecOutputFilename("Deck", []byte("y")); a == b {
		t.Errorf("two different specs both produced %q", a)
	}
	// A very long title cannot produce an unwieldy path.
	long := deckSpecOutputFilename(strings.Repeat("verylongword ", 20), []byte("x"))
	if len(long) > 70 {
		t.Errorf("name is %d chars: %q", len(long), long)
	}
}

// TestLockOutputPathSerializes checks the lock is per-path and that two
// spellings of one path share it.
func TestLockOutputPathSerializes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")

	var mu sync.Mutex
	inside := 0
	maxInside := 0
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := path
			if i%2 == 0 {
				p = filepath.Join(dir, "sub", "..", "deck.pptx") // same file, other spelling
			}
			unlock := lockOutputPath(p)
			mu.Lock()
			inside++
			if inside > maxInside {
				maxInside = inside
			}
			mu.Unlock()
			mu.Lock()
			inside--
			mu.Unlock()
			unlock()
		}(i)
	}
	wg.Wait()
	if maxInside != 1 {
		t.Errorf("%d goroutines held the same output path at once", maxInside)
	}

	// A different path must not be blocked by the first.
	other := lockOutputPath(filepath.Join(dir, "other.pptx"))
	done := make(chan struct{})
	go func() { lockOutputPath(path)(); close(done) }()
	<-done
	other()
}

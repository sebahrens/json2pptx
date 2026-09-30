package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// pngServer serves the small test PNG at every path except /missing*, which
// returns 404.
func pngServer(t *testing.T) *httptest.Server {
	t.Helper()
	png, err := os.ReadFile("../../internal/generator/testdata/test_image_small.png")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/missing") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// urlMediaDeck exercises every URL-bearing picture surface: a content
// image_value, a slide background, a pattern image (image-text-split) and a
// shape_grid image inside a NESTED cell grid.
func urlMediaDeck(base string) map[string]any {
	return map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{"layout_id": "content", "content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "A screenshot backs this case"},
				map[string]any{"placeholder_id": "body", "type": "image", "image_value": map[string]any{"url": base + "/content.png", "alt": "Required screenshot"}},
			}},
			map[string]any{"layout_id": "blank-title",
				"background": map[string]any{"url": base + "/bg.png"},
				"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Customer screenshot"}},
				"pattern": map[string]any{"name": "image-text-split", "values": map[string]any{
					"image": map[string]any{"url": base + "/pattern.png", "alt": "Customer dashboard"},
					"body":  "The required screenshot carries the source evidence.",
				}},
			},
			map[string]any{"layout_id": "blank-title",
				"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Nested grid picture"}},
				"shape_grid": map[string]any{"columns": []any{100}, "rows": []any{
					map[string]any{"cells": []any{map[string]any{"grid": map[string]any{"columns": []any{100}, "rows": []any{
						map[string]any{"cells": []any{map[string]any{"image": map[string]any{"url": base + "/nested.png", "alt": "Nested picture"}}}},
					}}}}},
				}},
			},
		},
	}
}

func findingCodesAt(findings []patterns.FitFinding, code string) []string {
	var paths []string
	for _, f := range findings {
		if f.Code == code {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// TestURLImageHandleRoundTrip is the go-slide-creator-b7qqg.9 regression: the
// deck handle stores the authored URLs (not the request-scoped download cache
// paths that are deleted on return), so a deck_id regeneration, score and
// repair all keep the pictures.
func TestURLImageHandleRoundTrip(t *testing.T) {
	srv := pngServer(t)
	mc := testMCPConfig(t)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	mc.resolverOpts = resource.ResolverOptions{HTTPClient: srv.Client()}

	first := mustCall(t, mc.handleGenerate, map[string]any{"presentation": urlMediaDeck(srv.URL)})
	if first.IsError {
		t.Fatalf("initial render: %s", textContent(first))
	}
	var out JSONOutput
	structuredInto(t, first.StructuredContent, &out)
	if paths := findingCodesAt(out.FitFindings, patterns.ErrCodeImageAssetUnavailable); len(paths) > 0 {
		t.Fatalf("initial render lost pictures at %v", paths)
	}

	stored, ok := mc.deckHandles.Load(out.DeckID)
	if !ok {
		t.Fatal("no deck handle stored")
	}
	raw := string(stored.RawPresentation)
	for _, name := range []string{"content.png", "bg.png", "pattern.png", "nested.png"} {
		if !strings.Contains(raw, srv.URL+"/"+name) {
			t.Errorf("stored deck lost authored URL for %s: %s", name, raw)
		}
	}
	if strings.Contains(raw, "go-slide-creator-resources") || strings.Contains(raw, filepath.ToSlash(os.TempDir())) {
		t.Errorf("stored deck references the request-scoped download cache: %s", raw)
	}

	again := mustCall(t, mc.handleGenerate, map[string]any{"deck_id": out.DeckID, "output_filename": "again.pptx"})
	if again.IsError {
		t.Fatalf("deck_id regeneration failed: %s", textContent(again))
	}
	var second JSONOutput
	structuredInto(t, again.StructuredContent, &second)
	if paths := findingCodesAt(second.FitFindings, patterns.ErrCodeImageAssetUnavailable); len(paths) > 0 {
		t.Fatalf("regeneration lost pictures at %v", paths)
	}

	score := mustCall(t, mc.handleScoreDeck, map[string]any{"deck_id": second.DeckID})
	if score.IsError {
		t.Fatalf("score_deck from handle failed: %s", textContent(score))
	}
	var ds deterministic.DeckScore
	structuredInto(t, score.StructuredContent, &ds)
	if strings.Contains(mustJSON(t, ds), patterns.ErrCodeImageAssetUnavailable) {
		t.Fatalf("score_deck from handle reports a lost picture: %s", mustJSON(t, ds))
	}

	repair := mustCall(t, mc.handleRepairSlide, map[string]any{
		"deck_id": second.DeckID, "slide_index": float64(0),
		"fixes": []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 5}}},
	})
	if repair.IsError {
		t.Fatalf("repair_slide from handle failed: %s", textContent(repair))
	}
	var rep repairSlideOutput
	structuredInto(t, repair.StructuredContent, &rep)
	if rep.DeckID != "" {
		third := mustCall(t, mc.handleGenerate, map[string]any{"deck_id": rep.DeckID, "output_filename": "third.pptx"})
		if third.IsError {
			t.Fatalf("regeneration after repair failed: %s", textContent(third))
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestScoreDeckMaterializesURLImages is the go-slide-creator-b7qqg.10
// regression: score_deck fetches URL images exactly like generation, so a
// valid URL scores clean while an unreachable one is a structured refusal
// addressed to its authored url field instead of a perfect 100.
func TestScoreDeckMaterializesURLImages(t *testing.T) {
	srv := pngServer(t)
	deck := func(url string) map[string]any {
		return map[string]any{"template": "midnight-blue", "slides": []any{map[string]any{
			"layout_id": "blank-title",
			"content":   []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Customer screenshot"}},
			"pattern": map[string]any{"name": "image-text-split", "values": map[string]any{
				"image": map[string]any{"url": url, "alt": "Customer dashboard"},
				"body":  "The required screenshot carries the source evidence.",
			}},
		}}}
	}

	t.Run("valid URL", func(t *testing.T) {
		mc := testMCPConfig(t)
		mc.resolverOpts = resource.ResolverOptions{HTTPClient: srv.Client()}
		res := mustCall(t, mc.handleScoreDeck, map[string]any{"presentation": deck(srv.URL + "/picture.png")})
		if res.IsError {
			t.Fatalf("score_deck: %s", textContent(res))
		}
		var ds deterministic.DeckScore
		structuredInto(t, res.StructuredContent, &ds)
		if strings.Contains(mustJSON(t, ds), patterns.ErrCodeImageAssetUnavailable) {
			t.Fatalf("valid URL image reported unavailable: %s", mustJSON(t, ds))
		}
	})

	t.Run("unreachable URL", func(t *testing.T) {
		mc := testMCPConfig(t)
		mc.resolverOpts = resource.ResolverOptions{HTTPClient: srv.Client()}
		res := mustCall(t, mc.handleScoreDeck, map[string]any{"presentation": deck(srv.URL + "/missing.png")})
		if !res.IsError {
			t.Fatalf("unreachable required image must not score: %s", textContent(res))
		}
		body := textContent(res)
		if !strings.Contains(body, "URL_FETCH_FAILED") || !strings.Contains(body, "/slides/0/pattern/values/image/url") {
			t.Fatalf("refusal must name URL_FETCH_FAILED at the authored url path: %s", body)
		}
	})
}

// TestMissingShapeGridImageBlocksReadiness is the go-slide-creator-b7qqg.2
// regression at the generator backstop: a picture that reaches generation
// but cannot be embedded (missing or corrupt, in a pattern or a nested grid
// cell) is a refuse-class IMAGE_ASSET_UNAVAILABLE finding addressed to the
// authored field, and deterministic_ready is false.
func TestMissingShapeGridImageBlocksReadiness(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.png")
	if err := os.WriteFile(corrupt, []byte("this is not a png at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	deck := map[string]any{"template": "midnight-blue", "slides": []any{
		map[string]any{"layout_id": "blank-title",
			"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Customer screenshot"}},
			"pattern": map[string]any{"name": "image-text-split", "values": map[string]any{
				"image": map[string]any{"path": corrupt, "alt": "Customer dashboard"},
				"body":  "The required screenshot carries the source evidence.",
			}},
		},
		map[string]any{"layout_id": "blank-title",
			"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Nested grid picture"}},
			"shape_grid": map[string]any{"columns": []any{100}, "rows": []any{
				map[string]any{"cells": []any{map[string]any{"grid": map[string]any{"columns": []any{100}, "rows": []any{
					map[string]any{"cells": []any{map[string]any{"image": map[string]any{"path": corrupt, "alt": "Nested picture"}}}},
				}}}}},
			}},
		},
	}}
	mc := testMCPConfig(t)
	res := mustCall(t, mc.handleGenerate, map[string]any{"presentation": deck})
	if res.IsError {
		t.Fatalf("generate: %s", textContent(res))
	}
	var out JSONOutput
	structuredInto(t, res.StructuredContent, &out)
	paths := findingCodesAt(out.FitFindings, patterns.ErrCodeImageAssetUnavailable)
	want := map[string]bool{
		"/slides/0/pattern/values/image":                                false,
		"/slides/1/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/image": false,
	}
	for _, p := range paths {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("missing IMAGE_ASSET_UNAVAILABLE at %s; got %v", p, paths)
		}
	}
	for _, f := range out.FitFindings {
		if f.Code == patterns.ErrCodeImageAssetUnavailable && f.Action != "refuse" {
			t.Errorf("IMAGE_ASSET_UNAVAILABLE must be refuse-class, got %q", f.Action)
		}
	}
	if out.DeterministicReady == nil || *out.DeterministicReady {
		t.Fatalf("deterministic_ready must be false when a required picture is lost")
	}
}

// TestNestedGridMissingImagePreflight: a nonexistent picture in a nested cell
// grid is caught by the asset preflight with its nested source path, like a
// top-level cell.
func TestNestedGridMissingImagePreflight(t *testing.T) {
	deck := map[string]any{"template": "midnight-blue", "slides": []any{
		map[string]any{"layout_id": "blank-title",
			"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Nested grid picture"}},
			"shape_grid": map[string]any{"columns": []any{100}, "rows": []any{
				map[string]any{"cells": []any{map[string]any{"grid": map[string]any{"columns": []any{100}, "rows": []any{
					map[string]any{"cells": []any{map[string]any{"image": map[string]any{"path": "/nonexistent/b7qqg-missing.png", "alt": "x"}}}},
				}}}}},
			}},
		},
	}}
	mc := testMCPConfig(t)
	res := mustCall(t, mc.handleGenerate, map[string]any{"presentation": deck})
	if !res.IsError {
		t.Fatalf("missing nested picture must refuse generation: %s", textContent(res))
	}
	if body := textContent(res); !strings.Contains(body, "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/image/path") {
		t.Fatalf("refusal must address the nested image path: %s", body)
	}
}

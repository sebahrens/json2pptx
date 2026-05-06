package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/template"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

func repairMC(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

// minimalDeck returns a PresentationInput JSON string with one slide containing
// the given content items.
func minimalDeck(content ...map[string]any) string {
	slides := []map[string]any{
		{
			"layout_id": "slideLayout2",
			"content":   content,
		},
	}
	deck := map[string]any{
		"template": "midnight-blue",
		"slides":   slides,
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

func TestRepairSlide_ReduceText_Bullets(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Test Slide",
		},
		map[string]any{
			"placeholder_id": "body",
			"type":           "bullets",
			"bullets_value":  []string{"one", "two", "three", "four", "five"},
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": float64(3)}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(output.AppliedFixes) != 1 {
		t.Fatalf("expected 1 applied fix, got %d", len(output.AppliedFixes))
	}
	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected reduce_text to be applied, got message: %s", output.AppliedFixes[0].Message)
	}

	// Verify bullets were truncated in the patched deck.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	for _, ci := range patched.Slides[0].Content {
		if ci.BulletsValue != nil {
			if len(*ci.BulletsValue) != 3 {
				t.Errorf("expected 3 bullets after truncation, got %d", len(*ci.BulletsValue))
			}
		}
	}
}

func TestRepairSlide_ShortenTitle(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "This is a very long title that should be truncated to a shorter length",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(20)}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected shorten_title applied")
	}

	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	for _, ci := range patched.Slides[0].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			if len(*ci.TextValue) != 20 {
				t.Errorf("expected title length 20, got %d", len(*ci.TextValue))
			}
		}
	}
}

func TestRepairSlide_SwapLayout(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Hello",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "swap_layout", "params": map[string]any{"layout_id": "slideLayout3"}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.AppliedFixes[0].Applied {
		t.Fatal("expected swap_layout applied")
	}

	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	if patched.Slides[0].LayoutID != "slideLayout3" {
		t.Errorf("expected layout_id slideLayout3, got %s", patched.Slides[0].LayoutID)
	}
}

func TestRepairSlide_SplitAtRow(t *testing.T) {
	mc := repairMC(t)

	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout2",
				"content": []any{
					map[string]any{
						"placeholder_id": "title",
						"type":           "text",
						"text_value":     "Data Table",
					},
					map[string]any{
						"placeholder_id": "body",
						"type":           "table",
						"table_value": map[string]any{
							"headers": []string{"Col A", "Col B"},
							"rows": []any{
								[]string{"r1a", "r1b"},
								[]string{"r2a", "r2b"},
								[]string{"r3a", "r3b"},
								[]string{"r4a", "r4b"},
								[]string{"r5a", "r5b"},
								[]string{"r6a", "r6b"},
							},
						},
					},
				},
			},
		},
	}
	deckJSON, _ := json.Marshal(deck)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(string(deckJSON)),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "split_at_row", "params": map[string]any{"row": float64(3)}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected split_at_row applied, got: %s", output.AppliedFixes[0].Message)
	}

	// Verify the deck now has 2 slides (6 rows / 3 per page = 2).
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	if len(patched.Slides) != 2 {
		t.Errorf("expected 2 slides after split, got %d", len(patched.Slides))
	}
}

func TestRepairSlide_UnsupportedKind(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Hello",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "reposition_shape"}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unsupported kind should not be an error, just not applied")
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.AppliedFixes[0].Applied {
		t.Fatal("expected reposition_shape to not be applied")
	}
	if output.AppliedFixes[0].Message != "kind_not_supported" {
		t.Errorf("expected message 'kind_not_supported', got %q", output.AppliedFixes[0].Message)
	}
}

func TestRepairSlide_InvalidSlideIndex(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Hello",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(5),
		"fixes":       []any{map[string]any{"kind": "swap_layout", "params": map[string]any{"layout_id": "x"}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for out-of-range slide_index")
	}
}

func TestRepairSlide_MissingFixes(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Hello",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing fixes")
	}
}

func TestRepairSlide_MultipleFixes(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "A very long title that needs to be shortened to fit properly",
		},
		map[string]any{
			"placeholder_id": "body",
			"type":           "bullets",
			"bullets_value":  []string{"a", "b", "c", "d", "e"},
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{
			map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(10)}},
			map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": float64(2)}},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(output.AppliedFixes) != 2 {
		t.Fatalf("expected 2 applied fixes, got %d", len(output.AppliedFixes))
	}
	for _, f := range output.AppliedFixes {
		if !f.Applied {
			t.Errorf("expected %s to be applied", f.Kind)
		}
	}

	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}

	// Verify title truncated.
	for _, ci := range patched.Slides[0].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			if len(*ci.TextValue) != 10 {
				t.Errorf("expected title length 10, got %d", len(*ci.TextValue))
			}
		}
	}

	// Verify bullets truncated.
	for _, ci := range patched.Slides[0].Content {
		if ci.BulletsValue != nil {
			if len(*ci.BulletsValue) != 2 {
				t.Errorf("expected 2 bullets, got %d", len(*ci.BulletsValue))
			}
		}
	}
}

// TestRepairSlide_ContractShape verifies the response shape agents depend on.
func TestRepairSlide_ContractShape(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Hello",
		},
	)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes":       []any{map[string]any{"kind": "reposition_shape"}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(textContent(result)), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Contract: patched_deck must be present.
	if _, ok := raw["patched_deck"]; !ok {
		t.Error("missing 'patched_deck' field in response")
	}

	// Contract: applied_fixes must be present and an array.
	fixesRaw, ok := raw["applied_fixes"]
	if !ok {
		t.Fatal("missing 'applied_fixes' field in response")
	}
	var fixes []map[string]any
	if err := json.Unmarshal(fixesRaw, &fixes); err != nil {
		t.Fatalf("applied_fixes is not an array: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("applied_fixes is empty")
	}

	// Each fix must have kind and applied.
	for _, f := range fixes {
		if _, ok := f["kind"]; !ok {
			t.Error("applied_fixes[].kind missing")
		}
		if _, ok := f["applied"]; !ok {
			t.Error("applied_fixes[].applied missing")
		}
	}
}

// --- replace_color tests ---

func shapeGridDeck(fillColor string) string {
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []map[string]any{
			{
				"layout_id": "slideLayout5",
				"content": []map[string]any{
					{"placeholder_id": "title", "type": "text", "text_value": "Grid Slide"},
				},
				"shape_grid": map[string]any{
					"rows": []map[string]any{
						{
							"cells": []map[string]any{
								{"shape": map[string]any{"geometry": "rect", "fill": fillColor}},
								{"shape": map[string]any{"geometry": "rect", "fill": "accent2"}},
							},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

func TestRepairSlide_ReplaceColor(t *testing.T) {
	mc := repairMC(t)

	deck := shapeGridDeck("#FFE8D4")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{map[string]any{
			"kind": "replace_color",
			"params": map[string]any{
				"from": "#FFE8D4",
				"to":   "#1A1A1A",
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(output.AppliedFixes) != 1 || !output.AppliedFixes[0].Applied {
		t.Fatalf("expected replace_color applied, got: %+v", output.AppliedFixes)
	}

	// Verify the color was changed in the patched deck.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	fill := patched.Slides[0].ShapeGrid.Rows[0].Cells[0].Shape.Fill
	var fillStr string
	if err := json.Unmarshal(fill, &fillStr); err != nil {
		t.Fatalf("unmarshal fill: %v", err)
	}
	if fillStr != "#1A1A1A" {
		t.Errorf("expected fill to be #1A1A1A, got %q", fillStr)
	}
}

func TestRepairSlide_ReplaceColor_ContrastAutoFixedParams(t *testing.T) {
	mc := repairMC(t)

	deck := shapeGridDeck("#FFE8D4")

	// Use the param names from contrast_autofixed findings.
	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{map[string]any{
			"kind": "replace_color",
			"params": map[string]any{
				"original_color":    "#FFE8D4",
				"replacement_color": "#333333",
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(output.AppliedFixes) != 1 || !output.AppliedFixes[0].Applied {
		t.Fatalf("expected replace_color applied, got: %+v", output.AppliedFixes)
	}
}

func TestRepairSlide_ReplaceColor_NotFound(t *testing.T) {
	mc := repairMC(t)

	deck := shapeGridDeck("accent1")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{map[string]any{
			"kind":   "replace_color",
			"params": map[string]any{"from": "#DEADBE", "to": "#000000"},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.AppliedFixes[0].Applied {
		t.Error("expected replace_color NOT applied when color not found")
	}
}

// --- use_semantic_color tests ---

func TestRepairSlide_UseSemanticColor_WithPath(t *testing.T) {
	mc := repairMC(t)

	deck := shapeGridDeck("#FF0000")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{map[string]any{
			"kind": "use_semantic_color",
			"params": map[string]any{
				"path":  "/slides/0/shape_grid/rows/0/cells/0/shape/fill",
				"value": "accent1",
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(output.AppliedFixes) != 1 || !output.AppliedFixes[0].Applied {
		t.Fatalf("expected use_semantic_color applied, got: %+v", output.AppliedFixes)
	}

	// Verify the fill is now "accent1".
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	fill := patched.Slides[0].ShapeGrid.Rows[0].Cells[0].Shape.Fill
	var fillStr string
	if err := json.Unmarshal(fill, &fillStr); err != nil {
		t.Fatalf("unmarshal fill: %v", err)
	}
	if fillStr != "accent1" {
		t.Errorf("expected fill to be accent1, got %q", fillStr)
	}
}

func TestRepairSlide_UseSemanticColor_NoPath_ReplacesAllHex(t *testing.T) {
	mc := repairMC(t)

	deck := shapeGridDeck("#FF0000")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation":  mustParseJSON(deck),
		"slide_index": float64(0),
		"fixes": []any{map[string]any{
			"kind":   "use_semantic_color",
			"params": map[string]any{"value": "accent3"},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected use_semantic_color applied")
	}

	// The first cell (#FF0000) should be replaced, the second (accent2) should not.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}

	var fill0, fill1 string
	_ = json.Unmarshal(patched.Slides[0].ShapeGrid.Rows[0].Cells[0].Shape.Fill, &fill0)
	_ = json.Unmarshal(patched.Slides[0].ShapeGrid.Rows[0].Cells[1].Shape.Fill, &fill1)

	if fill0 != "accent3" {
		t.Errorf("cell[0] fill should be accent3, got %q", fill0)
	}
	if fill1 != "accent2" {
		t.Errorf("cell[1] fill should remain accent2, got %q", fill1)
	}
}

func TestRepairSlide_SplitPattern(t *testing.T) {
	mc := repairMC(t)

	// Build a deck with one slide containing a 4x3 shape_grid (12 cells).
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout2",
				"content": []any{
					map[string]any{
						"placeholder_id": "title",
						"type":           "text",
						"text_value":     "Card Grid Overview",
					},
				},
				"shape_grid": map[string]any{
					"columns": 3,
					"rows": []any{
						map[string]any{"cells": []any{
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "A"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "B"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "C"}},
						}},
						map[string]any{"cells": []any{
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "D"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "E"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "F"}},
						}},
						map[string]any{"cells": []any{
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "G"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "H"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "I"}},
						}},
						map[string]any{"cells": []any{
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "J"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "K"}},
							map[string]any{"shape": map[string]any{"type": "rectangle", "text": "L"}},
						}},
					},
				},
			},
		},
	}
	deckJSON, _ := json.Marshal(deck)

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(string(deckJSON)),
		"slide_index":  float64(0),
		"fixes": []any{map[string]any{
			"kind": "split_pattern",
			"params": map[string]any{
				"first":        float64(6),
				"second":       float64(6),
				"title_part_2": "(continued)",
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected split_pattern applied, got: %s", output.AppliedFixes[0].Message)
	}

	// Verify the deck now has 2 slides.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	if len(patched.Slides) != 2 {
		t.Errorf("expected 2 slides after split, got %d", len(patched.Slides))
	}

	// Slide 1 should have 2 rows (6 cells), slide 2 should have 2 rows (6 cells).
	if patched.Slides[0].ShapeGrid == nil {
		t.Fatal("slide 1 has no shape_grid")
	}
	if patched.Slides[1].ShapeGrid == nil {
		t.Fatal("slide 2 has no shape_grid")
	}
	if len(patched.Slides[0].ShapeGrid.Rows) != 2 {
		t.Errorf("slide 1: expected 2 rows, got %d", len(patched.Slides[0].ShapeGrid.Rows))
	}
	if len(patched.Slides[1].ShapeGrid.Rows) != 2 {
		t.Errorf("slide 2: expected 2 rows, got %d", len(patched.Slides[1].ShapeGrid.Rows))
	}

	// Slide 2 title should have the "(continued)" suffix.
	slide2Title := ""
	for _, ci := range patched.Slides[1].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			slide2Title = *ci.TextValue
		}
	}
	if slide2Title != "Card Grid Overview (continued)" {
		t.Errorf("slide 2 title = %q, want %q", slide2Title, "Card Grid Overview (continued)")
	}
}

func TestRepairSlide_SplitPattern_NoGrid(t *testing.T) {
	mc := repairMC(t)

	deck := minimalDeck(map[string]any{
		"placeholder_id": "title",
		"type":           "text",
		"text_value":     "No Grid",
	})

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"slide_index":  float64(0),
		"fixes":        []any{map[string]any{"kind": "split_pattern", "params": map[string]any{"first": float64(3)}}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.AppliedFixes[0].Applied {
		t.Error("expected split_pattern not applied for slide without shape_grid")
	}
}

// --- reduce_cell_text tests ---

// gridDeck builds a test deck with one slide containing a shape_grid.
func gridDeck(cellTexts ...any) string {
	cells := make([]any, len(cellTexts))
	for i, t := range cellTexts {
		cells[i] = map[string]any{
			"shape": map[string]any{
				"geometry": "rect",
				"fill":     "accent1",
				"text":     t,
			},
		}
	}
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout2",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Grid"},
				},
				"shape_grid": map[string]any{
					"columns": len(cellTexts),
					"rows": []any{
						map[string]any{"cells": cells},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

func TestReduceCellText_SimpleString(t *testing.T) {
	mc := repairMC(t)
	deck := gridDeck("Hello World, this is a long text string")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"slide_index":  float64(0),
		"fixes": []any{map[string]any{
			"kind": "reduce_cell_text",
			"params": map[string]any{
				"cell_path": "/slides/0/shape_grid/rows/0/cells/0",
				"max_chars": float64(10),
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", textContent(result))
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected applied, got: %s", output.AppliedFixes[0].Message)
	}

	// Check the patched cell text.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched: %v", err)
	}
	cell := patched.Slides[0].ShapeGrid.Rows[0].Cells[0]
	var text string
	if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
		t.Fatalf("unmarshal text: %v", err)
	}
	runes := []rune(text)
	if len(runes) > 10 {
		t.Errorf("expected at most 10 runes, got %d: %q", len(runes), text)
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("expected ellipsis at end, got %q", text)
	}
}

func TestReduceCellText_ObjectForm(t *testing.T) {
	mc := repairMC(t)
	deck := gridDeck(map[string]any{
		"content": "This text is quite long and needs truncation",
		"bold":    true,
		"size":    float64(14),
	})

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"slide_index":  float64(0),
		"fixes": []any{map[string]any{
			"kind": "reduce_cell_text",
			"params": map[string]any{
				"cell_path": "/slides/0/shape_grid/rows/0/cells/0",
				"max_chars": float64(15),
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !output.AppliedFixes[0].Applied {
		t.Fatalf("expected applied")
	}

	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched: %v", err)
	}
	cell := patched.Slides[0].ShapeGrid.Rows[0].Cells[0]
	var obj map[string]any
	if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal text obj: %v", err)
	}
	content := obj["content"].(string)
	if len([]rune(content)) > 15 {
		t.Errorf("expected at most 15 runes, got %d: %q", len([]rune(content)), content)
	}
	// Bold should be preserved.
	if obj["bold"] != true {
		t.Error("expected bold to be preserved")
	}
}

func TestReduceCellText_MarkdownEmphasisBroken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxChars int
		wantNo   string // substring that should NOT appear in result
	}{
		{
			name:     "broken bold",
			input:    "Start **bold text** end",
			maxChars: 12,
			wantNo:   "**",
		},
		{
			name:     "broken italic",
			input:    "Start *italic text* end",
			maxChars: 10,
			wantNo:   "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateWithEllipsis(tt.input, tt.maxChars)
			runes := []rune(result)
			if len(runes) > tt.maxChars {
				t.Errorf("length %d exceeds max_chars %d: %q", len(runes), tt.maxChars, result)
			}
			// The result should not have orphaned emphasis markers.
			if tt.wantNo != "" {
				count := strings.Count(result, tt.wantNo)
				if tt.wantNo == "*" {
					// Don't count * that are part of **.
					count = strings.Count(strings.ReplaceAll(result, "**", ""), "*")
				}
				if count%2 != 0 {
					t.Errorf("orphaned %q marker in result: %q", tt.wantNo, result)
				}
			}
			if runes[len(runes)-1] != '…' {
				t.Errorf("expected ellipsis at end: %q", result)
			}
		})
	}
}


func TestReduceCellText_AlreadyWithinBudget(t *testing.T) {
	mc := repairMC(t)
	deck := gridDeck("Short")

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"slide_index":  float64(0),
		"fixes": []any{map[string]any{
			"kind": "reduce_cell_text",
			"params": map[string]any{
				"cell_path": "/slides/0/shape_grid/rows/0/cells/0",
				"max_chars": float64(100),
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if output.AppliedFixes[0].Applied {
		t.Error("expected not applied when text already within budget")
	}
}

func TestReduceCellText_MissingParams(t *testing.T) {
	mc := repairMC(t)
	deck := gridDeck("Hello")

	tests := []struct {
		name   string
		params map[string]any
	}{
		{"no cell_path", map[string]any{"max_chars": float64(5)}},
		{"no max_chars", map[string]any{"cell_path": "/slides/0/shape_grid/rows/0/cells/0"}},
		{"max_chars too small", map[string]any{"cell_path": "/slides/0/shape_grid/rows/0/cells/0", "max_chars": float64(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
				"presentation": mustParseJSON(deck),
				"slide_index":  float64(0),
				"fixes":        []any{map[string]any{"kind": "reduce_cell_text", "params": tt.params}},
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var output repairSlideOutput
			if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if output.AppliedFixes[0].Applied {
				t.Error("expected not applied with missing/invalid params")
			}
		})
	}
}

func TestReduceCellText_NoShapeGrid(t *testing.T) {
	mc := repairMC(t)
	deck := minimalDeck(map[string]any{
		"placeholder_id": "title",
		"type":           "text",
		"text_value":     "No Grid",
	})

	result, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"slide_index":  float64(0),
		"fixes": []any{map[string]any{
			"kind": "reduce_cell_text",
			"params": map[string]any{
				"cell_path": "/slides/0/shape_grid/rows/0/cells/0",
				"max_chars": float64(5),
			},
		}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if output.AppliedFixes[0].Applied {
		t.Error("expected not applied for slide without shape_grid")
	}
}

// textContent extracts the text from the first MCP content block.
func textContent(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

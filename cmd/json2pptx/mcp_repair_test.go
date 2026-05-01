package main

import (
	"context"
	"encoding/json"
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
		"json_input":  deck,
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
		"json_input":  deck,
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
		"json_input":  deck,
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
		"json_input":  string(deckJSON),
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
		"json_input":  deck,
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
		"json_input":  deck,
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
		"json_input":  deck,
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
		"json_input":  deck,
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
		"json_input":  deck,
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

// textContent extracts the text from the first MCP content block.
func textContent(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

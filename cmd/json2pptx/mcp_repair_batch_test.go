package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// bodyTooLongBullets returns six identical 20-word bullets — enough to trip
// the BODY_TOO_LONG advisory (which fires above 80 words on a body block).
func bodyTooLongBullets() []string {
	const twentyWords = "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty"
	out := make([]string, 6)
	for i := range out {
		out[i] = twentyWords
	}
	return out
}

// threeSlideHeavyDeck builds a midnight-blue deck with three slides whose body
// bullets each exceed the 80-word readability budget, producing one
// BODY_TOO_LONG finding per slide.
func threeSlideHeavyDeck() string {
	slides := make([]map[string]any, 3)
	for i := range slides {
		slides[i] = map[string]any{
			"layout_id": "slideLayout2",
			"content": []any{
				map[string]any{
					"placeholder_id": "title",
					"type":           "text",
					"text_value":     "Heavy slide",
				},
				map[string]any{
					"placeholder_id": "body",
					"type":           "bullets",
					"bullets_value": func() []any {
						bs := bodyTooLongBullets()
						out := make([]any, len(bs))
						for j, b := range bs {
							out[j] = b
						}
						return out
					}(),
				},
			},
		}
	}
	deck := map[string]any{
		"template": "midnight-blue",
		"slides":   slides,
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

func countBodyTooLong(findings []map[string]any) int {
	n := 0
	for _, f := range findings {
		if code, _ := f["code"].(string); code == "BODY_TOO_LONG" {
			n++
		}
	}
	return n
}

func TestRepairSlidesBatch_FixesThreeSlidesAndClearsFindings(t *testing.T) {
	mc := repairMC(t)

	// Sanity: confirm the unpatched deck has three BODY_TOO_LONG findings so
	// the test asserts a real reduction rather than coincidentally hitting
	// zero against an already-clean fixture.
	deckJSON := threeSlideHeavyDeck()

	// Submit one reduce_text directive per slide; max_items=2 trims the six
	// bullets to two (~40 words, below the 80-word threshold).
	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"fixes": []any{
			map[string]any{
				"slide_index": float64(0),
				"kind":        "reduce_text",
				"params":      map[string]any{"max_items": float64(2)},
			},
			map[string]any{
				"slide_index": float64(1),
				"kind":        "reduce_text",
				"params":      map[string]any{"max_items": float64(2)},
			},
			map[string]any{
				"slide_index": float64(2),
				"kind":        "reduce_text",
				"params":      map[string]any{"max_items": float64(2)},
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlidesBatchOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got, want := len(output.AppliedFixes), 3; got != want {
		t.Fatalf("expected %d applied_fixes, got %d", want, got)
	}
	for i, af := range output.AppliedFixes {
		if af.SlideIndex != i {
			t.Errorf("applied_fixes[%d].slide_index = %d, want %d", i, af.SlideIndex, i)
		}
		if af.Kind != "reduce_text" {
			t.Errorf("applied_fixes[%d].kind = %q, want reduce_text", i, af.Kind)
		}
		if !af.Applied {
			t.Errorf("applied_fixes[%d] not applied: %s", i, af.Message)
		}
	}

	// Every slide's body should now carry exactly two bullets.
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	if len(patched.Slides) != 3 {
		t.Fatalf("expected 3 slides in patched deck, got %d", len(patched.Slides))
	}
	for i, s := range patched.Slides {
		for _, ci := range s.Content {
			if ci.BulletsValue == nil {
				continue
			}
			if got := len(*ci.BulletsValue); got != 2 {
				t.Errorf("slide %d: expected 2 bullets after reduce_text, got %d", i, got)
			}
		}
	}

	// Round-trip new_findings through a generic map so the test does not
	// depend on the FitFinding struct shape, then assert no BODY_TOO_LONG
	// finding survived the batch.
	raw, _ := json.Marshal(output.NewFindings)
	var asMaps []map[string]any
	_ = json.Unmarshal(raw, &asMaps)
	if n := countBodyTooLong(asMaps); n != 0 {
		t.Errorf("expected 0 BODY_TOO_LONG findings after batch repair, got %d", n)
	}
}

func TestRepairSlidesBatch_OneFixFailsOthersStillApply(t *testing.T) {
	mc := repairMC(t)

	deckJSON := threeSlideHeavyDeck()

	// Middle directive uses an unsupported kind. The handler must record it
	// as applied:false and still run the slide-0 and slide-2 directives.
	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"fixes": []any{
			map[string]any{
				"slide_index": float64(0),
				"kind":        "reduce_text",
				"params":      map[string]any{"max_items": float64(2)},
			},
			map[string]any{
				"slide_index": float64(1),
				"kind":        "definitely_not_a_real_kind",
			},
			map[string]any{
				"slide_index": float64(2),
				"kind":        "reduce_text",
				"params":      map[string]any{"max_items": float64(2)},
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output repairSlidesBatchOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got, want := len(output.AppliedFixes), 3; got != want {
		t.Fatalf("expected %d applied_fixes, got %d", want, got)
	}

	// Slide 0 and slide 2 fixes applied; slide 1 unsupported-kind fix did not.
	if !output.AppliedFixes[0].Applied {
		t.Errorf("slide 0 reduce_text expected applied, got message=%q", output.AppliedFixes[0].Message)
	}
	if output.AppliedFixes[1].Applied {
		t.Errorf("slide 1 fake-kind directive should not be applied")
	}
	if output.AppliedFixes[1].Code != "kind_not_supported" {
		t.Errorf("slide 1 outcome: expected code 'kind_not_supported', got %q", output.AppliedFixes[1].Code)
	}
	if !output.AppliedFixes[2].Applied {
		t.Errorf("slide 2 reduce_text expected applied (later fixes must run after an earlier failure), got message=%q", output.AppliedFixes[2].Message)
	}

	// Verify the patched deck reflects slides 0 and 2 being trimmed but
	// slide 1 untouched (six bullets remain).
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatalf("unmarshal patched deck: %v", err)
	}
	for i, s := range patched.Slides {
		for _, ci := range s.Content {
			if ci.BulletsValue == nil {
				continue
			}
			got := len(*ci.BulletsValue)
			want := 2
			if i == 1 {
				want = 6
			}
			if got != want {
				t.Errorf("slide %d: expected %d bullets, got %d", i, want, got)
			}
		}
	}
}

func TestRepairSlidesBatch_RejectsMissingSlideIndex(t *testing.T) {
	mc := repairMC(t)
	deckJSON := threeSlideHeavyDeck()

	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"fixes": []any{
			map[string]any{
				// slide_index intentionally omitted
				"kind":   "reduce_text",
				"params": map[string]any{"max_items": float64(2)},
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected IsError for missing slide_index, got %+v", result)
	}
	if !strings.Contains(textContent(result), "slide_index is required") {
		t.Errorf("expected message to mention slide_index is required, got: %s", textContent(result))
	}
}

func TestRepairSlidesBatch_RejectsOutOfRangeSlideIndex(t *testing.T) {
	mc := repairMC(t)
	deckJSON := threeSlideHeavyDeck()

	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"fixes": []any{
			map[string]any{
				"slide_index": float64(99),
				"kind":        "reduce_text",
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected IsError for out-of-range slide_index, got %+v", result)
	}
	if !strings.Contains(textContent(result), "out of range") {
		t.Errorf("expected message to mention out of range, got: %s", textContent(result))
	}
}

func TestRepairSlidesBatch_MissingPresentation(t *testing.T) {
	mc := repairMC(t)
	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"fixes": []any{
			map[string]any{"slide_index": float64(0), "kind": "reduce_text"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected IsError for missing presentation, got %+v", result)
	}
}

func TestRepairSlidesBatch_EmptyFixes(t *testing.T) {
	mc := repairMC(t)
	deckJSON := threeSlideHeavyDeck()

	result, err := mc.handleRepairSlidesBatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"fixes":        []any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected IsError for empty fixes, got %+v", result)
	}
}

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// DeckSpec input handed to a raw-deck tool (go-slide-creator-xx9i).
//
// score_deck reads a raw PresentationInput: slides carry content[], shape_grid,
// or pattern. A semantic DeckSpec carries slides[].kind and per-kind payload
// fields instead, and strictUnmarshalJSON happily produces a PresentationInput
// whose slides are all empty. The tool then graded that emptiness and answered
// "overall_score 99, quality_gate PASS" for a deck it had never read — and
// reported a composition score describing its own misreading (35 on a
// well-built eight-slide board update, because every slide looked like pattern
// "content" with zero density variation).
//
// That mattered more once the composition floor joined the quality gate: a
// nonsense composition score would have turned a silent wrong answer into a
// confident wrong refusal on a good deck.

// deckSpecMarker is the value DeckSpec documents put in "_kind".
const deckSpecMarker = "spec"

// looksLikeDeckSpec reports whether the raw presentation JSON is a semantic
// DeckSpec rather than the raw PresentationInput this tool grades. A document
// qualifies when it declares itself with `_kind: "spec"`, or when every slide
// carries a `kind` and none carries raw content.
func looksLikeDeckSpec(raw []byte) bool {
	var doc struct {
		Kind   string            `json:"_kind"`
		Slides []json.RawMessage `json:"slides"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	if doc.Kind == deckSpecMarker {
		return true
	}
	if len(doc.Slides) == 0 {
		return false
	}
	for _, s := range doc.Slides {
		var slide struct {
			Kind      string          `json:"kind"`
			Content   json.RawMessage `json:"content"`
			ShapeGrid json.RawMessage `json:"shape_grid"`
			Pattern   json.RawMessage `json:"pattern"`
			Compose   json.RawMessage `json:"compose"`
		}
		if err := json.Unmarshal(s, &slide); err != nil {
			return false
		}
		if slide.Kind == "" {
			return false
		}
		if len(slide.Content) > 0 || len(slide.ShapeGrid) > 0 || len(slide.Pattern) > 0 || len(slide.Compose) > 0 {
			return false
		}
	}
	return true
}

// deckSpecInputError is the refusal a raw-deck tool returns for DeckSpec input,
// naming the tools that DO read a spec.
func deckSpecInputError(tool string) *mcp.CallToolResult {
	return mcpErrorWithNext(
		"INVALID_PARAMETER",
		fmt.Sprintf("%s grades a RAW deck (slides carry content[] / shape_grid / pattern), but this looks like a semantic DeckSpec "+
			"(slides carry kind). Scoring it would grade an empty deck and report a misleading score. "+
			"Use validate_deck_spec to check the spec, or compile_deck_spec to turn it into a raw deck and score that.", tool),
		&patterns.ToolCallSuggestion{
			Tool:         "validate_deck_spec",
			ArgsTemplate: map[string]any{"spec": "<the DeckSpec you passed as presentation>"},
		},
	)
}

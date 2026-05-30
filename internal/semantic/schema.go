package semantic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// schemaDialect is the JSON Schema dialect emitted by Schema.
const schemaDialect = "https://json-schema.org/draft/2020-12/schema"

// Schema returns a JSON Schema (draft 2020-12) describing the semantic DeckSpec
// authoring format. The slide kind and archetype enums are derived from the
// canonical registries, so the schema stays in sync with the code.
//
// This is the export scaffold backing a future `json2pptx semantic schema`
// command. Per-kind payload constraints are added by later compiler phases;
// slide payload fields are presently left open (additionalProperties: true).
func Schema() map[string]any {
	return map[string]any{
		"$schema":     schemaDialect,
		"title":       "DeckSpec",
		"description": "Compact semantic deck authoring format. A DeckSpec compiles into a json2pptx PresentationInput.",
		"type":        "object",
		"required":    []any{"slides"},
		"properties": map[string]any{
			"meta":   map[string]any{"$ref": "#/$defs/DeckMeta"},
			"slides": map[string]any{
				"type":        "array",
				"description": "Ordered list of semantic slides.",
				"items":       map[string]any{"$ref": "#/$defs/SlideSpec"},
			},
		},
		"additionalProperties": false,
		"$defs": map[string]any{
			"DeckMeta":  deckMetaSchema(),
			"SlideSpec": slideSpecSchema(),
		},
	}
}

// SchemaJSON returns the indented JSON encoding of Schema.
func SchemaJSON() ([]byte, error) {
	out, err := json.MarshalIndent(Schema(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal semantic schema: %w", err)
	}
	return out, nil
}

func deckMetaSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Deck-level intent and presentation context.",
		"properties": map[string]any{
			"title":     map[string]any{"type": "string", "description": "Deck title."},
			"subtitle":  map[string]any{"type": "string", "description": "Optional deck subtitle."},
			"archetype": map[string]any{
				"type":        "string",
				"description": "Overall purpose of the deck.",
				"enum":        archetypeEnum(),
			},
			"template": map[string]any{"type": "string", "description": "Optional json2pptx template name."},
			"audience": map[string]any{"type": "string", "description": "Intended audience (advisory)."},
			"author":   map[string]any{"type": "string", "description": "Deck author (advisory)."},
			"date":     map[string]any{"type": "string", "description": "Free-form date string (advisory)."},
		},
		"additionalProperties": false,
	}
}

func slideSpecSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "A single semantic slide: a kind discriminator plus kind-specific payload fields.\n\n" + kindDescriptions(),
		"required":    []any{"kind"},
		"properties": map[string]any{
			"kind": map[string]any{
				"type":        "string",
				"description": "Selects the slide payload shape.",
				"enum":        kindEnum(),
			},
		},
		// Payload fields vary by kind; left open in this scaffold. Later
		// compiler phases tighten per-kind constraints.
		"additionalProperties": true,
	}
}

func kindEnum() []any {
	kinds := AllSlideKinds()
	out := make([]any, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

func archetypeEnum() []any {
	arches := AllArchetypes()
	out := make([]any, len(arches))
	for i, a := range arches {
		out[i] = string(a)
	}
	return out
}

// kindDescriptions renders a per-kind summary block for the SlideSpec schema
// description, sourced from the kind registry.
func kindDescriptions() string {
	var b strings.Builder
	b.WriteString("Slide kinds:\n")
	for _, k := range AllSlideKinds() {
		if info, ok := LookupKind(k); ok {
			fmt.Fprintf(&b, "- %s: %s\n", k, info.Summary)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

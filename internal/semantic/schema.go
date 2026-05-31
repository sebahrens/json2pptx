package semantic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// schemaDialect is the JSON Schema dialect emitted by Schema.
const schemaDialect = "https://json-schema.org/draft/2020-12/schema"

// Schema returns a JSON Schema (draft 2020-12) describing the semantic DeckSpec
// authoring format. The slide kind and archetype enums, and the per-kind
// payload-field variants, are derived from the canonical registries, so the
// schema stays in sync with the code.
//
// SlideSpec is a discriminated union: each registered kind contributes a
// Slide_<kind> variant in $defs (referenced from SlideSpec.oneOf) that pins
// `kind` with const and documents that kind's required + typical payload
// fields. Payloads stay open (additionalProperties: true) — compiler-accepted
// aliases and extra keys are ignored rather than rejected.
func Schema() map[string]any {
	defs := map[string]any{
		"DeckMeta":  deckMetaSchema(),
		"SlideSpec": slideSpecSchema(),
	}
	for name, variant := range kindVariantSchemas() {
		defs[name] = variant
	}
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
		"$defs":                defs,
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
		// Discriminated union: exactly one Slide_<kind> variant applies,
		// selected by `kind`. Each variant documents that kind's required +
		// typical payload fields. Payloads stay open (additionalProperties:
		// true) so compiler-accepted aliases and extra keys are ignored
		// rather than rejected.
		"oneOf":                kindVariantRefs(),
		"additionalProperties": true,
	}
}

// kindDefName returns the $defs key for a slide kind's payload variant.
func kindDefName(k SlideKind) string {
	return "Slide_" + string(k)
}

// kindVariantRefs returns the SlideSpec.oneOf list, one $ref per registered
// kind variant, in stable (sorted) order.
func kindVariantRefs() []any {
	kinds := AllSlideKinds()
	out := make([]any, len(kinds))
	for i, k := range kinds {
		out[i] = map[string]any{"$ref": "#/$defs/" + kindDefName(k)}
	}
	return out
}

// kindVariantSchemas builds the per-kind payload variant sub-schemas keyed by
// their $defs name, sourced from the slide-kind registry.
func kindVariantSchemas() map[string]any {
	defs := map[string]any{}
	for _, k := range AllSlideKinds() {
		if info, ok := LookupKind(k); ok {
			defs[kindDefName(k)] = kindVariantSchema(info)
		}
	}
	return defs
}

// payloadFieldType maps canonical semantic payload field names to their JSON
// type, for schema hints. Fields absent from the map are emitted without a
// type constraint (payloads accept aliases, so the hint is advisory).
var payloadFieldType = map[string]string{
	"title": "string", "subtitle": "string", "eyebrow": "string",
	"takeaway": "string", "source": "string", "insight": "string",
	"recommendation": "string",
	"points":         "array", "takeaways": "array", "kpis": "array",
	"columns": "array", "steps": "array", "phases": "array",
	"options": "array", "insights": "array",
	"chart": "object", "slide": "object",
}

// payloadFieldSchema describes a single per-kind payload field.
func payloadFieldSchema(name string, required bool) map[string]any {
	s := map[string]any{}
	if t, ok := payloadFieldType[name]; ok {
		s["type"] = t
	}
	if required {
		s["description"] = "Required payload field."
	} else {
		s["description"] = "Typical (optional) payload field."
	}
	return s
}

// kindVariantSchema renders the discriminated-union variant for one slide kind:
// a const-pinned `kind`, the kind's required + typical payload fields as
// properties, and required set to {kind} ∪ RequiredFields. additionalProperties
// stays true so compiler-accepted aliases and extra keys are not rejected.
func kindVariantSchema(info KindInfo) map[string]any {
	props := map[string]any{
		"kind": map[string]any{"const": string(info.Kind)},
	}
	required := []any{"kind"}
	for _, f := range info.RequiredFields {
		props[f] = payloadFieldSchema(f, true)
		required = append(required, f)
	}
	for _, f := range info.TypicalFields {
		if _, exists := props[f]; exists {
			continue
		}
		props[f] = payloadFieldSchema(f, false)
	}
	return map[string]any{
		"type":                 "object",
		"title":                string(info.Kind) + " slide",
		"description":          info.Summary,
		"required":             required,
		"properties":           props,
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

package semantic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/layout"
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
// `kind` with const and lists exactly the payload fields that kind's compiler
// reads (canonical names plus accepted aliases, see kindPayloadFields), with
// additionalProperties:false and closed list-entry / chart object schemas. An
// unknown key is therefore schema-invalid; the validator reports the same key
// as a SEMANTIC_UNKNOWN_FIELD warning instead of silently dropping it.
func Schema() map[string]any {
	defs := map[string]any{
		"DeckMeta":      deckMetaSchema(),
		"SlideSpec":     slideSpecSchema(),
		"DeckStructure": deckStructureSchema(),
	}
	for name, variant := range kindVariantSchemas() {
		defs[name] = variant
	}
	return map[string]any{
		"$schema":     schemaDialect,
		"title":       "DeckSpec",
		"description": "Compact semantic deck authoring format. A DeckSpec compiles into a json2pptx PresentationInput.",
		"type":        "object",
		"properties": map[string]any{
			"meta": map[string]any{"$ref": "#/$defs/DeckMeta"},
			"slides": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "Ordered list of semantic slides.",
				"items":       map[string]any{"$ref": "#/$defs/SlideSpec"},
			},
			"structure": map[string]any{"$ref": "#/$defs/DeckStructure"},
		},
		"oneOf": []any{
			map[string]any{"required": []any{"slides"}, "not": map[string]any{"required": []any{"structure"}}},
			map[string]any{"required": []any{"structure"}, "not": map[string]any{"required": []any{"slides"}}},
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
			"title":    map[string]any{"type": "string", "description": "Deck title."},
			"subtitle": map[string]any{"type": "string", "description": "Optional deck subtitle."},
			"archetype": map[string]any{
				"type":        "string",
				"description": "Overall purpose of the deck.",
				"enum":        archetypeEnum(),
			},
			"template": map[string]any{"type": "string", "description": "Optional json2pptx template name."},
			"audience": map[string]any{"type": "string", "description": "Intended audience (advisory)."},
			"author":   map[string]any{"type": "string", "description": "Deck author (advisory)."},
			"date":     map[string]any{"type": "string", "description": "Free-form date string (advisory); fills chrome.footer_date when that is not set."},
			"viewing_mode": map[string]any{
				"type":        "string",
				"description": "Readability policy: \"present\" (default, stricter minimum text sizes) or \"read\".",
				"enum":        []any{"present", "read"},
			},
			"accent_strategy": map[string]any{
				"type":        "string",
				"description": "Accent colour rotation across the deck.",
				"enum":        []any{"primary", "rotate", "section-keyed"},
			},
			"design_mode": map[string]any{
				"type": "string",
				"description": "\"constrained\" (default: the template owns sizes and colours) or \"free\". " +
					"Only a deck using the raw_json2pptx escape hatch needs \"free\": the compiler's own output " +
					"never hand-sets what the template owns, and a raw slide that does is refused as " +
					"design_mode_violation — the same verdict generate_presentation gives it.",
				"enum": []any{"constrained", "free"},
			},
			"chrome": chromeSchema(),
			"required_layouts": map[string]any{
				"type": "array", "uniqueItems": true,
				"items":       map[string]any{"type": "string", "enum": stringEnum(layout.CanonicalNames())},
				"description": "Canonical layout IDs the planned deck must cover.",
			},
		},
		"additionalProperties": false,
	}
}

func deckStructureSchema() map[string]any {
	section := map[string]any{
		"type": "object", "required": []any{"title", "slides"},
		"properties": map[string]any{
			"title":  map[string]any{"type": "string", "minLength": 1},
			"slides": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"$ref": "#/$defs/SlideSpec"}},
		},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object", "required": []any{"sections"},
		"properties": map[string]any{
			"cover":       map[string]any{"$ref": "#/$defs/SlideSpec"},
			"auto_agenda": map[string]any{"type": "boolean"},
			"sections":    map[string]any{"type": "array", "minItems": 1, "items": section},
			"closing":     map[string]any{"$ref": "#/$defs/SlideSpec"},
		},
		"additionalProperties": false,
	}
}

// chromeSchema describes the deck furniture block: confidentiality stamp,
// client name, project code, footer date and page numbers
// (go-slide-creator-zmjs).
func chromeSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Deck chrome rendered into every slide's footer band.",
		"properties": map[string]any{
			"confidentiality": map[string]any{"type": "string", "description": "Classification stamp, e.g. \"Strictly confidential\"."},
			"client_name":     map[string]any{"type": "string", "description": "Client or company name."},
			"project_code":    map[string]any{"type": "string", "description": "Project identifier."},
			"footer_date":     map[string]any{"type": "string", "description": "Date shown in the footer; defaults to meta.date."},
			"section_crumb":   map[string]any{"type": "boolean", "description": "Show the running section title in the footer."},
			"page_numbers": map[string]any{
				"type":        "object",
				"description": "Slide numbering.",
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean", "description": "Default: true when chrome is set."},
					"format":  map[string]any{"type": "string", "description": "Supports {current} and {total}, e.g. \"{current} / {total}\"."},
					"skip":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Slide types that show no page number. Default: [\"title\", \"closing\"]."},
				},
				"additionalProperties": false,
			},
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
		// selected by `kind`. Each variant is closed (additionalProperties:
		// false) over the fields that kind's compiler reads, so the wrapper
		// itself stays open and defers to the variant.
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

// payloadFieldSchema renders the JSON Schema for one payload field from the
// closed payload contract (kindPayloadFields): its type, an authoring hint, and
// for lists / objects the closed entry / key schema the compiler reads.
func payloadFieldSchema(f payloadField, role string) map[string]any {
	s := map[string]any{"type": f.typ}
	desc := f.desc
	if role != "" {
		desc = strings.TrimSpace(role + " " + desc)
	}
	if desc != "" {
		s["description"] = desc
	}
	switch f.typ {
	case "array":
		if item := payloadItemSchema(f); item != nil {
			s["items"] = item
		}
	case "object":
		if len(f.objectKeys) > 0 {
			s["properties"] = objectKeySchemas(f.objectKeys)
			s["additionalProperties"] = false
		}
	}
	return s
}

// payloadItemSchema renders the entry schema for a list field: a string, a
// closed object over the keys the compiler reads, or either.
func payloadItemSchema(f payloadField) map[string]any {
	var obj map[string]any
	if len(f.itemKeys) > 0 {
		props := objectKeySchemas(f.itemKeys)
		for key, schema := range f.itemKeySchemas {
			props[key] = schema
		}
		obj = map[string]any{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": false,
		}
		if len(f.itemRequired) > 0 {
			obj["required"] = f.itemRequired
		}
	}
	switch {
	case f.itemStrings && obj != nil:
		return map[string]any{"anyOf": []any{map[string]any{"type": "string"}, obj}}
	case obj != nil:
		return obj
	case f.itemStrings:
		return map[string]any{"type": "string"}
	default:
		return nil
	}
}

// objectKeyTypes pins the JSON type of well-known entry / chart keys. Keys not
// listed are strings.
var objectKeyTypes = map[string]string{
	"items": "array", "pros": "array", "cons": "array", "bullets": "array",
	"active": "boolean", "data": "object",
}

// objectKeySchemas renders the property schemas for a closed entry / chart
// object from its key list.
func objectKeySchemas(keys []string) map[string]any {
	props := make(map[string]any, len(keys))
	for _, k := range keys {
		t, ok := objectKeyTypes[k]
		if !ok {
			t = "string"
		}
		p := map[string]any{"type": t}
		switch k {
		case "items", "pros", "cons", "bullets":
			p["items"] = map[string]any{"type": "string"}
		case "data":
			p["description"] = "Chart data. Bar/line/area: {categories:[…], series:[{name, values:[…]}]}. Pie/donut: {categories:[…], values:[…]}."
		case "type":
			p["description"] = "Chart type, e.g. bar_chart, line_chart, pie_chart (see get_chart_capabilities)."
		}
		props[k] = p
	}
	return props
}

// kindVariantSchema renders the discriminated-union variant for one slide kind:
// a const-pinned `kind`, every payload field the kind's compiler reads (from
// kindPayloadFields) as a typed property, and required set to {kind} ∪
// RequiredFields. A required field that has registered aliases is expressed as
// required-one-of (an anyOf over the canonical name and each alias) rather than
// a flat required entry, so a spec using only an alias is schema-valid —
// matching the validator and compiler, which read the aliases interchangeably.
// additionalProperties is false: a key outside the contract would be dropped by
// the compiler, so it is schema-invalid (and a SEMANTIC_UNKNOWN_FIELD finding).
func kindVariantSchema(info KindInfo) map[string]any {
	props := map[string]any{
		"kind": map[string]any{"const": string(info.Kind)},
	}
	fields := kindPayloadFields[info.Kind]
	roles := map[string]string{}
	for _, f := range info.TypicalFields {
		roles[f] = "Typical (optional) payload field."
	}
	for canonical, aliases := range info.RequiredAliases {
		for _, a := range aliases {
			roles[a] = fmt.Sprintf("Accepted alias for the required %q field.", canonical)
		}
	}
	for _, f := range info.RequiredFields {
		roles[f] = "Required payload field."
	}
	for _, name := range PayloadFieldNames(info.Kind) {
		props[name] = payloadFieldSchema(fields[name], roles[name])
	}

	required := []any{"kind"}
	var oneOfGroups []any
	for _, f := range info.RequiredFields {
		aliases := info.RequiredAliases[f]
		if len(aliases) == 0 {
			required = append(required, f)
			continue
		}
		// required-one-of: the canonical field or any alias satisfies the
		// requirement.
		opts := []any{map[string]any{"required": []any{f}}}
		for _, a := range aliases {
			opts = append(opts, map[string]any{"required": []any{a}})
		}
		oneOfGroups = append(oneOfGroups, map[string]any{"anyOf": opts})
	}
	variant := map[string]any{
		"type":                 "object",
		"title":                string(info.Kind) + " slide",
		"description":          info.Summary,
		"required":             required,
		"properties":           props,
		"additionalProperties": false,
	}
	// Each alias group becomes its own anyOf; all groups must hold, so they are
	// combined under allOf.
	if len(oneOfGroups) > 0 {
		variant["allOf"] = oneOfGroups
	}
	return variant
}

// KindItemSchema returns the closed JSON Schema for one slide of the given kind
// (the Slide_<kind> variant), or nil for an unknown kind. list_slide_kinds
// publishes it as item_schema.
func KindItemSchema(k SlideKind) map[string]any {
	info, ok := LookupKind(k)
	if !ok {
		return nil
	}
	return kindVariantSchema(info)
}

// InlineSchema returns Schema() with every local "#/$defs/..." reference
// replaced by the referenced definition and $defs / $schema removed, so the
// DeckSpec schema can be embedded as a nested property (e.g. an MCP tool's
// `spec` input) where root-relative references would not resolve.
func InlineSchema() map[string]any {
	root := Schema()
	defs, _ := root["$defs"].(map[string]any)
	out, _ := inlineRefs(root, defs).(map[string]any)
	delete(out, "$defs")
	delete(out, "$schema")
	return out
}

// CompactInlineSchema returns InlineSchema() without annotation keywords
// (description, title, examples, $comment). It keeps every structural
// constraint and supplies the meta portion used by OutlineInlineSchema.
func CompactInlineSchema() map[string]any {
	out, _ := stripAnnotations(InlineSchema(), false).(map[string]any)
	return out
}

// CompactSchemaAt keeps the schema's shared definitions and rewrites local
// references for embedding at a nested JSON-Schema path. This avoids four
// inlined copies of the complete SlideSpec union while preserving the closed
// payload contract for flat and structured slides.
func CompactSchemaAt(base string) map[string]any {
	out, _ := stripAnnotations(Schema(), false).(map[string]any)
	delete(out, "$schema")
	compactSchemaDefinitions(out)
	minifySchemaDefinitionNames(out)
	rewriteSchemaRefs(out, strings.TrimSuffix(base, "/"))
	return out
}

func minifySchemaDefinitionNames(root map[string]any) {
	defs, _ := root["$defs"].(map[string]any)
	if defs == nil {
		return
	}
	names := map[string]string{"DeckMeta": "M", "SlideSpec": "S", "DeckStructure": "T"}
	for i, kind := range AllSlideKinds() {
		names[kindDefName(kind)] = fmt.Sprintf("K%d", i)
	}
	minified := make(map[string]any, len(defs))
	for old, definition := range defs {
		name := names[old]
		if name == "" {
			name = old
		}
		minified[name] = definition
	}
	root["$defs"] = minified
	rewriteDefinitionNames(root, names)
}

func rewriteDefinitionNames(value any, names map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
			old := strings.TrimPrefix(ref, "#/$defs/")
			if name := names[old]; name != "" {
				typed["$ref"] = "#/$defs/" + name
			}
		}
		for _, child := range typed {
			rewriteDefinitionNames(child, names)
		}
	case []any:
		for _, child := range typed {
			rewriteDefinitionNames(child, names)
		}
	}
}

// compactSchemaDefinitions uses draft-2020-12 unevaluatedProperties at the
// SlideSpec union boundary. The full schema keeps each variant independently
// closed for readability; the nested MCP copy can state the shared object,
// kind type, and closure once while each selected variant evaluates its own
// payload fields. This preserves validation and saves enough repeated keywords
// to keep tools/list below its transport budget.
func compactSchemaDefinitions(root map[string]any) {
	defs, _ := root["$defs"].(map[string]any)
	slide, _ := defs["SlideSpec"].(map[string]any)
	if slide == nil {
		return
	}
	delete(slide, "additionalProperties")
	slide["unevaluatedProperties"] = false
	if properties, ok := slide["properties"].(map[string]any); ok {
		if kindProperty, ok := properties["kind"].(map[string]any); ok {
			// The selected oneOf variant already pins every supported kind with
			// const, so repeating the complete enum here adds bytes, not checks.
			delete(kindProperty, "enum")
		}
	}
	for _, kind := range AllSlideKinds() {
		variant, _ := defs[kindDefName(kind)].(map[string]any)
		if variant == nil {
			continue
		}
		delete(variant, "type")
		delete(variant, "additionalProperties")
		if properties, ok := variant["properties"].(map[string]any); ok {
			if kindProperty, ok := properties["kind"].(map[string]any); ok {
				delete(kindProperty, "type")
			}
		}
		if required, ok := variant["required"].([]any); ok {
			filtered := required[:0]
			for _, field := range required {
				if field != "kind" {
					filtered = append(filtered, field)
				}
			}
			if len(filtered) == 0 {
				delete(variant, "required")
			} else {
				variant["required"] = filtered
			}
		}
	}
}

func rewriteSchemaRefs(value any, base string) {
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
			typed["$ref"] = base + "/$defs/" + strings.TrimPrefix(ref, "#/$defs/")
		}
		for _, child := range typed {
			rewriteSchemaRefs(child, base)
		}
	case []any:
		for _, child := range typed {
			rewriteSchemaRefs(child, base)
		}
	}
}

func stringEnum(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

// OutlineInlineSchema returns the DeckSpec's SHAPE without the per-kind payload
// contracts: meta as it is, and slides as an array of objects whose `kind` is
// the registered enum, open to whatever fields that kind takes.
//
// It exists because the full closed schema is 17KB and was embedded in FOUR
// tools — twice in the core listing alone — while saying the same thing each
// time (go-slide-creator-uhaq). The full contract stays on validate_deck_spec,
// the tool the workflow already says to call before rendering, and the other
// three carry this outline plus the pointer to list_slide_kinds. An unknown
// payload field is still rejected: that check is the compiler's
// (SEMANTIC_UNKNOWN_FIELD), not the input schema's.
func OutlineInlineSchema() map[string]any {
	full := CompactInlineSchema()
	meta := map[string]any{"type": "object"}
	if props, ok := full["properties"].(map[string]any); ok {
		if m, ok := props["meta"].(map[string]any); ok {
			meta = m
		}
	}
	slide := slideOutlineSchema()
	structure := map[string]any{
		"type": "object", "required": []any{"sections"},
		"properties": map[string]any{
			"auto_agenda": map[string]any{"type": "boolean"},
			"sections":    map[string]any{"type": "array", "minItems": 1},
		},
	}
	return map[string]any{
		"type": "object",
		"oneOf": []any{
			map[string]any{"required": []any{"slides"}, "not": map[string]any{"required": []any{"structure"}}},
			map[string]any{"required": []any{"structure"}, "not": map[string]any{"required": []any{"slides"}}},
		},
		"properties": map[string]any{
			"meta": meta,
			"slides": map[string]any{
				"type": "array", "minItems": 1, "items": slide,
			},
			"structure": structure,
		},
	}
}

func slideOutlineSchema() map[string]any {
	return slideKindShapeSchema(true)
}

func slideKindShapeSchema(withEnum bool) map[string]any {
	kinds := AllSlideKinds()
	enum := make([]any, 0, len(kinds))
	for _, k := range kinds {
		enum = append(enum, string(k))
	}
	kind := map[string]any{"type": "string"}
	if withEnum {
		kind["enum"] = enum
	}
	return map[string]any{
		"type":     "object",
		"required": []any{"kind"},
		"properties": map[string]any{
			"kind": kind,
		},
	}
}

// stripAnnotations drops annotation keywords from schema objects. inProps
// marks a "properties" map, whose keys are field names (a field may itself be
// called "title" or "description") and must be kept.
func stripAnnotations(v any, inProps bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			if !inProps {
				switch k {
				case "description", "title", "examples", "$comment":
					continue
				}
			}
			out[k] = stripAnnotations(e, !inProps && (k == "properties" || k == "patternProperties"))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = stripAnnotations(e, false)
		}
		return out
	default:
		return v
	}
}

func inlineRefs(v any, defs map[string]any) any {
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
			if def, ok := defs[strings.TrimPrefix(ref, "#/$defs/")]; ok {
				return inlineRefs(def, defs)
			}
		}
		out := make(map[string]any, len(t))
		for k, e := range t {
			if k == "$defs" {
				continue
			}
			out[k] = inlineRefs(e, defs)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = inlineRefs(e, defs)
		}
		return out
	default:
		return v
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

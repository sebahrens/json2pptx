package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// get_input_schema — returns the authoritative JSON Schema for PresentationInput
// ---------------------------------------------------------------------------

// inputSchemaResponse is the JSON envelope for get_input_schema.
type inputSchemaResponse struct {
	Digest      string         `json:"digest"`
	NotModified bool           `json:"not_modified,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

func mcpGetInputSchemaTool() mcp.Tool {
	return mcp.NewTool("get_input_schema",
		mcp.WithDescription(`Returns the authoritative JSON Schema for the PresentationInput object accepted by generate_presentation and validate_input.

Includes all nested types (SlideInput, ContentInput, ShapeGridInput, PatternInput, etc.) as $defs, with inline enum values and x-field-scope annotations (deck, slide, content, shape) indicating where each field belongs in the hierarchy.

Use this to discover field names, types, allowed values, and correct nesting — eliminates field-scope confusion (e.g., putting contrast_check on content instead of slide).

Supports digest-based caching: pass a previous digest to get a not_modified response when the schema hasn't changed.`),
		mcp.WithRawOutputSchema(outputSchemaGetInputSchema),
		mcp.WithString("digest",
			mcp.Description("Digest from a previous get_input_schema response. If it matches the current schema, a not_modified response is returned instead of the full schema."),
		),
	)
}

func handleGetInputSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := buildInputSchema()
	digest := computeInputSchemaDigest(schema)

	// If the caller already has this digest, return a short not_modified response.
	if d, err := request.RequireString("digest"); err == nil && d == digest {
		resp := inputSchemaResponse{
			Digest:      digest,
			NotModified: true,
		}
		mcpResult, err := api.MCPSuccessResult(ctx, resp)
		if err != nil {
			return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
		}
		return mcpResult, nil
	}

	resp := inputSchemaResponse{
		Digest: digest,
		Schema: schema,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// computeInputSchemaDigest returns a short hex digest of the serialized schema.
func computeInputSchemaDigest(schema map[string]any) string {
	data, _ := json.Marshal(schema)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:16]
}

// ---------------------------------------------------------------------------
// Schema builder — reflection-based with field_scope and enum annotations
// ---------------------------------------------------------------------------

// fieldScopeMap defines which struct fields belong to which scope level.
// Only non-obvious or top-level fields need explicit annotation; nested
// structs inherit their parent's scope by context.
var fieldScopeMap = map[string]map[string]string{
	"PresentationInput": {
		"template":        "deck",
		"output_filename": "deck",
		"design_mode":     "deck",
		"accent_strategy": "deck",
		"footer":          "deck",
		"chrome":          "deck",
		"theme_override":  "deck",
		"defaults":        "deck",
		"grid":            "deck",
		"structure":       "deck",
		"slides":          "deck",
	},
	"SlideInput": {
		"layout_id":         "slide",
		"slide_type":        "slide",
		"eyebrow":           "slide",
		"background":        "slide",
		"content":           "slide",
		"shape_grid":        "slide",
		"pattern":           "slide",
		"compose":           "slide",
		"speaker_notes":     "slide",
		"source":            "slide",
		"takeaway":          "slide",
		"transition":        "slide",
		"transition_speed":  "slide",
		"build":             "slide",
		"contrast_check":    "slide",
	},
	"SplitSlideInput": {
		"type":  "slide",
		"base":  "slide",
		"split": "slide",
	},
	"SplitConfig": {
		"by":             "split",
		"group_size":     "split",
		"title_suffix":   "split",
		"repeat_headers": "split",
	},
	"ContentInput": {
		"placeholder_id":         "content",
		"type":                   "content",
		"value":                  "content",
		"text_value":             "content",
		"bullets_value":          "content",
		"body_and_bullets_value": "content",
		"body_and_lead_value":    "content",
		"bullet_groups_value":    "content",
		"table_value":            "content",
		"chart_value":            "content",
		"diagram_value":          "content",
		"image_value":            "content",
		"font_size":              "content",
	},
	"ShapeSpecInput": {
		"geometry":     "shape",
		"fill":         "shape",
		"line":         "shape",
		"text":         "shape",
		"rotation":     "shape",
		"adjustments":  "shape",
	},
}

// enumMap defines inline enum values for specific struct fields.
//
// Slide- and deck-level field vocabularies come from enums.go (the canonical
// source consumed by the validator, capabilities, and docs). Schema-only
// vocabularies (connector style/dash, compose direction, split_slide type
// and by) stay inline because they describe shapes peculiar to the JSON
// envelope, not slide-level enums published across surfaces.
var enumMap = map[string]map[string][]string{
	"PresentationInput": {
		"design_mode":     canonicalDesignModes,
		"accent_strategy": canonicalAccentStrategies,
	},
	"SlideInput": {
		"slide_type":       canonicalSlideTypes,
		"transition":       canonicalTransitions(),
		"transition_speed": canonicalTransitionSpeeds,
		"build":            canonicalBuilds,
	},
	"ContentInput": {
		// Content types include "chart" because the contentTypeDiscriminator
		// allOf branches pair type:"chart" with chart_value; the engine still
		// accepts that branch. Capabilities advertises the slimmer
		// generator.AllContentTypes() — alignment of these two surfaces is
		// tracked as a separate task.
		"type": {"text", "bullets", "body_and_bullets", "body_and_lead", "bullet_groups", "table", "chart", "diagram", "image"},
	},
	"BackgroundInput": {
		"fit": canonicalBackgroundFits,
	},
	"ConnectorSpecInput": {
		"style": {"arrow", "line"},
		"dash":  {"solid", "dash", "dot", "lgDash", "dashDot"},
	},
	"ComposeInput": {
		"direction": {"vertical", "horizontal"},
	},
	"SplitSlideInput": {
		"type": {"split_slide"},
	},
	"SplitConfig": {
		"by": {"table.rows"},
	},
}

// propertyOverrides allows specific (typeName, jsonName) pairs to replace the
// reflection-derived property schema wholesale. Use for fields that need
// discriminator-style oneOf branches or other shapes reflection cannot infer.
var propertyOverrides = map[string]map[string]map[string]any{
	"PresentationInput": {
		"slides": {
			"type":        "array",
			"description": "Ordered list of slides. Each item is either a regular SlideInput or a SplitSlideInput envelope (discriminated by the optional \"type\": \"split_slide\" field). split_slide expands at parse time into N regular slides by windowing a base slide's table rows across pages.",
			"items": map[string]any{
				"oneOf": []any{
					map[string]any{
						"title":       "split_slide",
						"description": "Declarative envelope that windows a table-bearing slide across N pages. Discriminator: type == \"split_slide\".",
						"$ref":        "#/$defs/SplitSlideInput",
					},
					map[string]any{
						"title":       "regular_slide",
						"description": "Standard slide. Must NOT set type to \"split_slide\".",
						"$ref":        "#/$defs/SlideInput",
					},
				},
			},
		},
	},
}

// typeOverrides applies whole-object schema additions to specific $defs
// entries (descriptions, type-level anyOf/allOf/oneOf constraints). These
// express semantics that reflection cannot infer, like required-alternative
// groups or mutually exclusive branches.
var typeOverrides = map[string]map[string]any{
	"SlideInput": {
		"description": "A regular slide. For table-windowing slides, use the SplitSlideInput branch of slides[] instead.\n\nAlternatives:\n- layout_id (pins a specific template layout) OR slide_type (hint for auto-selection) — at least one is required.\n- pattern, shape_grid, compose — at most one of these visual-envelope fields may be set (mutually exclusive). content[] may coexist with any of them or stand alone.",
		"anyOf": []any{
			map[string]any{"required": []any{"layout_id"}},
			map[string]any{"required": []any{"slide_type"}},
		},
		"allOf": []any{
			map[string]any{"not": map[string]any{"required": []any{"pattern", "shape_grid"}}},
			map[string]any{"not": map[string]any{"required": []any{"pattern", "compose"}}},
			map[string]any{"not": map[string]any{"required": []any{"shape_grid", "compose"}}},
		},
	},
	"SplitSlideInput": {
		"description": "Declarative envelope that expands into N regular slides by windowing the base slide's table rows. Only \"table.rows\" splitting is supported. Use this for the one painful case — large tables that need identical chrome (title, headers, footer) repeated on each page. For heterogeneous multi-slide narratives, author sibling SlideInput entries directly.",
	},
	"SplitConfig": {
		"description": "Configures how a split_slide base table is windowed across pages.",
	},
	"ContentInput": contentInputOverride(),
}

// contentTypeDiscriminator pairs each ContentInput "type" enum value with the
// typed *_value field that must accompany it. Keep these aligned with the
// ContentInput struct definition in json_schema.go and the enum in enumMap.
var contentTypeDiscriminator = []struct {
	typeName string
	valueKey string
}{
	{"text", "text_value"},
	{"bullets", "bullets_value"},
	{"body_and_bullets", "body_and_bullets_value"},
	{"body_and_lead", "body_and_lead_value"},
	{"bullet_groups", "bullet_groups_value"},
	{"table", "table_value"},
	{"chart", "chart_value"},
	{"diagram", "diagram_value"},
	{"image", "image_value"},
}

// contentInputOverride builds the type-level schema constraints that encode
// the ContentInput discriminator rules: for each "type" value, the matching
// typed *_value field is required and the other typed *_value fields are
// forbidden. The legacy "value" (json.RawMessage) field is intentionally
// left unconstrained for backward compatibility.
func contentInputOverride() map[string]any {
	allValueKeys := make([]string, 0, len(contentTypeDiscriminator))
	for _, e := range contentTypeDiscriminator {
		allValueKeys = append(allValueKeys, e.valueKey)
	}

	branches := make([]any, 0, len(contentTypeDiscriminator))
	for _, e := range contentTypeDiscriminator {
		forbidden := map[string]any{}
		for _, k := range allValueKeys {
			if k == e.valueKey {
				continue
			}
			// JSON Schema draft 2020-12: a property schema of `false`
			// rejects any value, which forbids the property entirely
			// when it is present in the instance.
			forbidden[k] = false
		}
		branch := map[string]any{
			"if": map[string]any{
				"properties": map[string]any{
					"type": map[string]any{"const": e.typeName},
				},
				"required": []any{"type"},
			},
			"then": map[string]any{
				"required":   []any{e.valueKey},
				"properties": forbidden,
			},
		}
		branches = append(branches, branch)
	}
	return map[string]any{
		"description": "Discriminated content item. The \"type\" field selects which typed *_value field must be set: type \"text\" requires text_value (and forbids bullets_value, table_value, etc.); type \"bullets\" requires bullets_value; and so on for body_and_bullets, body_and_lead, bullet_groups, table, chart, diagram, image. The legacy \"value\" (raw JSON) field is still accepted for backward compatibility and is unconstrained by the discriminator.",
		"allOf":       branches,
	}
}

// buildInputSchema builds a complete JSON Schema for PresentationInput
// with all nested types as $defs.
func buildInputSchema() map[string]any {
	defs := map[string]any{}

	// Register all types that should appear as $defs.
	typeRegistry := []struct {
		name string
		typ  reflect.Type
	}{
		{"PresentationInput", reflect.TypeOf(PresentationInput{})},
		{"SlideInput", reflect.TypeOf(SlideInput{})},
		{"SplitSlideInput", reflect.TypeOf(SplitSlideInput{})},
		{"SplitConfig", reflect.TypeOf(SplitConfig{})},
		{"ContentInput", reflect.TypeOf(ContentInput{})},
		{"BackgroundInput", reflect.TypeOf(BackgroundInput{})},
		{"ChromeInput", reflect.TypeOf(ChromeInput{})},
		{"PageNumbersInput", reflect.TypeOf(PageNumbersInput{})},
		{"StructureInput", reflect.TypeOf(StructureInput{})},
		{"SectionInput", reflect.TypeOf(SectionInput{})},
		{"DefaultsInput", reflect.TypeOf(DefaultsInput{})},
		{"GridConfig", reflect.TypeOf(GridConfig{})},
		{"ThemeInput", reflect.TypeOf(ThemeInput{})},
		{"JSONFooter", reflect.TypeOf(JSONFooter{})},
		{"PatternInput", reflect.TypeOf(PatternInput{})},
		{"ComposeInput", reflect.TypeOf(ComposeInput{})},
		{"SegmentInput", reflect.TypeOf(SegmentInput{})},
		{"BodyAndBulletsInput", reflect.TypeOf(BodyAndBulletsInput{})},
		{"BodyAndLeadInput", reflect.TypeOf(BodyAndLeadInput{})},
		{"BulletGroupsInput", reflect.TypeOf(BulletGroupsInput{})},
		{"BulletGroupInput", reflect.TypeOf(BulletGroupInput{})},
		{"ImageInput", reflect.TypeOf(ImageInput{})},
		{"ShapeGridInput", reflect.TypeOf(jsonschema.ShapeGridInput{})},
		{"GridBoundsInput", reflect.TypeOf(jsonschema.GridBoundsInput{})},
		{"GridRowInput", reflect.TypeOf(jsonschema.GridRowInput{})},
		{"GridCellInput", reflect.TypeOf(jsonschema.GridCellInput{})},
		{"ConnectorSpecInput", reflect.TypeOf(jsonschema.ConnectorSpecInput{})},
		{"AccentBarInput", reflect.TypeOf(jsonschema.AccentBarInput{})},
		{"GridImageInput", reflect.TypeOf(jsonschema.GridImageInput{})},
		{"GridOverlayInput", reflect.TypeOf(jsonschema.GridOverlayInput{})},
		{"GridImageTextInput", reflect.TypeOf(jsonschema.GridImageTextInput{})},
		{"IconInput", reflect.TypeOf(jsonschema.IconInput{})},
		{"ShapeSpecInput", reflect.TypeOf(jsonschema.ShapeSpecInput{})},
		{"ShapeFillInput", reflect.TypeOf(jsonschema.ShapeFillInput{})},
		{"TableInput", reflect.TypeOf(jsonschema.TableInput{})},
		{"TableCellInput", reflect.TypeOf(jsonschema.TableCellInput{})},
		{"TableStyleInput", reflect.TypeOf(jsonschema.TableStyleInput{})},
		{"PatternCallout", reflect.TypeOf(patterns.PatternCallout{})},
		{"BannerSpec", reflect.TypeOf(patterns.BannerSpec{})},
		{"ChartSpec", reflect.TypeOf(types.ChartSpec{})},  //nolint:staticcheck // ChartSpec is deprecated but still part of the input contract
		{"DiagramSpec", reflect.TypeOf(types.DiagramSpec{})},
	}

	for _, entry := range typeRegistry {
		defs[entry.name] = reflectStructSchema(entry.name, entry.typ)
	}

	// Apply type-level overrides (descriptions, anyOf/allOf/oneOf constraints).
	for typeName, overrides := range typeOverrides {
		def, ok := defs[typeName].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range overrides {
			def[k] = v
		}
	}

	// Root schema references PresentationInput.
	return map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "PresentationInput",
		"description": "JSON input schema for generate_presentation and validate_input. Field annotations include x-field-scope (deck/slide/content/shape) and enum values.",
		"$ref":        "#/$defs/PresentationInput",
		"$defs":       defs,
	}
}

// reflectStructSchema generates a JSON Schema object for a Go struct type.
func reflectStructSchema(typeName string, t reflect.Type) map[string]any {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return map[string]any{"type": "object"}
	}

	properties := map[string]any{}
	var required []string

	scopes := fieldScopeMap[typeName]
	enums := enumMap[typeName]
	overrides := propertyOverrides[typeName]

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		parts := strings.Split(tag, ",")
		jsonName := parts[0]
		if jsonName == "" {
			continue
		}

		isOmitempty := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				isOmitempty = true
			}
		}

		var prop map[string]any
		if overrides != nil {
			if ov, ok := overrides[jsonName]; ok {
				// Clone to avoid mutating the shared override map.
				prop = make(map[string]any, len(ov))
				for k, v := range ov {
					prop[k] = v
				}
			}
		}
		if prop == nil {
			prop = reflectFieldSchema(f.Type)
		}

		// Add field_scope annotation (preserve override if it set one).
		if scopes != nil {
			if scope, ok := scopes[jsonName]; ok {
				if _, exists := prop["x-field-scope"]; !exists {
					prop["x-field-scope"] = scope
				}
			}
		}

		// Add enum annotation.
		if enums != nil {
			if vals, ok := enums[jsonName]; ok {
				if _, exists := prop["enum"]; !exists {
					prop["enum"] = vals
				}
			}
		}

		// Add description from struct tag if available.
		if desc := extractDescription(f); desc != "" {
			if _, exists := prop["description"]; !exists {
				prop["description"] = desc
			}
		}

		properties[jsonName] = prop

		// Fields without omitempty are required.
		if !isOmitempty {
			required = append(required, jsonName)
		}
	}

	sort.Strings(required)

	result := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

// reflectFieldSchema maps a Go type to a JSON Schema type descriptor.
// For known struct types, it emits a $ref to $defs.
func reflectFieldSchema(t reflect.Type) map[string]any {
	// Unwrap pointer.
	isPtr := false
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		isPtr = true
	}

	// Special case: json.RawMessage is untyped JSON.
	if t == reflect.TypeOf(json.RawMessage{}) {
		return map[string]any{}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		if isPtr {
			return map[string]any{"type": "boolean"}
		}
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		items := reflectFieldSchema(t.Elem())
		return map[string]any{
			"type":  "array",
			"items": items,
		}
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			values := reflectFieldSchema(t.Elem())
			return map[string]any{
				"type":                 "object",
				"additionalProperties": values,
			}
		}
		return map[string]any{"type": "object"}
	case reflect.Struct:
		ref := knownTypeRef(t)
		if ref != "" {
			return map[string]any{"$ref": ref}
		}
		// Inline unknown structs.
		return map[string]any{"type": "object"}
	case reflect.Interface:
		return map[string]any{}
	default:
		return map[string]any{}
	}
}

// knownTypeRef returns a $ref path if the type matches a registered $defs entry.
var knownTypeRefMap = map[reflect.Type]string{
	reflect.TypeOf(PresentationInput{}):          "#/$defs/PresentationInput",
	reflect.TypeOf(SlideInput{}):                 "#/$defs/SlideInput",
	reflect.TypeOf(SplitSlideInput{}):            "#/$defs/SplitSlideInput",
	reflect.TypeOf(SplitConfig{}):                "#/$defs/SplitConfig",
	reflect.TypeOf(ContentInput{}):               "#/$defs/ContentInput",
	reflect.TypeOf(BackgroundInput{}):            "#/$defs/BackgroundInput",
	reflect.TypeOf(ChromeInput{}):                "#/$defs/ChromeInput",
	reflect.TypeOf(PageNumbersInput{}):           "#/$defs/PageNumbersInput",
	reflect.TypeOf(StructureInput{}):             "#/$defs/StructureInput",
	reflect.TypeOf(SectionInput{}):               "#/$defs/SectionInput",
	reflect.TypeOf(DefaultsInput{}):              "#/$defs/DefaultsInput",
	reflect.TypeOf(GridConfig{}):                 "#/$defs/GridConfig",
	reflect.TypeOf(ThemeInput{}):                 "#/$defs/ThemeInput",
	reflect.TypeOf(JSONFooter{}):                 "#/$defs/JSONFooter",
	reflect.TypeOf(PatternInput{}):               "#/$defs/PatternInput",
	reflect.TypeOf(ComposeInput{}):               "#/$defs/ComposeInput",
	reflect.TypeOf(SegmentInput{}):               "#/$defs/SegmentInput",
	reflect.TypeOf(BodyAndBulletsInput{}):        "#/$defs/BodyAndBulletsInput",
	reflect.TypeOf(BodyAndLeadInput{}):           "#/$defs/BodyAndLeadInput",
	reflect.TypeOf(BulletGroupsInput{}):          "#/$defs/BulletGroupsInput",
	reflect.TypeOf(BulletGroupInput{}):           "#/$defs/BulletGroupInput",
	reflect.TypeOf(ImageInput{}):                 "#/$defs/ImageInput",
	reflect.TypeOf(jsonschema.ShapeGridInput{}):   "#/$defs/ShapeGridInput",
	reflect.TypeOf(jsonschema.GridBoundsInput{}):  "#/$defs/GridBoundsInput",
	reflect.TypeOf(jsonschema.GridRowInput{}):     "#/$defs/GridRowInput",
	reflect.TypeOf(jsonschema.GridCellInput{}):    "#/$defs/GridCellInput",
	reflect.TypeOf(jsonschema.ConnectorSpecInput{}): "#/$defs/ConnectorSpecInput",
	reflect.TypeOf(jsonschema.AccentBarInput{}):   "#/$defs/AccentBarInput",
	reflect.TypeOf(jsonschema.GridImageInput{}):   "#/$defs/GridImageInput",
	reflect.TypeOf(jsonschema.GridOverlayInput{}): "#/$defs/GridOverlayInput",
	reflect.TypeOf(jsonschema.GridImageTextInput{}): "#/$defs/GridImageTextInput",
	reflect.TypeOf(jsonschema.IconInput{}):        "#/$defs/IconInput",
	reflect.TypeOf(jsonschema.ShapeSpecInput{}):   "#/$defs/ShapeSpecInput",
	reflect.TypeOf(jsonschema.ShapeFillInput{}):   "#/$defs/ShapeFillInput",
	reflect.TypeOf(jsonschema.TableInput{}):       "#/$defs/TableInput",
	reflect.TypeOf(jsonschema.TableCellInput{}):   "#/$defs/TableCellInput",
	reflect.TypeOf(jsonschema.TableStyleInput{}):  "#/$defs/TableStyleInput",
	reflect.TypeOf(patterns.PatternCallout{}):      "#/$defs/PatternCallout",
	reflect.TypeOf(patterns.BannerSpec{}):          "#/$defs/BannerSpec",
	reflect.TypeOf(types.ChartSpec{}):             "#/$defs/ChartSpec",  //nolint:staticcheck
	reflect.TypeOf(types.DiagramSpec{}):           "#/$defs/DiagramSpec",
}

func knownTypeRef(t reflect.Type) string {
	return knownTypeRefMap[t]
}

// extractDescription returns a description from the Go struct field comment
// embedded in the struct tag (if any). Since Go struct tags don't carry comments
// at runtime, we return an empty string. The doc is instead conveyed by the
// field_scope and enum annotations.
func extractDescription(_ reflect.StructField) string {
	return ""
}

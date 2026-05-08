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
		"transition":        "slide",
		"transition_speed":  "slide",
		"build":             "slide",
		"contrast_check":    "slide",
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
var enumMap = map[string]map[string][]string{
	"PresentationInput": {
		"design_mode":     {"constrained", "free"},
		"accent_strategy": {"primary", "rotate", "section-keyed"},
	},
	"SlideInput": {
		"slide_type":       {"title", "content", "section", "two-column", "blank", "chart", "diagram", "image", "comparison", "closing"},
		"transition":       {"fade", "push", "wipe", "split", "cover", "uncover", "reveal", "none"},
		"transition_speed": {"slow", "medium", "fast"},
		"build":            {"bullets"},
	},
	"ContentInput": {
		"type": {"text", "bullets", "body_and_bullets", "body_and_lead", "bullet_groups", "table", "chart", "diagram", "image"},
	},
	"BackgroundInput": {
		"fit": {"cover", "stretch", "tile"},
	},
	"ConnectorSpecInput": {
		"style": {"arrow", "line"},
		"dash":  {"solid", "dash", "dot", "lgDash", "dashDot"},
	},
	"ComposeInput": {
		"direction": {"vertical", "horizontal"},
	},
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
		{"ChartSpec", reflect.TypeOf(types.ChartSpec{})},  //nolint:staticcheck // ChartSpec is deprecated but still part of the input contract
		{"DiagramSpec", reflect.TypeOf(types.DiagramSpec{})},
	}

	for _, entry := range typeRegistry {
		defs[entry.name] = reflectStructSchema(entry.name, entry.typ)
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

		prop := reflectFieldSchema(f.Type)

		// Add field_scope annotation.
		if scopes != nil {
			if scope, ok := scopes[jsonName]; ok {
				prop["x-field-scope"] = scope
			}
		}

		// Add enum annotation.
		if enums != nil {
			if vals, ok := enums[jsonName]; ok {
				prop["enum"] = vals
			}
		}

		// Add description from struct tag if available.
		if desc := extractDescription(f); desc != "" {
			prop["description"] = desc
		}

		properties[jsonName] = prop

		// Fields without omitempty are required.
		if !isOmitempty {
			required = append(required, jsonName)
		}
	}

	sort.Strings(required)

	result := map[string]any{
		"type":       "object",
		"properties": properties,
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

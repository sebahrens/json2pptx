package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Compose recipes (go-slide-creator-okg00).
//
// A compose candidate used to carry only placement.composable_with, so the
// top-ranked composition was harder to act on than a lower-ranked chart with
// a runnable recipe. attachComposeRecipe turns a candidate's Composition —
// the single region representation shared by pattern pairs, heterogeneous
// chart / KPI / diagram compositions and shortlist round-trips — into:
//
//   - next_tool_call: render_deck_spec with a complete DeckSpec whose one
//     raw_json2pptx slide uses the canonical "blank-title" layout, a title,
//     and compose.segments built from the regions (direction, size_pct,
//     nested envelopes) with sample content in every leaf;
//   - per region: segment_path and a data_contract (chart / diagram data
//     keys, or pattern values keys) whose field_path points at the sample
//     content to replace;
//   - composition.instructions: how to adopt the slide in an existing
//     DeckSpec without converting other slides to raw.
//
// Only default-profile tools are needed: validate_deck_spec and
// render_deck_spec accept the spec as is.

// composeRecipeLayoutID is the canonical layout a compose recipe renders on:
// Blank + Title gives the composition the full content area under a title.
const composeRecipeLayoutID = "blank-title"

// composeRecipeSlidePath is where the recipe's slide sits in the spec.
const composeRecipeSlidePath = "slides[0].slide"

// attachComposeRecipe fills a compose candidate's region contracts and its
// runnable render_deck_spec call. It leaves the candidate untouched when a
// region has no sample content (an unknown pattern / type).
func attachComposeRecipe(c *patterns.VisualCandidate, templateName string, reg *patterns.Registry, hints map[string]skillDataFormat) {
	if c.Category != patterns.VisualCategoryCompose || c.Composition == nil {
		return
	}
	comp := c.Composition
	compose, hasData, err := composeRecipeEnvelope(comp, composeRecipeSlidePath+".compose", reg, hints)
	if err != nil {
		return
	}
	if templateName == "" {
		templateName = visualRecipeTemplate
	}
	label := composeRecipeLabel(comp)
	slide := map[string]any{
		"layout_id": composeRecipeLayoutID,
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Replace with the action title this " + label + " supports"},
		},
		"compose": compose,
	}
	if hasData {
		slide["source"] = "Illustrative sample data; replace with the real source"
	}
	c.NextToolCall = &patterns.ToolCallSuggestion{
		Tool: "render_deck_spec",
		ArgsTemplate: map[string]any{"spec": map[string]any{
			"meta":   map[string]any{"title": "Recommended " + label, "template": templateName},
			"slides": []any{map[string]any{"kind": "raw_json2pptx", "slide": slide}},
		}},
	}
	comp.Instructions = "Copy next_tool_call.args_template.spec.slides[0] into your DeckSpec slides as one raw_json2pptx slide; the other slides keep their semantic kinds. " +
		"Replace the title at " + composeRecipeSlidePath + ".content[0].text_value and each region's sample content at its data_contract.field_path with real content, " +
		"keeping compose.direction, every segment's size_pct, the nesting and any pattern overrides. Then call validate_deck_spec and render_deck_spec."
}

// composeRecipeEnvelope builds one compose envelope (direction + segments)
// from a composition, filling each region's segment_path / data_contract.
// hasData reports
// a region showing figures (chart, diagram, KPI), which needs a source line.
func composeRecipeEnvelope(comp *patterns.VisualComposition, path string, reg *patterns.Registry, hints map[string]skillDataFormat) (envelope map[string]any, hasData bool, err error) {
	segments := make([]any, 0, len(comp.Regions))
	for i := range comp.Regions {
		r := &comp.Regions[i]
		segPath := fmt.Sprintf("%s.segments[%d]", path, i)
		r.SegmentPath = segPath
		seg := map[string]any{}
		if r.SizePct > 0 {
			seg["size_pct"] = r.SizePct
		}
		switch r.Category {
		case patterns.VisualCategoryCompose:
			if r.Compose == nil {
				return nil, false, fmt.Errorf("region %s has no nested composition", segPath)
			}
			inner, innerData, err := composeRecipeEnvelope(r.Compose, segPath+".compose", reg, hints)
			if err != nil {
				return nil, false, err
			}
			hasData = hasData || innerData
			seg["compose"] = inner
		case patterns.VisualCategoryPattern:
			values, contract, err := patternRegionRecipe(reg, r.Name)
			if err != nil {
				return nil, false, err
			}
			contract.FieldPath = segPath + ".pattern.values"
			r.DataContract = contract
			// No size overrides: a pattern sizes itself to its segment
			// (stat-hero's figure included, go-slide-creator-hidji).
			seg["pattern"] = map[string]any{"name": r.Name, "values": values}
			if pat, ok := reg.Get(r.Name); ok && (pat.Taxonomy().Category == "data-display" || r.Name == "stat-hero") {
				hasData = true
			}
		case patterns.VisualCategoryChart, patterns.VisualCategoryDiagram:
			data, ok := visualRecipeData[r.Name]
			if !ok {
				return nil, false, fmt.Errorf("no sample data for %s %q", r.Category, r.Name)
			}
			hasData = true
			label := strings.ReplaceAll(r.Name, "_", " ")
			seg["diagram"] = map[string]any{
				"type": r.Name,
				"data": deepCopyAny(data),
				"alt":  "Replace with one sentence saying what this " + label + " shows",
			}
			if h, ok := hints[r.Name]; ok {
				r.DataContract = &patterns.VisualDataContract{
					RequiredKeys: h.RequiredKeys,
					OptionalKeys: h.OptionalKeys,
					Description:  h.Description,
					FieldPath:    segPath + ".diagram.data",
				}
			}
		default:
			return nil, false, fmt.Errorf("region %s: category %q cannot be a compose segment", segPath, r.Category)
		}
		segments = append(segments, seg)
	}
	return map[string]any{"direction": comp.Direction, "segments": segments}, hasData, nil
}

// patternRegionRecipe returns a pattern's exemplar values (JSON-shaped, ready
// to embed) and its values contract from the pattern's schema.
func patternRegionRecipe(reg *patterns.Registry, name string) (any, *patterns.VisualDataContract, error) {
	if reg == nil {
		return nil, nil, fmt.Errorf("no registry")
	}
	pat, ok := reg.Get(name)
	if !ok {
		return nil, nil, fmt.Errorf("unknown pattern %q", name)
	}
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		return nil, nil, fmt.Errorf("pattern %q has no exemplar values", name)
	}
	raw, err := json.Marshal(ex.ExemplarValues())
	if err != nil {
		return nil, nil, err
	}
	var values any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, nil, err
	}
	return values, patternValuesContract(pat), nil
}

// patternValuesContract describes a pattern's values: the keys of the values
// object, or of each item when values is an array.
func patternValuesContract(pat patterns.Pattern) *patterns.VisualDataContract {
	schema := pat.Schema()
	values := schema.Property("values").Deref(schema)
	shape := "values object"
	keysOf := values
	if values.TypeName() == "array" {
		shape = "values array; each item"
		keysOf = values.ItemSchema().Deref(schema)
	}
	// A tolerated shorthand ("$4.2M | ARR") sits in a oneOf beside the
	// object form; the contract describes the object form.
	if len(keysOf.PropertyNames()) == 0 {
		for _, branch := range keysOf.OneOfBranches() {
			if b := branch.Deref(schema); len(b.PropertyNames()) > 0 {
				keysOf = b
				break
			}
		}
	}
	required := keysOf.RequiredNames()
	sort.Strings(required)
	var optional []string
	for _, k := range keysOf.PropertyNames() {
		if !keysOf.IsRequired(k) {
			optional = append(optional, k)
		}
	}
	if required == nil {
		required = []string{}
	}
	desc := fmt.Sprintf("%s pattern %s takes the keys listed (cells: %s). %s", pat.Name(), shape, pat.CellsHint(), pat.UseWhen())
	return &patterns.VisualDataContract{
		RequiredKeys: required,
		OptionalKeys: optional,
		Description:  strings.TrimSpace(desc),
	}
}

// composeRecipeLabel names a composition for the sample title, e.g.
// "line + stat-hero + timeline composition".
func composeRecipeLabel(comp *patterns.VisualComposition) string {
	var names []string
	for _, l := range comp.Leaves() {
		names = append(names, strings.ReplaceAll(l.Name, "_", " "))
	}
	return strings.Join(names, " + ") + " composition"
}

package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// arch-stack pattern — labeled tiers with optional cross-cutting side rails
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&archStack{})
}

type archStack struct{}

func (a *archStack) Name() string { return "arch-stack" }
func (a *archStack) Description() string {
	return "Architecture stack diagram: 3-6 tier bands, each with one block per component (or a line of detail), and optional cross-cutting side rails"
}
func (a *archStack) UseWhen() string {
	return "Architecture layers or technology stack with vertical ordering; prefer pyramid when the hierarchy narrows visually, process-flow when layers have sequential flow"
}
func (a *archStack) NotWhen() string {
	return "Hierarchy narrows top-to-bottom like Maslow (use pyramid), or layers are sequential steps (use process-flow)"
}
func (a *archStack) Version() int      { return 1 }
func (a *archStack) CellsHint() string { return "3-6 tiers (1-12 components each) + rails" }
func (a *archStack) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"card-grid", "process-flow", "swimlane"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (a *archStack) SupportsCallout() bool        { return true }
func (a *archStack) SupportsInlineMarkdown() bool { return true }

func (a *archStack) ExemplarValues() any {
	return &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Channels", Components: []string{"Web", "Mobile app", "Partner API"}},
			{Label: "Experience", Components: []string{"API gateway", "Identity"}},
			{Label: "Domain services", Components: []string{"Orders", "Pricing", "Inventory", "Billing"}},
			{Label: "Data platform", Components: []string{"Operational DB", "Event stream", "Lakehouse"}},
		},
		SideRails: []string{"Security", "Observability"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// archStackTierBarPt is the accent bar on each tier's left edge.
const archStackTierBarPt = 3

// ArchStackTier represents one horizontal layer in the architecture stack.
type ArchStackTier struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	// Components are the tier's building blocks, drawn as one block each
	// inside the tier band (archstack_components.go). A tier has components
	// or a description, not both.
	Components []string `json:"components,omitempty"`
}

// ArchStackValues holds tiers (top to bottom) and optional side rails.
type ArchStackValues struct {
	Tiers     []ArchStackTier `json:"tiers"`
	SideRails []string        `json:"side_rails,omitempty"` // Cross-cutting concerns shown as vertical bars
}

// ArchStackOverrides is the standard text overrides.
type ArchStackOverrides = TextOverrides

// ArchStackCellOverride is the shared per-cell override.
type ArchStackCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *archStack) NewValues() any       { return &ArchStackValues{} }
func (a *archStack) NewOverrides() any    { return &ArchStackOverrides{} }
func (a *archStack) NewCellOverride() any { return &ArchStackCellOverride{} }

// archStackDescriptionBudgets returns the worded and unbroken-word budgets
// for every tier description, measured against the written size (no run
// stored below its role floor) on every shipped template with all tiers
// populated and every shape keeping the uniform 0.5 cm text margin
// (go-slide-creator-n1muf). Six tiers hold a label and no description.
func archStackDescriptionBudgets(tiers, _ int) (words, wide int) {
	switch {
	case tiers <= 4:
		return 120, 120
	case tiers == 5:
		return 40, 40
	default:
		return 0, 0
	}
}

// archStackRailWideBudget is the widest unbroken run a side rail holds.
const archStackRailWideBudget = 26

func (a *archStack) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ArchStackValues)
	if !ok || v == nil {
		return nil
	}
	if archStackHasComponents(v) {
		// The component layout sizes every band to its content; what can
		// still be written shrunk is a block's own name.
		ovr, _ := overrides.(*ArchStackOverrides)
		if ovr == nil {
			ovr = &ArchStackOverrides{}
		}
		return archStackComponentWarnings(ctx, v, ovr)
	}
	words, wide := archStackDescriptionBudgets(len(v.Tiers), len(v.SideRails))
	var warnings []string
	for i, tier := range v.Tiers {
		longest := 0
		for _, word := range strings.Fields(tier.Description) {
			longest = max(longest, runeLen(word))
		}
		switch {
		case words == 0 && strings.TrimSpace(tier.Description) != "":
			warnings = append(warnings, fmt.Sprintf("%s: arch-stack tiers[%d].description is %d characters; %d tiers hold a label and no readable description — drop the descriptions or use five tiers or fewer", ErrCodeBodyTooLong, i, runeLen(tier.Description), len(v.Tiers)))
		case longest > wide:
			warnings = append(warnings, fmt.Sprintf("%s: arch-stack tiers[%d].description contains a %d-character unbroken word; %d tiers with %d rails hold about %d wide characters per description — add word breaks, shorten the copy, or drop a rail", ErrCodeBodyTooLong, i, longest, len(v.Tiers), len(v.SideRails), wide))
		case runeLen(tier.Description) > words:
			warnings = append(warnings, fmt.Sprintf("%s: arch-stack tiers[%d].description is %d characters; %d tiers with %d rails hold about %d readable characters per description — shorten the copy", ErrCodeBodyTooLong, i, runeLen(tier.Description), len(v.Tiers), len(v.SideRails), words))
		}
	}
	for i, rail := range v.SideRails {
		longest := 0
		for _, word := range strings.Fields(rail) {
			longest = max(longest, runeLen(word))
		}
		if longest > archStackRailWideBudget {
			warnings = append(warnings, fmt.Sprintf("%s: arch-stack side_rails[%d] contains a %d-character unbroken word; a rail holds about %d wide characters — add a word break or shorten it", ErrCodeBodyTooLong, i, longest, archStackRailWideBudget))
		}
	}
	return warnings
}

func (a *archStack) Schema() *Schema {
	tierSchema := ObjectSchema(
		map[string]*Schema{
			"label":       StringSchema(60).WithDescription("Tier/layer name"),
			"description": StringSchema(120).WithDescription("Technologies or details for this tier as one line of text; about 120 readable characters with 3-4 tiers, 40 with 5 tiers; 6 tiers hold no readable description (label only). Use components instead when the tier is a set of named parts"),
			"components":  ArraySchema(StringSchema(archStackComponentMaxLen), 1, archStackMaxComponents).WithDescription("The tier's parts, drawn as one block each inside the tier band beside its label (1-12; 7 or more wrap to two rows of blocks). Not together with description. When any tier has components the whole stack takes the band layout: labels on the left, a description-only tier shows its text in place of blocks"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"tiers":      ArraySchema(tierSchema, 3, 6).WithDescription("Architecture tiers, top to bottom"),
			"side_rails": ArraySchema(StringSchema(30), 0, 3).WithDescription("Cross-cutting concerns shown as vertical side bars (0-3); keep unbroken runs near 26 characters"),
		},
		[]string{"tiers"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Architecture stack diagram with tiers and optional side rails")
}

func (a *archStack) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ArchStackValues)
	if !ok || vals == nil {
		return fmt.Errorf("arch-stack: values must be *ArchStackValues, got %T", values)
	}

	const name = "arch-stack"
	var errs []error

	// Validate cell_accent_mode
	if overrides != nil {
		if ovr, ok := overrides.(*ArchStackOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Tiers) < 3 {
		errs = append(errs, errMinItems(name, "tiers", 3, len(vals.Tiers), ""))
	}
	if len(vals.Tiers) > 6 {
		errs = append(errs, errMaxItems(name, "tiers", 6, len(vals.Tiers), ""))
	}

	for i, tier := range vals.Tiers {
		path := fmt.Sprintf("tiers[%d].label", i)
		if tier.Label == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(tier.Label) > 60 {
			errs = append(errs, errMaxLength(name, path, 60, runeLen(tier.Label)))
		}
		if tier.Description != "" && runeLen(tier.Description) > 120 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("tiers[%d].description", i), 120, runeLen(tier.Description)))
		}
		compPath := fmt.Sprintf("tiers[%d].components", i)
		if len(tier.Components) > archStackMaxComponents {
			errs = append(errs, errMaxItems(name, compPath, archStackMaxComponents, len(tier.Components), "(hint: group related components, or split the tier in two)"))
		}
		if len(tier.Components) > 0 && strings.TrimSpace(tier.Description) != "" {
			errs = append(errs, newValidationError(name, fmt.Sprintf("tiers[%d].description", i), ErrCodeInvalidShape,
				fmt.Sprintf("arch-stack: tiers[%d] sets both components and description; a tier draws its components as blocks or its description as text — keep one", i), nil))
		}
		for j, c := range tier.Components {
			if strings.TrimSpace(c) == "" {
				errs = append(errs, errRequired(name, fmt.Sprintf("%s[%d]", compPath, j)))
			} else if runeLen(c) > archStackComponentMaxLen {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("%s[%d]", compPath, j), archStackComponentMaxLen, runeLen(c)))
			}
		}
	}

	if len(vals.SideRails) > 3 {
		errs = append(errs, errMaxItems(name, "side_rails", 3, len(vals.SideRails), ""))
	}
	for i, rail := range vals.SideRails {
		if rail == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("side_rails[%d]", i)))
		} else if runeLen(rail) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("side_rails[%d]", i), 30, runeLen(rail)))
		}
	}

	// Total cells: tiers + side rails
	totalCells := len(vals.Tiers) + len(vals.SideRails)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (a *archStack) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ArchStackValues)
	if !ok {
		return nil, fmt.Errorf("arch-stack: values must be *ArchStackValues, got %T", values)
	}
	ovr := &ArchStackOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ArchStackOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("arch-stack: overrides must be *ArchStackOverrides, got %T", overrides)
		}
	}

	if ovr == nil {
		ovr = &ArchStackOverrides{}
	}
	if archStackHasComponents(vals) {
		return a.expandComponents(ctx, vals, ovr, cellOverrides)
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)
	cellAccentMode := ovr.CellAccentMode

	hasSideRails := len(vals.SideRails) > 0
	numRails := len(vals.SideRails)

	// Grid layout: tier column + side rail columns
	// If side rails: [80%, rail1%, rail2%, ...]
	numCols := 1 + numRails
	cols := make([]float64, numCols)
	if hasSideRails {
		railWidth := archStackRailWidthPct
		cols[0] = 100 - float64(numRails)*railWidth
		for i := 1; i <= numRails; i++ {
			cols[i] = railWidth
		}
	} else {
		cols[0] = 100
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	var rows []jsonschema.GridRowInput

	// Tier rows
	for i, tier := range vals.Tiers {
		cells := make([]*jsonschema.GridCellInput, numCols)

		// Build tier text: label + optional description
		var text json.RawMessage
		if tier.Description != "" {
			text = buildArchStackTierContent(
				pptx.ConvertMarkdownEmphasis(tier.Label), headerSize,
				pptx.ConvertMarkdownEmphasis(tier.Description), bodySize,
			)
		} else {
			text = buildArchStackSimpleContent(pptx.ConvertMarkdownEmphasis(tier.Label), headerSize)
		}

		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)

		// Tiers are structure: a neutral 16% block with dk1 text and a 3pt
		// accent bar, separated by the 4pt white gutter. The old 100%→40%
		// accent-alpha ramp painted every tier in muddy mid-tints that encode
		// nothing (go-slide-creator-8xsj3); ApplyReadableInk darkens the text.
		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     neutralFillJSON(NeutralTint16),
				Line:     noLine,
				Text:     text,
			},
			AccentBar: &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: archStackTierBarPt},
		}
		applyArchStackOverride(cells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Side rail cells: span all rows for visual continuity (we fill them on each row)
		for j := 0; j < numRails; j++ {
			if i == 0 {
				// First row: render the side rail label
				railText := buildArchStackRailContent(vals.SideRails[j])
				cells[j+1] = &jsonschema.GridCellInput{
					RowSpan: len(vals.Tiers),
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     neutralFillJSON(NeutralTint8),
						Text:     railText,
					},
				}
				applyArchStackOverride(cells[j+1], cellOverrides, len(vals.Tiers)+j, accent)
			} else {
				// Subsequent rows: nil cell (covered by rowspan)
				cells[j+1] = nil
			}
		}

		rows = append(rows, jsonschema.GridRowInput{Cells: cells})
	}

	// Tiers are content-sized (go-slide-creator-3nsll): every tier takes the
	// tallest tier's written need plus padding, capped at
	// archStackTierMaxHeightFrac of the content height, so four one-line
	// tiers no longer stretch into ~85px slabs; a longer description still
	// grows past the cap rather than shrinking below the readable floor. The
	// grid centres the stack in the content area.
	contentW, contentH := contentAreaPt(ctx)
	tierW := contentW * cols[0] / 100
	need := 0.0
	for _, row := range rows {
		if c := row.Cells[0]; c != nil && c.Shape != nil {
			need = math.Max(need, writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, tierW, 0))
		}
	}
	if need > 0 && contentH > 0 {
		tierH := math.Ceil(math.Max(need, math.Min(need+archStackTierPadPt, contentH*archStackTierMaxHeightFrac)))
		for i := range rows {
			rows[i].MaxHeight = tierH
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		Gap:           ctx.Gap(4),
		RowGap:        ctx.Gap(4),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// archStackTierMaxHeightFrac caps a content-sized tier at this share of the
// content height.
const archStackTierMaxHeightFrac = 0.20

// archStackTierPadPt is the breathing room a tier gets beyond its written need.
const archStackTierPadPt = 8.0

func buildArchStackTierContent(label string, labelSize float64, desc string, descSize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: label, Size: labelSize, Bold: true, Color: "lt1", Align: "ctr"},
			{Content: desc, Size: descSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

// archStackRailWidthPct is the width of one cross-cutting rail, as a share of
// the content area. A rail carries one short label and nothing else; at the old
// 12% two rails took a quarter of the slide's width to say "Security" and
// "Monitoring", leaving two tall empty columns beside the stack
// (go-slide-creator-pr3g). A band wide enough for the rotated label is the
// consulting convention and gives the width back to the tiers.
const archStackRailWidthPct = 4.0

// archStackRailLabelSize is the rail label size. It is the smallest text on the
// slide by design: the rail names a concern, the tiers carry the content.
const archStackRailLabelSize = scaleCaptionPt

// buildArchStackRailContent renders a cross-cutting rail label rotated to read
// bottom-to-top, so the band only needs to be as wide as one line of text.
func buildArchStackRailContent(label string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
		Vert          string      `json:"vert"`
	}{
		Paragraphs: []paragraph{
			{Content: label, Size: archStackRailLabelSize, Bold: true, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
		// vert270 rotates the TEXT inside an unrotated shape, so the band's
		// fill stays a clean vertical stripe beside the stack.
		Vert: "vert270",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildArchStackSimpleContent(content string, size float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func applyArchStackOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*ArchStackCellOverride)
	if !coOk {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}

package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// driver-tree pattern — left-to-right value-driver decomposition
// ---------------------------------------------------------------------------
//
// Layout: root node (left) → 2-4 branch nodes (middle) → 1-4 leaf items per
// branch (right) → optional per-branch annotation column (far right). Elbow
// connectors fan out root → each branch → each leaf; the unboxed annotation
// column is never connected. The default "nodes" style (driver_tree_nodes.go)
// sizes every node to its label and centres a parent on its children; the
// legacy "slabs" style spans root and branches over their rows and lets the
// shape-grid row connector follow the row_span parents. The pattern is implemented
// as a shape_grid with row spans, NOT via svggen — svggen's org_chart is for
// people/role hierarchies and lacks horizontal-decomposition semantics
// (metric/unit fields).

func init() {
	Default().Register(&driverTree{})
}

type driverTree struct{}

func (dt *driverTree) Name() string { return "driver-tree" }
func (dt *driverTree) Description() string {
	return "Hierarchical value driver tree: root metric → 2-4 branch metrics → 1-4 leaf items per branch, with optional per-branch annotations and connector lines between levels"
}
func (dt *driverTree) UseWhen() string {
	return "Value driver, cost driver, or KPI decomposition where a top-level metric (with unit) breaks down into 2-4 contributing branches, each with 1-4 leaf items; prefer process-flow for sequential steps, pyramid for narrowing hierarchies, swimlane for cross-actor flows, and svggen org_chart for people/role hierarchies"
}
func (dt *driverTree) NotWhen() string {
	return "Hierarchy describes people or roles (use svggen org_chart), content is a sequential process (use process-flow), levels narrow visually toward a single point (use pyramid), or there is only one branch (use a labelled card-grid)"
}
func (dt *driverTree) Version() int { return 1 }
func (dt *driverTree) CellsHint() string {
	return "root + 2-4 branches + 1-4 leaves each (+ optional annotations)"
}
func (dt *driverTree) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"kpi-3up", "waterfall-bridge", "chart-insights-split"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (dt *driverTree) SupportsCallout() bool        { return true }
func (dt *driverTree) SupportsInlineMarkdown() bool { return true }

func (dt *driverTree) ExemplarValues() any {
	return &DriverTreeValues{
		Root: DriverTreeNode{Label: "Net Benefits", Unit: "($m USD)"},
		Branches: []DriverTreeBranch{
			{
				Label:      "Revenue Benefits",
				Unit:       "($m USD)",
				Leaves:     []string{"Reduce unscheduled outages", "Increase scheduling flexibility"},
				Annotation: "Primary value driver",
			},
			{
				Label:      "Cost Benefits",
				Unit:       "($m USD)",
				Leaves:     []string{"Lower maintenance spend", "Reduce overtime hours"},
				Annotation: "Secondary value driver",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// DriverTreeNode is the root metric: a label plus an optional unit suffix.
type DriverTreeNode struct {
	Label string `json:"label"`
	Unit  string `json:"unit,omitempty"`
}

// DriverTreeBranch is a single mid-level driver decomposing the root metric.
type DriverTreeBranch struct {
	Label      string   `json:"label"`
	Unit       string   `json:"unit,omitempty"`
	Leaves     []string `json:"leaves"`
	Annotation string   `json:"annotation,omitempty"`
}

// DriverTreeValues holds the root node and 2-4 branches.
type DriverTreeValues struct {
	Root     DriverTreeNode     `json:"root"`
	Branches []DriverTreeBranch `json:"branches"`
}

// DriverTreeOverrides is the standard text overrides plus the tree style.
type DriverTreeOverrides struct {
	TextOverrides
	// Style is "nodes" (default: every node as tall and as wide as its label,
	// a parent centred on its children, the root the only solid accent node)
	// or "slabs" (the legacy look: root and branches as tall as the rows they
	// decompose into).
	Style string `json:"style,omitempty"`
}

// driverTreeStyles are the accepted overrides.style values.
var driverTreeStyles = []string{"nodes", "slabs"}

// driverTreeOverridesSchema is the standard text overrides plus style.
func driverTreeOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(driverTreeStyles...).WithDescription("nodes (default): root, branches and leaves are boxes sized to their labels, each parent centred on its children and joined to them by elbow connectors; only the root is a solid accent box. slabs: root and branch boxes as tall as the rows they decompose into (legacy look)").WithDefault("nodes")
	return s
}

// DriverTreeCellOverride is the shared per-cell override; cells are indexed in
// the order: root, branches, leaves (flat across branches), annotations.
type DriverTreeCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (dt *driverTree) NewValues() any       { return &DriverTreeValues{} }
func (dt *driverTree) NewOverrides() any    { return &DriverTreeOverrides{} }
func (dt *driverTree) NewCellOverride() any { return &DriverTreeCellOverride{} }

// Budgets measured by TestDriverTreeBudgetProbe and TestDriverTreeSpanProbe
// against the written size (no run stored below its role floor) on every
// shipped template, at default text sizes and without units
// (go-slide-creator-n1muf), every shape keeping the uniform 0.5 cm text
// margin. Trees stay capped at 12 leaf rows.
const driverTreeMaxRows = 12

func driverTreeLeafBudget(total int, annotated bool) int {
	switch {
	case total > driverTreeMaxRows:
		return 0
	case total <= 4:
		return 120
	case total == 5 && annotated:
		return 81
	case total == 5:
		return 120
	case annotated:
		return 41
	default:
		return 61
	}
}

func driverTreeBranchBudget(total, span int, annotated bool) int {
	switch {
	case total > driverTreeMaxRows:
		return 0
	case !annotated && (span == 1 && total >= 6 || span == 2 && total >= 11):
		return 31
	case !annotated:
		return 60
	case span == 1 && total >= 6, span == 2 && total >= 11:
		return 25
	case span == 1 && total == 5, span == 2 && total >= 9:
		return 50
	default:
		return 60
	}
}

func driverTreeAnnotationBudget(total, span int) int {
	if total > driverTreeMaxRows {
		return 0
	}
	switch span {
	case 1:
		switch {
		case total <= 2:
			return 140
		case total == 3:
			return 100
		case total == 4:
			return 60
		case total == 5:
			return 40
		default:
			return 20
		}
	case 2:
		switch {
		case total <= 4:
			return 140
		case total == 5:
			return 120
		case total == 6:
			return 100
		case total == 7:
			return 80
		case total == 8:
			return 60
		case total <= 10:
			return 40
		default:
			return 20
		}
	case 3:
		switch {
		case total <= 7:
			return 140
		case total <= 9:
			return 100
		case total == 10:
			return 80
		default:
			return 60
		}
	default:
		switch {
		case total <= 9:
			return 140
		case total == 10:
			return 120
		default:
			return 100
		}
	}
}

func (dt *driverTree) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*DriverTreeValues)
	if !ok || v == nil {
		return nil
	}
	total, annotated := 0, false
	for _, b := range v.Branches {
		total += len(b.Leaves)
		annotated = annotated || strings.TrimSpace(b.Annotation) != ""
	}
	if total > driverTreeMaxRows {
		return []string{fmt.Sprintf("%s: driver-tree has %d leaf rows; this layout holds at most %d readable rows — aggregate leaves or split the tree", ErrCodeBodyTooLong, total, driverTreeMaxRows)}
	}
	var warnings []string
	for i, b := range v.Branches {
		span := len(b.Leaves)
		branchBudget := driverTreeBranchBudget(total, span, annotated)
		if n := runeLen(b.Label); n > branchBudget {
			warnings = append(warnings, fmt.Sprintf("%s: driver-tree branches[%d].label is %d characters; with %d leaf rows and a %d-row branch, use about %d characters — shorten the label or reduce leaves", ErrCodeBodyTooLong, i, n, total, span, branchBudget))
		}
		leafBudget := driverTreeLeafBudget(total, annotated)
		for j, leaf := range b.Leaves {
			if n := runeLen(leaf); n > leafBudget {
				warnings = append(warnings, fmt.Sprintf("%s: driver-tree branches[%d].leaves[%d] is %d characters; with %d leaf rows and annotations=%t, use about %d per leaf — shorten the item or reduce leaves", ErrCodeBodyTooLong, i, j, n, total, annotated, leafBudget))
			}
		}
		if note := strings.TrimSpace(b.Annotation); note != "" {
			annotationBudget := driverTreeAnnotationBudget(total, span)
			if n := runeLen(note); n > annotationBudget {
				warnings = append(warnings, fmt.Sprintf("%s: driver-tree branches[%d].annotation is %d characters; with %d leaf rows and a %d-row branch, use about %d characters — shorten the note or reduce leaves", ErrCodeBodyTooLong, i, n, total, span, annotationBudget))
			}
		}
	}
	return warnings
}

func (dt *driverTree) Schema() *Schema {
	rootSchema := ObjectSchema(
		map[string]*Schema{
			"label": StringSchema(60).WithDescription("Root metric name (e.g. \"Net Benefits\")"),
			"unit":  StringSchema(24).WithDescription("Optional unit suffix rendered under the label (e.g. \"($m USD)\")"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	branchSchema := ObjectSchema(
		map[string]*Schema{
			"label":      StringSchema(60).WithDescription("Branch metric name. At default sizes without a unit: about 60 chars normally; a one-leaf branch in a tree with 6-12 total leaves (or a two-leaf branch with 11-12) holds about 31 without annotations or 25 with them; annotated trees hold 50 for a one-leaf branch at 5 leaves and a two-leaf branch at 9-10"),
			"unit":       StringSchema(24).WithDescription("Optional unit suffix rendered under the branch label"),
			"leaves":     ArraySchema(StringSchema(120), 1, 4).WithDescription("1-4 leaf items; per-item budget by total leaf rows: 2-4: 120; 5: 120 without annotations, 81 with; 6-12: 61 without, 41 with. At most 12 rows per tree"),
			"annotation": StringSchema(140).WithDescription("Optional italic note. Approximate characters by branch span and total leaf rows: span 1: 2:140, 3:100, 4:60, 5:40, 6-12:20; span 2: <=4:140, 5:120, 6:100, 7:80, 8:60, 9-10:40, 11-12:20; span 3: <=7:140, 8-9:100, 10:80, 11-12:60; span 4: <=9:140, 10:120, 11-12:100"),
		},
		[]string{"label", "leaves"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"root":     rootSchema,
			"branches": ArraySchema(branchSchema, 2, 4).WithDescription("2-4 branches decomposing the root metric"),
		},
		[]string{"root", "branches"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      driverTreeOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Hierarchical value driver tree: root metric decomposed into 2-4 branches each with 1-4 leaves, plus optional per-branch annotations")
}

func (dt *driverTree) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*DriverTreeValues)
	if !ok || vals == nil {
		return fmt.Errorf("driver-tree: values must be *DriverTreeValues, got %T", values)
	}

	const name = "driver-tree"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*DriverTreeOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Style != "" && !slices.Contains(driverTreeStyles, ovr.Style) {
				errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, driverTreeStyles))
			}
		}
	}

	if strings.TrimSpace(vals.Root.Label) == "" {
		errs = append(errs, errRequired(name, "root.label"))
	} else if runeLen(vals.Root.Label) > 60 {
		errs = append(errs, errMaxLength(name, "root.label", 60, runeLen(vals.Root.Label)))
	}
	if runeLen(vals.Root.Unit) > 24 {
		errs = append(errs, errMaxLength(name, "root.unit", 24, runeLen(vals.Root.Unit)))
	}

	if len(vals.Branches) < 2 {
		errs = append(errs, errMinItems(name, "branches", 2, len(vals.Branches), "(hint: use a labelled card-grid for a single branch)"))
	}
	if len(vals.Branches) > 4 {
		errs = append(errs, errMaxItems(name, "branches", 4, len(vals.Branches), "(hint: split across two slides or aggregate sibling branches)"))
	}

	for i, b := range vals.Branches {
		labelPath := fmt.Sprintf("branches[%d].label", i)
		if strings.TrimSpace(b.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if runeLen(b.Label) > 60 {
			errs = append(errs, errMaxLength(name, labelPath, 60, runeLen(b.Label)))
		}
		if runeLen(b.Unit) > 24 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("branches[%d].unit", i), 24, runeLen(b.Unit)))
		}

		leavesPath := fmt.Sprintf("branches[%d].leaves", i)
		if len(b.Leaves) < 1 {
			errs = append(errs, errMinItems(name, leavesPath, 1, len(b.Leaves), ""))
		}
		if len(b.Leaves) > 4 {
			errs = append(errs, errMaxItems(name, leavesPath, 4, len(b.Leaves), ""))
		}
		for j, leaf := range b.Leaves {
			leafPath := fmt.Sprintf("branches[%d].leaves[%d]", i, j)
			if strings.TrimSpace(leaf) == "" {
				errs = append(errs, errRequired(name, leafPath))
			} else if runeLen(leaf) > 120 {
				errs = append(errs, errMaxLength(name, leafPath, 120, runeLen(leaf)))
			}
		}

		if runeLen(b.Annotation) > 140 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("branches[%d].annotation", i), 140, runeLen(b.Annotation)))
		}
	}

	totalCells := driverTreeTotalCells(vals)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// driverTreeTotalCells returns the cell-index ceiling for cell_overrides keys.
// Cell ordering: 0 = root, 1..B = branches, B+1..B+L = leaves (flat), then one
// annotation slot per branch that has annotation text.
func driverTreeTotalCells(vals *DriverTreeValues) int {
	total := 1 + len(vals.Branches)
	for _, b := range vals.Branches {
		total += len(b.Leaves)
	}
	for _, b := range vals.Branches {
		if strings.TrimSpace(b.Annotation) != "" {
			total++
		}
	}
	return total
}

func (dt *driverTree) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*DriverTreeValues)
	if !ok {
		return nil, fmt.Errorf("driver-tree: values must be *DriverTreeValues, got %T", values)
	}
	ovr := &DriverTreeOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*DriverTreeOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("driver-tree: overrides must be *DriverTreeOverrides, got %T", overrides)
		}
	}

	if ovr == nil {
		ovr = &DriverTreeOverrides{}
	}
	if ovr.Style != "slabs" {
		return dt.expandNodes(ctx, vals, ovr, cellOverrides)
	}
	return dt.expandSlabs(ctx, vals, ovr, cellOverrides)
}

// expandSlabs is the legacy look (overrides.style "slabs"): the root spans
// every leaf row and each branch its own leaves' rows, at full height.
func (dt *driverTree) expandSlabs(ctx ExpandContext, vals *DriverTreeValues, ovr *DriverTreeOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	rootSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	branchSize := ResolveSize(ovr.HeaderSize, scaleBodyPt)
	leafSize := ResolveSize(ovr.BodySize, sizeDenseCaptionPt)
	cellAccentMode := ovr.CellAccentMode

	totalLeaves := 0
	for _, b := range vals.Branches {
		totalLeaves += len(b.Leaves)
	}

	hasAnnotation := false
	for _, b := range vals.Branches {
		if strings.TrimSpace(b.Annotation) != "" {
			hasAnnotation = true
			break
		}
	}

	// Column widths as percentages. Annotation column only present when any
	// branch supplies an annotation.
	var cols []float64
	if hasAnnotation {
		cols = []float64{20, 24, 36, 20}
	} else {
		cols = []float64{22, 28, 50}
	}
	colsJSON, _ := json.Marshal(cols)

	// Cell index map for cell_overrides:
	//   0           = root
	//   1..B        = branches in order
	//   B+1..B+L    = leaves in flat order (branch1 leaves, branch2 leaves, ...)
	//   B+L+1..     = annotations (only for branches that provide one)
	branchIdx0 := 1
	leafIdx0 := 1 + len(vals.Branches)
	annotIdx0 := leafIdx0 + totalLeaves

	// Build a row per leaf. Only emit cells that should actually render — the
	// shapegrid validator's skip-occupied logic advances column position past
	// rowspan-covered cells, so we MUST NOT include nil placeholders to the
	// left of a visible cell (that confuses the column counter and produces
	// spurious col_span errors).
	rows := make([]jsonschema.GridRowInput, 0, totalLeaves)
	leafCounter := 0
	annotCounter := 0
	for branchPos, branch := range vals.Branches {
		branchAccent := ctx.ResolveCellAccent(baseAccent, branchPos, cellAccentMode)
		// Branch nodes are boxed in a light tint of their accent (they used to
		// be lt1-on-white, i.e. unboxed floating labels) so the elbow
		// connectors from the root and to the leaves attach to a visible box.
		branchTone := inactiveTintTone(branchAccent)
		branchSpan := len(branch.Leaves)
		annotationText := strings.TrimSpace(branch.Annotation)

		for leafPos, leaf := range branch.Leaves {
			var rowCells []*jsonschema.GridCellInput

			// Column 0 — root cell only on the very first row, with rowspan
			// covering every leaf row in the grid.
			if branchPos == 0 && leafPos == 0 {
				rootCell := &jsonschema.GridCellInput{
					RowSpan: totalLeaves,
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
						Text:     buildDriverTreeNodeText(vals.Root.Label, vals.Root.Unit, rootSize, readableTextOn(ctx, fillTone{Color: baseAccent}, "lt1")),
					},
				}
				applyDriverTreeOverride(rootCell, cellOverrides, 0, baseAccent)
				rowCells = append(rowCells, rootCell)
			}

			// Column 1 — branch cell only on the first leaf-row of each branch.
			if leafPos == 0 {
				branchCell := &jsonschema.GridCellInput{
					RowSpan: branchSpan,
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     branchTone.fillJSON(),
						Text:     buildDriverTreeNodeText(branch.Label, branch.Unit, branchSize, readableTextOn(ctx, branchTone, "dk1")),
					},
				}
				applyDriverTreeOverride(branchCell, cellOverrides, branchIdx0+branchPos, branchAccent)
				rowCells = append(rowCells, branchCell)
			}

			// Column 2 — leaf cell, always present, one per row.
			leafCell := &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"lt2"`),
					Text:     buildDriverTreeLeafText(pptx.ConvertMarkdownEmphasis(leaf), leafSize),
				},
			}
			applyDriverTreeOverride(leafCell, cellOverrides, leafIdx0+leafCounter, branchAccent)
			rowCells = append(rowCells, leafCell)
			leafCounter++

			// Column 3 — annotation cell only when the annotation column exists
			// and only on the first leaf-row of each branch. Branches without
			// annotation text get an invisible placeholder so column 3 width
			// stays consistent in the rendered grid.
			if hasAnnotation && leafPos == 0 {
				if annotationText != "" {
					annotCell := &jsonschema.GridCellInput{
						RowSpan: branchSpan,
						Shape: &jsonschema.ShapeSpecInput{
							Geometry: "rect",
							Fill:     json.RawMessage(`"none"`), // unboxed note: not a connector anchor
							Text:     buildDriverTreeAnnotationText(pptx.ConvertMarkdownEmphasis(annotationText), leafSize),
						},
					}
					applyDriverTreeOverride(annotCell, cellOverrides, annotIdx0+annotCounter, branchAccent)
					rowCells = append(rowCells, annotCell)
					annotCounter++
				} else {
					rowCells = append(rowCells, &jsonschema.GridCellInput{
						RowSpan: branchSpan,
						Shape: &jsonschema.ShapeSpecInput{
							Geometry: "rect",
							Fill:     json.RawMessage(`"none"`),
						},
					})
				}
			}

			row := jsonschema.GridRowInput{
				Cells:     rowCells,
				Connector: &jsonschema.ConnectorSpecInput{Style: "line", Color: baseAccent, Width: 1.0},
			}
			rows = append(rows, row)
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  ctx.Gap(driverTreeColGapPt), // room for the elbow connectors between columns
		RowGap:  ctx.Gap(4),
		Rows:    rows,
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type driverTreeParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Italic  bool    `json:"italic,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type driverTreeTextObj struct {
	Paragraphs    []driverTreeParagraph `json:"paragraphs"`
	Align         string                `json:"align"`
	VerticalAlign string                `json:"vertical_align"`
}

// buildDriverTreeNodeText renders a label with an optional unit line beneath.
func buildDriverTreeNodeText(label, unit string, size float64, color string) json.RawMessage {
	paras := []driverTreeParagraph{
		{
			Content: pptx.ConvertMarkdownEmphasis(label),
			Size:    size,
			Bold:    true,
			Color:   color,
			Align:   "ctr",
		},
	}
	if strings.TrimSpace(unit) != "" {
		paras = append(paras, driverTreeParagraph{
			Content: pptx.ConvertMarkdownEmphasis(unit),
			Size:    size - 2,
			Color:   color,
			Align:   "ctr",
		})
	}
	textObj := driverTreeTextObj{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildDriverTreeLeafText(content string, size float64) json.RawMessage {
	textObj := driverTreeTextObj{
		Paragraphs: []driverTreeParagraph{
			{Content: content, Size: size, Color: "dk1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildDriverTreeAnnotationText(content string, size float64) json.RawMessage {
	textObj := driverTreeTextObj{
		Paragraphs: []driverTreeParagraph{
			{Content: content, Size: size, Italic: true, Color: "dk1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func applyDriverTreeOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*DriverTreeCellOverride)
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

// driverTreeColGapPt is the column gap: wide enough for the elbow
// connectors (root -> branch -> leaf) to read as a tree.
const driverTreeColGapPt = 20.0

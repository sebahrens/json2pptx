package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// numbered-step-strip pattern — ordered steps WITHOUT flowchart/diamond
// semantics, with a built-in detail zone. Three render styles:
//
//   chevron     — 3-6 connected chevrons in a single row (optional description row)
//   stacked-box — table-like rows: narrow colored number/tip column + no-border body
//   toc         — high-polish numbered agenda (badge + title, optional body)
//
// The scorecard principle applies: a compact primary lane carries the ordinal
// label, an optional detail zone carries the per-step explanation. Unlike
// process-flow, this pattern never emits decision diamonds — it is for ordered
// sequences and table-of-contents lists, not branching workflows.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&numberedStepStrip{})
}

type numberedStepStrip struct{}

func (n *numberedStepStrip) Name() string { return "numbered-step-strip" }
func (n *numberedStepStrip) Description() string {
	return "Ordered numbered steps without flowchart diamonds, in chevron / stacked-box / toc styles, each with an optional per-step detail zone"
}
func (n *numberedStepStrip) UseWhen() string {
	return "Ordered steps or a table of contents that need numbering and an optional short explanation per item but NOT decision branching (3-6 steps); use stacked-box for a scorecard look, chevron for a compact ribbon, toc to replace a plain agenda"
}
func (n *numberedStepStrip) NotWhen() string {
	return "The flow has decision points/branches (use process-flow with type:decision), steps belong to different actors (use swimlane), stops are calendar dates (use timeline-horizontal), or items are unordered categories (use icon-row or card-grid)"
}
func (n *numberedStepStrip) Version() int      { return 1 }
func (n *numberedStepStrip) CellsHint() string { return "3-6" }
func (n *numberedStepStrip) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"open", "frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "stat-hero"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (n *numberedStepStrip) SupportsCallout() bool        { return true }
func (n *numberedStepStrip) SupportsInlineMarkdown() bool { return true }

func (n *numberedStepStrip) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 1},
		{Columns: 4, Rows: 1},
		{Columns: 5, Rows: 1},
		{Columns: 6, Rows: 1},
	}
}

func (n *numberedStepStrip) ExemplarValues() any {
	return &NumberedStepStripValues{
		Style: "stacked-box",
		Steps: []NumberedStepStripStep{
			{Label: "Discover", Body: "Map the current state and surface the constraints."},
			{Label: "Design", Body: "Shape the target operating model and the roadmap."},
			{Label: "Deliver", Body: "Stand up the capability and migrate the workload."},
			{Label: "Sustain", Body: "Embed the run model and track the value captured."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// numberedStepStripStyles enumerates the supported render styles.
const (
	numberedStepStripChevron    = "chevron"
	numberedStepStripStackedBox = "stacked-box"
	numberedStepStripTOC        = "toc"
)

// NumberedStepStripStep is a single ordered step.
type NumberedStepStripStep struct {
	Label    string `json:"label"`
	Body     string `json:"body,omitempty"`      // optional 1-3 line explanation (detail zone)
	Number   string `json:"number,omitempty"`    // optional ordinal override (e.g. "01", "A"); defaults to %02d
	TipColor string `json:"tip_color,omitempty"` // optional scheme color for this step's number/tip lane
}

// NumberedStepStripValues holds the render style and the ordered steps.
type NumberedStepStripValues struct {
	Style string                  `json:"style,omitempty"` // chevron | stacked-box | toc (default stacked-box)
	Steps []NumberedStepStripStep `json:"steps"`
}

// NumberedStepStripOverrides is the standard text overrides.
type NumberedStepStripOverrides = TextOverrides

// NumberedStepStripCellOverride is the shared per-cell override; indexed by step.
type NumberedStepStripCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) NewValues() any       { return &NumberedStepStripValues{} }
func (n *numberedStepStrip) NewOverrides() any    { return &NumberedStepStripOverrides{} }
func (n *numberedStepStrip) NewCellOverride() any { return &NumberedStepStripCellOverride{} }

func (n *numberedStepStrip) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label":     StringSchema(60).WithDescription("Short ordinal step label"),
			"body":      StringSchema(180).WithDescription("Optional 1-3 line explanation rendered in the detail zone"),
			"number":    StringSchema(6).WithDescription("Optional ordinal override (e.g. \"01\", \"A\"); defaults to the 1-based index"),
			"tip_color": StringSchema(0).WithDescription("Optional scheme color for this step's number / tip lane (default: rotating accent)"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"style": EnumSchema(numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC).
				WithDescription("Render style: chevron (connected ribbon), stacked-box (scorecard rows), or toc (numbered agenda)").
				WithDefault(numberedStepStripStackedBox),
			"steps": ArraySchema(stepSchema, 3, 6).WithDescription("Ordered steps (3-6)"),
		},
		[]string{"steps"},
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
	}).WithDescription("Ordered numbered steps without flowchart diamonds, in chevron / stacked-box / toc styles, each with an optional per-step detail zone")
}

func (n *numberedStepStrip) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*NumberedStepStripValues)
	if !ok || vals == nil {
		return fmt.Errorf("numbered-step-strip: values must be *NumberedStepStripValues, got %T", values)
	}

	const name = "numbered-step-strip"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*NumberedStepStripOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if vals.Style != "" &&
		vals.Style != numberedStepStripChevron &&
		vals.Style != numberedStepStripStackedBox &&
		vals.Style != numberedStepStripTOC {
		errs = append(errs, newValidationError(name, "style", ErrCodeUnknownEnum,
			fmt.Sprintf("numbered-step-strip: style must be %q, %q, or %q, got %q",
				numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC, vals.Style),
			UseOneOfFix("style", []string{numberedStepStripChevron, numberedStepStripStackedBox, numberedStepStripTOC})))
	}

	if len(vals.Steps) < 3 {
		errs = append(errs, errMinItems(name, "steps", 3, len(vals.Steps), ""))
	}
	if len(vals.Steps) > 6 {
		errs = append(errs, errMaxItems(name, "steps", 6, len(vals.Steps), "(hint: split across two slides or use agenda for a longer list)"))
	}

	for i, step := range vals.Steps {
		labelPath := fmt.Sprintf("steps[%d].label", i)
		if strings.TrimSpace(step.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if len(step.Label) > 60 {
			errs = append(errs, errMaxLength(name, labelPath, 60, len(step.Label)))
		}
		if len(step.Body) > 180 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].body", i), 180, len(step.Body)))
		}
		if len(step.Number) > 6 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].number", i), 6, len(step.Number)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (n *numberedStepStrip) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*NumberedStepStripValues)
	if !ok {
		return nil, fmt.Errorf("numbered-step-strip: values must be *NumberedStepStripValues, got %T", values)
	}
	ovr := &NumberedStepStripOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*NumberedStepStripOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("numbered-step-strip: overrides must be *NumberedStepStripOverrides, got %T", overrides)
		}
	}

	style := vals.Style
	if style == "" {
		style = numberedStepStripStackedBox
	}

	switch style {
	case numberedStepStripChevron:
		return n.expandChevron(ctx, vals, ovr, cellOverrides), nil
	case numberedStepStripTOC:
		return n.expandTOC(ctx, vals, ovr, cellOverrides), nil
	default:
		return n.expandStackedBox(ctx, vals, ovr, cellOverrides), nil
	}
}

// hasBody reports whether any step carries detail-zone text.
func (n *numberedStepStrip) hasBody(vals *NumberedStepStripValues) bool {
	for _, s := range vals.Steps {
		if strings.TrimSpace(s.Body) != "" {
			return true
		}
	}
	return false
}

// stepNumber returns the explicit ordinal override or the zero-padded index.
func stepNumber(step NumberedStepStripStep, idx int) string {
	if strings.TrimSpace(step.Number) != "" {
		return step.Number
	}
	return fmt.Sprintf("%02d", idx+1)
}

// ---------------------------------------------------------------------------
// chevron style — single row of connected chevrons; optional description row.
// When no step carries body text the strip is capped at ~35% content height.
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandChevron(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, 13.0)
	descSize := ResolveSize(ovr.BodySize, chevronDescDefaultSize)
	cellAccentMode := ovr.CellAccentMode

	withBody := n.hasBody(vals)
	count := len(vals.Steps)

	geo := chevronStripGeometry(ctx, count)
	notchInsetPt := geo.notchPt + chevronTextPadPt

	chevronCells := make([]*jsonschema.GridCellInput, count)
	descCells := make([]*jsonschema.GridCellInput, count)
	maxDescLines := 0
	for i, step := range vals.Steps {
		fill := step.TipColor
		if fill == "" {
			fill = ResolveCellAccent(baseAccent, i, cellAccentMode)
		}
		textColor := readableTextOn(ctx, fillTone{Color: fill}, "lt1")
		text := buildChevronLabelText(stepNumber(step, i), pptx.ConvertMarkdownEmphasis(step.Label), labelSize, textColor, notchInsetPt)
		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "chevron",
				Fill:        json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
				Text:        text,
				Adjustments: map[string]int64{"adj": chevronAdj},
			},
		}
		applyNumberedStepOverride(cell, cellOverrides, i, baseAccent)
		chevronCells[i] = cell

		body := strings.TrimSpace(step.Body)
		descShape := &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}
		if body != "" {
			descShape.Text = buildChevronDescText(pptx.ConvertMarkdownEmphasis(body), descSize)
			if lines := estimateWrappedLines(body, descSize, geo.stepWPt-2*chevronDescInsetPt); lines > maxDescLines {
				maxDescLines = lines
			}
		}
		descCells[i] = &jsonschema.GridCellInput{Shape: descShape}
	}

	colsJSON, _ := json.Marshal(count)

	// Height budget: the chevron row is capped so every chevron is at least
	// twice as wide as it is tall, and the detail row is sized to its text so
	// descriptions sit directly under the chevrons rather than floating in a
	// flex row that fills the rest of the slide.
	totalPt := geo.chevHPt
	chevRow := jsonschema.GridRowInput{Cells: chevronCells}
	rows := []jsonschema.GridRowInput{chevRow}
	if withBody {
		descHPt := float64(maxDescLines)*descSize*chevronLineHeight + 2*chevronDescInsetPt
		totalPt = geo.chevHPt + chevronRowGapPt + descHPt
		rows[0].Height = geo.chevHPt / totalPt * 100
		rows = append(rows, jsonschema.GridRowInput{Cells: descCells})
	}
	heightPct := totalPt / geo.contentHPt * 100
	if heightPct > 100 {
		// Content taller than the area: fall back to a proportional split so
		// the chevrons keep their capped share and descriptions get the rest.
		heightPct = 100
		if withBody {
			rows[0].Height = math.Min(geo.chevHPt/geo.contentHPt*100, 40)
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     0,
		RowGap:  chevronRowGapPt,
		Rows:    rows,
		Bounds:  &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: math.Round(heightPct*10) / 10},
	}
	return grid
}

const (
	// chevronAdj is the chevron point/notch depth as a fraction (×100000) of
	// the shape height. The OOXML default (50000) on a near-square chevron
	// swallows the label; 30% keeps a clear arrow with room for text.
	chevronAdj = 30000
	// chevronDescDefaultSize is the default detail-zone text size (pt).
	chevronDescDefaultSize = 12.0
	// chevronTextPadPt is extra breathing room beyond the notch depth.
	chevronTextPadPt = 3.0
	// chevronDescInsetPt matches the default shape text inset (0.1in ≈ 7.2pt).
	chevronDescInsetPt = 7.2
	chevronRowGapPt    = 6.0
	chevronLineHeight  = 1.25
	// chevronMaxAspectH caps chevron height at half its width (width >= 2x height).
	chevronMaxAspectH = 0.5
	// chevronMaxHeightFrac caps the chevron row at this share of the content height.
	chevronMaxHeightFrac = 0.3
	// chevronMinHeightPt keeps a two-line (number + label) chevron legible.
	chevronMinHeightPt = 44.0
)

// chevronGeometry is the estimated per-step geometry of a chevron strip.
type chevronGeometry struct {
	stepWPt    float64 // width of one chevron column
	chevHPt    float64 // chevron row height
	notchPt    float64 // depth of the tail notch (= depth of the point)
	contentHPt float64 // content-area height the grid bounds are relative to
}

// chevronStripGeometry estimates chevron sizes from the content area so the
// row height, notch depth and text insets can be fixed at expand time.
func chevronStripGeometry(ctx ExpandContext, count int) chevronGeometry {
	w, h := expandContentSize(ctx)
	if w <= 0 || h <= 0 {
		db := shapegrid.DefaultBounds(12192000, 6858000)
		w, h = db.CX, db.CY
	}
	if count < 1 {
		count = 1
	}
	const emuPerPt = 12700.0
	stepW := float64(w) / emuPerPt / float64(count)
	contentH := float64(h) / emuPerPt
	chevH := math.Min(stepW*chevronMaxAspectH, contentH*chevronMaxHeightFrac)
	if chevH < chevronMinHeightPt {
		chevH = math.Min(chevronMinHeightPt, stepW*chevronMaxAspectH)
	}
	// OOXML chevron: notch/point depth = adj × min(w, h); h is the minimum
	// because of the aspect cap above.
	notch := float64(chevronAdj) / 100000 * math.Min(chevH, stepW)
	return chevronGeometry{stepWPt: stepW, chevHPt: chevH, notchPt: notch, contentHPt: contentH}
}

// estimateWrappedLines estimates how many lines text wraps to at sizePt in a
// box widthPt wide (average glyph advance ≈ 0.5 em).
func estimateWrappedLines(text string, sizePt, widthPt float64) int {
	if widthPt <= 0 || sizePt <= 0 {
		return 1
	}
	perLine := int(widthPt / (sizePt * 0.5))
	if perLine < 1 {
		perLine = 1
	}
	lines := 0
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		cur := 0
		paraLines := 1
		for _, w := range words {
			l := len([]rune(w))
			switch {
			case cur == 0:
				cur = l
			case cur+1+l <= perLine:
				cur += 1 + l
			default:
				paraLines++
				cur = l
			}
			for cur > perLine {
				paraLines++
				cur -= perLine
			}
		}
		lines += paraLines
	}
	if lines < 1 {
		lines = 1
	}
	return lines
}

// buildChevronLabelText renders the number + label inside a chevron. The
// left inset clears the tail notch and the right inset clears the point, so
// no glyph falls into the notch (which shows the slide background) even in
// renderers that lay text out over the whole shape box.
func buildChevronLabelText(number, label string, size float64, color string, insetPt float64) json.RawMessage {
	obj := struct {
		numberedStepTextObj
		InsetLeft  float64 `json:"inset_left"`
		InsetRight float64 `json:"inset_right"`
	}{
		numberedStepTextObj: numberedStepTextObj{
			Paragraphs: []numberedStepParagraph{
				{Content: number, Size: size - 2, Bold: true, Color: color, Align: "ctr"},
				{Content: label, Size: size, Bold: true, Color: color, Align: "ctr"},
			},
			Align:         "ctr",
			VerticalAlign: "ctr",
		},
		InsetLeft:  insetPt,
		InsetRight: insetPt,
	}
	data, _ := json.Marshal(obj)
	return data
}

// buildChevronDescText renders a detail-zone description, top-anchored so it
// sits directly under its chevron.
func buildChevronDescText(body string, size float64) json.RawMessage {
	obj := numberedStepTextObj{
		Paragraphs: []numberedStepParagraph{
			{Content: body, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "t",
	}
	data, _ := json.Marshal(obj)
	return data
}

// ---------------------------------------------------------------------------
// stacked-box style — table-like rows: narrow colored number/tip column on the
// left + no-border body column on the right (the scorecard look).
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandStackedBox(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	numberSize := ResolveSize(ovr.HeaderSize, 18.0)
	labelSize := 13.0
	bodySize := ResolveSize(ovr.BodySize, 10.0)
	cellAccentMode := ovr.CellAccentMode

	rows := make([]jsonschema.GridRowInput, len(vals.Steps))
	for i, step := range vals.Steps {
		tip := step.TipColor
		if tip == "" {
			tip = ResolveCellAccent(baseAccent, i, cellAccentMode)
		}

		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, tip)),
				Text:     buildNumberedStepNumberText(stepNumber(step, i), numberSize, "lt1"),
			},
		}

		bodyCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text: buildNumberedStepStackedBody(
					pptx.ConvertMarkdownEmphasis(step.Label), labelSize,
					pptx.ConvertMarkdownEmphasis(strings.TrimSpace(step.Body)), bodySize),
			},
		}
		applyNumberedStepOverride(bodyCell, cellOverrides, i, baseAccent)

		rows[i] = jsonschema.GridRowInput{
			AutoHeight: true,
			Cells:      []*jsonschema.GridCellInput{numberCell, bodyCell},
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[1, 8]`),
		Gap:     8,
		RowGap:  6,
		Rows:    rows,
	}
}

// ---------------------------------------------------------------------------
// toc style — high-polish numbered agenda: a rounded accent number badge and a
// no-fill title (optionally with a body line). Can replace a plain agenda.
// ---------------------------------------------------------------------------

func (n *numberedStepStrip) expandTOC(ctx ExpandContext, vals *NumberedStepStripValues, ovr *NumberedStepStripOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	numberSize := ResolveSize(ovr.HeaderSize, 20.0)
	titleSize := 14.0
	bodySize := ResolveSize(ovr.BodySize, 10.0)
	cellAccentMode := ovr.CellAccentMode

	rows := make([]jsonschema.GridRowInput, len(vals.Steps))
	for i, step := range vals.Steps {
		badge := step.TipColor
		if badge == "" {
			badge = ResolveCellAccent(baseAccent, i, cellAccentMode)
		}

		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, badge)),
				Text:     buildNumberedStepNumberText(stepNumber(step, i), numberSize, "lt1"),
			},
		}

		titleCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text: buildNumberedStepStackedBody(
					pptx.ConvertMarkdownEmphasis(step.Label), titleSize,
					pptx.ConvertMarkdownEmphasis(strings.TrimSpace(step.Body)), bodySize),
			},
		}
		applyNumberedStepOverride(titleCell, cellOverrides, i, baseAccent)

		rows[i] = jsonschema.GridRowInput{
			AutoHeight: true,
			Cells:      []*jsonschema.GridCellInput{numberCell, titleCell},
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[1, 6]`),
		Gap:     10,
		RowGap:  6,
		Rows:    rows,
	}
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type numberedStepParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type numberedStepTextObj struct {
	Paragraphs    []numberedStepParagraph `json:"paragraphs"`
	Align         string                  `json:"align"`
	VerticalAlign string                  `json:"vertical_align"`
}

// buildNumberedStepNumberText renders a single centered ordinal, used in the
// number / tip lane of stacked-box and toc styles.
func buildNumberedStepNumberText(number string, size float64, color string) json.RawMessage {
	obj := numberedStepTextObj{
		Paragraphs: []numberedStepParagraph{
			{Content: number, Size: size, Bold: true, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

// buildNumberedStepStackedBody renders a bold label and, when present, a body
// line beneath it — the left-aligned body column for stacked-box and toc.
func buildNumberedStepStackedBody(label string, labelSize float64, body string, bodySize float64) json.RawMessage {
	paras := []numberedStepParagraph{
		{Content: label, Size: labelSize, Bold: true, Color: "dk1", Align: "l"},
	}
	if body != "" {
		paras = append(paras, numberedStepParagraph{Content: body, Size: bodySize, Color: "dk2", Align: "l"})
	}
	obj := numberedStepTextObj{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

// applyNumberedStepOverride applies the shared per-cell accent-bar override.
func applyNumberedStepOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*NumberedStepStripCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}

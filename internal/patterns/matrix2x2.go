package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// matrix-2x2 pattern — quadrant/positioning matrix with axis labels
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&matrix2x2{})
}

type matrix2x2 struct{}

func (m *matrix2x2) Name() string { return "matrix-2x2" }
func (m *matrix2x2) Description() string {
	return "2×2 quadrant matrix: four tonal quadrant fields split by a white gutter (one optional highlight quadrant in the solid accent), each with a bold name over its body, and two dark axis bars carrying low end, title and high end"
}
func (m *matrix2x2) UseWhen() string {
	return "Items positioned on two named axes (priority/effort, impact/feasibility); prefer comparison-2col when only one dimension matters, card-grid when items don't map to axes"
}
func (m *matrix2x2) NotWhen() string {
	return "Only one comparison dimension (use comparison-2col), items are unstructured cards (use card-grid), or layout is a standard BMC (use bmc-canvas)"
}
func (m *matrix2x2) Version() int      { return 2 }
func (m *matrix2x2) CellsHint() string { return "4 + axes" }
func (m *matrix2x2) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "compare"},
		PairsWith:     []string{"kpi-3up", "card-grid", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
		DataVisual:    true,
	}
}

func (m *matrix2x2) ExemplarValues() any {
	return &Matrix2x2Values{
		XAxisLabel:  "Market Share",
		YAxisLabel:  "Market Growth",
		TopLeft:     Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
		TopRight:    Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
		BottomLeft:  Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
		BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Matrix2x2Quadrant is a single quadrant cell with a header and body.
type Matrix2x2Quadrant struct {
	Header string   `json:"header"`
	Body   string   `json:"body,omitempty"`
	Icon   *IconRef `json:"icon,omitempty"` // Icon: bundled-name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object
	// Highlight marks the quadrant the slide is about (at most one): the
	// solid accent field of the tonal matrix, the only tinted area of the
	// open one.
	Highlight bool `json:"highlight,omitempty"`
}

// UnmarshalJSON supports string shorthand "Header | Body" or object {header, body}.
func (q *Matrix2x2Quadrant) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) == 2 {
			q.Header = parts[0]
			q.Body = parts[1]
		} else {
			q.Header = s
		}
		return nil
	}
	type alias Matrix2x2Quadrant
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("Matrix2x2Quadrant must be string or {header, body}: %w", err)
	}
	*q = Matrix2x2Quadrant(a)
	return nil
}

// Matrix2x2Values is the values type for matrix-2x2.
// Supports both named fields (top_left, top_right, bottom_left, bottom_right)
// and positional array form (quadrants: [TL, TR, BL, BR]).
type Matrix2x2Values struct {
	XAxisLabel  string            `json:"x_axis_label"`
	YAxisLabel  string            `json:"y_axis_label"`
	XLow        string            `json:"x_low,omitempty"`  // left end of the x axis (default "Low")
	XHigh       string            `json:"x_high,omitempty"` // right end of the x axis (default "High")
	YLow        string            `json:"y_low,omitempty"`  // bottom end of the y axis (default "Low")
	YHigh       string            `json:"y_high,omitempty"` // top end of the y axis (default "High")
	TopLeft     Matrix2x2Quadrant `json:"top_left"`
	TopRight    Matrix2x2Quadrant `json:"top_right"`
	BottomLeft  Matrix2x2Quadrant `json:"bottom_left"`
	BottomRight Matrix2x2Quadrant `json:"bottom_right"`

	// Quadrants holds a positional quadrants array whose length is not 4.
	// UnmarshalJSON maps a 4-item array onto the named slots; any other
	// length is kept here (not dropped) so Validate can report the count
	// instead of the input inspector calling "quadrants" an unknown field
	// (go-slide-creator-s1uvj.42). Nil in every valid input.
	Quadrants []Matrix2x2Quadrant `json:"quadrants,omitempty"`
}

// UnmarshalJSON supports positional quadrants array: {"quadrants": [TL, TR, BL, BR]}
// as an alternative to named fields {top_left, top_right, bottom_left, bottom_right}.
func (v *Matrix2x2Values) UnmarshalJSON(data []byte) error {
	// Try the quadrants array form first.
	var withArray struct {
		XAxisLabel string              `json:"x_axis_label"`
		YAxisLabel string              `json:"y_axis_label"`
		XLow       string              `json:"x_low"`
		XHigh      string              `json:"x_high"`
		YLow       string              `json:"y_low"`
		YHigh      string              `json:"y_high"`
		Quadrants  []Matrix2x2Quadrant `json:"quadrants"`
	}
	if err := json.Unmarshal(data, &withArray); err == nil && len(withArray.Quadrants) == 4 {
		v.XAxisLabel = withArray.XAxisLabel
		v.YAxisLabel = withArray.YAxisLabel
		v.XLow, v.XHigh = withArray.XLow, withArray.XHigh
		v.YLow, v.YHigh = withArray.YLow, withArray.YHigh
		v.TopLeft = withArray.Quadrants[0]
		v.TopRight = withArray.Quadrants[1]
		v.BottomLeft = withArray.Quadrants[2]
		v.BottomRight = withArray.Quadrants[3]
		return nil
	}
	// Fall back to named fields. The alias carries Quadrants too, so a
	// positional array of the wrong length is kept for Validate to report.
	type alias Matrix2x2Values
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("Matrix2x2Values: %w", err)
	}
	*v = Matrix2x2Values(a)
	return nil
}

// Matrix2x2Overrides contains pattern-level overrides for matrix-2x2.
type Matrix2x2Overrides struct {
	TextOverrides
	LabelSize float64 `json:"label_size,omitempty"`
	// Style is "tonal" (default: four quadrant fields in the accent's tint
	// split by a white gutter, the highlight quadrant in the solid accent,
	// and two dark axis bars, go-slide-creator-ckpye), "open" (two crossing
	// axis lines, open quadrants, axis titles and low / high ends along the
	// left and bottom edges, go-slide-creator-jnkiq) or "tiles" (four neutral
	// quadrant tiles with thin arrow axes above and beside them).
	Style string `json:"style,omitempty"`
}

// The accepted overrides.style values.
const (
	matrix2x2StyleTonal = "tonal"
	matrix2x2StyleOpen  = "open"
	matrix2x2StyleTiles = "tiles"
)

var matrix2x2Styles = []string{matrix2x2StyleTonal, matrix2x2StyleOpen, matrix2x2StyleTiles}

// matrix2x2Style is the style the matrix renders in.
func matrix2x2Style(ovr *Matrix2x2Overrides) string {
	if ovr == nil || ovr.Style == "" {
		return matrix2x2StyleTonal
	}
	return ovr.Style
}

// matrix2x2Open reports whether the matrix renders in the open style.
func matrix2x2Open(ovr *Matrix2x2Overrides) bool { return matrix2x2Style(ovr) == matrix2x2StyleOpen }

// matrix2x2Tonal reports whether the matrix renders in the tonal style.
func matrix2x2Tonal(ovr *Matrix2x2Overrides) bool { return matrix2x2Style(ovr) == matrix2x2StyleTonal }

// Matrix2x2CellOverride is an alias for the shared CellOverride struct.
type Matrix2x2CellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (m *matrix2x2) NewValues() any       { return &Matrix2x2Values{} }
func (m *matrix2x2) NewOverrides() any    { return &Matrix2x2Overrides{} }
func (m *matrix2x2) NewCellOverride() any { return &Matrix2x2CellOverride{} }

// matrixBodyRunBudget is the unbroken body run a quadrant holds beside header.
// matrixHeaderRunBudget is the header unbroken run past which the body keeps
// only about 40 characters.
const matrixHeaderRunBudget = 47

func matrixBodyRunBudget(header string, headerRun int) int {
	switch {
	case headerRun > matrixHeaderRunBudget:
		return 40
	case runeLen(header) >= 40:
		return 126
	default:
		return 163
	}
}

func (m *matrix2x2) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*Matrix2x2Values)
	if !ok || v == nil {
		return nil
	}
	warnings := matrixRunBudgetWarnings(v)
	if len(warnings) > 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// The run budgets assume a typical content area; with the template's own
	// area the quadrant rows are measured against it (go-slide-creator-n1muf).
	ovr, _ := overrides.(*Matrix2x2Overrides)
	if ovr == nil {
		ovr = &Matrix2x2Overrides{}
	}
	lay := layoutMatrix2x2(ctx, v, ovr)
	if total := lay.needs[0] + lay.needs[1]; total > lay.availPt+1 {
		warnings = append(warnings, fmt.Sprintf("%s: matrix-2x2 quadrants need %.0fpt at readable sizes but the content area holds about %.0fpt for them — shorten the quadrant headers or bodies", ErrCodeBodyTooLong, total, lay.availPt))
	}
	return warnings
}

// matrixRunBudgetWarnings reports quadrants whose unbroken runs exceed the
// measured run budgets.
func matrixRunBudgetWarnings(v *Matrix2x2Values) []string {
	quadrants := []struct {
		name  string
		value Matrix2x2Quadrant
	}{
		{"top_left", v.TopLeft}, {"top_right", v.TopRight},
		{"bottom_left", v.BottomLeft}, {"bottom_right", v.BottomRight},
	}
	var warnings []string
	for _, quadrant := range quadrants {
		headerRun, bodyRun := 0, 0
		for _, word := range strings.Fields(quadrant.value.Header) {
			headerRun = max(headerRun, runeLen(word))
		}
		for _, word := range strings.Fields(quadrant.value.Body) {
			bodyRun = max(bodyRun, runeLen(word))
		}
		// Budgets measured against the written size on every shipped
		// template with the uniform 0.5 cm shape text margin
		// (go-slide-creator-n1muf): a body's unbroken run holds about 163
		// characters beside a short header, 126 beside a long worded header,
		// and 40 beside a header with an unbroken run over 47 characters
		// (which also leaves no room for longer worded copy).
		budget := matrixBodyRunBudget(quadrant.value.Header, headerRun)
		if bodyRun > budget || (headerRun > matrixHeaderRunBudget && runeLen(quadrant.value.Body) > 40) {
			warnings = append(warnings, fmt.Sprintf("%s: matrix-2x2 %s.header/body contain %d/%d-character unbroken runs; this header leaves about %d wide characters for its body — add word breaks or shorten the paired copy", ErrCodeBodyTooLong, quadrant.name, headerRun, bodyRun, budget))
		}
	}
	return warnings
}

func (m *matrix2x2) Schema() *Schema {
	// Define the quadrant schema once in $defs and reference it from each
	// quadrant slot so a polymorphic icon spec doesn't multiply the schema size
	// by 5 (4 named slots + 1 array form).
	quadrantObjSchema := ObjectSchema(
		map[string]*Schema{
			"header":    StringSchema(80).WithDescription("Quadrant header text"),
			"body":      StringSchema(200).WithDescription("Quadrant body text; keep unbroken runs near 163 characters (126 beside a long header; 40 and at most 40 characters of copy beside a header with an unbroken run over 47) or add word breaks"),
			"icon":      IconRefSchema(""),
			"highlight": BooleanSchema().WithDescription("The quadrant the slide is about (at most one): drawn in the solid accent"),
		},
		[]string{"header"},
	).WithAdditionalProperties(false)

	quadrantSchema := OneOfSchema(
		quadrantObjSchema,
		StringSchema(0).WithDescription(`String shorthand: "Header" or "Header | Body"`),
	)

	quadrantRef := RefSchema("quadrant")
	axisEndRef := RefSchema("axisEnd")

	// Named form: top_left, top_right, bottom_left, bottom_right
	namedValuesSchema := ObjectSchema(
		map[string]*Schema{
			"x_axis_label": StringSchema(matrix2x2XAxisMax).WithDescription("X-axis label (horizontal dimension)"),
			"y_axis_label": StringSchema(60).WithDescription("Y-axis label (vertical dimension)"),
			"x_low":        axisEndRef,
			"x_high":       axisEndRef,
			"y_low":        axisEndRef,
			"y_high":       axisEndRef,
			"top_left":     quadrantRef,
			"top_right":    quadrantRef,
			"bottom_left":  quadrantRef,
			"bottom_right": quadrantRef,
		},
		[]string{"x_axis_label", "y_axis_label", "top_left", "top_right", "bottom_left", "bottom_right"},
	).WithAdditionalProperties(false)

	// Positional form: quadrants: [TL, TR, BL, BR]
	arrayValuesSchema := ObjectSchema(
		map[string]*Schema{
			"x_axis_label": StringSchema(matrix2x2XAxisMax).WithDescription("X-axis label (horizontal dimension)"),
			"y_axis_label": StringSchema(60).WithDescription("Y-axis label (vertical dimension)"),
			"x_low":        axisEndRef,
			"x_high":       axisEndRef,
			"y_low":        axisEndRef,
			"y_high":       axisEndRef,
			"quadrants":    ArraySchema(quadrantRef, 4, 4).WithDescription("Positional quadrants: [top_left, top_right, bottom_left, bottom_right]"),
		},
		[]string{"x_axis_label", "y_axis_label", "quadrants"},
	).WithAdditionalProperties(false)

	valuesSchema := OneOfSchema(namedValuesSchema, arrayValuesSchema)

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"header_size":     NumberSchema(6, 120).WithDescription("Font size for quadrant headers in points"),
					"body_size":       NumberSchema(6, 120).WithDescription("Font size for quadrant body text in points"),
					"label_size":      NumberSchema(6, 120).WithDescription("Font size for axis labels in points"),
					"style":           EnumSchema(matrix2x2Styles...).WithDescription("tonal (default: four accent-tint quadrant fields split by a white gutter, the highlight quadrant solid, dark axis bars carrying low end, title and high end), open (two crossing axis lines, unfilled quadrants) or tiles (four neutral quadrant tiles with thin arrow axes)").WithDefault(matrix2x2StyleTonal),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
		"quadrant":     quadrantSchema,
		"axisEnd":      StringSchema(matrix2x2AxisEndMax).WithDescription("Axis end label (x_low/x_high: left/right; y_low/y_high: bottom/top); default \"Low\" / \"High\""),
	}).WithDescription("2×2 quadrant matrix with axis labels")
}

func (m *matrix2x2) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*Matrix2x2Values)
	if !ok || vals == nil {
		return fmt.Errorf("matrix-2x2: values must be *Matrix2x2Values, got %T", values)
	}

	const name = "matrix-2x2"
	var errs []error

	// Axis labels required
	if vals.XAxisLabel == "" {
		errs = append(errs, errRequired(name, "x_axis_label"))
	} else if runeLen(vals.XAxisLabel) > matrix2x2XAxisMax {
		errs = append(errs, errMaxLength(name, "x_axis_label", matrix2x2XAxisMax, runeLen(vals.XAxisLabel)))
	}
	if vals.YAxisLabel == "" {
		errs = append(errs, errRequired(name, "y_axis_label"))
	} else if runeLen(vals.YAxisLabel) > 60 {
		errs = append(errs, errMaxLength(name, "y_axis_label", 60, runeLen(vals.YAxisLabel)))
	}

	for _, end := range []struct{ path, v string }{
		{"x_low", vals.XLow}, {"x_high", vals.XHigh}, {"y_low", vals.YLow}, {"y_high", vals.YHigh},
	} {
		if runeLen(end.v) > matrix2x2AxisEndMax {
			errs = append(errs, errMaxLength(name, end.path, matrix2x2AxisEndMax, runeLen(end.v)))
		}
	}

	// A positional quadrants array must map one-to-one onto the four slots;
	// report its length rather than four missing named quadrants.
	if vals.Quadrants != nil {
		errs = append(errs, errCountMismatch(name, "quadrants", 4, len(vals.Quadrants),
			"([top_left, top_right, bottom_left, bottom_right]; hint: for more or fewer than four items use card-grid)"))
	}

	// Validate each quadrant (skipped when a wrong-length quadrants array
	// left the named slots empty — the count error above says why).
	quads := []struct {
		name string
		q    Matrix2x2Quadrant
	}{
		{"top_left", vals.TopLeft},
		{"top_right", vals.TopRight},
		{"bottom_left", vals.BottomLeft},
		{"bottom_right", vals.BottomRight},
	}
	if vals.Quadrants != nil {
		quads = nil
	}
	for _, qd := range quads {
		headerPath := qd.name + ".header"
		if qd.q.Header == "" {
			errs = append(errs, errRequired(name, headerPath))
		} else if runeLen(qd.q.Header) > 80 {
			errs = append(errs, errMaxLength(name, headerPath, 80, runeLen(qd.q.Header)))
		}
		if runeLen(qd.q.Body) > 200 {
			errs = append(errs, errMaxLength(name, qd.name+".body", 200, runeLen(qd.q.Body)))
		}
		if qd.q.Icon != nil {
			errs = append(errs, validateIconRef(name, qd.name+".icon", *qd.q.Icon)...)
		}
	}

	errs = append(errs, singleHighlightErrors(name, "quadrant", "quadrants", len(quads),
		func(i int) bool { return quads[i].q.Highlight },
		func(i int) string { return quads[i].name + ".highlight" })...)
	if ovr, ok := overrides.(*Matrix2x2Overrides); ok && ovr != nil && ovr.Style != "" && !slices.Contains(matrix2x2Styles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, matrix2x2Styles))
	}

	// Validate cell_overrides: indices 0-3 only
	const totalCells = 4
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, "(hint: 0=top_left, 1=top_right, 2=bottom_left, 3=bottom_right)"); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (m *matrix2x2) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*Matrix2x2Values)
	if !ok {
		return nil, fmt.Errorf("matrix-2x2: values must be *Matrix2x2Values, got %T", values)
	}
	ovr := &Matrix2x2Overrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*Matrix2x2Overrides)
		if !ovrOk {
			return nil, fmt.Errorf("matrix-2x2: overrides must be *Matrix2x2Overrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := layoutMatrix2x2(ctx, vals, ovr)
	headerSize, bodySize := lay.headerSize, lay.bodySize
	labelSize := ResolveSize(ovr.LabelSize, scaleSubheadPt)
	if matrix2x2Tonal(ovr) {
		return expandMatrix2x2Tonal(ctx, vals, lay, labelSize, accent, cellOverrides), nil
	}
	if matrix2x2Open(ovr) {
		return expandMatrix2x2Open(ctx, vals, lay, labelSize, accent, cellOverrides), nil
	}

	// Layout: 3 columns [y-axis label, left quadrants, right quadrants]
	// Row 0: [empty corner, x-axis label (col_span=2)]
	// Row 1: [y-axis label (row_span=2, vertical text), TL quadrant, TR quadrant]
	// Row 2: [BL quadrant, BR quadrant]  (y-axis spans from row 1)

	// X-axis label cell
	// Both axes are drawn as thin arrows pointing in the "high" direction
	// (right for x, up for y) flanked by low / high end labels, so the reader can
	// tell which quadrant is high-high without guessing from the headers.
	xLow, xHigh := axisEnds(vals.XLow, vals.XHigh)
	yLow, yHigh := axisEnds(vals.YLow, vals.YHigh)
	xAxisCell := &jsonschema.GridCellInput{
		ColSpan: 2,
		Grid:    buildMatrix2x2XAxis(vals.XAxisLabel, xLow, xHigh, labelSize, accent),
	}

	// Empty (unpainted) corner cell
	cornerCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`), // blank corner: the axes carry their own low/high cues
		},
	}

	// Y-axis label cell (row_span=2). The colored band stays an UNROTATED
	// narrow-tall rect; only the TEXT is rotated to read bottom-to-top via the
	// bodyPr vert="vert270" attribute. Rotating the whole cell (Rotation:270)
	// flips its width/height about its center, making the band render wide-short
	// and intrude into the quadrants (J2P-MATRIX-005); vert270 avoids that
	// because the fill geometry is never transformed.
	yAxisCell := &jsonschema.GridCellInput{
		RowSpan: 2,
		Grid:    buildMatrix2x2YAxis(vals.YAxisLabel, yLow, yHigh, labelSize, accent),
	}

	// Quadrants are built from gutters, not borders: a neutral 4% tint with
	// no outline, separated by the grid's white gutters, so the 2×2 reads as
	// one field split by a white cross (go-slide-creator-pgdkp).

	// Quadrant cells: cell index 0=TL, 1=TR, 2=BL, 3=BR
	quadrants := []Matrix2x2Quadrant{vals.TopLeft, vals.TopRight, vals.BottomLeft, vals.BottomRight}
	quadrantCells := make([]*jsonschema.GridCellInput, 4)
	for i, q := range quadrants {
		shape := &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     neutralFillJSON(NeutralTint4),
			Line:     noLine,
			Text:     buildMatrix2x2QuadrantContent(q, headerSize, bodySize, accent),
		}
		if q.Icon != nil {
			if icon := q.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "top"); icon != nil {
				shape.Icon = icon
			}
		}
		quadrantCells[i] = &jsonschema.GridCellInput{
			Shape: shape,
		}
		applyMatrix2x2CellOverride(quadrantCells[i], cellOverrides, i, accent)
	}

	rows := []jsonschema.GridRowInput{
		// Row 0: corner + x-axis label
		{
			Height: 16,
			Cells:  []*jsonschema.GridCellInput{cornerCell, xAxisCell},
		},
		// Row 1: y-axis label + TL + TR
		{
			Cells: []*jsonschema.GridCellInput{yAxisCell, quadrantCells[0], quadrantCells[1]},
		},
		// Row 2: BL + BR (y-axis spans from row 1, so only 2 cells)
		{
			Cells: []*jsonschema.GridCellInput{quadrantCells[2], quadrantCells[3]},
		},
	}

	// Quadrant rows share the height equally unless one pair's written fit
	// needs more than half; then each is floored at its need and they share
	// the slack in proportion (go-slide-creator-n1muf).
	floorFlexRowsAtNeeds(rows[1:], lay.needs[:], lay.availPt)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[12, 44, 44]`),
		Gap:     ctx.Gap(matrix2x2GapPt),
		Rows:    rows,
	}

	return grid, nil
}

const (
	// matrix2x2GapPt is the grid gap between the axis column, the axis row
	// and the quadrants.
	matrix2x2GapPt = 6.0
	// matrix2x2AxisRowPct is the x-axis row's share of the grid height.
	matrix2x2AxisRowPct = 16.0
	// matrix2x2MinHeaderPt is the floor the default quadrant header steps
	// down to before the quadrants report BODY_TOO_LONG.
	matrix2x2MinHeaderPt = 12.0
)

// matrix2x2Layout is the measured quadrant sizing: the header and body sizes,
// the written-fit height each quadrant row (top pair, bottom pair) needs, and
// the height the two rows share.
type matrix2x2Layout struct {
	headerSize, bodySize float64
	needs                [2]float64
	availPt              float64
	bars                 matrix2x2Bars // tonal style: the measured axis bars
}

// Open matrix geometry (go-slide-creator-jnkiq).
const (
	// matrix2x2AxisLinePt is the thickness of the two crossing axis lines.
	matrix2x2AxisLinePt = 1.0
	// matrix2x2OpenGapPt is the open grid's gap: the axis lines and the
	// quadrants touch, so the lines cross and a highlighted quadrant's tint
	// runs up to both axes.
	matrix2x2OpenGapPt = 0.01
	// matrix2x2YStripPct is the width of the y-axis strip (title and ends)
	// left of the matrix, the share the tile style's axis column takes.
	matrix2x2YStripPct = 12.0
)

// matrix2x2AxisLineTone is the ink of the crossing axis lines.
var matrix2x2AxisLineTone = fillTone{Color: "dk1", Alpha: 50}

// matrix2x2XStripPt is the height of the x-axis strip under the open matrix:
// the written fit of its title and end labels, plus the nested grid's inset.
func matrix2x2XStripPt(ctx ExpandContext, v *Matrix2x2Values, labelSize float64) float64 {
	areaW, _ := sizingAreaPt(ctx)
	stripW := areaW*(100-matrix2x2YStripPct)/100 - 2*SubGridInsetPt
	low, high := axisEnds(v.XLow, v.XHigh)
	endSize := math.Max(labelSize-3, 9)
	need := rowTextNeedPt(ctx.themeFonts(), buildMatrix2x2LabelContent(v.XAxisLabel, labelSize, "dk1", "ctr", ""), stripW*0.64)
	for _, end := range []string{low, high} {
		need = math.Max(need, rowTextNeedPt(ctx.themeFonts(), matrix2x2AxisEndText(end, endSize, "l", "ctr"), stripW*0.18))
	}
	return math.Ceil(need + 2*SubGridInsetPt)
}

// expandMatrix2x2Open draws the matrix as two crossing axis lines with open
// quadrants. Columns are [y strip | left | vertical axis | right]; rows are
// [top | horizontal axis | bottom | x strip]. The y strip (HIGH above the
// rotated title above LOW) runs the height of the vertical axis and the x
// strip (LOW, title, HIGH) the width of the horizontal one, so every title
// and end label sits along its own axis.
func expandMatrix2x2Open(ctx ExpandContext, vals *Matrix2x2Values, lay matrix2x2Layout, labelSize float64, accent string, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	areaW, _ := sizingAreaPt(ctx)
	linePct := matrix2x2AxisLinePt / areaW * 100
	quadPct := (100 - matrix2x2YStripPct - linePct) / 2

	quadrants := []Matrix2x2Quadrant{vals.TopLeft, vals.TopRight, vals.BottomLeft, vals.BottomRight}
	cells := make([]*jsonschema.GridCellInput, 4)
	for i, q := range quadrants {
		shape := &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine}
		headerInk, bodyInk := accentInkOnLight(ctx, accent, 4.5), "dk1"
		if q.Highlight {
			surface := inactiveTintTone(accent)
			shape.Fill = surface.fillJSON()
			headerInk = inkOnFill(ctx, accent, surface, 4.5)
			bodyInk = readableTextOn(ctx, surface, "dk1")
		}
		shape.Text = recolorTextInk(buildMatrix2x2QuadrantContent(q, lay.headerSize, lay.bodySize, headerInk), "dk1", bodyInk)
		if q.Icon != nil {
			if icon := q.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "top"); icon != nil {
				shape.Icon = icon
			}
		}
		cells[i] = &jsonschema.GridCellInput{Shape: shape}
		applyMatrix2x2CellOverride(cells[i], cellOverrides, i, accent)
	}

	line := func() *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: matrix2x2AxisLineTone.fillJSON(), Line: noLine}}
	}
	xLow, xHigh := axisEnds(vals.XLow, vals.XHigh)
	yLow, yHigh := axisEnds(vals.YLow, vals.YHigh)
	yStrip := &jsonschema.GridCellInput{RowSpan: 3, Grid: buildMatrix2x2YStrip(vals.YAxisLabel, yLow, yHigh, labelSize)}
	vAxis := line()
	vAxis.RowSpan = 3
	xStrip := &jsonschema.GridCellInput{ColSpan: 3, Grid: buildMatrix2x2XStrip(vals.XAxisLabel, xLow, xHigh, labelSize)}
	xStripPt := matrix2x2XStripPt(ctx, vals, labelSize)

	rows := []jsonschema.GridRowInput{
		{Cells: []*jsonschema.GridCellInput{yStrip, cells[0], vAxis, cells[1]}},
		{MinHeight: matrix2x2AxisLinePt, MaxHeight: matrix2x2AxisLinePt, Cells: []*jsonschema.GridCellInput{line(), line()}},
		{Cells: []*jsonschema.GridCellInput{cells[2], cells[3]}},
		{MinHeight: xStripPt, MaxHeight: xStripPt, Cells: []*jsonschema.GridCellInput{{}, xStrip}},
	}
	// The quadrant rows share the height equally unless one pair's written
	// fit needs more than half (go-slide-creator-n1muf).
	quadRows := []jsonschema.GridRowInput{rows[0], rows[2]}
	floorFlexRowsAtNeeds(quadRows, lay.needs[:], lay.availPt)
	rows[0], rows[2] = quadRows[0], quadRows[1]

	colsJSON, _ := json.Marshal([]float64{matrix2x2YStripPct, quadPct, linePct, quadPct})
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  matrix2x2OpenGapPt,
		RowGap:  matrix2x2OpenGapPt,
		Rows:    rows,
	}
}

// buildMatrix2x2XStrip is the x axis under the open matrix: [low | title |
// high], the ends under the two ends of the horizontal axis line.
func buildMatrix2x2XStrip(label, low, high string, size float64) *jsonschema.ShapeGridInput {
	endSize := math.Max(size-3, 9)
	title := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     buildMatrix2x2LabelContent(label, size, "dk1", "ctr", ""),
	}}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[18, 64, 18]`),
		ColGap:  4,
		RowGap:  0.01,
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			matrix2x2EndCell(low, endSize, "l", "ctr"),
			title,
			matrix2x2EndCell(high, endSize, "r", "ctr"),
		}}},
	}
}

// buildMatrix2x2YStrip is the y axis beside the open matrix: [high / title /
// low], the title reading bottom-to-top (vert270; no shape is rotated,
// J2P-MATRIX-005) and the ends level with the two ends of the vertical axis
// line.
func buildMatrix2x2YStrip(label, low, high string, size float64) *jsonschema.ShapeGridInput {
	endSize := math.Max(size-3, 9)
	title := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     buildMatrix2x2LabelContent(label, size, "dk1", "ctr", "vert270"),
	}}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		ColGap:  0.01,
		RowGap:  4,
		Rows: []jsonschema.GridRowInput{
			{Height: 12, Cells: []*jsonschema.GridCellInput{matrix2x2EndCell(high, endSize, "ctr", "t")}},
			{Height: 76, Cells: []*jsonschema.GridCellInput{title}},
			{Height: 12, Cells: []*jsonschema.GridCellInput{matrix2x2EndCell(low, endSize, "ctr", "b")}},
		},
	}
}

// layoutMatrix2x2 measures each quadrant cell as the writer will write it
// (writtenFitHeightPt at the quadrant's real width, plus the top icon zone)
// and steps the default 16pt header through 14pt to the 12pt floor when the
// two quadrant rows would not otherwise fit; an authored header_size is kept.
func layoutMatrix2x2(ctx ExpandContext, v *Matrix2x2Values, ovr *Matrix2x2Overrides) matrix2x2Layout {
	areaW, areaH := sizingAreaPt(ctx)
	quadW := (areaW - 2*ctx.Gap(matrix2x2GapPt)) * 0.44
	lay := matrix2x2Layout{
		bodySize: ResolveSize(ovr.BodySize, scaleBodyPt),
		availPt:  (areaH - 2*ctx.Gap(matrix2x2GapPt)) * (1 - matrix2x2AxisRowPct/100),
	}
	if matrix2x2Open(ovr) {
		// Open: the quadrants take everything beside the y strip and above
		// the content-sized x strip — more room than the tiles had.
		quadW = (areaW*(100-matrix2x2YStripPct)/100 - matrix2x2AxisLinePt) / 2
		lay.availPt = areaH - matrix2x2AxisLinePt - matrix2x2XStripPt(ctx, v, ResolveSize(ovr.LabelSize, scaleSubheadPt))
	}
	sizes := [][2]float64{{ResolveSize(ovr.HeaderSize, sizeHeaderPt), lay.bodySize}}
	if ovr.HeaderSize == 0 {
		sizes = append(sizes, [2]float64{scaleSubheadPt, lay.bodySize}, [2]float64{matrix2x2MinHeaderPt, lay.bodySize})
	}
	content := buildMatrix2x2QuadrantContent
	if matrix2x2Tonal(ovr) {
		lay.bars = layoutMatrix2x2Bars(ctx, v, ResolveSize(ovr.LabelSize, scaleSubheadPt))
		quadW, lay.availPt = lay.bars.quadW, lay.bars.availPt
		content = matrix2x2TonalContent
		if ovr.HeaderSize == 0 && ovr.BodySize == 0 {
			sizes = matrix2x2TonalScales
		}
	}
	pairs := [2][2]Matrix2x2Quadrant{{v.TopLeft, v.TopRight}, {v.BottomLeft, v.BottomRight}}
	for _, step := range sizes {
		size := step[0]
		lay.headerSize, lay.bodySize = size, step[1]
		for r, pair := range pairs {
			lay.needs[r] = 0
			for _, q := range pair {
				fit := writtenFitHeightPt(ctx.themeFonts(), content(q, size, lay.bodySize, "accent1"), quadW, 0)
				lay.needs[r] = math.Max(lay.needs[r], matrix2x2QuadrantNeedPt(fit, quadW, q.Icon != nil && !q.Icon.IsEmpty()))
			}
		}
		if lay.needs[0]+lay.needs[1] <= lay.availPt {
			break
		}
	}
	return lay
}

// matrix2x2QuadrantNeedPt is the quadrant height that leaves fitPt for its
// text below a top icon. The renderer sizes a top overlay at 0.6 × the
// shorter side, capped at 0.4 × the height on a landscape shape, plus a 3pt
// gap above and below (shapegrid iconOverlayBounds).
func matrix2x2QuadrantNeedPt(fitPt, widthPt float64, icon bool) float64 {
	if !icon {
		return fitPt
	}
	if landscape := (fitPt + 6) / (1 - 0.4); widthPt > landscape*1.2 {
		return math.Ceil(landscape)
	}
	return math.Ceil(fitPt + 0.6*widthPt + 6)
}

// buildMatrix2x2LabelContent creates a JSON text object for an axis label.
// When vert is non-empty (e.g. "vert270"), the text is rendered with that
// OOXML text-direction so a label can read vertically inside an unrotated band
// without rotating the band's fill geometry.
func buildMatrix2x2LabelContent(content string, size float64, color, align, vert string) json.RawMessage {
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
		Vert          string      `json:"vert,omitempty"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
		Vert:          vert,
	}
	data, _ := json.Marshal(textObj)
	return data
}

// buildMatrix2x2QuadrantContent creates a JSON text object for a quadrant cell
// with a bold header and optional body text.
func buildMatrix2x2QuadrantContent(q Matrix2x2Quadrant, headerSize, bodySize float64, accent string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	paras := []paragraph{
		{Content: q.Header, Size: headerSize, Bold: true, Color: accent, Align: "ctr"},
	}
	if q.Body != "" {
		paras = append(paras, paragraph{Content: q.Body, Size: bodySize, Color: "dk1", Align: "ctr"})
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// applyMatrix2x2CellOverride applies cell_overrides for a given quadrant cell index.
func applyMatrix2x2CellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*Matrix2x2CellOverride)
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

// matrix2x2XAxisMax fits even unbroken wide copy in the horizontal title box
// at >=12pt with visual clearance from the arrow on the narrowest shipped
// layout (abstract). The vertical axis retains its 60-character budget.
const matrix2x2XAxisMax = 16

// matrix2x2AxisEndMax bounds low/high labels for the narrow axis-end boxes.
// Eighteen characters rendered at 8.4pt after autofit in a 1.25in box; eleven
// keeps ordinary end labels at 12pt without narrowing the quadrants.
const matrix2x2AxisEndMax = 11

// axisEnds applies the "Low" / "High" defaults to the axis end labels.
func axisEnds(low, high string) (string, string) {
	if strings.TrimSpace(low) == "" {
		low = "Low"
	}
	if strings.TrimSpace(high) == "" {
		high = "High"
	}
	return low, high
}

// matrix2x2AxisEndText renders a small low/high end label next to an axis arrow.
func matrix2x2AxisEndText(content string, size float64, align, vAlign string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    []paragraph{{Content: content, Size: size, Color: "dk1", Align: align}},
		Align:         align,
		VerticalAlign: vAlign,
	}
	data, _ := json.Marshal(textObj)
	return data
}

func matrix2x2EndCell(content string, size float64, align, vAlign string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     matrix2x2AxisEndText(content, size, align, vAlign),
	}}
}

// Axis arrows are thin rules with a small arrowhead (go-slide-creator-7z5we):
// a 1.5pt shaft and an 8pt head, not a block arrow. The arrow shape is a
// preset rightArrow / upArrow held to matrix2x2AxisArrowPt across its shaft
// (a cell max_height for x, a narrow column for y), so adj1 (shaft share of
// that extent) and adj2 (head length, share of the same extent) give the
// point sizes below regardless of the template's content width.
const (
	matrix2x2AxisArrowPt = 8.0  // arrowhead width across the shaft
	matrix2x2AxisShaftPt = 1.5  // shaft (line) thickness
	matrix2x2AxisHeadLen = 0.75 // head length as a share of the head width
)

var matrix2x2ArrowAdj = map[string]int64{
	"adj1": int64(math.Round(matrix2x2AxisShaftPt / matrix2x2AxisArrowPt * 100000)),
	"adj2": int64(matrix2x2AxisHeadLen * 100000),
}

// buildMatrix2x2XAxis renders the x axis as [low | title -> | high], the arrow
// pointing right (towards "high"). Keeping title and arrow in separate cells
// prevents the shaft from occluding the title without making its text row too
// short for the stored autofit scale.
func buildMatrix2x2XAxis(label, low, high string, size float64, accent string) *jsonschema.ShapeGridInput {
	endSize := math.Max(size-3, 9)
	arrow := &jsonschema.GridCellInput{
		MaxHeight: matrix2x2AxisArrowPt,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry:    "rightArrow",
			Fill:        json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Adjustments: matrix2x2ArrowAdj,
		},
	}
	title := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     buildMatrix2x2LabelContent(label, size, "dk1", "ctr", ""),
	}}
	center := &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[70, 30]`),
		ColGap:  4,
		Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{title, arrow}}},
	}}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[18, 64, 18]`),
		ColGap:  4,
		RowGap:  0.01,
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			matrix2x2EndCell(low, endSize, "r", "ctr"),
			center,
			matrix2x2EndCell(high, endSize, "l", "ctr"),
		}}},
	}
}

// matrix2x2YArrowColPct is the y-axis arrow column's share of the axis cell:
// the axis column is 12% of the grid (~100pt on the shipped templates), so
// this share is an ~8pt-wide arrowhead column beside the rotated title.
const matrix2x2YArrowColPct = 9.0

// buildMatrix2x2YAxis renders the y axis as [high / label + up arrow / low], a
// thin arrow pointing up (towards "high") beside the label, which reads
// bottom-to-top. No shape is rotated (J2P-MATRIX-005); only the label's text
// direction is (vert270).
func buildMatrix2x2YAxis(label, low, high string, size float64, accent string) *jsonschema.ShapeGridInput {
	endSize := math.Max(size-3, 9)
	arrow := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry:    "upArrow",
		Fill:        json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
		Adjustments: matrix2x2ArrowAdj,
	}}
	title := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     buildMatrix2x2LabelContent(label, size, "dk1", "ctr", "vert270"),
	}}
	middle := &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`[%g, %g]`, 100-matrix2x2YArrowColPct, matrix2x2YArrowColPct)),
		ColGap:  2,
		Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{title, arrow}}},
	}}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		ColGap:  0.01,
		RowGap:  4,
		Rows: []jsonschema.GridRowInput{
			{Height: 12, Cells: []*jsonschema.GridCellInput{matrix2x2EndCell(high, endSize, "ctr", "b")}},
			{Height: 76, Cells: []*jsonschema.GridCellInput{middle}},
			{Height: 12, Cells: []*jsonschema.GridCellInput{matrix2x2EndCell(low, endSize, "ctr", "t")}},
		},
	}
}

package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	validator "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// consultingPattern shares the native shape canvas and strict schema gate of
// the bounded consulting visuals. Each visual supplies its own closed contract
// and geometry; text is measured at its written size before any shape is emitted.
type consultingPattern struct {
	name, description string
	valuesSchema      *Schema
	example           map[string]any
	draw              func(*consultingCanvas, map[string]any) error
	check             func(map[string]any) error
	once              sync.Once
	compiled          *validator.Schema
	schemaErr         error
}

func (p *consultingPattern) Name() string        { return p.name }
func (p *consultingPattern) Description() string { return p.description }
func (p *consultingPattern) UseWhen() string     { return consultingUseWhen[p.name] }
func (p *consultingPattern) NotWhen() string {
	return "Content does not match this visual's closed contract, or measured text cannot fit at the readable size; split the slide."
}
func (p *consultingPattern) Version() int         { return 1 }
func (p *consultingPattern) CellsHint() string    { return "bounded native shapes" }
func (p *consultingPattern) NewValues() any       { return &map[string]any{} }
func (p *consultingPattern) NewOverrides() any    { return &map[string]any{} }
func (p *consultingPattern) NewCellOverride() any { return nil }
func (p *consultingPattern) ExemplarValues() any {
	raw, err := json.Marshal(p.example)
	if err != nil {
		panic(err)
	}
	values := p.NewValues()
	if err := json.Unmarshal(raw, values); err != nil {
		panic(err)
	}
	return values
}
func (p *consultingPattern) Taxonomy() PatternTaxonomy {
	category := "structural"
	switch p.name {
	case "raci-matrix", "status-board", "progress-list", "numeric-heatmap":
		category = "data-display"
	case "prose-2col", "sources-list", "checklist", "image-hero":
		category = "narrative"
	}
	return PatternTaxonomy{Category: category, NarrativeRole: []string{"frame", "evidence"}, PairsWith: []string{"exec-summary", "chart-insights-split"}, DensityClass: "medium", AccentWeight: "subtle"}
}
func (p *consultingPattern) Schema() *Schema {
	return ObjectSchema(map[string]*Schema{"values": p.valuesSchema, "overrides": RefSchema("overrides")}, []string{"values"}).WithAdditionalProperties(false).AsRoot().WithDefs(map[string]*Schema{"overrides": ObjectSchema(nil, nil).WithAdditionalProperties(false)})
}
func (p *consultingPattern) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*map[string]any)
	if !ok || v == nil {
		return fmt.Errorf("%s: values must be an object", p.name)
	}
	if len(cellOverrides) > 0 {
		return fmt.Errorf("%s: cell_overrides are not supported", p.name)
	}
	p.once.Do(func() {
		raw, _ := json.Marshal(p.Schema())
		var doc any
		p.schemaErr = json.Unmarshal(raw, &doc)
		if p.schemaErr != nil {
			return
		}
		c := validator.NewCompiler()
		p.schemaErr = c.AddResource("https://json2pptx.local/"+p.name, doc)
		if p.schemaErr == nil {
			p.compiled, p.schemaErr = c.Compile("https://json2pptx.local/" + p.name)
		}
	})
	if p.schemaErr != nil {
		return p.schemaErr
	}
	obj := map[string]any{"values": *v}
	if overrides != nil {
		obj["overrides"] = overrides
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	var normalized any
	if err = json.Unmarshal(raw, &normalized); err != nil {
		return err
	}
	if err = p.compiled.Validate(normalized); err != nil {
		return &ValidationError{Pattern: p.name, Path: "values", Code: ErrCodeInvalidShape, Message: err.Error()}
	}
	if p.check != nil {
		return p.check(*v)
	}
	return nil
}
func (p *consultingPattern) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	if err := p.Validate(values, overrides, cellOverrides); err != nil {
		return nil, err
	}
	w, h := contentAreaPt(ctx)
	c := &consultingCanvas{ctx: ctx, name: p.name, w: w, h: h}
	typed, ok := values.(*map[string]any)
	if !ok || typed == nil {
		return nil, fmt.Errorf("%s: values must be an object", p.name)
	}
	if err := p.draw(c, *typed); err != nil {
		return nil, err
	}
	if c.diagram != nil {
		return &jsonschema.ShapeGridInput{Columns: json.RawMessage(`[100]`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Diagram: c.diagram}}}}}, nil
	}
	if c.table != nil {
		return c.tableGrid()
	}
	return &jsonschema.ShapeGridInput{Columns: json.RawMessage(`[100]`), VerticalAlign: "top", Rows: []jsonschema.GridRowInput{{MinHeight: h, MaxHeight: h, Cells: []*jsonschema.GridCellInput{{Layers: c.layers, Image: c.image}}}}}, nil
}

type consultingCanvas struct {
	ctx         ExpandContext
	name        string
	w, h        float64
	layers      []jsonschema.LayerInput
	diagram     *types.DiagramSpec
	image       *jsonschema.GridImageInput
	table       *jsonschema.TableInput
	tableLegend string
}

func (c *consultingCanvas) refuse(message string) error {
	return &ValidationError{Pattern: c.name, Path: "values", Code: ErrCodeBodyTooLong, Message: message + "; split the content or enlarge the frame; text is never clipped or reduced below its written readable size", Fix: &FixSuggestion{Kind: "split_slide"}}
}
func (c *consultingCanvas) shape(x, y, w, h float64, geometry, fill string, text json.RawMessage) error {
	if w <= 0 || h <= 0 || x < -0.01 || y < -0.01 || x+w > c.w+.01 || y+h > c.h+.01 {
		return c.refuse("native geometry does not fit the content frame")
	}
	c.layers = append(c.layers, jsonschema.LayerInput{Name: fmt.Sprintf("%s-%d", c.name, len(c.layers)), Frame: jsonschema.LayerFrameInput{X: x / c.w, Y: y / c.h, W: w / c.w, H: h / c.h}, Shape: &jsonschema.ShapeSpecInput{Geometry: geometry, Fill: visualRaw(fill), Text: text}})
	return nil
}
func (c *consultingCanvas) text(x, y, w, h float64, lines []visualParagraph) error {
	raw, _ := json.Marshal(map[string]any{"paragraphs": lines, "vertical_align": "ctr", "align": "l"})
	need := shapeTextHeightPt(c.ctx.Theme.BodyFont, raw, w-2*defaultShapeInsetLRPt) + 2*defaultShapeInsetTBPt
	if need > h+.1 {
		return c.refuse(fmt.Sprintf("text needs %.0fpt at its written readable size but only %.0fpt is allocated", need, h))
	}
	return c.shape(x, y, w, h, "rect", "none", raw)
}
func (c *consultingCanvas) rule(x, y, w, h float64, fill string) error {
	return c.shape(x, y, math.Max(.5, w), math.Max(.5, h), "rect", fill, nil)
}

type visualParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color"`
}

func visualP(text string, size float64, bold bool) visualParagraph {
	return visualParagraph{Content: text, Size: size, Bold: bold, Color: "dk1"}
}
func visualRaw(s string) json.RawMessage { b, _ := json.Marshal(s); return b }
func visualStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}
func visualNum(m map[string]any, key string) float64 {
	switch n := m[key].(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
func visualItems(m map[string]any, key string) []any { v, _ := m[key].([]any); return v }
func visualObj(v any) map[string]any                 { m, _ := v.(map[string]any); return m }
func visualStrings(m map[string]any, key string) []string {
	var out []string
	for _, v := range visualItems(m, key) {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}
func visualObject(props map[string]*Schema, required ...string) *Schema {
	return ObjectSchema(props, required).WithAdditionalProperties(false)
}
func visualString(max int) *Schema                   { s := StringSchema(max); s.raw.MinLength = intPtr(1); return s }
func visualArray(item *Schema, min, max int) *Schema { return ArraySchema(item, min, max) }

func (c *consultingCanvas) tableGrid() (*jsonschema.ShapeGridInput, error) {
	c.table.MinimumFontSize = 1200
	widths := types.ResolveTableColumnWidths(c.table.ColumnWidths, len(c.table.Headers), int64(c.w*12700))
	if widths == nil {
		return nil, c.refuse("matrix column weights are invalid")
	}
	rows := [][]string{c.table.Headers}
	for _, row := range c.table.Rows {
		var texts []string
		for _, cell := range row {
			texts = append(texts, cell.Content)
		}
		rows = append(rows, texts)
	}
	need := 0.0
	for _, row := range rows {
		rowHeight := 12*1.2 + 7.2 // generator.compactRowHeight: 1.2 line height plus both cell margins
		for i, text := range row {
			m, err := textfit.MeasureRun(text, c.ctx.Theme.BodyFont, 12, widths[i]-91440, 0)
			if err != nil {
				return nil, err
			}
			rowHeight = max(rowHeight, float64(m.RequiredEMU)/12700+3.6)
		}
		need += rowHeight
	}
	legendH := 44.0
	gap := c.ctx.Gap(8)
	if need+legendH+gap > c.h {
		return nil, c.refuse(fmt.Sprintf("matrix needs %.0fpt at 12pt but the frame holds %.0fpt", need+legendH+gap, c.h))
	}
	legend, _ := json.Marshal(map[string]any{"paragraphs": []visualParagraph{visualP(c.tableLegend, 10, false)}, "vertical_align": "ctr"})
	return &jsonschema.ShapeGridInput{Gap: gap, Source: "pattern:" + c.name, VerifiedMatrixPattern: c.name, Columns: json.RawMessage(`[100]`), VerticalAlign: "top", Rows: []jsonschema.GridRowInput{{MinHeight: need, MaxHeight: need, Cells: []*jsonschema.GridCellInput{{Table: c.table}}}, {MinHeight: legendH, MaxHeight: legendH, Cells: []*jsonschema.GridCellInput{{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: visualRaw("none"), Text: legend}}}}}}, nil
}

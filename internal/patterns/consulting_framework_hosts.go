package patterns

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func init() {
	for _, p := range consultingFrameworkHosts() {
		Default().Register(p)
	}
}
func consultingFrameworkHosts() []*consultingPattern {
	list := func() *Schema { return visualArray(visualString(100), 1, 5) }
	circle := visualObject(map[string]*Schema{"label": visualString(50), "items": list()}, "label", "items")
	cause := visualObject(map[string]*Schema{"name": visualString(50), "causes": list()}, "name", "causes")
	employee := visualObject(map[string]*Schema{"name": visualString(50), "performance": EnumSchema("low", "medium", "high"), "potential": EnumSchema("low", "medium", "high")}, "name", "performance", "potential")
	pestelProps := map[string]*Schema{}
	pestelEx := map[string]any{}
	keys := []string{"political", "economic", "social", "technological", "environmental", "legal"}
	for _, key := range keys {
		pestelProps[key] = list()
		pestelEx[key] = []any{"Assess implications"}
	}
	specs := []struct {
		name, typ, desc string
		schema          *Schema
		example         map[string]any
		check           func(map[string]any) error
	}{
		{"venn-framework", "venn", "Two or three overlapping sets with explicit item lists", visualObject(map[string]*Schema{"circles": visualArray(circle, 2, 3)}, "circles"), map[string]any{"circles": []any{map[string]any{"label": "Customer needs", "items": []any{"Speed"}}, map[string]any{"label": "Our strengths", "items": []any{"Expertise"}}}}, nil},
		{"fishbone-framework", "fishbone", "Cause-and-effect analysis with 2–6 named cause categories", visualObject(map[string]*Schema{"effect": visualString(120), "categories": visualArray(cause, 2, 6)}, "effect", "categories"), map[string]any{"effect": "Delivery delays", "categories": []any{map[string]any{"name": "People", "causes": []any{"Capacity"}}, map[string]any{"name": "Process", "causes": []any{"Handoffs"}}}}, nil},
		{"pestel-framework", "pestel", "All six PESTEL dimensions with 1–5 explicit implications each", visualObject(pestelProps, keys...), pestelEx, nil},
		{"talent-nine-box", "nine_box_talent", "Talent portfolio with explicit low/medium/high performance and potential", visualObject(map[string]*Schema{"employees": visualArray(employee, 1, 12)}, "employees"), map[string]any{"employees": []any{map[string]any{"name": "Team A", "performance": "high", "potential": "high"}, map[string]any{"name": "Team B", "performance": "medium", "potential": "high"}}}, nil},
		{"numeric-heatmap", "heatmap", "Labeled rectangular numeric heatmap with at most 8 rows and columns", visualObject(map[string]*Schema{"row_labels": visualArray(visualString(40), 1, 8), "col_labels": visualArray(visualString(40), 1, 8), "values": visualArray(visualArray(NumberSchema(0, 0), 1, 8), 1, 8)}, "row_labels", "col_labels", "values"), map[string]any{"row_labels": []any{"North", "South"}, "col_labels": []any{"Q1", "Q2"}, "values": []any{[]any{2.0, 4.0}, []any{3.0, 5.0}}}, checkConsultingHeatmap},
	}
	out := make([]*consultingPattern, 0, len(specs))
	for _, s := range specs {
		typ := s.typ
		out = append(out, &consultingPattern{name: s.name, description: s.desc, valuesSchema: s.schema, example: s.example, check: s.check, draw: func(c *consultingCanvas, v map[string]any) error {
			if typ == "heatmap" {
				return drawConsultingHeatmap(c, v)
			}
			if typ == "fishbone" || typ == "venn" {
				if err := measureConsultingSVG(c, typ, v); err != nil {
					return err
				}
			}
			c.diagram = &types.DiagramSpec{Type: typ, Data: v}
			return nil
		}})
	}
	return out
}
func checkConsultingHeatmap(v map[string]any) error {
	rows := visualItems(v, "values")
	if len(rows) != len(visualItems(v, "row_labels")) {
		return fmt.Errorf("heatmap needs one row label per numeric row")
	}
	cols := len(visualItems(v, "col_labels"))
	for _, raw := range rows {
		row, ok := raw.([]any)
		if !ok || len(row) != cols {
			return fmt.Errorf("heatmap rows must match column labels")
		}
	}
	return nil
}

// Consult the actual SVG layout rather than inventing a character-count cap.
// Legacy raw diagrams may abbreviate labels; these closed consulting hosts
// promise complete authored content and therefore refuse those placements.
func measureConsultingSVG(c *consultingCanvas, typ string, v map[string]any) error {
	findings, err := svggen.DryRender(&svggen.RequestEnvelope{Type: typ, Data: v, Output: svggen.OutputSpec{Width: int(c.w), Height: int(c.h), StrictFit: "strict"}})
	if err != nil {
		return c.refuse("diagram layout cannot fit: " + err.Error())
	}
	for _, f := range findings {
		switch f.Code {
		case svggen.FindingLabelTruncated, svggen.FindingLabelEllipsized, svggen.FindingLabelClipped, svggen.FindingOverflowSuppressed, svggen.FindingDiagramTextOverlap, svggen.FindingDiagramRegionOverflow:
			return c.refuse("diagram content cannot be shown in full: " + f.Message)
		}
	}
	return nil
}

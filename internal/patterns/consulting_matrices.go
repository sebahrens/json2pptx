package patterns

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

func init() {
	for _, p := range consultingMatrixPatterns() {
		Default().Register(p)
	}
}
func consultingMatrixPatterns() []*consultingPattern {
	raciRow := visualObject(map[string]*Schema{"activity": visualString(70), "scores": visualArray(EnumSchema("R", "A", "C", "I", "-"), 2, 6)}, "activity", "scores")
	statusRow := visualObject(map[string]*Schema{"workstream": visualString(70), "statuses": visualArray(EnumSchema("red", "amber", "green"), 2, 5), "trend": EnumSchema("up", "flat", "down"), "comment": StringSchema(120)}, "workstream", "statuses")
	colors := visualObject(map[string]*Schema{"red": StringSchema(16), "amber": StringSchema(16), "green": StringSchema(16)})
	return []*consultingPattern{
		{name: "raci-matrix", description: "3–10 activities × 2–6 roles, R/A/C/I responsibilities, exactly one accountable role per activity, and a letter legend", valuesSchema: visualObject(map[string]*Schema{"roles": visualArray(visualString(28), 2, 6), "activities": visualArray(raciRow, 3, 10)}, "roles", "activities"), example: map[string]any{"roles": []any{"Lead", "Sponsor", "Team"}, "activities": []any{map[string]any{"activity": "Agree scope", "scores": []any{"R", "A", "C"}}, map[string]any{"activity": "Deliver", "scores": []any{"A", "I", "R"}}, map[string]any{"activity": "Approve", "scores": []any{"R", "A", "I"}}}}, check: checkRACI, draw: drawRACI},
		{name: "status-board", description: "3–10 workstreams × 2–5 RAG status columns, optional up/flat/down trend and comment, with measured status ink", valuesSchema: visualObject(map[string]*Schema{"columns": visualArray(visualString(28), 2, 5), "rows": visualArray(statusRow, 3, 10), "rag_colors": colors}, "columns", "rows"), example: map[string]any{"columns": []any{"Scope", "Delivery"}, "rows": []any{map[string]any{"workstream": "Platform", "statuses": []any{"green", "amber"}, "trend": "up", "comment": "Test next week"}, map[string]any{"workstream": "Operations", "statuses": []any{"amber", "green"}, "trend": "flat"}, map[string]any{"workstream": "Controls", "statuses": []any{"red", "amber"}, "trend": "down", "comment": "Owner needed"}}}, check: checkStatusBoard, draw: drawStatusBoard},
	}
}
func checkRACI(v map[string]any) error {
	roles := visualItems(v, "roles")
	for i, raw := range visualItems(v, "activities") {
		scores := visualStrings(visualObj(raw), "scores")
		if len(scores) != len(roles) {
			return fmt.Errorf("raci-matrix activities[%d].scores: requires one responsibility per role", i)
		}
		accountable := 0
		for _, score := range scores {
			if score == "A" {
				accountable++
			}
		}
		if accountable != 1 {
			return fmt.Errorf("raci-matrix activities[%d].scores: requires exactly one A, got %d", i, accountable)
		}
	}
	return nil
}
func drawRACI(c *consultingCanvas, v map[string]any) error {
	roles := visualStrings(v, "roles")
	headers := append([]string{"Activity"}, roles...)
	table := &jsonschema.TableInput{Headers: headers, ColumnWidths: []float64{2.2}, ColumnAlignments: []string{"left"}}
	for range roles {
		table.ColumnWidths = append(table.ColumnWidths, 1)
		table.ColumnAlignments = append(table.ColumnAlignments, "center")
	}
	for _, raw := range visualItems(v, "activities") {
		row := visualObj(raw)
		cells := []jsonschema.TableCellInput{{Content: visualStr(row, "activity")}}
		for _, score := range visualStrings(row, "scores") {
			cells = append(cells, jsonschema.TableCellInput{Content: score})
		}
		table.Rows = append(table.Rows, cells)
	}
	c.table = table
	c.tableLegend = "R Responsible   A Accountable (one per row)   C Consulted   I Informed   – No role"
	return nil
}
func checkStatusBoard(v map[string]any) error {
	for i, raw := range visualItems(v, "rows") {
		if len(visualItems(visualObj(raw), "statuses")) != len(visualItems(v, "columns")) {
			return fmt.Errorf("status-board rows[%d].statuses requires one status per column", i)
		}
	}
	for key, raw := range visualObj(v["rag_colors"]) {
		s, _ := raw.(string)
		if !isCardGridColor(s) {
			return fmt.Errorf("status-board rag_colors.%s must be a theme reference or hex colour", key)
		}
	}
	return nil
}
func drawStatusBoard(c *consultingCanvas, v map[string]any) error {
	columns := visualStrings(v, "columns")
	table := &jsonschema.TableInput{Headers: append([]string{"Workstream"}, columns...), ColumnWidths: []float64{2}, ColumnAlignments: []string{"left"}}
	for range columns {
		table.ColumnWidths = append(table.ColumnWidths, 1)
		table.ColumnAlignments = append(table.ColumnAlignments, "center")
	}
	trend, comment := false, false
	for _, raw := range visualItems(v, "rows") {
		r := visualObj(raw)
		trend = trend || visualStr(r, "trend") != ""
		comment = comment || visualStr(r, "comment") != ""
	}
	if trend {
		table.Headers = append(table.Headers, "Trend")
		table.ColumnWidths = append(table.ColumnWidths, .8)
		table.ColumnAlignments = append(table.ColumnAlignments, "center")
	}
	if comment {
		table.Headers = append(table.Headers, "Comment")
		table.ColumnWidths = append(table.ColumnWidths, 2.5)
		table.ColumnAlignments = append(table.ColumnAlignments, "left")
	}
	for _, raw := range visualItems(v, "rows") {
		row := visualObj(raw)
		cells := []jsonschema.TableCellInput{{Content: visualStr(row, "workstream")}}
		for _, status := range visualStrings(row, "statuses") {
			ink, err := consultingStatusInk(c.ctx, status, visualObj(v["rag_colors"]))
			if err != nil {
				return err
			}
			cells = append(cells, jsonschema.TableCellInput{Content: "●", TextColor: ink})
		}
		if trend {
			symbol := map[string]string{"up": "↑", "flat": "→", "down": "↓"}[visualStr(row, "trend")]
			cells = append(cells, jsonschema.TableCellInput{Content: symbol})
		}
		if comment {
			cells = append(cells, jsonschema.TableCellInput{Content: visualStr(row, "comment")})
		}
		table.Rows = append(table.Rows, cells)
	}
	c.table = table
	c.tableLegend = "● Red: off track   ● Amber: at risk   ● Green: on track   ↑ Improving  → Stable  ↓ Deteriorating"
	return nil
}
func consultingStatusInk(ctx ExpandContext, status string, overrides map[string]any) (string, error) {
	palette := map[string]string{"red": "#B71C1C", "amber": "#946200", "green": "#2E7D32"}
	color := palette[status]
	if raw, ok := overrides[status].(string); ok && raw != "" {
		color = raw
	}
	ink, err := svggen.ParseColor(thHex(ctx, color, palette[status]))
	if err != nil {
		return "", err
	}
	paper, err := svggen.ParseColor(thHex(ctx, "lt1", "#FFFFFF"))
	if err != nil {
		return "", err
	}
	if ink.ContrastWith(paper) < 3 {
		return "", fmt.Errorf("status-board rag_colors.%s (%s) does not clear 3:1 against the table paper; choose a darker distinguishable status colour", status, color)
	}
	return "#" + strings.TrimPrefix(ink.Hex(), "#"), nil
}

package patterns

import (
	"fmt"
	"strings"
)

func init() {
	for _, p := range consultingListPatterns() {
		Default().Register(p)
	}
}
func consultingListPatterns() []*consultingPattern {
	item := visualObject(map[string]*Schema{"label": visualString(120), "state": EnumSchema("done", "open", "blocked"), "detail": StringSchema(180)}, "label", "state")
	column := visualObject(map[string]*Schema{"heading": StringSchema(60), "paragraphs": visualArray(visualString(700), 1, 4)}, "paragraphs")
	box := visualObject(map[string]*Schema{"heading": visualString(60), "bullets": visualArray(visualString(180), 2, 5)}, "heading", "bullets")
	horizon := visualObject(map[string]*Schema{"title": visualString(60), "timeframe": visualString(40), "items": visualArray(visualString(140), 1, 4)}, "title", "timeframe", "items")
	source := visualObject(map[string]*Schema{"text": visualString(220), "url": StringSchema(300)}, "text")
	return []*consultingPattern{
		{name: "checklist", description: "3–10 explicit done/open/blocked requirements with optional detail, at 12pt or refused when the frame is too short", valuesSchema: visualObject(map[string]*Schema{"items": visualArray(item, 3, 10)}, "items"), example: map[string]any{"items": []any{map[string]any{"label": "Owners agreed", "state": "done"}, map[string]any{"label": "Test results", "state": "open"}, map[string]any{"label": "Approval", "state": "blocked"}}}, draw: drawConsultingChecklist},
		{name: "prose-2col", description: "Two balanced columns of 1–4 running prose paragraphs, with optional headings and a measured 12pt floor", valuesSchema: visualObject(map[string]*Schema{"columns": visualArray(column, 2, 2)}, "columns"), example: map[string]any{"columns": []any{map[string]any{"heading": "What changed", "paragraphs": []any{"Demand has shifted toward managed services."}}, map[string]any{"heading": "What follows", "paragraphs": []any{"Build delivery capacity before adding new markets."}}}}, draw: drawConsultingProse},
		{name: "ipo-flow", description: "Input → process → output: exactly three headed boxes, each with 2–5 bullets and native arrow connectors", valuesSchema: visualObject(map[string]*Schema{"boxes": visualArray(box, 3, 3)}, "boxes"), example: map[string]any{"boxes": []any{map[string]any{"heading": "Input", "bullets": []any{"Requests", "Evidence"}}, map[string]any{"heading": "Process", "bullets": []any{"Assess", "Decide"}}, map[string]any{"heading": "Output", "bullets": []any{"Owner", "Next action"}}}}, draw: drawConsultingIPO},
		{name: "three-horizons", description: "Three ascending strategy bands, each with title, timeframe and 1–4 initiatives", valuesSchema: visualObject(map[string]*Schema{"horizons": visualArray(horizon, 3, 3)}, "horizons"), example: map[string]any{"horizons": []any{map[string]any{"title": "Horizon 1", "timeframe": "Now", "items": []any{"Protect the core"}}, map[string]any{"title": "Horizon 2", "timeframe": "Next", "items": []any{"Scale adjacencies"}}, map[string]any{"title": "Horizon 3", "timeframe": "Future", "items": []any{"Create new engines"}}}}, draw: drawConsultingHorizons},
		{name: "sources-list", description: "4–16 numbered appendix sources with optional URLs, measured at 10pt", valuesSchema: visualObject(map[string]*Schema{"entries": visualArray(source, 4, 16)}, "entries"), example: map[string]any{"entries": []any{map[string]any{"text": "Management interviews"}, map[string]any{"text": "Finance plan, FY26"}, map[string]any{"text": "Customer survey"}, map[string]any{"text": "Market statistics", "url": "https://example.org/research"}}}, draw: drawConsultingSources},
	}
}
func drawConsultingChecklist(c *consultingCanvas, v map[string]any) error {
	items := visualItems(v, "items")
	columns := 1
	if len(items) > 6 {
		columns = 2
	}
	rows := (len(items) + columns - 1) / columns
	rowH := c.h / float64(rows)
	width := c.w
	if columns == 2 {
		width = (c.w - 20) / 2
	}
	colors := map[string]string{"done": "#2E7D32", "open": "#6B7280", "blocked": "#B71C1C"}
	for i, raw := range items {
		item := visualObj(raw)
		row, column := i%rows, i/rows
		x := float64(column) * (width + 20)
		y := float64(row) * rowH
		state := visualStr(item, "state")
		diam := 10.0
		if err := c.shape(x+5, y+rowH/2-diam/2, diam, diam, "ellipse", colors[state], nil); err != nil {
			return err
		}
		lines := []visualParagraph{visualP(visualStr(item, "label")+" — "+state, 12, true)}
		if detail := visualStr(item, "detail"); detail != "" {
			lines = append(lines, visualP(detail, 12, false))
		}
		if err := c.text(x+20, y, width-20, rowH, lines); err != nil {
			return err
		}
		if row > 0 {
			if err := c.rule(x+20, y, width-20, .5, "dk1"); err != nil {
				return err
			}
		}
	}
	return nil
}
func drawConsultingProse(c *consultingCanvas, v map[string]any) error {
	cols := visualItems(v, "columns")
	gap := 20.0
	width := (c.w - gap) / 2
	for i, raw := range cols {
		col := visualObj(raw)
		var lines []visualParagraph
		if heading := visualStr(col, "heading"); heading != "" {
			lines = append(lines, visualP(heading, 14, true))
		}
		for _, para := range visualStrings(col, "paragraphs") {
			lines = append(lines, visualP(para, 12, false))
		}
		if err := c.text(float64(i)*(width+gap), 0, width, c.h, lines); err != nil {
			return err
		}
	}
	return nil
}
func consultingBullets(items []string) []visualParagraph {
	var out []visualParagraph
	for _, item := range items {
		out = append(out, visualP("• "+item, 12, false))
	}
	return out
}
func drawConsultingIPO(c *consultingCanvas, v map[string]any) error {
	gap := 24.0
	width := (c.w - 2*gap) / 3
	for i, raw := range visualItems(v, "boxes") {
		box := visualObj(raw)
		x := float64(i) * (width + gap)
		if err := c.shape(x, 0, width, c.h, "rect", "lt2", nil); err != nil {
			return err
		}
		lines := append([]visualParagraph{visualP(visualStr(box, "heading"), 14, true)}, consultingBullets(visualStrings(box, "bullets"))...)
		if err := c.text(x, 0, width, c.h, lines); err != nil {
			return err
		}
		if i < 2 {
			if err := c.shape(x+width+3, c.h/2-7, gap-6, 14, "rightArrow", c.ctx.DefaultAccent(), nil); err != nil {
				return err
			}
		}
	}
	return nil
}
func drawConsultingHorizons(c *consultingCanvas, v map[string]any) error {
	width := c.w / 3
	for i, raw := range visualItems(v, "horizons") {
		h := visualObj(raw)
		x := float64(i) * width
		y := float64(2-i) * c.h * .10
		if err := c.shape(x, y, width-8, c.h-y, "rect", "lt2", nil); err != nil {
			return err
		}
		if err := c.rule(x, y, width-8, 4, c.ctx.DefaultAccent()); err != nil {
			return err
		}
		lines := []visualParagraph{visualP(visualStr(h, "title"), 14, true), visualP(visualStr(h, "timeframe"), 12, true)}
		lines = append(lines, consultingBullets(visualStrings(h, "items"))...)
		if err := c.text(x, y+5, width-8, c.h-y-5, lines); err != nil {
			return err
		}
	}
	return nil
}
func drawConsultingSources(c *consultingCanvas, v map[string]any) error {
	var lines []visualParagraph
	for i, raw := range visualItems(v, "entries") {
		entry := visualObj(raw)
		text := fmt.Sprintf("%d. %s", i+1, visualStr(entry, "text"))
		if url := visualStr(entry, "url"); url != "" {
			text += " — " + url
		}
		lines = append(lines, visualP(strings.TrimSpace(text), 10, false))
	}
	return c.text(0, 0, c.w, c.h, lines)
}

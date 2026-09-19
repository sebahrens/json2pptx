package slides

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// table -> a native table content slide (go-slide-creator-e4h1).
//
// A plain table — the financials, the segment split, the pricing tiers — had no
// kind. The DeckSpec path could express an options × criteria matrix only as
// prose, and capability-gaps probing `kind: table` got "unknown slide kind", so
// the most ordinary business slide there is needed the raw escape hatch.

// TableMaxColumns is the widest table the renderer lays out before the density
// rules complain. Validation shares it so an author hears about a too-wide
// table before the render, not after.
const TableMaxColumns = jsonschema.TDRMaxCols

// TableMaxRows is the tallest table (header included) before the same applies.
const TableMaxRows = jsonschema.TDRMaxRows

// CompileTable compiles a table slide to a native table content block: headers,
// rows, optional per-column alignment, an optional highlighted column, and an
// optional totals row. The engine's table renderer does the styling from the
// template's own table style, so nothing here hardcodes a colour.
func CompileTable(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	headers := tableHeaders(in.Body)
	rows := tableRows(in.Body, len(headers))
	if len(headers) == 0 || len(rows) == 0 {
		// Without a header row there is no table to render; the rows still say
		// something, so they degrade to bullets rather than vanishing.
		return compileTableFallback(in, headers, rows)
	}

	slide := &deckinput.SlideInput{SlideType: "content"}
	links := titleLink(slide, in)

	table := &jsonschema.TableInput{Headers: headers, Rows: rows, Alt: visualAltText(in)}
	if alignments := tableColumnAlignments(in.Body, len(headers)); len(alignments) > 0 {
		table.ColumnAlignments = alignments
	}
	if style := tableStyle(in.Body, headers); style != nil {
		table.Style = style
	}

	idx := appendContent(slide, deckinput.ContentInput{
		PlaceholderID: "body",
		Type:          "table",
		TableValue:    table,
	})
	links = append(links,
		SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].table_value.headers", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + tableHeadersField(in.Body),
		},
		SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].table_value.rows", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".rows",
		},
	)

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// TableFeasible reports whether a table payload will render as a table rather
// than degrade to bullets: it needs a header row and at least one data row.
func TableFeasible(body map[string]any) bool {
	headers := tableHeaders(body)
	return len(headers) > 0 && len(tableRows(body, len(headers))) > 0
}

// UsableTableCounts returns the header and data-row counts that survive
// extraction, so validation counts what compile will render.
func UsableTableCounts(body map[string]any) (columns, rows int) {
	headers := tableHeaders(body)
	return len(headers), len(tableRows(body, len(headers)))
}

// tableHeadersField names the payload key the header row came from.
func tableHeadersField(body map[string]any) string {
	if _, ok := body["headers"].([]any); ok {
		return "headers"
	}
	return "columns"
}

// tableHeaders extracts the header row.
func tableHeaders(body map[string]any) []string {
	for _, field := range []string{"headers", "columns"} {
		if headers, ok := stringList(body, field); ok && len(headers) > 0 {
			return headers
		}
	}
	return nil
}

// tableRows extracts the data rows. A row is a list of cells, or an object
// keyed by header label — the shape an author reaches for when the columns are
// named. Short rows are padded and long ones are NOT truncated: the renderer
// squares the grid, and silently dropping a cell is worse than a ragged table.
func tableRows(body map[string]any, columns int) [][]jsonschema.TableCellInput {
	raw, ok := body["rows"].([]any)
	if !ok {
		return nil
	}
	headers := tableHeaders(body)
	out := make([][]jsonschema.TableCellInput, 0, len(raw))
	for _, entry := range raw {
		switch t := entry.(type) {
		case []any:
			cells := make([]jsonschema.TableCellInput, 0, len(t))
			for _, cell := range t {
				cells = append(cells, jsonschema.TableCellInput{Content: tableCellText(cell)})
			}
			out = append(out, padTableRow(cells, columns))
		case map[string]any:
			// An object row addresses its cells by header label; a "cells"/"values"
			// list inside one is the list form with a label attached.
			if list, ok := t["cells"].([]any); ok {
				out = append(out, tableRowFromList(list, columns))
				continue
			}
			if list, ok := t["values"].([]any); ok {
				out = append(out, tableRowFromList(list, columns))
				continue
			}
			cells := make([]jsonschema.TableCellInput, 0, len(headers))
			for _, h := range headers {
				cells = append(cells, jsonschema.TableCellInput{Content: tableCellText(t[h])})
			}
			out = append(out, cells)
		case string:
			// A single-column table's row may be a bare string.
			out = append(out, padTableRow([]jsonschema.TableCellInput{{Content: strings.TrimSpace(t)}}, columns))
		}
	}
	return out
}

// tableRowFromList builds a padded row from a list of cell values.
func tableRowFromList(list []any, columns int) []jsonschema.TableCellInput {
	cells := make([]jsonschema.TableCellInput, 0, len(list))
	for _, cell := range list {
		cells = append(cells, jsonschema.TableCellInput{Content: tableCellText(cell)})
	}
	return padTableRow(cells, columns)
}

// padTableRow pads a short row with empty cells so the grid is square.
func padTableRow(cells []jsonschema.TableCellInput, columns int) []jsonschema.TableCellInput {
	for len(cells) < columns {
		cells = append(cells, jsonschema.TableCellInput{})
	}
	return cells
}

// tableCellText renders one cell value. Numbers keep the author's own literal
// rather than picking up %v's exponent form, and a nil cell is empty, not "-".
func tableCellText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		return strconv.FormatBool(t)
	case map[string]any:
		return firstNonEmpty(strField(t, "content"), strField(t, "text"), strField(t, "value"), strField(t, "label"))
	}
	return fmt.Sprintf("%v", v)
}

// tableColumnAlignments resolves per-column alignment to the words the table
// renderer reads ("left" / "center" / "right"), accepting both those words and
// the shape-grid codes an author may carry over from a shape_grid text block.
func tableColumnAlignments(body map[string]any, columns int) []string {
	values, ok := stringList(body, "column_alignments")
	if !ok || len(values) == 0 {
		return nil
	}
	out := make([]string, 0, columns)
	for i := 0; i < columns; i++ {
		if i >= len(values) {
			out = append(out, "left")
			continue
		}
		out = append(out, tableAlignment(values[i]))
	}
	return out
}

// tableAlignment normalizes an alignment word. The renderer's own vocabulary is
// "left" / "center" / "right" (generator.alignmentToOOXML); anything else falls
// back to left rather than silently producing a code the renderer ignores.
func tableAlignment(word string) string {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "r", "right", "end":
		return "right"
	case "c", "ctr", "center", "centre", "middle":
		return "center"
	default:
		return "left"
	}
}

// tableStyle builds the table's style block from the payload's emphasis fields:
// a highlighted column (by header name or index) and a totals row.
func tableStyle(body map[string]any, headers []string) *jsonschema.TableStyleInput {
	style := &jsonschema.TableStyleInput{}
	set := false

	if idx, ok := tableHighlightColumn(body, headers); ok {
		// The renderer's highlight_column is 1-based; 0 means "none".
		style.HighlightColumn = idx + 1
		set = true
	}
	if totals, ok := body["totals_row"].(bool); ok && totals {
		style.TotalsRow = true
		set = true
	}
	if types, ok := stringList(body, "column_types"); ok && len(types) > 0 {
		style.ColumnTypes = types
		set = true
	}
	if !set {
		return nil
	}
	return style
}

// tableHighlightColumn resolves the emphasised column by header label or index.
func tableHighlightColumn(body map[string]any, headers []string) (int, bool) {
	for _, field := range []string{"highlight_column", "highlight_col"} {
		v, ok := body[field]
		if !ok {
			continue
		}
		if idx, ok := optionMatrixIndexValue(v, len(headers)); ok {
			return idx, true
		}
		label, ok := v.(string)
		if !ok {
			continue
		}
		for i, h := range headers {
			if strings.EqualFold(strings.TrimSpace(label), h) {
				return i, true
			}
		}
	}
	return 0, false
}

// compileTableFallback renders the rows as bullets when there is no header row
// to build a table from, so the content still reaches the slide.
func compileTableFallback(in Input, headers []string, rows [][]jsonschema.TableCellInput) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(rows))
	for _, row := range rows {
		parts := make([]string, 0, len(row))
		for i, cell := range row {
			if cell.Content == "" {
				continue
			}
			if i < len(headers) && headers[i] != "" {
				parts = append(parts, headers[i]+": "+cell.Content)
				continue
			}
			parts = append(parts, cell.Content)
		}
		if len(parts) > 0 {
			bullets = append(bullets, strings.Join(parts, "; "))
		}
	}
	if len(bullets) == 0 {
		bullets = headers
	}
	return contentFallback(in, "rows", bullets)
}

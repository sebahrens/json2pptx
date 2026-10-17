package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func composeNativeLeafCount(s SegmentInput) int {
	n := 0
	for _, set := range []bool{s.ShapeGrid != nil, s.Table != nil, s.Text != nil, s.Image != nil} {
		if set {
			n++
		}
	}
	return n
}
func composeHasNativeLeaf(s SegmentInput) bool { return composeNativeLeafCount(s) > 0 }
func composeNativeLeaf(s SegmentInput) *jsonschema.ShapeGridInput {
	if s.ShapeGrid != nil {
		var clone jsonschema.ShapeGridInput
		raw, _ := json.Marshal(s.ShapeGrid)
		_ = json.Unmarshal(raw, &clone)
		if len(clone.Columns) == 0 {
			clone.Columns, _ = json.Marshal(inferColumnCount(&clone))
		}
		return &clone
	}
	cell := &jsonschema.GridCellInput{Table: s.Table, Image: s.Image}
	if s.Text != nil {
		paras := []map[string]any{}
		if strings.TrimSpace(s.Text.Body) != "" {
			paras = append(paras, map[string]any{"content": s.Text.Body, "size": 12, "align": "l"})
		}
		for _, b := range s.Text.Bullets {
			paras = append(paras, map[string]any{"content": b, "size": 12, "align": "l", "bullet": true})
		}
		text, _ := json.Marshal(map[string]any{"paragraphs": paras, "vertical_align": "top"})
		cell.Shape = &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: text}
	}
	return &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{cell}}}, VerticalAlign: "top"}
}
func validateComposeOptions(c *ComposeInput) error {
	if math.IsNaN(c.Gap) || math.IsInf(c.Gap, 0) || c.Gap < 0 {
		return fmt.Errorf("compose.gap must be finite and >=0")
	}
	if c.Gaps != nil && len(c.Gaps) != len(c.Segments)-1 {
		return fmt.Errorf("compose.gaps must have one value per segment seam")
	}
	for _, g := range c.Gaps {
		if math.IsNaN(g) || math.IsInf(g, 0) || g < 0 {
			return fmt.Errorf("compose.gaps must be finite and >=0")
		}
	}
	for i, s := range c.Segments {
		if err := validateComposeSegmentOptions(s, c.Direction, i); err != nil {
			return err
		}
	}

	return nil
}
func validateHorizontalSegmentRows(grids []*jsonschema.ShapeGridInput) error {
	for i, g := range grids {
		if g == nil {
			return fmt.Errorf("compose segment[%d] has no grid", i)
		}
		if err := validateComposeGridOccupancy(g); err != nil {
			return fmt.Errorf("COMPOSE_HORIZONTAL_TRUNCATION: segment[%d]: %w; split the pattern or reduce cells", i, err)
		}
	}
	return nil
}
func validateComposeGridOccupancy(g *jsonschema.ShapeGridInput) error {
	columns := inferColumnCount(g)
	occupied := make([][]bool, len(g.Rows))
	for r := range occupied {
		occupied[r] = make([]bool, columns)
	}
	for r, row := range g.Rows {
		col := 0
		for _, cell := range row.Cells {
			for col < columns && occupied[r][col] {
				col++
			}
			width, height := 1, 1
			if cell != nil {
				width = max(cell.ColSpan, 1)
				height = max(cell.RowSpan, 1)
			}
			if col+width > columns || r+height > len(g.Rows) {
				return fmt.Errorf("row[%d] cell at column%d exceeds its allocated lattice", r, col)
			}
			for y := r; y < r+height; y++ {
				for x := col; x < col+width; x++ {
					if occupied[y][x] {
						return fmt.Errorf("row[%d] cell overlaps a spanning cell", r)
					}
					occupied[y][x] = true
				}
			}
			col += width
		}
	}
	return nil
}

func invalidComposeShare(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < 0
}
func validateComposeSegmentOptions(s SegmentInput, direction string, i int) error {
	if s.Size != "" && s.Size != "content" {
		return fmt.Errorf("compose segment[%d].size must be content", i)
	}
	if s.Size == "content" && (direction != "vertical" || s.SizePct != 0) {
		return fmt.Errorf("compose segment[%d] size content requires vertical direction and no size_pct", i)
	}
	if len([]rune(s.Heading)) > 60 || len([]rune(s.Caption)) > 120 {
		return fmt.Errorf("compose segment[%d] heading/caption exceeds60/120 characters", i)
	}
	if s.Text != nil && strings.TrimSpace(s.Text.Body) == "" && len(s.Text.Bullets) == 0 {
		return fmt.Errorf("compose segment[%d].text requires body or bullets", i)
	}
	if s.Image != nil && s.Image.Path == "" && s.Image.URL == "" {
		return fmt.Errorf("compose segment[%d].image requires path or url", i)
	}
	if s.Table != nil {
		if len(s.Table.Headers) == 0 {
			return fmt.Errorf("compose segment[%d].table.headers requires at least one header", i)
		}
		if err := s.Table.ToTableSpec().CheckRowWidths(); err != nil {
			return fmt.Errorf("compose segment[%d].table: %w", i, err)
		}
	}
	return nil
}

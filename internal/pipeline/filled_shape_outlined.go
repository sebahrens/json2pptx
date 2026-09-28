package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// minFilledAlpha is the fill opacity (percent) at or above which a shape
// reads as a filled box. Below it the shape is a translucent wash and an
// outline is what makes it visible at all.
const minFilledAlpha = 50

// detectFilledShapeOutlined flags every authored shape that has both an
// opaque fill and a visible outline (go-slide-creator-pgdkp). A stroke round
// a filled box adds a second contour and doubles the visual noise; pattern
// expanders never draw one. Separate filled shapes with the grid gap (white
// gutters) or two neutral tints instead. Advisory: the shape renders as
// authored. Nested cell sub-grids are walked with their own paths.
func detectFilledShapeOutlined(grid *jsonschema.ShapeGridInput, gridPath string, slideIdx int) []*patterns.ValidationError {
	if grid == nil {
		return nil
	}
	var out []*patterns.ValidationError
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", gridPath, ri, ci)
			if cell.Grid != nil {
				out = append(out, detectFilledShapeOutlined(cell.Grid, cellPath+"/grid", slideIdx)...)
			}
			if cell.Shape == nil || !isOpaqueFill(cell.Shape.Fill) || !isVisibleOutline(cell.Shape.Line) {
				continue
			}
			path := cellPath + "/shape/line"
			out = append(out, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeFilledShapeOutlined,
				Message: fmt.Sprintf("slide %d: filled shape at %s also has an outline; drop the line and separate filled shapes with the grid gap (white gutters) or two neutral tints", slideIdx+1, cellPath),
				Fix: &patterns.FixSuggestion{
					Kind: "remove_outline",
					Params: map[string]any{
						"path": path,
						"key":  "line",
					},
				},
			})
		}
	}
	return out
}

// isOpaqueFill reports whether a shape_grid fill paints an opaque surface.
func isOpaqueFill(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s != "" && !isNoneColor(s)
	}
	var obj struct {
		Color string   `json:"color"`
		Alpha *float64 `json:"alpha"`
	}
	if json.Unmarshal(raw, &obj) != nil || obj.Color == "" || isNoneColor(obj.Color) {
		return false
	}
	return obj.Alpha == nil || *obj.Alpha >= minFilledAlpha
}

// isVisibleOutline reports whether a shape_grid line draws a stroke.
func isVisibleOutline(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s != "" && !isNoneColor(s)
	}
	var obj struct {
		Color string `json:"color"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	return obj.Color != "" && !isNoneColor(obj.Color)
}

func isNoneColor(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "transparent", "nofill":
		return true
	}
	return false
}

package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// TestNoPatternOutlinesAFilledShape: every registered pattern, expanded from
// its exemplar with a theme, draws no visible outline round an opaque filled
// shape (go-slide-creator-pgdkp). Separation comes from gutters and neutral
// tints. The only line allowed on a filled shape is a seam cover in the fill's
// own colour (metric-list's highlighted band), which draws no contour.
func TestNoPatternOutlinesAFilledShape(t *testing.T) {
	for _, pat := range Default().List() {
		ex, ok := pat.(Exemplar)
		if !ok {
			continue
		}
		t.Run(pat.Name(), func(t *testing.T) {
			data, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				t.Fatal(err)
			}
			vals := pat.NewValues()
			if err := json.Unmarshal(data, vals); err != nil {
				t.Fatal(err)
			}
			grid, err := pat.Expand(fullThemeCtx(), vals, nil, nil)
			if err != nil {
				t.Skipf("exemplar does not expand in the test context: %v", err)
			}
			walkOutlinedFills(t, grid, pat.Name())
		})
	}
}

func walkOutlinedFills(t *testing.T, grid *jsonschema.ShapeGridInput, path string) {
	t.Helper()
	if grid == nil {
		return
	}
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			walkOutlinedFills(t, cell.Grid, path)
			if cell.Shape == nil || !filledForTest(cell.Shape.Fill) || !outlinedForTest(cell.Shape.Line) {
				continue
			}
			if sameColourLine(cell.Shape.Fill, cell.Shape.Line) {
				continue
			}
			t.Errorf("%s rows[%d].cells[%d]: filled shape %s carries outline %s", path, ri, ci, cell.Shape.Fill, cell.Shape.Line)
		}
	}
}

func filledForTest(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	tone, ok := opaqueFillTone(raw)
	return ok && tone.Color != "" && !strings.EqualFold(tone.Color, "none")
}

func outlinedForTest(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s != "" && s != "none"
	}
	var obj struct {
		Color string `json:"color"`
	}
	return json.Unmarshal(raw, &obj) == nil && obj.Color != "" && obj.Color != "none"
}

func sameColourLine(fill, line json.RawMessage) bool {
	f, fok := parseFillTone(fill)
	l, lok := parseFillTone(line)
	return fok && lok && f.Color == l.Color && f.LumMod == l.LumMod && f.LumOff == l.LumOff
}

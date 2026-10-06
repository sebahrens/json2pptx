package shapegrid

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A layer is written through the cell-shape writer: one <p:sp> per layer with
// its preset geometry, adjust values, rotation, flip, fill, line and text.
func TestLayerShapesAreWrittenLikeCellShapes(t *testing.T) {
	grid := layeredGrid([]float64{100}, Cell{
		Fit:   FitContain,
		Shape: &ShapeSpec{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`), Line: json.RawMessage(`"none"`)},
		Layers: []Layer{
			{Name: "segment", Frame: LayerFrame{X: 0, Y: 0, W: 1, H: 1}, Shape: &ShapeSpec{
				Geometry:    "blockArc",
				Fill:        json.RawMessage(`"accent1"`),
				Line:        json.RawMessage(`"none"`),
				Adjustments: map[string]int64{"adj1": 16200000, "adj2": 0, "adj3": 20000},
				Rotation:    45,
				FlipH:       true,
			}},
			{Name: "badge", Frame: LayerFrame{X: 0.7, Y: 0.1, W: 0.2, H: 0.2}, Shape: &ShapeSpec{
				Geometry: "ellipse",
				Fill:     json.RawMessage(`"dk2"`),
				Line:     json.RawMessage(`{"color":"lt1","width":1.5}`),
				Text:     json.RawMessage(`{"content":"1","size":12,"bold":true,"color":"lt1","align":"ctr","vertical_align":"ctr"}`),
			}},
		},
	})
	res := mustResolve(t, grid)
	if len(res.Cells) != 3 {
		t.Fatalf("resolved %d entries, want 3", len(res.Cells))
	}
	var xmls []string
	for _, c := range res.Cells {
		data, err := GenerateCellShapeXML(c)
		if err != nil {
			t.Fatalf("GenerateCellShapeXML(layer=%v idx=%d): %v", c.Layer, c.LayerIdx, err)
		}
		xmls = append(xmls, string(data))
	}
	for i, x := range xmls {
		if n := strings.Count(x, "<p:sp>"); n != 1 {
			t.Errorf("entry %d wrote %d <p:sp> elements, want 1", i, n)
		}
		if want := fmt.Sprintf(`id="%d"`, res.Cells[i].ID); !strings.Contains(x, want) {
			t.Errorf("entry %d does not carry its shape %s", i, want)
		}
	}

	segment := xmls[1]
	for _, want := range []string{
		`<a:prstGeom prst="blockArc">`,
		`<a:gd name="adj1" fmla="val 16200000"/>`,
		`<a:gd name="adj2" fmla="val 0"/>`,
		`<a:gd name="adj3" fmla="val 20000"/>`,
		`rot="2700000"`,
		`flipH="1"`,
		`<a:schemeClr val="accent1"/>`,
		`<a:noFill/></a:ln>`,
	} {
		if !strings.Contains(segment, want) {
			t.Errorf("segment layer XML lacks %s:\n%s", want, segment)
		}
	}
	// The three adjust values keep their name order inside one <a:avLst>.
	if a, b, c := strings.Index(segment, `name="adj1"`), strings.Index(segment, `name="adj2"`), strings.Index(segment, `name="adj3"`); a >= b || b >= c {
		t.Errorf("adjust values are not written in name order (%d, %d, %d)", a, b, c)
	}

	badge := xmls[2]
	for _, want := range []string{
		`<a:prstGeom prst="ellipse">`,
		`<a:schemeClr val="dk2"/>`,
		`<a:t>1</a:t>`,
		`sz="1200"`,
		`b="1"`,
		`anchor="ctr"`,
	} {
		if !strings.Contains(badge, want) {
			t.Errorf("badge layer XML lacks %s:\n%s", want, badge)
		}
	}
	if strings.Contains(badge, "<a:noFill/></a:ln>") || !strings.Contains(badge, `<a:ln w="19050">`) {
		t.Errorf("badge layer lost its outline:\n%s", badge)
	}
	if strings.Contains(badge, "fontScale") {
		t.Errorf("a one-character 12pt badge was written with an autofit shrink:\n%s", badge)
	}

	// The layer's frame is what the shape is written at.
	b := res.Cells[2].Bounds
	if want := fmt.Sprintf(`<a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/>`, b.X, b.Y, b.CX, b.CY); !strings.Contains(badge, want) {
		t.Errorf("badge layer is not written at its frame %s:\n%s", want, badge)
	}
}

// A layer's text is measured in the theme face the pattern sized it in, like
// a cell shape's: the stored autofit shrink depends on it.
func TestLayerTextKeepsItsMeasureFonts(t *testing.T) {
	text := json.RawMessage(`{"content":"A label sized to fit its frame exactly at this size","size":12}`)
	build := func(major string) ResolvedCell {
		grid := layeredGrid([]float64{100}, Cell{Layers: []Layer{{
			Frame: LayerFrame{X: 0, Y: 0, W: 0.2, H: 0.3},
			Shape: &ShapeSpec{Geometry: "rect", Text: text},
		}}})
		grid.Rows[0].Cells[0].Layers[0].Shape.ThemeFonts.Minor = major
		res, err := resolveGrid(grid, newAlloc(100), nil)
		if err != nil {
			t.Fatal(err)
		}
		return res.Cells[0]
	}
	narrow, wide := build("Arial Narrow"), build("Verdana")
	if narrow.ShapeSpec.ThemeFonts.Minor != "Arial Narrow" || wide.ShapeSpec.ThemeFonts.Minor != "Verdana" {
		t.Fatalf("layer shapes lost their theme fonts: %+v / %+v", narrow.ShapeSpec.ThemeFonts, wide.ShapeSpec.ThemeFonts)
	}
	if a, b := canvasAutofit(&narrow), canvasAutofit(&wide); a < b {
		t.Errorf("the narrow face needs a harder shrink (%v) than the wide one (%v): the face is not measured", a, b)
	}
}

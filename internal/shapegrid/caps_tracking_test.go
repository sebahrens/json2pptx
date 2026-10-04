package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func capsShapeXML(t *testing.T, text string, widthPt float64) string {
	t.Helper()
	spec := &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)}
	xml, err := GenerateShapeXML(spec, 1, pptx.RectEmu{CX: PtToEMU(widthPt), CY: PtToEMU(60)})
	if err != nil {
		t.Fatal(err)
	}
	return string(xml)
}

func TestCapsLabelsGetTracking(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		widthPt    float64
		want       string // "" = no tracking expected
	}{
		{"caps label", `{"content":"BOTTOM LINE","size":12,"bold":true}`, 200, `spc="84"`},
		{"caps label scales with size", `{"content":"WHY","size":18,"bold":true}`, 200, `spc="126"`},
		{"mixed case is not a label", `{"content":"Bottom line","size":12}`, 200, ""},
		{"regular-weight initialism is body text", `{"content":"VP CS","size":14}`, 200, ""},
		{"regular-weight caps cell is body text", `{"content":"CFO","size":12}`, 200, ""},
		{"acronym too short", `{"content":"AI","size":12}`, 200, ""},
		{"figure is not a label", `{"content":"$4.2M","size":12}`, 200, ""},
		{"display heading keeps its spacing", `{"content":"DOMESTIC","size":32}`, 400, ""},
		{"long caps sentence is not a label", `{"content":"THIS IS A VERY LONG UPPERCASE SENTENCE OF TEXT","size":12}`, 600, ""},
		{"label that would wrap is left alone", `{"content":"CAPABILITIES","size":12,"bold":true}`, 108, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := capsShapeXML(t, tc.text, tc.widthPt)
			if tc.want == "" {
				if strings.Contains(xml, ` spc="`) {
					t.Errorf("unexpected tracking: %s", xml)
				}
				return
			}
			if !strings.Contains(xml, tc.want) {
				t.Errorf("missing %s: %s", tc.want, xml)
			}
		})
	}
}

func TestCapsTrackingOnlyOnLabelParagraph(t *testing.T) {
	xml := capsShapeXML(t, `{"paragraphs":[{"content":"CASE STUDY","size":12,"bold":true},{"content":"Retailer cut stock-outs by a third","size":14}]}`, 300)
	if strings.Count(xml, ` spc="84"`) != 1 {
		t.Errorf("want tracking on the label run only: %s", xml)
	}
}

// Caps labels of one size in one grid are tracked together: when one of them
// has no room for tracking, none is tracked, so a pair of row labels does not
// read as two styles (go-slide-creator-5uqfo). Labels of another size, and a
// grid whose labels all have room, are tracked as before.
func TestPeerCapsLabelsAreTrackedTogether(t *testing.T) {
	label := func(text string, size float64) Cell {
		raw, _ := json.Marshal(map[string]any{"content": text, "size": size, "bold": true, "align": "ctr"})
		return Cell{Shape: &ShapeSpec{Geometry: "rect", Text: raw}}
	}
	tracked := func(t *testing.T, widthPt float64, cells ...Cell) []bool {
		t.Helper()
		rows := make([]Row, len(cells))
		for i, c := range cells {
			rows[i] = Row{Cells: []Cell{c}}
		}
		res, err := Resolve(&Grid{Bounds: pptx.RectEmu{CX: PtToEMU(widthPt), CY: PtToEMU(300)}, Columns: []float64{100}, Rows: rows}, pptx.NewShapeIDAllocator(nil))
		if err != nil {
			t.Fatal(err)
		}
		out := make([]bool, len(res.Cells))
		for i, c := range res.Cells {
			xml, err := GenerateCellShapeXML(c)
			if err != nil {
				t.Fatal(err)
			}
			out[i] = strings.Contains(string(xml), `spc="`)
		}
		return out
	}

	// Find a width at which the one-word label has no room for tracking and
	// the two-word label, on two lines, has.
	found := false
	for w := 80.0; w <= 160; w++ {
		alone := tracked(t, w, label("DESIGN PROCESS", 14))
		tight := tracked(t, w, label("PRODUCTION", 14))
		if !alone[0] || tight[0] {
			continue
		}
		found = true
		got := tracked(t, w, label("DESIGN PROCESS", 14), label("PRODUCTION", 14), label("STEP", 12))
		if got[0] || got[1] {
			t.Errorf("at %.0fpt the 14pt peers are tracked %v, want neither: one has no room", w, got[:2])
		}
		if !got[2] {
			t.Errorf("at %.0fpt the 12pt label lost its tracking to peers of another size", w)
		}
		break
	}
	if !found {
		t.Fatal("no width leaves one label without room for tracking")
	}
	if got := tracked(t, 400, label("DESIGN PROCESS", 14), label("PRODUCTION", 14)); !got[0] || !got[1] {
		t.Errorf("labels with room are tracked %v, want both", got)
	}
}

// Tracking and line balancing measure the lines the text is set in — the
// preset's text rectangle, which an ellipse or a chevron pulls inside the
// shape — so a label that fills its chevron is not tracked onto a second
// line and a hub label is not balanced against the circle's full width
// (go-slide-creator-69ums).
func TestFinishingMeasuresThePresetTextRectangle(t *testing.T) {
	bounds := pptx.RectEmu{CX: PtToEMU(200), CY: PtToEMU(200)}
	if rect := finishingTextRect("ellipse", nil, bounds); rect.CX >= bounds.CX || rect.CX <= 0 {
		t.Fatalf("ellipse text rectangle is %d EMU wide in a %d EMU shape, want it inside", rect.CX, bounds.CX)
	}
	if rect := finishingTextRect("rect", nil, bounds); rect.CX != bounds.CX || rect.CY != bounds.CY {
		t.Errorf("rect text rectangle = %dx%d, want the shape", rect.CX, rect.CY)
	}
	raw := json.RawMessage(`{"content":"MANUFACTURING","size":12,"bold":true,"align":"ctr"}`)
	onlyRect, both := false, false
	for w := 110.0; w <= 260; w += 2 {
		b := pptx.RectEmu{CX: PtToEMU(w), CY: PtToEMU(40)}
		inRect, err := GenerateShapeXML(&ShapeSpec{Geometry: "rect", Text: raw}, 1, b)
		if err != nil {
			t.Fatal(err)
		}
		inChevron, err := GenerateShapeXML(&ShapeSpec{Geometry: "chevron", Text: raw}, 1, b)
		if err != nil {
			t.Fatal(err)
		}
		r, c := strings.Contains(string(inRect), `spc="`), strings.Contains(string(inChevron), `spc="`)
		if c && !r {
			t.Errorf("at %.0fpt the label is tracked in a chevron and not in the wider rect", w)
		}
		onlyRect = onlyRect || (r && !c)
		both = both || (r && c)
	}
	if !onlyRect || !both {
		t.Errorf("fixture: want a width where only the rect tracks (%v) and one where both do (%v)", onlyRect, both)
	}
}

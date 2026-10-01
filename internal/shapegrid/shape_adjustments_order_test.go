package shapegrid

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// A shape with several adjustment handles must emit them in a stable (name)
// order; ranging over the map made identical input produce different bytes.
func TestGenerateShapeXMLAdjustmentsDeterministic(t *testing.T) {
	spec := &ShapeSpec{
		Geometry:    "upArrow",
		Adjustments: map[string]int64{"adj2": 75000, "adj1": 18750, "adj3": 5000},
	}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 914400, CY: 914400}
	first, err := GenerateShapeXML(spec, 2, bounds)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		got, err := GenerateShapeXML(spec, 2, bounds)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("run %d produced different XML", i)
		}
	}
	s := string(first)
	a1, a2, a3 := strings.Index(s, `name="adj1"`), strings.Index(s, `name="adj2"`), strings.Index(s, `name="adj3"`)
	if a1 < 0 || !(a1 < a2 && a2 < a3) {
		t.Errorf("adjustments not in name order: adj1@%d adj2@%d adj3@%d", a1, a2, a3)
	}
}

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

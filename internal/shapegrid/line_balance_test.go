package shapegrid

import (
	"encoding/json"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

var marRPattern = regexp.MustCompile(`marR="(\d+)"`)

func TestBalanceHeadingRemovesOneWordLastLine(t *testing.T) {
	const heading = "Production use is now the norm, not the exception"
	text := `{"content":"` + heading + `","size":14,"bold":true,"align":"l"}`
	const head = "Production use is now the norm, not the"
	spec := &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)}
	tb, _ := ResolveTextInput(spec.Text)
	measure := func(s string, w int64) int {
		r, err := textfit.MeasureStyledRuns(textfit.StyledMeasureParams{Runs: []textfit.StyledRun{{Text: s, Bold: true}}, FontName: "Liberation Sans", FontPt: 14, WidthEMU: w})
		if err != nil {
			t.Fatal(err)
		}
		return r.Lines
	}
	// Find a box width at which the heading ends in a one-word line.
	var bounds pptx.RectEmu
	var width int64
	for pt := 200.0; pt <= 500; pt++ {
		b := pptx.RectEmu{CX: PtToEMU(pt), CY: PtToEMU(80)}
		w, _ := scaledTextRect(b, tb, [4]int64{})
		if measure(heading, w) == 2 && measure(head, w) == 1 {
			bounds, width = b, w
			break
		}
	}
	if width == 0 {
		t.Fatal("no widowed width found for the fixture")
	}
	full := 2
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	m := marRPattern.FindStringSubmatch(string(xml))
	if m == nil {
		t.Fatalf("widowed heading was not balanced: %s", xml)
	}
	margin, _ := strconv.ParseInt(m[1], 10, 64)
	if got := measure(heading, width-margin); got != full {
		t.Errorf("balanced measure wraps to %d lines, want %d", got, full)
	}
	if measure(head, width-margin) != full {
		t.Error("last line still holds a single word")
	}
}

func TestBalanceLeavesBodyAndSingleLinesAlone(t *testing.T) {
	for _, text := range []string{
		`{"content":"Production use is now the norm, not the exception","size":14,"align":"l"}`, // not bold
		`{"content":"Agents are the new unit","size":14,"bold":true,"align":"l"}`,               // one line
		`{"content":"GOVERNANCE","size":14,"bold":true}`,                                        // one word
	} {
		spec := &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)}
		xml, err := GenerateShapeXML(spec, 1, pptx.RectEmu{CX: PtToEMU(210), CY: PtToEMU(80)})
		if err != nil {
			t.Fatal(err)
		}
		if marRPattern.Match(xml) {
			t.Errorf("unexpected balancing for %s: %s", text, xml)
		}
	}
}

func TestUnifyCardAlignment(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"centred heading over left bullets", []string{"ctr", "l", "l"}, []string{"l", "l", "l"}},
		{"all centred tile", []string{"ctr", "ctr"}, []string{"ctr", "ctr"}},
		{"right-aligned figure kept", []string{"r", "l"}, []string{"r", "l"}},
		{"centred caption below left text kept", []string{"l", "ctr"}, []string{"l", "ctr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := &pptx.TextBody{}
			for _, a := range tc.in {
				tb.Paragraphs = append(tb.Paragraphs, pptx.Paragraph{Align: a, Runs: []pptx.Run{{Text: "x"}}})
			}
			unifyCardAlignment(tb)
			for i, p := range tb.Paragraphs {
				if p.Align != tc.want[i] {
					t.Errorf("paragraph %d align = %q, want %q", i, p.Align, tc.want[i])
				}
			}
		})
	}
}

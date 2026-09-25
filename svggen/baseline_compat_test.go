package svggen

import (
	"encoding/xml"
	"math"
	"strings"
	"testing"
)

func TestBakeSVGTextBaselinesUsesAlphabeticY(t *testing.T) {
	b := &SVGBuilder{
		textBaselines: []TextBaseline{TextBaselineTop, TextBaselineMiddle, TextBaselineBottom, TextBaselineAlphabetic},
		textFontSizes: []float64{15, 15, 15, 15}, // 15pt in pre-scaled mm SVG
	}
	raw := `<svg xmlns="http://www.w3.org/2000/svg">` +
		`<text x="10" dominant-baseline="text-before-edge" y="50.00"><tspan x="10" y="50.00">top</tspan></text>` +
		`<text x="20" dominant-baseline="central" y="50.00"><tspan x="20" y="50.00">middle</tspan></text>` +
		`<text x="30" dominant-baseline="text-after-edge" y="50.00"><tspan x="30" y="50.00">bottom</tspan></text>` +
		`<text x="40" y="50.00"><tspan x="40" y="50.00">alpha</tspan></text></svg>`
	out := b.bakeSVGTextBaselines([]byte(raw))
	if strings.Contains(string(out), "dominant-baseline") {
		t.Fatalf("semantic baseline attribute survived normalization: %s", out)
	}
	var doc struct {
		Texts []struct {
			Y     float64 `xml:"y,attr"`
			Tspan struct {
				Y       float64 `xml:"y,attr"`
				Content string  `xml:",chardata"`
			} `xml:"tspan"`
		} `xml:"text"`
	}
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"top": 53.44, "middle": 51.85, "bottom": 48.94, "alpha": 50}
	if len(doc.Texts) != len(want) {
		t.Fatalf("normalized SVG has %d text elements, want %d", len(doc.Texts), len(want))
	}
	for _, text := range doc.Texts {
		expected, ok := want[text.Tspan.Content]
		if !ok {
			t.Errorf("unexpected label %q", text.Tspan.Content)
			continue
		}
		if math.Abs(text.Y-expected) > 0.01 || math.Abs(text.Tspan.Y-expected) > 0.01 {
			t.Errorf("%s y=(%.2f, %.2f), want %.2f", text.Tspan.Content, text.Y, text.Tspan.Y, expected)
		}
	}
}

func TestBakeSVGTextBaselinesLeavesRotatedAndUnknownText(t *testing.T) {
	b := &SVGBuilder{textBaselines: []TextBaseline{TextBaselineMiddle}, textFontSizes: []float64{15}}
	raw := `<svg><text transform="rotate(-90)"><tspan x="0" y="0">rotated</tspan></text>` +
		`<text x="10" y="20"><tspan x="10" y="20">extra</tspan></text></svg>`
	if got := string(b.bakeSVGTextBaselines([]byte(raw))); got != raw {
		t.Errorf("baseline shim changed text without a baseline attribute: %s", got)
	}
}

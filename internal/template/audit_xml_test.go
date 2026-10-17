package template

import (
	"encoding/xml"
	"github.com/sebahrens/json2pptx/svggen"
	"testing"
)

func TestGraphicsAuditTemplateXMLAndLuminance(t *testing.T) {
	raw := `<root name="` + xmlEscapeAttr("Bad\x00\ufffe&\"font") + `">` + xmlEscapeText("A\x0b\uffff&B") + `</root>`
	var parsed struct {
		Name string `xml:"name,attr"`
		Text string `xml:",chardata"`
	}
	if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Name != "Bad&\"font" || parsed.Text != "A&B" {
		t.Fatalf("lost valid XML text %+v", parsed)
	}
	for _, hex := range []string{"FFFFFF", "112233", "#c89362", "000000"} {
		if hexLuminance(hex) != svggen.MustParseColor(hex).Luminance() {
			t.Fatalf("luminance drift %s", hex)
		}
	}
	if hexLuminance("invalid") != 0 {
		t.Fatal("invalid luminance should remain zero")
	}
}

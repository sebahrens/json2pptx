package pptx

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGeneratePic(t *testing.T) {
	opts := PicOptions{
		ID:       42,
		Name:     "Test Picture",
		PNGRelID: "rId5",
		OffsetX:  914400,
		OffsetY:  914400,
		ExtentCX: 1828800,
		ExtentCY: 1371600,
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)

	// Check namespaces
	if !strings.Contains(picStr, `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`) {
		t.Error("missing p namespace")
	}
	if !strings.Contains(picStr, `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`) {
		t.Error("missing a namespace")
	}
	if !strings.Contains(picStr, `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`) {
		t.Error("missing r namespace")
	}

	// Check essential elements
	if !strings.Contains(picStr, `id="42"`) {
		t.Error("missing shape id")
	}
	if !strings.Contains(picStr, `name="Test Picture"`) {
		t.Error("missing shape name")
	}
	if !strings.Contains(picStr, `r:embed="rId5"`) {
		t.Error("missing blip embed reference")
	}
	if !strings.Contains(picStr, `noChangeAspect="1"`) {
		t.Error("missing picLocks noChangeAspect")
	}

	// Check position/size
	if !strings.Contains(picStr, `x="914400"`) {
		t.Error("missing offset x")
	}
	if !strings.Contains(picStr, `y="914400"`) {
		t.Error("missing offset y")
	}
	if !strings.Contains(picStr, `cx="1828800"`) {
		t.Error("missing extent cx")
	}
	if !strings.Contains(picStr, `cy="1371600"`) {
		t.Error("missing extent cy")
	}

	// Check geometry
	if !strings.Contains(picStr, `prst="rect"`) {
		t.Error("missing rect preset geometry")
	}
}

func TestGeneratePic_WithSVG(t *testing.T) {
	opts := PicOptions{
		ID:       10,
		PNGRelID: "rId2",
		SVGRelID: "rId3",
		ExtentCX: 1000000,
		ExtentCY: 1000000,
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)

	// Check SVG extension URI
	if !strings.Contains(picStr, SVGBlipExtensionURI) {
		t.Error("missing SVG extension URI")
	}

	// Check asvg namespace
	if !strings.Contains(picStr, SVGBlipNamespace) {
		t.Error("missing asvg namespace")
	}

	// Check svgBlip element
	if !strings.Contains(picStr, `<asvg:svgBlip`) {
		t.Error("missing asvg:svgBlip element")
	}

	// Check SVG reference
	if !strings.Contains(picStr, `r:embed="rId3"`) {
		t.Error("missing SVG embed reference")
	}

	// PNG reference should still be on the main blip
	if !strings.Contains(picStr, `<a:blip r:embed="rId2">`) {
		t.Error("missing PNG embed reference on a:blip")
	}
}

func TestGeneratePic_WithDescription(t *testing.T) {
	opts := PicOptions{
		ID:          5,
		Name:        "Chart",
		Description: "A pie chart showing Q4 results",
		PNGRelID:    "rId1",
		ExtentCX:    500000,
		ExtentCY:    500000,
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)
	if !strings.Contains(picStr, `descr="A pie chart showing Q4 results"`) {
		t.Error("missing description attribute")
	}
}

func TestGeneratePic_SpecialCharacters(t *testing.T) {
	opts := PicOptions{
		ID:          1,
		Name:        `Picture with "quotes" & <special> chars`,
		Description: `Alt text with "quotes" & <tags>`,
		PNGRelID:    "rId1",
		ExtentCX:    100000,
		ExtentCY:    100000,
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)

	// Check that special characters are escaped
	if strings.Contains(picStr, `"quotes"`) && !strings.Contains(picStr, "&quot;") {
		t.Error("quotes should be escaped")
	}
	if strings.Contains(picStr, `<special>`) && !strings.Contains(picStr, "&lt;") {
		t.Error("angle brackets should be escaped")
	}
}

func TestGeneratePic_DefaultName(t *testing.T) {
	opts := PicOptions{
		ID:       7,
		PNGRelID: "rId1",
		ExtentCX: 100000,
		ExtentCY: 100000,
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)
	if !strings.Contains(picStr, `name="Picture 7"`) {
		t.Error("default name should be 'Picture N'")
	}
}

func TestGeneratePic_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		opts PicOptions
	}{
		{
			name: "missing ID",
			opts: PicOptions{PNGRelID: "rId1"},
		},
		{
			name: "missing PNGRelID",
			opts: PicOptions{ID: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GeneratePic(tt.opts)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestGeneratePicSimple(t *testing.T) {
	pic, err := GeneratePicSimple(1, "rId1", 100, 200, 300, 400)
	if err != nil {
		t.Fatalf("GeneratePicSimple failed: %v", err)
	}

	picStr := string(pic)
	if !strings.Contains(picStr, `id="1"`) {
		t.Error("missing id")
	}
	if !strings.Contains(picStr, `r:embed="rId1"`) {
		t.Error("missing embed")
	}
	if !strings.Contains(picStr, `x="100"`) {
		t.Error("missing x")
	}
	if !strings.Contains(picStr, `y="200"`) {
		t.Error("missing y")
	}
}

func TestGeneratePicWithSVG(t *testing.T) {
	pic, err := GeneratePicWithSVG(5, "rId10", "rId11", 0, 0, 500000, 500000)
	if err != nil {
		t.Fatalf("GeneratePicWithSVG failed: %v", err)
	}

	picStr := string(pic)
	if !strings.Contains(picStr, `id="5"`) {
		t.Error("missing id")
	}
	if !strings.Contains(picStr, `r:embed="rId10"`) {
		t.Error("missing PNG embed")
	}
	if !strings.Contains(picStr, `r:embed="rId11"`) {
		t.Error("missing SVG embed")
	}
	if !strings.Contains(picStr, SVGBlipExtensionURI) {
		t.Error("missing SVG extension URI")
	}
}

// TestGeneratePic_XMLValidity verifies the generated XML is well-formed.
func TestGeneratePic_XMLValidity(t *testing.T) {
	tests := []struct {
		name string
		opts PicOptions
	}{
		{
			name: "simple PNG",
			opts: PicOptions{
				ID:       1,
				PNGRelID: "rId1",
				ExtentCX: 1000000,
				ExtentCY: 1000000,
			},
		},
		{
			name: "with SVG",
			opts: PicOptions{
				ID:       2,
				PNGRelID: "rId1",
				SVGRelID: "rId2",
				ExtentCX: 1000000,
				ExtentCY: 1000000,
			},
		},
		{
			name: "with description",
			opts: PicOptions{
				ID:          3,
				Name:        "Test",
				Description: "Description with special chars: & < > \"",
				PNGRelID:    "rId1",
				ExtentCX:    1000000,
				ExtentCY:    1000000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pic, err := GeneratePic(tt.opts)
			if err != nil {
				t.Fatalf("GeneratePic failed: %v", err)
			}

			// Try to parse as XML to verify well-formedness
			var v interface{}
			if err := xml.Unmarshal(pic, &v); err != nil {
				t.Errorf("generated XML is not well-formed: %v\nXML: %s", err, pic)
			}
		})
	}
}

// TestGeneratePic_NoTransformWithoutSize verifies no xfrm is generated without size.
func TestGeneratePic_NoTransformWithoutSize(t *testing.T) {
	opts := PicOptions{
		ID:       1,
		PNGRelID: "rId1",
		// No ExtentCX/ExtentCY
	}

	pic, err := GeneratePic(opts)
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	picStr := string(pic)
	if strings.Contains(picStr, `<a:xfrm>`) {
		t.Error("xfrm should not be present without size")
	}
	if strings.Contains(picStr, `<a:prstGeom`) {
		t.Error("prstGeom should not be present without size")
	}
}

func TestEscapeXMLAttr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with "quotes"`, `with &quot;quotes&quot;`},
		{"with & ampersand", "with &amp; ampersand"},
		{"with <angle> brackets", "with &lt;angle&gt; brackets"},
		{`all "special" <chars> & more`, `all &quot;special&quot; &lt;chars&gt; &amp; more`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeXMLAttr(tt.input)
			if got != tt.expected {
				t.Errorf("escapeXMLAttr(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestGeneratePic_SVGBlipConstants verifies the constants match expected values.
func TestGeneratePic_SVGBlipConstants(t *testing.T) {
	// These are the official Microsoft values for SVG support in Office
	expectedURI := "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"
	expectedNS := "http://schemas.microsoft.com/office/drawing/2016/SVG/main"

	if SVGBlipExtensionURI != expectedURI {
		t.Errorf("SVGBlipExtensionURI = %q, want %q", SVGBlipExtensionURI, expectedURI)
	}
	if SVGBlipNamespace != expectedNS {
		t.Errorf("SVGBlipNamespace = %q, want %q", SVGBlipNamespace, expectedNS)
	}
}

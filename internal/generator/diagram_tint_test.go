package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDiagramTintFillPreservesThemeAndAuthoredColors(t *testing.T) {
	tests := []struct {
		name, color      string
		retained, offset int
		want             string
	}{
		{"pale saturated accent", "accent4", 20000, 80000, `<a:schemeClr val="accent4"><a:tint val="20000"/></a:schemeClr>`},
		{"moderate accent", "accent1", 60000, 40000, `<a:schemeClr val="accent1"><a:tint val="60000"/></a:schemeClr>`},
		{"full accent", "accent1", 100000, 0, `<a:schemeClr val="accent1"/>`},
		{"authored hex", "#0097A7", 20000, 80000, `<a:srgbClr val="0097A7"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var xml bytes.Buffer
			diagramTintFill(tt.color, tt.retained, tt.offset).WriteTo(&xml)
			if !strings.Contains(xml.String(), tt.want) {
				t.Fatalf("fill = %s, want %s", xml.String(), tt.want)
			}
			if strings.Contains(xml.String(), "lumMod") || strings.Contains(xml.String(), "lumOff") {
				t.Fatalf("fill still uses luminance modifiers: %s", xml.String())
			}
		})
	}
}

func TestTaxonomyAuthoredHexEmitsValidRGBColors(t *testing.T) {
	spec := &types.DiagramSpec{Style: &types.DiagramStyle{Colors: []string{"#0097A7"}}}
	bounds := types.BoundingBox{Width: 8000000, Height: 5000000}
	panels := make([]nativePanelData, 10)
	for i := range panels {
		panels[i] = nativePanelData{title: "Panel", body: "- Detail"}
	}
	tests := []struct {
		name  string
		build func() string
	}{
		{"SWOT", func() string {
			return generateSWOTGroupXML(panels[:4], bounds, 100, taxonomyPalette(spec, 4, swotDefaultTint))
		}},
		{"PESTEL", func() string {
			return generatePESTELGroupXML(panels[:6], bounds, 100, taxonomyPalette(spec, 6, uniformTaxonomyTint))
		}},
		{"business canvas", func() string {
			return generateBMCGroupXML(panels[:9], bounds, 100, taxonomyPalette(spec, 9, bmcDefaultTint))
		}},
		{"nine box semantic accent", func() string {
			return generateNineBoxGroupXML(panels, bounds, 100, nineBoxSemanticTints(map[string]string{
				"negative": "#0097A7", "neutral": "#0097A7", "positive": "#0097A7",
			}))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := tt.build()
			if xml == "" {
				t.Fatal("empty native diagram")
			}
			if strings.Contains(xml, `schemeClr val="#0097A7"`) || !strings.Contains(xml, `srgbClr val="0097A7"`) {
				t.Fatalf("authored color was not serialized as RGB: %s", xml)
			}
		})
	}
}

func TestNativeDiagramBuildersEmitAccentTints(t *testing.T) {
	bounds := types.BoundingBox{Width: 8000000, Height: 5000000}
	panels := []nativePanelData{{title: "One", body: "- Detail"}, {title: "Two", body: "- Detail"}, {title: "Three", body: "- Detail"}}
	tests := []struct {
		name  string
		build func() string
	}{
		{"value chain", func() string {
			return generateVCSupportBarXML(panels[0], 0, 0, 2000000, 500000, 1, "accent4", 40000, 60000)
		}},
		{"SWOT", func() string {
			return generateSWOTHeaderXML("Strengths", 0, 0, 2000000, 500000, 1, "accent4", 20000, 80000)
		}},
		{"PESTEL", func() string {
			return generatePESTELHeaderXML("Political", 0, 0, 2000000, 500000, 1, "accent4", 20000, 80000)
		}},
		{"nine box", func() string {
			return generateNineBoxCellLabelXML("Star", 0, 0, 2000000, 500000, 1, "accent4", 20000, 80000)
		}},
		{"KPI dashboard", func() string { return generateKPICardXML(panels[0], 0, 0, 2000000, 1500000, 1, nil) }},
		{"house", func() string {
			return string(generateHouseSingleFloorShape(1, panels[0], 0, 0, 2000000, 500000, 1200, 1000))
		}},
		{"pyramid", func() string { return generatePyramidGroupXML(panels, bounds, 1, "Arial") }},
		{"stylish panels", func() string { return generateStylishPanelsGroupXML(panels, bounds, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := tt.build()
			if !strings.Contains(xml, `<a:tint val="`) {
				t.Fatalf("native diagram has no RGB-tinted accent: %s", xml)
			}
		})
	}
}

func TestAuthoredTaxonomyPanelsUseReadableTextAndBullets(t *testing.T) {
	builders := []struct {
		name   string
		header func(string) string
		body   func(string) string
	}{
		{"SWOT", func(c string) string { return generateSWOTHeaderXML("Title", 0, 0, 2000000, 400000, 1, c, 0, 0) }, func(c string) string { return generateSWOTBodyXML("- Detail", 0, 0, 2000000, 1000000, 2, c, 0, 0) }},
		{"PESTEL", func(c string) string { return generatePESTELHeaderXML("Title", 0, 0, 2000000, 400000, 1, c, 0, 0) }, func(c string) string { return generatePESTELBodyXML("- Detail", 0, 0, 2000000, 1000000, 2, c, 0, 0) }},
		{"business canvas", func(c string) string { return generateBMCCellHeaderXML("Title", 0, 0, 2000000, 400000, 1, c, 0, 0) }, func(c string) string { return generateBMCCellBodyXML("- Detail", 0, 0, 2000000, 1000000, 2, c, 0, 0) }},
		{"nine box", func(c string) string { return generateNineBoxCellLabelXML("Title", 0, 0, 2000000, 400000, 1, c, 0, 0) }, func(c string) string {
			return generateNineBoxCellBodyXML("- Detail", 0, 0, 2000000, 1000000, 2, c, 0, 0)
		}},
	}
	for _, color := range []struct{ hex, text string }{{"#102030", "FFFFFF"}, {"#F5F5F5", "000000"}} {
		for _, builder := range builders {
			t.Run(builder.name+"/"+color.hex, func(t *testing.T) {
				xml := builder.header(color.hex) + builder.body(color.hex)
				if !strings.Contains(xml, `srgbClr val="`+strings.TrimPrefix(color.hex, "#")+`"`) {
					t.Fatalf("authored fill changed: %s", xml)
				}
				if got := strings.Count(xml, `srgbClr val="`+color.text+`"`); got < 3 {
					t.Fatalf("header, body, and bullet must use readable text color; got %d in %s", got, xml)
				}
			})
		}
	}
}

func TestDiagramPanelTextFillFallsBackForUnknownColor(t *testing.T) {
	for _, color := range []string{"accent1", "invalid"} {
		var xml bytes.Buffer
		diagramPanelTextFill(color).WriteTo(&xml)
		if !strings.Contains(xml.String(), `schemeClr val="dk1"`) {
			t.Errorf("color %q fallback = %s, want theme dark text", color, xml.String())
		}
	}
}

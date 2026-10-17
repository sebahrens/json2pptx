package main

import (
	"encoding/json"
	"encoding/xml"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"math"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type stakeholderWrittenShape struct {
	Properties struct {
		Transform struct {
			Offset struct {
				X int64 `xml:"x,attr"`
				Y int64 `xml:"y,attr"`
			} `xml:"off"`
			Extent struct {
				W int64 `xml:"cx,attr"`
				H int64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
		Geometry struct {
			Preset string `xml:"prst,attr"`
		} `xml:"prstGeom"`
	} `xml:"spPr"`
}

func TestStakeholderBoundaryMarkersClearWrittenAxisBands(t *testing.T) {
	for _, tpl := range []string{"midnight-blue", "p-style"} {
		for _, flip := range []bool{false, true} {
			name := tpl + "/opposite-corners"
			if flip {
				name = tpl + "/other-corners"
			}
			t.Run(name, func(t *testing.T) {
				if tpl == "p-style" && !slices.Contains(schemaMaximaTemplateNames(t), tpl) {
					t.Skip("optional local p-style template is not installed")
				}
				top, bottom := 100.0, 0.0
				if flip {
					top, bottom = bottom, top
				}
				out := renderDeckSpecCall(t, semanticTestConfig(t), map[string]any{"spec": map[string]any{"meta": map[string]any{"title": "Stakeholder boundary review", "template": tpl}, "slides": []any{map[string]any{"kind": "stakeholder_map", "title": "Boundary stakeholders retain their authored coordinates", "x_axis_label": "Interest", "y_axis_label": "Influence", "points": []any{map[string]any{"label": "A", "x": 0.0, "y": top, "size": 40.0, "status": "supporter"}, map[string]any{"label": "B", "x": 100.0, "y": bottom, "size": 40.0, "status": "blocker"}}}}}})
				if !out.Success {
					t.Fatalf("valid boundary points refused: %s %+v", out.Error, out.Diagnostics)
				}
				source := string(pptxParts(t, out.PptxPath)["ppt/slides/slide1.xml"])
				var markers, axes []stakeholderWrittenShape
				for _, raw := range regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(source, -1) {
					var shape stakeholderWrittenShape
					if err := xml.Unmarshal([]byte(raw), &shape); err != nil {
						t.Fatal(err)
					}
					if shape.Properties.Geometry.Preset == "ellipse" && shape.Properties.Transform.Extent.W > 20*12700 {
						markers = append(markers, shape)
					}
					if strings.Contains(raw, ">Influence ↑<") || strings.Contains(raw, ">Interest →<") {
						axes = append(axes, shape)
					}
				}
				if len(markers) != 2 || len(axes) != 2 {
					t.Fatalf("missing authored markers/axis text: %d/%d", len(markers), len(axes))
				}
				for _, marker := range markers {
					m := marker.Properties.Transform
					if math.Abs(float64(m.Extent.W)/12700-40) > .1 || math.Abs(float64(m.Extent.H)/12700-40) > .1 {
						t.Errorf("authored40pt diameter changed: %+v", m)
					}
					for _, axis := range axes {
						a := axis.Properties.Transform
						if m.Offset.X < a.Offset.X+a.Extent.W && m.Offset.X+m.Extent.W > a.Offset.X && m.Offset.Y < a.Offset.Y+a.Extent.H && m.Offset.Y+m.Extent.H > a.Offset.Y {
							t.Errorf("marker %+v overlaps reserved written axis band %+v", m, a)
						}
					}
				}
				first, second := markers[0].Properties.Transform, markers[1].Properties.Transform
				if first.Offset.X >= second.Offset.X {
					t.Error("authored x ordering changed")
				}
				if !flip && first.Offset.Y >= second.Offset.Y || flip && first.Offset.Y <= second.Offset.Y {
					t.Error("authored y ordering changed")
				}
			})
		}
	}
}

func TestStakeholderPaddingRetainsExhibitCoverage(t *testing.T) {
	p, _ := patterns.Default().Get("stakeholder-map")
	raw, _ := json.Marshal(p.(patterns.Exemplar).ExemplarValues())
	for _, tpl := range []string{"midnight-blue", "p-style"} {
		t.Run(tpl, func(t *testing.T) {
			if tpl == "p-style" && !slices.Contains(schemaMaximaTemplateNames(t), tpl) {
				t.Skip("optional local p-style template is not installed")
			}
			a := loadTemplateAnalysis(t, tpl)
			deck := PresentationInput{Slides: []SlideInput{titledPatternSlide("stakeholder-map", raw)}}
			measured := 0
			collectGeometry(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme, func(u slideUsage) {
				measured++
				if u.frac < u.threshold+exemplarUnderusedMargin {
					t.Errorf("coverage %.1f%% below unchanged %.1f%% threshold+margin", u.frac*100, (u.threshold+exemplarUnderusedMargin)*100)
				}
				if u.lowerBand > slideLowerBandMaxFrac-exemplarLowerBandMargin {
					t.Errorf("empty lowerband %.1f%%", u.lowerBand*100)
				}
			})
			if measured != 1 {
				t.Fatalf("expected actual exhibit measurement, got %d", measured)
			}
		})
	}
}

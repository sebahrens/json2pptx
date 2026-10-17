package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Repair relational constraints without shortening any schema-sized copy or
// reducing array counts: the probe remains an actual worst-case payload.
func coherentConsultingMaximum(name string, value any) any {
	m, ok := value.(map[string]any)
	if !ok {
		return value
	}
	switch name {
	case "issue-tree", "decision-tree":
		nodes := m["nodes"].([]any)
		for i, raw := range nodes {
			node := raw.(map[string]any)
			node["id"] = fmt.Sprintf("%024d", i)
			if i == 0 {
				delete(node, "parent")
			} else {
				node["parent"] = fmt.Sprintf("%024d", 0)
			}
		}
	case "image-hero":
		delete(m["image"].(map[string]any), "url")
	case "raci-matrix":
		for _, raw := range m["activities"].([]any) {
			scores := raw.(map[string]any)["scores"].([]any)
			for i := range scores {
				scores[i] = "R"
			}
			scores[0] = "A"
		}
	case "status-board":
		m["rag_colors"] = map[string]any{"red": "#B71C1C", "amber": "#A96B00", "green": "#2E7D32"}
	}
	return value
}

func TestConsultingSchemaMaximaAreMeasured(t *testing.T) {
	for _, name := range []string{"checklist", "decision-tree", "fishbone-framework", "image-hero", "ipo-flow", "issue-tree", "numeric-heatmap", "pestel-framework", "progress-list", "prose-2col", "raci-matrix", "sources-list", "spectrum", "stakeholder-map", "status-board", "talent-nine-box", "three-horizons", "timeline-vertical", "venn-framework"} {
		t.Run(name, func(t *testing.T) {
			p, _ := patterns.Default().Get(name)
			values, note := schemaMaximumValues(p)
			if note != "" {
				t.Fatal(note)
			}
			raw, _ := json.Marshal(values)
			for _, tpl := range schemaMaximaRunTemplateNames(t) {
				geom := loadSchemaMaximaGeometry(t, tpl)
				a := loadTemplateAnalysis(t, tpl)
				_, bounds := patternExpansionGeometry(titledPatternSlide(name, raw), geom.layouts, geom.width, geom.height, nil)
				grid, err := p.Expand(patterns.ExpandContext{Theme: a.Theme, LayoutBounds: patterns.LayoutBounds{Width: bounds.CX, Height: bounds.CY}}, values, nil, nil)
				if err != nil {
					var measured *patterns.ValidationError
					if !errors.As(err, &measured) || measured.Code != patterns.ErrCodeBodyTooLong || !strings.Contains(err.Error(), "split") {
						t.Fatalf("%s unmeasured refusal: %v", tpl, err)
					}
					t.Logf("%s measured expansion refusal: %v", tpl, err)
				} else {
					for _, row := range grid.Rows {
						for _, cell := range row.Cells {
							if cell.Diagram == nil {
								continue
							}
							spec := cell.Diagram
							if !generator.IsGridNativeDiagram(spec) {
								req := &svggen.RequestEnvelope{Type: spec.Type, Data: spec.Data, Output: svggen.OutputSpec{Width: int(bounds.CX / 12700), Height: int(bounds.CY / 12700), StrictFit: "strict"}}
								if err := svggen.DefaultRegistry().Get(spec.Type).Validate(req); err != nil {
									t.Fatalf("%s invalid SVG maximum: %v", tpl, err)
								}
								result, err := svggen.Render(req)
								if err != nil {
									t.Logf("%s measured SVG refusal: %v", tpl, err)
									continue
								}
								if strings.Contains(string(result.Content), "…") {
									t.Error("SVG maximum silently elided authored text")
								}
								matches := regexp.MustCompile(`(?:font-size[=:]\s*"?|font:\s*(?:bold\s+)?)([0-9.]+)`).FindAllStringSubmatch(string(result.Content), -1)
								if len(matches) == 0 {
									t.Fatal("SVG emitted no measurable text sizes")
								}
								for _, m := range matches {
									size, _ := strconv.ParseFloat(m[1], 64)
									if size < 12 {
										t.Errorf("%s SVGwriter emitted %.1fpt", tpl, size)
									}
								}
								continue
							}

							native, err := generator.GenerateGridNativeDiagram(spec, generator.NativeDiagramRegion{Bounds: types.BoundingBox{Width: bounds.CX, Height: bounds.CY}}, "maximum", func(int) uint32 { return 1 })
							if err != nil {
								var capacity *generator.NativeRegionCapacityError
								if !errors.As(err, &capacity) {
									t.Fatalf("%s unmeasured native refusal: %v", tpl, err)
								}
								t.Logf("%s measured native refusal: %v", tpl, err)
								continue
							}

							if strings.Contains(native.XML, "…") {
								t.Error("native maximum silently elided authored text")
							}
							for _, run := range regexp.MustCompile(`(?s)<a:r>.*?</a:r>`).FindAllString(native.XML, -1) {
								sizeMatch := regexp.MustCompile(`\bsz="([0-9]+)"`).FindStringSubmatch(run)
								textMatch := regexp.MustCompile(`(?s)<a:t>(.*?)</a:t>`).FindStringSubmatch(run)
								if len(sizeMatch) == 0 || len(textMatch) == 0 {
									continue
								}
								size, _ := strconv.Atoi(sizeMatch[1])
								floor := 1200
								// Only numeric cell values are dense-matrix captions. Axis labels remain body copy.
								if name == "numeric-heatmap" && regexp.MustCompile(`^[0-9.+%-]+$`).MatchString(textMatch[1]) {
									floor = 1000
								}
								if size < floor {
									t.Errorf("%s native writer emitted %.1fpt for %q below %.1fpt role floor", tpl, float64(size)/100, textMatch[1], float64(floor)/100)
								}
							}

						}
					}

				}
				pt := measureSchemaMaximumEncodedPt(name, raw, geom)
				if pt != 0 {
					t.Errorf("%s max renders belowfloor at %.1fpt", tpl, pt)
				}
				t.Logf("%s: maximal relationally valid payload measured, below-floor prediction %.1fpt", tpl, pt)
			}
		})
	}
}

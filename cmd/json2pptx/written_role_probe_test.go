package main

import (
	"encoding/xml"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

var writtenShapeRE = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)

type writtenProbeShape struct {
	NV struct {
		C struct {
			ID uint32 `xml:"id,attr"`
		} `xml:"cNvPr"`
	} `xml:"nvSpPr"`
	Body struct {
		Properties struct {
			Autofit struct {
				Scale int `xml:"fontScale,attr"`
			} `xml:"normAutofit"`
		} `xml:"bodyPr"`
		Paragraphs []struct {
			Runs []struct {
				Properties struct {
					Size int `xml:"sz,attr"`
				} `xml:"rPr"`
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"txBody"`
}

// writtenRoleViolations counts written grid paragraphs whose populated run
// size times the stored autofit scale falls below their source role's floor
// — the same check generation applies before publishing. Budget probes use it
// so documented character budgets are calibrated against written output, not
// the advisory preflight predictor (which tolerates shrinks down to 85%).
func writtenRoleViolations(t *testing.T, input *PresentationInput, geom schemaMaximaGeometry) int {
	t.Helper()
	specs, _, _, err := convertPresentationSlides(input.Slides, geom.layouts, geom.width, geom.height, nil, nil, "", nil, false)
	if err != nil {
		return 1 // an unexpandable payload is not a clean budget
	}
	mode := tokens.ParseViewingMode(input.ViewingMode)
	violations := 0
	for _, spec := range specs {
		for _, fragment := range spec.RawShapeXML {
			for _, raw := range writtenShapeRE.FindAllString(string(fragment), -1) {
				var shape writtenProbeShape
				if xml.Unmarshal([]byte(raw), &shape) != nil {
					continue
				}
				roles := spec.GridTextRoles[shape.NV.C.ID]
				if len(roles) != len(shape.Body.Paragraphs) {
					continue
				}
				scale := shape.Body.Properties.Autofit.Scale
				if scale == 0 {
					scale = 100000
				}
				for i, p := range shape.Body.Paragraphs {
					if roles[i] == "" {
						continue
					}
					smallest := 0
					for _, r := range p.Runs {
						if strings.TrimSpace(r.Text) != "" && r.Properties.Size > 0 && (smallest == 0 || r.Properties.Size < smallest) {
							smallest = r.Properties.Size
						}
					}
					if smallest > 0 && smallest*scale/100000 < tokens.MinReadableHPt(mode, roles[i]) {
						violations++
					}
				}
			}
		}
	}
	return violations
}

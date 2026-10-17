package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"os"
	"path/filepath"
	"testing"
)

func TestConsultingExemplarsUseContentArea(t *testing.T) {
	for _, tpl := range []string{"business-template", "modern-template", "midnight-blue", "p-style"} {
		t.Run(tpl, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
				t.Skipf("template not installed: %v", err)
			}
			a := loadTemplateAnalysis(t, tpl)
			deck := PresentationInput{}
			for _, name := range []string{"checklist", "progress-list", "prose-2col", "raci-matrix", "status-board", "sources-list", "timeline-vertical"} {
				p, _ := patterns.Default().Get(name)
				raw, _ := json.Marshal(p.(patterns.Exemplar).ExemplarValues())
				deck.Slides = append(deck.Slides, titledPatternSlide(name, raw))
			}
			collectGeometry(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme, func(u slideUsage) {
				t.Logf("%s coverage %.1f%% lower %.1f%%", u.pattern, u.frac*100, u.lowerBand*100)
				if u.frac < u.threshold+exemplarUnderusedMargin {
					t.Errorf("%s coverage %.1f%% below threshold+margin %.1f%%", u.pattern, u.frac*100, (u.threshold+exemplarUnderusedMargin)*100)
				}
				if u.lowerBand > slideLowerBandMaxFrac-exemplarLowerBandMargin {
					t.Errorf("%s lower band %.1f%%", u.pattern, u.lowerBand*100)
				}
			})
		})
	}
}

package rhythm_test

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/rhythm"
)

func TestDensityUsesActualContentAndReportsUncertainty(t *testing.T) {
	short, long := "Short statement", strings.Repeat("Detailed evidence with additional context. ", 300)
	slides := []rhythm.Slide{{Text: short, ContentKinds: []string{"text"}}, {Text: long, ContentKinds: []string{"text"}}, {Text: short, ContentKinds: []string{"text"}}, {Text: long, ContentKinds: []string{"text"}}, {ContentKinds: []string{"table", "chart", "bullets"}}}
	r := rhythm.Analyze(slides)
	if r.PerSlide[0].DensityClass != "low" || r.PerSlide[1].DensityClass != "high" || r.Aggregates.DensityCV < 0.1 {
		t.Fatalf("pacing not reflected: %+v", r)
	}
	if r.PerSlide[0].DensitySource != "estimated" || r.PerSlide[4].DensityClass != "unknown" || r.PerSlide[4].DensitySource != "unknown" {
		t.Fatalf("unmeasured geometry reported as confident density: %+v", r.PerSlide)
	}
	slides[0].HasCompose = true
	slides[0].ContentKinds = []string{"bullets"}
	changed := rhythm.Analyze(slides)
	if changed.PerSlide[0].DensityOccupancy != r.PerSlide[0].DensityOccupancy {
		t.Fatal("representation alone changed density")
	}
}

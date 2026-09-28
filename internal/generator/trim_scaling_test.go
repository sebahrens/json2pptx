package generator

import (
	"fmt"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

func bulletShape(n int) (*shapeXML, textfit.Params) {
	paragraphs := make([]paragraphXML, n)
	texts := make([]string, n)
	for i := range paragraphs {
		texts[i] = fmt.Sprintf("Bullet %d on pricing discipline and service mix", i)
		paragraphs[i] = paragraphXML{Runs: []runXML{{Text: texts[i]}}}
	}
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: paragraphs}}
	params := textfit.Params{Paragraphs: texts, WidthEMU: 8_000_000, HeightEMU: 4_000_000, FontSizeHPt: 2000, FontName: "Calibri", ExtraSpacingPt: 6}
	return shape, params
}

// go-slide-creator-8hg02: trimOverflowParagraphs dropped one paragraph per
// iteration and re-measured all the rest each time, so 2000 bullets in one
// placeholder pinned a core for 3+ minutes (500: 13 s, 1000: 51 s). The
// binary search keeps it to O(log n) measurements.
func TestTrimOverflowParagraphs2000BulletsIsFast(t *testing.T) {
	if _, err := textfit.Calculate(textfit.Params{Paragraphs: []string{"x"}, WidthEMU: 1_000_000, HeightEMU: 1_000_000}); err != nil {
		t.Skipf("font cache unavailable: %v", err)
	}
	shape, params := bulletShape(2000)
	var findings []patterns.FitFinding
	cfg := autofitConfig{findings: &findings, findingPath: "/slides/0/content/1"}

	start := time.Now()
	result := trimOverflowParagraphs(shape, params, &cfg)
	elapsed := time.Since(start)
	if budget := 5 * time.Second * raceSlowdown; elapsed > budget {
		t.Fatalf("trimming 2000 bullets took %v, want < %v", elapsed, budget)
	}
	t.Logf("trimmed 2000 bullets in %v", elapsed)

	if result.Overflow {
		t.Fatalf("trimmed content still overflows: %+v", result)
	}
	kept := len(shape.TextBody.Paragraphs) - 1 // minus the ellipsis
	if kept < 2 || kept >= 2000 || shape.TextBody.Paragraphs[kept].Runs[0].Text != "\u2026" {
		t.Fatalf("unexpected trim: kept %d paragraphs", kept)
	}
	if len(findings) != 1 || findings[0].Code != patterns.ErrCodeTextTrimmed {
		t.Fatalf("want one %s finding, got %+v", patterns.ErrCodeTextTrimmed, findings)
	}
}

// The binary search must land on exactly the prefix the old linear loop
// chose: the largest paragraph count that still fits.
func TestTrimOverflowParagraphsKeepsLargestFittingPrefix(t *testing.T) {
	if _, err := textfit.Calculate(textfit.Params{Paragraphs: []string{"x"}, WidthEMU: 1_000_000, HeightEMU: 1_000_000}); err != nil {
		t.Skipf("font cache unavailable: %v", err)
	}
	for _, n := range []int{3, 12, 40} {
		shape, params := bulletShape(n)
		params.HeightEMU = 1_200_000
		cfg := autofitConfig{}
		result := trimOverflowParagraphs(shape, params, &cfg)
		if result.Overflow {
			continue // even 2 paragraphs overflow; nothing to compare
		}
		kept := len(shape.TextBody.Paragraphs) - 1
		// Linear reference: the largest k in [2, n-1] that fits.
		want := 0
		for k := n - 1; k >= 2; k-- {
			r, err := textfit.Calculate(trimmedFitParams(params, params.Paragraphs, k))
			if err != nil {
				t.Fatal(err)
			}
			if !r.Overflow {
				want = k
				break
			}
		}
		if kept != want {
			t.Errorf("n=%d: kept %d paragraphs, linear reference keeps %d", n, kept, want)
		}
	}
}

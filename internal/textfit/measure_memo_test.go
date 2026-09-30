package textfit

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// The MeasureRun memo (go-slide-creator-b7qqg.16) must be invisible: a cached
// answer is the answer a fresh measurement gives, for every input shape the
// fit searches use, and the memo stays bounded.
func TestMeasureRunMemoMatchesUncachedMeasurement(t *testing.T) {
	texts := []string{
		"Enterprise Platform Subscriptions",
		strings.Repeat("W", 400),
		"See " + strings.Repeat("W", 120) + " and more words after it",
		"line one\nline two that is a little longer\n\nline four",
	}
	for _, font := range []string{"Liberation Sans", "Arial", "Lora"} {
		ff, resolved, substituted := fontcache.Resolve(font, "Arial")
		if ff == nil {
			t.Skip("no font cache")
		}
		for _, text := range texts {
			for _, pt := range []float64{9, 11.76, 12, 14, 28} {
				for _, w := range []int64{0, 180000, 914400, 2743200} {
					for _, maxLines := range []int{0, 2} {
						want := measureRunUncached(ff, resolved, substituted, text, pt, w, maxLines)
						for pass := 0; pass < 2; pass++ { // miss, then hit
							got, err := MeasureRun(text, font, pt, w, maxLines)
							if err != nil {
								t.Fatal(err)
							}
							if got != want {
								t.Fatalf("%s %q @%.2fpt w=%d max=%d pass %d: memo %+v, fresh %+v", font, text[:min(len(text), 20)], pt, w, maxLines, pass, got, want)
							}
						}
					}
				}
			}
		}
	}
}

func TestLineWidthMemoMatchesUncachedMeasurement(t *testing.T) {
	ff, _, _ := fontcache.Resolve("Liberation Sans", "Arial")
	if ff == nil {
		t.Skip("no font cache")
	}
	for _, text := range []string{"Managemen", strings.Repeat("W", 200), "two\nlines of text"} {
		for _, pt := range []float64{10, 12, 18.5, 28} {
			for _, bold := range []bool{false, true} {
				want := measureLineWidthUncached(ff, text, pt, bold)
				for pass := 0; pass < 2; pass++ {
					got, err := MeasureStyledLineWidth(text, "Liberation Sans", pt, bold)
					if err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Fatalf("%q @%.1fpt bold=%v pass %d: memo %d, fresh %d", text[:min(len(text), 20)], pt, bold, pass, got, want)
					}
				}
				if !bold {
					if got, _ := MeasureLineWidth(text, "Liberation Sans", pt); got != want {
						t.Fatalf("MeasureLineWidth %q @%.1fpt = %d, want %d", text[:min(len(text), 20)], pt, got, want)
					}
				}
			}
		}
	}
}

func TestMeasureMemosAreBounded(t *testing.T) {
	for i := 0; i < measureMemoMax+50; i++ {
		pt := 12 + float64(i)/1000
		if _, err := MeasureRun("x", "Liberation Sans", pt, 914400, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := MeasureLineWidth("x", "Liberation Sans", pt); err != nil {
			t.Fatal(err)
		}
	}
	if n := runMeasureMemo.size(); n > measureMemoMax {
		t.Fatalf("run memo holds %d entries, bound is %d", n, measureMemoMax)
	}
	if n := lineWidthMemo.size(); n > measureMemoMax {
		t.Fatalf("line-width memo holds %d entries, bound is %d", n, measureMemoMax)
	}
}

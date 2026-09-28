package svggen

import (
	"testing"
)

// go-slide-creator-7oz3c: a bidi control + symbol + combining mark panicked
// the HarfBuzz shaper. Chart labels go through DrawText / MeasureText.
func TestDrawTextSurvivesShaperPanic(t *testing.T) {
	b := NewSVGBuilder(400, 200)
	labels := []string{
		string([]rune{0x202E, 0x1F600, 0x0301}) + "Q3",
		string([]rune{0x200F, 0x2713, 0x0301}) + " done",
	}
	for _, s := range labels {
		if w, _ := b.MeasureText(s); w <= 0 {
			t.Errorf("MeasureText(%+q) = %v, want positive", s, w)
		}
		b.DrawText(s, 10, 50, TextAlignLeft, TextBaselineAlphabetic)
	}
	if _, err := b.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

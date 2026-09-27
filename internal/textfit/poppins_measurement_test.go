package textfit

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

func TestPoppinsNativeMeasurementWrapsAtNativeWidth(t *testing.T) {
	fontcache.Reset()
	const text = "Illustrative test data for review"
	width, err := MeasureLineWidth(text, "Poppins Light", 18)
	if err != nil {
		t.Fatal(err)
	}
	// Exact original assets and canvas metrics are pinned in fontcache tests.
	// This checks native binding and a real missed-wrap case, not the separately
	// tracked rounded point-to-mm conversion's physical precision.
	if width <= 3240000 || width >= 3420000 {
		t.Fatalf("native width=%d, outside 90–95 mm", width)
	}
	result, err := MeasureStyledRuns(StyledMeasureParams{Runs: []StyledRun{{Text: text}}, FontName: "Poppins Light", FontPt: 18, WidthEMU: 3240000, MaxLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.FontFamily != "Poppins Light" || result.FontSubstituted || result.Lines != 2 || result.Fits || result.OverflowChars != len([]rune(text)) {
		t.Fatalf("native wrapping not measured: %+v", result)
	}
}

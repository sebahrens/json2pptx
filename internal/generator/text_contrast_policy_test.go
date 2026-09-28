package generator

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func gridTextCell(fill, color string, hundredths int, bold bool) []byte {
	b := ""
	if bold {
		b = ` b="1"`
	}
	return []byte(`<p:sp><p:spPr><a:solidFill><a:srgbClr val="` + fill + `"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="` + strconv.Itoa(hundredths) + `"` + b + `><a:solidFill><a:srgbClr val="` + color + `"/></a:solidFill></a:rPr><a:t>Label</a:t></a:r></a:p></p:txBody></p:sp>`)
}

// White on midnight-blue's accent2 reads 4.43; black would read 4.74. The swap
// buys nothing visible and paints the accent card black, so it is skipped
// (go-slide-creator-z668n).
func TestShapeGridContrast_MarginalGainKeepsAuthoredColor(t *testing.T) {
	theme := []types.ThemeColor{{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"}}
	cell := gridTextCell("D4463A", "FFFFFF", 1300, true)
	fixed, swaps := enforceShapeGridContrast([][]byte{cell}, theme, nil, 0, "#FFFFFF")
	if len(swaps) != 0 || string(fixed[0]) != string(cell) {
		t.Errorf("marginal swap applied: %+v", swaps)
	}
	pairs := []ContrastPreflightPair{{Foreground: "#FFFFFF", Background: "#D4463A", TextPt: 13, Bold: true, Source: "shape_grid"}}
	if got := DetectContrastPreflight(pairs, theme); len(got) != 0 {
		t.Errorf("preflight must agree with the renderer, got %+v", got)
	}
}

// On one fill a slide shows one text colour: once a small white label on the
// orange flips to black, a larger white heading on the same orange (which
// would pass 3:1 alone) flips with it (go-slide-creator-z668n).
func TestShapeGridContrast_OneTextColorPerFill(t *testing.T) {
	theme := []types.ThemeColor{{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#000000"}}
	small := gridTextCell("FD5108", "FFFFFF", 1300, true)
	large := gridTextCell("FD5108", "FFFFFF", 2400, true)
	other := gridTextCell("2E5090", "FFFFFF", 1300, true)
	fixed, swaps := enforceShapeGridContrast([][]byte{small, large, other}, theme, nil, 0, "#FFFFFF")
	for i := 0; i < 2; i++ {
		if !strings.Contains(string(fixed[i]), `val="000000"`) {
			t.Errorf("cell %d on the orange kept white: %s", i, fixed[i])
		}
	}
	if string(fixed[2]) != string(other) {
		t.Errorf("readable white on a different fill must not change: %s", fixed[2])
	}
	if len(swaps) != 2 || swaps[1].Path != "/slides/0/shape_grid/shapes/1" {
		t.Errorf("want one swap per recoloured cell, got %+v", swaps)
	}
}

// The pattern ink fix and the render-time pass share one large-text bar.
func TestContrastThresholdMatchesPatternInk(t *testing.T) {
	for _, tc := range []struct {
		pt   float64
		bold bool
		want float64
	}{
		{13, true, 4.5}, {14, true, 3}, {17, false, 4.5}, {18, false, 3}, {0, false, 4.5},
	} {
		if got := contrastThresholdFor(tc.pt, tc.bold); tc.pt > 0 && got != tc.want {
			t.Errorf("contrastThresholdFor(%v, %v) = %v, want %v", tc.pt, tc.bold, got, tc.want)
		}
	}
}

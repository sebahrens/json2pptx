package generator

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Reporting a shrink that leaves text unreadable (go-slide-creator-wvr0).
//
// Native diagram shapes now store the shrink PowerPoint should apply, so the
// file renders the same everywhere. That makes a previously invisible fact
// visible: a quadrant holding twelve bullets is shrunk to 28% — about 3.4pt —
// and nobody is going to read it. The shrink is honest; the deck is not fine.
//
// The scan reads the written slide XML rather than the builders, so it covers
// every shape that carries a computed scale without threading a finding channel
// through twenty call sites.

// autofitScaleRE captures the fontScale of a normAutofit that carries one.
var autofitScaleRE = regexp.MustCompile(`<a:normAutofit fontScale="(\d+)"`)

// shapeRunSizeRE captures a run's declared size in hundredths of a point.
var shapeRunSizeRE = regexp.MustCompile(`sz="(\d+)"`)

// autofitScaleDenominator converts OOXML percent-thousandths to a 0..1 scale.
const autofitScaleDenominator = 100000.0

// reportUnreadableAutofit emits TEXT_BELOW_READABLE_MIN for every shape on the
// slide whose stored autofit scale takes its smallest text under the floor for
// the deck's viewing mode.
func (ctx *singlePassContext) reportUnreadableAutofit(slideData []byte, slideNum int) {
	slideIndex := slideNum - ctx.calculateStartingSlideNum()
	if slideIndex < 0 {
		return
	}
	for _, shape := range splitShapeElements(string(slideData)) {
		m := autofitScaleRE.FindStringSubmatch(shape)
		if m == nil {
			continue
		}
		scaleThousandths, err := strconv.Atoi(m[1])
		if err != nil || scaleThousandths <= 0 {
			continue
		}
		smallest := smallestRunSizeHPt(shape)
		if smallest <= 0 {
			continue
		}
		effective := int(float64(smallest) * float64(scaleThousandths) / autofitScaleDenominator)
		if f := NewReadabilityFinding(ReadabilityFindingInput{
			Path:              slidepath.Slide(slideIndex),
			Mode:              ctx.viewingMode,
			Role:              tokens.TextRoleBody,
			EffectiveHPt:      effective,
			Paragraphs:        countParagraphs(shape),
			MeasurementSource: "generated",
			Context: fmt.Sprintf("autofit %d%% to fit the shape",
				scaleThousandths*100/int(autofitScaleDenominator)),
		}); f != nil {
			ctx.emitFitFinding(*f)
		}
	}
}

// splitShapeElements returns each <p:sp>…</p:sp> element in a slide.
func splitShapeElements(slideXML string) []string {
	return shapeElementRE.FindAllString(slideXML, -1)
}

var shapeElementRE = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
var paragraphElementRE = regexp.MustCompile(`<a:p>`)

// smallestRunSizeHPt returns the smallest declared run size in a shape, in
// hundredths of a point, or 0 when no run declares one.
func smallestRunSizeHPt(shape string) int {
	smallest := 0
	for _, m := range shapeRunSizeRE.FindAllStringSubmatch(shape, -1) {
		v, err := strconv.Atoi(m[1])
		if err != nil || v <= 0 {
			continue
		}
		if smallest == 0 || v < smallest {
			smallest = v
		}
	}
	return smallest
}

// countParagraphs counts the paragraphs in a shape, which decides whether the
// finding suggests shortening or splitting.
func countParagraphs(shape string) int {
	return len(paragraphElementRE.FindAllString(shape, -1))
}

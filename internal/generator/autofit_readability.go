package generator

import (
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

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
var autofitScaleRE = regexp.MustCompile(`<a:normAutofit\b[^>]*\bfontScale="(\d+)"`)

// shapeRunSizeRE captures a run's declared size in hundredths of a point.
var shapeRunSizeRE = regexp.MustCompile(`sz="(\d+)"`)
var renderedShapeIDRE = regexp.MustCompile(`<p:cNvPr\b[^>]*\bid="(\d+)"`)
var renderedShapeTextRE = regexp.MustCompile(`(?s)<a:t>(.*?)</a:t>`)

// autofitScaleDenominator converts OOXML percent-thousandths to a 0..1 scale.
const autofitScaleDenominator = 100000.0

// Missing scale means only the declared upper-bound size is known. Invalid
// explicit scales are not measurements and must not be treated as shrinkage.
func storedShapeFontScale(shape string) (int, bool) {
	m := autofitScaleRE.FindStringSubmatch(shape)
	if m == nil {
		return int(autofitScaleDenominator), false
	}
	scale, err := strconv.Atoi(m[1])
	if err != nil || scale <= 0 {
		return 0, true
	}
	return scale, true
}

// reportUnreadableAutofit emits TEXT_BELOW_READABLE_MIN for every shape on the
// slide whose stored autofit scale takes its smallest text under the floor for
// the deck's viewing mode.
func (ctx *singlePassContext) reportUnreadableAutofit(slideData []byte, slideNum int) {
	slideIndex := slideNum - ctx.calculateStartingSlideNum()
	if slideIndex < 0 {
		return
	}
	for _, shape := range splitShapeElements(string(slideData)) {
		scaleThousandths, hasStoredScale := storedShapeFontScale(shape)
		if scaleThousandths <= 0 {
			continue
		}
		smallest := smallestPopulatedRunSizeHPt(shape)
		if smallest <= 0 {
			continue
		}
		// A missing scale leaves the exact authored size as an upper bound:
		// bare renderer autofit can shrink it, never make tiny text readable.
		// Do not infer a semantic-role violation from unknown renderer shrink.
		if !hasStoredScale && smallest >= tokens.FootnoteMinHPt {
			continue
		}
		effective := int(float64(smallest) * float64(scaleThousandths) / autofitScaleDenominator)
		context := fmt.Sprintf("autofit %d%% to fit the shape", scaleThousandths*100/int(autofitScaleDenominator))
		if !hasStoredScale {
			context = "declared font size before any renderer shrink"
		}
		shapePath := slidepath.Slide(slideIndex)
		if id := renderedShapeIDRE.FindStringSubmatch(shape); len(id) == 2 {
			shapePath += "/rendered_shapes/" + id[1]
		}
		if f := NewReadabilityFinding(ReadabilityFindingInput{
			Path:              shapePath,
			Mode:              ctx.viewingMode,
			Role:              tokens.TextRoleBody,
			EffectiveHPt:      effective,
			Paragraphs:        countParagraphs(shape),
			MeasurementSource: "generated",
			Context:           context,
		}); f != nil {
			// This scale is written into the output, not a preflight estimate.
			// Below even the most compact footnote floor, no semantic role can
			// make the populated text acceptable. Do not publish or suggest
			// deleting source. Higher sizes still need role-aware visual review.
			if effective < tokens.FootnoteMinHPt {
				f.Action = "refuse"
				f.Message = strings.TrimSuffix(strings.TrimSuffix(f.Message, "; shorten the text"), "; split the text") + "; preserve all source at a readable size"
				f.Fix = nil
			}
			if matches := renderedShapeTextRE.FindAllStringSubmatch(shape, -1); len(matches) > 0 {
				var parts []string
				for _, m := range matches {
					if s := strings.TrimSpace(html.UnescapeString(m[1])); s != "" {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 && f.Fix != nil {
					f.Fix.Params["rendered_shape_text"] = strings.Join(parts, " ")
				}
			}
			ctx.emitFitFinding(*f)
		}
	}
}

// Only populated runs establish an actual written size. Unused list levels,
// empty prompts and end-paragraph cursor styles must never trigger a refusal.
func smallestPopulatedRunSizeHPt(shape string) int {
	var parsed shapeXML
	if err := xml.Unmarshal([]byte(shape), &parsed); err != nil || parsed.TextBody == nil {
		return 0
	}
	smallest := 0
	for _, p := range parsed.TextBody.Paragraphs {
		if size := smallestPopulatedParagraphRunSizeHPt(p); size > 0 && (smallest == 0 || size < smallest) {
			smallest = size
		}
	}
	return smallest
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

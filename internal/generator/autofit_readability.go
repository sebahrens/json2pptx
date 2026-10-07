package generator

import (
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
// the deck's viewing mode. Grid paragraphs whose source role is known were
// already judged against their role's floor (reportGridReadability).
func (ctx *singlePassContext) reportUnreadableAutofit(slideData []byte, slideNum int) {
	slideIndex := slideNum - ctx.calculateStartingSlideNum()
	if slideIndex < 0 {
		return
	}
	shapePath := func(id string) string {
		p := slidepath.Slide(slideIndex)
		if id != "" {
			p += "/rendered_shapes/" + id
		}
		return p
	}
	var roles map[uint32][]tokens.TextRole
	if spec, ok := ctx.slideContentMap[slideNum]; ok {
		roles = spec.GridTextRoles
	}
	for _, f := range unreadableAutofitFindings(string(slideData), ctx.viewingMode, shapePath, roles) {
		ctx.applyNativeFitBudget(&f, slideIndex)
		ctx.emitFitFinding(f)
	}
}

// applyNativeFitBudget adds, to a readability finding raised on a shape of a
// native diagram whose layout measured a budget, what the diagram holds at the
// authored size. A shrink is then reported with the count and length to cut
// to, never as a bare size (go-slide-creator-6ne1m).
func (ctx *singlePassContext) applyNativeFitBudget(f *patterns.FitFinding, slideIndex int) {
	const marker = "/rendered_shapes/"
	at := strings.Index(f.Path, marker)
	if at < 0 {
		return
	}
	id, err := strconv.ParseUint(f.Path[at+len(marker):], 10, 32)
	if err != nil {
		return
	}
	for _, s := range ctx.nativeShapeSources {
		if s.slideIndex == slideIndex && uint32(id) >= s.lo && uint32(id) < s.hi {
			s.budget.apply(f, s.diagramType)
			return
		}
	}
}

// unreadableAutofitFindings is reportUnreadableAutofit's scan of written shape
// XML, shared with the native-diagram validate preflight so the two measure
// the same thing. shapePath names a shape by its cNvPr id ("" when it has none).
//
// The scan has no paragraph semantics, so it holds text to the body floor.
// roles (shape cNvPr id -> per-paragraph source roles, as generation records
// them for shape_grid cells) exempts the paragraphs whose role is known: the
// role-aware grid check judges those against their own floor, and a 10.08pt
// KPI caption under its 36pt figure is not 12pt body copy
// (go-slide-creator-ntvhh). A shape whose recorded roles do not line up with
// its paragraphs is scanned whole, as the grid check then skips it.
func unreadableAutofitFindings(slideXML string, mode tokens.ViewingMode, shapePath func(id string) string, roles map[uint32][]tokens.TextRole) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, shape := range splitShapeElements(slideXML) {
		scaleThousandths, hasStoredScale := storedShapeFontScale(shape)
		if scaleThousandths <= 0 {
			continue
		}
		smallest := smallestUnroledRunSizeHPt(shape, roles)
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
		} else if scaleThousandths >= int(autofitScaleDenominator) {
			// A size set outright so that peers share it (bmcApplyPeerSize).
			context = "set below the body size so that every section fits at one size"
		}
		id := ""
		if m := renderedShapeIDRE.FindStringSubmatch(shape); len(m) == 2 {
			id = m[1]
		}
		if f := NewReadabilityFinding(ReadabilityFindingInput{
			Path:              shapePath(id),
			Mode:              mode,
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
			out = append(out, *f)
		}
	}
	return out
}

// Only populated runs establish an actual written size. Unused list levels,
// empty prompts and end-paragraph cursor styles must never trigger a refusal.
// Paragraphs carrying a known source role in roles are left to the role-aware
// grid check.
func smallestUnroledRunSizeHPt(shape string, roles map[uint32][]tokens.TextRole) int {
	var parsed shapeXML
	if err := xml.Unmarshal([]byte(shape), &parsed); err != nil || parsed.TextBody == nil {
		return 0
	}
	paragraphRoles := roles[parsed.NonVisualProperties.ConnectionNonVisual.ID]
	if len(paragraphRoles) != len(parsed.TextBody.Paragraphs) {
		paragraphRoles = nil
	}
	smallest := 0
	for i, p := range parsed.TextBody.Paragraphs {
		if paragraphRoles != nil && paragraphRoles[i] != "" {
			continue
		}
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

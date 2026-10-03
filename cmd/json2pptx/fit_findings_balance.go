package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Layout-balance and title-quality advisories (go-slide-creator-u9xfy,
// go-slide-creator-d830i). All are review-level and exempt from the gate's
// problem-slide share: they describe how a slide reads, not a broken render.

const (
	// verticalImbalanceMinGapEMU is the smallest empty band worth reporting.
	verticalImbalanceMinGapEMU = 1143000 // 1.25in
	// verticalImbalanceSkewFrac is how much larger (as a share of the content
	// zone's height) the empty band on one side must be than on the other.
	verticalImbalanceSkewFrac = 0.30
	// bodyLineToleranceEMU is how far (6pt) below the template's body line a
	// block may start and still count as hung from it.
	bodyLineToleranceEMU = 6 * 12700
	// hungBlockMaxEmptyFrac is the share of the content zone a block hung
	// from the body line may leave empty beneath it. Native body text that
	// stops two thirds of the way down is an ordinary slide; a row of cards
	// in the top half over an empty bottom half is not, and scored 100
	// (go-slide-creator-wwmod). Measured on the ink, so a block the layout
	// centres or stretches is unaffected whatever its pattern.
	hungBlockMaxEmptyFrac = 0.40

	// sparsePlaceholderMaxFrac: body text filling less than this share of its
	// placeholder's height reads as a few lines stuck to the top of an empty box.
	sparsePlaceholderMaxFrac = 0.25
	// sparsePlaceholderMinBoxFrac: only placeholders that are a real share of
	// the slide qualify; a short caption box is supposed to be short.
	sparsePlaceholderMinBoxFrac = 0.35
)

// checkVerticalImbalance reports a grid / pattern slide whose content sits
// against one edge of the content zone and leaves a large empty band on the
// other — an empty band above a roadmap, or a row of cards pinned to the top.
// Centred content with equal margins is balanced and is not reported;
// sparseness as such is SLIDE_UNDERUSED's job.
//
// bodyLine is the template's body line (ContentZone.BodyTop, 0 when
// unknown). A block whose top sits on it hangs from the line native body
// text starts on — the deliberate placement for content-sized blocks
// (go-slide-creator-e17xy) — so the band below it is the slide's normal
// bottom margin, not imbalance, until it reaches hungBlockMaxEmptyFrac of the
// zone: content in the top half only is reported wherever it hangs from.
//
// takeawayReserved says the zone already ends above the slide's takeaway
// band (resolveGridGeometry reserves it); the band below the ink is then
// empty space between the content and the takeaway, and counts.
func checkVerticalImbalance(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, si int, patternName string, bodyLine int64, takeawayReserved ...bool) *patterns.FitFinding {
	if safe.CY <= 0 || len(ink) == 0 || hasBodyPlaceholderContent(slide) {
		return nil
	}
	first, last := safe.Y+safe.CY, safe.Y
	for _, r := range ink {
		r = intersectRect(r, safe)
		if r.CY <= 0 {
			continue
		}
		first = minI64(first, r.Y)
		last = maxI64(last, r.Y+r.CY)
	}
	if last <= first {
		return nil
	}
	top, bottom := first-safe.Y, safe.Y+safe.CY-last
	if strings.TrimSpace(slide.Takeaway) != "" && !(len(takeawayReserved) > 0 && takeawayReserved[0]) {
		// The injected takeaway band renders in the bottom of the zone,
		// outside the grid ink; the space below the grid is not empty.
		bottom = 0
	}
	band, side := top, "above"
	if bottom > top {
		band, side = bottom, "below"
	}
	skew := math.Abs(float64(top-bottom)) / float64(safe.CY)
	if band < verticalImbalanceMinGapEMU || skew < verticalImbalanceSkewFrac {
		// The content reaches both ends of the zone, or sits centred in it.
		// A block at the top and a conclusion band at the bottom with an
		// empty band between them is the same top-heavy slide with its
		// callout moved down (go-slide-creator-wwmod).
		if gap := largestInkGap(ink, safe); float64(gap) >= hungBlockMaxEmptyFrac*float64(safe.CY) && gap >= verticalImbalanceMinGapEMU {
			return verticalImbalanceFinding(si, patternName, gap, "between the content and the band beneath", minI64(top, bottom), safe)
		}
		return nil
	}
	bandFrac := float64(band) / float64(safe.CY)
	if side == "below" && bodyLine > 0 && first <= max(safe.Y, bodyLine)+bodyLineToleranceEMU && bandFrac < hungBlockMaxEmptyFrac {
		return nil
	}
	return verticalImbalanceFinding(si, patternName, band, side+" it", minI64(top, bottom), safe)
}

// largestInkGap is the tallest horizontal band inside the content zone that
// no ink rectangle touches, between the first and the last ink.
func largestInkGap(ink []pptx.RectEmu, safe pptx.RectEmu) int64 {
	spans := make([][2]int64, 0, len(ink))
	for _, r := range ink {
		r = intersectRect(r, safe)
		if r.CY > 0 {
			spans = append(spans, [2]int64{r.Y, r.Y + r.CY})
		}
	}
	slices.SortFunc(spans, func(a, b [2]int64) int { return cmp.Compare(a[0], b[0]) })
	var gap, end int64
	for i, s := range spans {
		if i > 0 && s[0]-end > gap {
			gap = s[0] - end
		}
		end = maxI64(end, s[1])
	}
	return gap
}

func verticalImbalanceFinding(si int, patternName string, band int64, where string, other int64, safe pptx.RectEmu) *patterns.FitFinding {
	bandFrac := float64(band) / float64(safe.CY)
	side := strings.Fields(where)[0]
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    slidepath.Slide(si),
			Code:    patterns.ErrCodeVerticalImbalance,
			Message: fmt.Sprintf("content leaves a %.1fin empty band %s (%.0f%% of the content area; %.1fin on the other side) — the slide reads top-heavy or bottom-heavy", float64(band)/914400, where, 100*bandFrac, float64(other)/914400),
			Fix: &patterns.FixSuggestion{
				Kind: "add_detail_or_resize",
				Params: map[string]any{
					"empty_band_in":   round1(float64(band) / 914400),
					"empty_band_pct":  math.Round(100 * bandFrac),
					"empty_band_side": side,
					"hint":            "centre or stretch the block (pattern vertical_align / shape_grid vertical_align), add a supporting zone in the empty band, or choose a pattern that fills the area with this amount of content",
				},
			},
		},
		Action: "review",
	}
}

const (
	// horizontalImbalanceMinBandFrac is the share of the content zone's width
	// an empty band beside the content must reach to be reported, and
	// horizontalImbalanceSkewFrac how much wider it must be than the band on
	// the other side: rows of short text that end mid-slide fill the left
	// half only (go-slide-creator-wwmod).
	horizontalImbalanceMinBandFrac = 0.40
	horizontalImbalanceSkewFrac    = 0.30
)

// checkHorizontalImbalance reports a grid / pattern slide whose visible
// content sits against one side of the content zone. It is measured on the
// ink — a text block counts at its measured width, a filled shape at its
// bounds — so a row layout whose text reaches across the slide is balanced
// and the same layout with one short phrase per row is not.
func checkHorizontalImbalance(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, si int, patternName string) *patterns.FitFinding {
	if safe.CX <= 0 || len(ink) == 0 || hasBodyPlaceholderContent(slide) {
		return nil
	}
	first, last := safe.X+safe.CX, safe.X
	for _, r := range ink {
		r = intersectRect(r, safe)
		if r.CX <= 0 {
			continue
		}
		first = minI64(first, r.X)
		last = maxI64(last, r.X+r.CX)
	}
	if last <= first {
		return nil
	}
	left, right := first-safe.X, safe.X+safe.CX-last
	band, side := left, "left of"
	if right > left {
		band, side = right, "right of"
	}
	bandFrac := float64(band) / float64(safe.CX)
	skew := math.Abs(float64(left-right)) / float64(safe.CX)
	if bandFrac < horizontalImbalanceMinBandFrac || skew < horizontalImbalanceSkewFrac {
		return nil
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    slidepath.Slide(si),
			Code:    patterns.ErrCodeHorizontalImbalance,
			Message: fmt.Sprintf("content leaves a %.1fin empty band %s it (%.0f%% of the content width) — the slide fills one side only", float64(band)/914400, side, 100*bandFrac),
			Fix: &patterns.FixSuggestion{
				Kind: "add_detail_or_resize",
				Params: map[string]any{
					"empty_band_in":   round1(float64(band) / 914400),
					"empty_band_pct":  math.Round(100 * bandFrac),
					"empty_band_side": strings.TrimSuffix(side, " of"),
					"hint":            "use the empty side (evidence, a chart, an image or a so-what column via compose), choose a pattern that spreads this content across the width, or write each row as a full sentence",
				},
			},
		},
		Action: "review",
	}
}

// collectSparsePlaceholderFindings flags a bullet / text body whose measured
// height fills only a small share of a large body placeholder: four short
// bullets in the top quarter of an otherwise empty slide produced no finding
// at all (go-slide-creator-u9xfy).
func collectSparsePlaceholderFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideHeight int64) []patterns.FitFinding {
	if input == nil || len(layouts) == 0 {
		return nil
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}
	predicted := predictSlideLayouts(input, layouts)
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := &input.Slides[si]
		if slide.ShapeGrid != nil || slide.Pattern != nil || slide.Compose != nil || !slideCarriesArgument(*slide, layouts...) {
			continue
		}
		ci, item := soleBodyItem(slide)
		if item == nil || si >= len(predicted) || predicted[si] == nil {
			continue
		}
		ph := findPlaceholderByID(item.PlaceholderID, predicted[si].Placeholders)
		if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
			continue
		}
		if float64(ph.Bounds.Height) < sparsePlaceholderMinBoxFrac*float64(slideHeight) {
			continue
		}
		paras := extractContentParagraphs(item)
		if len(paras) == 0 {
			continue
		}
		size := ph.FontSize
		if item.FontSize != nil && *item.FontSize > 0 {
			size = int(*item.FontSize * 100)
		}
		if size <= 0 {
			size = 1800
		}
		h, err := textfit.MeasureHeight(textfit.Params{
			WidthEMU:    ph.Bounds.Width,
			HeightEMU:   ph.Bounds.Height,
			FontSizeHPt: size,
			FontName:    ph.FontFamily,
			Paragraphs:  paras,
		})
		if err != nil || h <= 0 {
			continue
		}
		frac := float64(h) / float64(ph.Bounds.Height)
		if frac >= sparsePlaceholderMaxFrac {
			continue
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: slidepath.ContentIndex(si, ci),
				Code: patterns.ErrCodeSparsePlaceholder,
				Message: fmt.Sprintf("%d paragraph(s) fill about %.0f%% of the %.1fin-tall %s placeholder — the text sits at the top of a mostly empty slide",
					len(paras), 100*frac, float64(ph.Bounds.Height)/914400, item.PlaceholderID),
				Fix: &patterns.FixSuggestion{
					Kind: "add_detail_or_resize",
					Params: map[string]any{
						"text_height_pct": math.Round(100 * frac),
						"threshold_pct":   math.Round(100 * sparsePlaceholderMaxFrac),
						"hint":            "turn the points into a pattern (card-grid, icon-row, labeled-rows), add evidence or a takeaway, or merge the slide with its neighbour",
					},
				},
			},
			Action: "review",
		})
	}
	return out
}

// soleBodyItem returns the slide's single text-bearing body item when every
// other content item is a title / subtitle. Slides that also carry a chart,
// table, image or a second body are laid out by that content, not this text.
func soleBodyItem(slide *SlideInput) (int, *ContentInput) {
	idx := -1
	for i := range slide.Content {
		c := &slide.Content[i]
		if isHeadlinePlaceholderID(c.PlaceholderID) || c.PlaceholderID == "subtitle" {
			continue
		}
		switch c.Type {
		case "text", "bullets", "body_and_bullets", "bullet_groups":
		default:
			return -1, nil
		}
		if idx >= 0 {
			return -1, nil
		}
		idx = i
	}
	if idx < 0 {
		return -1, nil
	}
	return idx, &slide.Content[idx]
}

// titleTopicExempt are navigation titles that are topics by design.
var titleTopicExempt = map[string]bool{
	"agenda": true, "contents": true, "table of contents": true, "appendix": true,
	"q&a": true, "questions": true, "thank you": true, "thanks": true,
	"contact": true, "contacts": true, "references": true, "sources": true,
	"glossary": true, "disclaimer": true, "backup": true,
}

// titleVerbs is a small lexicon of verbs common in consulting action titles.
// Suffix -s forms are matched by stripping the "s"; -ed forms by suffix.
var titleVerbs = map[string]bool{
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"has": true, "have": true, "had": true, "will": true, "can": true, "must": true,
	"should": true, "need": true, "drive": true, "grow": true, "fall": true,
	"rise": true, "lead": true, "win": true, "beat": true, "cut": true, "save": true,
	"lift": true, "double": true, "triple": true, "halve": true, "outperform": true,
	"deliver": true, "enable": true, "unlock": true, "require": true, "reduce": true,
	"increase": true, "improve": true, "accelerate": true, "expand": true,
	"shift": true, "make": true, "take": true, "keep": true, "stay": true,
	"remain": true, "become": true, "pay": true, "work": true, "matter": true,
	"lag": true, "trail": true, "dominate": true, "stall": true, "decline": true,
	"slow": true, "outpace": true, "exceed": true, "miss": true, "hit": true,
	"close": true, "open": true, "add": true, "lose": true, "gain": true,
	"fund": true, "build": true, "launch": true, "scale": true, "shrink": true,
}

func isTitleVerb(word string) bool {
	w := strings.ToLower(strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) }))
	if w == "" {
		return false
	}
	if titleVerbs[w] || (strings.HasSuffix(w, "s") && titleVerbs[strings.TrimSuffix(w, "s")]) ||
		(strings.HasSuffix(w, "es") && titleVerbs[strings.TrimSuffix(w, "es")]) {
		return true
	}
	return len(w) > 4 && strings.HasSuffix(w, "ed")
}

// collectTitleNotActionFindings flags content-slide titles that are not action
// titles: longer than 15 words, a stock label, or a topic with no verb and no
// number (go-slide-creator-d830i, go-slide-creator-kuurd). Review-weighted:
// at info it cost nothing and a deck of topic titles scored 99 and passed.
func collectTitleNotActionFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := input.Slides[si]
		if !slideQualifiesForDuplicateTitleCheck(slide) || !slideCarriesArgument(slide, layouts...) {
			continue
		}
		phID, text, ci := extractTitleTextAt(slide)
		if text == "" || titleExemptFromAction(input, si, text) {
			continue
		}
		// A slide that states its point in a takeaway may title the topic —
		// the DeckSpec convention (title + takeaway) — but not with a stock
		// label or a 20-word headline.
		reason := titleNotActionReason(text, strings.TrimSpace(slide.Takeaway) != "")
		if reason == "" {
			continue
		}
		if phID == "" {
			phID = "title"
		}
		words := len(strings.Fields(text))
		msg, hint := titleNotActionMessage(si, text, reason, words)
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    slidepath.ContentIndex(si, ci),
				Code:    patterns.ErrCodeTitleNotAction,
				Message: msg,
				Fix: &patterns.FixSuggestion{
					Kind: "review",
					Params: map[string]any{
						"placeholder_id": phID,
						"current_words":  words,
						"max_words":      titleActionMaxWords,
						"reason":         reason,
						"hint":           hint,
					},
				},
			},
			Action: "review",
		})
	}
	return out
}

// titleTooLongFinding reports a content-slide title that renders on more than
// two lines at the size generation writes (go-slide-creator-d830i). It returns
// nil when the title fits in two lines or cannot be measured.
func titleTooLongFinding(path, title, fontName string, caps bool, sizeHPt int, widthEMU int64) *patterns.FitFinding {
	if title == "" || sizeHPt <= 0 || widthEMU <= 0 {
		return nil
	}
	measureText := title
	if caps {
		measureText = strings.ToUpper(title)
	}
	fontPt := float64(sizeHPt) / 100
	m, err := textfit.MeasureRun(measureText, fontName, fontPt, widthEMU, 0)
	if err != nil || m.Lines <= 2 {
		return nil
	}
	budget := 0
	words := strings.Fields(measureText)
	for n := len(words) - 1; n > 0; n-- {
		prefix := strings.Join(words[:n], " ")
		if pm, err := textfit.MeasureRun(prefix, fontName, fontPt, widthEMU, 0); err == nil && pm.Lines <= 2 {
			budget = len([]rune(prefix))
			break
		}
	}
	params := map[string]any{"current_chars": len([]rune(title)), "rendered_lines": m.Lines, "max_lines": 2}
	if budget > 0 {
		params["max_chars"] = budget
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "placeholder",
			Path:    path,
			Code:    patterns.ErrCodeTitleTooLong,
			Message: fmt.Sprintf("title renders on %d lines at %.0fpt — a headline over two lines stops reading as one claim; tighten it or move detail into the body", m.Lines, fontPt),
			Fix:     &patterns.FixSuggestion{Kind: "shorten_title", Params: params},
		},
		Action: "review",
	}
}

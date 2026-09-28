package main

import (
	"fmt"
	"math"
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
func checkVerticalImbalance(ink []pptx.RectEmu, safe pptx.RectEmu, slide *SlideInput, si int, patternName string) *patterns.FitFinding {
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
	if strings.TrimSpace(slide.Takeaway) != "" {
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
		return nil
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    slidepath.Slide(si),
			Code:    patterns.ErrCodeVerticalImbalance,
			Message: fmt.Sprintf("content leaves a %.1fin empty band %s it (%.1fin on the other side) — the slide reads top-heavy or bottom-heavy", float64(band)/914400, side, float64(minI64(top, bottom))/914400),
			Fix: &patterns.FixSuggestion{
				Kind: "add_detail_or_resize",
				Params: map[string]any{
					"empty_band_in":   round1(float64(band) / 914400),
					"empty_band_side": side,
					"hint":            "centre the block (vertical_align), add a supporting zone or takeaway in the empty band, or size the grid to its content",
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

// titleNotActionMaxWords: a title with fewer words than this, no number and no
// verb names a topic ("Market Overview") rather than the slide's point.
const titleNotActionMaxWords = 4

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

// collectTitleNotActionFindings flags content-slide titles that name a topic
// instead of stating a point (go-slide-creator-d830i).
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
		// A slide that states its point in a takeaway may title the topic —
		// the DeckSpec convention (title + takeaway).
		if strings.TrimSpace(slide.Takeaway) != "" {
			continue
		}
		phID, text, ci := extractTitleTextAt(slide)
		if text == "" || titleTopicExempt[normalizeTitleText(strings.TrimRight(text, ".:!?"))] {
			continue
		}
		words := strings.Fields(text)
		if len(words) >= titleNotActionMaxWords || strings.ContainsAny(text, "0123456789") {
			continue
		}
		hasVerb := false
		for _, w := range words {
			if isTitleVerb(w) {
				hasVerb = true
				break
			}
		}
		if hasVerb {
			continue
		}
		if phID == "" {
			phID = "title"
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    slidepath.ContentIndex(si, ci),
				Code:    patterns.ErrCodeTitleNotAction,
				Message: fmt.Sprintf("slide %d: title %q names a topic, not the slide's point — state the takeaway as a sentence with a verb or a number", si+1, text),
				Fix: &patterns.FixSuggestion{
					Kind: "review",
					Params: map[string]any{
						"placeholder_id": phID,
						"current_words":  len(words),
						"hint":           "rewrite as an action title, e.g. \"Market Overview\" → \"Mid-market demand doubled while enterprise stalled\"",
					},
				},
			},
			Action: "info",
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

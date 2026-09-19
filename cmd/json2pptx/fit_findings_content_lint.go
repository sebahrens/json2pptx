package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Content lint budgets. Tuned to the consulting-deck readability heuristics:
// a 12-word headline is the upper end of a single line at 36–40pt; an
// 80-word body is the upper end of five tightly-packed bullets at 12pt;
// nesting beyond two levels collapses visual hierarchy.
const (
	maxHeadlineWords      = 12
	maxBodyWords          = 80
	maxBulletNestingDepth = 2
)

// collectContentLintFindings emits advisory findings when slide content
// exceeds readability budgets: HEADLINE_TOO_LONG (>12 words on title),
// BODY_TOO_LONG (>80 words on a text block), BULLET_NESTING_DEEP (bullets
// nested more than two levels). All findings have action "review"; they
// never block render.
func collectContentLintFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		for ci := range slide.Content {
			findings = append(findings, lintContentItem(si, ci, &slide.Content[ci])...)
		}
	}
	return findings
}

// lintContentItem dispatches the content lint checks for one content item by
// its type. The contentIdx is unused in path construction (paths target the
// placeholder ID, which is more stable across slide rearrangements) but is
// accepted so future authors can switch to indexed paths if needed.
func lintContentItem(slideIdx, _ int, content *ContentInput) []patterns.FitFinding {
	switch content.Type {
	case "text":
		return lintText(slideIdx, content)
	case "bullets":
		return lintBullets(slideIdx, content)
	case "body_and_bullets":
		return lintBodyAndBullets(slideIdx, content)
	case "bullet_groups":
		return lintBulletGroups(slideIdx, content)
	}
	return nil
}

// lintText applies the word budget, which differs for a headline placeholder.
func lintText(slideIdx int, content *ContentInput) []patterns.FitFinding {
	if content.TextValue == nil {
		return nil
	}
	wc := countWords(*content.TextValue)
	if isHeadlinePlaceholderID(content.PlaceholderID) {
		if wc > maxHeadlineWords {
			return []patterns.FitFinding{makeHeadlineFinding(slideIdx, content.PlaceholderID, wc)}
		}
		return nil
	}
	if wc > maxBodyWords {
		return []patterns.FitFinding{makeBodyFinding(slideIdx, content.PlaceholderID, wc)}
	}
	return nil
}

// lintBullets applies the nesting, word and ordered-list checks to a plain
// bullets list.
func lintBullets(slideIdx int, content *ContentInput) []patterns.FitFinding {
	if content.BulletsValue == nil {
		return nil
	}
	return lintBulletList(slideIdx, content.PlaceholderID, *content.BulletsValue, countBulletWords(*content.BulletsValue), 1)
}

// lintBodyAndBullets budgets the body, lead-out and bullets together.
func lintBodyAndBullets(slideIdx int, content *ContentInput) []patterns.FitFinding {
	v := content.BodyAndBulletsValue
	if v == nil {
		return nil
	}
	words := countWords(v.Body) + countBulletWords(v.Bullets) + countWords(v.TrailingBody)
	return lintBulletList(slideIdx, content.PlaceholderID, v.Bullets, words, 1)
}

// lintBulletList is the shared body of the bullet-bearing content types: the
// word budget counts everything on the placeholder, while nesting and ordered
// numbering are judged on the bullets themselves.
func lintBulletList(slideIdx int, phID string, bullets []string, words, baseDepth int) []patterns.FitFinding {
	var findings []patterns.FitFinding
	if d := maxBulletDepth(bullets, baseDepth); d > maxBulletNestingDepth {
		findings = append(findings, makeBulletDepthFinding(slideIdx, phID, d))
	}
	if words > maxBodyWords {
		findings = append(findings, makeBodyFinding(slideIdx, phID, words))
	}
	if f := numberedListFinding(slideIdx, phID, bullets); f != nil {
		findings = append(findings, *f)
	}
	return findings
}

// lintBulletGroups budgets every group's label, header, body and bullets
// together. In bullet_groups the header occupies level 1 and bullets render at
// level 2 by default, so indent inside a bullet string pushes it deeper.
func lintBulletGroups(slideIdx int, content *ContentInput) []patterns.FitFinding {
	v := content.BulletGroupsValue
	if v == nil {
		return nil
	}
	words := countWords(v.Body) + countWords(v.TrailingBody)
	maxDepth := 0
	for _, g := range v.Groups {
		words += countWords(g.GroupLabel) + countWords(g.Header) + countWords(g.Body)
		words += countBulletWords(g.Bullets)
		if d := maxBulletDepth(g.Bullets, 2); d > maxDepth {
			maxDepth = d
		}
	}

	var findings []patterns.FitFinding
	if words > maxBodyWords {
		findings = append(findings, makeBodyFinding(slideIdx, content.PlaceholderID, words))
	}
	if maxDepth > maxBulletNestingDepth {
		findings = append(findings, makeBulletDepthFinding(slideIdx, content.PlaceholderID, maxDepth))
	}
	return findings
}

// numberedListFinding reports typed "N. " prefixes the renderer will NOT turn
// into auto-numbering, because they print beside the layout's own bullet glyph
// as a double marker — the symptom that made ordered lists unusable in a
// placeholder at all (go-slide-creator-6or2).
//
// A complete list numbered from 1 is auto-numbered and its prefixes removed, so
// it draws nothing here. A single line that merely opens with a number is
// prose, not a list, and is left alone.
func numberedListFinding(slideIdx int, phID string, bullets []string) *patterns.FitFinding {
	if _, numbered := generator.NumberedList(bullets); numbered {
		return nil
	}
	prefixed := 0
	for _, bullet := range bullets {
		if generator.HasNumberedPrefix(bullet) {
			prefixed++
		}
	}
	if prefixed == 0 || len(bullets) < 2 {
		return nil
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: slidepath.Content(slideIdx, phID),
			Code: patterns.ErrCodeNumberedListNotApplied,
			Message: fmt.Sprintf(
				"slide %d: %d of %d bullets start with a typed \"N. \" but the list is not numbered 1..%d, so the numbers print beside the layout's bullet glyph as a double marker — number every bullet from 1 (the engine then supplies the numbers) or drop the prefixes",
				slideIdx+1, prefixed, len(bullets), len(bullets)),
			Fix: &patterns.FixSuggestion{
				Kind: "renumber_bullets",
				Params: map[string]any{
					"path":            slidepath.Content(slideIdx, phID),
					"prefixed":        prefixed,
					"total":           len(bullets),
					"expected_format": "1. , 2. , 3. …",
				},
			},
		},
		Action: "review",
	}
}

// isHeadlinePlaceholderID reports whether the placeholder ID names a
// title-class slot for the headline word-count budget. Subtitles are
// excluded — taglines have different length expectations.
func isHeadlinePlaceholderID(id string) bool {
	if isTitlePlaceholderID(id) {
		return true
	}
	switch id {
	case "headline", "ctrTitle":
		return true
	}
	return false
}

// countWords returns the number of whitespace-separated tokens in s.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// countBulletWords sums word counts across a bullet list.
func countBulletWords(bullets []string) int {
	total := 0
	for _, b := range bullets {
		total += countWords(b)
	}
	return total
}

// bulletIndentDepth measures the nesting depth implied by a bullet's leading
// whitespace. Each tab counts as one indent unit; two leading spaces count
// as one indent unit. Markdown bullet markers (`-`, `*`, `•`) on the leading
// edge are ignored — the indent that precedes them is what conveys nesting.
func bulletIndentDepth(s string) int {
	units := 0
	spaces := 0
	for _, r := range s {
		switch r {
		case '\t':
			units++
			spaces = 0
		case ' ':
			spaces++
			if spaces >= 2 {
				units++
				spaces = 0
			}
		default:
			return units
		}
	}
	return units
}

// maxBulletDepth returns the maximum rendered nesting depth across a list of
// bullet strings, where baseLevel is the level a non-indented bullet renders
// at (1 for flat bullets, 2 for bullets inside a bullet_groups header).
func maxBulletDepth(bullets []string, baseLevel int) int {
	maxDepth := 0
	for _, b := range bullets {
		d := baseLevel + bulletIndentDepth(b)
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

func makeHeadlineFinding(slideIdx int, phID string, words int) patterns.FitFinding {
	path := slidepath.Content(slideIdx, phID)

	// Truncation is only a sane remedy for a mild overrun. Cutting a headline
	// in half turns a long-but-meaningful line into a fragment while making the
	// score look better — the reward-hacking shape to avoid — so a drastic
	// overrun asks for a REWRITE instead, and repair_slide refuses to truncate
	// that far even if asked (go-slide-creator-28zf).
	fix := &patterns.FixSuggestion{
		Kind: "shorten_title",
		Params: map[string]any{
			"current_words": words,
			"max_words":     maxHeadlineWords,
			// max_length is what repair_slide's own description documents;
			// supplying both means an agent replaying this directive and one
			// constructing the call from the tool description agree.
			"max_length": headlineCharBudget(maxHeadlineWords),
		},
	}
	message := fmt.Sprintf(
		"slide %d: headline is %d words; trim to %d or fewer for readability",
		slideIdx+1, words, maxHeadlineWords)

	if words > 0 && float64(words-maxHeadlineWords)/float64(words) > maxShortenedTitleWordLossFrac {
		fix = &patterns.FixSuggestion{
			Kind: "review",
			Params: map[string]any{
				"current_words": words,
				"max_words":     maxHeadlineWords,
				"reason":        "truncating to the budget would cut more than half the headline, leaving a fragment",
			},
		}
		message = fmt.Sprintf(
			"slide %d: headline is %d words, more than twice the %d-word budget — rewrite it as a shorter claim rather than truncating, and move the detail into the body or takeaway",
			slideIdx+1, words, maxHeadlineWords)
	}

	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    path,
			Code:    patterns.ErrCodeHeadlineTooLong,
			Message: message,
			Fix:     fix,
		},
		Action: "review",
	}
}

// headlineCharBudget converts a word budget into the character budget
// repair_slide's max_length expects, at a conservative average word length.
func headlineCharBudget(maxWords int) int {
	const avgWordChars = 7 // 6 letters plus a space
	return maxWords * avgWordChars
}

func makeBodyFinding(slideIdx int, phID string, words int) patterns.FitFinding {
	path := slidepath.Content(slideIdx, phID)
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeBodyTooLong,
			Message: fmt.Sprintf(
				"slide %d: body text is %d words; trim to %d or fewer (audiences read at most 5 lines per slide)",
				slideIdx+1, words, maxBodyWords),
			Fix: &patterns.FixSuggestion{
				Kind: "reduce_text",
				Params: map[string]any{
					"current_words": words,
					"max_words":     maxBodyWords,
				},
			},
		},
		Action: "review",
	}
}

func makeBulletDepthFinding(slideIdx int, phID string, depth int) patterns.FitFinding {
	path := slidepath.Content(slideIdx, phID)
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeBulletNestingDeep,
			Message: fmt.Sprintf(
				"slide %d: bullets nest %d levels; flatten to %d or fewer — deep nesting reads as visual noise",
				slideIdx+1, depth, maxBulletNestingDepth),
			Fix: &patterns.FixSuggestion{
				Kind: "reduce_text",
				Params: map[string]any{
					"current_depth": depth,
					"max_depth":     maxBulletNestingDepth,
				},
			},
		},
		Action: "review",
	}
}

// collectBackgroundFindings reports a slide whose text sits on a background
// photo with no scrim over it (go-slide-creator-uy5s). The contrast pass reads
// a solid background fill, so an image background is simply invisible to it:
// the template's dark title can land on the dark half of the picture and
// nothing says a word. Advisory — the deck renders, and only the author knows
// whether the photo is uniform under the text.
func collectBackgroundFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := input.Slides[si]
		bg := slide.Background
		if bg == nil || (bg.Image == "" && bg.URL == "") || bg.Overlay != nil {
			continue
		}
		// A slide that opts out of contrast checking has already said it knows.
		if slide.ContrastCheck != nil && !*slide.ContrastCheck {
			continue
		}
		if !slideHasVisibleText(slide) {
			continue
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: slidepath.SlideField(si, "background"),
				Code: patterns.ErrCodeTextOverImageUnverified,
				Message: fmt.Sprintf("slide %d puts text on a background image with no overlay — a photo has no single colour, so the contrast pass cannot check the text against it and the template's own title colour may land on a dark part of the picture",
					si+1),
				Fix: &patterns.FixSuggestion{
					Kind: "provide_value",
					Params: map[string]any{
						"path":  slidepath.SlideField(si, "background") + "/overlay",
						"value": map[string]any{"color": "dk1", "alpha": 0.45},
						"hint":  "add a scrim over the photo; the contrast pass then judges the text against the scrim colour",
					},
				},
			},
			Action: "review",
		})
	}
	return out
}

// slideHasVisibleText reports whether a slide puts any text on the slide, so a
// picture-only slide is not asked to dim a photo nothing sits on.
func slideHasVisibleText(slide SlideInput) bool {
	for _, c := range slide.Content {
		switch c.Type {
		case "chart", "diagram", "image":
			continue
		default:
			return true
		}
	}
	return slide.Pattern != nil || slide.ShapeGrid != nil || slide.Compose != nil || slide.Takeaway != ""
}

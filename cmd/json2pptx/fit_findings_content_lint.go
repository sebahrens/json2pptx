package main

import (
	"fmt"
	"strings"

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

// lintContentItem applies the three content lint checks to a single content
// item. The contentIdx is unused in path construction (paths target the
// placeholder ID, which is more stable across slide rearrangements) but is
// accepted so future authors can switch to indexed paths if needed.
func lintContentItem(slideIdx, _ int, content *ContentInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	isTitle := isHeadlinePlaceholderID(content.PlaceholderID)

	switch content.Type {
	case "text":
		if content.TextValue == nil {
			return nil
		}
		wc := countWords(*content.TextValue)
		if isTitle {
			if wc > maxHeadlineWords {
				findings = append(findings, makeHeadlineFinding(slideIdx, content.PlaceholderID, wc))
			}
		} else if wc > maxBodyWords {
			findings = append(findings, makeBodyFinding(slideIdx, content.PlaceholderID, wc))
		}
	case "bullets":
		if content.BulletsValue == nil {
			return nil
		}
		bullets := *content.BulletsValue
		if d := maxBulletDepth(bullets, 1); d > maxBulletNestingDepth {
			findings = append(findings, makeBulletDepthFinding(slideIdx, content.PlaceholderID, d))
		}
		if wc := countBulletWords(bullets); wc > maxBodyWords {
			findings = append(findings, makeBodyFinding(slideIdx, content.PlaceholderID, wc))
		}
	case "body_and_bullets":
		v := content.BodyAndBulletsValue
		if v == nil {
			return nil
		}
		wc := countWords(v.Body) + countBulletWords(v.Bullets) + countWords(v.TrailingBody)
		if wc > maxBodyWords {
			findings = append(findings, makeBodyFinding(slideIdx, content.PlaceholderID, wc))
		}
		if d := maxBulletDepth(v.Bullets, 1); d > maxBulletNestingDepth {
			findings = append(findings, makeBulletDepthFinding(slideIdx, content.PlaceholderID, d))
		}
	case "bullet_groups":
		v := content.BulletGroupsValue
		if v == nil {
			return nil
		}
		wc := countWords(v.Body) + countWords(v.TrailingBody)
		maxDepth := 0
		for _, g := range v.Groups {
			wc += countWords(g.GroupLabel) + countWords(g.Header) + countWords(g.Body)
			wc += countBulletWords(g.Bullets)
			// In bullet_groups the header occupies level 1 and bullets render
			// at level 2 by default. Indent inside a bullet string pushes it
			// deeper.
			if d := maxBulletDepth(g.Bullets, 2); d > maxDepth {
				maxDepth = d
			}
		}
		if wc > maxBodyWords {
			findings = append(findings, makeBodyFinding(slideIdx, content.PlaceholderID, wc))
		}
		if maxDepth > maxBulletNestingDepth {
			findings = append(findings, makeBulletDepthFinding(slideIdx, content.PlaceholderID, maxDepth))
		}
	}
	return findings
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
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeHeadlineTooLong,
			Message: fmt.Sprintf(
				"slide %d: headline is %d words; trim to %d or fewer for readability",
				slideIdx+1, words, maxHeadlineWords),
			Fix: &patterns.FixSuggestion{
				Kind: "shorten_title",
				Params: map[string]any{
					"current_words": words,
					"max_words":     maxHeadlineWords,
				},
			},
		},
		Action: "review",
	}
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

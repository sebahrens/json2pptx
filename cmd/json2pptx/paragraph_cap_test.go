package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-8hg02: a placeholder carrying more than
// maxPlaceholderParagraphs paragraphs is refused with BODY_TOO_LONG, which
// replaces the review-level word-budget finding for the same block.
func TestParagraphCapRefusesOversizedPlaceholder(t *testing.T) {
	bullets := func(n int) *[]string {
		b := make([]string, n)
		for i := range b {
			b[i] = "x"
		}
		return &b
	}
	legacy, _ := json.Marshal(*bullets(maxPlaceholderParagraphs + 1))
	cases := []struct {
		name    string
		content ContentInput
		refuse  bool
	}{
		{"at cap", ContentInput{PlaceholderID: "body", Type: "bullets", BulletsValue: bullets(maxPlaceholderParagraphs)}, false},
		{"over cap", ContentInput{PlaceholderID: "body", Type: "bullets", BulletsValue: bullets(2000)}, true},
		{"legacy value", ContentInput{PlaceholderID: "body", Type: "bullets", Value: legacy}, true},
		{"body_and_bullets", ContentInput{PlaceholderID: "body", Type: "body_and_bullets", BodyAndBulletsValue: &BodyAndBulletsInput{Body: "Intro", Bullets: *bullets(maxPlaceholderParagraphs)}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := lintContentItem(0, 0, &tc.content, measuredTitleSet{})
			var refusals, reviews int
			for _, f := range findings {
				if f.Code != patterns.ErrCodeBodyTooLong {
					continue
				}
				if f.Action == "refuse" {
					refusals++
					if f.Fix == nil || f.Fix.Params["max_paragraphs"] != maxPlaceholderParagraphs {
						t.Errorf("refusal must carry max_paragraphs: %+v", f.Fix)
					}
				} else {
					reviews++
				}
			}
			if tc.refuse && (refusals != 1 || reviews != 0) {
				t.Errorf("want exactly one refusing BODY_TOO_LONG, got refusals=%d reviews=%d", refusals, reviews)
			}
			if !tc.refuse && refusals != 0 {
				t.Errorf("at the cap must not refuse, got %d refusals", refusals)
			}
		})
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func bulletsSlide(bullets []string) SlideInput {
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "bullets",
			BulletsValue:  &bullets,
		}},
	}
}

func numberedFindings(findings []patterns.FitFinding) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range findings {
		if f.Code == patterns.ErrCodeNumberedListNotApplied {
			out = append(out, f)
		}
	}
	return out
}

// A list the renderer cannot auto-number prints the author's numbers beside the
// layout's own glyph. Nothing said so (go-slide-creator-6or2).
func TestNumberedListFindingFires(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{bulletsSlide([]string{
		"1. Freeze the schema", "Replay the log", "3. Cut over",
	})}}
	got := numberedFindings(collectContentLintFindings(in))
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d (%+v)", len(got), got)
	}
	f := got[0]
	if f.Action != "review" {
		t.Errorf("action = %q, want review", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "renumber_bullets" {
		t.Errorf("fix = %+v, want kind renumber_bullets", f.Fix)
	}
	if !strings.Contains(f.Message, "2 of 3") {
		t.Errorf("message should count the prefixed bullets, got %q", f.Message)
	}
}

// A list the renderer DOES number is correct authoring and must stay silent,
// as must a list with no prefixes at all and a lone numeric line.
func TestNumberedListFindingStaysSilent(t *testing.T) {
	quiet := [][]string{
		{"1. Freeze the schema", "2. Replay the log", "3. Cut over"},
		{"Freeze the schema", "Replay the log"},
		{"2024. A big year"},
	}
	for _, bullets := range quiet {
		in := &PresentationInput{Slides: []SlideInput{bulletsSlide(bullets)}}
		if got := numberedFindings(collectContentLintFindings(in)); len(got) != 0 {
			t.Errorf("%v drew %+v", bullets, got)
		}
	}
}

// The fix the finding names must actually run.
func TestApplyRenumberBullets(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{bulletsSlide([]string{
		"1. Freeze the schema", "Replay the log", "3. Cut over",
	})}}
	res := applyRenumberBullets(in, 0, map[string]any{"path": "/slides/0/content/body"})
	if !res.Applied {
		t.Fatalf("repair did not apply: %s", res.Message)
	}
	got := *in.Slides[0].Content[0].BulletsValue
	want := []string{"1. Freeze the schema", "2. Replay the log", "3. Cut over"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("bullets = %v, want %v", got, want)
	}
	// The repaired list is now one the renderer numbers itself, so the finding
	// is gone.
	if f := numberedFindings(collectContentLintFindings(in)); len(f) != 0 {
		t.Errorf("finding survived the repair: %+v", f)
	}

	// strip: true is the other answer — the list was never ordered.
	stripped := &PresentationInput{Slides: []SlideInput{bulletsSlide([]string{
		"1. Freeze the schema", "Replay the log", "3. Cut over",
	})}}
	res = applyRenumberBullets(stripped, 0, map[string]any{"path": "/slides/0/content/body", "strip": true})
	if !res.Applied {
		t.Fatalf("strip did not apply: %s", res.Message)
	}
	for _, bullet := range *stripped.Slides[0].Content[0].BulletsValue {
		if strings.HasPrefix(bullet, "1. ") || strings.HasPrefix(bullet, "3. ") {
			t.Errorf("prefix survived strip: %q", bullet)
		}
	}
}

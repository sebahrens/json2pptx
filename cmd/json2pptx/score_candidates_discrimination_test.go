package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// candidateSlide builds a slide JSON for one candidate.
func candidateSlide(title string, bullets []string) json.RawMessage {
	items := make([]any, len(bullets))
	for i, b := range bullets {
		items[i] = b
	}
	b, _ := json.Marshal(map[string]any{
		"layout_id":  "blank-title",
		"slide_type": "content",
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": title},
			map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": items},
		},
	})
	return b
}

// TestScoreCandidatesDiscriminates is go-slide-creator-sbqm: the tool scored a
// real slide 100, a near-empty one 95 and an unreadable one 90 — a spread an
// agent reads as "all three are fine, take the first".
func TestScoreCandidatesDiscriminates(t *testing.T) {
	base, layouts, w, h := loadCalibrationDeckWithTemplate(t, "G2_legacy_strategy_review")
	real, err := json.Marshal(base.Slides[1])
	if err != nil {
		t.Fatal(err)
	}

	lorem := strings.Repeat("Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore ", 2)
	bullets := make([]string, 9)
	for i := range bullets {
		bullets[i] = lorem
	}

	got := map[string]CandidateScore{
		"real":       scoreCandidate(0, 1, real, base, layouts, w, h, nil),
		"near-empty": scoreCandidate(1, 1, candidateSlide("Update", []string{"TBD"}), base, layouts, w, h, nil),
		"lorem":      scoreCandidate(2, 1, candidateSlide(lorem, bullets), base, layouts, w, h, nil),
	}
	for name, c := range got {
		if c.ParseError != "" {
			t.Fatalf("%s: %s", name, c.ParseError)
		}
		t.Logf("%-11s score=%3d slide=%3d axes={fit:%d content:%d rhythm:%d} blocking=%d",
			name, c.Score, c.SlideScore, c.Axes.Fit, c.Axes.Content, c.Axes.Rhythm, c.Blocking)
	}

	if got["real"].Score <= 90 {
		t.Errorf("a real slide scored %d, want > 90", got["real"].Score)
	}
	if got["near-empty"].Score >= 60 {
		t.Errorf("a near-empty candidate scored %d, want < 60", got["near-empty"].Score)
	}
	if got["lorem"].Score >= 50 {
		t.Errorf("an unreadable candidate scored %d, want < 50", got["lorem"].Score)
	}

	// The axes say WHERE each one lost: the empty slide's geometry is fine.
	if got["near-empty"].Axes.Fit <= got["near-empty"].Axes.Content {
		t.Errorf("near-empty axes = %+v; it should lose on content, not fit", got["near-empty"].Axes)
	}
	// slide_score stays comparable with score_deck — it is not the ranking number.
	if got["near-empty"].SlideScore <= got["near-empty"].Score {
		t.Errorf("slide_score %d should stay above the ranking score %d for a refused candidate",
			got["near-empty"].SlideScore, got["near-empty"].Score)
	}
}

// TestRefusalCeilingExplainsItself: a candidate ranked from the ceiling says so
// in its notes, rather than leaving the agent to wonder about the arithmetic.
func TestRefusalCeilingExplainsItself(t *testing.T) {
	base, layouts, w, h := loadCalibrationDeckWithTemplate(t, "G2_legacy_strategy_review")
	c := scoreCandidate(0, 1, candidateSlide("Update", []string{"TBD"}), base, layouts, w, h, nil)
	if c.Blocking == 0 {
		t.Fatal("a near-empty placeholder slide should carry a refuse-action finding")
	}
	found := false
	for _, n := range c.Notes {
		if strings.Contains(n, "would refuse this candidate") {
			found = true
		}
	}
	if !found {
		t.Errorf("notes %v do not explain the refusal ceiling", c.Notes)
	}
}

// TestTopTieNoteNamesWhatItCannotJudge: two candidates the tool cannot tell
// apart must not be presented as ranked.
func TestTopTieNoteNamesWhatItCannotJudge(t *testing.T) {
	tied := []CandidateScore{{Index: 0, Score: 95}, {Index: 1, Score: 95}, {Index: 2, Score: 80}}
	note := topTieNote(tied)
	if note == "" {
		t.Fatal("expected a tie note")
	}
	for _, want := range []string{"[0 1]", "input order", "render_slide_image"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q does not mention %q", note, want)
		}
	}
	if topTieNote([]CandidateScore{{Index: 0, Score: 95}, {Index: 1, Score: 80}}) != "" {
		t.Error("a clear winner must not carry a tie note")
	}
	if topTieNote(nil) != "" {
		t.Error("no candidates, no note")
	}
}

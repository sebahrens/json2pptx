package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-maq6l: the closing slide compiled onto the cover layout, so
// it was a second title slide and an action title wrapped to four lines. It
// now takes the template's own closing layout, and validation flags a closing
// title too long for that layout's display title.
func TestClosingUsesTheClosingLayout(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{
		{Kind: KindClosing, Body: map[string]any{"title": "Approve the pod this month", "subtitle": "Owner: COO · 31 July"}},
	}}
	input, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
	}
	if got := input.Slides[0].LayoutID; got != "closing" {
		t.Fatalf("closing layout_id = %q, want closing", got)
	}
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticDensity, "slides[0].title"); ok {
		t.Fatal("a short closing title was flagged")
	}
	spec.Slides[0].Body["title"] = "Approve the SMB success pod this month to protect H2 growth"
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticDensity, "slides[0].title"); !ok {
		t.Fatal("a closing title past the display budget was not flagged")
	}
	// With bullets the closer is a content slide under an ordinary title.
	spec.Slides[0].Body["bullets"] = []any{"One", "Two"}
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticDensity, "slides[0].title"); ok {
		t.Fatal("a bulleted closing is not on the closing layout and should not be flagged")
	}
}

// go-slide-creator-zj4yq: an image_case without a picture renders a dashed
// placeholder. Validation names the DeckSpec field; an image_label marks a
// deliberate placeholder.
func TestImageCaseWithoutImageWarns(t *testing.T) {
	body := map[string]any{"title": "Case", "heading": "Two clearers migrated", "body": "The cutover ran on the rehearsed plan.", "takeaway": "Rehearsal pays."}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{Kind: KindImageCase, Body: body}}}
	d, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticImageMissing, "slides[0].image")
	if !ok || !strings.Contains(d.Message, "image") {
		t.Fatalf("missing SEMANTIC_IMAGE_MISSING: %+v", d)
	}
	body["image"] = "images/cutover.jpg"
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticImageMissing, "slides[0].image"); ok {
		t.Fatal("an image was given")
	}
	delete(body, "image")
	body["image_label"] = "Cutover room photo"
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticImageMissing, "slides[0].image"); ok {
		t.Fatal("a labelled placeholder is deliberate")
	}
}

// go-slide-creator-zifd3: image.fit is cover or contain; anything else is
// refused at the DeckSpec field rather than at the compiled pattern.
func TestImageCaseImageFitValidated(t *testing.T) {
	img := map[string]any{"path": "images/queue.png", "fit": "contain"}
	body := map[string]any{"title": "Case", "body": "The cutover ran on the rehearsed plan.", "image": img}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{Kind: KindImageCase, Body: body}}}
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticFieldType, "slides[0].image.fit"); ok {
		t.Fatal("contain is a valid fit")
	}
	img["fit"] = "stretch"
	if _, ok := findAt(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticFieldType, "slides[0].image.fit"); !ok {
		t.Fatal("fit stretch should be refused at slides[0].image.fit")
	}
}

// go-slide-creator-zvu7c: a takeaway beside a decision's recommendation is
// kept in the speaker notes, after any notes the author wrote.
func TestDroppedTakeawayJoinsAuthoredNotes(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{Kind: KindExecutiveSummary, Body: map[string]any{
		"title":       "Summary",
		"points":      []any{"Revenue rose", "Churn fell", "Cycle shortened"},
		"bottom_line": "Fund the retention pod in Q3.",
		"takeaway":    "Begin hiring next month.",
		"notes":       "Pause here.",
	}}}}
	input, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
	}
	slide := input.Slides[0]
	if slide.Takeaway != "" {
		t.Errorf("second conclusion band = %q", slide.Takeaway)
	}
	if !strings.HasPrefix(slide.SpeakerNotes, "Pause here.") || !strings.Contains(slide.SpeakerNotes, "Begin hiring next month.") {
		t.Errorf("speaker notes = %q", slide.SpeakerNotes)
	}
}

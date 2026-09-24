package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-t64e. The measured title check only ran when a slide carried
// an explicit layout_id: layoutForSlideResolved returned nil for an empty one.
// So the SAME 112-character title produced the measured verdict with
// layout_id: "content" and the legacy "title too long (112 chars, max 60)"
// character heuristic with slide_type: "content" — and the semantic compilers
// emit SlideType and no LayoutID, so the DeckSpec path (the one get_started
// recommends) never measured a title at all.

// longTitle is long enough that it cannot fit a title placeholder at full size.
const longTitle = "Enterprise revenue expansion continued to outpace our internal forecast across every single region we operate in"

func predictedTestLayouts(t *testing.T, tmpl string) []types.LayoutMetadata {
	t.Helper()
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", tmpl+".pptx"))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse layouts: %v", err)
	}
	return layouts
}

func predictedTitleSlide(extra func(*SlideInput)) SlideInput {
	title := longTitle
	s := SlideInput{Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: &title},
		{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{"One", "Two", "Three"}},
	}}
	extra(&s)
	return s
}

// TestTitleFitMeasuredWithoutLayoutID is the bead's VERIFY: the same title with
// slide_type only and with layout_id must produce the same title findings.
func TestTitleFitMeasuredWithoutLayoutID(t *testing.T) {
	layouts := predictedTestLayouts(t, "modern-template")

	withID := &PresentationInput{Slides: []SlideInput{predictedTitleSlide(func(s *SlideInput) { s.LayoutID = "content" })}}
	withType := &PresentationInput{Slides: []SlideInput{predictedTitleSlide(func(s *SlideInput) { s.SlideType = "content" })}}

	codesOf := func(input *PresentationInput) []string {
		var out []string
		titleFindings, _ := collectTitleFitFindings(input, layouts)
		for _, f := range titleFindings {
			out = append(out, f.Code)
		}
		return out
	}
	byID, byType := codesOf(withID), codesOf(withType)
	if len(byType) == 0 {
		t.Fatal("a slide with slide_type and no layout_id produced no measured title finding")
	}
	if strings.Join(byID, ",") != strings.Join(byType, ",") {
		t.Errorf("layout_id gives %v but slide_type gives %v — the two paths disagree", byID, byType)
	}
}

func TestCollectTitleFitFindingsIgnoresComfortableTwoLineTitle(t *testing.T) {
	const width = int64(7772400)
	layouts := []types.LayoutMetadata{{ID: "slideLayout1", Name: "Title Slide", Placeholders: []types.PlaceholderInfo{{
		ID: "title", Type: types.PlaceholderTitle, FontSize: 3600, FontFamily: "Arial",
		Bounds: types.BoundingBox{Width: width, Height: 2 * 914400},
	}}}}
	for n := 1; n <= 30; n++ {
		title := strings.Repeat("Revenue growth and margin expansion ", n)
		measurement, err := textfit.MeasureRun(title, "Arial", 36, width, 0)
		if err != nil {
			t.Fatal(err)
		}
		if measurement.Lines != 2 {
			continue
		}
		slide := SlideInput{LayoutID: "slideLayout1", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
		findings, _ := collectTitleFitFindings(&PresentationInput{Slides: []SlideInput{slide}}, layouts)
		if len(findings) != 0 {
			t.Errorf("comfortable two-line title produced %+v", findings)
		}
		return
	}
	t.Fatal("test fixture could not produce a two-line title")
}

// TestQualityScoreUsesTheMeasuredTitleWithoutLayoutID pins the score half: the
// slide_type path scored 70 with the character heuristic while layout_id scored
// 85 with the measurement.
func TestQualityScoreUsesTheMeasuredTitleWithoutLayoutID(t *testing.T) {
	layouts := predictedTestLayouts(t, "modern-template")

	withID := computeQualityScoreWithLayouts([]SlideInput{predictedTitleSlide(func(s *SlideInput) { s.LayoutID = "content" })}, nil, layouts)
	withType := computeQualityScoreWithLayouts([]SlideInput{predictedTitleSlide(func(s *SlideInput) { s.SlideType = "content" })}, nil, layouts)

	if withID.Score != withType.Score {
		t.Errorf("layout_id scores %.0f but slide_type scores %.0f — the same deck, two numbers", withID.Score, withType.Score)
	}
	issues := strings.Join(withType.SlideScores[0].Issues, " ")
	if strings.Contains(issues, "max 60") {
		t.Errorf("slide_type path still uses the 60-character heuristic: %q", issues)
	}
	if !strings.Contains(issues, "only fits its title placeholder") {
		t.Errorf("slide_type path did not report the measured verdict: %q", issues)
	}
}

func TestPredictSlideLayouts(t *testing.T) {
	layouts := predictedTestLayouts(t, "modern-template")

	input := &PresentationInput{Slides: []SlideInput{
		predictedTitleSlide(func(s *SlideInput) { s.LayoutID = "content" }),
		predictedTitleSlide(func(s *SlideInput) { s.SlideType = "content" }),
		{SlideType: "title"},
	}}
	got := predictSlideLayouts(input, layouts)
	if len(got) != 3 {
		t.Fatalf("got %d predictions, want 3", len(got))
	}
	for i, l := range got {
		if l == nil {
			t.Errorf("slide %d has no predicted layout", i)
		}
	}

	// No layouts at all: no predictions, and no panic.
	empty := predictSlideLayouts(input, nil)
	if len(empty) != 3 {
		t.Fatalf("got %d entries with no layouts, want 3", len(empty))
	}
	for i, l := range empty {
		if l != nil {
			t.Errorf("slide %d predicted %v with no layouts available", i, l.ID)
		}
	}
	if predictSlideLayouts(nil, layouts) != nil {
		t.Error("nil input should predict nothing")
	}
}

// TestValidateDeckSpecReportsMeasuredTitle covers the path the bead cares about
// most: DeckSpec compilers emit slide_type and no layout_id. Both measured
// verdicts are pinned, because they are what distinguishes a measurement from
// the old 60-character heuristic: a title that fits only when shrunk is
// TITLE_WRAPS, and one that does not fit even at the minimum autofit size is
// TITLE_OVERFLOW. The two are mutually exclusive — the stronger one replaces
// the weaker (go-slide-creator-r87g) — so a test asserting both codes on one
// title asserts a report that contradicts itself.
func TestValidateDeckSpecReportsMeasuredTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("analyzes a template")
	}
	cases := []struct {
		name  string
		title string
		code  string
		says  string
	}{
		{
			name:  "shrinks to fit",
			title: longTitle,
			code:  "title_wraps",
			says:  "only fits its title placeholder",
		},
		{
			name:  "does not fit at all",
			title: longTitle + " during the current fiscal year and the next",
			code:  "TITLE_OVERFLOW",
			says:  "does not fit its",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := map[string]any{
				"meta": map[string]any{"title": "T", "template": "modern-template"},
				"slides": []any{
					map[string]any{"kind": "title", "title": "T"},
					map[string]any{
						"kind":   "executive_summary",
						"title":  tc.title,
						"points": []any{map[string]any{"lead": "Grow", "support": "Revenue is up."}},
					},
				},
			}
			res, err := testValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
			if err != nil {
				t.Fatalf("validate_deck_spec: %v", err)
			}
			b, _ := json.Marshal(res.StructuredContent)
			if !strings.Contains(string(b), tc.code) {
				t.Errorf("validate_deck_spec did not report %s:\n%s", tc.code, string(b))
			}
			if !strings.Contains(string(b), tc.says) {
				t.Errorf("the finding is not the measured one (no %q):\n%s", tc.says, string(b))
			}
			if strings.Contains(string(b), "max 60") {
				t.Errorf("the DeckSpec path still uses the 60-character heuristic:\n%s", string(b))
			}
		})
	}
}

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

const (
	vjwnLongTitle  = "Procurement is fragmented across 14 business units, leaving an estimated $38-52M of annual savings uncaptured"
	vjwnShortTitle = "Procurement savings of $38-52M are at risk" // ~40 chars
)

func loadTemplateAnalysis(t *testing.T, name string) *types.TemplateAnalysis {
	t.Helper()
	path := filepath.Join("..", "..", "templates", name+".pptx")
	reader, err := template.OpenTemplate(path)
	if err != nil {
		t.Skipf("template %s: %v", name, err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts: %v", err)
	}
	w, h := template.ParseSlideDimensions(reader)
	return &types.TemplateAnalysis{TemplatePath: path, SlideWidth: w, SlideHeight: h, Layouts: layouts, Theme: template.ParseTheme(reader)}
}

func titleSlide(title string) SlideInput {
	return SlideInput{
		LayoutID: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"` + title + `"`)},
			{PlaceholderID: "body", Type: "bullets", Value: json.RawMessage(`["one","two"]`)},
		},
	}
}

func TestTitlePlaceholderInheritsModernTemplateStyle(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	s := titleSlide(vjwnLongTitle)
	ph := titlePlaceholderFor(&s, "title", a.Layouts)
	if ph == nil {
		t.Fatal("canonical 'content' layout title placeholder not resolved")
	}
	if ph.FontSize != 4500 || !ph.TextCaps || ph.LineSpacingPct != 80 {
		t.Errorf("title placeholder style = size %d caps %v lnSpc %d, want 4500/true/80", ph.FontSize, ph.TextCaps, ph.LineSpacingPct)
	}
}

// go-slide-creator-vjwn: validate, quality score and fit report share one
// measured title verdict. A ~40-char title passes all three; the 104-char
// consulting title on modern-template is flagged by all three.
func TestTitleChecksUnifiedOnMeasuredFit(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")

	type verdicts struct{ validate, quality, fit bool }
	check := func(title string) verdicts {
		var v verdicts
		slides := []SlideInput{titleSlide(title)}
		resolveCanonicalLayoutIDs(slides, a.Layouts)

		out := dryRunOutput{Valid: true}
		validateSlidesAgainstTemplate(&out, slides, a)
		for _, d := range out.Diagnostics {
			if strings.HasPrefix(d.Path, "/slides/0/content/0") && (d.Code == patterns.ErrCodeMaxLength || d.Code == patterns.ErrCodeTitleOverflow) {
				v.validate = true
			}
		}

		q := computeQualityScoreWithLayouts([]SlideInput{titleSlide(title)}, nil, a.Layouts)
		for _, issue := range q.SlideScores[0].Issues {
			if strings.Contains(issue, "title") {
				v.quality = true
			}
		}

		in := &PresentationInput{Slides: []SlideInput{titleSlide(title)}}
		for _, f := range collectTitleFitFindings(in, a.Layouts) {
			if f.Code == patterns.ErrCodeTitleOverflow || (f.Code == patterns.ErrCodeTitleWraps && f.Action == "shrink_or_split") {
				v.fit = true
			}
		}
		return v
	}

	if v := check(vjwnShortTitle); v.validate || v.quality || v.fit {
		t.Errorf("40-char title should pass all checks, got %+v", v)
	}
	if v := check(vjwnLongTitle); !v.validate || !v.quality || !v.fit {
		t.Errorf("104-char title on modern-template should be flagged by all three, got %+v", v)
	}
}

func TestMeasureTitleFlaggedCarriesMaxChars(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	s := titleSlide(vjwnLongTitle)
	m := measureTitleInPlaceholder(vjwnLongTitle, titlePlaceholderFor(&s, "title", a.Layouts))
	if !m.Flagged() {
		t.Fatalf("expected flagged measurement, got %+v", m)
	}
	if m.MaxChars <= 0 || m.MaxChars >= m.Chars {
		t.Errorf("MaxChars = %d, want 0 < n < %d", m.MaxChars, m.Chars)
	}
	f := m.fitFinding("/slides/0/content/title")
	if f == nil || f.Fix == nil || f.Fix.Kind != "shorten_title" {
		t.Fatalf("fit finding = %+v", f)
	}
}

func TestMeasureTitleUnmeasurableFallsBack(t *testing.T) {
	if m := measureTitleInPlaceholder("x", nil); m.OK {
		t.Error("nil placeholder should not be measurable")
	}
	q := computeQualityScoreWithLayouts([]SlideInput{{LayoutID: "l", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"` + strings.Repeat("word ", 20) + `"`)},
	}}}, nil, nil)
	found := false
	for _, issue := range q.SlideScores[0].Issues {
		if strings.Contains(issue, "title too long") {
			found = true
		}
	}
	if !found {
		t.Error("without layouts the 60-char heuristic should still apply")
	}
}

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/generator"
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

func TestModernCoverTitleDoesNotClaimTruncationWhenMeasuredFitSucceeds(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	title := "Entering DACH: A partner-led go-to-market for Northwind Analytics"
	slide := SlideInput{LayoutID: "title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr(title)}}}
	ph := titlePlaceholderFor(&slide, "title", a.Layouts)
	if ph == nil {
		t.Fatal("modern cover title placeholder was not resolved")
	}
	if ph.FontFamily != "Lora" {
		t.Fatalf("modern cover title font = %q, want resolved Lora", ph.FontFamily)
	}
	measurement := measureTitleInPlaceholder(title, ph)
	if !measurement.OK || measurement.Flagged() {
		t.Fatalf("rendered cover title must have a comfortable measured fit: %+v", measurement)
	}
	findings, _ := collectTitleFitFindings(&PresentationInput{Slides: []SlideInput{slide}}, a.Layouts)
	for _, finding := range findings {
		if isTitleLengthCode(finding.Code) && (finding.Action == "shrink_or_split" || finding.Code == patterns.ErrCodeTitleOverflow || strings.Contains(finding.Message, "will be truncated")) {
			t.Errorf("rendered three-line cover title must not block the gate or claim truncation: %+v", finding)
		}
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
			if isTitleLengthCode(d.Code) && strings.Contains(d.Path, "/slides/0/content/") {
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
		titleFindings, _ := collectTitleFitFindings(in, a.Layouts)
		for _, f := range titleFindings {
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

func TestAuthoredTitleFontSizeChangesMeasuredVerdictAndCapacity(t *testing.T) {
	analysis := loadTemplateAnalysis(t, "modern-template")
	title := vjwnShortTitle
	baseline := titleSlide(title)
	large := titleSlide(title)
	largePt := 96.0
	large.Content[0].FontSize = &largePt
	small := titleSlide(title)
	smallPt := 18.0
	small.Content[0].FontSize = &smallPt

	baselineFindings, _ := collectTitleFitFindings(&PresentationInput{Slides: []SlideInput{baseline}}, analysis.Layouts)
	largeFindings, _ := collectTitleFitFindings(&PresentationInput{Slides: []SlideInput{large}}, analysis.Layouts)
	smallFindings, _ := collectTitleFitFindings(&PresentationInput{Slides: []SlideInput{small}}, analysis.Layouts)
	if len(baselineFindings) != 0 || len(smallFindings) != 0 || len(largeFindings) == 0 {
		t.Fatalf("title verdict must change with author size: baseline=%+v small=%+v large=%+v", baselineFindings, smallFindings, largeFindings)
	}
	if largeFindings[0].Path != "/slides/0/content/0" || !isTitleLengthCode(largeFindings[0].Code) {
		t.Errorf("large title finding = %+v", largeFindings[0])
	}

	capacity := func(slide SlideInput) (int, bool) {
		out := dryRunOutput{Valid: true}
		slides := []SlideInput{slide}
		resolveCanonicalLayoutIDs(slides, analysis.Layouts)
		validateSlidesAgainstTemplate(&out, slides, analysis)
		if len(out.Slides) != 1 {
			t.Fatalf("missing dry-run slide: %+v", out)
		}
		var found bool
		for _, d := range out.Diagnostics {
			if isTitleLengthCode(d.Code) && d.Path == "/slides/0/content/0" {
				found = true
			}
		}
		for _, ph := range out.Slides[0].Placeholders {
			if ph.PlaceholderID == "title" {
				return ph.MaxChars, found
			}
		}
		t.Fatal("dry run omitted title placeholder")
		return 0, false
	}
	baseCapacity, baseFlag := capacity(baseline)
	largeCapacity, largeFlag := capacity(large)
	smallCapacity, smallFlag := capacity(small)
	if baseFlag || smallFlag || !largeFlag {
		t.Errorf("dry-run title diagnostics: baseline=%v large=%v small=%v", baseFlag, largeFlag, smallFlag)
	}
	// The comfort-size policy caps large display titles at 32pt, so a 96pt
	// override can share the baseline capacity even though it changes the
	// measured fit verdict. A smaller explicit size must increase capacity.
	if !(largeCapacity <= baseCapacity && baseCapacity < smallCapacity) {
		t.Errorf("dry-run title capacities: large=%d baseline=%d small=%d, want large<=base<small", largeCapacity, baseCapacity, smallCapacity)
	}
	// The same authored size must be honored when the layout is inferred.
	predictedLarge := large
	predictedLarge.LayoutID = ""
	predictedLarge.SlideType = "content"
	predictedSmall := small
	predictedSmall.LayoutID = ""
	predictedSmall.SlideType = "content"
	predictedLargeCapacity, predictedLargeFlag := capacity(predictedLarge)
	predictedSmallCapacity, predictedSmallFlag := capacity(predictedSmall)
	if !predictedLargeFlag || predictedSmallFlag || predictedLargeCapacity >= predictedSmallCapacity {
		t.Errorf("inferred-layout title sizing: large=(%d,%v) small=(%d,%v)", predictedLargeCapacity, predictedLargeFlag, predictedSmallCapacity, predictedSmallFlag)
	}

	quality := computeQualityScoreWithLayouts([]SlideInput{large}, nil, analysis.Layouts)
	var qualityTitleIssue bool
	for _, issue := range quality.SlideScores[0].Issues {
		if strings.Contains(issue, "title") {
			qualityTitleIssue = true
		}
	}
	if !qualityTitleIssue {
		t.Errorf("quality score ignored authored 96pt title: %+v", quality.SlideScores[0])
	}
}

func TestMeasuredTitleFindingsUseAuthoredContentIndex(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	slide := titleSlide(vjwnLongTitle)
	slide.Content = append([]ContentInput{{PlaceholderID: "body", Type: "text", TextValue: strPtr("intro")}}, slide.Content...)
	input := &PresentationInput{Slides: []SlideInput{slide}}
	findings, _ := collectTitleFitFindings(input, a.Layouts)
	if len(findings) == 0 {
		t.Fatal("expected a measured long-title finding")
	}
	for _, finding := range findings {
		if finding.Path != "/slides/0/content/1" {
			t.Errorf("title finding path = %q, want authored title index 1", finding.Path)
		}
		assertFindingPointerReplaceable(t, input, finding.Path)
	}
	var output dryRunOutput
	validateSlidesAgainstTemplate(&output, input.Slides, a)
	for _, diagnostic := range output.Diagnostics {
		if isTitleLengthCode(diagnostic.Code) && diagnostic.Path != "/slides/0/content/1" {
			t.Errorf("validate title path = %q, want authored title index 1", diagnostic.Path)
		}
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

func TestSectionTitlePreflightUsesReadableFloor(t *testing.T) {
	a := loadTemplateAnalysis(t, "business-template")
	short := "Revenue grew 17% in Q3 while margins expanded globally"
	long := short + " and operating costs stayed flat across every region"
	for _, tc := range []struct {
		title      string
		wantRefuse bool
	}{
		{short, false}, {long, true},
	} {
		t.Run(tc.title, func(t *testing.T) {
			input := PresentationInput{Slides: []SlideInput{{
				LayoutID: "section", SlideType: "section",
				Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr(tc.title)}},
			}}}
			resolveCanonicalLayoutIDs(input.Slides, a.Layouts)
			findings, _ := collectTitleFitFindings(&input, a.Layouts)
			var floor *patterns.FitFinding
			for i := range findings {
				if findings[i].Code == patterns.ErrCodeTitleTruncated {
					floor = &findings[i]
				}
			}
			if (floor != nil) != tc.wantRefuse {
				t.Fatalf("TITLE_TRUNCATED = %+v, wantRefuse=%v; all findings=%+v", floor, tc.wantRefuse, findings)
			}
			if floor != nil && (floor.Action != "refuse" || floor.Fix == nil || floor.Fix.Params["max_chars"] == nil) {
				t.Errorf("floor finding lacks blocking action or max_chars: %+v", floor)
			}
		})
	}
}

// isTitleLengthCode reports whether a code is one of the two the measured title
// check owns. Nothing else may report a title's length (go-slide-creator-jcph).
func isTitleLengthCode(code string) bool {
	return code == patterns.ErrCodeTitleWraps || code == patterns.ErrCodeTitleOverflow || code == patterns.ErrCodeTitleTruncated
}

// TestUnmeasurableTitleFallsBackToWordCount pins what happens when there is no
// geometry to measure against: the score says nothing (the 60-character rule it
// used to invent a number with is gone) and the word-count lint is the only
// verdict left (go-slide-creator-jcph).
func TestUnmeasurableTitleFallsBackToWordCount(t *testing.T) {
	if m := measureTitleInPlaceholder("x", nil); m.OK {
		t.Error("nil placeholder should not be measurable")
	}
	slides := []SlideInput{{LayoutID: "l", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"` + strings.Repeat("word ", 20) + `"`)},
	}}}
	q := computeQualityScoreWithLayouts(slides, nil, nil)
	for _, issue := range q.SlideScores[0].Issues {
		if strings.Contains(issue, "title too long") || strings.Contains(issue, "max 60") {
			t.Errorf("the 60-character fallback is gone; got issue %q", issue)
		}
	}

	// The word-count lint still covers it: unmeasurable is exactly the case
	// HEADLINE_TOO_LONG is for.
	in := &PresentationInput{Slides: []SlideInput{{LayoutID: "l", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: strPtr(strings.Repeat("word ", 20))},
	}}}}
	found := false
	for _, f := range collectContentLintFindings(in, nil) {
		if f.Code == patterns.ErrCodeHeadlineTooLong {
			found = true
		}
	}
	if !found {
		t.Error("an unmeasured 20-word headline should still raise HEADLINE_TOO_LONG")
	}
}

// TestMeasuredTitleReportedByExactlyOneCode is go-slide-creator-jcph's VERIFY:
// a title that does not fit is reported once, by one code, and the word-count
// lint stands down rather than adding a second differently-worded verdict.
func TestMeasuredTitleReportedByExactlyOneCode(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	slides := []SlideInput{titleSlide(vjwnLongTitle)}
	resolveCanonicalLayoutIDs(slides, a.Layouts)
	in := &PresentationInput{Template: "modern-template", Slides: slides}

	var titleCodes []string
	for _, f := range collectFitFindings(in, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		switch {
		case isTitleLengthCode(f.Code):
			titleCodes = append(titleCodes, f.Code)
		case f.Code == patterns.ErrCodeHeadlineTooLong:
			t.Errorf("HEADLINE_TOO_LONG fired on a measured title: %s", f.Message)
		}
	}
	if len(titleCodes) != 1 {
		t.Errorf("title reported %d times (%v), want exactly once", len(titleCodes), titleCodes)
	}

	// And the same holds through the findings envelope, where the validate-time
	// diagnostic and the fit finding meet: they are the same record, so the
	// envelope carries one.
	out := dryRunOutput{Valid: true}
	validateSlidesAgainstTemplate(&out, slides, a)
	out.FitFindings = collectFitFindings(in, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme)
	out.buildFindingsEnvelope()
	n := 0
	for _, f := range out.Findings.Findings {
		if strings.Contains(f.Code, patterns.ErrCodeTitleWraps) || strings.Contains(f.Code, patterns.ErrCodeTitleOverflow) {
			n++
		}
		if strings.Contains(f.Message, "max 60") || strings.Contains(f.Message, "headline is") {
			t.Errorf("stale title rule in envelope: %s", f.Message)
		}
	}
	if n != 1 {
		t.Errorf("envelope carries the title %d times, want once", n)
	}
}

// TestTitlePlaceholderMaxCharsIsMeasured pins the third rule's replacement: the
// max_chars validate_input reports for a title placeholder is the measured
// capacity at the resolved font, not estimateMaxChars' geometric guess (29 chars
// for a box that renders 96) — go-slide-creator-jcph.
func TestTitlePlaceholderMaxCharsIsMeasured(t *testing.T) {
	a := loadTemplateAnalysis(t, "modern-template")
	slides := []SlideInput{titleSlide(vjwnShortTitle)}
	resolveCanonicalLayoutIDs(slides, a.Layouts)

	ph := titlePlaceholderFor(&slides[0], "title", a.Layouts)
	if ph == nil {
		t.Fatal("title placeholder not resolved")
	}
	want := generator.TitlePlaceholderCapacityChars(ph)
	if want <= 0 {
		t.Fatal("measured capacity should be positive")
	}
	// The capacity must not depend on what the slide currently says — the whole
	// point is that it describes the box, not the text in it.
	if other := generator.TitlePlaceholderCapacityChars(ph); other != want {
		t.Errorf("capacity is not text-independent: %d vs %d", other, want)
	}
	if want <= len(vjwnShortTitle) {
		t.Errorf("capacity %d should exceed the comfortably-fitting %d-char title", want, len(vjwnShortTitle))
	}

	out := dryRunOutput{Valid: true}
	validateSlidesAgainstTemplate(&out, slides, a)
	for _, p := range out.Slides[0].Placeholders {
		if p.PlaceholderID != "title" {
			continue
		}
		if p.MaxChars != want {
			t.Errorf("title max_chars = %d, want the measured %d (geometric estimate was %d)", p.MaxChars, want, ph.MaxChars)
		}
		return
	}
	t.Fatal("no title placeholder in the dry-run output")
}

func TestTitleCapacityConsistentAcrossTemplateSurfaces(t *testing.T) {
	a := loadTemplateAnalysis(t, "abstract")
	validated := buildLayoutsOutput(a.Layouts, true)
	full := buildFullLayoutInfos(a.Layouts, nil)
	var sawSectionDivider bool
	for i := range a.Layouts {
		l := &a.Layouts[i]
		projected := examine.ProjectLayoutPlaceholders(l)
		for j := range l.Placeholders {
			ph := &l.Placeholders[j]
			if ph.Type != types.PlaceholderTitle || ph.FontSize <= 0 {
				continue
			}
			want := generator.ReportedMaxChars(ph)
			perLine := generator.ReportedMaxCharsPerLine(ph)
			if want <= 0 {
				t.Fatalf("layout %q title %q has no measured capacity", l.Name, ph.ID)
			}
			if perLine <= 0 || want > 3*perLine {
				t.Errorf("layout %q title %q capacity %d must fit the conservative 3-line ceiling of %d", l.Name, ph.ID, want, perLine)
			}
			if got := validated[i].Placeholders[j].MaxChars; got != want {
				t.Errorf("validate-template %q/%q max_chars=%d, want %d", l.Name, ph.ID, got, want)
			}
			if got := validated[i].Placeholders[j].MaxCharsPerLine; got != perLine {
				t.Errorf("validate-template %q/%q max_chars_per_line=%d, want %d", l.Name, ph.ID, got, perLine)
			}
			if got := projected[j].MaxChars; got != want {
				t.Errorf("examine-template %q/%q max_chars=%d, want %d", l.Name, ph.ID, got, want)
			}
			if got := projected[j].MaxCharsPerLine; got != perLine {
				t.Errorf("examine-template %q/%q max_chars_per_line=%d, want %d", l.Name, ph.ID, got, perLine)
			}
			if got := full[i].Placeholders[j].MaxChars; got != want {
				t.Errorf("list_templates %q/%q max_chars=%d, want %d", l.Name, ph.ID, got, want)
			}
			if got := full[i].Placeholders[j].MaxCharsPerLine; got != perLine {
				t.Errorf("list_templates %q/%q max_chars_per_line=%d, want %d", l.Name, ph.ID, got, perLine)
			}
			if l.Name == "Section Divider" {
				sawSectionDivider = true
				t.Logf("abstract Section Divider title budget: total=%d per_line=%d (old area estimate=%d)", want, perLine, ph.MaxChars)
				if want >= 126 || perLine >= 26 {
					t.Errorf("abstract Section Divider repeats the misleading 126-char/26-char-line guidance: max_chars=%d per_line=%d", want, perLine)
				}
			}
		}
	}
	if !sawSectionDivider {
		t.Fatal("abstract Section Divider title was not checked")
	}
}

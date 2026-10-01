package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-fabz4: the sovereign deck's titles render on one or two
// lines at the autofit size generation bakes, and its risk table renders
// cleanly. Neither may be reported as wrapping / overflowing on midnight-blue.
func TestSovereignDeckHasNoFalseTitleOrTableFindings(t *testing.T) {
	input, _, w, h := loadExampleForFit(t, "sovereign-ai-strategy")
	a := loadTemplateAnalysis(t, "midnight-blue")
	input.Template = "midnight-blue"
	for _, f := range collectFitFindings(input, a.Layouts, w, h, nil) {
		if f.Code == patterns.ErrCodeTitleWraps {
			t.Errorf("title_wraps on a title that renders within its box: %s %s", f.Path, f.Message)
		}
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "/table/") {
			t.Errorf("table fit_overflow on a table that renders cleanly: %s %s", f.Path, f.Message)
		}
	}
}

// An exactly-one-line title (48pt → 731520 EMU) must not read as wrapping
// because 48 × 1.2 × 12700 truncated to 731519.
func TestDetectTitleWrapsRoundsOneLineThreshold(t *testing.T) {
	f := generator.DetectTitleWraps(generator.TitleWrapsInput{
		Path: "/slides/0/content/0", Title: "Product Launch Strategy",
		WidthEMU: 8001000, HeightEMU: 731519, FontSizeHPt: 4800, FontName: "Arial",
	})
	if f != nil {
		t.Fatalf("one-line title reported as wrapping: %+v", f)
	}
}

func TestTitleNotAction(t *testing.T) {
	title := func(s string) SlideInput {
		v := s
		body := []string{"Revenue grew 12% on enterprise renewals"}
		return SlideInput{SlideType: "content", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &v},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &body},
		}}
	}
	withTakeaway := title("Regional demand")
	withTakeaway.Takeaway = "Mid-market demand doubled."
	input := &PresentationInput{Slides: []SlideInput{
		title("Market Overview"),          // stock label → flagged
		title("Revenue grew 12%"),         // number
		title("Costs fall"),               // verb (-s form)
		title("Agenda"),                   // navigation
		title("Churn stabilised quickly"), // -ed
		withTakeaway,                      // point stated in the takeaway
		title("Closing remarks"),          // the deck's last slide may keep a short label
	}}
	got := collectTitleNotActionFindings(input, nil)
	if len(got) != 1 || got[0].Path != "/slides/0/content/0" || got[0].Action != "review" {
		t.Fatalf("TITLE_NOT_ACTION = %+v, want one review finding on slide 0", got)
	}
}

func TestTitleTooLongFinding(t *testing.T) {
	long := strings.Repeat("Mid-market demand doubled while enterprise stalled ", 4)
	if f := titleTooLongFinding("/slides/1/content/0", long, "Arial", false, 3600, 9144000); f == nil || f.Code != patterns.ErrCodeTitleTooLong {
		t.Fatalf("expected TITLE_TOO_LONG for a 4-line title, got %+v", f)
	} else if mc, _ := f.Fix.Params["max_chars"].(int); mc <= 0 || mc >= len(long) {
		t.Errorf("max_chars = %v, want a shorter two-line budget", f.Fix.Params["max_chars"])
	}
	if f := titleTooLongFinding("/slides/1/content/0", "Mid-market demand doubled", "Arial", false, 3600, 9144000); f != nil {
		t.Errorf("one-line title reported: %+v", f)
	}
}

func TestSparsePlaceholderFinding(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	title := "Demand is growing faster than supply"
	sparse := []string{"Demand is growing", "Pricing is stable", "Two new entrants"}
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: "content", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: &title},
		{PlaceholderID: "body", Type: "bullets", BulletsValue: &sparse},
	}}}}
	resolveCanonicalLayoutIDs(input.Slides, a.Layouts)
	got := collectSparsePlaceholderFindings(input, a.Layouts, 0)
	if len(got) != 1 || got[0].Code != patterns.ErrCodeSparsePlaceholder {
		t.Fatalf("expected SPARSE_PLACEHOLDER, got %+v", got)
	}
}

func TestVerticalImbalance(t *testing.T) {
	safe := pptx.RectEmu{X: 0, Y: 0, CX: 9144000, CY: 4572000}
	slide := &SlideInput{}
	top := []pptx.RectEmu{{X: 0, Y: 0, CX: 9144000, CY: 1828800}}
	if f := checkVerticalImbalance(top, safe, slide, 0, "", 0); f == nil || f.Code != patterns.ErrCodeVerticalImbalance {
		t.Fatalf("top-pinned block not reported: %+v", f)
	}
	centred := []pptx.RectEmu{{X: 0, Y: 1371600, CX: 9144000, CY: 1828800}}
	if f := checkVerticalImbalance(centred, safe, slide, 0, "", 0); f != nil {
		t.Errorf("centred block reported: %+v", f)
	}
	// go-slide-creator-e17xy: a block hung from the template's body line
	// leaves its band below as the normal bottom margin.
	if f := checkVerticalImbalance(top, safe, slide, 0, "", safe.Y+12700); f != nil {
		t.Errorf("block hung from the body line reported: %+v", f)
	}
	slide.Takeaway = "The takeaway renders below the grid."
	if f := checkVerticalImbalance(top, safe, slide, 0, "", 0); f != nil {
		t.Errorf("band below a grid with a takeaway reported: %+v", f)
	}
}

// Six consecutive chart slides of differing chart types are an advisory, not
// a refusal; six of the same type still refuse.
func TestMonotonyChartRunOfDifferentTypesIsReview(t *testing.T) {
	chartSlide := func(kind string) SlideInput {
		title := "Revenue grew 12% in " + kind
		return SlideInput{SlideType: "content", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "chart", ChartValue: &types.ChartSpec{Type: types.ChartType(kind), Data: map[string]any{"a": 1.0, "b": 2.0}}},
		}}
	}
	varied := &PresentationInput{}
	same := &PresentationInput{}
	for _, k := range []string{"bar", "line", "pie", "donut", "area", "funnel"} {
		varied.Slides = append(varied.Slides, chartSlide(k))
		same.Slides = append(same.Slides, chartSlide("bar"))
	}
	for _, tc := range []struct {
		in   *PresentationInput
		want string
	}{{varied, "review"}, {same, "refuse"}} {
		got := collectMonotonyFindings(tc.in)
		if len(got) != 1 || got[0].Action != tc.want {
			t.Errorf("monotony = %+v, want one %s finding", got, tc.want)
		}
	}
}

// khzni: appendix back matter is exempt from DECK_MONOTONY — after a divider
// titled Appendix (flat deck) or in a structure section marked appendix.
func TestMonotonySkipsAppendixBackMatter(t *testing.T) {
	chartSlide := func() SlideInput {
		title := "Revenue grew 12%"
		return SlideInput{SlideType: "content", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "chart", ChartValue: &types.ChartSpec{Type: types.ChartType("bar"), Data: map[string]any{"a": 1.0, "b": 2.0}}},
		}}
	}
	divider := func(title string) SlideInput {
		return SlideInput{SlideType: "section", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
	}
	flat := &PresentationInput{Slides: []SlideInput{divider("Appendix")}}
	chapter := &PresentationInput{Slides: []SlideInput{divider("Market")}}
	crumb := &PresentationInput{}
	for i := 0; i < 6; i++ {
		flat.Slides = append(flat.Slides, chartSlide())
		chapter.Slides = append(chapter.Slides, chartSlide())
		s := chartSlide()
		s.SectionTitle = "Appendix: Supporting data"
		crumb.Slides = append(crumb.Slides, s)
	}
	if got := collectMonotonyFindings(flat); len(got) != 0 {
		t.Errorf("backup charts after an Appendix divider flagged: %+v", got)
	}
	if got := collectMonotonyFindings(crumb); len(got) != 0 {
		t.Errorf("appendix-section charts flagged: %+v", got)
	}
	if got := collectMonotonyFindings(chapter); len(got) != 1 {
		t.Errorf("an ordinary chapter run must still be flagged, got %+v", got)
	}
}

func TestSupersedeRealizedContrastPredictions(t *testing.T) {
	in := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: "/slides/2/shape_grid/rows/0/cells/1/shape/text", Code: patterns.ErrCodeContrastPredicted,
			Message: "predicted: low-contrast text will be auto-replaced — #FFFFFF → #000000 (on #D4463A, ratio 4.4 → 4.7)"}, Action: "info"},
		{ValidationError: patterns.ValidationError{Path: "/slides/3/shape_grid/rows/0/cells/1/shape/text", Code: patterns.ErrCodeContrastPredicted,
			Message: "predicted: low-contrast text will be auto-replaced — #FFFFFF → #111111 (on #D4463A, ratio 4.4 → 4.7)"}, Action: "info"},
		{ValidationError: patterns.ValidationError{Path: "/slides/2/shape_grid/shapes/1", Code: "contrast_autofixed",
			Message: "auto-fixed low-contrast text on shape_grid: #FFFFFF → #000000 (on #D4463A, ratio 4.4 → 4.7)"}, Action: "info"},
		{ValidationError: patterns.ValidationError{Path: "/slides/2/shape_grid/shapes/4", Code: "contrast_autofixed",
			Message: "auto-fixed low-contrast text on shape_grid: #FFFFFF → #1B2A4A (on #E8A838, ratio 2.1 → 6.8)"}, Action: "info"},
	}
	out := supersedeRealizedContrastPredictions(in)
	codes := map[string]int{}
	for _, f := range out {
		codes[f.Code]++
	}
	if codes[patterns.ErrCodeContrastPredicted] != 1 || codes["contrast_autofixed"] != 1 {
		t.Fatalf("want the unconfirmed prediction and one folded record, got %v: %+v", codes, out)
	}
	for _, f := range out {
		if f.Code == "contrast_autofixed" && !strings.Contains(f.Message, "2 surfaces") {
			t.Errorf("folded record should count its surfaces: %q", f.Message)
		}
	}
}

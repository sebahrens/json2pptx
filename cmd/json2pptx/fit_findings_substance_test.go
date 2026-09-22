package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func sectionTextContent(id, value string) ContentInput {
	return ContentInput{PlaceholderID: id, Type: "text", TextValue: &value}
}

func TestSectionNumberSequenceFindings(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID:            "section-layout",
		CanonicalType: types.CanonicalLayoutSectionDivider,
		Placeholders: []types.PlaceholderInfo{{
			ID: "Section Number", Role: types.PlaceholderRoleSectionNumber,
		}},
	}}
	cases := []struct {
		name      string
		labels    []string
		ids       []string
		wantCount int
		wantSlide string
	}{
		{name: "first divider contradicts sequence", labels: []string{"02"}, ids: []string{"Section Number"}, wantCount: 1, wantSlide: "/slides/0/"},
		{name: "matching unpadded label", labels: []string{"1"}, ids: []string{"section_number"}},
		{name: "matching padded label", labels: []string{"01"}, ids: []string{"section_no"}},
		{name: "huge numeric label cannot evade mismatch", labels: []string{strings.Repeat("9", 100)}, ids: []string{"section_no"}, wantCount: 1, wantSlide: "/slides/0/"},
		{name: "all zero label mismatches section one", labels: []string{"000"}, ids: []string{"section_no"}, wantCount: 1, wantSlide: "/slides/0/"},
		{name: "only second divider mismatches", labels: []string{"01", "03"}, ids: []string{"large_number", "section_number"}, wantCount: 1, wantSlide: "/slides/1/"},
		{name: "custom labels remain supported", labels: []string{"PART II"}, ids: []string{"section_number"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &PresentationInput{}
			for i, label := range tc.labels {
				in.Slides = append(in.Slides, SlideInput{
					LayoutID: "section-layout", SlideType: "section",
					Content: []ContentInput{sectionTextContent(tc.ids[i], label)},
				})
			}
			got := collectSectionNumberSequenceFindings(in, layouts)
			if len(got) != tc.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tc.wantCount, got)
			}
			if tc.wantCount > 0 {
				if got[0].Code != patterns.ErrCodeSectionNumberSequenceMismatch || got[0].Action != "refuse" {
					t.Fatalf("finding = %+v", got[0])
				}
				if !strings.Contains(got[0].Path, tc.wantSlide) {
					t.Errorf("path = %q, want slide fragment %q", got[0].Path, tc.wantSlide)
				}
			}
		})
	}
}

func TestSectionNumberSequenceFindings_BodyFallback(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID: "section-layout", CanonicalType: types.CanonicalLayoutSectionDivider,
		Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody}},
	}}
	in := &PresentationInput{Slides: []SlideInput{{
		LayoutID: "section-layout", SlideType: "section",
		Content: []ContentInput{sectionTextContent("body", "02")},
	}}}
	got := collectSectionNumberSequenceFindings(in, layouts)
	if len(got) != 1 || got[0].Code != patterns.ErrCodeSectionNumberSequenceMismatch {
		t.Fatalf("fallback finding = %+v", got)
	}
}

func TestContentBearingBlankCanvasRequiresHeadline(t *testing.T) {
	image := "image"
	cases := []struct {
		name  string
		slide SlideInput
		want  bool
	}{
		{name: "pattern", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank", Pattern: &PatternInput{Name: "strategy-house"}}, want: true},
		{name: "shape grid", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank", ShapeGrid: &ShapeGridInput{}}, want: true},
		{name: "compose", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank", Compose: &ComposeInput{}}, want: true},
		{name: "image content", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank", Content: []ContentInput{{Type: image}}}, want: true},
		{name: "empty interstitial", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank"}, want: false},
		{name: "headline satisfies requirement", slide: SlideInput{LayoutID: "blank-canvas", SlideType: "blank", Headline: "A visible point", ShapeGrid: &ShapeGridInput{}}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collectSlideSubstanceFindings(&PresentationInput{Slides: []SlideInput{tc.slide}})
			found := false
			for _, f := range got {
				if f.Code == patterns.ErrCodeMissingTitle {
					found = true
					if f.Fix == nil || f.Fix.Params["path"] != "headline" {
						t.Errorf("blank-canvas title fix = %+v, want headline", f.Fix)
					}
				}
			}
			if found != tc.want {
				t.Errorf("MISSING_TITLE found=%v, want %v; findings=%+v", found, tc.want, got)
			}
		})
	}
}

func TestHeadlineTitleRequirementUsesResolvedBlankLayout(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout9", CanonicalType: types.CanonicalLayoutBlank}}
	cases := []struct {
		name, layout, slideType, fix string
		wantMissing                  bool
	}{
		{name: "headline is ignored on content layout", layout: "content", slideType: "content", fix: "title", wantMissing: true},
		{name: "concrete blank layout renders headline", layout: "slideLayout9", slideType: "blank", fix: "headline", wantMissing: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slide := SlideInput{LayoutID: tc.layout, SlideType: tc.slideType, Headline: "Visible claim", ShapeGrid: &ShapeGridInput{}}
			findings := collectSlideSubstanceFindings(&PresentationInput{Slides: []SlideInput{slide}}, layouts...)
			var missing *patterns.FitFinding
			for i := range findings {
				if findings[i].Code == patterns.ErrCodeMissingTitle {
					missing = &findings[i]
				}
			}
			if (missing != nil) != tc.wantMissing {
				t.Fatalf("missing-title=%v, want %v: %+v", missing != nil, tc.wantMissing, findings)
			}
			if missing != nil && missing.Fix.Params["path"] != tc.fix {
				t.Errorf("fix path = %v, want %s", missing.Fix.Params["path"], tc.fix)
			}
		})
	}
}

// go-slide-creator-r87g: the legibility ceiling read the chart data map's KEYS,
// so a 15-slice pie authored in the STRUCTURED form SKILL.md documents —
// {categories: [...], values: [...]} — counted as two categories and sailed
// through. The detector was blind to exactly the wide datasets it exists for.
func TestChartCategoryLabels_ReadsTheStructuredForm(t *testing.T) {
	cats := make([]any, 15)
	for i := range cats {
		cats[i] = fmt.Sprintf("Site %d", i+1)
	}

	structured := &types.ChartSpec{ //nolint:staticcheck // the authored chart shape
		Type:      "pie",
		Data:      map[string]any{"categories": cats, "values": []any{1, 2, 3}},
		DataOrder: []string{"categories", "values"}, // what the decoder records for this shape
	}
	if got := chartCategoryLabels(structured); len(got) != 15 {
		t.Errorf("structured form read %d categories, want 15 (got %v)", len(got), got)
	}

	// The flat map form still works, and its authored order still wins.
	flat := &types.ChartSpec{ //nolint:staticcheck
		Type:      "pie",
		Data:      map[string]any{"B": 2.0, "A": 1.0},
		DataOrder: []string{"B", "A"},
	}
	if got := chartCategoryLabels(flat); len(got) != 2 || got[0] != "B" {
		t.Errorf("flat form = %v, want the authored order [B A]", got)
	}
}

// The ceiling has to fire on the structured form end to end, not just count it.
func TestChartLegibility_FiresOnStructuredPie(t *testing.T) {
	cats := make([]any, 15)
	for i := range cats {
		cats[i] = fmt.Sprintf("Segment %d", i+1)
	}
	var in PresentationInput
	raw := `{"template":"t","slides":[{"slide_type":"chart","content":[
		{"placeholder_id":"body","type":"chart","chart_value":{"type":"pie","data":{"categories":[],"values":[]}}}]}]}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	in.Slides[0].Content[0].ChartValue.Data = map[string]any{"categories": cats, "values": cats}
	in.Slides[0].Content[0].ChartValue.DataOrder = []string{"categories", "values"}

	var found bool
	for _, f := range collectChartLegibilityFindings(&in) {
		if f.Code == patterns.ErrCodeChartOverloaded {
			found = true
			if !strings.Contains(f.Message, "15 categories") {
				t.Errorf("message does not report the real count: %s", f.Message)
			}
		}
	}
	if !found {
		t.Error("a 15-slice pie in the structured form was not reported")
	}
}

package textfit

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestReadabilityPolicySeparatesFitFromViewingQuality(t *testing.T) {
	report := CheckReadability(1000, 100000, tokens.ViewingModeReport, tokens.TextRoleBody)
	live := CheckReadability(1000, 100000, tokens.ViewingModePresentation, tokens.TextRoleBody)
	if !report.Readable || live.Readable {
		t.Fatalf("10pt body readability report=%v live=%v", report.Readable, live.Readable)
	}
}

func TestMeasureStyledRuns(t *testing.T) {
	cases := []struct {
		name string
		runs []StyledRun
		max  int
	}{
		{"regular", []StyledRun{{Text: "Revenue grew by 18 percent"}}, 2},
		{"bold-mixed", []StyledRun{{Text: "Revenue", Bold: true}, {Text: " grew by 18 percent"}}, 2},
		{"multiline", []StyledRun{{Text: "First bullet\nSecond bullet\nThird bullet"}}, 3},
		{"non-latin", []StyledRun{{Text: "これは読みやすさを測定する文章です"}}, 4},
		{"long-kpi", []StyledRun{{Text: "CHF 142.3 million", Bold: true}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := MeasureStyledRuns(StyledMeasureParams{Runs: tc.runs, FontName: "Arial", FontPt: 12, WidthEMU: 2 * 914400, MaxLines: tc.max, InsetsPt: [4]float64{8, 6, 8, 6}})
			if err != nil {
				t.Fatal(err)
			}
			if m.Lines < 1 || m.RequiredEMU <= 0 {
				t.Fatalf("invalid measurement: %+v", m)
			}
		})
	}
}

func TestMeasureStyledRunsReportsFontSubstitution(t *testing.T) {
	m, err := MeasureStyledRuns(StyledMeasureParams{Runs: []StyledRun{{Text: "Fallback"}}, FontName: "Definitely Missing Font 123", FontPt: 12, WidthEMU: 3 * 914400})
	if err != nil {
		t.Fatal(err)
	}
	if !m.FontSubstituted || m.FontFamily == "" {
		t.Fatalf("substitution not exposed: %+v", m)
	}
}

func TestCalculateNeverReturnsPolicyViolationAsReadable(t *testing.T) {
	r, err := Calculate(Params{WidthEMU: 10 * 914400, HeightEMU: 5 * 914400, FontSizeHPt: 1000, FontName: "Arial", Paragraphs: []string{"Fits easily"}, ViewingMode: tokens.ViewingModePresentation, TextRole: tokens.TextRoleBody})
	if err != nil {
		t.Fatal(err)
	}
	if r.Readable {
		t.Fatalf("10pt projected body reported readable: %+v", r)
	}
}

package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// One deck showed its sources in three sizes, two alignments and two colours
// depending on which pattern owned the slide, and an author who wrote
// "Source: …" into the slide field got "Source: Source: …" on a pattern slide
// (go-slide-creator-7eib).
func TestSourceNoteTextLabelsWithoutStuttering(t *testing.T) {
	cases := map[string]string{
		"Helio finance data warehouse":         "Source: Helio finance data warehouse",
		"Source: Helio finance data warehouse": "Source: Helio finance data warehouse",
		"source: helio":                        "source: helio",
		"  Company filings FY26  ":             "Source: Company filings FY26",
		"":                                     "",
		"   ":                                  "",
	}
	for in, want := range cases {
		if got := SourceNoteText(in); got != want {
			t.Errorf("SourceNoteText(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every pattern that draws its own source line draws it the same way.
func TestPatternSourceLinesShareTheConvention(t *testing.T) {
	t.Run("chart-insights-split", func(t *testing.T) {
		pat, ok := Default().Get("chart-insights-split")
		if !ok {
			t.Fatal("chart-insights-split not registered")
		}
		values := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2"},
					"series":     []any{map[string]any{"name": "Revenue", "values": []any{1, 2}}},
				},
			},
			Insights: []string{"Growth held through the year."},
			Source:   "Helio finance data warehouse",
		}
		grid, err := pat.Expand(ExpandContext{}, values, nil, nil)
		if err != nil {
			t.Fatalf("expand: %v", err)
		}
		assertSourceParagraph(t, grid.Rows, "Source: Helio finance data warehouse")
	})

	t.Run("stat-hero", func(t *testing.T) {
		pat, ok := Default().Get("stat-hero")
		if !ok {
			t.Fatal("stat-hero not registered")
		}
		values := &StatHeroValues{
			Value:  "$2.4B",
			Label:  "Addressable market",
			Source: "Oliver Wyman, 2026",
		}
		grid, err := pat.Expand(ExpandContext{}, values, nil, nil)
		if err != nil {
			t.Fatalf("expand: %v", err)
		}
		assertSourceParagraph(t, grid.Rows, "Source: Oliver Wyman, 2026")
	})
}

// assertSourceParagraph finds the source paragraph in an expanded grid and
// checks it against the shared convention.
func assertSourceParagraph(t *testing.T, rows []jsonschema.GridRowInput, want string) {
	t.Helper()
	for _, row := range rows {
		for _, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
				continue
			}
			var text struct {
				Paragraphs []struct {
					Content string  `json:"content"`
					Size    float64 `json:"size"`
					Color   string  `json:"color"`
					Italic  bool    `json:"italic"`
				} `json:"paragraphs"`
			}
			if json.Unmarshal(cell.Shape.Text, &text) != nil {
				continue
			}
			for _, para := range text.Paragraphs {
				if !strings.Contains(para.Content, "Source:") {
					continue
				}
				if para.Content != want {
					t.Errorf("source line = %q, want %q", para.Content, want)
				}
				if para.Size != SourceNoteSizePt {
					t.Errorf("source size = %.1f, want the shared %.1f", para.Size, SourceNoteSizePt)
				}
				if para.Color != SourceNoteScheme {
					t.Errorf("source colour = %q, want the shared %q", para.Color, SourceNoteScheme)
				}
				if !para.Italic {
					t.Error("source line is not italic")
				}
				return
			}
		}
	}
	t.Errorf("no source paragraph found in the expansion")
}

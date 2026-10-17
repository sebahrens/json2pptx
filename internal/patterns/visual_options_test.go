package patterns

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRAGLegendOnlyUsedColors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scores []TableHighlightScore
		want   []string
	}{
		{"red amber", []TableHighlightScore{"red", "amber"}, []string{"Off track", "At risk"}},
		{"green alias", []TableHighlightScore{"g", "-"}, []string{"On track"}},
		{"all", []TableHighlightScore{"green", "red", "amber"}, []string{"Off track", "At risk", "On track"}},
		{"empty", []TableHighlightScore{"-", "n/a"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &TableHighlightValues{Scale: "rag", Criteria: []TableHighlightCriterion{{Label: "RAG"}, {Label: "Text", Scale: "text"}, {Label: "Harvey", Scale: "harvey"}}, LegendLabelsRAG: []string{"On track", "At risk", "Off track"}}
			for i, score := range tc.scores {
				v.Options = append(v.Options, TableHighlightOption{Name: string(rune('A' + i)), Scores: []TableHighlightScore{score, "green", "4"}})
			}
			var got []string
			if len(thUsedRAG(v)) > 0 {
				cell := thLegendCell(ExpandContext{}, v, thScaleRAG, "dk1", nil, 3, false)
				for _, c := range cell.Grid.Rows[0].Cells {
					if c.Shape == nil || len(c.Shape.Text) == 0 {
						continue
					}
					var text chartInsightsText
					if err := json.Unmarshal(c.Shape.Text, &text); err != nil {
						t.Fatal(err)
					}
					for _, p := range text.Paragraphs {
						got = append(got, p.Content)
					}
				}
			} else {
				for _, kind := range thLegendKinds(v) {
					if kind == thScaleRAG {
						t.Fatal("empty RAG scale has legend")
					}
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("legend = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeroDetailHeadingColor(t *testing.T) {
	p := &heroDetail{}
	for _, style := range []string{"minimal", "cards"} {
		for _, color := range []string{"", "dk1", "accent3"} {
			t.Run(style+color, func(t *testing.T) {
				v := heroValues()
				v.DetailTitleColor = color
				if err := p.Validate(v, &HeroDetailOverrides{Style: style}, nil); err != nil {
					t.Fatal(err)
				}
				grid, err := p.Expand(ExpandContext{}, v, &HeroDetailOverrides{Style: style}, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := color
				if want == "" {
					want = "accent1"
					if style == "cards" {
						want = "lt1"
					}
				}
				found := 0
				for _, row := range grid.Rows {
					for _, cell := range row.Cells {
						if cell.Shape == nil {
							continue
						}
						var text heroDetailText
						if json.Unmarshal(cell.Shape.Text, &text) != nil || len(text.Paragraphs) == 0 {
							continue
						}
						for _, detail := range v.Details {
							if text.Paragraphs[0].Content == detail.Title {
								found++
								if text.Paragraphs[0].Color != want {
									t.Errorf("heading color %q want %q", text.Paragraphs[0].Color, want)
								}
							}
						}
					}
				}
				if found != len(v.Details) {
					t.Errorf("found %d headings, want %d", found, len(v.Details))
				}
			})
		}
	}
	v := heroValues()
	v.DetailTitleColor = "not-a-color"
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "detail_title_color") {
		t.Fatalf("invalid color: %v", err)
	}
	schema, _ := json.Marshal(p.Schema())
	if !strings.Contains(string(schema), "detail_title_color") {
		t.Fatal("missing schema field")
	}
}

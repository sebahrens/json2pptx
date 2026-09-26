package generator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestParagraphLossRepairUsesAuthoredGroups(t *testing.T) {
	largeGroup := make([]string, 10000)
	largeGroup[0], largeGroup[len(largeGroup)-1] = "Parent", "Next parent"
	for i := 1; i < len(largeGroup)-1; i++ {
		largeGroup[i] = "\tRequired child"
	}
	for _, tc := range []struct {
		name     string
		columns  [][]string
		compound bool
		budget   int
	}{
		{"plain", [][]string{{"First", "Second", "Third"}}, false, 1},
		{"nested group", [][]string{{"Parent", "\tChild", "Next"}}, false, 2},
		{"aligned columns", [][]string{{"A", "\tA child", "B"}, {"C", "\tC child", "D"}}, false, 2},
		{"unequal columns", [][]string{{"A", "B"}, {"C"}}, false, 0},
		{"whole indivisible group", [][]string{{"Parent", "\tChild"}}, false, 0},
		{"orphan child", [][]string{{"\tChild", "Root"}}, false, 0},
		{"blank item", [][]string{{"Root", "", "Next"}}, false, 0},
		{"compound", [][]string{{"A", "B"}}, true, 0},
		{"large atomic group", [][]string{largeGroup}, false, 9999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := SlideSpec{}
			base := deckinput.SlideInput{}
			for _, column := range tc.columns {
				slide.Content = append(slide.Content, ContentItem{Type: ContentBullets, Value: column})
				base.Content = append(base.Content, deckinput.ContentInput{Type: "bullets", BulletsValue: &column})
			}
			if tc.compound {
				slide.Content = append(slide.Content, ContentItem{Type: ContentBulletGroups})
			}
			before, err := json.Marshal(slide)
			if err != nil {
				t.Fatal(err)
			}
			for _, code := range []string{patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed} {
				loss := patterns.ValidationError{Code: code, Path: "/slides/0/content/0", Message: "Source omitted", Fix: &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"remaining_count": 999}}}
				got := SourcePreservingParagraphRepair(loss, []SlideSpec{slide})
				if got.Code != code || got.Path != loss.Path {
					t.Fatalf("identity changed: %+v", got)
				}
				if loss.Fix.Kind != "reduce_text" {
					t.Fatal("original finding mutated")
				}
				if tc.budget == 0 {
					if got.Fix != nil || !strings.Contains(got.Message, "cannot use automatic") {
						t.Fatalf("unsupported source given unsafe repair: %+v", got)
					}
					continue
				}
				if got.Fix == nil || got.Fix.Kind != "split_bullets" || got.Fix.Params["max_items"] != tc.budget {
					t.Fatalf("wrong grouped split: %+v", got)
				}
				pages, err := deckinput.SplitBulletSlide(base, got.Fix.Params["max_items"].(int))
				if err != nil {
					t.Fatal(err)
				}
				for i, column := range tc.columns {
					var joined []string
					for _, page := range pages {
						joined = append(joined, (*page.Content[i].BulletsValue)...)
					}
					if !reflect.DeepEqual(joined, column) {
						t.Fatalf("source changed: %v != %v", joined, column)
					}
				}
			}
			after, err := json.Marshal(slide)
			if err != nil || string(before) != string(after) {
				t.Fatal("authored source mutated")
			}
		})
	}
}

func TestParagraphLossRepairRejectsInvalidContextWithoutInventingFix(t *testing.T) {
	for _, path := range []string{"", "/slides/-1/content/0", "/slides/no/content/0", "/slides/2/content/0", "/slides/0/content/-1", "/slides/0/content/2", "/slides/0/content/body", "/slides/0/content/0/extra", "/slides/+0/content/0", "/slides/00/content/0", "/slides/-0/content/0", "/slides/0/content/+0", "/slides/0/content/00", "/slides/0/content/-0"} {
		got := SourcePreservingParagraphRepair(patterns.ValidationError{Code: patterns.ErrCodeTextTrimmed, Path: path}, []SlideSpec{{Content: []ContentItem{{Type: ContentBullets, Value: []string{"A", "B"}}}}})
		if got.Fix != nil {
			t.Fatalf("invalid context %s invented fix: %+v", path, got)
		}
	}
	loss := patterns.ValidationError{Code: patterns.ErrCodeTableRowsTruncated, Fix: &patterns.FixSuggestion{Kind: "split_at_row"}}
	if got := SourcePreservingParagraphRepair(loss, nil); got.Fix != loss.Fix {
		t.Fatal("table repair changed")
	}
}

func TestParagraphLossRepairUnsupportedAuthoredValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		item ContentItem
	}{
		{"ordinary text", ContentItem{Type: ContentText, Value: "Required sentence"}},
		{"nil bullets", ContentItem{Type: ContentBullets}},
		{"empty bullets", ContentItem{Type: ContentBullets, Value: []string{}}},
		{"unconverted generic list", ContentItem{Type: ContentBullets, Value: []any{"A", "B"}}},
		{"string rather than list", ContentItem{Type: ContentBullets, Value: "A\nB"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := SlideSpec{Content: []ContentItem{tc.item}}
			before, err := json.Marshal(slide)
			if err != nil {
				t.Fatal(err)
			}
			for _, code := range []string{patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed} {
				got := SourcePreservingParagraphRepair(patterns.ValidationError{Code: code, Path: "/slides/0/content/0", Fix: &patterns.FixSuggestion{Kind: "reduce_text"}}, []SlideSpec{slide})
				if got.Fix != nil || !strings.Contains(got.Message, "preserving groups") {
					t.Fatalf("unsupported source invented repair: %+v", got)
				}
			}
			after, err := json.Marshal(slide)
			if err != nil || string(after) != string(before) {
				t.Fatal("unsupported authored source mutated")
			}
		})
	}
}

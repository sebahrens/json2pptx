package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestSplitAtRowTargetsSecondTable(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			base := baseSplitSlide(makeRows(2), 2).Base
			right := &TableInput{Headers: []string{"Right case"}, Rows: makeRows(6)}
			for i := range right.Rows {
				right.Rows[i][0].Content = fmt.Sprintf("R%02d", i)
			}
			item := ContentInput{PlaceholderID: "body_2", Type: "table", TableValue: right}
			if legacy {
				data, err := json.Marshal(right)
				if err != nil {
					t.Fatal(err)
				}
				item.TableValue = nil
				item.Value = data
			}
			base.Content = append(base.Content, item)
			deck := &PresentationInput{Slides: []SlideInput{base}}
			fix := applySplitAtRow(deck, 0, map[string]any{"path": "/slides/0/content/2", "row": 2})
			if !fix.Applied || len(deck.Slides) != 3 {
				t.Fatalf("second table not split: %+v, pages=%d", fix, len(deck.Slides))
			}
			for i, slide := range deck.Slides {
				if !reflect.DeepEqual(slide.Content[1], base.Content[1]) {
					t.Fatal("non-target first table changed")
				}
				if slide.Content[2].PlaceholderID != "body_2" || slide.Content[2].TableValue == nil {
					t.Fatal("target position or typed table lost")
				}
				table := slide.Content[2].TableValue
				if len(table.Rows) != 2 || table.Rows[0][0].Content != fmt.Sprintf("R%02d", 2*i) || table.Rows[1][0].Content != fmt.Sprintf("R%02d", 2*i+1) {
					t.Fatalf("wrong source rows on page%d: %+v", i, table.Rows)
				}
			}
		})
	}
}

func TestTableContinuationsPreserveWholeSlideMetadata(t *testing.T) {
	base := baseSplitSlide(makeRows(6), 2)
	base.Base.Eyebrow = "Required context"
	base.Base.Headline = "Required headline"
	base.Base.Takeaway = "Required conclusion"
	base.Base.ColumnLeftPercent = 60
	base.Base.ColumnBaseLayoutID = "original-column-layout"
	base.Base.SectionTitle = "Required section"
	base.Base.Compose = &ComposeInput{Direction: "vertical", Gap: 8}
	base.Base.Overlays = []*OverlayShapeInput{{Kind: "badge", Text: "Required annotation"}}
	base.Base.SourceLink = &LinkInput{URL: "https://example.invalid/evidence"}
	pages, err := expandSplitSlide(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 3 {
		t.Fatalf("pages=%d", len(pages))
	}
	for i, page := range pages {
		want := base.Base
		want.Content = page.Content
		if i > 0 {
			want.SpeakerNotes = ""
			want.Source = ""
			want.SourceLink = nil
		}
		if !reflect.DeepEqual(page, want) {
			t.Fatalf("page%d discarded authored/internal metadata: got=%+v want=%+v", i, page, want)
		}
	}
	if *base.Base.Content[0].TextValue != "Vendor Matrix" || len(base.Base.Content[1].TableValue.Rows) != 6 {
		t.Fatal("base source mutated during split")
	}
}

func TestRepairSlideSecondTablePathAndMetadataSurviveToolResponse(t *testing.T) {
	base := baseSplitSlide(makeRows(2), 2).Base
	right := &TableInput{Headers: []string{"Right case", "Owner"}, Rows: makeRows(6)}
	for i := range right.Rows {
		right.Rows[i][0].Content = fmt.Sprintf("R%02d", i)
	}
	base.Content = append(base.Content, ContentInput{PlaceholderID: "body_2", Type: "table", TableValue: right})
	base.LayoutID = "two-column"
	base.Eyebrow = "Required context"
	base.Takeaway = "Required conclusion"
	base.SourceLink = &LinkInput{URL: "https://example.invalid/evidence"}
	deck := PresentationInput{Template: "midnight-blue", Slides: []SlideInput{base}}
	data, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repairMC(t).handleRepairSlide(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(string(data)), "slide_index": float64(0), "fixes": []any{map[string]any{"kind": "split_at_row", "params": map[string]any{"path": "/slides/0/content/2", "row": float64(2)}}}}))
	if err != nil || result.IsError {
		t.Fatalf("repair tool failed: %v %v", err, result)
	}
	var response repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.AppliedFixes) != 1 || !response.AppliedFixes[0].Applied {
		t.Fatalf("repair was not applied: %+v", response)
	}
	var patched PresentationInput
	if err := json.Unmarshal(response.PatchedDeck, &patched); err != nil {
		t.Fatal(err)
	}
	if len(patched.Slides) != 3 {
		t.Fatalf("tool returned %d pages", len(patched.Slides))
	}
	for i, slide := range patched.Slides {
		if slide.Eyebrow != base.Eyebrow || slide.Takeaway != base.Takeaway {
			t.Fatal("tool discarded authored context or conclusion")
		}
		if len(slide.Content[1].TableValue.Rows) != 2 || slide.Content[2].TableValue.Rows[0][0].Content != fmt.Sprintf("R%02d", 2*i) {
			t.Fatal("tool split wrong table or lost source rows")
		}
		if i == 0 && !reflect.DeepEqual(slide.SourceLink, base.SourceLink) {
			t.Fatal("tool dropped source hyperlink")
		}
	}
}

func TestSelectedTableSplitFailureDoesNotMutateSource(t *testing.T) {
	base := baseSplitSlide(makeRows(2), 2).Base
	base.Content = append(base.Content, ContentInput{PlaceholderID: "body_2", Type: "table", TableValue: &TableInput{Headers: []string{"A", "B"}, Rows: makeRows(6)}})
	base.Content[2].TableValue.Rows[1][0].RowSpan = 2
	for _, params := range []map[string]any{{"path": "/slides/0/content/9", "row": 2}, {"path": "/slides/0/content/2", "row": 0}, {"path": "/slides/0/content/2", "row": 2}} {
		input := &PresentationInput{Slides: []SlideInput{base}}
		before, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		fix := applySplitAtRow(input, 0, params)
		if fix.Applied {
			t.Fatalf("invalid target/budget/cross-page span accepted: %+v", fix)
		}
		after, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("failed repair mutated source")
		}
	}
}

func TestNamedTableSplitRowsDoNotAliasSource(t *testing.T) {
	base := baseSplitSlide(makeRows(2), 2).Base
	right := &TableInput{Headers: []string{"A", "B"}, Rows: makeRows(5)}
	base.Content = append(base.Content, ContentInput{PlaceholderID: "body_2", Type: "table", TableValue: right})
	deck := &PresentationInput{Slides: []SlideInput{base}}
	before, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	fix := applySplitAtRow(deck, 0, map[string]any{"path": "/slides/0/content/body_2", "row": 2})
	if !fix.Applied || len(deck.Slides) != 3 || len(deck.Slides[2].Content[2].TableValue.Rows) != 1 {
		t.Fatalf("named target or partial final page lost: %+v", fix)
	}
	deck.Slides[0].Content[2].TableValue.Rows[0][0].Content = "Edited continuation"
	deck.Slides[0].Content[2].TableValue.Rows[0] = nil
	after, err := json.Marshal(&PresentationInput{Slides: []SlideInput{base}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("editing split row storage mutated the original source")
	}
}

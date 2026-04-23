package main

import (
	"encoding/json"
	"testing"
)

func makeRows(n int) [][]TableCellInput {
	rows := make([][]TableCellInput, n)
	for i := range rows {
		rows[i] = []TableCellInput{
			{Content: "A", ColSpan: 1, RowSpan: 1},
			{Content: "B", ColSpan: 1, RowSpan: 1},
		}
	}
	return rows
}

func baseSplitSlide(rows [][]TableCellInput, groupSize int) SplitSlideInput {
	title := "Vendor Matrix"
	return SplitSlideInput{
		Type: "split_slide",
		Base: SlideInput{
			LayoutID: "slideLayout2",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: &title},
				{
					PlaceholderID: "body",
					Type:          "table",
					TableValue: &TableInput{
						Headers: []string{"Col1", "Col2"},
						Rows:    rows,
					},
				},
			},
			SpeakerNotes: "notes",
			Source:        "source",
		},
		Split: SplitConfig{
			By:            "table.rows",
			GroupSize:     groupSize,
			TitleSuffix:   " ({page}/{total})",
			RepeatHeaders: true,
		},
	}
}

func TestExpandSplitSlide_Basic18Rows(t *testing.T) {
	ss := baseSplitSlide(makeRows(18), 6)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}

	// Check row counts per page
	for i, slide := range slides {
		_, table := findTableContent(slide.Content)
		if table == nil {
			t.Fatalf("slide %d: no table found", i)
		}
		if len(table.Rows) != 6 {
			t.Errorf("slide %d: expected 6 rows, got %d", i, len(table.Rows))
		}
		// Headers should be present on all pages (repeat_headers: true)
		if len(table.Headers) != 2 {
			t.Errorf("slide %d: expected 2 headers, got %d", i, len(table.Headers))
		}
	}
}

func TestExpandSplitSlide_UnevenSplit(t *testing.T) {
	ss := baseSplitSlide(makeRows(7), 3)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}

	expectedRows := []int{3, 3, 1}
	for i, slide := range slides {
		_, table := findTableContent(slide.Content)
		if table == nil {
			t.Fatalf("slide %d: no table found", i)
		}
		if len(table.Rows) != expectedRows[i] {
			t.Errorf("slide %d: expected %d rows, got %d", i, expectedRows[i], len(table.Rows))
		}
	}
}

func TestExpandSplitSlide_GroupSizeGERows(t *testing.T) {
	ss := baseSplitSlide(makeRows(5), 10)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(slides))
	}
	// No suffix should be applied
	for _, ci := range slides[0].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			if *ci.TextValue != "Vendor Matrix" {
				t.Errorf("expected title without suffix, got %q", *ci.TextValue)
			}
		}
	}
}

func TestExpandSplitSlide_TitleSuffix(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 2)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}

	expected := []string{
		"Vendor Matrix (1/3)",
		"Vendor Matrix (2/3)",
		"Vendor Matrix (3/3)",
	}
	for i, slide := range slides {
		for _, ci := range slide.Content {
			if ci.PlaceholderID == "title" && ci.TextValue != nil {
				if *ci.TextValue != expected[i] {
					t.Errorf("slide %d: expected title %q, got %q", i, expected[i], *ci.TextValue)
				}
			}
		}
	}
}

func TestExpandSplitSlide_RepeatHeadersFalse(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 2)
	ss.Split.RepeatHeaders = false
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}

	// First slide should have headers
	_, t0 := findTableContent(slides[0].Content)
	if len(t0.Headers) != 2 {
		t.Errorf("slide 0: expected 2 headers, got %d", len(t0.Headers))
	}

	// Subsequent slides should NOT have headers
	for i := 1; i < len(slides); i++ {
		_, ti := findTableContent(slides[i].Content)
		if len(ti.Headers) != 0 {
			t.Errorf("slide %d: expected 0 headers, got %d", i, len(ti.Headers))
		}
	}
}

func TestExpandSplitSlide_SpeakerNotesOnlyFirstPage(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 2)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slides[0].SpeakerNotes != "notes" {
		t.Errorf("slide 0: expected speaker notes, got %q", slides[0].SpeakerNotes)
	}
	if slides[0].Source != "source" {
		t.Errorf("slide 0: expected source, got %q", slides[0].Source)
	}
	for i := 1; i < len(slides); i++ {
		if slides[i].SpeakerNotes != "" {
			t.Errorf("slide %d: expected empty speaker notes, got %q", i, slides[i].SpeakerNotes)
		}
		if slides[i].Source != "" {
			t.Errorf("slide %d: expected empty source, got %q", i, slides[i].Source)
		}
	}
}

func TestValidateSplitSlide_InvalidBy(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 2)
	ss.Split.By = "content.bullets"
	_, err := expandSplitSlide(ss)
	if err == nil {
		t.Fatal("expected error for invalid split.by")
	}
	if got := err.Error(); !contains(got, "split.by must be 'table.rows'") {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestValidateSplitSlide_NoTable(t *testing.T) {
	title := "No Table"
	ss := SplitSlideInput{
		Type: "split_slide",
		Base: SlideInput{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: &title},
			},
		},
		Split: SplitConfig{By: "table.rows", GroupSize: 3},
	}
	_, err := expandSplitSlide(ss)
	if err == nil {
		t.Fatal("expected error for missing table")
	}
	if got := err.Error(); !contains(got, "must contain a table") {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestValidateSplitSlide_GroupSizeZero(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 0)
	_, err := expandSplitSlide(ss)
	if err == nil {
		t.Fatal("expected error for group_size=0")
	}
	if got := err.Error(); !contains(got, "group_size must be > 0") {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestValidateSplitSlide_TablePlusChart(t *testing.T) {
	ss := baseSplitSlide(makeRows(6), 2)
	ss.Base.Content = append(ss.Base.Content, ContentInput{
		PlaceholderID: "chart1",
		Type:          "chart",
	})
	_, err := expandSplitSlide(ss)
	if err == nil {
		t.Fatal("expected error for table + chart")
	}
	if got := err.Error(); !contains(got, "cannot contain both a table and a chart") {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestValidateSplitSlide_RowSpanCrossesBoundary(t *testing.T) {
	rows := [][]TableCellInput{
		{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "B", ColSpan: 1, RowSpan: 3}},
		{{Content: "C", ColSpan: 1, RowSpan: 1}, {Content: "", ColSpan: 1, RowSpan: 1}},
		{{Content: "D", ColSpan: 1, RowSpan: 1}, {Content: "", ColSpan: 1, RowSpan: 1}},
		{{Content: "E", ColSpan: 1, RowSpan: 1}, {Content: "F", ColSpan: 1, RowSpan: 1}},
	}
	// groupSize=2: boundary at row 2, row 0 has cell with RowSpan=3 crossing it
	ss := baseSplitSlide(rows, 2)
	_, err := expandSplitSlide(ss)
	if err == nil {
		t.Fatal("expected error for row_span crossing boundary")
	}
	if got := err.Error(); !contains(got, "row_span=3 that crosses split boundary") {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestExpandSplitSlide_RowSpanWithinBoundary(t *testing.T) {
	rows := [][]TableCellInput{
		{{Content: "A", ColSpan: 1, RowSpan: 2}, {Content: "B", ColSpan: 1, RowSpan: 1}},
		{{Content: "", ColSpan: 1, RowSpan: 1}, {Content: "C", ColSpan: 1, RowSpan: 1}},
		{{Content: "D", ColSpan: 1, RowSpan: 1}, {Content: "E", ColSpan: 1, RowSpan: 1}},
		{{Content: "F", ColSpan: 1, RowSpan: 1}, {Content: "G", ColSpan: 1, RowSpan: 1}},
	}
	// groupSize=2: boundary at row 2, row 0 has RowSpan=2 which fits within boundary
	ss := baseSplitSlide(rows, 2)
	slides, err := expandSplitSlide(ss)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(slides))
	}
}

func TestUnmarshalPresentationInput_MixedSlides(t *testing.T) {
	input := `{
		"template": "midnight-blue",
		"slides": [
			{
				"layout_id": "layout1",
				"content": [
					{"placeholder_id": "title", "type": "text", "text_value": "Intro"}
				]
			},
			{
				"type": "split_slide",
				"base": {
					"layout_id": "layout2",
					"content": [
						{"placeholder_id": "title", "type": "text", "text_value": "Data"},
						{
							"placeholder_id": "body",
							"type": "table",
							"table_value": {
								"headers": ["A", "B"],
								"rows": [["1","2"],["3","4"],["5","6"],["7","8"]]
							}
						}
					]
				},
				"split": {
					"by": "table.rows",
					"group_size": 2,
					"title_suffix": " ({page}/{total})",
					"repeat_headers": true
				}
			},
			{
				"layout_id": "layout3",
				"content": [
					{"placeholder_id": "title", "type": "text", "text_value": "Outro"}
				]
			}
		]
	}`

	var p PresentationInput
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if p.Template != "midnight-blue" {
		t.Errorf("expected template midnight-blue, got %q", p.Template)
	}

	// Expected: Intro + 2 split pages + Outro = 4 slides
	if len(p.Slides) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(p.Slides))
	}

	// Check Intro slide
	if p.Slides[0].LayoutID != "layout1" {
		t.Errorf("slide 0: expected layout1, got %q", p.Slides[0].LayoutID)
	}

	// Check split slides have correct titles
	for _, ci := range p.Slides[1].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			if *ci.TextValue != "Data (1/2)" {
				t.Errorf("slide 1: expected 'Data (1/2)', got %q", *ci.TextValue)
			}
		}
	}
	for _, ci := range p.Slides[2].Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil {
			if *ci.TextValue != "Data (2/2)" {
				t.Errorf("slide 2: expected 'Data (2/2)', got %q", *ci.TextValue)
			}
		}
	}

	// Check Outro slide
	if p.Slides[3].LayoutID != "layout3" {
		t.Errorf("slide 3: expected layout3, got %q", p.Slides[3].LayoutID)
	}
}

func TestUnmarshalPresentationInput_InvalidSplitSlide(t *testing.T) {
	input := `{
		"template": "test",
		"slides": [
			{
				"type": "split_slide",
				"base": {
					"layout_id": "layout1",
					"content": [
						{"placeholder_id": "title", "type": "text", "text_value": "No table"}
					]
				},
				"split": {"by": "table.rows", "group_size": 3}
			}
		]
	}`

	var p PresentationInput
	err := json.Unmarshal([]byte(input), &p)
	if err == nil {
		t.Fatal("expected unmarshal error for split_slide without table")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

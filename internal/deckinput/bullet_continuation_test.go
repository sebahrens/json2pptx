package deckinput

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSplitBulletSlidePreservesColumnsAndMetadata(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "legacy"}[legacy], func(t *testing.T) {
			left := []string{"<b>L0 Café</b>", "\tL1 résumé", "L2 17% not optional", "L3", "L4"}
			right := []string{"R0", "    R1 child", "R2", "R3", "R4"}
			title := "Source title"
			font := 18.0
			base := SlideInput{LayoutID: "native", ColumnLeftPercent: 40, ColumnBaseLayoutID: "native", SlideType: "two-column", Eyebrow: "Review", Headline: "Headline", SpeakerNotes: "Notes", Source: "Visible source", SourceLink: &LinkInput{URL: "https://example.com/source"}, Takeaway: "Takeaway", Transition: "fade", Build: "by-paragraph", SectionTitle: "Section", Background: &BackgroundInput{Color: "lt1"}, Content: []ContentInput{{Type: "text", PlaceholderID: "title", TextValue: &title}, {Type: "bullets", PlaceholderID: "body_1", BulletsValue: &left, FontSize: &font, Link: &LinkInput{URL: "https://example.com/left"}}, {Type: "bullets", PlaceholderID: "body_2", BulletsValue: &right}}}
			if legacy {
				for i := 1; i < len(base.Content); i++ {
					b, _ := json.Marshal(*base.Content[i].BulletsValue)
					base.Content[i].Value, base.Content[i].BulletsValue = b, nil
				}
			}
			before, _ := json.Marshal(base)
			pages, err := SplitBulletSlide(base, 2)
			if err != nil || len(pages) != 3 {
				t.Fatalf("pages=%d err=%v", len(pages), err)
			}
			for i, page := range pages {
				want := base
				want.Content = page.Content
				if i > 0 {
					want.SpeakerNotes, want.Source, want.SourceLink = "", "", nil
				}
				if !reflect.DeepEqual(page, want) || !reflect.DeepEqual(page.Content[0], base.Content[0]) {
					t.Fatal("native layout, non-bullet content or metadata changed")
				}
				for col := 1; col <= 2; col++ {
					wantItem := base.Content[col]
					wantItem.BulletsValue, wantItem.Value = page.Content[col].BulletsValue, nil
					if !reflect.DeepEqual(wantItem, page.Content[col]) {
						t.Fatal("bullet style/link changed")
					}
				}
			}
			for col, want := range [][]string{left, right} {
				var joined []string
				for _, page := range pages {
					joined = append(joined, (*page.Content[col+1].BulletsValue)...)
				}
				if !reflect.DeepEqual(joined, want) {
					t.Fatal("strings omitted, rewritten or reordered")
				}
			}
			after, _ := json.Marshal(base)
			if string(before) != string(after) {
				t.Fatal("source mutated")
			}
			(*pages[0].Content[1].BulletsValue)[0] = "edit"
			if (*pages[1].Content[1].BulletsValue)[0] != "L2 17% not optional" || left[0] != "<b>L0 Café</b>" {
				t.Fatal("page bullet slices alias source or other pages")
			}
		})
	}
}

func TestSplitBulletSlideBoundaryAndRejection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cols        [][]string
		budget      int
		compound    string
		legacy      json.RawMessage
		wantLengths []int
	}{
		{name: "nested-boundary-backs-up", cols: [][]string{{"a", "b", "\tc", "d"}}, budget: 2, wantLengths: []int{1, 2, 1}},
		{name: "coordinated-nesting", cols: [][]string{{"a", "b", "c", "d"}, {"w", "x", "    y", "z"}}, budget: 2, wantLengths: []int{1, 2, 1}},
		{name: "single-page", cols: [][]string{{"a", "b"}}, budget: 10, wantLengths: []int{2}},
		{name: "zero", cols: [][]string{{"a"}}, budget: 0},
		{name: "negative", cols: [][]string{{"a"}}, budget: -1},
		{name: "none", budget: 2},
		{name: "empty", cols: [][]string{{}}, budget: 2},
		{name: "blank", cols: [][]string{{"a", "", "\tb"}}, budget: 2},
		{name: "whitespace", cols: [][]string{{"a", " \t ", "\tb"}}, budget: 2},
		{name: "unequal", cols: [][]string{{"a"}, {"a", "b"}}, budget: 2},
		{name: "orphan-tab", cols: [][]string{{"\ta"}}, budget: 2},
		{name: "orphan-spaces", cols: [][]string{{"    a"}}, budget: 2},
		{name: "oversize-group", cols: [][]string{{"a", "\tb", "\t\tc"}}, budget: 2},
		{name: "body", cols: [][]string{{"a"}}, compound: "body_and_bullets", budget: 2},
		{name: "groups", cols: [][]string{{"a"}}, compound: "bullet_groups", budget: 2},
		{name: "lead", cols: [][]string{{"a"}}, compound: "body_and_lead", budget: 2},
		{name: "malformed", legacy: json.RawMessage(`{"x":1}`), budget: 2},
		{name: "null", legacy: json.RawMessage(`null`), budget: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := SlideInput{}
			for i := range tc.cols {
				base.Content = append(base.Content, ContentInput{Type: "bullets", BulletsValue: &tc.cols[i]})
			}
			if tc.compound != "" {
				base.Content = append(base.Content, ContentInput{Type: tc.compound})
			}
			if tc.legacy != nil {
				base.Content = append(base.Content, ContentInput{Type: "bullets", Value: tc.legacy})
			}
			before, _ := json.Marshal(base)
			pages, err := SplitBulletSlide(base, tc.budget)
			if tc.wantLengths == nil {
				if err == nil || pages != nil {
					t.Fatal("invalid input not rejected atomically")
				}
			} else {
				if err != nil || len(pages) != len(tc.wantLengths) {
					t.Fatalf("pages=%d err=%v", len(pages), err)
				}
				for i, page := range pages {
					for _, item := range page.Content {
						if len(*item.BulletsValue) != tc.wantLengths[i] {
							t.Fatal("incorrect coordinated boundary")
						}
					}
				}
			}
			after, _ := json.Marshal(base)
			if string(before) != string(after) {
				t.Fatal("source changed on failure/success")
			}
		})
	}
}

package patterns

import (
	"strings"
	"testing"
)

func agendaWithImagesBudgetValues(rows int, images bool) *AgendaWithImagesValues {
	v := &AgendaWithImagesValues{}
	for i := 0; i < rows; i++ {
		item := AgendaWithImagesItem{Title: "Title"}
		if rows <= 4 {
			// Five or six rows hold no readable subtitle.
			item.Subtitle = "Summary"
		}
		if images {
			item.ImageLabel = "Image"
		}
		v.Items = append(v.Items, item)
	}
	return v
}

func TestAgendaWithImagesSubtitleWarnings(t *testing.T) {
	pat := &agendaWithImages{}
	for _, tc := range []struct {
		name        string
		rows        int
		images      bool
		titleChars  int
		budget      int
		warningText string
	}{
		{"three rows with images", 3, true, 80, 160, ""},
		{"three rows without images", 3, false, 80, 160, ""},
		{"four rows without images", 4, false, 80, 111, "about 111"},
		{"four image rows readable title", 4, true, 65, 65, "about 65"},
		{"four image rows long title", 4, true, 66, 0, "no readable subtitle room"},
		{"five rows without images", 5, false, 80, 0, "no readable subtitle room"},
		{"five image rows", 5, true, 65, 0, "no readable subtitle room"},
		{"six rows without images", 6, false, 80, 0, "no readable subtitle room"},
		{"six image rows", 6, true, 61, 0, "no readable subtitle room"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := agendaWithImagesBudgetValues(tc.rows, tc.images)
			v.Items[1].Title = strings.Repeat("T", tc.titleChars)
			v.Items[1].Subtitle = strings.Repeat("S", tc.budget)
			if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
				t.Fatalf("at readable budget: %v", got)
			}
			if tc.budget == 160 {
				return
			}
			v.Items[1].Subtitle += "S"
			got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
				!strings.Contains(got[0], "items[1].subtitle") || !strings.Contains(got[0], tc.warningText) {
				t.Fatalf("over readable budget: %v", got)
			}
		})
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	props := pat.Schema().raw.Properties["values"].raw.Properties["items"].raw.Items.raw.Properties
	if max := props["subtitle"].raw.MaxLength; max == nil || *max != 160 ||
		!strings.Contains(props["subtitle"].raw.Description, "5-6 rows hold no readable subtitle") {
		t.Fatalf("subtitle schema loses sparse maximum or dense guidance: %+v", props["subtitle"].raw)
	}
}

func TestAgendaWithImagesTitleAndLabelWarnings(t *testing.T) {
	pat := &agendaWithImages{}
	for _, tc := range []struct {
		rows, title, label int
	}{
		{5, 65, 41},
		{6, 61, 40},
	} {
		v := agendaWithImagesBudgetValues(tc.rows, true)
		v.Items[1].Title = strings.Repeat("T", tc.title)
		v.Items[1].ImageLabel = strings.Repeat("L", tc.label)
		if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
			t.Fatalf("%d rows at title/label budget: %v", tc.rows, got)
		}
		v.Items[1].Title += "T"
		v.Items[1].ImageLabel += "L"
		got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
		if len(got) != 2 || !strings.Contains(got[0], "items[1].title") || !strings.Contains(got[1], "items[1].image_label") {
			t.Fatalf("%d rows over title/label budget: %v", tc.rows, got)
		}
	}
	// Without image labels the title keeps its schema maximum.
	v := agendaWithImagesBudgetValues(6, false)
	v.Items[1].Title = strings.Repeat("T", 80)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("title without image labels: %v", got)
	}
}

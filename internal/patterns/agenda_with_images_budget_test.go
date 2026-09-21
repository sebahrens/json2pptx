package patterns

import (
	"strings"
	"testing"
)

func agendaWithImagesBudgetValues(rows int, images bool) *AgendaWithImagesValues {
	v := &AgendaWithImagesValues{}
	for i := 0; i < rows; i++ {
		item := AgendaWithImagesItem{Title: "Title", Subtitle: "Summary"}
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
		{"four rows with images", 4, true, 80, 160, ""},
		{"six rows without images", 6, false, 80, 160, ""},
		{"five image rows short title", 5, true, 65, 150, "about 150"},
		{"five image rows long title", 5, true, 80, 75, "about 75"},
		{"six image rows short title", 6, true, 65, 150, "about 150"},
		{"six image rows long title", 6, true, 80, 0, "no readable subtitle room"},
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
		!strings.Contains(props["subtitle"].raw.Description, "75 at 5 rows") {
		t.Fatalf("subtitle schema loses sparse maximum or dense guidance: %+v", props["subtitle"].raw)
	}
}

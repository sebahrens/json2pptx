package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func stylishBudgetValues(panels, bullets, titleChars, bodyChars int) *StylishPanelsValues {
	v := StylishPanelsValues{}
	for i := 0; i < panels; i++ {
		item := StylishPanelsItem{Title: strings.Repeat("T", titleChars)}
		for j := 0; j < bullets; j++ {
			item.Body = append(item.Body, strings.Repeat("B", bodyChars))
		}
		v = append(v, item)
	}
	return &v
}

func TestStylishPanelsBodyWarningsUseAverageCopy(t *testing.T) {
	pat := &stylishPanels{}
	for _, tc := range []struct {
		panels, bullets, title, budget int
	}{
		{3, 6, 30, 121},
		{3, 6, 31, 81},
		{4, 3, 20, 180},
		{4, 3, 21, 150},
		{5, 2, 16, 182},
		{5, 2, 17, 162},
		{5, 8, 80, 26},
	} {
		t.Run(fmt.Sprintf("%d panels %d bullets %d title", tc.panels, tc.bullets, tc.title), func(t *testing.T) {
			v := stylishBudgetValues(tc.panels, tc.bullets, tc.title, tc.budget)
			if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
				t.Fatalf("at average readable budget: %v", got)
			}
			if tc.budget == 200 {
				return
			}
			v0 := *v
			v0[0].Body = append([]string(nil), v0[0].Body...)
			v0[0].Body[0] += strings.Repeat("X", tc.bullets)
			got := pat.PostExpandWarnings(ExpandContext{}, &v0, nil)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
				!strings.Contains(got[0], "values[0].body") ||
				!strings.Contains(got[0], fmt.Sprintf("about %d characters per bullet", tc.budget)) {
				t.Fatalf("over average readable budget: %v", got)
			}
		})
	}
	v := stylishBudgetValues(3, 8, 80, 5)
	(*v)[0].Body[0] = strings.Repeat("B", 200)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("one long bullet should fit with short neighbors: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	body := pat.Schema().raw.Properties["values"].raw.Items.raw.Properties["body"]
	if body.raw.Items.raw.MaxLength == nil || *body.raw.Items.raw.MaxLength != 200 ||
		!strings.Contains(body.raw.Description, "5 panels 200/182/122") {
		t.Fatalf("schema loses sparse maximum or panel-density guidance: %+v", body.raw)
	}
}

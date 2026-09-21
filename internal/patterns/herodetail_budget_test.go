package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func TestHeroDetailIconBodyWarnings(t *testing.T) {
	pat := &heroDetail{}
	for _, tc := range []struct {
		details, title, budget int
		icon                   bool
	}{
		{3, 38, 200, true},
		{3, 39, 181, true},
		{4, 27, 90, true},
		{4, 28, 60, true},
		{4, 49, 30, true},
		{4, 60, 200, false},
	} {
		t.Run(fmt.Sprintf("%d cards %d title icon=%t", tc.details, tc.title, tc.icon), func(t *testing.T) {
			v := &HeroDetailValues{Hero: HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
			for i := 0; i < tc.details; i++ {
				v.Details = append(v.Details, HeroDetailItem{Title: "Driver", Body: "Brief"})
			}
			v.Details[1].Title = strings.Repeat("T", tc.title)
			v.Details[1].Body = strings.Repeat("B", tc.budget)
			if tc.icon {
				v.Details[1].Icon = &IconRef{Name: "rocket"}
			}
			if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
				t.Fatalf("at readable budget: %v", got)
			}
			if tc.budget == 200 {
				return
			}
			v.Details[1].Body += "B"
			got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
				!strings.Contains(got[0], "details[1].body") ||
				!strings.Contains(got[0], fmt.Sprintf("about %d", tc.budget)) {
				t.Fatalf("over readable budget: %v", got)
			}
		})
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	body := pat.Schema().raw.Properties["values"].raw.Properties["details"].raw.Items.raw.Properties["body"]
	if body.raw.MaxLength == nil || *body.raw.MaxLength != 200 ||
		!strings.Contains(body.raw.Description, "90/60/30") {
		t.Fatalf("schema loses sparse maximum or dense guidance: %+v", body.raw)
	}
}

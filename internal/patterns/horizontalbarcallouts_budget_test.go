package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func TestHorizontalBarCalloutBudgetWarnings(t *testing.T) {
	pat := &horizontalBarCallouts{}
	for _, tc := range []struct {
		bars, readable int
	}{
		{3, 200}, {5, 200}, {6, 181}, {7, 121}, {8, 121},
	} {
		t.Run(fmt.Sprintf("%d bars", tc.bars), func(t *testing.T) {
			v := &HorizontalBarCalloutsValues{Unit: "%"}
			for i := 0; i < tc.bars; i++ {
				v.Bars = append(v.Bars, HorizontalBarCalloutsBar{Label: "Item", Value: 80, Callout: "Insight"})
			}
			v.Bars[1].Callout = strings.Repeat("x", tc.readable)
			if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
				t.Fatalf("at readable budget: %v", got)
			}
			if tc.readable == hbcCalloutMax {
				return
			}
			v.Bars[1].Callout += "x"
			got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
				!strings.Contains(got[0], "bars[1].callout") ||
				!strings.Contains(got[0], fmt.Sprintf("about %d", tc.readable)) {
				t.Fatalf("over readable budget: %v", got)
			}
		})
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	callout := pat.Schema().raw.Properties["values"].raw.Properties["bars"].raw.Items.raw.Properties["callout"]
	if callout.raw.MaxLength == nil || *callout.raw.MaxLength != 200 || !strings.Contains(callout.raw.Description, "121 with 7-8") {
		t.Fatalf("schema loses sparse maximum or dense guidance: %+v", callout.raw)
	}
}

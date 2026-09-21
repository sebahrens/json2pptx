package patterns

import (
	"strings"
	"testing"
)

func TestKPIInlineCaptionBudgetDependsOnDenseIconContent(t *testing.T) {
	for _, tc := range []struct {
		cells, big, sub, want int
		icon                  bool
	}{
		{2, 8, 12, 40, true}, {4, 8, 12, 40, true}, {5, 8, 12, 40, false},
		{5, 3, 0, 40, true}, {5, 3, 3, 31, true}, {5, 7, 12, 31, true},
		{5, 8, 0, 16, true}, {5, 8, 1, 0, true},
		{6, 5, 0, 31, true}, {6, 5, 3, 21, true}, {6, 5, 10, 21, true},
		{6, 5, 11, 11, true}, {6, 6, 0, 11, true}, {6, 6, 1, 0, true},
	} {
		if got := kpiInlineCaptionBudget(tc.cells, tc.big, tc.sub, tc.icon); got != tc.want {
			t.Errorf("budget(%d,%d,%d,%t)=%d, want %d", tc.cells, tc.big, tc.sub, tc.icon, got, tc.want)
		}
	}
	values := (&kpiInline{}).Schema().raw.Properties["values"]
	if !strings.Contains(values.raw.Description, "no readable caption") {
		t.Errorf("schema omits dense icon limit: %q", values.raw.Description)
	}
	cell := values.raw.Items.raw.OneOf[1]
	if max := cell.raw.Properties["small"].raw.MaxLength; max == nil || *max != 40 {
		t.Error("sparse caption maximum was reduced")
	}
}

func TestKPIInlineWarningsNameTheSpecificCellAndFix(t *testing.T) {
	pat := &kpiInline{}
	v := KPINupValues{}
	for i := 0; i < 5; i++ {
		v = append(v, KPICell{Big: "42%", Small: "Revenue"})
	}
	v[2].Icon = &IconRef{Name: "rocket"}
	v[2].Big = "12345678"
	v[2].Small = strings.Repeat("C", 16)
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, nil); len(got) != 0 {
		t.Fatalf("at budget: %v", got)
	}
	v[2].Small += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, &v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "values[2].small") || !strings.Contains(got[0], "about 16") {
		t.Fatalf("caption warning: %v", got)
	}
	v[2].Sub = "+5%"
	got = pat.PostExpandWarnings(ExpandContext{}, &v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "no readable caption fits") || !strings.Contains(got[0], "omit the delta/icon") {
		t.Fatalf("composition warning: %v", got)
	}
	v[2].Icon = nil
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, nil); len(got) != 0 {
		t.Fatalf("icon removed: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

package patterns

import (
	"strings"
	"testing"
)

func compactBudgetValues(items int) *BeforeAfterValues {
	v := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Current"},
		After:  BeforeAfterColumn{Header: "Future"},
	}
	for i := 0; i < items; i++ {
		v.Before.Items = append(v.Before.Items, "Short")
		v.After.Items = append(v.After.Items, "Short")
	}
	return v
}

func TestBeforeAfterCompactWarningsUseSharedLineBudget(t *testing.T) {
	pat := &beforeAfterCompact{}
	v := compactBudgetValues(8)
	v.Before.Items[0] = strings.Repeat("w", 200)
	v.Before.Items[1] = strings.Repeat("w", 200)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("two long bullets fit with short headers: %v", got)
	}
	v.Before.Items[2] = strings.Repeat("w", 68)
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
		!strings.Contains(got[0], "before.items") || !strings.Contains(got[0], "holds about 12 lines") {
		t.Fatalf("over short-header line budget: %v", got)
	}
	v.Before.Items[2] = "Short"
	v.Before.Header = strings.Repeat("H", 47)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "holds about 10 lines") {
		t.Fatalf("long header should reduce the line budget: %v", got)
	}
	v.Before.Items[1] = "Short"
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("one long bullet fits with a long header: %v", got)
	}
	v.After.Items[0] = strings.Repeat("w", 200)
	v.After.Items[1] = strings.Repeat("w", 200)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "after.items") {
		t.Fatalf("other column warning: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	column := pat.Schema().raw.Properties["values"].raw.Properties["before"]
	items := column.raw.Properties["items"]
	if items.raw.Items.raw.MaxLength == nil || *items.raw.Items.raw.MaxLength != 200 ||
		!strings.Contains(items.raw.Description, "12 lines") || !strings.Contains(items.raw.Description, "10 if either header") {
		t.Fatalf("schema loses sparse maximum or shared line guidance: %+v", items.raw)
	}
}

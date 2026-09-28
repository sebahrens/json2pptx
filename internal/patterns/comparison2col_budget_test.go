package patterns

import (
	"strings"
	"testing"
)

func TestComparisonBodyBudgetUsesEffectiveRows(t *testing.T) {
	for _, tc := range []struct {
		rows    int
		headers bool
		want    int
	}{
		{1, false, 200}, {2, true, 200}, {3, false, 200},
		{3, true, 193}, {4, false, 193}, {4, true, 65},
		{5, false, 65}, {7, false, 65}, {7, true, 65},
		{10, false, 65}, {9, true, 65}, {10, true, 65},
	} {
		if got := comparisonBodyBudget(tc.rows, tc.headers); got != tc.want {
			t.Errorf("comparisonBodyBudget(%d, %t) = %d, want %d", tc.rows, tc.headers, got, tc.want)
		}
	}
	schema := (&comparison2col{}).Schema()
	row := schema.raw.Properties["values"].raw.Properties["rows"].raw.Items
	if !strings.Contains(row.raw.Description, "5-11: 65") {
		t.Errorf("schema omits the dense-row copy target: %q", row.raw.Description)
	}
	objectRow := row.raw.OneOf[1]
	for _, side := range []string{"left", "right"} {
		maxLength := objectRow.raw.Properties[side].raw.MaxLength
		if maxLength == nil || *maxLength != 200 {
			t.Errorf("%s cap was reduced for sparse comparisons", side)
		}
	}
}

func TestComparisonWarningsNameTheCellAndTarget(t *testing.T) {
	pat := &comparison2col{}
	v := &Comparison2colValues{Rows: make([]Comparison2colRow, 4)}
	for i := range v.Rows {
		v.Rows[i] = Comparison2colRow{Left: "Left", Right: "Right"}
	}
	v.Rows[0].Left = strings.Repeat("L", 193)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("at budget: %v", got)
	}
	v.Rows[0].Left += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "rows[0].left") || !strings.Contains(got[0], "about 193") {
		t.Fatalf("without headers: %v", got)
	}
	v.Headers = [2]string{"Left", "Right"}
	v.Rows[0].Left = strings.Repeat("L", 66)
	v.Rows[3].Right = strings.Repeat("R", 66)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 2 || !strings.Contains(got[0], "about 65") || !strings.Contains(got[1], "rows[3].right") {
		t.Fatalf("with headers: %v", got)
	}
	v.Headers = [2]string{}
	v.HeaderLeft = "Legacy"
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 2 {
		t.Fatalf("legacy header must consume a row: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

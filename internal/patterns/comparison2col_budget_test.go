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
		{1, false, 200}, {3, true, 200}, {4, false, 200},
		{4, true, 196}, {5, false, 196}, {5, true, 131},
		{6, false, 131}, {7, false, 131}, {7, true, 66},
		{8, false, 66}, {10, true, 66},
	} {
		if got := comparisonBodyBudget(tc.rows, tc.headers); got != tc.want {
			t.Errorf("comparisonBodyBudget(%d, %t) = %d, want %d", tc.rows, tc.headers, got, tc.want)
		}
	}
	schema := (&comparison2col{}).Schema()
	row := schema.raw.Properties["values"].raw.Properties["rows"].raw.Items
	if !strings.Contains(row.raw.Description, "8-11: 66") {
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
	v := &Comparison2colValues{Rows: make([]Comparison2colRow, 7)}
	for i := range v.Rows {
		v.Rows[i] = Comparison2colRow{Left: "Left", Right: "Right"}
	}
	v.Rows[0].Left = strings.Repeat("L", 131)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("at budget: %v", got)
	}
	v.Rows[0].Left += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "rows[0].left") || !strings.Contains(got[0], "about 131") {
		t.Fatalf("without headers: %v", got)
	}
	v.Headers = [2]string{"Left", "Right"}
	v.Rows[0].Left = strings.Repeat("L", 67)
	v.Rows[6].Right = strings.Repeat("R", 67)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 2 || !strings.Contains(got[0], "about 66") || !strings.Contains(got[1], "rows[6].right") {
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

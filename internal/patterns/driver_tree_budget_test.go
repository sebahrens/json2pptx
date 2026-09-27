package patterns

import (
	"strings"
	"testing"
)

func budgetDriverTree(counts ...int) *DriverTreeValues {
	v := &DriverTreeValues{Root: DriverTreeNode{Label: "Root"}}
	for _, count := range counts {
		b := DriverTreeBranch{Label: "Branch"}
		for i := 0; i < count; i++ {
			b.Leaves = append(b.Leaves, "Item")
		}
		v.Branches = append(v.Branches, b)
	}
	return v
}

func TestDriverTreeMeasuredBudgets(t *testing.T) {
	for _, tc := range []struct {
		total     int
		annotated bool
		want      int
	}{
		{5, true, 120}, {7, false, 120}, {7, true, 90}, {8, false, 66}, {8, true, 45},
		{9, false, 62}, {12, true, 45}, {13, false, 0},
	} {
		if got := driverTreeLeafBudget(tc.total, tc.annotated); got != tc.want {
			t.Errorf("leafBudget(%d, %t) = %d, want %d", tc.total, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct {
		total, span, want int
		annotated         bool
	}{
		{7, 1, 52, true}, {10, 1, 35, false}, {10, 1, 27, true},
		{12, 2, 58, true}, {11, 2, 60, true}, {13, 4, 0, false},
	} {
		if got := driverTreeBranchBudget(tc.total, tc.span, tc.annotated); got != tc.want {
			t.Errorf("branchBudget(%d, %d, %t) = %d, want %d", tc.total, tc.span, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct{ total, span, want int }{
		{3, 1, 140}, {4, 1, 100}, {5, 1, 62}, {6, 1, 48},
		{8, 1, 23}, {10, 1, 22}, {8, 2, 100}, {10, 2, 62},
		{12, 2, 48}, {12, 3, 100}, {10, 3, 122}, {12, 4, 140},
	} {
		if got := driverTreeAnnotationBudget(tc.total, tc.span); got != tc.want {
			t.Errorf("annotationBudget(%d, %d) = %d, want %d", tc.total, tc.span, got, tc.want)
		}
	}
}

func TestDriverTreeWarningsNameTheBindingContent(t *testing.T) {
	pat := &driverTree{}
	v := budgetDriverTree(4, 4, 2)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("short tree warned: %v", got)
	}
	v.Branches[0].Leaves[0] = strings.Repeat("L", 63)
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[0].leaves[0]") || !strings.Contains(got[0], "about 62") {
		t.Fatalf("leaf warning = %v", got)
	}
	v.Branches[0].Leaves[0] = "Item"
	v.Branches[0].Annotation = "Note" // adds the annotation column for all leaves
	v.Branches[0].Leaves[0] = strings.Repeat("L", 46)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "about 45 per leaf") {
		t.Fatalf("annotated leaf warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 2, 1)
	v.Branches[3].Label = strings.Repeat("B", 36)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].label") || !strings.Contains(got[0], "about 35") {
		t.Fatalf("branch warning = %v", got)
	}
	v.Branches[3].Label = "Branch"
	v.Branches[3].Annotation = strings.Repeat("N", 23)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].annotation") || !strings.Contains(got[0], "about 22") {
		t.Fatalf("annotation warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 4, 1)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "13 leaf rows") || !strings.Contains(got[0], "at most 12") {
		t.Fatalf("row-count warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 2)
	v.Branches[0].Annotation = strings.Repeat(" ", 150) // renderer omits blank notes
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("blank annotation should not warn: %v", got)
	}
}

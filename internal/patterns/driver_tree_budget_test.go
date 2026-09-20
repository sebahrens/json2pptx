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
		{7, false, 120}, {7, true, 120}, {8, false, 120}, {8, true, 101},
		{9, true, 101}, {10, false, 75}, {10, true, 51}, {14, true, 51}, {15, false, 0},
	} {
		if got := driverTreeLeafBudget(tc.total, tc.annotated); got != tc.want {
			t.Errorf("leafBudget(%d, %t) = %d, want %d", tc.total, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct {
		total, span, want int
		annotated         bool
	}{
		{9, 1, 60, true}, {10, 1, 38, false}, {10, 1, 32, true},
		{13, 2, 60, true}, {15, 4, 0, false},
	} {
		if got := driverTreeBranchBudget(tc.total, tc.span, tc.annotated); got != tc.want {
			t.Errorf("branchBudget(%d, %d, %t) = %d, want %d", tc.total, tc.span, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct{ total, span, want int }{
		{3, 1, 140}, {4, 1, 126}, {5, 1, 101}, {6, 1, 76},
		{8, 1, 51}, {10, 1, 27}, {8, 2, 126}, {10, 2, 101},
		{12, 2, 76}, {12, 3, 126}, {14, 3, 101}, {14, 4, 140},
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
	v.Branches[0].Leaves[0] = strings.Repeat("L", 76)
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[0].leaves[0]") || !strings.Contains(got[0], "about 75") {
		t.Fatalf("leaf warning = %v", got)
	}
	v.Branches[0].Leaves[0] = "Item"
	v.Branches[0].Annotation = "Note" // adds the annotation column for all leaves
	v.Branches[0].Leaves[0] = strings.Repeat("L", 52)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "about 51 per leaf") {
		t.Fatalf("annotated leaf warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 4, 1)
	v.Branches[3].Label = strings.Repeat("B", 39)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].label") || !strings.Contains(got[0], "about 38") {
		t.Fatalf("branch warning = %v", got)
	}
	v.Branches[3].Label = "Branch"
	v.Branches[3].Annotation = strings.Repeat("N", 28)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].annotation") || !strings.Contains(got[0], "about 27") {
		t.Fatalf("annotation warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 4, 3)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "15 leaf rows") || !strings.Contains(got[0], "at most 14") {
		t.Fatalf("row-count warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 2)
	v.Branches[0].Annotation = strings.Repeat(" ", 150) // renderer omits blank notes
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("blank annotation should not warn: %v", got)
	}
}

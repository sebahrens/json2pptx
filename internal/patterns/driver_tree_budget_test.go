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
		{4, true, 120}, {5, false, 120}, {5, true, 81}, {6, false, 61}, {6, true, 41},
		{9, false, 61}, {12, true, 41}, {13, false, 0},
	} {
		if got := driverTreeLeafBudget(tc.total, tc.annotated); got != tc.want {
			t.Errorf("leafBudget(%d, %t) = %d, want %d", tc.total, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct {
		total, span, want int
		annotated         bool
	}{
		{4, 1, 60, true}, {5, 1, 50, true}, {7, 1, 31, false}, {10, 1, 25, true},
		{8, 2, 60, false}, {10, 2, 50, true}, {11, 2, 31, false}, {12, 2, 25, true},
		{13, 4, 0, false},
	} {
		if got := driverTreeBranchBudget(tc.total, tc.span, tc.annotated); got != tc.want {
			t.Errorf("branchBudget(%d, %d, %t) = %d, want %d", tc.total, tc.span, tc.annotated, got, tc.want)
		}
	}
	for _, tc := range []struct{ total, span, want int }{
		{2, 1, 140}, {3, 1, 100}, {4, 1, 60}, {5, 1, 40},
		{6, 1, 20}, {12, 1, 20}, {4, 2, 140}, {5, 2, 120},
		{8, 2, 60}, {10, 2, 40}, {12, 2, 20}, {7, 3, 140},
		{10, 3, 80}, {12, 3, 60}, {9, 4, 140}, {10, 4, 120}, {12, 4, 100},
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
	v.Branches[0].Leaves[0] = strings.Repeat("L", 62)
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[0].leaves[0]") || !strings.Contains(got[0], "about 61") {
		t.Fatalf("leaf warning = %v", got)
	}
	v.Branches[0].Leaves[0] = "Item"
	v.Branches[0].Annotation = "Note" // adds the annotation column for all leaves
	v.Branches[0].Leaves[0] = strings.Repeat("L", 42)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "about 41 per leaf") {
		t.Fatalf("annotated leaf warning = %v", got)
	}

	v = budgetDriverTree(4, 4, 2, 1)
	v.Branches[3].Label = strings.Repeat("B", 32)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].label") || !strings.Contains(got[0], "about 31") {
		t.Fatalf("branch warning = %v", got)
	}
	v.Branches[3].Label = "Branch"
	v.Branches[3].Annotation = strings.Repeat("N", 21)
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "branches[3].annotation") || !strings.Contains(got[0], "about 20") {
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

package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func testRegistryWithAllPatterns() *patterns.Registry {
	return patterns.Default()
}

func TestSuggestAlternativeLayouts_NilBudgets(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	got := suggestAlternativeLayouts("card-grid", nil, reg)
	if got != nil {
		t.Errorf("expected nil for nil budgets, got %v", got)
	}
}

func TestSuggestAlternativeLayouts_NilRegistry(t *testing.T) {
	budgets := []cellBudgetEntry{{CellIndex: 0, ActualChars: 10, DensityPct: 30, Status: "underfilled"}}
	got := suggestAlternativeLayouts("card-grid", budgets, nil)
	if got != nil {
		t.Errorf("expected nil for nil registry, got %v", got)
	}
}

func TestSuggestAlternativeLayouts_AllEmpty(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 100, ActualChars: 0, DensityPct: 0, Status: "underfilled"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 100, ActualChars: 0, DensityPct: 0, Status: "underfilled"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	if got != nil {
		t.Errorf("expected nil when all cells are empty, got %v", got)
	}
}

func TestSuggestAlternativeLayouts_OptimalDensity(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 100, ActualChars: 80, DensityPct: 80, Status: "optimal"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 100, ActualChars: 90, DensityPct: 90, Status: "optimal"},
		{CellIndex: 2, Row: 1, Col: 0, MaxChars: 100, ActualChars: 70, DensityPct: 70, Status: "optimal"},
		{CellIndex: 3, Row: 1, Col: 1, MaxChars: 100, ActualChars: 85, DensityPct: 85, Status: "optimal"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	if got != nil {
		t.Errorf("expected nil for optimal density, got %v", got)
	}
}

func TestSuggestAlternativeLayouts_MixedDensity(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 100, ActualChars: 30, DensityPct: 30, Status: "underfilled"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 100, ActualChars: 90, DensityPct: 90, Status: "optimal"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	if got != nil {
		t.Errorf("expected nil for mixed density, got %v", got)
	}
}

func TestSuggestAlternativeLayouts_CardGrid_AllUnderfilled_3x2(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 200, ActualChars: 20, DensityPct: 10, Status: "underfilled"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 200, ActualChars: 30, DensityPct: 15, Status: "underfilled"},
		{CellIndex: 2, Row: 0, Col: 2, MaxChars: 200, ActualChars: 25, DensityPct: 12, Status: "underfilled"},
		{CellIndex: 3, Row: 1, Col: 0, MaxChars: 200, ActualChars: 15, DensityPct: 7, Status: "underfilled"},
		{CellIndex: 4, Row: 1, Col: 1, MaxChars: 200, ActualChars: 20, DensityPct: 10, Status: "underfilled"},
		{CellIndex: 5, Row: 1, Col: 2, MaxChars: 200, ActualChars: 18, DensityPct: 9, Status: "underfilled"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	if len(got) == 0 {
		t.Fatal("expected layout suggestions for all-underfilled card-grid 3x2")
	}

	// Should suggest a smaller grid (2x2)
	foundSmallerGrid := false
	foundKPI := false
	for _, s := range got {
		if s.Pattern == "card-grid" && s.Overrides != nil {
			foundSmallerGrid = true
			cols, ok := s.Overrides["columns"]
			if !ok {
				t.Error("smaller grid suggestion missing columns override")
			}
			if cols != 2 {
				t.Errorf("expected columns=2 for smaller grid, got %v", cols)
			}
		}
		if s.Pattern == "kpi-6up" {
			foundKPI = true
		}
	}
	if !foundSmallerGrid {
		t.Error("expected a card-grid suggestion with smaller dimensions")
	}
	if !foundKPI {
		t.Error("expected a kpi-6up suggestion for 6 underfilled cells")
	}
}

func TestSuggestAlternativeLayouts_CardGrid_AllOverflowing_2x2(t *testing.T) {
	reg := testRegistryWithAllPatterns()
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 100, ActualChars: 150, DensityPct: 150, Status: "overflow"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 100, ActualChars: 140, DensityPct: 140, Status: "overflow"},
		{CellIndex: 2, Row: 1, Col: 0, MaxChars: 100, ActualChars: 160, DensityPct: 160, Status: "overflow"},
		{CellIndex: 3, Row: 1, Col: 1, MaxChars: 100, ActualChars: 130, DensityPct: 130, Status: "overflow"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	if len(got) == 0 {
		t.Fatal("expected layout suggestions for all-overflowing card-grid 2x2")
	}

	// Should suggest a larger grid
	foundLarger := false
	for _, s := range got {
		if s.Pattern == "card-grid" && s.Overrides != nil {
			foundLarger = true
			cols, ok := s.Overrides["columns"]
			if !ok {
				t.Error("larger grid suggestion missing columns override")
			}
			if cols != 3 {
				t.Errorf("expected columns=3 for larger grid, got %v", cols)
			}
		}
	}
	if !foundLarger {
		t.Error("expected a card-grid suggestion with larger dimensions")
	}
}

func TestSuggestAlternativeLayouts_SuggestionsReferenceRegisteredPatterns(t *testing.T) {
	reg := testRegistryWithAllPatterns()

	// All underfilled card-grid — suggestions must reference real patterns
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 200, ActualChars: 20, DensityPct: 10, Status: "underfilled"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 200, ActualChars: 30, DensityPct: 15, Status: "underfilled"},
		{CellIndex: 2, Row: 1, Col: 0, MaxChars: 200, ActualChars: 15, DensityPct: 7, Status: "underfilled"},
		{CellIndex: 3, Row: 1, Col: 1, MaxChars: 200, ActualChars: 25, DensityPct: 12, Status: "underfilled"},
	}
	got := suggestAlternativeLayouts("card-grid", budgets, reg)
	for _, s := range got {
		if s.Pattern == "card-grid" {
			continue // self-reference with different overrides is valid
		}
		if _, ok := reg.Get(s.Pattern); !ok {
			t.Errorf("suggestion references unregistered pattern %q", s.Pattern)
		}
	}
}

func TestSuggestAlternativeLayouts_NonCardGrid_DensityClassSibling(t *testing.T) {
	reg := testRegistryWithAllPatterns()

	// Use a pattern that has pairs_with containing patterns with different density
	// kpi-3up (low density) pairs with process-flow (medium density)
	budgets := []cellBudgetEntry{
		{CellIndex: 0, Row: 0, Col: 0, MaxChars: 50, ActualChars: 80, DensityPct: 160, Status: "overflow"},
		{CellIndex: 1, Row: 0, Col: 1, MaxChars: 50, ActualChars: 70, DensityPct: 140, Status: "overflow"},
		{CellIndex: 2, Row: 0, Col: 2, MaxChars: 50, ActualChars: 75, DensityPct: 150, Status: "overflow"},
	}
	got := suggestAlternativeLayouts("kpi-3up", budgets, reg)
	// kpi-3up is "low" density, all overflowing → should suggest "medium" density sibling
	for _, s := range got {
		if _, ok := reg.Get(s.Pattern); !ok {
			t.Errorf("suggestion references unregistered pattern %q", s.Pattern)
		}
	}
}

func TestKpiNameForCount(t *testing.T) {
	reg := testRegistryWithAllPatterns()

	tests := []struct {
		count int
		want  string
	}{
		{1, ""},
		{2, "kpi-2up"},
		{3, "kpi-3up"},
		{4, "kpi-4up"},
		{5, "kpi-5up"},
		{6, "kpi-6up"},
		{7, ""},
	}
	for _, tt := range tests {
		got := kpiNameForCount(tt.count, reg)
		if got != tt.want {
			t.Errorf("kpiNameForCount(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestLowerHigherDensity(t *testing.T) {
	tests := []struct {
		fn   func(string) string
		in   string
		want string
	}{
		{lowerDensity, "high", "medium"},
		{lowerDensity, "medium", "low"},
		{lowerDensity, "low", ""},
		{higherDensity, "low", "medium"},
		{higherDensity, "medium", "high"},
		{higherDensity, "high", ""},
	}
	for _, tt := range tests {
		got := tt.fn(tt.in)
		if got != tt.want {
			t.Errorf("density(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

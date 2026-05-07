package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestComputeTextBudgetGuide_CardGrid(t *testing.T) {
	pat, ok := patterns.Default().Get("card-grid")
	if !ok {
		t.Fatal("card-grid pattern not registered")
	}

	guide := computeTextBudgetGuide(pat)
	if guide == nil {
		t.Fatal("expected non-nil text_budget_guide for card-grid")
	}

	if guide.TargetDensity.MinPct != 60 {
		t.Errorf("expected min_pct=60, got %d", guide.TargetDensity.MinPct)
	}
	if guide.TargetDensity.IdealPct != 85 {
		t.Errorf("expected ideal_pct=85, got %d", guide.TargetDensity.IdealPct)
	}
	if guide.TargetDensity.MaxPct != 110 {
		t.Errorf("expected max_pct=110, got %d", guide.TargetDensity.MaxPct)
	}

	if len(guide.Configurations) < 3 {
		t.Fatalf("expected at least 3 configurations, got %d", len(guide.Configurations))
	}

	// Verify all configurations have positive budgets
	for _, cfg := range guide.Configurations {
		if cfg.BodyMaxChars <= 0 {
			t.Errorf("config %d×%d: expected positive body_max_chars, got %d", cfg.Columns, cfg.Rows, cfg.BodyMaxChars)
		}
		if cfg.HeaderMaxChars <= 0 {
			t.Errorf("config %d×%d: expected positive header_max_chars, got %d", cfg.Columns, cfg.Rows, cfg.HeaderMaxChars)
		}
		// Body (12pt) should fit more than header (16pt)
		if cfg.BodyMaxChars <= cfg.HeaderMaxChars {
			t.Errorf("config %d×%d: body (%d) should exceed header (%d)", cfg.Columns, cfg.Rows, cfg.BodyMaxChars, cfg.HeaderMaxChars)
		}
	}

	// More columns = smaller cells = fewer chars
	if guide.Configurations[1].BodyMaxChars >= guide.Configurations[0].BodyMaxChars {
		t.Errorf("3×2 body (%d) should be less than 2×2 body (%d)",
			guide.Configurations[1].BodyMaxChars, guide.Configurations[0].BodyMaxChars)
	}
}

func TestComputeTextBudgetGuide_NonGridPatterns(t *testing.T) {
	nonGrid := []string{"pull-quote", "stat-hero", "comparison-2col"}
	for _, name := range nonGrid {
		pat, ok := patterns.Default().Get(name)
		if !ok {
			t.Errorf("pattern %q not registered", name)
			continue
		}
		guide := computeTextBudgetGuide(pat)
		if guide != nil {
			t.Errorf("expected nil text_budget_guide for %s, got %+v", name, guide)
		}
	}
}

func TestComputeTextBudgetGuide_AllPatterns(t *testing.T) {
	// Verify no panics for any registered pattern
	for _, pat := range patterns.Default().List() {
		guide := computeTextBudgetGuide(pat)
		_, isBCP := pat.(patterns.BudgetConfigProvider)
		if isBCP && guide == nil {
			t.Errorf("pattern %s implements BudgetConfigProvider but returned nil guide", pat.Name())
		}
		if !isBCP && guide != nil {
			t.Errorf("pattern %s does not implement BudgetConfigProvider but returned non-nil guide", pat.Name())
		}
	}
}

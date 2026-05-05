package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestBuildDeckPlan_Basic12Slides(t *testing.T) {
	reg := patterns.Default()
	result := buildDeckPlan(reg, "Pitch our Series B for an AI infra company", 12, "investors", nil)

	if len(result.Slides) != 12 {
		t.Fatalf("expected 12 slides, got %d", len(result.Slides))
	}

	// Verify slide indices are sequential.
	for i, s := range result.Slides {
		if s.SlideIndex != i {
			t.Errorf("slide %d has index %d", i, s.SlideIndex)
		}
	}

	// Verify opening and closing roles.
	if result.Slides[0].NarrativeRole != "opening" {
		t.Errorf("first slide should be opening, got %q", result.Slides[0].NarrativeRole)
	}
	if result.Slides[11].NarrativeRole != "closing" {
		t.Errorf("last slide should be closing, got %q", result.Slides[11].NarrativeRole)
	}

	// Verify no pattern runs exceed 2 (rhythm rule).
	if result.RhythmCheck.LongestPatternRun > 2 {
		t.Errorf("longest pattern run should be <=2, got %d", result.RhythmCheck.LongestPatternRun)
		for _, s := range result.Slides {
			t.Logf("  slide %d: %s (%s) — %s", s.SlideIndex, s.RecommendedPattern, s.NarrativeRole, s.Rationale)
		}
	}

	// Verify at least one emphasis pattern.
	if !result.RhythmCheck.HasEmphasis {
		t.Error("expected at least one emphasis pattern (stat-hero or pull-quote)")
	}

	// Verify all slides have patterns.
	for i, s := range result.Slides {
		if s.RecommendedPattern == "" {
			t.Errorf("slide %d has empty recommended_pattern", i)
		}
		if s.ContentSeed == "" {
			t.Errorf("slide %d has empty content_seed", i)
		}
		if s.Rationale == "" {
			t.Errorf("slide %d has empty rationale", i)
		}
	}

	// Verify pattern variety (should use more than 3 unique patterns for a 12-slide deck).
	if result.RhythmCheck.PatternVariety < 3 {
		t.Errorf("expected at least 3 unique patterns, got %d", result.RhythmCheck.PatternVariety)
	}
}

func TestBuildDeckPlan_MustInclude(t *testing.T) {
	reg := patterns.Default()
	result := buildDeckPlan(reg, "Business model overview", 8, "", []string{"bmc-canvas", "kpi-3up"})

	// Verify must_include patterns appear in the plan.
	found := map[string]bool{}
	for _, s := range result.Slides {
		if s.RecommendedPattern == "bmc-canvas" || s.RecommendedPattern == "kpi-3up" {
			found[s.RecommendedPattern] = true
		}
	}

	if !found["bmc-canvas"] {
		t.Error("must_include pattern bmc-canvas not found in plan")
	}
	if !found["kpi-3up"] {
		t.Error("must_include pattern kpi-3up not found in plan")
	}
}

func TestBuildDeckPlan_MinBudget(t *testing.T) {
	reg := patterns.Default()
	result := buildDeckPlan(reg, "Quick update", 3, "", nil)

	if len(result.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(result.Slides))
	}
	if result.Slides[0].NarrativeRole != "opening" {
		t.Errorf("first slide should be opening, got %q", result.Slides[0].NarrativeRole)
	}
	if result.Slides[2].NarrativeRole != "closing" {
		t.Errorf("last slide should be closing, got %q", result.Slides[2].NarrativeRole)
	}
}

func TestDistributeRoles(t *testing.T) {
	tests := []struct {
		budget     int
		wantFirst  string
		wantLast   string
	}{
		{3, "opening", "closing"},
		{5, "opening", "closing"},
		{10, "opening", "closing"},
		{20, "opening", "closing"},
	}

	for _, tt := range tests {
		roles := distributeRoles(tt.budget)
		if len(roles) != tt.budget {
			t.Errorf("budget %d: got %d roles", tt.budget, len(roles))
		}
		if roles[0] != tt.wantFirst {
			t.Errorf("budget %d: first role = %q, want %q", tt.budget, roles[0], tt.wantFirst)
		}
		if roles[len(roles)-1] != tt.wantLast {
			t.Errorf("budget %d: last role = %q, want %q", tt.budget, roles[len(roles)-1], tt.wantLast)
		}
		// All roles should be non-empty.
		for i, r := range roles {
			if r == "" {
				t.Errorf("budget %d: role at index %d is empty", tt.budget, i)
			}
		}
	}
}

func TestEnforceRhythm_BreaksLongRuns(t *testing.T) {
	reg := patterns.Default()

	// Create slides with a 5-slide run of the same pattern.
	slides := make([]planSlide, 7)
	for i := range slides {
		slides[i] = planSlide{
			SlideIndex:         i,
			NarrativeRole:      "evidence",
			RecommendedPattern: "card-grid",
			ContentSeed:        "test",
			Rationale:          "test",
		}
	}
	slides[0].NarrativeRole = "opening"
	slides[6].NarrativeRole = "closing"

	result := enforceRhythm(reg, slides, "test")

	// Count longest run.
	longestRun := 1
	currentRun := 1
	for i := 1; i < len(result); i++ {
		if result[i].RecommendedPattern == result[i-1].RecommendedPattern {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
		} else {
			currentRun = 1
		}
	}

	if longestRun > 2 {
		t.Errorf("after enforceRhythm, longest run should be <=2, got %d", longestRun)
		for _, s := range result {
			t.Logf("  slide %d: %s (%s)", s.SlideIndex, s.RecommendedPattern, s.NarrativeRole)
		}
	}
}

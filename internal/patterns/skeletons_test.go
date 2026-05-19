package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkeletonForPattern_BasicShape(t *testing.T) {
	reg := Default()
	tests := []struct {
		name string
		role string
	}{
		{"comparison-2col", "comparison"},
		{"kpi-3up", "evidence"},
		{"stat-hero", "emphasis"},
		{"bmc-canvas", "framework"},
		{"agenda", "framework"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := SkeletonForPattern(reg, tt.name, tt.role)
			if err != nil {
				t.Fatalf("SkeletonForPattern(%q): %v", tt.name, err)
			}
			if len(raw) == 0 {
				t.Fatalf("SkeletonForPattern(%q): empty result", tt.name)
			}

			var slide map[string]any
			if err := json.Unmarshal(raw, &slide); err != nil {
				t.Fatalf("skeleton is not valid JSON: %v", err)
			}

			// Must have layout_id, content, pattern.
			if _, ok := slide["layout_id"]; !ok {
				t.Errorf("skeleton missing layout_id")
			}
			content, ok := slide["content"].([]any)
			if !ok || len(content) == 0 {
				t.Fatalf("skeleton missing content array")
			}
			pattern, ok := slide["pattern"].(map[string]any)
			if !ok {
				t.Fatalf("skeleton missing pattern object")
			}
			if pattern["name"] != tt.name {
				t.Errorf("pattern.name = %v, want %q", pattern["name"], tt.name)
			}
			if _, ok := pattern["values"]; !ok {
				t.Errorf("skeleton pattern missing values")
			}

			// At least one __FILL__ token must be present somewhere.
			if !strings.Contains(string(raw), FillPlaceholder) {
				t.Errorf("skeleton has no %s placeholder: %s", FillPlaceholder, string(raw))
			}
		})
	}
}

func TestSkeletonForPattern_TitlePlaceholderFilled(t *testing.T) {
	reg := Default()
	raw, err := SkeletonForPattern(reg, "comparison-2col", "comparison")
	if err != nil {
		t.Fatal(err)
	}
	var slide struct {
		Content []struct {
			PlaceholderID string `json:"placeholder_id"`
			Type          string `json:"type"`
			TextValue     string `json:"text_value"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &slide); err != nil {
		t.Fatal(err)
	}
	if len(slide.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(slide.Content))
	}
	if slide.Content[0].PlaceholderID != "title" {
		t.Errorf("expected title placeholder, got %q", slide.Content[0].PlaceholderID)
	}
	if slide.Content[0].TextValue != FillPlaceholder {
		t.Errorf("expected text_value=%q, got %q", FillPlaceholder, slide.Content[0].TextValue)
	}
}

func TestSkeletonForPattern_StringLeavesReplaced(t *testing.T) {
	reg := Default()
	raw, err := SkeletonForPattern(reg, "comparison-2col", "comparison")
	if err != nil {
		t.Fatal(err)
	}
	// comparison-2col exemplar has Headers=["Pros","Cons"] and Rows with
	// Left/Right strings — none of those literals should leak into the skeleton.
	for _, leak := range []string{"Pros", "Cons", "Fast", "Expensive", "Reliable", "Complex"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("exemplar literal %q leaked into skeleton: %s", leak, string(raw))
		}
	}
}

func TestSkeletonForPattern_PreservesNumericLeaves(t *testing.T) {
	reg := Default()
	// card-grid exemplar carries Columns=3, Rows=2 — numeric structural
	// defaults must survive the placeholder pass so the skeleton remains
	// structurally valid.
	raw, err := SkeletonForPattern(reg, "card-grid", "evidence")
	if err != nil {
		t.Fatal(err)
	}
	var slide struct {
		Pattern struct {
			Values map[string]any `json:"values"`
		} `json:"pattern"`
	}
	if err := json.Unmarshal(raw, &slide); err != nil {
		t.Fatal(err)
	}
	if cols, ok := slide.Pattern.Values["columns"].(float64); !ok || cols != 3 {
		t.Errorf("card-grid skeleton lost columns=3, got %v", slide.Pattern.Values["columns"])
	}
	if rows, ok := slide.Pattern.Values["rows"].(float64); !ok || rows != 2 {
		t.Errorf("card-grid skeleton lost rows=2, got %v", slide.Pattern.Values["rows"])
	}
}

func TestSkeletonForPattern_LayoutIDByRole(t *testing.T) {
	reg := Default()
	cases := map[string]string{
		"opening":    "title",
		"closing":    "closing",
		"framework":  "section",
		"evidence":   "blank",
		"comparison": "blank",
		"emphasis":   "blank",
		"":           "blank",
	}
	for role, want := range cases {
		raw, err := SkeletonForPattern(reg, "stat-hero", role)
		if err != nil {
			t.Fatalf("role=%q: %v", role, err)
		}
		var slide struct {
			LayoutID string `json:"layout_id"`
		}
		if err := json.Unmarshal(raw, &slide); err != nil {
			t.Fatal(err)
		}
		if slide.LayoutID != want {
			t.Errorf("role=%q: layout_id=%q, want %q", role, slide.LayoutID, want)
		}
	}
}

func TestSkeletonForPattern_UnknownPattern(t *testing.T) {
	reg := Default()
	if _, err := SkeletonForPattern(reg, "no-such-pattern", "evidence"); err == nil {
		t.Error("expected error for unknown pattern, got nil")
	}
}

func TestSkeletonForPattern_NilRegistry(t *testing.T) {
	if _, err := SkeletonForPattern(nil, "stat-hero", "evidence"); err == nil {
		t.Error("expected error for nil registry, got nil")
	}
}

func TestSkeletonForPattern_AllRegisteredPatterns(t *testing.T) {
	reg := Default()
	// Every registered pattern that implements Exemplar must produce a usable
	// skeleton. Patterns without an Exemplar implementation are allowed to
	// fail — they're flagged so we know which ones need exemplars added.
	for _, pat := range reg.List() {
		t.Run(pat.Name(), func(t *testing.T) {
			if _, ok := pat.(Exemplar); !ok {
				t.Skipf("pattern %q has no Exemplar", pat.Name())
			}
			raw, err := SkeletonForPattern(reg, pat.Name(), "evidence")
			if err != nil {
				t.Fatalf("SkeletonForPattern(%q): %v", pat.Name(), err)
			}
			var slide map[string]any
			if err := json.Unmarshal(raw, &slide); err != nil {
				t.Errorf("invalid JSON: %v", err)
			}
		})
	}
}

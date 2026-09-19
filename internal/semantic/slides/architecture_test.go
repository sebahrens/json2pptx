package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// archInput builds a compiler Input for an architecture payload.
func archInput(body map[string]any) Input {
	in := Input{Body: body}
	if t, ok := body["title"].(string); ok {
		in.Title = t
	}
	if t, ok := body["takeaway"].(string); ok {
		in.Takeaway = t
	}
	return in
}

func archValues(t *testing.T, raw json.RawMessage) archStackValues {
	t.Helper()
	var v archStackValues
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode arch-stack values: %v", err)
	}
	return v
}

// TestCompileArchitectureToPattern is the go-slide-creator-162os acceptance
// test: the payload an author writes compiles onto arch-stack, with a tier's
// items joined into its detail line and the rails carried across.
func TestCompileArchitectureToPattern(t *testing.T) {
	body := map[string]any{
		"title": "Platform architecture",
		"tiers": []any{
			map[string]any{"label": "Experience", "items": []any{"Web console", "Mobile approvals", "Partner portal"}},
			map[string]any{"label": "Services", "description": "Orders, pricing, fulfilment, identity"},
			map[string]any{"label": "Data", "items": []any{"Event stream", "Warehouse"}},
			map[string]any{"label": "Platform", "description": "Kubernetes, observability, secrets"},
		},
		"rails":    []any{"Security & compliance", "Cost governance"},
		"takeaway": "Four tiers ship independently.",
	}
	slide, links, err := CompileArchitecture(archInput(body))
	if err != nil {
		t.Fatalf("CompileArchitecture: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "arch-stack" {
		t.Fatalf("pattern = %+v, want arch-stack", slide.Pattern)
	}
	values := archValues(t, slide.Pattern.Values)
	if len(values.Tiers) != 4 {
		t.Fatalf("got %d tiers, want 4", len(values.Tiers))
	}
	if values.Tiers[0].Description != "Web console, mobile approvals, partner portal" &&
		values.Tiers[0].Description != "Web console, Mobile approvals, Partner portal" {
		t.Errorf("tier 1 detail = %q, want the items joined", values.Tiers[0].Description)
	}
	if values.Tiers[1].Description != "Orders, pricing, fulfilment, identity" {
		t.Errorf("tier 2 detail = %q, want the explicit description", values.Tiers[1].Description)
	}
	if len(values.SideRails) != 2 || values.SideRails[0] != "Security & compliance" {
		t.Errorf("side_rails = %v, want the two rails", values.SideRails)
	}
	if slide.Takeaway == "" {
		t.Error("takeaway did not reach the slide")
	}
	// The source map has to point a diagnostic back at the field the author wrote.
	var sawTiers, sawRails bool
	for _, l := range links {
		if strings.HasSuffix(l.SemanticPath, ".tiers") {
			sawTiers = true
		}
		if strings.HasSuffix(l.SemanticPath, ".rails") {
			sawRails = true
		}
	}
	if !sawTiers || !sawRails {
		t.Errorf("source links missing tiers=%v rails=%v: %+v", sawTiers, sawRails, links)
	}
}

// TestCompileArchitectureAliases pins the spellings an author might reach for.
func TestCompileArchitectureAliases(t *testing.T) {
	body := map[string]any{
		"title": "Stack",
		"layers": []any{
			map[string]any{"name": "Front", "components": []any{"Console"}},
			map[string]any{"title": "Middle", "detail": "Services"},
			map[string]any{"tier": "Back", "summary": "Storage"},
		},
		"side_rails": []any{"Security"},
	}
	slide, _, err := CompileArchitecture(archInput(body))
	if err != nil {
		t.Fatalf("CompileArchitecture: %v", err)
	}
	if slide.Pattern == nil {
		t.Fatal("aliases did not compile to the pattern")
	}
	values := archValues(t, slide.Pattern.Values)
	got := []string{values.Tiers[0].Label, values.Tiers[1].Label, values.Tiers[2].Label}
	want := []string{"Front", "Middle", "Back"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tier %d label = %q, want %q", i, got[i], want[i])
		}
	}
	if len(values.SideRails) != 1 {
		t.Errorf("side_rails = %v, want one rail", values.SideRails)
	}
}

// TestArchitectureDegradesRatherThanTruncates pins the budget rule: a payload
// the pattern cannot take renders as bullets with every word intact, rather
// than being clipped into a maxLength or blocking the deck.
func TestArchitectureDegradesRatherThanTruncates(t *testing.T) {
	long := strings.Repeat("x", archStackDescMax+10)
	cases := []struct {
		name string
		body map[string]any
		keep string
	}{
		{
			name: "too few tiers",
			body: map[string]any{"tiers": []any{"Only", "Two"}},
			keep: "Only",
		},
		{
			name: "too many tiers",
			body: map[string]any{"tiers": []any{"a", "b", "c", "d", "e", "f", "g"}},
			keep: "g",
		},
		{
			name: "a detail line over the budget",
			body: map[string]any{"tiers": []any{
				map[string]any{"label": "Experience", "description": long},
				"Services", "Platform",
			}},
			keep: long,
		},
		{
			name: "more rails than the pattern draws",
			body: map[string]any{
				"tiers": []any{"Experience", "Services", "Platform"},
				"rails": []any{"One", "Two", "Three", "Four"},
			},
			keep: "Four",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ArchitecturePatternFeasible(tc.body) {
				t.Fatal("payload should not be pattern-feasible")
			}
			if over := ArchitectureOverBudget(tc.body); over == "" {
				t.Error("no over-budget explanation for a payload that does not fit")
			}
			slide, _, err := CompileArchitecture(archInput(tc.body))
			if err != nil {
				t.Fatalf("CompileArchitecture: %v", err)
			}
			if slide.Pattern != nil {
				t.Fatalf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
			}
			encoded, err := json.Marshal(slide)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), tc.keep) {
				t.Errorf("the fallback dropped content that should survive: %q not in %s", tc.keep, encoded)
			}
		})
	}
}

// TestArchitectureFeasibilityMatchesCompile pins explain/compile parity at the
// boundary: what ArchitecturePatternFeasible says is what CompileArchitecture
// emits, for every count around the pattern's range.
func TestArchitectureFeasibilityMatchesCompile(t *testing.T) {
	for n := 0; n <= 8; n++ {
		tiers := make([]any, 0, n)
		for i := 0; i < n; i++ {
			tiers = append(tiers, map[string]any{"label": "Tier"})
		}
		body := map[string]any{"tiers": tiers}
		feasible := ArchitecturePatternFeasible(body)
		slide, _, err := CompileArchitecture(archInput(body))
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if got := slide.Pattern != nil; got != feasible {
			t.Errorf("n=%d: feasible=%v but compiled pattern=%v", n, feasible, got)
		}
		if want := n >= archStackMinTiers && n <= archStackMaxTiers; feasible != want {
			t.Errorf("n=%d: feasible=%v, want %v", n, feasible, want)
		}
	}
}

// TestArchitectureDropsUnusableTiers pins that a tier with no label is dropped
// rather than rendered blank, and that the count the validator reports reflects
// what will actually be drawn.
func TestArchitectureDropsUnusableTiers(t *testing.T) {
	body := map[string]any{"tiers": []any{
		map[string]any{"label": "Experience"},
		map[string]any{"description": "no label here"},
		"   ",
		map[string]any{"label": "Platform"},
	}}
	if n := UsableTierCount(body); n != 2 {
		t.Errorf("UsableTierCount = %d, want 2", n)
	}
}

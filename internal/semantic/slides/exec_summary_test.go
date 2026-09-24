package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-ku6t. CompileExecutiveSummary always produced a bullet list,
// so the exec-summary pattern — numbered conclusions with supporting sentences
// and a bottom-line bar, shipped in round 1 — could not be reached from the
// DeckSpec path that get_started recommends. The deck's most important slide was
// the flattest one on it.

func execValues(t *testing.T, body map[string]any) (execSummaryValues, bool) {
	t.Helper()
	slide, _, err := CompileExecutiveSummary(Input{Title: "Summary", Body: body})
	if err != nil {
		t.Fatalf("CompileExecutiveSummary: %v", err)
	}
	if slide.Pattern == nil {
		return execSummaryValues{}, false
	}
	if slide.Pattern.Name != "exec-summary" {
		t.Fatalf("pattern = %q, want exec-summary", slide.Pattern.Name)
	}
	var values execSummaryValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatalf("unmarshal pattern values: %v", err)
	}
	return values, true
}

func TestExecutiveSummaryCompilesToPattern(t *testing.T) {
	values, ok := execValues(t, map[string]any{
		"points": []any{
			map[string]any{"lead": "Growth is ahead of plan.", "support": "Revenue grew 41% to $48M."},
			map[string]any{"lead": "Enterprise carries the mix.", "support": "55% of new bookings."},
			map[string]any{"lead": "SMB retention is the risk.", "support": "Churn rose to 3.1%."},
		},
		"bottom_line": "Fund an SMB retention pod in Q3.",
	})
	if !ok {
		t.Fatal("three points did not reach the exec-summary pattern")
	}
	if len(values.Points) != 3 {
		t.Fatalf("points = %d, want 3", len(values.Points))
	}
	if values.Points[0].Lead != "Growth is ahead of plan." || values.Points[0].Support != "Revenue grew 41% to $48M." {
		t.Errorf("point 0 = %+v", values.Points[0])
	}
	if values.BottomLine != "Fund an SMB retention pod in Q3." {
		t.Errorf("bottom_line = %q", values.BottomLine)
	}
}

func TestExecutiveSummaryUsesOneConclusionBand(t *testing.T) {
	base := map[string]any{
		"points":      []any{"Revenue rose", "Churn fell", "Cycle shortened"},
		"bottom_line": "Fund the retention pod in Q3.",
		"takeaway":    "Begin hiring next month.",
	}
	slide, _, err := CompileExecutiveSummary(Input{Body: base, Takeaway: base["takeaway"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Takeaway != "" {
		t.Fatalf("duplicate takeaway band = %q", slide.Takeaway)
	}
	var values execSummaryValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatal(err)
	}
	if want := "Fund the retention pod in Q3. — Begin hiring next month."; values.BottomLine != want {
		t.Errorf("bottom_line = %q, want %q", values.BottomLine, want)
	}
	if !ExecSummaryPatternFeasible(base) {
		t.Fatal("feasibility disagrees with compiler")
	}
}

func TestExecutiveSummaryOverfullConclusionDegradesWithoutDroppingText(t *testing.T) {
	body := map[string]any{
		"points":      []any{"Revenue rose", "Churn fell", "Cycle shortened"},
		"bottom_line": strings.Repeat("Action ", 15),
		"takeaway":    strings.Repeat("Implication ", 10),
	}
	if ExecSummaryPatternFeasible(body) {
		t.Fatal("overfull conclusion should not fit the pattern")
	}
	slide, _, err := CompileExecutiveSummary(Input{Body: body, Takeaway: body["takeaway"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil || slide.Takeaway != body["takeaway"] {
		t.Fatalf("fallback lost takeaway: pattern=%v takeaway=%q", slide.Pattern, slide.Takeaway)
	}
	if len(slide.Content) == 0 || slide.Content[0].BulletsValue == nil {
		t.Fatalf("fallback lost bottom line: %+v", slide.Content)
	}
	bullets := *slide.Content[0].BulletsValue
	if bullets[len(bullets)-1] != strings.TrimSpace(body["bottom_line"].(string)) {
		t.Errorf("last bullet = %q, want bottom line", bullets[len(bullets)-1])
	}
}

// A string point is the conclusion with no evidence line — the shape every
// existing spec uses, which must keep working and still get the visual.
func TestExecutiveSummaryStringPointsBecomeLeads(t *testing.T) {
	values, ok := execValues(t, map[string]any{
		"points": []any{"Revenue is up", "Churn is up", "Cycle is shorter"},
	})
	if !ok {
		t.Fatal("string points did not reach the exec-summary pattern")
	}
	for i, want := range []string{"Revenue is up", "Churn is up", "Cycle is shorter"} {
		if values.Points[i].Lead != want {
			t.Errorf("points[%d].lead = %q, want %q", i, values.Points[i].Lead, want)
		}
		if values.Points[i].Support != "" {
			t.Errorf("points[%d].support = %q, want empty", i, values.Points[i].Support)
		}
	}
}

func TestExecutiveSummaryAcceptsFieldAliases(t *testing.T) {
	values, ok := execValues(t, map[string]any{
		"takeaways": []any{
			map[string]any{"statement": "One", "evidence": "Because A"},
			map[string]any{"title": "Two", "description": "Because B"},
			map[string]any{"point": "Three", "detail": "Because C"},
		},
	})
	if !ok {
		t.Fatal("aliased points did not reach the exec-summary pattern")
	}
	for i, want := range []struct{ lead, support string }{
		{"One", "Because A"}, {"Two", "Because B"}, {"Three", "Because C"},
	} {
		if values.Points[i].Lead != want.lead || values.Points[i].Support != want.support {
			t.Errorf("points[%d] = %+v, want %+v", i, values.Points[i], want)
		}
	}
}

// Degradation is bounded by the pattern's own contract, and it keeps every word
// the author wrote — including the bottom line, which has no bar to ride in the
// fallback and used to have nowhere to go at all.
func TestExecutiveSummaryDegradesOutsideThePatternBounds(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want []string
	}{
		{
			name: "two points is below the pattern minimum",
			body: map[string]any{"points": []any{"One", "Two"}, "bottom_line": "Do the thing"},
			want: []string{"One", "Two", "Do the thing"},
		},
		{
			name: "six points is above the pattern maximum",
			body: map[string]any{"points": []any{"a", "b", "c", "d", "e", "f"}},
			want: []string{"a", "b", "c", "d", "e", "f"},
		},
		{
			name: "a lead past the 90-char budget",
			body: map[string]any{"points": []any{
				strings.Repeat("x", 120), "Two", "Three",
			}},
			want: []string{strings.Repeat("x", 120), "Two", "Three"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide, _, err := CompileExecutiveSummary(Input{Title: "Summary", Body: tt.body})
			if err != nil {
				t.Fatalf("CompileExecutiveSummary: %v", err)
			}
			if slide.Pattern != nil {
				t.Fatalf("expected the bullet fallback, got pattern %q", slide.Pattern.Name)
			}
			if len(slide.Content) < 2 || slide.Content[1].BulletsValue == nil {
				t.Fatalf("no bullets block: %+v", slide.Content)
			}
			got := *slide.Content[1].BulletsValue
			if len(got) != len(tt.want) {
				t.Fatalf("bullets = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("bullets[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// An object point in the fallback must read as a sentence, not as Go map syntax.
func TestExecutiveSummaryFallbackRendersObjectPoints(t *testing.T) {
	slide, _, err := CompileExecutiveSummary(Input{Title: "Summary", Body: map[string]any{
		"points": []any{
			map[string]any{"lead": "One", "support": "Because A"},
			map[string]any{"lead": "Two"},
		},
	}})
	if err != nil {
		t.Fatalf("CompileExecutiveSummary: %v", err)
	}
	if slide.Pattern != nil {
		t.Fatalf("two points should degrade, got pattern %q", slide.Pattern.Name)
	}
	got := *slide.Content[1].BulletsValue
	want := []string{"One — Because A", "Two"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bullets[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ExecSummaryPatternFeasible is what explain and validate consult, so it has to
// agree with the compiler on every input — an agent told "exec-summary" and
// handed bullets is worse than being told nothing.
func TestExecSummaryPatternFeasibleMatchesCompile(t *testing.T) {
	bodies := []map[string]any{
		{"points": []any{"a", "b", "c"}},
		{"points": []any{"a", "b"}},
		{"points": []any{"a", "b", "c", "d", "e", "f"}},
		{"takeaways": []any{"a", "b", "c", "d"}},
		{"points": []any{strings.Repeat("x", 120), "b", "c"}},
		{"points": []any{map[string]any{"lead": "a", "support": strings.Repeat("y", 300)}, "b", "c"}},
		{},
		{"points": []any{}},
	}
	for _, body := range bodies {
		slide, _, err := CompileExecutiveSummary(Input{Title: "T", Body: body})
		if err != nil {
			t.Fatalf("compile %v: %v", body, err)
		}
		compiled := slide.Pattern != nil
		if got := ExecSummaryPatternFeasible(body); got != compiled {
			t.Errorf("ExecSummaryPatternFeasible(%v) = %v, compile emitted pattern = %v", body, got, compiled)
		}
	}
}

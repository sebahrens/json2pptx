package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-6o1r. The DeckSpec path compiled to 5 of 43 patterns, and the
// options × criteria evaluation matrix — the slide a recommendation is actually
// argued on — was not one of them. Three reviewers independently reached for it
// and had to drop to kind raw_json2pptx with a hand-written pattern block.

func compiledMatrix(t *testing.T, body map[string]any) (*optionMatrixValues, bool) {
	t.Helper()
	slide, _, err := CompileOptionMatrix(Input{Title: "Options", Body: body})
	if err != nil {
		t.Fatalf("CompileOptionMatrix: %v", err)
	}
	if slide.Pattern == nil {
		return nil, false
	}
	if slide.Pattern.Name != "table-highlight" {
		t.Fatalf("pattern = %q, want table-highlight", slide.Pattern.Name)
	}
	var values optionMatrixValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatalf("unmarshal pattern values: %v", err)
	}
	return &values, true
}

func harveyMatrix(overlay map[string]any) map[string]any {
	body := map[string]any{
		"criteria": []any{"Capex", "Payback", "Risk"},
		"options": []any{
			map[string]any{"name": "Automate", "detail": "Three largest hubs", "scores": []any{1, 1, 3}},
			map[string]any{"name": "Consolidate", "scores": []any{2, 4, 3}},
			map[string]any{"name": "Partner", "scores": []any{4, 4, 1}},
		},
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

func TestOptionMatrixCompilesToTableHighlight(t *testing.T) {
	values, ok := compiledMatrix(t, harveyMatrix(nil))
	if !ok {
		t.Fatal("a 3×3 matrix did not reach table-highlight")
	}
	if len(values.Criteria) != 3 || values.Criteria[1].Label != "Payback" {
		t.Errorf("criteria = %+v", values.Criteria)
	}
	if len(values.Options) != 3 || values.Options[0].Name != "Automate" || values.Options[0].Detail != "Three largest hubs" {
		t.Errorf("options = %+v", values.Options)
	}
	// Scores keep the author's own literal so the pattern reads them on its scale.
	if got := string(values.Options[1].Scores[1]); got != "4" {
		t.Errorf("scores[1][1] = %s, want 4", got)
	}
}

func TestOptionMatrixLongDetailKeepsSparseMatrix(t *testing.T) {
	body := harveyMatrix(nil)
	options := body["options"].([]any)
	options[0].(map[string]any)["detail"] = strings.Repeat("D", 62)
	values, ok := compiledMatrix(t, body)
	if !ok || values.Options[0].Detail != strings.Repeat("D", 62) {
		t.Fatalf("62-character descriptor was lost or degraded: %+v, %v", values, ok)
	}
	if _, _, _, _, exceeded := OptionMatrixDetailBudgetIssue(body); exceeded {
		t.Error("sparse 3×3 matrix should not exceed its detail budget")
	}

	options[0].(map[string]any)["detail"] = strings.Repeat("D", 81)
	if _, ok := compiledMatrix(t, body); ok {
		t.Error("81-character descriptor should exceed the sparse matrix budget")
	}
	if i, field, actual, limit, exceeded := OptionMatrixDetailBudgetIssue(body); !exceeded || i != 0 || field != "detail" || actual != 81 || limit != 80 {
		t.Errorf("detail budget issue = (%d, %q, %d, %d, %v)", i, field, actual, limit, exceeded)
	}
}

// An author names the recommended option; they do not count rows.
func TestOptionMatrixResolvesHighlightsByName(t *testing.T) {
	values, ok := compiledMatrix(t, harveyMatrix(map[string]any{
		"recommended":        "consolidate", // case-insensitive
		"decisive_criterion": "Payback",
		"highlight_label":    "Recommended",
	}))
	if !ok {
		t.Fatal("matrix did not reach table-highlight")
	}
	if values.HighlightRow == nil || *values.HighlightRow != 1 {
		t.Errorf("highlight_row = %v, want 1 (Consolidate)", values.HighlightRow)
	}
	if values.HighlightCol == nil || *values.HighlightCol != 1 {
		t.Errorf("highlight_col = %v, want 1 (Payback)", values.HighlightCol)
	}
	if values.HighlightLabel != "Recommended" {
		t.Errorf("highlight_label = %q", values.HighlightLabel)
	}

	// An explicit index works too, and an out-of-range one is ignored rather
	// than emitting a highlight the pattern would refuse.
	values, _ = compiledMatrix(t, harveyMatrix(map[string]any{"recommended": float64(2)}))
	if values.HighlightRow == nil || *values.HighlightRow != 2 {
		t.Errorf("highlight_row = %v, want 2", values.HighlightRow)
	}
	values, _ = compiledMatrix(t, harveyMatrix(map[string]any{"recommended": float64(9)}))
	if values.HighlightRow != nil {
		t.Errorf("highlight_row = %v for an out-of-range index, want none", *values.HighlightRow)
	}
}

func TestOptionMatrixAcceptsFieldAliases(t *testing.T) {
	values, ok := compiledMatrix(t, map[string]any{
		"columns": []any{map[string]any{"label": "Cost"}, "Speed"},
		"rows": []any{
			map[string]any{"option": "A", "description": "first", "values": []any{"green", "amber"}},
			map[string]any{"title": "B", "values": []any{"red", "green"}},
		},
		"scale": "rag",
	})
	if !ok {
		t.Fatal("aliased matrix did not reach table-highlight")
	}
	if values.Criteria[0].Label != "Cost" || values.Criteria[1].Label != "Speed" {
		t.Errorf("criteria = %+v", values.Criteria)
	}
	if values.Options[0].Name != "A" || values.Options[0].Detail != "first" {
		t.Errorf("options[0] = %+v", values.Options[0])
	}
	if values.Scale != "rag" {
		t.Errorf("scale = %q, want rag", values.Scale)
	}
}

// A per-column scale lets one matrix mix Harvey balls with a text column.
func TestOptionMatrixPerColumnScale(t *testing.T) {
	values, ok := compiledMatrix(t, map[string]any{
		"criteria": []any{"Fit", map[string]any{"label": "Owner", "scale": "text"}},
		"options": []any{
			map[string]any{"name": "A", "scores": []any{4, "Ops"}},
			map[string]any{"name": "B", "scores": []any{2, "Finance"}},
		},
	})
	if !ok {
		t.Fatal("mixed-scale matrix did not reach table-highlight")
	}
	if values.Criteria[1].Scale != "text" {
		t.Errorf("criteria[1].scale = %q, want text", values.Criteria[1].Scale)
	}
}

func TestOptionMatrixDegradesOutsideThePatternBounds(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want []string
	}{
		{
			name: "one criterion is a ranking, not a matrix",
			body: map[string]any{
				"criteria": []any{"Cost"},
				"options": []any{
					map[string]any{"name": "A", "scores": []any{1}},
					map[string]any{"name": "B", "scores": []any{2}},
				},
			},
			want: []string{"A (Cost: 1)", "B (Cost: 2)"},
		},
		{
			name: "a row with the wrong number of scores",
			body: map[string]any{
				"criteria": []any{"Cost", "Speed"},
				"options": []any{
					map[string]any{"name": "A", "scores": []any{1, 2}},
					map[string]any{"name": "B", "scores": []any{3}},
				},
			},
			want: []string{"A (Cost: 1; Speed: 2)", "B (Cost: 3)"},
		},
		{
			name: "a score the scale cannot read",
			body: map[string]any{
				"criteria": []any{"Cost", "Speed"},
				"options": []any{
					map[string]any{"name": "A", "scores": []any{"yes", "no"}},
					map[string]any{"name": "B", "scores": []any{1, 2}},
				},
				"scale": "harvey",
			},
			want: []string{"A (Cost: yes; Speed: no)", "B (Cost: 1; Speed: 2)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide, _, err := CompileOptionMatrix(Input{Title: "Options", Body: tt.body})
			if err != nil {
				t.Fatalf("CompileOptionMatrix: %v", err)
			}
			if slide.Pattern != nil {
				t.Fatalf("expected the scored-bullet fallback, got pattern %q", slide.Pattern.Name)
			}
			if len(slide.Content) < 2 || slide.Content[1].BulletsValue == nil {
				t.Fatalf("no bullets block: %+v", slide.Content)
			}
			got := *slide.Content[1].BulletsValue
			if len(got) != len(tt.want) {
				t.Fatalf("bullets = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if !strings.Contains(got[i], tt.want[i]) {
					t.Errorf("bullets[%d] = %q, want it to contain %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// A badge over the pattern's budget costs the badge, not the whole matrix: the
// row it marks is highlighted either way, and losing the table over a label
// would be the wrong trade.
func TestOptionMatrixKeepsBadgeWithinBudget(t *testing.T) {
	values, ok := compiledMatrix(t, harveyMatrix(map[string]any{
		"recommended":     "Consolidate",
		"highlight_label": "Recommended",
	}))
	if !ok || values.HighlightLabel != "Recommended" {
		t.Fatalf("a badge within budget must render: %+v", values)
	}

	long := strings.Repeat("x", 40)
	values, ok = compiledMatrix(t, harveyMatrix(map[string]any{
		"recommended":     "Consolidate",
		"highlight_label": long,
	}))
	if !ok {
		t.Fatal("an over-long badge must not cost the whole matrix")
	}
	if values.HighlightLabel != "" {
		t.Errorf("highlight_label = %q, want it dropped", values.HighlightLabel)
	}
	if values.HighlightRow == nil {
		t.Error("the row is still highlighted without its badge")
	}
	if OptionMatrixLabelFits(long) {
		t.Error("OptionMatrixLabelFits must report the over-long badge so validation can warn")
	}
}

// OptionMatrixPatternFeasible is what explain and validate consult, so it has to
// agree with the compiler on every input.
func TestOptionMatrixPatternFeasibleMatchesCompile(t *testing.T) {
	bodies := []map[string]any{
		harveyMatrix(nil),
		harveyMatrix(map[string]any{"scale": "rag"}),
		{"criteria": []any{"a"}, "options": []any{map[string]any{"name": "A", "scores": []any{1}}}},
		{"criteria": []any{"a", "b"}, "options": []any{map[string]any{"name": "A", "scores": []any{1, 2}}}},
		{"criteria": []any{"a", "b"}, "options": []any{
			map[string]any{"name": "A", "scores": []any{1, 2}},
			map[string]any{"name": "B", "scores": []any{9, 9}},
		}},
		{},
	}
	for _, body := range bodies {
		slide, _, err := CompileOptionMatrix(Input{Title: "T", Body: body})
		if err != nil {
			t.Fatalf("compile %v: %v", body, err)
		}
		compiled := slide.Pattern != nil
		if got := OptionMatrixPatternFeasible(body); got != compiled {
			t.Errorf("OptionMatrixPatternFeasible(%v) = %v, compile emitted pattern = %v", body, got, compiled)
		}
	}
}

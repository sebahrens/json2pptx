package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestKPIComparatorRendered pins go-slide-creator-lsfbi: a comparator ("vs
// plan +4 pts") renders as its own line after the caption and delta, in the
// delta's size and ink, and every card reserves the line so baselines align.
func TestKPIComparatorRendered(t *testing.T) {
	var cells KPINupValues
	if err := json.Unmarshal([]byte(`[
		{"big":"38%","small":"Gross margin","delta":"+4 pts","vs":"vs plan +4 pts"},
		{"big":"$12M","small":"Free cash flow","comparator":"vs PY -2%"},
		{"big":"94%","small":"On-time delivery"}]`), &cells); err != nil {
		t.Fatal(err)
	}
	if cells[0].Comparator != "vs plan +4 pts" || cells[1].Comparator != "vs PY -2%" {
		t.Fatalf("comparator aliases not read: %+v", cells)
	}
	p, _ := Default().Get("kpi-3up")
	if err := p.Validate(&cells, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, &cells, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	type para struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color"`
	}
	var sizes []float64
	for i := range cells {
		var body struct {
			Paragraphs []para `json:"paragraphs"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[i].Shape.Text, &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Paragraphs) != 4 {
			t.Fatalf("cell %d paragraphs = %+v, want value/caption/delta/comparator", i, body.Paragraphs)
		}
		delta, cmp := body.Paragraphs[2], body.Paragraphs[3]
		if cmp.Size != delta.Size || cmp.Color != delta.Color {
			t.Errorf("cell %d comparator style %+v differs from delta %+v", i, cmp, delta)
		}
		sizes = append(sizes, cmp.Size)
		if i == 2 && cmp.Content != " " {
			t.Errorf("cell 2 should reserve a blank comparator line, got %q", cmp.Content)
		}
	}
	if sizes[0] != sizes[1] || sizes[1] != sizes[2] {
		t.Errorf("comparator sizes differ across the row: %v", sizes)
	}

	cells[0].Comparator = strings.Repeat("x", kpiComparatorMaxChars+1)
	if err := p.Validate(&cells, nil, nil); err == nil || !strings.Contains(err.Error(), "comparator") {
		t.Errorf("over-long comparator: err = %v", err)
	}
}

// The height-capped kpi-inline bar has no comparator line: it says so rather
// than silently dropping the reference.
func TestKPIInlineRejectsComparator(t *testing.T) {
	p, _ := Default().Get("kpi-inline")
	cells := KPINupValues{{Big: "38%", Small: "Margin", Comparator: "vs plan"}, {Big: "$12M", Small: "FCF"}}
	if err := p.Validate(&cells, nil, nil); err == nil || !strings.Contains(err.Error(), "comparator") {
		t.Errorf("kpi-inline comparator: err = %v", err)
	}
}

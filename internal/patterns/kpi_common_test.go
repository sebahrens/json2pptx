package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestKPICellSubAliases verifies that the sub/delta/trend/change keys are
// captured into KPICell.Sub instead of being silently dropped (go-slide-creator-09pa).
func TestKPICellSubAliases(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"canonical_sub", `{"big":"$50M","small":"Revenue","sub":"+5%"}`, "+5%"},
		{"alias_delta", `{"value":"$50M","label":"Revenue","delta":"+5%"}`, "+5%"},
		{"alias_trend", `{"big":"127%","small":"NRR","trend":"+3%"}`, "+3%"},
		{"alias_change", `{"big":"2.1%","small":"Churn","change":"-0.4%"}`, "-0.4%"},
		{"no_sub", `{"big":"$50M","small":"Revenue"}`, ""},
		{"sub_precedence_over_delta", `{"big":"x","small":"y","sub":"a","delta":"b"}`, "a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cell KPICell
			if err := json.Unmarshal([]byte(tc.json), &cell); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if cell.Sub != tc.want {
				t.Errorf("Sub = %q, want %q", cell.Sub, tc.want)
			}
		})
	}
}

// TestKPISubRendered verifies the sub annotation reaches the expanded shape text
// as its own paragraph, between the big number and the caption.
func TestKPISubRendered(t *testing.T) {
	p, ok := Default().Get("kpi-3up")
	if !ok {
		t.Fatal("kpi-3up not registered")
	}
	vals := KPINupValues{
		{Big: "$50M", Small: "Revenue", Sub: "+5%"},
		{Big: "127%", Small: "NRR", Sub: "+3%"},
		{Big: "2.1%", Small: "Churn"}, // no sub
	}
	grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	type para struct {
		Content string `json:"content"`
	}
	type textObj struct {
		Paragraphs []para `json:"paragraphs"`
	}

	parse := func(i int) textObj {
		var to textObj
		if err := json.Unmarshal(grid.Rows[0].Cells[i].Shape.Text, &to); err != nil {
			t.Fatalf("cell[%d] text unmarshal: %v", i, err)
		}
		return to
	}

	// Cell 0 has a sub: expect 3 paragraphs big, sub, small in order.
	c0 := parse(0)
	if len(c0.Paragraphs) != 3 {
		t.Fatalf("cell[0] paragraphs = %d, want 3", len(c0.Paragraphs))
	}
	gotOrder := []string{c0.Paragraphs[0].Content, c0.Paragraphs[1].Content, c0.Paragraphs[2].Content}
	wantOrder := []string{"$50M", "+5%", "Revenue"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("cell[0] paragraph[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}

	// Cell 2 has no sub: expect 2 paragraphs (unchanged behavior).
	c2 := parse(2)
	if len(c2.Paragraphs) != 2 {
		t.Errorf("cell[2] paragraphs = %d, want 2", len(c2.Paragraphs))
	}
	if strings.Contains(string(grid.Rows[0].Cells[2].Shape.Text), "+") {
		t.Errorf("cell[2] should not contain a delta annotation")
	}
}

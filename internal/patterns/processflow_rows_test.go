package patterns

import (
	"strings"
	"testing"
)

// go-slide-creator-pfyeg: seven or eight steps on one row left each box about
// 90pt wide, so a label inside the documented budget wrapped into a column of
// one- and two-word lines. The flow now bends onto two rows.

func TestProcessFlowTwoRows(t *testing.T) {
	p := &processFlow{}
	// rowLabels reads each grid row's labels left to right ("" for a spacer).
	rowLabels := func(t *testing.T, steps []ProcessFlowStep, ovr any) ([]string, [][4]int) {
		t.Helper()
		grid, err := p.Expand(testThemeCtx(), &ProcessFlowValues{Steps: steps}, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		var rows []string
		for _, row := range grid.Rows {
			var cells []string
			for _, c := range row.Cells {
				if c == nil || c.Shape == nil {
					cells = append(cells, "")
					continue
				}
				text := cellText(t, c.Shape.Text)
				cells = append(cells, text.Paragraphs[0].Content)
			}
			rows = append(rows, strings.Join(cells, "|"))
		}
		var links [][4]int
		for _, l := range grid.Links {
			if l.Connector == nil || l.Connector.Style != "arrow" {
				t.Errorf("link %v has connector %+v, want the flow's arrow", l, l.Connector)
			}
			links = append(links, [4]int{l.From[0], l.From[1], l.To[0], l.To[1]})
		}
		return rows, links
	}

	t.Run("eight steps bend back under the first four", func(t *testing.T) {
		rows, links := rowLabels(t, pfSteps(8), nil)
		want := []string{"Request|Review|Approve|Build", "Close|Monitor|Deploy|Test"}
		if strings.Join(rows, " / ") != strings.Join(want, " / ") {
			t.Errorf("rows = %q, want %q", rows, want)
		}
		// Down from the fourth step, then right to left along the second row.
		wantLinks := [][4]int{{0, 3, 1, 3}, {1, 3, 1, 2}, {1, 2, 1, 1}, {1, 1, 1, 0}}
		if len(links) != len(wantLinks) {
			t.Fatalf("links = %v, want %v", links, wantLinks)
		}
		for i := range links {
			if links[i] != wantLinks[i] {
				t.Errorf("link %d = %v, want %v", i, links[i], wantLinks[i])
			}
		}
	})

	t.Run("seven steps leave the last column of the second row empty", func(t *testing.T) {
		rows, links := rowLabels(t, pfSteps(7), nil)
		if len(rows) != 2 || rows[1] != "|Monitor|Deploy|Test" {
			t.Errorf("rows = %q, want the second row right-aligned under the first", rows)
		}
		if len(links) != 3 {
			t.Errorf("links = %v, want the drop and two returning arrows", links)
		}
	})

	t.Run("chevrons wrap like a line of text", func(t *testing.T) {
		steps := pfSteps(8)
		for i := range steps {
			steps[i].Type = "chevron"
		}
		rows, links := rowLabels(t, steps, nil)
		if len(rows) != 2 || rows[1] != "Test|Deploy|Monitor|Close" || len(links) != 0 {
			t.Errorf("rows = %q links = %v, want a left-to-right second row and no connectors", rows, links)
		}
	})

	t.Run("rows 1 keeps one row and rows 2 bends a shorter flow", func(t *testing.T) {
		if rows, links := rowLabels(t, pfSteps(8), &ProcessFlowOverrides{Rows: 1}); len(rows) != 1 || len(links) != 0 {
			t.Errorf("rows 1: rows = %q links = %v, want one row", rows, links)
		}
		if rows, _ := rowLabels(t, pfSteps(6), nil); len(rows) != 1 {
			t.Errorf("six steps: rows = %q, want one row", rows)
		}
		if rows, _ := rowLabels(t, pfSteps(6), &ProcessFlowOverrides{Rows: 2}); len(rows) != 2 || rows[1] != "Deploy|Test|Build" {
			t.Errorf("rows 2 on six steps: rows = %q", rows)
		}
		// A chevron cannot point back along a returning row: a mixed flow keeps
		// one row unless rows says otherwise.
		mixed := pfSteps(8)
		mixed[0].Type = "chevron"
		if rows, _ := rowLabels(t, mixed, nil); len(rows) != 1 {
			t.Errorf("mixed flow: rows = %q, want one row", rows)
		}
	})

	t.Run("a label at the full budget fits both rows", func(t *testing.T) {
		steps := pfSteps(8)
		for i := range steps {
			steps[i].Label = strings.TrimSpace(strings.Repeat("word ", 16))
		}
		for _, area := range writtenFitAreas {
			ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(area.w * 12700), Height: int64(area.h * 12700)}}
			ctx.Theme.BodyFont = area.font
			if w := p.PostExpandWarnings(ctx, &ProcessFlowValues{Steps: steps}, nil); len(w) != 0 {
				t.Errorf("%s: eight 79-character steps were reported: %v", area.name, w)
			}
		}
	})

	t.Run("validation", func(t *testing.T) {
		for _, tc := range []struct {
			n, rows int
			ok      bool
		}{{8, 2, true}, {8, 1, true}, {4, 2, true}, {3, 2, false}, {5, 3, false}} {
			err := p.Validate(&ProcessFlowValues{Steps: pfSteps(tc.n)}, &ProcessFlowOverrides{Rows: tc.rows}, nil)
			if (err == nil) != tc.ok {
				t.Errorf("%d steps, rows %d: err = %v, want ok=%v", tc.n, tc.rows, err, tc.ok)
			}
		}
		compact := &processFlowCompact{}
		if err := compact.Validate(&ProcessFlowValues{Steps: pfSteps(5)}, &ProcessFlowOverrides{Rows: 2}, nil); err == nil || !strings.Contains(err.Error(), "overrides.rows") {
			t.Errorf("process-flow-compact accepted overrides.rows: %v", err)
		}
	})
}

// The budget the semantic compiler writes to follows the default layout.
func TestProcessFlowLabelBudgetFollowsRows(t *testing.T) {
	for _, n := range []int{7, 8} {
		if word, unbroken := ProcessFlowLabelBudget(n, false); word != 80 || unbroken != 80 {
			t.Errorf("%d steps: budget %d/%d, want the four-step box's 80/80", n, word, unbroken)
		}
		if word, unbroken := ProcessFlowLabelBudget(n, true); word != 80 || unbroken != 69 {
			t.Errorf("%d chevrons: budget %d/%d, want the four-chevron 80/69", n, word, unbroken)
		}
	}
	if word, _ := ProcessFlowLabelBudget(6, true); word != 31 {
		t.Errorf("six chevrons: budget %d, want 31", word)
	}
}

package rhythm

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// go-slide-creator-xx9i. Every hand-authored shape_grid slide was fingerprinted
// as the literal pattern name "shape_grid", so six structurally DIFFERENT grids
// in a row read as a six-slide pattern_run. That was tolerable while the
// composition score was advisory; once it gates the deck it would have blocked
// genuinely varied decks — sovereign-ai-strategy, 25 slides of hand-built
// grids, scored composition 15 on three such phantom runs.

func gridOf(rows ...int) *shapegrid.Grid {
	g := &shapegrid.Grid{}
	for _, n := range rows {
		row := shapegrid.Row{}
		for i := 0; i < n; i++ {
			row.Cells = append(row.Cells, shapegrid.Cell{})
		}
		g.Rows = append(g.Rows, row)
	}
	return g
}

func TestShapeGridFingerprintDistinguishesLayouts(t *testing.T) {
	threeUp := Slide{HasShapeGrid: true, Grid: gridOf(3)}
	twoByTwo := Slide{HasShapeGrid: true, Grid: gridOf(2, 2)}

	a := fingerprint(0, threeUp).Pattern
	b := fingerprint(1, twoByTwo).Pattern
	if a == b {
		t.Errorf("a 3-cell row and a 2x2 grid share the fingerprint %q — different layouts count as a run", a)
	}
	if !strings.HasPrefix(a, "shape_grid") || !strings.HasPrefix(b, "shape_grid") {
		t.Errorf("fingerprints %q / %q should still identify the surface as a shape_grid", a, b)
	}

	// Two identical layouts must still collide — that IS a run.
	if fingerprint(2, threeUp).Pattern != a {
		t.Error("two identical grids must share a fingerprint")
	}
}

func TestShapeGridFingerprintFallsBack(t *testing.T) {
	// No resolved grid: fall back to the cell count, then to the bare name.
	if got := shapeGridFingerprint(Slide{HasShapeGrid: true, CellCount: 4}); got != "shape_grid:4cell" {
		t.Errorf("cell-count fallback = %q", got)
	}
	if got := shapeGridFingerprint(Slide{HasShapeGrid: true}); got != "shape_grid" {
		t.Errorf("bare fallback = %q", got)
	}
}

// TestVariedGridsDoNotCountAsAPatternRun is the deck-level consequence.
func TestVariedGridsDoNotCountAsAPatternRun(t *testing.T) {
	varied := []Slide{
		{HasShapeGrid: true, Grid: gridOf(3)},
		{HasShapeGrid: true, Grid: gridOf(2, 2)},
		{HasShapeGrid: true, Grid: gridOf(4)},
		{HasShapeGrid: true, Grid: gridOf(1, 3)},
		{HasShapeGrid: true, Grid: gridOf(2)},
	}
	same := []Slide{
		{HasShapeGrid: true, Grid: gridOf(3)},
		{HasShapeGrid: true, Grid: gridOf(3)},
		{HasShapeGrid: true, Grid: gridOf(3)},
		{HasShapeGrid: true, Grid: gridOf(3)},
		{HasShapeGrid: true, Grid: gridOf(3)},
	}
	if got := Analyze(varied).CompositionScore; got <= Analyze(same).CompositionScore {
		t.Errorf("five different grids score %d, no better than five identical ones (%d)",
			got, Analyze(same).CompositionScore)
	}
}

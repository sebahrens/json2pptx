package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

func TestComputeCellBudgets_AllPatternsAllTemplates(t *testing.T) {
	// Table-driven: every registered pattern × every template-like ExpandContext.
	// Verifies that computeCellBudgets never panics and returns plausible results.
	reg := patterns.Default()
	allPatterns := reg.List()

	// Two representative ExpandContexts: default fallback and a typical template.
	contexts := []struct {
		name string
		ctx  patterns.ExpandContext
	}{
		{
			name: "default_fallback",
			ctx: patterns.ExpandContext{
				SlideWidth:  9144000,
				SlideHeight: 5143500,
				LayoutBounds: patterns.LayoutBounds{
					X: 457200, Y: 457200,
					Width: 8229600, Height: 4229100,
				},
			},
		},
		{
			name: "wide_template",
			ctx: patterns.ExpandContext{
				SlideWidth:  12192000,
				SlideHeight: 6858000,
				LayoutBounds: patterns.LayoutBounds{
					X: 457200, Y: 1371600,
					Width: 11277600, Height: 5029200,
				},
			},
		},
	}

	for _, pat := range allPatterns {
		// Get exemplar values to expand
		ex, ok := pat.(patterns.Exemplar)
		if !ok {
			continue // skip patterns without exemplar values
		}

		values := ex.ExemplarValues()
		for _, tc := range contexts {
			t.Run(pat.Name()+"/"+tc.name, func(t *testing.T) {
				grid, err := pat.Expand(tc.ctx, values, nil, nil)
				if err != nil {
					t.Fatalf("expand failed: %v", err)
				}
				if grid == nil {
					t.Skip("pattern returned nil grid")
				}

				budgets, warnings := computeCellBudgets(grid, tc.ctx)

				// Every expanded grid should produce at least one cell budget
				if len(budgets) == 0 {
					t.Errorf("expected cell_budgets, got none")
				}

				// Verify budget invariants
				for _, b := range budgets {
					if b.CellIndex < 0 {
						t.Errorf("cell_index %d < 0", b.CellIndex)
					}
					if b.MaxChars < 0 {
						t.Errorf("cell %d: max_chars %d < 0", b.CellIndex, b.MaxChars)
					}
					if b.ActualChars < 0 {
						t.Errorf("cell %d: actual_chars %d < 0", b.CellIndex, b.ActualChars)
					}
					if b.DensityPct < 0 {
						t.Errorf("cell %d: density_pct %d < 0", b.CellIndex, b.DensityPct)
					}
					if b.FontSizePt <= 0 {
						t.Errorf("cell %d: font_size_pt %f <= 0", b.CellIndex, b.FontSizePt)
					}
					// Status must be a known value
					switch textcapacity.Status(b.Status) {
					case textcapacity.StatusUnderfilled, textcapacity.StatusOptimal, textcapacity.StatusOverflow:
						// ok
					default:
						t.Errorf("cell %d: unknown status %q", b.CellIndex, b.Status)
					}
				}

				// Warnings should only appear for non-optimal cells with content
				for _, w := range warnings {
					if w.Actual <= 0 {
						t.Errorf("warning for cell %d has actual=%d but warnings should only appear for cells with content", w.CellIndex, w.Actual)
					}
					if w.Status == string(textcapacity.StatusOptimal) {
						t.Errorf("warning for cell %d has status=optimal but warnings should only appear for non-optimal cells", w.CellIndex)
					}
				}
			})
		}
	}
}

func TestComputeCellBudgets_NilGrid(t *testing.T) {
	ctx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}
	budgets, warnings := computeCellBudgets(nil, ctx)
	if budgets != nil {
		t.Errorf("expected nil budgets for nil grid, got %d", len(budgets))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for nil grid, got %d", len(warnings))
	}
}

func TestComputeCellBudgets_EmptyGrid(t *testing.T) {
	ctx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}
	grid := &jsonschema.ShapeGridInput{Rows: nil}
	budgets, warnings := computeCellBudgets(grid, ctx)
	if budgets != nil {
		t.Errorf("expected nil budgets for empty grid, got %d", len(budgets))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for empty grid, got %d", len(warnings))
	}
}

func TestComputeCellBudgets_DeterministicAcrossRuns(t *testing.T) {
	// Verify that budgets are deterministic (same input → same output).
	reg := patterns.Default()
	pat, ok := reg.Get("kpi-3up")
	if !ok {
		t.Skip("kpi-3up pattern not registered")
	}
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		t.Skip("kpi-3up has no exemplar values")
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}

	grid, err := pat.Expand(ctx, ex.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}

	budgets1, _ := computeCellBudgets(grid, ctx)
	budgets2, _ := computeCellBudgets(grid, ctx)

	if len(budgets1) != len(budgets2) {
		t.Fatalf("different budget counts: %d vs %d", len(budgets1), len(budgets2))
	}
	for i := range budgets1 {
		if budgets1[i] != budgets2[i] {
			t.Errorf("cell %d: budgets differ across runs: %+v vs %+v", i, budgets1[i], budgets2[i])
		}
	}
}

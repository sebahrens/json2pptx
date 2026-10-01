package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProcessFlowCompactPointedStepsKeepReadableTextWidth(t *testing.T) {
	ctx := processFlowChevronCtx()
	for _, tc := range []struct {
		name  string
		steps []ProcessFlowStep
	}{
		{name: "all chevrons", steps: []ProcessFlowStep{
			{Label: "Baseline", Type: "chevron"}, {Label: "Review", Type: "chevron"},
			{Label: "Approve", Type: "chevron"}, {Label: "Launch", Type: "chevron"},
		}},
		{name: "mixed shapes", steps: []ProcessFlowStep{
			{Label: "Baseline", Type: "chevron"}, {Label: "Review", Type: "step"},
			{Label: "Approve", Type: "arrow"}, {Label: "Launch", Type: "step"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grid, err := (&processFlowCompact{}).Expand(ctx, &ProcessFlowValues{Steps: tc.steps}, nil, nil)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if grid.Bounds == nil {
				t.Fatal("missing compact bounds")
			}
			width, bandCap := processFlowCompactCellSize(ctx, len(tc.steps), true)
			_, contentHeight := contentAreaPt(ctx)
			// Content-sized at or below the pointed cap (go-slide-creator-xb06p).
			height := grid.Bounds.Height / 100 * contentHeight
			if height <= 0 || height > bandCap+0.01 {
				t.Fatalf("rendered band height = %.2fpt, want content-sized within the %.2fpt cap", height, bandCap)
			}
			if height > width*chevronMaxAspectH {
				t.Errorf("%.1fpt pointed band exceeds half of %.1fpt step width", height, width)
			}
			for i, cell := range grid.Rows[0].Cells {
				if tc.steps[i].Type != "chevron" && tc.steps[i].Type != "arrow" {
					continue
				}
				// Pointed labels keep the uniform shape text margin inside
				// the preset's text rectangle: no per-side insets.
				var text map[string]any
				if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
					t.Fatalf("cell %d text: %v", i, err)
				}
				for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
					if _, has := text[k]; has {
						t.Errorf("cell %d emits %s; want the uniform margin", i, k)
					}
				}
				notch := float64(chevronAdj) / 100000 * height
				usable := width - 2*notch - 2*defaultShapeInsetLRPt
				// The notch is counted once (the preset's text rectangle) and
				// the uniform margin once per side; a doubled notch inset
				// would drop well below half the step.
				if usable < width*0.55 {
					t.Errorf("cell %d leaves only %.1fpt of %.1fpt for text", i, usable, width)
				}
				if measuredLines(tc.steps[i].Label, ctx.Theme.BodyFont, true, processFlowDefaultFontPt(len(tc.steps)), usable) != 1 {
					t.Errorf("cell %d label %q wraps despite short authored text (usable %.1fpt)", i, tc.steps[i].Label, usable)
				}
			}
		})
	}
}

func TestProcessFlowCompactPointedLabelBudgets(t *testing.T) {
	p := &processFlowCompact{}
	v := &ProcessFlowValues{Steps: make([]ProcessFlowStep, 8)}
	for i := range v.Steps {
		v.Steps[i] = ProcessFlowStep{Label: "Stage", Type: "chevron"}
	}
	v.Steps[3].Label = "word wordw" // 10 word-like characters
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured word-like target should fit: %v", got)
	}
	v.Steps[3].Label += "w"
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "steps[3].label") || !strings.Contains(got[0], "about 10 word-like or 8 wide") {
		t.Fatalf("dense chevron warning: %v", got)
	}
	v.Steps[3].Label = strings.Repeat("W", 9)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 {
		t.Fatalf("wide chevron should warn: %v", got)
	}
	v.Steps[3].Type = "step"
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("rectangular step should have more room: %v", got)
	}
}

func TestProcessFlowCompact_Registered(t *testing.T) {
	_, ok := Default().Get("process-flow-compact")
	if !ok {
		t.Fatal("process-flow-compact pattern not registered")
	}
}

func TestProcessFlowCompact_ExpandBasic(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Start", Type: "step"},
			{Label: "Check", Type: "decision"},
			{Label: "End", Type: "step"},
		},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// Bounds are content-sized at or below the compact cap
	// (go-slide-creator-xb06p).
	if grid.Bounds == nil {
		t.Fatal("expected bounds to be set for compact variant")
	}
	if grid.Bounds.Height <= 0 || grid.Bounds.Height > processFlowCompactHeightPct {
		t.Errorf("expected bounds height within (0, %v], got %v", processFlowCompactHeightPct, grid.Bounds.Height)
	}
	if grid.Bounds.Width != 100 {
		t.Errorf("expected bounds width 100, got %v", grid.Bounds.Width)
	}

	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}
	if grid.Rows[0].Connector == nil {
		t.Error("expected connector on the row")
	}
}

func TestProcessFlowCompact_ValidateMinSteps(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")

	vals := &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "A"},
			{Label: "B"},
		},
	}
	if err := p.Validate(vals, nil, nil); err == nil {
		t.Error("expected validation error for < 3 steps")
	}
}

func TestProcessFlowCompact_TaxonomyDensityLow(t *testing.T) {
	p, _ := Default().Get("process-flow-compact")
	tax := p.Taxonomy()
	if tax.DensityClass != "low" {
		t.Errorf("expected DensityClass 'low', got %q", tax.DensityClass)
	}
}

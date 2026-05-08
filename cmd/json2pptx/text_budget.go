package main

import (
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

// computeTextBudgetGuide computes on-demand text budgets for a pattern that
// implements BudgetConfigProvider. Returns nil for non-grid patterns.
func computeTextBudgetGuide(pat patterns.Pattern) *textcapacity.TextBudgetGuide {
	bcp, ok := pat.(patterns.BudgetConfigProvider)
	if !ok {
		return nil
	}
	patConfigs := bcp.BudgetConfigurations()
	if len(patConfigs) == 0 {
		return nil
	}

	// Use default 16:9 slide dimensions and layout bounds (same as
	// resolveExpandContext fallback) so budgets are template-independent.
	const (
		slideWidth  int64 = 9144000
		slideHeight int64 = 5143500
	)
	layoutBounds := pptx.RectEmu{
		X:  457200,
		Y:  457200,
		CX: 8229600,
		CY: 4229100,
	}

	expandCtx := patterns.ExpandContext{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
		LayoutBounds: patterns.LayoutBounds{
			X:      layoutBounds.X,
			Y:      layoutBounds.Y,
			Width:  layoutBounds.CX,
			Height: layoutBounds.CY,
		},
	}

	configs := make([]textcapacity.GridBudgetConfig, len(patConfigs))
	for i, c := range patConfigs {
		configs[i] = textcapacity.GridBudgetConfig{Columns: c.Columns, Rows: c.Rows}
	}

	expandFn := func(cols, rows int) (*jsonschema.ShapeGridInput, error) {
		values := syntheticValues(pat, cols, rows)
		if values == nil {
			return nil, nil
		}
		return pat.Expand(expandCtx, values, nil, nil)
	}

	bounds := shapegrid.DefaultBounds(slideWidth, slideHeight)

	return textcapacity.ComputeBudgetGuide(configs, expandFn, bounds, slideWidth, slideHeight)
}

// syntheticValues creates minimal valid values for a pattern at the given
// columns×rows configuration. Each pattern type needs its own synthetic
// generation; unknown types return nil.
func syntheticValues(pat patterns.Pattern, cols, rows int) any {
	switch pat.Name() {
	case "card-grid":
		n := cols * rows
		cells := make([]patterns.CardGridCell, n)
		for i := range cells {
			cells[i] = patterns.CardGridCell{
				Header: "Header",
				Body:   "Body text placeholder",
			}
		}
		return &patterns.CardGridValues{
			Columns: cols,
			Rows:    rows,
			Cells:   cells,
		}
	case "hero-detail":
		// For hero-detail, cols = number of detail items, rows is always 2
		details := make([]patterns.HeroDetailItem, cols)
		for i := range details {
			details[i] = patterns.HeroDetailItem{
				Title: "Detail Title",
				Body:  "Supporting detail text",
			}
		}
		return &patterns.HeroDetailValues{
			Hero: patterns.HeroDetailHero{
				Value: "$1.0B",
				Label: "Metric label placeholder",
			},
			Details: details,
		}
	case "process-flow":
		// cols = number of steps (3-8), rows is always 1
		steps := make([]patterns.ProcessFlowStep, cols)
		for i := range steps {
			steps[i] = patterns.ProcessFlowStep{
				Label: "Step",
				Type:  "step",
			}
		}
		return &patterns.ProcessFlowValues{Steps: steps}
	case "before-after":
		// Grid is always 3 columns × 2 rows; items fill the body cells
		return &patterns.BeforeAfterValues{
			Before: patterns.BeforeAfterColumn{Header: "Before", Items: []string{"Item one", "Item two", "Item three"}},
			After:  patterns.BeforeAfterColumn{Header: "After", Items: []string{"Item one", "Item two", "Item three"}},
		}
	case "kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up":
		// cols = number of KPI cells, rows is always 1
		cells := make(patterns.KPINupValues, cols)
		for i := range cells {
			cells[i] = patterns.KPICell{Big: "$1.0M", Small: "Metric"}
		}
		return &cells
	case "stylish-panels":
		// cols = number of panels, rows is always 2 (header + body)
		items := make(patterns.StylishPanelsValues, cols)
		for i := range items {
			items[i] = patterns.StylishPanelsItem{
				Title: "Panel Title",
				Body:  []string{"Bullet one", "Bullet two", "Bullet three"},
			}
		}
		return &items
	default:
		return nil
	}
}

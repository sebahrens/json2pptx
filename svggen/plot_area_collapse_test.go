package svggen

import (
	"testing"
)

// longCategories builds n category labels of roughly the given character
// length — the European revenue-stream breakdown shape that collapsed the plot.
func longCategories(n, chars int) []string {
	base := "Recurring licence revenue stream"
	out := make([]string, n)
	for i := range out {
		label := base
		for len(label) < chars {
			label += "x"
		}
		out[i] = label[:chars]
	}
	return out
}

// go-slide-creator-k478: AdaptXLabels grew the bottom margin by whatever the
// rotated label height needed, uncapped. With 16 categories of ~30 characters
// the plot was squeezed to 5% of the canvas — a hairline of bars above a wall
// of rotated text — and nothing told the agent the chart had vanished.
func TestCapXLabelBand_KeepsThePlotReadable(t *testing.T) {
	// A wide, short chart panel: 150px of plot before the label band is
	// subtracted, which is the geometry a half-height chart slide gives.
	const prelimPlotH = 150.0
	b := NewSVGBuilder(1024, 456)
	cats := longCategories(16, 30)

	layout := AdaptXLabels(b, cats, 900, 12, false)
	if layout.ExtraBottomMargin <= prelimPlotH*(1-minPlotHeightFrac) {
		t.Fatalf("test fixture is stale: labels need only %.2fpx, which does not trip the cap",
			layout.ExtraBottomMargin)
	}
	CapXLabelBand(b, &layout, prelimPlotH, cats)

	maxBand := prelimPlotH * (1 - minPlotHeightFrac)
	if layout.ExtraBottomMargin > maxBand {
		t.Errorf("ExtraBottomMargin = %.2f, must not exceed %.2f (the plot keeps %.0f%% of its height)",
			layout.ExtraBottomMargin, maxBand, minPlotHeightFrac*100)
	}

	// The cap binding is not silent: the agent needs a signal that the labels
	// are now clipped and the chart is still too dense.
	var found *Finding
	for i, f := range b.Findings() {
		if f.Code == FindingPlotAreaCollapsed {
			found = &b.Findings()[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("capping the label band must emit %s, got %+v", FindingPlotAreaCollapsed, b.Findings())
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want warning", found.Severity)
	}
	if found.Fix == nil || found.Fix.Kind != FixKindShortenLabels {
		t.Errorf("fix = %+v, want kind %q", found.Fix, FixKindShortenLabels)
	}
	if found.Fix != nil {
		if got, ok := found.Fix.Params["total_categories"].(int); !ok || got != 16 {
			t.Errorf("fix.params.total_categories = %v, want 16", found.Fix.Params["total_categories"])
		}
		if got, ok := found.Fix.Params["longest_label_len"].(int); !ok || got != 30 {
			t.Errorf("fix.params.longest_label_len = %v, want 30", found.Fix.Params["longest_label_len"])
		}
	}
}

// Short labels must not trip the cap or the finding.
func TestCapXLabelBand_ShortLabelsUnaffected(t *testing.T) {
	b := NewSVGBuilder(1024, 456)
	cats := []string{"Q1", "Q2", "Q3", "Q4"}
	layout := AdaptXLabels(b, cats, 900, 12, false)
	CapXLabelBand(b, &layout, 300, cats)

	if layout.ExtraBottomMargin > 0 {
		t.Errorf("short labels should need no extra bottom margin, got %.2f", layout.ExtraBottomMargin)
	}
	for _, f := range b.Findings() {
		if f.Code == FindingPlotAreaCollapsed {
			t.Errorf("short labels must not report a collapsed plot area: %+v", f)
		}
	}
}

// End-to-end through the public render path at a wide, short chart panel
// (900x260 — the geometry a half-height chart slide or a chart-insights-split
// panel actually gives svggen), with the reported ~30-character labels over 16
// categories. Every affected family must report the collapse rather than
// silently shipping a hairline of bars above a wall of rotated text.
func TestRenderLongCategories_ReportsCollapse(t *testing.T) {
	chartTypes := []string{"bar_chart", "grouped_bar_chart", "stacked_bar_chart", "line_chart", "area_chart", "stacked_area_chart"}
	cats := longCategories(16, 30)

	catsAny := make([]any, len(cats))
	values := make([]any, len(cats))
	for i, c := range cats {
		catsAny[i] = c
		values[i] = float64(10 + i)
	}

	for _, ct := range chartTypes {
		t.Run(ct, func(t *testing.T) {
			series := []any{map[string]any{"name": "Revenue", "values": values}}
			if ct == "grouped_bar_chart" {
				series = append(series, map[string]any{"name": "Plan", "values": values})
			}
			findings, err := DryRender(&RequestEnvelope{
				Type:   ct,
				Data:   map[string]any{"categories": catsAny, "series": series},
				Output: OutputSpec{Width: 900, Height: 260},
			})
			if err != nil {
				t.Fatalf("dry render: %v", err)
			}

			var collapsed *Finding
			for i := range findings {
				if findings[i].Code == FindingPlotAreaCollapsed {
					collapsed = &findings[i]
					break
				}
			}
			if collapsed == nil {
				t.Fatalf("16 x 30-character categories on a 900x260 panel must report %s, got %+v",
					FindingPlotAreaCollapsed, findings)
			}
			if collapsed.Fix == nil || collapsed.Fix.Kind != FixKindShortenLabels {
				t.Errorf("fix = %+v, want kind %q", collapsed.Fix, FixKindShortenLabels)
			}
		})
	}
}

// A comfortably sized canvas with the same data must stay quiet: the guard is
// a backstop, not a new source of noise.
func TestRenderLongCategories_LargeCanvasStaysQuiet(t *testing.T) {
	cats := longCategories(16, 30)
	catsAny := make([]any, len(cats))
	values := make([]any, len(cats))
	for i, c := range cats {
		catsAny[i] = c
		values[i] = float64(10 + i)
	}

	findings, err := DryRender(&RequestEnvelope{
		Type:   "bar_chart",
		Data:   map[string]any{"categories": catsAny, "series": []any{map[string]any{"name": "Revenue", "values": values}}},
		Output: OutputSpec{Width: 1024, Height: 768},
	})
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for _, f := range findings {
		if f.Code == FindingPlotAreaCollapsed {
			t.Errorf("a 1024x768 canvas has room for these labels; reporting %s is noise: %+v", f.Code, f)
		}
	}
}

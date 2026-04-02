package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// =============================================================================
// KPI Dashboard Native Shapes — Grid of Metric Cards
// =============================================================================
//
// Replaces SVG-rendered KPI dashboards with native OOXML grouped shapes.
// Each metric card is a single roundRect with scheme-colored fill containing
// three text zones: hero value (large, accent-colored), label (small, bold),
// and delta/trend indicator (colored by trend direction).
//
// Layout:
//
//   ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   │ $21M     │       │ 1,250    │       │ 72       │       │ 2.1%     │
//   │ REVENUE  │       │ CUSTOMERS│       │ NPS      │       │ CHURN    │
//   │ ▲ +17%   │       │ ▲ +8%    │       │ ▲ +5     │       │ ▼ -0.3%  │
//   └──────────┘       └──────────┘       └──────────┘       └──────────┘
//
// Color strategy: card fills use accent1 with high lumMod/lumOff tints.
// Hero values use accent1. Labels use dk1. Delta text uses accent6 (up/green)
// or accent2 (down/red). All scheme-based for theme awareness.

// KPI dashboard EMU constants.
const (
	// kpiGap is the gap between metric cards in EMU.
	// ~0.15" = 137160 EMU — matches stat card gap.
	kpiGap int64 = 137160

	// kpiCornerRadius is the roundRect adjustment value.
	// 5000 = subtle rounding, same as stat cards.
	kpiCornerRadius int64 = 5000

	// kpiValueFontSize is the hero value font size (hundredths of a point).
	// 2800 = 28pt — slightly smaller than stat cards (32pt) to leave room for delta.
	kpiValueFontSize int = 2800

	// kpiLabelFontSize is the metric label font size (hundredths of a point).
	// 1100 = 11pt
	kpiLabelFontSize int = 1100

	// kpiDeltaFontSize is the delta/trend indicator font size (hundredths of a point).
	// 1000 = 10pt
	kpiDeltaFontSize int = 1000

	// kpiInset is the text inset for card shapes (EMU).
	kpiInset int64 = 108000 // ~0.118"

	// kpiMaxCols is the maximum number of columns in the KPI grid.
	kpiMaxCols = 4

	// kpiSpaceAfterValue is the space after the hero value paragraph (hundredths of a point).
	kpiSpaceAfterValue = 200

	// kpiSpaceAfterLabel is the space after the label paragraph (hundredths of a point).
	kpiSpaceAfterLabel = 100
)

// kpiMetric holds parsed data for a single KPI metric card.
type kpiMetric struct {
	label string // Metric name (e.g., "Revenue")
	value string // Display value (e.g., "$21M")
	delta string // Change indicator (e.g., "+17%")
	trend string // Direction: "up", "down", "flat"
}

// isKPIDashboardDiagram returns true if the diagram spec is a kpi_dashboard type.
func isKPIDashboardDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "kpi_dashboard"
}

// processKPIDashboardNativeShapes parses KPI dashboard data from a DiagramSpec and
// registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processKPIDashboardNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("kpi dashboard native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("kpi dashboard native shapes: themeOverride is set but scheme color refs will not reflect overrides",
			"slide", slideNum)
	}

	// Parse metrics from DiagramSpec.Data — accept "metrics" or "kpis" key.
	metrics := parseKPIMetrics(diagramSpec.Data)
	if len(metrics) == 0 {
		slog.Warn("kpi dashboard native shapes: no metrics found", "slide", slideNum)
		return
	}

	// Convert metrics to nativePanelData for the panelShapeInsert system.
	var panels []nativePanelData
	for _, m := range metrics {
		// Build delta text with trend arrow prefix.
		deltaText := buildKPIDeltaText(m.delta, m.trend)
		panels = append(panels, nativePanelData{
			title: m.label,
			value: m.value,
			body:  deltaText,
		})
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native kpi dashboard shapes: registered",
		"slide", slideNum,
		"metrics", len(metrics),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx:   shapeIdx,
		bounds:           placeholderBounds,
		panels:           panels,
		kpiDashboardMode: true,
	})
}

// generateKPIDashboardGroupXML produces the complete <p:grpSp> XML for a KPI dashboard.
// Each metric is a roundRect card with hero value, label, and delta/trend indicator.
func generateKPIDashboardGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		return ""
	}

	cols, rows := kpiGridLayout(n)

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	hGapTotal := int64(cols-1) * kpiGap
	vGapTotal := int64(rows-1) * kpiGap
	cardW := (totalWidth - hGapTotal) / int64(cols)
	cardH := (totalHeight - vGapTotal) / int64(rows)

	var children [][]byte
	idx := 0
	for row := range rows {
		for col := range cols {
			if idx >= n {
				break
			}
			panel := panels[idx]

			cardX := bounds.X + int64(col)*(cardW+kpiGap)
			cardY := bounds.Y + int64(row)*(cardH+kpiGap)

			// Each KPI card uses 1 shape ID.
			shapeID := shapeIDBase + uint32(idx) + 1

			cardXML := generateKPICardXML(panel, cardX, cardY, cardW, cardH, shapeID)
			children = append(children, []byte(cardXML))

			idx++
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "KPI Dashboard",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateKPIDashboardGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateKPICardXML produces a single KPI metric card as a roundRect shape
// with hero value, label, and delta/trend paragraphs.
func generateKPICardXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32) string {
	var paras []pptx.Paragraph

	// Hero value paragraph — large, bold, accent-colored, centered.
	if panel.value != "" {
		paras = append(paras, pptx.Paragraph{
			Align:      "ctr",
			NoBullet:   true,
			SpaceAfter: kpiSpaceAfterValue,
			Runs: []pptx.Run{{
				Text:     panel.value,
				Lang:     "en-US",
				FontSize: kpiValueFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    pptx.SchemeFill("accent1"),
			}},
		})
	}

	// Label paragraph — uppercase, small, bold, dk1.
	if panel.title != "" {
		paras = append(paras, pptx.Paragraph{
			Align:      "ctr",
			NoBullet:   true,
			SpaceAfter: kpiSpaceAfterLabel,
			Runs: []pptx.Run{{
				Text:     strings.ToUpper(panel.title),
				Lang:     "en-US",
				FontSize: kpiLabelFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    pptx.SchemeFill("dk1"),
			}},
		})
	}

	// Delta/trend paragraph — colored by trend direction.
	if panel.body != "" {
		deltaRun := pptx.Run{
			Text:     panel.body,
			Lang:     "en-US",
			FontSize: kpiDeltaFontSize,
			Dirty:    true,
		}
		// Color based on trend: body starts with ▲ for up, ▼ for down.
		switch {
		case strings.HasPrefix(panel.body, "\u25B2"): // ▲ up
			deltaRun.Color = pptx.SchemeFill("accent6") // green-ish
		case strings.HasPrefix(panel.body, "\u25BC"): // ▼ down
			deltaRun.Color = pptx.SchemeFill("accent2") // red-ish
		default:
			deltaRun.Color = pptx.SchemeFill("dk1", pptx.LumMod(50000), pptx.LumOff(50000))
		}
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs:     []pptx.Run{deltaRun},
		})
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "KPI Card",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: kpiCornerRadius},
		},
		Fill: pptx.SchemeFill(panelHeaderFillSchemeColor, pptx.LumMod(panelHeaderFillLumMod), pptx.LumOff(panelHeaderFillLumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{kpiInset, kpiInset, kpiInset, kpiInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateKPICardXML failed", "error", err)
		return ""
	}
	return string(b)
}

// kpiGridLayout calculates columns and rows for n KPI metric cards.
// Uses the same adaptive algorithm as stat cards.
func kpiGridLayout(n int) (cols, rows int) {
	if n <= kpiMaxCols {
		return n, 1
	}
	cols = kpiMaxCols
	if n <= 6 {
		cols = 3
	}
	if n <= 4 {
		cols = 2
	}
	rows = (n + cols - 1) / cols
	return cols, rows
}

// parseKPIMetrics extracts KPI metric data from diagram data map.
// Accepts "metrics" or "kpis" as the array key.
func parseKPIMetrics(data map[string]any) []kpiMetric {
	metricsRaw, ok := data["metrics"].([]any)
	if !ok {
		metricsRaw, ok = data["kpis"].([]any)
	}
	if !ok {
		return nil
	}

	var metrics []kpiMetric
	for _, item := range metricsRaw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		metric := kpiMetric{}
		if label, ok := m["label"].(string); ok {
			metric.label = label
		}
		if value, ok := m["value"].(string); ok {
			metric.value = value
		} else if value, ok := m["value"].(float64); ok {
			metric.value = fmt.Sprintf("%.0f", value)
		}
		if delta, ok := m["delta"].(string); ok {
			metric.delta = delta
		}
		if trend, ok := m["trend"].(string); ok {
			metric.trend = trend
		}

		metrics = append(metrics, metric)
	}

	return metrics
}

// buildKPIDeltaText builds the delta display text with trend arrow prefix.
func buildKPIDeltaText(delta, trend string) string {
	if delta == "" && trend == "" {
		return ""
	}

	var arrow string
	switch strings.ToLower(trend) {
	case "up":
		arrow = "\u25B2 " // ▲
	case "down":
		arrow = "\u25BC " // ▼
	case "flat":
		arrow = "\u2192 " // →
	}

	if delta == "" {
		return strings.TrimSpace(arrow)
	}
	return arrow + delta
}

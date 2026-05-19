package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestChartInsightsSplit(t *testing.T) {
	p := &chartInsightsSplit{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "chart-insights-split" {
			t.Errorf("Name() = %q, want %q", p.Name(), "chart-insights-split")
		}
		if p.Version() != 1 {
			t.Errorf("Version() = %d, want 1", p.Version())
		}
		if p.CellsHint() != "1 + 1" {
			t.Errorf("CellsHint() = %q, want %q", p.CellsHint(), "1 + 1")
		}
	})

	t.Run("registered", func(t *testing.T) {
		pat, ok := Default().Get("chart-insights-split")
		if !ok {
			t.Fatal("chart-insights-split not in default registry")
		}
		if pat.Name() != "chart-insights-split" {
			t.Errorf("registry returned wrong pattern: %q", pat.Name())
		}
	})

	t.Run("schema_is_draft_2020_12", func(t *testing.T) {
		s := p.Schema()
		if s == nil {
			t.Fatal("Schema() returned nil")
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatalf("Schema marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Schema unmarshal: %v", err)
		}
		if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("missing $schema draft 2020-12")
		}
		if m["type"] != "object" {
			t.Errorf("root type = %v, want object", m["type"])
		}
	})

	t.Run("validate_happy_path", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"categories": []any{"A", "B"},
					"series":     []any{map[string]any{"name": "x", "values": []any{1, 2}}},
				},
			},
			Insights: []string{"First insight.", "Second insight."},
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_no_chart_is_ok", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Insights: []string{"Only insight."},
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_missing_insights", func(t *testing.T) {
		v := &ChartInsightsSplitValues{}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error for missing insights, got nil")
		}
		if !strings.Contains(err.Error(), "values.insights is required") {
			t.Errorf("error %q does not mention values.insights required", err)
		}
	})

	t.Run("validate_too_many_insights", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Insights: []string{"1", "2", "3", "4", "5", "6", "7"},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error for >6 insights, got nil")
		}
		if !strings.Contains(err.Error(), "at most 6") {
			t.Errorf("error %q does not mention max items", err)
		}
	})

	t.Run("validate_chart_without_type", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Data: map[string]any{"categories": []any{"A"}},
			},
			Insights: []string{"x"},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error for chart without type, got nil")
		}
		if !strings.Contains(err.Error(), "values.chart.type") {
			t.Errorf("error %q does not mention values.chart.type", err)
		}
	})

	t.Run("validate_chart_without_data", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart:    &types.DiagramSpec{Type: "bar_chart"},
			Insights: []string{"x"},
		}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error for chart without data, got nil")
		}
		if !strings.Contains(err.Error(), "values.chart.data") {
			t.Errorf("error %q does not mention values.chart.data", err)
		}
	})

	t.Run("expand_with_chart_renders_split", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"categories": []any{"A", "B"},
					"series":     []any{map[string]any{"name": "x", "values": []any{1, 2}}},
				},
			},
			Insights: []string{"Insight 1", "Insight 2"},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row when no source, got %d", len(grid.Rows))
		}
		if len(grid.Rows[0].Cells) != 2 {
			t.Fatalf("expected 2 cells (chart + insights), got %d", len(grid.Rows[0].Cells))
		}
		// Cell 0 should be a Diagram cell.
		if grid.Rows[0].Cells[0].Diagram == nil {
			t.Error("expected first cell to be a Diagram cell")
		}
		// Cell 1 should be a Shape cell carrying the insights text.
		if grid.Rows[0].Cells[1].Shape == nil {
			t.Fatal("expected second cell to be a Shape cell")
		}
		text := string(grid.Rows[0].Cells[1].Shape.Text)
		if !strings.Contains(text, "Insight 1") || !strings.Contains(text, "Insight 2") {
			t.Errorf("insights cell text missing bullets: %s", text)
		}
		if !strings.Contains(text, "Key Insights") {
			t.Errorf("expected default title 'Key Insights', got: %s", text)
		}
		// Divider on by default.
		if grid.Rows[0].Cells[1].AccentBar == nil {
			t.Error("expected divider accent bar on insights cell by default")
		}
	})

	t.Run("expand_without_chart_renders_full_width", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Insights: []string{"Only insight."},
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(grid.Rows))
		}
		if len(grid.Rows[0].Cells) != 1 {
			t.Fatalf("expected 1 cell (insights only), got %d", len(grid.Rows[0].Cells))
		}
		if grid.Rows[0].Cells[0].Diagram != nil {
			t.Error("expected no diagram cell when chart is absent")
		}
		if grid.Rows[0].Cells[0].Shape == nil {
			t.Fatal("expected insights shape cell")
		}
		// Columns should be 1 (full width).
		var cols any
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if n, ok := cols.(float64); !ok || n != 1 {
			t.Errorf("expected columns=1, got %v", cols)
		}
	})

	t.Run("expand_source_adds_row", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "x", "values": []any{1}}}},
			},
			Insights: []string{"i1"},
			Source:   "Source: ACME (2025)",
		}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if len(grid.Rows) != 2 {
			t.Fatalf("expected 2 rows (split + source), got %d", len(grid.Rows))
		}
		sourceText := string(grid.Rows[1].Cells[0].Shape.Text)
		if !strings.Contains(sourceText, "ACME") {
			t.Errorf("expected source text to contain attribution, got: %s", sourceText)
		}
	})

	t.Run("expand_columns_use_chart_width_pct", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "x", "values": []any{1}}}},
			},
			Insights: []string{"x"},
		}
		ovr := &ChartInsightsSplitOverrides{ChartWidthPct: 70}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if len(cols) != 2 {
			t.Fatalf("expected 2-column ratio, got %v", cols)
		}
		if cols[0] != 70 || cols[1] != 30 {
			t.Errorf("expected [70, 30], got %v", cols)
		}
	})

	t.Run("expand_chart_width_pct_clamped", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "x", "values": []any{1}}}},
			},
			Insights: []string{"x"},
		}
		ovr := &ChartInsightsSplitOverrides{ChartWidthPct: 95} // above 80 → clamped
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("columns unmarshal: %v", err)
		}
		if cols[0] != 80 {
			t.Errorf("expected chart width clamped to 80, got %v", cols[0])
		}
	})

	t.Run("expand_show_divider_false_omits_bar", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "x", "values": []any{1}}}},
			},
			Insights: []string{"x"},
		}
		off := false
		ovr := &ChartInsightsSplitOverrides{ShowDivider: &off}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid.Rows[0].Cells[1].AccentBar != nil {
			t.Error("expected no accent divider when show_divider=false")
		}
	})

	t.Run("post_expand_warnings_emits_chart_placeholder_empty", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Insights: []string{"x"},
		}
		ws := p.PostExpandWarnings(ExpandContext{}, v, nil)
		if len(ws) != 1 {
			t.Fatalf("expected 1 warning, got %d: %v", len(ws), ws)
		}
		if !strings.HasPrefix(ws[0], ErrCodeChartPlaceholderEmpty+":") {
			t.Errorf("warning does not carry expected prefix: %q", ws[0])
		}
	})

	t.Run("post_expand_warnings_no_warning_when_chart_present", func(t *testing.T) {
		v := &ChartInsightsSplitValues{
			Chart: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "x", "values": []any{1}}}},
			},
			Insights: []string{"x"},
		}
		ws := p.PostExpandWarnings(ExpandContext{}, v, nil)
		if len(ws) != 0 {
			t.Errorf("expected no warnings when chart present, got %d: %v", len(ws), ws)
		}
	})
}

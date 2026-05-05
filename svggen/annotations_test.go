package svggen

import (
	"math"
	"strings"
	"testing"
)

func TestLinearRegression(t *testing.T) {
	tests := []struct {
		name      string
		values    []float64
		wantSlope float64
		wantIntercept float64
	}{
		{
			name:          "perfect ascending",
			values:        []float64{0, 1, 2, 3, 4},
			wantSlope:     1.0,
			wantIntercept: 0.0,
		},
		{
			name:          "perfect descending",
			values:        []float64{4, 3, 2, 1, 0},
			wantSlope:     -1.0,
			wantIntercept: 4.0,
		},
		{
			name:          "flat line",
			values:        []float64{5, 5, 5, 5},
			wantSlope:     0.0,
			wantIntercept: 5.0,
		},
		{
			name:          "single value",
			values:        []float64{42},
			wantSlope:     0.0,
			wantIntercept: 42.0,
		},
		{
			name:          "two values",
			values:        []float64{10, 20},
			wantSlope:     10.0,
			wantIntercept: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slope, intercept := linearRegression(tt.values)
			if math.Abs(slope-tt.wantSlope) > 1e-9 {
				t.Errorf("slope = %f, want %f", slope, tt.wantSlope)
			}
			if math.Abs(intercept-tt.wantIntercept) > 1e-9 {
				t.Errorf("intercept = %f, want %f", intercept, tt.wantIntercept)
			}
		})
	}
}

func TestIsPeak(t *testing.T) {
	values := []float64{1, 3, 2, 5, 1}

	// Index 0 and 4 are always peaks (endpoints)
	if !isPeak(0, values) {
		t.Error("index 0 should be a peak (endpoint)")
	}
	if !isPeak(4, values) {
		t.Error("index 4 should be a peak (endpoint)")
	}
	// Index 1: local max (1 < 3 > 2)
	if !isPeak(1, values) {
		t.Error("index 1 should be a peak (local max)")
	}
	// Index 2: local min (3 > 2 < 5)
	if !isPeak(2, values) {
		t.Error("index 2 should be a peak (local min)")
	}
	// Index 3: local max (2 < 5 > 1)
	if !isPeak(3, values) {
		t.Error("index 3 should be a peak (local max)")
	}
}

func TestDataLabelConfig_FormatValue(t *testing.T) {
	tests := []struct {
		format string
		value  float64
		want   string
	}{
		{"", 42.7, "43"},
		{"$%.1fM", 1.234, "$1.2M"},
		{"%.0f%%", 85.0, "85%"},
	}

	for _, tt := range tests {
		dlc := DataLabelConfig{Format: tt.format}
		got := dlc.FormatValue(tt.value)
		if got != tt.want {
			t.Errorf("FormatValue(%f) with format %q = %q, want %q", tt.value, tt.format, got, tt.want)
		}
	}
}

func TestDataLabelConfig_ShouldShow(t *testing.T) {
	values := []float64{10, 20, 30, 25, 15}

	tests := []struct {
		showOn string
		index  int
		want   bool
	}{
		{"all", 0, true},
		{"all", 2, true},
		{"last", 0, false},
		{"last", 4, true},
		{"first_last", 0, true},
		{"first_last", 2, false},
		{"first_last", 4, true},
		{"peaks", 0, true},  // endpoint
		{"peaks", 2, true},  // local max
		{"peaks", 1, false}, // not a peak (10 < 20 < 30)
	}

	for _, tt := range tests {
		dlc := DataLabelConfig{ShowOn: tt.showOn}
		got := dlc.ShouldShow(tt.index, len(values), values)
		if got != tt.want {
			t.Errorf("ShouldShow(index=%d, showOn=%q) = %v, want %v", tt.index, tt.showOn, got, tt.want)
		}
	}
}

func TestDrawAnnotations_ReferenceLine(t *testing.T) {
	builder := NewSVGBuilder(800, 600)
	style := DefaultStyleGuide()
	builder.SetStyleGuide(style)

	plotArea := Rect{X: 60, Y: 40, W: 700, H: 500}

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(plotArea.H, 0)

	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)

	data := ChartData{
		Categories: categories,
		Series: []ChartSeries{
			{Name: "Revenue", Values: []float64{50, 70, 80, 90}},
		},
	}

	annotations := []Annotation{
		{Kind: AnnotationReferenceLine, Axis: "y", Value: 95, Label: "Target", Style: "dashed"},
	}

	DrawAnnotations(builder, annotations, plotArea, xScale, yScale, data, nil)

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(string(svg.Content), "Target") {
		t.Error("SVG should contain reference line label 'Target'")
	}
}

func TestDrawAnnotations_Trendline(t *testing.T) {
	builder := NewSVGBuilder(800, 600)
	style := DefaultStyleGuide()
	builder.SetStyleGuide(style)

	plotArea := Rect{X: 60, Y: 40, W: 700, H: 500}

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(plotArea.H, 0)

	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)

	data := ChartData{
		Categories: categories,
		Series: []ChartSeries{
			{Name: "Revenue", Values: []float64{50, 60, 70, 80}},
		},
	}

	colors := []Color{MustParseColor("#4E79A7")}
	annotations := []Annotation{
		{Kind: AnnotationTrendline, Series: "Revenue", Method: "linear", Label: "Trend"},
	}

	DrawAnnotations(builder, annotations, plotArea, xScale, yScale, data, colors)

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(string(svg.Content), "Trend") {
		t.Error("SVG should contain trendline label 'Trend'")
	}
}

func TestDrawAnnotations_Callout(t *testing.T) {
	builder := NewSVGBuilder(800, 600)
	style := DefaultStyleGuide()
	builder.SetStyleGuide(style)

	plotArea := Rect{X: 60, Y: 40, W: 700, H: 500}

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(plotArea.H, 0)

	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)

	data := ChartData{
		Categories: categories,
		Series: []ChartSeries{
			{Name: "Revenue", Values: []float64{50, 60, 70, 80}},
		},
	}

	annotations := []Annotation{
		{Kind: AnnotationCallout, X: 2, Y: 70, Text: "Launch of product X"},
	}

	DrawAnnotations(builder, annotations, plotArea, xScale, yScale, data, nil)

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !strings.Contains(string(svg.Content), "Launch of product X") {
		t.Error("SVG should contain callout text 'Launch of product X'")
	}
}

func TestExtractAnnotations(t *testing.T) {
	data := map[string]any{
		"annotations": []any{
			map[string]any{
				"kind":  "reference_line",
				"axis":  "y",
				"value": float64(95),
				"label": "Target",
				"style": "dashed",
			},
			map[string]any{
				"kind":   "trendline",
				"series": "Revenue",
				"method": "linear",
			},
			map[string]any{
				"kind": "callout",
				"x":    float64(4),
				"y":    float64(120),
				"text": "Launch of product X",
			},
		},
	}

	annotations := extractAnnotations(data)
	if len(annotations) != 3 {
		t.Fatalf("expected 3 annotations, got %d", len(annotations))
	}

	// Reference line
	if annotations[0].Kind != AnnotationReferenceLine {
		t.Errorf("ann[0].Kind = %q, want reference_line", annotations[0].Kind)
	}
	if annotations[0].Value != 95 {
		t.Errorf("ann[0].Value = %f, want 95", annotations[0].Value)
	}
	if annotations[0].Label != "Target" {
		t.Errorf("ann[0].Label = %q, want Target", annotations[0].Label)
	}

	// Trendline
	if annotations[1].Kind != AnnotationTrendline {
		t.Errorf("ann[1].Kind = %q, want trendline", annotations[1].Kind)
	}
	if annotations[1].Series != "Revenue" {
		t.Errorf("ann[1].Series = %q, want Revenue", annotations[1].Series)
	}

	// Callout
	if annotations[2].Kind != AnnotationCallout {
		t.Errorf("ann[2].Kind = %q, want callout", annotations[2].Kind)
	}
	if annotations[2].Text != "Launch of product X" {
		t.Errorf("ann[2].Text = %q, want 'Launch of product X'", annotations[2].Text)
	}
}

func TestExtractDataLabels(t *testing.T) {
	data := map[string]any{
		"data_labels": map[string]any{
			"format":  "$%.1fM",
			"show_on": "last",
		},
	}

	dlc := extractDataLabels(data)
	if dlc == nil {
		t.Fatal("expected non-nil DataLabelConfig")
	}
	if dlc.Format != "$%.1fM" {
		t.Errorf("Format = %q, want $%%.1fM", dlc.Format)
	}
	if dlc.ShowOn != "last" {
		t.Errorf("ShowOn = %q, want last", dlc.ShowOn)
	}
}

func TestExtractDataLabels_Missing(t *testing.T) {
	data := map[string]any{}
	dlc := extractDataLabels(data)
	if dlc != nil {
		t.Error("expected nil DataLabelConfig when no data_labels in data")
	}
}

func TestColorblindSafePalette(t *testing.T) {
	p := ColorblindSafePalette()
	if p.Name != "colorblind-safe" {
		t.Errorf("Name = %q, want colorblind-safe", p.Name)
	}
	accents := p.AccentColors()
	if len(accents) != 6 {
		t.Errorf("expected 6 accent colors, got %d", len(accents))
	}
}

func TestGetPaletteByName_Colorblind(t *testing.T) {
	for _, name := range []string{"colorblind-safe", "colorblind", "cb-safe"} {
		p := GetPaletteByName(name)
		if p.Name != "colorblind-safe" {
			t.Errorf("GetPaletteByName(%q).Name = %q, want colorblind-safe", name, p.Name)
		}
	}
}

func TestApplyDataLabelsToConfig(t *testing.T) {
	config := DefaultChartConfig(800, 600)

	// nil should be a no-op
	applyDataLabelsToConfig(&config, nil)
	if config.ShowValues {
		t.Error("nil DataLabelConfig should not enable ShowValues")
	}

	dlc := &DataLabelConfig{Format: "$%.1fM"}
	applyDataLabelsToConfig(&config, dlc)
	if !config.ShowValues {
		t.Error("DataLabelConfig should enable ShowValues")
	}
	if config.ValueFormat != "$%.1fM" {
		t.Errorf("ValueFormat = %q, want $%%.1fM", config.ValueFormat)
	}
}

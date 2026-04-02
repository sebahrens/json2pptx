package svggen

import (
	"strings"
	"testing"
)

func TestBarChartDiagram(t *testing.T) {
	d := &BarChartDiagram{NewBaseDiagram("bar_chart")}

	t.Run("Type", func(t *testing.T) {
		if got := d.Type(); got != "bar_chart" {
			t.Errorf("Type() = %q, want bar_chart", got)
		}
	})

	t.Run("Validate with valid data", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "bar_chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series": []any{
					map[string]any{
						"name":   "Sales",
						"values": []any{10.0, 20.0, 15.0},
					},
				},
			},
		}
		if err := d.Validate(req); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Validate missing categories", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "bar_chart",
			Data: map[string]any{
				"series": []any{
					map[string]any{"name": "Sales", "values": []any{10.0}},
				},
			},
		}
		if err := d.Validate(req); err == nil {
			t.Error("Validate() expected error for missing categories")
		}
	})

	t.Run("Validate missing series", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "bar_chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
			},
		}
		if err := d.Validate(req); err == nil {
			t.Error("Validate() expected error for missing series")
		}
	})

	t.Run("Render bar chart", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "bar_chart",
			Title: "Test Bar Chart",
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{
						"name":   "Revenue",
						"values": []any{100.0, 150.0, 120.0, 180.0},
					},
				},
			},
			Output: OutputSpec{
				Width:  800,
				Height: 600,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
		if len(doc.Content) == 0 {
			t.Error("Render() returned empty content")
		}
		svg := string(doc.Content)
		if !strings.Contains(svg, "<svg") {
			t.Error("Output missing <svg tag")
		}
	})
}

func TestLineChartDiagram(t *testing.T) {
	d := &LineChartDiagram{NewBaseDiagram("line_chart")}

	t.Run("Type", func(t *testing.T) {
		if got := d.Type(); got != "line_chart" {
			t.Errorf("Type() = %q, want line_chart", got)
		}
	})

	t.Run("Render line chart", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "line_chart",
			Title: "Monthly Trend",
			Data: map[string]any{
				"categories": []any{"Jan", "Feb", "Mar", "Apr"},
				"series": []any{
					map[string]any{
						"name":   "Sales",
						"values": []any{100.0, 120.0, 110.0, 130.0},
					},
				},
			},
			Output: OutputSpec{
				Width:  600,
				Height: 400,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
		svg := string(doc.Content)
		if !strings.Contains(svg, "<svg") {
			t.Error("Output missing <svg tag")
		}
	})
}

func TestPieChartDiagram(t *testing.T) {
	d := &PieChartDiagram{NewBaseDiagram("pie_chart")}

	t.Run("Type", func(t *testing.T) {
		if got := d.Type(); got != "pie_chart" {
			t.Errorf("Type() = %q, want pie_chart", got)
		}
	})

	t.Run("Validate with valid data", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Data: map[string]any{
				"categories": []any{"Desktop", "Mobile", "Tablet"},
				"values":     []any{60.0, 30.0, 10.0},
			},
		}
		if err := d.Validate(req); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("Validate missing values", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
			},
		}
		if err := d.Validate(req); err == nil {
			t.Error("Validate() expected error for missing values")
		}
	})

	t.Run("Render pie chart", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "pie_chart",
			Title: "Market Share",
			Data: map[string]any{
				"categories": []any{"Chrome", "Firefox", "Safari", "Edge"},
				"values":     []any{65.0, 10.0, 15.0, 10.0},
			},
			Output: OutputSpec{
				Width:  500,
				Height: 500,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
		svg := string(doc.Content)
		if !strings.Contains(svg, "<svg") {
			t.Error("Output missing <svg tag")
		}
	})
}

func TestDonutChartDiagram(t *testing.T) {
	d := &DonutChartDiagram{NewBaseDiagram("donut_chart")}

	t.Run("Type", func(t *testing.T) {
		if got := d.Type(); got != "donut_chart" {
			t.Errorf("Type() = %q, want donut_chart", got)
		}
	})

	t.Run("Render donut chart", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "donut_chart",
			Title: "Disk Usage",
			Data: map[string]any{
				"categories": []any{"Used", "Free"},
				"values":     []any{75.0, 25.0},
			},
			Output: OutputSpec{
				Width:  400,
				Height: 400,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
		svg := string(doc.Content)
		if !strings.Contains(svg, "<svg") {
			t.Error("Output missing <svg tag")
		}
	})
}

func TestChartDiagramsRegistration(t *testing.T) {
	// Verify that chart diagrams are registered
	types := Types()

	expected := []string{"bar_chart", "line_chart", "pie_chart", "donut_chart"}
	for _, exp := range expected {
		found := false
		for _, typ := range types {
			if typ == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Diagram type %q not registered", exp)
		}
	}
}

func TestRenderViaRegistry(t *testing.T) {
	// Test rendering through the default registry
	req := &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Registry Test",
		Data: map[string]any{
			"categories": []any{"A", "B"},
			"series": []any{
				map[string]any{
					"name":   "Data",
					"values": []any{10.0, 20.0},
				},
			},
		},
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("Render() via registry error = %v", err)
	}
	if doc == nil {
		t.Fatal("Render() via registry returned nil")
	}
	if len(doc.Content) == 0 {
		t.Error("Render() via registry returned empty content")
	}
}

func TestRenderMultiFormatViaRegistry(t *testing.T) {
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"X", "Y"},
			"series": []any{
				map[string]any{
					"name":   "Series",
					"values": []any{5.0, 10.0},
				},
			},
		},
		Output: OutputSpec{
			Width:  400,
			Height: 300,
		},
	}

	result, err := RenderMultiFormat(req, "svg", "png")
	if err != nil {
		t.Fatalf("RenderMultiFormat() error = %v", err)
	}
	if result == nil {
		t.Fatal("RenderMultiFormat() returned nil")
	}
	if result.SVG == nil {
		t.Error("SVG output missing")
	}
	if len(result.PNG) == 0 {
		t.Error("PNG output missing")
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   []string
		wantOK bool
	}{
		{
			name:   "[]string",
			input:  []string{"a", "b"},
			want:   []string{"a", "b"},
			wantOK: true,
		},
		{
			name:   "[]any with strings",
			input:  []any{"a", "b", "c"},
			want:   []string{"a", "b", "c"},
			wantOK: true,
		},
		{
			name:   "[]any with non-strings",
			input:  []any{"a", 123},
			want:   nil,
			wantOK: false,
		},
		{
			name:   "non-slice",
			input:  "not a slice",
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toStringSlice(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toStringSlice() ok = %v, wantOK %v", ok, tt.wantOK)
			}
			if tt.wantOK && len(got) != len(tt.want) {
				t.Errorf("toStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToFloat64Slice(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   []float64
		wantOK bool
	}{
		{
			name:   "[]float64",
			input:  []float64{1.0, 2.0},
			want:   []float64{1.0, 2.0},
			wantOK: true,
		},
		{
			name:   "[]any with floats",
			input:  []any{1.0, 2.0, 3.0},
			want:   []float64{1.0, 2.0, 3.0},
			wantOK: true,
		},
		{
			name:   "[]any with ints",
			input:  []any{1, 2, 3},
			want:   []float64{1.0, 2.0, 3.0},
			wantOK: true,
		},
		{
			name:   "[]int",
			input:  []int{1, 2, 3},
			want:   []float64{1.0, 2.0, 3.0},
			wantOK: true,
		},
		{
			name:   "[]any with mixed",
			input:  []any{1.0, "not a number"},
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64Slice(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toFloat64Slice() ok = %v, wantOK %v", ok, tt.wantOK)
			}
			if tt.wantOK && len(got) != len(tt.want) {
				t.Errorf("toFloat64Slice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractChartData(t *testing.T) {
	t.Run("extract bar chart data", func(t *testing.T) {
		req := &RequestEnvelope{
			Title: "Test",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series": []any{
					map[string]any{
						"name":   "Series 1",
						"values": []any{10.0, 20.0, 30.0},
					},
				},
			},
		}

		data, err := extractChartData(req)
		if err != nil {
			t.Fatalf("extractChartData() error = %v", err)
		}
		if data.Title != "Test" {
			t.Errorf("Title = %q, want Test", data.Title)
		}
		if len(data.Categories) != 3 {
			t.Errorf("len(Categories) = %d, want 3", len(data.Categories))
		}
		if len(data.Series) != 1 {
			t.Errorf("len(Series) = %d, want 1", len(data.Series))
		}
		if data.Series[0].Name != "Series 1" {
			t.Errorf("Series[0].Name = %q, want Series 1", data.Series[0].Name)
		}
		if len(data.Series[0].Values) != 3 {
			t.Errorf("len(Series[0].Values) = %d, want 3", len(data.Series[0].Values))
		}
	})
}

func TestExtractPieChartData(t *testing.T) {
	t.Run("extract pie chart data", func(t *testing.T) {
		req := &RequestEnvelope{
			Title: "Pie Chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"values":     []any{60.0, 40.0},
			},
		}

		data, err := extractPieChartData(req)
		if err != nil {
			t.Fatalf("extractPieChartData() error = %v", err)
		}
		if data.Title != "Pie Chart" {
			t.Errorf("Title = %q, want Pie Chart", data.Title)
		}
		if len(data.Categories) != 2 {
			t.Errorf("len(Categories) = %d, want 2", len(data.Categories))
		}
		if len(data.Series) != 1 {
			t.Errorf("len(Series) = %d, want 1", len(data.Series))
		}
		if len(data.Series[0].Values) != 2 {
			t.Errorf("len(Series[0].Values) = %d, want 2", len(data.Series[0].Values))
		}
	})
}

func TestExtractPalette(t *testing.T) {
	t.Run("nil palette returns nil", func(t *testing.T) {
		colors := extractPalette(nil)
		if colors != nil {
			t.Errorf("extractPalette(nil) = %v, want nil", colors)
		}
	})

	t.Run("string slice of hex colors", func(t *testing.T) {
		colors := extractPalette([]any{"#FF0000", "#00FF00", "#0000FF"})
		if len(colors) != 3 {
			t.Errorf("len(colors) = %d, want 3", len(colors))
		}
		if colors[0].Hex() != "#FF0000" {
			t.Errorf("colors[0] = %q, want #FF0000", colors[0].Hex())
		}
	})

	t.Run("named palette corporate", func(t *testing.T) {
		colors := extractPalette("corporate")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Corporate palette first accent is #4E79A7 (blue)
		if colors[0].Hex() != "#4E79A7" {
			t.Errorf("colors[0] = %q, want #4E79A7", colors[0].Hex())
		}
	})

	t.Run("named palette vibrant", func(t *testing.T) {
		colors := extractPalette("vibrant")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Vibrant palette first accent is #FF6B6B (coral)
		if colors[0].Hex() != "#FF6B6B" {
			t.Errorf("colors[0] = %q, want #FF6B6B", colors[0].Hex())
		}
	})

	t.Run("named palette muted", func(t *testing.T) {
		colors := extractPalette("muted")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Muted palette first accent is #8B9DC3 (soft blue)
		if colors[0].Hex() != "#8B9DC3" {
			t.Errorf("colors[0] = %q, want #8B9DC3", colors[0].Hex())
		}
	})

	t.Run("named palette monochrome", func(t *testing.T) {
		colors := extractPalette("monochrome")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Monochrome palette first accent is #1A1A1A
		if colors[0].Hex() != "#1A1A1A" {
			t.Errorf("colors[0] = %q, want #1A1A1A", colors[0].Hex())
		}
	})

	t.Run("named palette case insensitive", func(t *testing.T) {
		colors := extractPalette("VIBRANT")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Should still work regardless of case
		if colors[0].Hex() != "#FF6B6B" {
			t.Errorf("colors[0] = %q, want #FF6B6B", colors[0].Hex())
		}
	})

	t.Run("unknown palette falls back to default", func(t *testing.T) {
		colors := extractPalette("nonexistent")
		if len(colors) != 6 {
			t.Errorf("len(colors) = %d, want 6", len(colors))
		}
		// Unknown palette falls back to corporate (default)
		if colors[0].Hex() != "#4E79A7" {
			t.Errorf("colors[0] = %q, want #4E79A7", colors[0].Hex())
		}
	})

	t.Run("unsupported type returns nil", func(t *testing.T) {
		colors := extractPalette(12345)
		if colors != nil {
			t.Errorf("extractPalette(int) = %v, want nil", colors)
		}
	})
}

func TestShowGridConfiguration(t *testing.T) {
	// Grid lines default to ON for all axis-based charts (professional dashboard standard).
	// The grid uses solid lines with the grid color (#E0E0E0) for a clean look.
	tests := []struct {
		name    string
		diagram Diagram
	}{
		{"bar_chart_default_grid", &BarChartDiagram{NewBaseDiagram("bar_chart")}},
		{"line_chart_default_grid", &LineChartDiagram{NewBaseDiagram("line_chart")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  tt.diagram.Type(),
				Title: "Test Chart",
				Data: map[string]any{
					"categories": []any{"A", "B", "C"},
					"series": []any{
						map[string]any{
							"name":   "Series 1",
							"values": []any{10.0, 20.0, 30.0},
						},
					},
				},
				Style: StyleSpec{
					ShowLegend: true,
				},
				Output: OutputSpec{
					Width:  800,
					Height: 600,
				},
			}

			doc, err := tt.diagram.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			svg := string(doc.Content)

			// Grid lines are drawn as solid horizontal lines with the grid color.
			// Check for the grid color (#E0E0E0) which identifies grid lines.
			gridColor := strings.ToLower("E0E0E0")
			hasGridLines := strings.Contains(strings.ToLower(svg), gridColor)

			if !hasGridLines {
				t.Errorf("Expected grid lines by default, but SVG doesn't contain grid color %s", gridColor)
			}
		})
	}
}

func TestGetOutputDimensions(t *testing.T) {
	t.Run("defaults to 800x600", func(t *testing.T) {
		req := &RequestEnvelope{}
		w, h := getOutputDimensions(req)
		if w != 800 || h != 600 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (800, 600)", w, h)
		}
	})

	t.Run("explicit dimensions override defaults", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Width:  1024,
				Height: 768,
			},
		}
		w, h := getOutputDimensions(req)
		if w != 1024 || h != 768 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (1024, 768)", w, h)
		}
	})

	t.Run("preset slide_16x9 returns 1920x1080", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "slide_16x9",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 1920 || h != 1080 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (1920, 1080)", w, h)
		}
	})

	t.Run("preset content_16x9 returns 1600x900", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "content_16x9",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 1600 || h != 900 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (1600, 900)", w, h)
		}
	})

	t.Run("preset half_16x9 returns 760x720", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "half_16x9",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 760 || h != 720 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (760, 720)", w, h)
		}
	})

	t.Run("preset square returns 600x600", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "square",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 600 || h != 600 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (600, 600)", w, h)
		}
	})

	t.Run("preset thumbnail returns 400x300", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "thumbnail",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 400 || h != 300 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (400, 300)", w, h)
		}
	})

	t.Run("explicit dimensions override preset", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "slide_16x9",
				Width:  1024,
				Height: 768,
			},
		}
		w, h := getOutputDimensions(req)
		if w != 1024 || h != 768 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (1024, 768)", w, h)
		}
	})

	t.Run("partial explicit dimensions override preset partially", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "slide_16x9", // 1920x1080
				Width:  1024,         // Only width overrides
			},
		}
		w, h := getOutputDimensions(req)
		if w != 1024 || h != 1080 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (1024, 1080)", w, h)
		}
	})

	t.Run("invalid preset falls back to defaults", func(t *testing.T) {
		req := &RequestEnvelope{
			Output: OutputSpec{
				Preset: "nonexistent_preset",
			},
		}
		w, h := getOutputDimensions(req)
		if w != 800 || h != 600 {
			t.Errorf("getOutputDimensions() = (%v, %v), want (800, 600) for invalid preset", w, h)
		}
	})

	t.Run("all presets", func(t *testing.T) {
		tests := []struct {
			preset string
			width  float64
			height float64
		}{
			{"slide_16x9", 1920, 1080},
			{"content_16x9", 1600, 900},
			{"half_16x9", 760, 720},
			{"third_16x9", 500, 720},
			{"slide_4x3", 1024, 768},
			{"half_4x3", 420, 540},
			{"square", 600, 600},
			{"thumbnail", 400, 300},
		}

		for _, tt := range tests {
			t.Run(tt.preset, func(t *testing.T) {
				req := &RequestEnvelope{
					Output: OutputSpec{
						Preset: tt.preset,
					},
				}
				w, h := getOutputDimensions(req)
				if w != tt.width || h != tt.height {
					t.Errorf("getOutputDimensions() = (%v, %v), want (%v, %v)", w, h, tt.width, tt.height)
				}
			})
		}
	})
}

// TestFitMode tests the FitMode functionality for aspect ratio preservation.
func TestFitMode(t *testing.T) {
	t.Run("applyFitMode with stretch (default)", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Output: OutputSpec{
				Width:   800,
				Height:  400,
				FitMode: "stretch",
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 800, 400)
		// Stretch mode should use container dimensions directly
		if contentW != 800 || contentH != 400 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 800, 400", contentW, contentH)
		}
		if offsetX != 0 || offsetY != 0 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 0", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode with contain for pie chart uses full container", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Output: OutputSpec{
				Width:   800,
				Height:  400,
				FitMode: "contain",
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 800, 400)
		// Circular charts use container ratio as source (no 1:1 forcing),
		// so contain mode fills the full container.
		if contentW != 800 || contentH != 400 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 800, 400", contentW, contentH)
		}
		if offsetX != 0 || offsetY != 0 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 0", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode with contain for donut chart uses full container", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "donut_chart",
			Output: OutputSpec{
				Width:   400,
				Height:  800,
				FitMode: "contain",
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 400, 800)
		// Circular charts use container ratio as source, filling the full container.
		if contentW != 400 || contentH != 800 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 400, 800", contentW, contentH)
		}
		if offsetX != 0 || offsetY != 0 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 0", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode with contain for radar chart uses full container", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "radar_chart",
			Output: OutputSpec{
				Width:   1000,
				Height:  500,
				FitMode: "contain",
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 1000, 500)
		// Radar charts use container ratio as source, filling the full container.
		if contentW != 1000 || contentH != 500 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 1000, 500", contentW, contentH)
		}
		if offsetX != 0 || offsetY != 0 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 0", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode with custom aspect ratio", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "bar_chart",
			Output: OutputSpec{
				Width:       800,
				Height:      600,
				FitMode:     "contain",
				AspectRatio: 2.0, // 2:1 ratio (wide content)
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 800, 600)
		// With 2:1 aspect ratio in 800x600 container, should fit as 800x400
		if contentW != 800 || contentH != 400 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 800, 400", contentW, contentH)
		}
		// Offset should center vertically: (600-400)/2 = 100
		if offsetX != 0 || offsetY != 100 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 100", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode with cover mode", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "bar_chart",
			Output: OutputSpec{
				Width:       800,
				Height:      400,
				FitMode:     "cover",
				AspectRatio: 1.0, // Force 1:1 to test cover mode scaling
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 800, 400)
		// 1:1 aspect ratio with cover mode should scale up to fill 800x800
		if contentW != 800 || contentH != 800 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 800, 800", contentW, contentH)
		}
		if offsetX != 0 || offsetY != -200 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, -200", offsetX, offsetY)
		}
	})

	t.Run("applyFitMode empty FitMode defaults to stretch", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Output: OutputSpec{
				Width:  800,
				Height: 400,
				// FitMode not set
			},
		}
		contentW, contentH, offsetX, offsetY := applyFitMode(req, 800, 400)
		// Empty FitMode should default to stretch
		if contentW != 800 || contentH != 400 {
			t.Errorf("applyFitMode() contentW=%v, contentH=%v, want 800, 400", contentW, contentH)
		}
		if offsetX != 0 || offsetY != 0 {
			t.Errorf("applyFitMode() offsetX=%v, offsetY=%v, want 0, 0", offsetX, offsetY)
		}
	})
}

// TestGetSourceAspectRatio tests the natural aspect ratio detection for different chart types.
func TestGetSourceAspectRatio(t *testing.T) {
	tests := []struct {
		name        string
		chartType   string
		aspectRatio float64 // Custom aspect ratio from OutputSpec
		wantSrcW    float64
		wantSrcH    float64
	}{
		// All chart types use container ratio (no 1:1 forcing).
		{"pie_chart uses container ratio", "pie_chart", 0, 800, 600},
		{"donut_chart uses container ratio", "donut_chart", 0, 800, 600},
		{"radar_chart uses container ratio", "radar_chart", 0, 800, 600},
		{"gauge uses container ratio", "gauge", 0, 800, 600},
		{"bar_chart uses container ratio", "bar_chart", 0, 800, 600},
		{"line_chart uses container ratio", "line_chart", 0, 800, 600},
		// Custom aspect ratio overrides container ratio for all chart types.
		{"pie_chart with custom ratio", "pie_chart", 1.5, 1.5, 1.0},
		{"bar_chart with custom ratio", "bar_chart", 2.0, 2.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type: tt.chartType,
				Output: OutputSpec{
					Width:       800,
					Height:      600,
					AspectRatio: tt.aspectRatio,
				},
			}
			srcW, srcH := getSourceAspectRatio(req)
			if srcW != tt.wantSrcW || srcH != tt.wantSrcH {
				t.Errorf("getSourceAspectRatio() = (%v, %v), want (%v, %v)",
					srcW, srcH, tt.wantSrcW, tt.wantSrcH)
			}
		})
	}
}

// TestFitModeRendersPieChart tests that FitMode is actually applied during rendering.
func TestFitModeRendersPieChart(t *testing.T) {
	d := &PieChartDiagram{NewBaseDiagram("pie_chart")}

	t.Run("pie chart with contain mode fills full container", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "pie_chart",
			Title: "Market Share",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"values":     []any{50.0, 30.0, 20.0},
			},
			Output: OutputSpec{
				Width:   800,
				Height:  400,
				FitMode: "contain",
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// Pie chart fills the full container (source ratio = container ratio).
		// 800pt = 1067px, 400pt = 533px in CSS pixels.
		if doc.Width != 1067 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1067, 533)", doc.Width, doc.Height)
		}

		// Container dimensions should be stored
		if doc.ContainerWidth != 800 || doc.ContainerHeight != 400 {
			t.Errorf("container dimensions = (%v, %v), want (800, 400)",
				doc.ContainerWidth, doc.ContainerHeight)
		}

		// No offset — content fills the full container
		if doc.OffsetX != 0 || doc.OffsetY != 0 {
			t.Errorf("offset = (%v, %v), want (0, 0)", doc.OffsetX, doc.OffsetY)
		}

		// FitMode should be recorded
		if doc.FitMode != "contain" {
			t.Errorf("FitMode = %q, want contain", doc.FitMode)
		}
	})

	t.Run("pie chart auto-applies contain mode when FitMode not set", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"values":     []any{60.0, 40.0},
			},
			Output: OutputSpec{
				Width:  800,
				Height: 400,
				// FitMode not set - should auto-apply "contain"
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// Pie chart fills full container. 800pt = 1067px, 400pt = 533px.
		if doc.Width != 1067 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1067, 533)", doc.Width, doc.Height)
		}

		// Container dimensions should be stored
		if doc.ContainerWidth != 800 || doc.ContainerHeight != 400 {
			t.Errorf("container dimensions = (%v, %v), want (800, 400)",
				doc.ContainerWidth, doc.ContainerHeight)
		}

		// No offset — content fills the full container
		if doc.OffsetX != 0 || doc.OffsetY != 0 {
			t.Errorf("offset = (%v, %v), want (0, 0)", doc.OffsetX, doc.OffsetY)
		}

		// FitMode should be recorded as contain (auto-applied)
		if doc.FitMode != "contain" {
			t.Errorf("FitMode = %q, want contain", doc.FitMode)
		}
	})

	t.Run("pie chart with explicit stretch mode overrides auto-contain", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "pie_chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"values":     []any{60.0, 40.0},
			},
			Output: OutputSpec{
				Width:   800,
				Height:  400,
				FitMode: "stretch", // Explicit stretch should override auto-contain
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// With explicit stretch mode, dimensions should match container (in CSS pixels)
		// 800pt = 1067px, 400pt = 533px
		if doc.Width != 1067 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1067, 533)", doc.Width, doc.Height)
		}

		// No offset for stretch mode
		if doc.OffsetX != 0 || doc.OffsetY != 0 {
			t.Errorf("offset = (%v, %v), want (0, 0)", doc.OffsetX, doc.OffsetY)
		}

		// FitMode should be stretch
		if doc.FitMode != "stretch" {
			t.Errorf("FitMode = %q, want stretch", doc.FitMode)
		}
	})
}

// TestExtractScatterChartData tests both data formats for scatter charts.
func TestExtractScatterChartData(t *testing.T) {
	t.Run("parallel arrays format (values + x_values)", func(t *testing.T) {
		req := &RequestEnvelope{
			Title: "Scatter Test",
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name":     "Points",
						"values":   []any{20.0, 35.0, 50.0},
						"x_values": []any{10.0, 25.0, 40.0},
					},
				},
			},
		}

		data, err := extractScatterChartData(req)
		if err != nil {
			t.Fatalf("extractScatterChartData() error = %v", err)
		}
		if len(data.Series) != 1 {
			t.Fatalf("len(Series) = %d, want 1", len(data.Series))
		}
		if len(data.Series[0].Values) != 3 {
			t.Errorf("len(Values) = %d, want 3", len(data.Series[0].Values))
		}
		if len(data.Series[0].XValues) != 3 {
			t.Errorf("len(XValues) = %d, want 3", len(data.Series[0].XValues))
		}
		if data.Series[0].XValues[0] != 10.0 {
			t.Errorf("XValues[0] = %v, want 10", data.Series[0].XValues[0])
		}
		if data.Series[0].Values[0] != 20.0 {
			t.Errorf("Values[0] = %v, want 20", data.Series[0].Values[0])
		}
	})

	t.Run("point objects format (from ChartSpec.ToDiagramSpec)", func(t *testing.T) {
		// This is the format produced by buildChartData for ChartScatter
		req := &RequestEnvelope{
			Title: "Scatter Points",
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Data",
						"points": []any{
							map[string]any{"x": 1.0, "y": 85.0},
							map[string]any{"x": 2.0, "y": 92.0},
							map[string]any{"x": 3.0, "y": 78.0},
						},
					},
				},
			},
		}

		data, err := extractScatterChartData(req)
		if err != nil {
			t.Fatalf("extractScatterChartData() error = %v", err)
		}
		if len(data.Series) != 1 {
			t.Fatalf("len(Series) = %d, want 1", len(data.Series))
		}
		s := data.Series[0]
		if s.Name != "Data" {
			t.Errorf("Name = %q, want Data", s.Name)
		}
		if len(s.Values) != 3 {
			t.Fatalf("len(Values) = %d, want 3", len(s.Values))
		}
		if len(s.XValues) != 3 {
			t.Fatalf("len(XValues) = %d, want 3", len(s.XValues))
		}
		// Verify x,y are mapped correctly
		if s.XValues[0] != 1.0 || s.Values[0] != 85.0 {
			t.Errorf("point 0: x=%v,y=%v, want x=1,y=85", s.XValues[0], s.Values[0])
		}
		if s.XValues[1] != 2.0 || s.Values[1] != 92.0 {
			t.Errorf("point 1: x=%v,y=%v, want x=2,y=92", s.XValues[1], s.Values[1])
		}
		if s.XValues[2] != 3.0 || s.Values[2] != 78.0 {
			t.Errorf("point 2: x=%v,y=%v, want x=3,y=78", s.XValues[2], s.Values[2])
		}
	})

	t.Run("point objects with labels", func(t *testing.T) {
		req := &RequestEnvelope{
			Title: "Labeled Points",
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Data",
						"points": []any{
							map[string]any{"x": 1.0, "y": 10.0, "label": "A"},
							map[string]any{"x": 2.0, "y": 20.0, "label": "B"},
						},
					},
				},
			},
		}

		data, err := extractScatterChartData(req)
		if err != nil {
			t.Fatalf("extractScatterChartData() error = %v", err)
		}
		s := data.Series[0]
		if len(s.Labels) != 2 {
			t.Fatalf("len(Labels) = %d, want 2", len(s.Labels))
		}
		if s.Labels[0] != "A" {
			t.Errorf("Labels[0] = %q, want A", s.Labels[0])
		}
		if s.Labels[1] != "B" {
			t.Errorf("Labels[1] = %q, want B", s.Labels[1])
		}
	})

	t.Run("parallel arrays take precedence over points", func(t *testing.T) {
		// If both values and points are present, values wins
		req := &RequestEnvelope{
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name":   "Data",
						"values": []any{99.0},
						"points": []any{
							map[string]any{"x": 1.0, "y": 50.0},
						},
					},
				},
			},
		}

		data, err := extractScatterChartData(req)
		if err != nil {
			t.Fatalf("extractScatterChartData() error = %v", err)
		}
		// values should be used, not points
		if data.Series[0].Values[0] != 99.0 {
			t.Errorf("Values[0] = %v, want 99 (from values, not points)", data.Series[0].Values[0])
		}
	})
}

// TestScatterChartDiagramWithPointFormat tests the full scatter chart rendering
// via the diagram interface using the point-object format (the previously broken path).
func TestScatterChartDiagramWithPointFormat(t *testing.T) {
	d := &ScatterChartDiagram{NewBaseDiagram("scatter_chart")}

	// Render with point-object format (produced by ChartSpec.ToDiagramSpec)
	reqPoints := &RequestEnvelope{
		Type:  "scatter_chart",
		Title: "Sensitivity vs Specificity",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name": "Data",
					"points": []any{
						map[string]any{"x": 1.0, "y": 85.0},
						map[string]any{"x": 2.0, "y": 92.0},
						map[string]any{"x": 3.0, "y": 78.0},
						map[string]any{"x": 4.0, "y": 88.0},
						map[string]any{"x": 5.0, "y": 95.0},
					},
				},
			},
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	docPoints, err := d.Render(reqPoints)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if docPoints == nil {
		t.Fatal("Render() returned nil")
	}

	svgPoints := string(docPoints.Content)
	if !strings.Contains(svgPoints, "<svg") {
		t.Error("Output missing <svg tag")
	}

	// Also render with parallel-arrays format (known working)
	reqArrays := &RequestEnvelope{
		Type:  "scatter_chart",
		Title: "Sensitivity vs Specificity",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":     "Data",
					"values":   []any{85.0, 92.0, 78.0, 88.0, 95.0},
					"x_values": []any{1.0, 2.0, 3.0, 4.0, 5.0},
				},
			},
		},
		Output: OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	docArrays, err := d.Render(reqArrays)
	if err != nil {
		t.Fatalf("Render() with arrays error = %v", err)
	}

	svgArrays := string(docArrays.Content)

	// Both formats should produce similar-length SVG content.
	// The point-format SVG should not be significantly shorter than the array-format SVG,
	// which would indicate missing data points.
	pointLen := len(svgPoints)
	arrayLen := len(svgArrays)
	ratio := float64(pointLen) / float64(arrayLen)
	if ratio < 0.8 {
		t.Errorf("Point-format SVG is suspiciously smaller than array-format (ratio=%.2f). "+
			"Point-format may be missing data points. Lengths: points=%d, arrays=%d",
			ratio, pointLen, arrayLen)
	}
}

// TestToPointSlice tests the toPointSlice helper.
func TestToPointSlice(t *testing.T) {
	t.Run("valid points", func(t *testing.T) {
		input := []any{
			map[string]any{"x": 1.0, "y": 2.0},
			map[string]any{"x": 3.0, "y": 4.0, "label": "P"},
		}
		pts, ok := toPointSlice(input)
		if !ok {
			t.Fatal("toPointSlice() ok = false, want true")
		}
		if len(pts) != 2 {
			t.Fatalf("len(pts) = %d, want 2", len(pts))
		}
		if pts[0].x != 1.0 || pts[0].y != 2.0 {
			t.Errorf("pts[0] = (%v,%v), want (1,2)", pts[0].x, pts[0].y)
		}
		if pts[1].label != "P" {
			t.Errorf("pts[1].label = %q, want P", pts[1].label)
		}
	})

	t.Run("invalid - missing x", func(t *testing.T) {
		input := []any{
			map[string]any{"y": 2.0},
		}
		_, ok := toPointSlice(input)
		if ok {
			t.Error("toPointSlice() ok = true, want false for missing x")
		}
	})

	t.Run("invalid - missing y", func(t *testing.T) {
		input := []any{
			map[string]any{"x": 1.0},
		}
		_, ok := toPointSlice(input)
		if ok {
			t.Error("toPointSlice() ok = true, want false for missing y")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []any{}
		_, ok := toPointSlice(input)
		if ok {
			t.Error("toPointSlice() ok = true, want false for empty slice")
		}
	})

	t.Run("non-map element", func(t *testing.T) {
		input := []any{"not a map"}
		_, ok := toPointSlice(input)
		if ok {
			t.Error("toPointSlice() ok = true, want false for non-map element")
		}
	})

	t.Run("non-slice input", func(t *testing.T) {
		_, ok := toPointSlice("not a slice")
		if ok {
			t.Error("toPointSlice() ok = true, want false for non-slice input")
		}
	})
}

// =============================================================================
// Stacked Bar Chart Tests
// =============================================================================

// TestStackedBarChartDiagram tests the stacked bar chart diagram via the registry.
func TestStackedBarChartDiagram(t *testing.T) {
	d := &StackedBarChartDiagram{NewBaseDiagram("stacked_bar_chart")}

	t.Run("Type", func(t *testing.T) {
		if got := d.Type(); got != "stacked_bar_chart" {
			t.Errorf("Type() = %q, want stacked_bar_chart", got)
		}
	})

	t.Run("Validate with valid data", func(t *testing.T) {
		req := &RequestEnvelope{
			Type: "stacked_bar_chart",
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{
						"name":   "Payments",
						"values": []any{0.8, 0.9, 1.0, 1.1},
					},
					map[string]any{
						"name":   "Subscriptions",
						"values": []any{0.2, 0.25, 0.3, 0.35},
					},
					map[string]any{
						"name":   "APIs",
						"values": []any{0.15, 0.15, 0.15, 0.15},
					},
				},
			},
		}
		if err := d.Validate(req); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Render stacked bar chart with multiple series", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "stacked_bar_chart",
			Title: "Quarterly Revenue by Segment",
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{
						"name":   "Payments",
						"values": []any{0.8, 0.9, 1.0, 1.1},
					},
					map[string]any{
						"name":   "Subscriptions",
						"values": []any{0.2, 0.25, 0.3, 0.35},
					},
					map[string]any{
						"name":   "APIs",
						"values": []any{0.15, 0.15, 0.15, 0.15},
					},
				},
			},
			Output: OutputSpec{
				Width:  800,
				Height: 600,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
		svg := string(doc.Content)
		if !strings.Contains(svg, "<svg") {
			t.Error("Output missing <svg tag")
		}

		// A stacked bar chart with 3 series should have at least 3 distinct
		// fill colors in the SVG (one per series). Count unique fill attributes.
		// Each series segment is drawn as a <rect> with a different fill color.
		fillCount := strings.Count(svg, "fill=")
		if fillCount < 3 {
			t.Errorf("Expected at least 3 fill attributes for 3 series segments, got %d", fillCount)
		}
	})

	t.Run("Render stacked bar chart with single series", func(t *testing.T) {
		// Single series should still render without error (graceful degradation)
		req := &RequestEnvelope{
			Type:  "stacked_bar_chart",
			Title: "Single Series Stacked",
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{
						"name":   "Revenue",
						"values": []any{1.15, 1.3, 1.45, 1.6},
					},
				},
			},
			Output: OutputSpec{
				Width:  800,
				Height: 600,
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if doc == nil {
			t.Fatal("Render() returned nil")
		}
	})
}

// TestStackedBarChartMultipleColors verifies that stacked bar chart renders
// distinct colors for each series by checking SVG content.
func TestStackedBarChartMultipleColors(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.Stacked = true
	config.ShowLegend = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Revenue by Segment",
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []ChartSeries{
			{Name: "Payments", Values: []float64{0.8, 0.9, 1.0, 1.1}},
			{Name: "Subscriptions", Values: []float64{0.2, 0.25, 0.3, 0.35}},
			{Name: "APIs", Values: []float64{0.15, 0.15, 0.15, 0.15}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw stacked bar chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := doc.String()

	// Verify the SVG has substantial content (stacked bars with 3 series x 4 categories
	// produce significantly more drawing commands than a single-series chart)
	if len(svg) < 1000 {
		t.Errorf("SVG output suspiciously short (%d bytes), stacked segments may be missing", len(svg))
	}

	// Verify SVG contains fill color attributes (each series segment has its own color)
	fillCount := strings.Count(svg, "fill:")
	if fillCount < 6 {
		t.Errorf("Expected at least 6 fill color directives for 3 series, got %d", fillCount)
	}
}

// TestStackedBarChartDomain verifies that the Y-axis domain is calculated
// correctly for stacked bars (max is sum of all series at any category).
func TestStackedBarChartDomain(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.Stacked = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B"},
		Series: []ChartSeries{
			{Name: "S1", Values: []float64{30, 40}},
			{Name: "S2", Values: []float64{20, 10}},
		},
	}

	// Test that calculateDomain returns correct stacked max
	yMin, yMax := chart.calculateDomain(data)
	if yMin != 0 {
		t.Errorf("yMin = %v, want 0", yMin)
	}
	// Max stack: A=30+20=50, B=40+10=50
	if yMax != 50 {
		t.Errorf("yMax = %v, want 50 (sum of stacked values)", yMax)
	}
}

// TestStackedBarChartWithValues verifies that value labels are drawn on segments.
func TestStackedBarChartWithValues(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.Stacked = true
	config.ShowValues = true
	config.ValueFormat = "%.1f"
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"Q1", "Q2"},
		Series: []ChartSeries{
			{Name: "A", Values: []float64{100, 200}},
			{Name: "B", Values: []float64{50, 80}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw stacked bar chart with values: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := doc.String()
	// Check that value labels appear in the SVG
	if !strings.Contains(svg, "100") {
		t.Error("Expected value label '100' in SVG output")
	}
	if !strings.Contains(svg, "200") {
		t.Error("Expected value label '200' in SVG output")
	}
}

// TestChartsAutoApplyContainAndFillContainer tests that all chart types auto-apply
// contain mode and fill their full container dimensions (no 1:1 forcing).
func TestChartsAutoApplyContainAndFillContainer(t *testing.T) {
	// Pie chart fills full container — landscape legend mode uses extra width.
	t.Run("PieChart fills wide container", func(t *testing.T) {
		d := &PieChartDiagram{NewBaseDiagram("pie_chart")}
		req := &RequestEnvelope{
			Type:  "pie_chart",
			Title: "Test Pie Chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"values":     []any{30.0, 40.0, 30.0},
			},
			Output: OutputSpec{
				Width:  1200,
				Height: 400,
				// FitMode not set
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// Pie chart uses container ratio, filling the full 1200×400 placeholder.
		// 1200pt = 1600px, 400pt = 533px in CSS pixels.
		if doc.Width != 1600 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1600, 533)", doc.Width, doc.Height)
		}

		// FitMode is auto-applied contain
		if doc.FitMode != "contain" {
			t.Errorf("FitMode = %q, want contain (auto-applied)", doc.FitMode)
		}
	})

	// Donut chart fills full container.
	t.Run("DonutChart fills wide container", func(t *testing.T) {
		d := &DonutChartDiagram{NewBaseDiagram("donut_chart")}
		req := &RequestEnvelope{
			Type:  "donut_chart",
			Title: "Test Donut Chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"values":     []any{30.0, 40.0, 30.0},
			},
			Output: OutputSpec{
				Width:  1200,
				Height: 400,
				// FitMode not set
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// Donut chart uses container ratio, filling the full placeholder.
		// 1200pt = 1600px, 400pt = 533px in CSS pixels.
		if doc.Width != 1600 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1600, 533)", doc.Width, doc.Height)
		}

		// FitMode is auto-applied contain
		if doc.FitMode != "contain" {
			t.Errorf("FitMode = %q, want contain (auto-applied)", doc.FitMode)
		}
	})

	// Radar chart fills full container — labels use extra horizontal space.
	t.Run("RadarChart fills wide container", func(t *testing.T) {
		d := &RadarChartDiagram{NewBaseDiagram("radar_chart")}
		req := &RequestEnvelope{
			Type:  "radar_chart",
			Title: "Test Radar Chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C", "D"},
				"series": []any{
					map[string]any{
						"name":   "Series 1",
						"values": []any{50.0, 70.0, 60.0, 80.0},
					},
				},
			},
			Output: OutputSpec{
				Width:  1200,
				Height: 400,
				// FitMode not set
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// Radar chart uses container ratio, filling the full placeholder.
		// 1200pt = 1600px, 400pt = 533px in CSS pixels.
		if doc.Width != 1600 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1600, 533)", doc.Width, doc.Height)
		}

		// FitMode should be contain (auto-applied)
		if doc.FitMode != "contain" {
			t.Errorf("FitMode = %q, want contain", doc.FitMode)
		}
	})

	// Bar chart also fills full container (all charts use container ratio).
	t.Run("BarChart fills container", func(t *testing.T) {
		d := &BarChartDiagram{NewBaseDiagram("bar_chart")}
		req := &RequestEnvelope{
			Type:  "bar_chart",
			Title: "Test Bar Chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series": []any{
					map[string]any{
						"name":   "Series 1",
						"values": []any{30.0, 40.0, 30.0},
					},
				},
			},
			Output: OutputSpec{
				Width:  800,
				Height: 400,
				// FitMode not set - auto-apply "contain", uses container ratio
			},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		// All charts fill their container (in CSS pixels).
		// 800pt = 1067px, 400pt = 533px
		if doc.Width != 1067 || doc.Height != 533 {
			t.Errorf("doc dimensions = (%v, %v), want (1067, 533)", doc.Width, doc.Height)
		}
	})
}

// =============================================================================
// Color Override Tests — verify data.colors propagates to chart rendering
// =============================================================================

// TestAxisTitles_BarLineArea verifies that y_label and x_label from request
// data are rendered into the SVG output for bar, line, and area charts.
func TestAxisTitles_BarLineArea(t *testing.T) {
	tests := []struct {
		name     string
		diagram  Diagram
		chartType string
	}{
		{"bar_chart", &BarChartDiagram{NewBaseDiagram("bar_chart")}, "bar_chart"},
		{"line_chart", &LineChartDiagram{NewBaseDiagram("line_chart")}, "line_chart"},
		{"area_chart", &AreaChartDiagram{NewBaseDiagram("area_chart")}, "area_chart"},
		{"stacked_bar_chart", &StackedBarChartDiagram{NewBaseDiagram("stacked_bar_chart")}, "stacked_bar_chart"},
		{"grouped_bar_chart", &GroupedBarChartDiagram{NewBaseDiagram("grouped_bar_chart")}, "grouped_bar_chart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  tt.chartType,
				Title: "Test Chart",
				Data: map[string]any{
					"categories": []any{"A", "B", "C"},
					"series": []any{
						map[string]any{
							"name":   "S1",
							"values": []any{10.0, 20.0, 30.0},
						},
						map[string]any{
							"name":   "S2",
							"values": []any{15.0, 25.0, 35.0},
						},
					},
					"y_label": "Revenue ($M)",
					"x_label": "Quarter",
				},
				Output: OutputSpec{
					Width:  800,
					Height: 600,
				},
			}

			doc, err := tt.diagram.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			svg := string(doc.Content)

			if !strings.Contains(svg, "Revenue ($M)") {
				t.Error("SVG missing y_label 'Revenue ($M)'")
			}
			if !strings.Contains(svg, "Quarter") {
				t.Error("SVG missing x_label 'Quarter'")
			}
		})
	}
}

// svgContainsColor checks if the SVG content contains the given hex color
// using case-insensitive matching (the SVG renderer outputs lowercase hex).
func svgContainsColor(svg, hexColor string) bool {
	return strings.Contains(strings.ToLower(svg), strings.ToLower(hexColor))
}

func TestBarChartDiagram_ColorOverride(t *testing.T) {
	d := &BarChartDiagram{NewBaseDiagram("bar_chart")}

	t.Run("custom colors appear in SVG", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "bar_chart",
			Title: "Color Test",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10.0, 20.0}},
				},
				"colors": []any{"#FF1493", "#00CED1"},
			},
			Output: OutputSpec{Width: 800, Height: 600},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		svg := string(doc.Content)
		if !svgContainsColor(svg, "#FF1493") {
			t.Error("expected custom color #FF1493 in SVG output")
		}
	})

	t.Run("no colors field uses default palette", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "bar_chart",
			Title: "Default Colors",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10.0, 20.0}},
				},
			},
			Output: OutputSpec{Width: 800, Height: 600},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		svg := string(doc.Content)
		// Default palette Accent1 is #4E79A7
		if !svgContainsColor(svg, "#4E79A7") {
			t.Error("expected default palette color #4E79A7 in SVG output")
		}
	})
}

func TestLineChartDiagram_ColorOverride(t *testing.T) {
	d := &LineChartDiagram{NewBaseDiagram("line_chart")}

	req := &RequestEnvelope{
		Type:  "line_chart",
		Title: "Line Color Test",
		Data: map[string]any{
			"categories": []any{"Jan", "Feb", "Mar"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10.0, 20.0, 15.0}},
			},
			"colors": []any{"#8B0000"},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	if !svgContainsColor(svg, "#8B0000") {
		t.Error("expected custom color #8B0000 in line chart SVG output")
	}
}

func TestScatterChartDiagram_ColorOverride(t *testing.T) {
	d := &ScatterChartDiagram{NewBaseDiagram("scatter_chart")}

	t.Run("custom colors applied to points", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "scatter_chart",
			Title: "Scatter Color Test",
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name":     "S1",
						"values":   []any{10.0, 20.0, 15.0},
						"x_values": []any{1.0, 2.0, 3.0},
					},
				},
				"colors": []any{"#DAA520"},
			},
			Output: OutputSpec{Width: 800, Height: 600},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		svg := strings.ToLower(string(doc.Content))

		// Scatter points render with 0.8 opacity, so the color appears as
		// rgba(218,165,31,.8) in the SVG output (the canvas library may
		// round RGB channels by 1). Check for a distinctive substring.
		if !strings.Contains(svg, "rgba(218,165,") {
			t.Error("expected custom color #DAA520 (as rgba) in scatter chart SVG output")
		}
	})

	t.Run("no colors field renders without error", func(t *testing.T) {
		req := &RequestEnvelope{
			Type:  "scatter_chart",
			Title: "Scatter Default",
			Data: map[string]any{
				"series": []any{
					map[string]any{
						"name":     "S1",
						"values":   []any{10.0, 20.0, 15.0},
						"x_values": []any{1.0, 2.0, 3.0},
					},
				},
			},
			Output: OutputSpec{Width: 800, Height: 600},
		}

		doc, err := d.Render(req)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		svg := strings.ToLower(string(doc.Content))

		// Without color overrides, the chart should NOT contain our custom color
		if strings.Contains(svg, "rgba(218,165,") {
			t.Error("expected default palette, not custom #DAA520 color, when no overrides set")
		}
	})
}

func TestPieChartDiagram_ColorOverride(t *testing.T) {
	d := &PieChartDiagram{NewBaseDiagram("pie_chart")}

	req := &RequestEnvelope{
		Type:  "pie_chart",
		Title: "Pie Color Test",
		Data: map[string]any{
			"labels": []any{"A", "B", "C"},
			"values": []any{30.0, 50.0, 20.0},
			"colors": []any{"#FF6347", "#4682B4", "#32CD32"},
		},
		Output: OutputSpec{Width: 600, Height: 600},
	}

	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	if !svgContainsColor(svg, "#FF6347") {
		t.Error("expected custom color #FF6347 in pie chart SVG output")
	}
	if !svgContainsColor(svg, "#32CD32") {
		t.Error("expected custom color #32CD32 in pie chart SVG output")
	}
}

func TestAreaChartDiagram_ColorOverride(t *testing.T) {
	d := &AreaChartDiagram{NewBaseDiagram("area_chart")}

	req := &RequestEnvelope{
		Type:  "area_chart",
		Title: "Area Color Test",
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{10.0, 20.0, 15.0}},
			},
			"colors": []any{"#9932CC"},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	if !svgContainsColor(svg, "#9932CC") {
		t.Error("expected custom color #9932CC in area chart SVG output")
	}
}

func TestRadarChartDiagram_ColorOverride(t *testing.T) {
	d := &RadarChartDiagram{NewBaseDiagram("radar_chart")}

	req := &RequestEnvelope{
		Type:  "radar_chart",
		Title: "Radar Color Test",
		Data: map[string]any{
			"categories": []any{"Speed", "Power", "Defense", "Range", "Magic"},
			"series": []any{
				map[string]any{"name": "Hero", "values": []any{80.0, 60.0, 90.0, 40.0, 70.0}},
			},
			"colors": []any{"#FF4500"},
		},
		Output: OutputSpec{Width: 600, Height: 600},
		Style:  StyleSpec{ShowGrid: true},
	}

	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	if !svgContainsColor(svg, "#FF4500") {
		t.Error("expected custom color #FF4500 in radar chart SVG output")
	}
}

func TestExtractChartColors(t *testing.T) {
	t.Run("valid hex colors", func(t *testing.T) {
		data := map[string]any{
			"colors": []any{"#FF0000", "#00FF00", "#0000FF"},
		}
		colors := extractChartColors(data)
		if len(colors) != 3 {
			t.Fatalf("expected 3 colors, got %d", len(colors))
		}
		if colors[0].Hex() != "#FF0000" {
			t.Errorf("colors[0] = %s, want #FF0000", colors[0].Hex())
		}
		if colors[1].Hex() != "#00FF00" {
			t.Errorf("colors[1] = %s, want #00FF00", colors[1].Hex())
		}
		if colors[2].Hex() != "#0000FF" {
			t.Errorf("colors[2] = %s, want #0000FF", colors[2].Hex())
		}
	})

	t.Run("missing colors field", func(t *testing.T) {
		data := map[string]any{
			"categories": []any{"A", "B"},
		}
		colors := extractChartColors(data)
		if colors != nil {
			t.Errorf("expected nil for missing colors, got %v", colors)
		}
	})

	t.Run("empty colors array", func(t *testing.T) {
		data := map[string]any{
			"colors": []any{},
		}
		colors := extractChartColors(data)
		if colors != nil {
			t.Errorf("expected nil for empty colors, got %v", colors)
		}
	})

	t.Run("invalid color strings ignored", func(t *testing.T) {
		data := map[string]any{
			"colors": []any{"#FF0000", "not-a-color", "#00FF00"},
		}
		colors := extractChartColors(data)
		if len(colors) != 2 {
			t.Fatalf("expected 2 valid colors, got %d", len(colors))
		}
	})

	t.Run("all invalid returns nil", func(t *testing.T) {
		data := map[string]any{
			"colors": []any{"invalid1", "invalid2"},
		}
		colors := extractChartColors(data)
		if colors != nil {
			t.Errorf("expected nil for all-invalid colors, got %v", colors)
		}
	})

	t.Run("non-array value returns nil", func(t *testing.T) {
		data := map[string]any{
			"colors": "not-an-array",
		}
		colors := extractChartColors(data)
		if colors != nil {
			t.Errorf("expected nil for non-array colors, got %v", colors)
		}
	})
}

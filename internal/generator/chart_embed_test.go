package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/ahrens/svggen"
)

// TestRenderAndEmbed tests the basic RenderAndEmbed functionality.
func TestRenderAndEmbed(t *testing.T) {
	t.Parallel()

	// Load baseline PPTX - we need a real template with slides
	templatePath := filepath.Join("..", "pptx", "testdata", "baseline.pptx")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Skipf("Skipping test: baseline.pptx not found: %v", err)
	}

	t.Run("embeds matrix2x2 chart", func(t *testing.T) {
		t.Parallel()

		// Open document from bytes
		doc, err := pptx.OpenDocumentFromBytes(templateData)
		if err != nil {
			t.Fatalf("failed to open document: %v", err)
		}

		// Create chart request
		chartReq := &svggen.RequestEnvelope{
			Type:  "matrix_2x2",
			Title: "Strategic Analysis",
			Data: map[string]any{
				"x_axis": map[string]any{
					"label": "Market Attractiveness",
					"min":   "Low",
					"max":   "High",
				},
				"y_axis": map[string]any{
					"label": "Competitive Position",
					"min":   "Weak",
					"max":   "Strong",
				},
				"quadrants": []map[string]any{
					{"label": "Stars", "position": "top_right"},
					{"label": "Cash Cows", "position": "bottom_right"},
					{"label": "Question Marks", "position": "top_left"},
					{"label": "Dogs", "position": "bottom_left"},
				},
				"items": []map[string]any{
					{"name": "Product A", "x": 0.8, "y": 0.7},
					{"name": "Product B", "x": 0.3, "y": 0.4},
				},
			},
			Output: svggen.OutputSpec{
				Width:  400,
				Height: 300,
				Scale:  2.0,
			},
		}

		// Embed chart
		err = RenderAndEmbed(chartReq, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(1, 2, 5, 4),
			Name:       "Strategic Matrix",
			AltText:    "A 2x2 matrix showing product positioning",
		})
		if err != nil {
			t.Fatalf("RenderAndEmbed failed: %v", err)
		}

		// Save and verify output
		outputBytes, err := doc.SaveToBytes()
		if err != nil {
			t.Fatalf("failed to save document: %v", err)
		}

		// Basic validation - should be a valid ZIP
		if len(outputBytes) < 100 {
			t.Errorf("output too small: %d bytes", len(outputBytes))
		}

		// Re-open and verify slide count
		verifyDoc, err := pptx.OpenDocumentFromBytes(outputBytes)
		if err != nil {
			t.Fatalf("failed to re-open document: %v", err)
		}

		slideCount, err := verifyDoc.SlideCount()
		if err != nil {
			t.Fatalf("failed to get slide count: %v", err)
		}
		if slideCount < 1 {
			t.Errorf("expected at least 1 slide, got %d", slideCount)
		}

		t.Logf("Successfully embedded chart. Output size: %d bytes", len(outputBytes))
	})

	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()

		doc, err := pptx.OpenDocumentFromBytes(templateData)
		if err != nil {
			t.Fatalf("failed to open document: %v", err)
		}

		// Test nil request
		err = RenderAndEmbed(nil, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(1, 1, 4, 3),
		})
		if err == nil {
			t.Error("expected error for nil request")
		}

		// Test nil document
		chartReq := &svggen.RequestEnvelope{
			Type: "matrix_2x2",
			Data: map[string]any{},
		}
		err = RenderAndEmbed(chartReq, nil, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(1, 1, 4, 3),
		})
		if err == nil {
			t.Error("expected error for nil document")
		}

		// Test zero bounds
		err = RenderAndEmbed(chartReq, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectEmu{}, // Zero area
		})
		if err == nil {
			t.Error("expected error for zero bounds")
		}
	})
}

// TestRenderAndEmbedWithResult tests the result-returning variant.
func TestRenderAndEmbedWithResult(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join("..", "pptx", "testdata", "baseline.pptx")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Skipf("Skipping test: baseline.pptx not found: %v", err)
	}

	doc, err := pptx.OpenDocumentFromBytes(templateData)
	if err != nil {
		t.Fatalf("failed to open document: %v", err)
	}

	chartReq := &svggen.RequestEnvelope{
		Type:  "timeline",
		Title: "Development Timeline",
		Data: map[string]any{
			"activities": []map[string]any{
				{"label": "Plan", "start_date": "2024-01-01", "end_date": "2024-02-01"},
				{"label": "Build", "start_date": "2024-02-01", "end_date": "2024-04-01"},
				{"label": "Test", "start_date": "2024-04-01", "end_date": "2024-05-01"},
				{"label": "Deploy", "start_date": "2024-05-01", "end_date": "2024-06-01"},
			},
		},
		Output: svggen.OutputSpec{
			Width:  600,
			Height: 200,
			Scale:  2.0,
		},
	}

	result, err := RenderAndEmbedWithResult(chartReq, doc, RenderAndEmbedOptions{
		SlideIndex: 0,
		Bounds:     pptx.RectFromInches(0.5, 4, 7, 2),
		Name:       "Process Flow",
	})
	if err != nil {
		t.Fatalf("RenderAndEmbedWithResult failed: %v", err)
	}

	// Verify result
	if result.SVGWidth <= 0 {
		t.Errorf("expected positive SVG width, got %f", result.SVGWidth)
	}
	if result.SVGHeight <= 0 {
		t.Errorf("expected positive SVG height, got %f", result.SVGHeight)
	}
	if result.PNGSize <= 0 {
		t.Errorf("expected positive PNG size, got %d", result.PNGSize)
	}

	t.Logf("Result: SVG %.0fx%.0f, PNG %d bytes", result.SVGWidth, result.SVGHeight, result.PNGSize)
}

// TestChartEmbedder tests the stateful embedder API.
func TestChartEmbedder(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join("..", "pptx", "testdata", "baseline.pptx")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Skipf("Skipping test: baseline.pptx not found: %v", err)
	}

	t.Run("embeds multiple charts", func(t *testing.T) {
		t.Parallel()

		doc, err := pptx.OpenDocumentFromBytes(templateData)
		if err != nil {
			t.Fatalf("failed to open document: %v", err)
		}

		embedder := NewChartEmbedder()

		// First chart
		chart1 := &svggen.RequestEnvelope{
			Type:  "timeline",
			Title: "Project Timeline",
			Data: map[string]any{
				"milestones": []map[string]any{
					{"name": "Kickoff", "date": "2024-01"},
					{"name": "Alpha", "date": "2024-03"},
					{"name": "Beta", "date": "2024-06"},
					{"name": "Launch", "date": "2024-09"},
				},
			},
			Output: svggen.OutputSpec{Width: 500, Height: 150},
		}

		err = embedder.Embed(chart1, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(1, 1, 6, 2),
			Name:       "Timeline Chart",
		})
		if err != nil {
			t.Fatalf("failed to embed first chart: %v", err)
		}

		// Second chart (same slide)
		chart2 := &svggen.RequestEnvelope{
			Type:  "waterfall",
			Title: "Budget Analysis",
			Data: map[string]any{
				"points": []any{
					map[string]any{"label": "Start", "value": 1000.0, "type": "total"},
					map[string]any{"label": "Revenue", "value": 500.0, "type": "increase"},
					map[string]any{"label": "Costs", "value": -200.0, "type": "decrease"},
					map[string]any{"label": "End", "value": 1300.0, "type": "total"},
				},
			},
			Output: svggen.OutputSpec{Width: 400, Height: 300},
		}

		err = embedder.Embed(chart2, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(1, 4, 5, 3),
			Name:       "Waterfall Chart",
		})
		if err != nil {
			t.Fatalf("failed to embed second chart: %v", err)
		}

		// Save and verify
		outputBytes, err := doc.SaveToBytes()
		if err != nil {
			t.Fatalf("failed to save document: %v", err)
		}

		t.Logf("Embedded 2 charts. Output size: %d bytes", len(outputBytes))
	})

	t.Run("custom registry", func(t *testing.T) {
		t.Parallel()

		doc, err := pptx.OpenDocumentFromBytes(templateData)
		if err != nil {
			t.Fatalf("failed to open document: %v", err)
		}

		// Create embedder with custom registry (same as default for this test)
		embedder := &ChartEmbedder{
			Registry:        svggen.DefaultRegistry(),
			DefaultPNGScale: 3.0, // Higher resolution
		}

		chart := &svggen.RequestEnvelope{
			Type:  "timeline",
			Title: "Project Timeline",
			Data: map[string]any{
				"activities": []any{
					map[string]any{"label": "Planning", "start_date": "2024-01-01", "end_date": "2024-02-01"},
					map[string]any{"label": "Development", "start_date": "2024-02-01", "end_date": "2024-04-01"},
				},
			},
			Output: svggen.OutputSpec{Width: 300, Height: 400},
		}

		err = embedder.Embed(chart, doc, RenderAndEmbedOptions{
			SlideIndex: 0,
			Bounds:     pptx.RectFromInches(2, 1.5, 4, 5),
		})
		if err != nil {
			t.Fatalf("failed to embed with custom registry: %v", err)
		}
	})
}

// TestRenderAndEmbed_InvalidChartType tests error handling for unknown chart types.
func TestRenderAndEmbed_InvalidChartType(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join("..", "pptx", "testdata", "baseline.pptx")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Skipf("Skipping test: baseline.pptx not found: %v", err)
	}

	doc, err := pptx.OpenDocumentFromBytes(templateData)
	if err != nil {
		t.Fatalf("failed to open document: %v", err)
	}

	// Unknown chart type
	chartReq := &svggen.RequestEnvelope{
		Type: "nonexistent_chart_type",
		Data: map[string]any{"foo": "bar"},
	}

	err = RenderAndEmbed(chartReq, doc, RenderAndEmbedOptions{
		SlideIndex: 0,
		Bounds:     pptx.RectFromInches(1, 1, 4, 3),
	})
	if err == nil {
		t.Error("expected error for unknown chart type")
	}

	// Verify error message mentions rendering failure
	if err != nil && !strings.Contains(err.Error(), "chart rendering failed") {
		t.Errorf("expected 'chart rendering failed' in error, got: %v", err)
	}
}

// TestRenderAndEmbed_SlideOutOfRange tests error handling for invalid slide indices.
func TestRenderAndEmbed_SlideOutOfRange(t *testing.T) {
	t.Parallel()

	templatePath := filepath.Join("..", "pptx", "testdata", "baseline.pptx")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Skipf("Skipping test: baseline.pptx not found: %v", err)
	}

	doc, err := pptx.OpenDocumentFromBytes(templateData)
	if err != nil {
		t.Fatalf("failed to open document: %v", err)
	}

	chartReq := &svggen.RequestEnvelope{
		Type: "matrix_2x2",
		Data: map[string]any{
			"quadrants": []map[string]any{},
		},
		Output: svggen.OutputSpec{Width: 300, Height: 300},
	}

	// Try to embed on slide 999 (doesn't exist)
	err = RenderAndEmbed(chartReq, doc, RenderAndEmbedOptions{
		SlideIndex: 999,
		Bounds:     pptx.RectFromInches(1, 1, 4, 3),
	})
	if err == nil {
		t.Error("expected error for out-of-range slide index")
	}
}

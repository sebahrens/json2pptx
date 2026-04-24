package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDiagramSpecToSVGGen(t *testing.T) {
	tests := []struct {
		name        string
		spec        *types.DiagramSpec
		themeColors []types.ThemeColor
		wantType    string
		wantFitMode string
		wantWidth   int
		wantHeight  int
	}{
		{
			name: "basic diagram spec",
			spec: &types.DiagramSpec{
				Type:  "bar_chart",
				Title: "Test Chart",
				Data:  map[string]any{"categories": []string{"A", "B"}, "values": []float64{10, 20}},
			},
			wantType:    "bar_chart",
			wantFitMode: "",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with FitMode contain",
			spec: &types.DiagramSpec{
				Type:    "pie_chart",
				Title:   "Pie Chart",
				Data:    map[string]any{"categories": []string{"A"}, "values": []float64{100}},
				FitMode: "contain",
			},
			wantType:    "pie_chart",
			wantFitMode: "contain",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with FitMode cover",
			spec: &types.DiagramSpec{
				Type:    "donut_chart",
				Data:    map[string]any{"categories": []string{"A"}, "values": []float64{100}},
				FitMode: "cover",
			},
			wantType:    "donut_chart",
			wantFitMode: "cover",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with custom dimensions and FitMode",
			spec: &types.DiagramSpec{
				Type:    "radar_chart",
				Data:    map[string]any{"categories": []string{"A", "B", "C"}, "values": []float64{10, 20, 30}},
				Width:   1024,
				Height:  768,
				FitMode: "contain",
			},
			wantType:    "radar_chart",
			wantFitMode: "contain",
			wantWidth:   1024,
			wantHeight:  768,
		},
		{
			name: "diagram spec with subtitle passthrough",
			spec: &types.DiagramSpec{
				Type:     "bar_chart",
				Title:    "Revenue by Region",
				Subtitle: "Note: APAC includes Japan launch in Q3",
				Data:     map[string]any{"categories": []string{"EMEA", "Americas"}, "values": []float64{45, 62}},
			},
			wantType:   "bar_chart",
			wantWidth:  types.DefaultChartWidth,
			wantHeight: types.DefaultChartHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diagramSpecToSVGGen(tt.spec, tt.themeColors, 0, "")

			if result.Type != tt.wantType {
				t.Errorf("diagramSpecToSVGGen() Type = %v, want %v", result.Type, tt.wantType)
			}
			if result.Output.FitMode != tt.wantFitMode {
				t.Errorf("diagramSpecToSVGGen() Output.FitMode = %v, want %v", result.Output.FitMode, tt.wantFitMode)
			}
			if result.Output.Width != tt.wantWidth {
				t.Errorf("diagramSpecToSVGGen() Output.Width = %v, want %v", result.Output.Width, tt.wantWidth)
			}
			if result.Output.Height != tt.wantHeight {
				t.Errorf("diagramSpecToSVGGen() Output.Height = %v, want %v", result.Output.Height, tt.wantHeight)
			}
			if result.Output.Format != "png" {
				t.Errorf("diagramSpecToSVGGen() Output.Format = %v, want png", result.Output.Format)
			}
			if result.Output.Scale != 2.0 {
				t.Errorf("diagramSpecToSVGGen() Output.Scale = %v, want 2.0", result.Output.Scale)
			}
			// Verify subtitle passthrough
			if result.Subtitle != tt.spec.Subtitle {
				t.Errorf("diagramSpecToSVGGen() Subtitle = %q, want %q", result.Subtitle, tt.spec.Subtitle)
			}
		})
	}

	// Verify StrictFit is threaded to OutputSpec.
	t.Run("strict_fit_threaded", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "series": []any{}},
		}
		result := diagramSpecToSVGGen(spec, nil, 0, "strict")
		if result.Output.StrictFit != "strict" {
			t.Errorf("StrictFit = %q, want %q", result.Output.StrictFit, "strict")
		}
		resultOff := diagramSpecToSVGGen(spec, nil, 0, "")
		if resultOff.Output.StrictFit != "" {
			t.Errorf("StrictFit = %q, want empty", resultOff.Output.StrictFit)
		}
	})
}

func TestRenderDiagramSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *types.DiagramSpec
		wantErr bool
	}{
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: true,
		},
		{
			name: "empty type",
			spec: &types.DiagramSpec{
				Type: "",
				Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}},
			},
			wantErr: true,
		},
		{
			name: "bar chart with FitMode",
			spec: &types.DiagramSpec{
				Type:    "bar_chart",
				Title:   "FitMode Test",
				Data:    map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
				FitMode: "contain",
			},
			wantErr: false,
		},
		{
			name: "pie chart with FitMode contain",
			spec: &types.DiagramSpec{
				Type:    "pie_chart",
				Title:   "Pie FitMode",
				Data:    map[string]any{"categories": []string{"A", "B"}, "values": []float64{40, 60}},
				Width:   800,
				Height:  400,
				FitMode: "contain",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderDiagramSpec(tt.spec, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RenderDiagramSpec() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("RenderDiagramSpec() unexpected error: %v", err)
				return
			}

			// Verify we got PNG data
			if len(result) == 0 {
				t.Errorf("RenderDiagramSpec() returned empty result")
				return
			}

			// Verify PNG signature (first 8 bytes)
			if len(result) < 8 {
				t.Errorf("RenderDiagramSpec() result too short for PNG")
				return
			}

			pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			for i, b := range pngSignature {
				if result[i] != b {
					t.Errorf("RenderDiagramSpec() result is not a valid PNG (byte %d: got %02x, want %02x)", i, result[i], b)
					return
				}
			}

			t.Logf("Successfully rendered %s diagram: %d bytes", tt.spec.Type, len(result))
		})
	}
}

func TestRenderDiagramSpecWithMetadata_Treemap(t *testing.T) {
	// Simulates the full e2e path: chartutil.BuildChartDataPayload → DiagramSpec → RenderDiagramSpecWithMetadata
	// buildLabelValuePoints returns []map[string]any (NOT []any), which Go cannot type-assert to []any.
	// This was the root cause of go-slide-creator-wionh (treemap "Data unavailable").
	spec := &types.DiagramSpec{
		Type:  "treemap_chart",
		Title: "treemap Performance",
		Data: map[string]any{
			"values": []map[string]any{
				{"label": "Technology", "value": 35.0},
				{"label": "Healthcare", "value": 25.0},
				{"label": "Finance", "value": 20.0},
				{"label": "Energy", "value": 12.0},
				{"label": "Consumer", "value": 8.0},
			},
		},
		Width:  800,
		Height: 600,
	}

	result, err := RenderDiagramSpecWithMetadata(spec, nil, 0, false)
	if err != nil {
		t.Fatalf("RenderDiagramSpecWithMetadata() error = %v", err)
	}
	if result == nil || len(result.PNG) == 0 {
		t.Fatal("Expected non-empty PNG result")
	}
}

package svggen

import (
	"strings"
	"testing"
)

func TestTreemapChart_Draw(t *testing.T) {
	tests := []struct {
		name    string
		data    TreemapData
		wantErr bool
	}{
		{
			name: "basic treemap",
			data: TreemapData{
				Title: "Budget Allocation",
				Nodes: []*TreemapNode{
					{Label: "Engineering", Value: 500000},
					{Label: "Marketing", Value: 300000},
					{Label: "Sales", Value: 200000},
					{Label: "Operations", Value: 150000},
				},
			},
			wantErr: false,
		},
		{
			name: "treemap with custom colors",
			data: TreemapData{
				Title: "Market Share",
				Nodes: []*TreemapNode{
					{Label: "Company A", Value: 45, Color: colorPtr(MustParseColor("#4E79A7"))},
					{Label: "Company B", Value: 30, Color: colorPtr(MustParseColor("#F28E2B"))},
					{Label: "Company C", Value: 15, Color: colorPtr(MustParseColor("#E15759"))},
					{Label: "Others", Value: 10, Color: colorPtr(MustParseColor("#76B7B2"))},
				},
			},
			wantErr: false,
		},
		{
			name: "hierarchical treemap",
			data: TreemapData{
				Title: "File Sizes",
				Nodes: []*TreemapNode{
					{
						Label: "src",
						Children: []*TreemapNode{
							{Label: "main.go", Value: 1000},
							{Label: "utils.go", Value: 500},
							{Label: "config.go", Value: 300},
						},
					},
					{
						Label: "test",
						Children: []*TreemapNode{
							{Label: "main_test.go", Value: 800},
							{Label: "utils_test.go", Value: 400},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty treemap",
			data:    TreemapData{},
			wantErr: true,
		},
		{
			name: "treemap with zero total value",
			data: TreemapData{
				Nodes: []*TreemapNode{
					{Label: "A", Value: 0},
					{Label: "B", Value: 0},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSVGBuilder(800, 600)
			config := DefaultTreemapChartConfig(800, 600)
			config.ShowLabels = true
			config.ShowValueLabels = true

			chart := NewTreemapChart(builder, config)
			err := chart.Draw(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("TreemapChart.Draw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				doc, err := builder.Render()
				if err != nil {
					t.Errorf("Failed to render SVG: %v", err)
					return
				}

				if doc == nil || len(doc.Content) == 0 {
					t.Error("Expected non-empty SVG document")
				}
			}
		})
	}
}

func TestTreemapNode_TotalValue(t *testing.T) {
	tests := []struct {
		name     string
		node     *TreemapNode
		expected float64
	}{
		{
			name:     "leaf node",
			node:     &TreemapNode{Label: "A", Value: 100},
			expected: 100,
		},
		{
			name: "parent with children",
			node: &TreemapNode{
				Label: "Parent",
				Children: []*TreemapNode{
					{Label: "Child1", Value: 30},
					{Label: "Child2", Value: 20},
					{Label: "Child3", Value: 50},
				},
			},
			expected: 100,
		},
		{
			name: "nested children",
			node: &TreemapNode{
				Label: "Root",
				Children: []*TreemapNode{
					{
						Label: "Branch1",
						Children: []*TreemapNode{
							{Label: "Leaf1", Value: 25},
							{Label: "Leaf2", Value: 25},
						},
					},
					{Label: "Leaf3", Value: 50},
				},
			},
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.TotalValue()
			if got != tt.expected {
				t.Errorf("TreemapNode.TotalValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTreemapDiagram_Validate(t *testing.T) {
	diagram := &TreemapDiagram{NewBaseDiagram("treemap_chart")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid with nodes",
			req: &RequestEnvelope{
				Type: "treemap_chart",
				Data: map[string]any{
					"nodes": []any{
						map[string]any{"label": "A", "value": 100},
						map[string]any{"label": "B", "value": 80},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with values",
			req: &RequestEnvelope{
				Type: "treemap_chart",
				Data: map[string]any{
					"values":     []any{100.0, 80.0, 60.0},
					"categories": []any{"A", "B", "C"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &RequestEnvelope{
				Type: "treemap_chart",
				Data: nil,
			},
			wantErr: true,
		},
		{
			name: "missing values and nodes",
			req: &RequestEnvelope{
				Type: "treemap_chart",
				Data: map[string]any{
					"categories": []any{"A", "B"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("TreemapDiagram.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTreemapDiagram_Render(t *testing.T) {
	diagram := &TreemapDiagram{NewBaseDiagram("treemap_chart")}

	req := &RequestEnvelope{
		Type:  "treemap_chart",
		Title: "Test Treemap",
		Data: map[string]any{
			"nodes": []any{
				map[string]any{"label": "Category A", "value": 400},
				map[string]any{"label": "Category B", "value": 300},
				map[string]any{"label": "Category C", "value": 200},
				map[string]any{"label": "Category D", "value": 100},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("TreemapDiagram.Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if doc.Width != 1067 || doc.Height != 800 {
		t.Errorf("Expected dimensions 1067x800 (800x600pt in CSS pixels), got %vx%v", doc.Width, doc.Height)
	}
}

func TestTreemapDiagram_Render_LabelValuePoints(t *testing.T) {
	// This tests the format produced by buildLabelValuePoints in chartutil:
	// "values": [{"label": "X", "value": 100}, {"label": "Y", "value": 200}]
	// This was the root cause of go-slide-creator-wionh (treemap "Data unavailable").
	diagram := &TreemapDiagram{NewBaseDiagram("treemap_chart")}

	req := &RequestEnvelope{
		Type:  "treemap_chart",
		Title: "Budget Allocation",
		Data: map[string]any{
			"values": []any{
				map[string]any{"label": "Engineering", "value": 500000.0},
				map[string]any{"label": "Marketing", "value": 300000.0},
				map[string]any{"label": "Sales", "value": 200000.0},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("TreemapDiagram.Render() with label-value points error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}
}

func TestTreemapDiagram_Render_MapSlice(t *testing.T) {
	// This tests the actual Go type produced by buildLabelValuePoints: []map[string]any
	// (not []any). Go's type system doesn't allow asserting []map[string]any as []any.
	diagram := &TreemapDiagram{NewBaseDiagram("treemap_chart")}

	req := &RequestEnvelope{
		Type:  "treemap_chart",
		Title: "Budget Allocation",
		Data: map[string]any{
			"values": []map[string]any{
				{"label": "Engineering", "value": 500000.0},
				{"label": "Marketing", "value": 300000.0},
				{"label": "Sales", "value": 200000.0},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("TreemapDiagram.Render() with []map[string]any error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}
}

func TestTreemapSquarifyLayout(t *testing.T) {
	builder := NewSVGBuilder(800, 600)
	config := DefaultTreemapChartConfig(800, 600)
	chart := NewTreemapChart(builder, config)

	// Test that squarify produces reasonable aspect ratios
	nodes := []*TreemapNode{
		{Label: "A", Value: 600},
		{Label: "B", Value: 300},
		{Label: "C", Value: 100},
	}

	bounds := Rect{X: 0, Y: 0, W: 800, H: 600}
	totalValue := 1000.0

	chart.squarifyLayout(nodes, bounds, totalValue)

	// Verify all nodes have valid bounds
	for _, node := range nodes {
		if node.bounds.W <= 0 || node.bounds.H <= 0 {
			t.Errorf("Node %s has invalid bounds: %v", node.Label, node.bounds)
		}

		// Check aspect ratio is reasonable (not too skinny)
		aspectRatio := node.bounds.W / node.bounds.H
		if aspectRatio < 0.1 || aspectRatio > 10 {
			t.Errorf("Node %s has extreme aspect ratio: %v", node.Label, aspectRatio)
		}
	}
}

// TestTreemapChart_SmallCellLabelsVisible is a regression test for pptx-6ly:
// the 5th (smallest) cell in a 5-node treemap must still have its label rendered
// even when the cell dimensions are modest.
func TestTreemapChart_SmallCellLabelsVisible(t *testing.T) {
	// Use descending values so the 5th node is the smallest cell.
	data := TreemapData{
		Title: "Market Segments",
		Nodes: []*TreemapNode{
			{Label: "Enterprise", Value: 50},
			{Label: "Mid-Market", Value: 30},
			{Label: "SMB", Value: 20},
			{Label: "Startup", Value: 15},
			{Label: "Consumer", Value: 10},
		},
	}

	builder := NewSVGBuilder(800, 600)
	config := DefaultTreemapChartConfig(800, 600)
	config.ShowLabels = true
	config.ShowValueLabels = false

	chart := NewTreemapChart(builder, config)
	if err := chart.Draw(data); err != nil {
		t.Fatalf("Draw() error: %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	svg := string(doc.Content)
	for _, node := range data.Nodes {
		if !strings.Contains(svg, node.Label) {
			t.Errorf("label %q missing from SVG output — smallest cells must still be labeled", node.Label)
		}
	}
}

// TestTreemapChart_AbbreviatedLabelFallback is a regression test for pptx-ylk:
// on a small canvas where the smallest cell falls below LabelMinSize, the cell
// must still show at least an abbreviated label (first 2 characters).
func TestTreemapChart_AbbreviatedLabelFallback(t *testing.T) {
	data := TreemapData{
		Nodes: []*TreemapNode{
			{Label: "Enterprise", Value: 50},
			{Label: "Mid-Market", Value: 30},
			{Label: "SMB", Value: 20},
			{Label: "Startup", Value: 15},
			{Label: "Consumer", Value: 10},
		},
	}

	// Use a small canvas so the smallest cell is below LabelMinSize but >= 8px.
	builder := NewSVGBuilder(120, 90)
	config := DefaultTreemapChartConfig(120, 90)
	config.ShowLabels = true
	config.ShowValueLabels = false
	config.ShowTitle = false
	config.MarginTop = 2
	config.MarginBottom = 2
	config.MarginLeft = 2
	config.MarginRight = 2

	chart := NewTreemapChart(builder, config)
	if err := chart.Draw(data); err != nil {
		t.Fatalf("Draw() error: %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	svg := string(doc.Content)

	// Every node must have at least its first 2 characters (abbreviated label)
	// or full label present in the SVG output.
	for _, node := range data.Nodes {
		abbrev := node.Label
		if len(abbrev) > 2 {
			abbrev = abbrev[:2]
		}
		if !strings.Contains(svg, abbrev) {
			t.Errorf("abbreviated label %q (from %q) missing from SVG — small cells must show abbreviated labels", abbrev, node.Label)
		}
	}
}

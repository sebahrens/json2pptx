// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestEstimateDiagramComplexity verifies complexity estimation for each diagram type.
func TestEstimateDiagramComplexity(t *testing.T) {
	tests := []struct {
		name       string
		spec       *types.DiagramSpec
		wantAtLeast int
		wantAtMost  int
	}{
		{
			name: "nil spec",
			spec: nil,
			wantAtLeast: 0,
			wantAtMost:  0,
		},
		{
			name: "empty data",
			spec: &types.DiagramSpec{
				Type: "org_chart",
				Data: map[string]any{},
			},
			wantAtLeast: 0,
			wantAtMost:  0,
		},
		{
			name: "org chart with 3 nodes",
			spec: &types.DiagramSpec{
				Type: "org_chart",
				Data: map[string]any{
					"root": map[string]any{
						"name": "CEO",
						"children": []any{
							map[string]any{"name": "VP Engineering"},
							map[string]any{"name": "VP Sales"},
						},
					},
				},
			},
			wantAtLeast: 3,
			wantAtMost:  3,
		},
		{
			name: "org chart with 10 nodes (deep tree)",
			spec: &types.DiagramSpec{
				Type: "org_chart",
				Data: map[string]any{
					"root": map[string]any{
						"name": "CEO",
						"children": []any{
							map[string]any{
								"name": "VP Eng",
								"children": []any{
									map[string]any{"name": "Dir Frontend"},
									map[string]any{"name": "Dir Backend"},
									map[string]any{"name": "Dir DevOps"},
								},
							},
							map[string]any{
								"name": "VP Sales",
								"children": []any{
									map[string]any{"name": "Sales East"},
									map[string]any{"name": "Sales West"},
									map[string]any{"name": "Sales EMEA"},
								},
							},
							map[string]any{"name": "CFO"},
						},
					},
				},
			},
			wantAtLeast: 10,
			wantAtMost:  10,
		},
		{
			name: "fishbone with 12 causes",
			spec: &types.DiagramSpec{
				Type: "fishbone",
				Data: map[string]any{
					"effect": "Product Defects",
					"categories": []any{
						map[string]any{
							"name":   "People",
							"causes": []any{"Training", "Motivation", "Staffing"},
						},
						map[string]any{
							"name":   "Process",
							"causes": []any{"Documentation", "Review", "Testing"},
						},
						map[string]any{
							"name":   "Materials",
							"causes": []any{"Quality", "Sourcing", "Storage"},
						},
						map[string]any{
							"name":   "Equipment",
							"causes": []any{"Calibration", "Maintenance", "Age"},
						},
					},
				},
			},
			wantAtLeast: 12,
			wantAtMost:  12,
		},
		{
			name: "swot with items in map form",
			spec: &types.DiagramSpec{
				Type: "swot",
				Data: map[string]any{
					"strengths":     map[string]any{"items": []any{"Brand", "Team", "Tech"}},
					"weaknesses":    map[string]any{"items": []any{"Cash", "Scale"}},
					"opportunities": map[string]any{"items": []any{"Market", "Expansion", "AI"}},
					"threats":       map[string]any{"items": []any{"Competition", "Regulation"}},
				},
			},
			wantAtLeast: 10,
			wantAtMost:  10,
		},
		{
			name: "swot with items as direct arrays",
			spec: &types.DiagramSpec{
				Type: "swot",
				Data: map[string]any{
					"strengths":     []any{"A", "B", "C"},
					"weaknesses":    []any{"D", "E"},
					"opportunities": []any{"F", "G", "H"},
					"threats":       []any{"I", "J"},
				},
			},
			wantAtLeast: 10,
			wantAtMost:  10,
		},
		{
			name: "heatmap 4x5",
			spec: &types.DiagramSpec{
				Type: "heatmap",
				Data: map[string]any{
					"row_labels": []any{"Q1", "Q2", "Q3", "Q4"},
					"col_labels": []any{"Product A", "Product B", "Product C", "Product D", "Product E"},
					"values": []any{
						[]any{1.0, 2.0, 3.0, 4.0, 5.0},
						[]any{2.0, 3.0, 4.0, 5.0, 6.0},
						[]any{3.0, 4.0, 5.0, 6.0, 7.0},
						[]any{4.0, 5.0, 6.0, 7.0, 8.0},
					},
				},
			},
			wantAtLeast: 20, // 4 rows * 5 cols
			wantAtMost:  20,
		},
		{
			name: "heatmap from values matrix only",
			spec: &types.DiagramSpec{
				Type: "heatmap",
				Data: map[string]any{
					"values": []any{
						[]any{1.0, 2.0, 3.0},
						[]any{4.0, 5.0, 6.0},
					},
				},
			},
			wantAtLeast: 6, // 2 rows * 3 cols
			wantAtMost:  6,
		},
		{
			name: "gantt with 10 tasks",
			spec: &types.DiagramSpec{
				Type: "gantt",
				Data: map[string]any{
					"tasks": []any{
						"Task 1", "Task 2", "Task 3", "Task 4", "Task 5",
						"Task 6", "Task 7", "Task 8", "Task 9", "Task 10",
					},
				},
			},
			wantAtLeast: 10,
			wantAtMost:  10,
		},
		{
			name: "kpi dashboard with 6 cards",
			spec: &types.DiagramSpec{
				Type: "kpi_dashboard",
				Data: map[string]any{
					"cards": []any{
						map[string]any{"title": "Revenue"},
						map[string]any{"title": "Users"},
						map[string]any{"title": "Churn"},
						map[string]any{"title": "NPS"},
						map[string]any{"title": "MRR"},
						map[string]any{"title": "ARR"},
					},
				},
			},
			wantAtLeast: 6,
			wantAtMost:  6,
		},
		{
			name: "simple bar chart (not complex type)",
			spec: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"labels": []any{"A", "B", "C"},
					"values": []any{10, 20, 30},
				},
			},
			// bar_chart uses countGenericItems
			wantAtLeast: 1,
			wantAtMost:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateDiagramComplexity(tt.spec)
			if got < tt.wantAtLeast {
				t.Errorf("estimateDiagramComplexity() = %d, want at least %d", got, tt.wantAtLeast)
			}
			if got > tt.wantAtMost {
				t.Errorf("estimateDiagramComplexity() = %d, want at most %d", got, tt.wantAtMost)
			}
		})
	}
}

// TestCheckDiagramInNarrowPlaceholder_WarningEmitted verifies that a quality
// warning is emitted when a complex diagram is placed in a narrow placeholder.
func TestCheckDiagramInNarrowPlaceholder_WarningEmitted(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	// Narrow placeholder: ~50% of slide width (6,096,000 EMU = half of 12,192,000)
	narrowWidth := int64(6096000)

	spec := &types.DiagramSpec{
		Type: "org_chart",
		Data: map[string]any{
			"root": map[string]any{
				"name": "CEO",
				"children": []any{
					map[string]any{
						"name": "VP1",
						"children": []any{
							map[string]any{"name": "M1"},
							map[string]any{"name": "M2"},
							map[string]any{"name": "M3"},
						},
					},
					map[string]any{
						"name": "VP2",
						"children": []any{
							map[string]any{"name": "M4"},
							map[string]any{"name": "M5"},
							map[string]any{"name": "M6"},
						},
					},
				},
			},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, narrowWidth, "Content Placeholder 2")

	if len(ctx.warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(ctx.warnings), ctx.warnings)
	}

	w := ctx.warnings[0]
	if !strings.Contains(w, "org_chart") {
		t.Errorf("warning should mention diagram type, got: %s", w)
	}
	if !strings.Contains(w, "narrow placeholder") {
		t.Errorf("warning should mention narrow placeholder, got: %s", w)
	}
	if !strings.Contains(w, "illegible") {
		t.Errorf("warning should mention illegibility, got: %s", w)
	}
	if !strings.Contains(w, "full-width layout") {
		t.Errorf("warning should suggest full-width layout, got: %s", w)
	}
}

// TestCheckDiagramInNarrowPlaceholder_NoWarningForWideLayout verifies that
// no warning is emitted when a complex diagram is in a full-width placeholder.
func TestCheckDiagramInNarrowPlaceholder_NoWarningForWideLayout(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	// Full-width placeholder: ~90% of slide width
	fullWidth := int64(10972800) // ~90% of 12,192,000

	spec := &types.DiagramSpec{
		Type: "org_chart",
		Data: map[string]any{
			"root": map[string]any{
				"name": "CEO",
				"children": []any{
					map[string]any{"name": "VP1"},
					map[string]any{"name": "VP2"},
					map[string]any{"name": "VP3"},
					map[string]any{"name": "VP4"},
					map[string]any{"name": "VP5"},
					map[string]any{"name": "VP6"},
					map[string]any{"name": "VP7"},
					map[string]any{"name": "VP8"},
					map[string]any{"name": "VP9"},
				},
			},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, fullWidth, "Content Placeholder 1")

	if len(ctx.warnings) != 0 {
		t.Errorf("expected no warnings for full-width layout, got %d: %v", len(ctx.warnings), ctx.warnings)
	}
}

// TestCheckDiagramInNarrowPlaceholder_NoWarningForSimpleDiagram verifies that
// no warning is emitted for a simple diagram (below complexity threshold) even
// in a narrow placeholder.
func TestCheckDiagramInNarrowPlaceholder_NoWarningForSimpleDiagram(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	narrowWidth := int64(6096000)

	// Simple org chart with only 3 nodes (below threshold of 8)
	spec := &types.DiagramSpec{
		Type: "org_chart",
		Data: map[string]any{
			"root": map[string]any{
				"name": "CEO",
				"children": []any{
					map[string]any{"name": "VP1"},
					map[string]any{"name": "VP2"},
				},
			},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, narrowWidth, "Content Placeholder 2")

	if len(ctx.warnings) != 0 {
		t.Errorf("expected no warnings for simple diagram, got %d: %v", len(ctx.warnings), ctx.warnings)
	}
}

// TestCheckDiagramInNarrowPlaceholder_NoWarningForNonComplexType verifies that
// non-complex diagram types (like bar_chart) do not trigger warnings even when
// they have many data points in a narrow placeholder.
func TestCheckDiagramInNarrowPlaceholder_NoWarningForNonComplexType(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	narrowWidth := int64(6096000)

	spec := &types.DiagramSpec{
		Type: "bar_chart",
		Data: map[string]any{
			"labels": []any{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"},
			"values": []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, narrowWidth, "Content Placeholder 2")

	if len(ctx.warnings) != 0 {
		t.Errorf("expected no warnings for bar_chart type, got %d: %v", len(ctx.warnings), ctx.warnings)
	}
}

// TestCheckDiagramInNarrowPlaceholder_FishboneComplex verifies the warning is
// emitted for a complex fishbone diagram in a narrow placeholder.
func TestCheckDiagramInNarrowPlaceholder_FishboneComplex(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	narrowWidth := int64(5500000) // ~45% of slide width

	spec := &types.DiagramSpec{
		Type: "fishbone",
		Data: map[string]any{
			"effect": "Quality Issues",
			"categories": []any{
				map[string]any{"name": "People", "causes": []any{"A", "B", "C"}},
				map[string]any{"name": "Process", "causes": []any{"D", "E", "F"}},
				map[string]any{"name": "Materials", "causes": []any{"G", "H"}},
				map[string]any{"name": "Equipment", "causes": []any{"I", "J"}},
			},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, narrowWidth, "1")

	if len(ctx.warnings) != 1 {
		t.Fatalf("expected 1 warning for complex fishbone, got %d: %v", len(ctx.warnings), ctx.warnings)
	}
	if !strings.Contains(ctx.warnings[0], "fishbone") {
		t.Errorf("warning should mention fishbone, got: %s", ctx.warnings[0])
	}
}

// TestCheckDiagramInNarrowPlaceholder_SWOTComplex verifies the warning for SWOT.
func TestCheckDiagramInNarrowPlaceholder_SWOTComplex(t *testing.T) {
	ctx := &singlePassContext{
		OutputContext: OutputContext{},
	}

	narrowWidth := int64(5500000)

	spec := &types.DiagramSpec{
		Type: "swot",
		Data: map[string]any{
			"strengths":     map[string]any{"items": []any{"S1", "S2", "S3"}},
			"weaknesses":    map[string]any{"items": []any{"W1", "W2"}},
			"opportunities": map[string]any{"items": []any{"O1", "O2", "O3"}},
			"threats":       map[string]any{"items": []any{"T1", "T2"}},
		},
	}

	ctx.checkDiagramInNarrowPlaceholder(1, spec, narrowWidth, "1")

	if len(ctx.warnings) != 1 {
		t.Fatalf("expected 1 warning for complex SWOT, got %d: %v", len(ctx.warnings), ctx.warnings)
	}
	if !strings.Contains(ctx.warnings[0], "swot") {
		t.Errorf("warning should mention swot, got: %s", ctx.warnings[0])
	}
}

// TestCountOrgChartNodes_EdgeCases tests org chart node counting edge cases.
func TestCountOrgChartNodes_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want int
	}{
		{
			name: "no root key",
			data: map[string]any{"title": "No Root"},
			want: 0,
		},
		{
			name: "root is not a map",
			data: map[string]any{"root": "just a string"},
			want: 0,
		},
		{
			name: "root with no children",
			data: map[string]any{
				"root": map[string]any{"name": "Sole Leader"},
			},
			want: 1,
		},
		{
			name: "children is not an array",
			data: map[string]any{
				"root": map[string]any{
					"name":     "Leader",
					"children": "invalid",
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countOrgChartNodes(tt.data)
			if got != tt.want {
				t.Errorf("countOrgChartNodes() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCountFishboneCauses_EdgeCases tests fishbone cause counting edge cases.
func TestCountFishboneCauses_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want int
	}{
		{
			name: "no categories key",
			data: map[string]any{"effect": "Problem"},
			want: 0,
		},
		{
			name: "categories is not array",
			data: map[string]any{"categories": "invalid"},
			want: 0,
		},
		{
			name: "category without causes counts itself",
			data: map[string]any{
				"categories": []any{
					map[string]any{"name": "People"},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countFishboneCauses(tt.data)
			if got != tt.want {
				t.Errorf("countFishboneCauses() = %d, want %d", got, tt.want)
			}
		})
	}
}

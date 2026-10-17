package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestConsultingNativeListContractsAndFit(t *testing.T) {
	for _, name := range []string{"checklist", "prose-2col", "ipo-flow", "three-horizons", "sources-list", "timeline-vertical", "spectrum", "issue-tree", "decision-tree", "progress-list"} {
		t.Run(name, func(t *testing.T) {
			p, ok := Default().Get(name)
			if !ok {
				t.Fatal("pattern missing")
			}
			raw, _ := json.Marshal(p.(interface{ ExemplarValues() any }).ExemplarValues())
			values := p.NewValues()
			if err := json.Unmarshal(raw, values); err != nil {
				t.Fatal(err)
			}
			if err := p.Validate(values, nil, nil); err != nil {
				t.Fatal(err)
			}
			grid, err := p.Expand(ExpandContext{}, values, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(grid.Rows) != 1 || len(grid.Rows[0].Cells[0].Layers) == 0 {
				t.Fatal("no native layers")
			}
			for _, layer := range grid.Rows[0].Cells[0].Layers {
				if layer.Frame.X < 0 || layer.Frame.Y < 0 || layer.Frame.X+layer.Frame.W > 1.00001 || layer.Frame.Y+layer.Frame.H > 1.00001 {
					t.Fatalf("out of bounds layer: %+v", layer)
				}
			}
			m := values.(*map[string]any)
			(*m)["unknown"] = true
			if err := p.Validate(values, nil, nil); err == nil {
				t.Fatal("unknown field accepted")
			}
		})
	}
}
func TestConsultingNativeRefusesMeasuredOverflow(t *testing.T) {
	p, _ := Default().Get("prose-2col")
	values := p.NewValues()
	_ = json.Unmarshal([]byte(`{"columns":[{"paragraphs":["`+strings.Repeat("Long evidence paragraph with supporting detail. ", 14)+`"]},{"paragraphs":["Other conclusion."]}]}`), values)
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 4000000, Height: 1000000}}
	if _, err := p.Expand(ctx, values, nil, nil); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("expected actionable refusal, got %v", err)
	}
}

func TestConsultingTreesRejectCyclesAndDeepTrees(t *testing.T) {
	p, _ := Default().Get("issue-tree")
	for _, raw := range []string{
		`{"nodes":[{"id":"a","parent":"b","label":"a"},{"id":"b","parent":"a","label":"b"},{"id":"c","label":"c"}]}`,
		`{"nodes":[{"id":"a","label":"a"},{"id":"b","label":"b"},{"id":"c","parent":"a","label":"c"}]}`,
		`{"nodes":[{"id":"a","label":"a"},{"id":"b","parent":"a","label":"b"},{"id":"c","parent":"b","label":"c"},{"id":"d","parent":"c","label":"d"},{"id":"e","parent":"d","label":"e"}]}`,
	} {
		v := p.NewValues()
		_ = json.Unmarshal([]byte(raw), v)
		if err := p.Validate(v, nil, nil); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestConsultingSpectrumDenseMarkersRefuseWithoutDropping(t *testing.T) {
	p, _ := Default().Get("spectrum")
	v := p.NewValues()
	_ = json.Unmarshal([]byte(`{"left_label":"Low","right_label":"High","markers":[{"label":"A","position":0.5},{"label":"B","position":0.5},{"label":"C","position":0.5},{"label":"D","position":0.5},{"label":"E","position":0.5},{"label":"F","position":0.5}]}`), v)
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 6000000, Height: 2000000}}
	if _, err := p.Expand(ctx, v, nil, nil); err == nil || !strings.Contains(err.Error(), "split") {
		t.Fatalf("expected bounded refusal, got %v", err)
	}
}

func TestConsultingChecklistTenShortItemsRemainReadable(t *testing.T) {
	p, _ := Default().Get("checklist")
	items := []any{}
	for i := 0; i < 10; i++ {
		items = append(items, map[string]any{"label": "Requirement", "state": "done"})
	}
	v := map[string]any{"items": items}
	grid, err := p.Expand(ExpandContext{}, &v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(grid)
	if count := strings.Count(string(raw), "Requirement"); count != 10 {
		t.Fatalf("wrote %d requirements", count)
	}
}

func TestConsultingProgressEightRowsNeedMeasuredHeight(t *testing.T) {
	p, _ := Default().Get("progress-list")
	rows := []any{}
	for i := 0; i < 8; i++ {
		rows = append(rows, map[string]any{"label": "KPI", "actual": 8.0, "target": 10.0})
	}
	v := map[string]any{"rows": rows}
	for _, height := range []int64{3733800, 4572000} {
		ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 11000000, Height: height}}
		grid, err := p.Expand(ctx, &v, nil, nil)
		if height == 3733800 {
			if err == nil || !strings.Contains(err.Error(), "44pt") {
				t.Fatalf("short frame needs actionable 44pt-row refusal, got %v", err)
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(grid)
			if strings.Count(string(raw), "target 10") != 8 {
				t.Fatal("an actual/target row was dropped")
			}
		}
	}
}

func TestConsultingMatrixIncludesTemplateGutterInFit(t *testing.T) {
	p, _ := Default().Get("raci-matrix")
	for _, tc := range []struct {
		name      string
		height    int64
		gutter    float64
		wantError bool
	}{{"gap-overflow", 138, 8, true}, {"fits-with-gap", 139, 8, false}, {"larger-template-gap", 139, 16, true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 11000000, Height: tc.height * 12700}, Metadata: &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: tc.gutter}}}
			values := p.(interface{ ExemplarValues() any }).ExemplarValues().(*map[string]any)
			(*values)["activities"] = (*values)["activities"].([]any)[:3]
			grid, err := p.Expand(ctx, values, nil, nil)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "split") {
					t.Fatalf("expected measured refusal, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			used := grid.Gap
			for _, row := range grid.Rows {
				used += row.MinHeight
			}
			if used > float64(tc.height) {
				t.Fatalf("rows and gap use %gpt in %dpt frame", used, tc.height)
			}
		})
	}
}

func TestConsultingHeatmapAxisAndValueRoles(t *testing.T) {
	p, _ := Default().Get("numeric-heatmap")
	grid, err := p.Expand(ExpandContext{}, p.(Exemplar).ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	axes, values := 0, 0
	for _, layer := range grid.Rows[0].Cells[0].Layers {
		if layer.Shape == nil || len(layer.Shape.Text) == 0 {
			continue
		}
		var text struct {
			Paragraphs []visualParagraph `json:"paragraphs"`
		}
		if err := json.Unmarshal(layer.Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		for _, run := range text.Paragraphs {
			if run.Content == "North" || run.Content == "South" || run.Content == "Q1" || run.Content == "Q2" {
				axes++
				if run.Size != 12 {
					t.Errorf("axis at%gpt", run.Size)
				}
			} else {
				values++
				if run.Content == "5" && run.Color != "#FFFFFF" {
					t.Errorf("dark numeric cell must use explicit white ink, got %s", run.Color)
				}
				if run.Content == "4" && run.Color != "#000000" {
					t.Errorf("medium numeric cell must use explicit black ink, got %s", run.Color)
				}
				if run.Size != 10 {
					t.Errorf("dense value at%gpt", run.Size)
				}
			}
		}
	}
	if axes != 4 || values != 4 {
		t.Fatalf("lost content: %d axes %d values", axes, values)
	}
}

func TestConsultingSpectrumAdvertisesSupportingPlacement(t *testing.T) {
	p, _ := Default().Get("spectrum")
	if !SegmentFitFor(p).SegmentOnly {
		t.Fatal("spectrum must advertise supporting placement")
	}
	if _, err := p.Expand(ExpandContext{LayoutBounds: LayoutBounds{Width: 9000000, Height: 2500000}}, p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestConsultingStakeholderPreservesGeometryAndReadableLabels(t *testing.T) {
	p, _ := Default().Get("stakeholder-map")
	values := p.(Exemplar).ExemplarValues()
	grid, err := p.Expand(ExpandContext{}, values, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	texts, markers := 0, 0
	for _, layer := range grid.Rows[0].Cells[0].Layers {
		if layer.Shape.Geometry == "ellipse" {
			markers++
		}
		if len(layer.Shape.Text) == 0 {
			continue
		}
		var text struct {
			Paragraphs []visualParagraph `json:"paragraphs"`
		}
		if err := json.Unmarshal(layer.Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		for _, run := range text.Paragraphs {
			texts++
			if run.Size != 12 {
				t.Errorf("%q at%gpt", run.Content, run.Size)
			}
		}
	}
	if texts != 8 || markers != 6 {
		t.Fatalf("lost axes/points/legend: %d texts %d markers", texts, markers)
	}
	v := values.(*map[string]any)
	points := visualItems(*v, "points")
	for _, raw := range points {
		point := visualObj(raw)
		point["x"] = 50.0
		point["y"] = 50.0
	}
	if _, err := p.Expand(ExpandContext{}, v, nil, nil); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected explicit coincident marker refusal: %v", err)
	}
}

func TestConsultingHeatmapExtremeFiniteRange(t *testing.T) {
	p, _ := Default().Get("numeric-heatmap")
	v := map[string]any{"row_labels": []any{"North", "South"}, "col_labels": []any{"A", "B"}, "values": []any{[]any{-1e308, 0.0}, []any{1e308, 1.0}}}
	grid, err := p.Expand(ExpandContext{}, &v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	fills := []string{}
	for _, layer := range grid.Rows[0].Cells[0].Layers {
		if len(layer.Shape.Text) == 0 {
			var fill string
			if err := json.Unmarshal(layer.Shape.Fill, &fill); err != nil {
				t.Fatal(err)
			}
			fills = append(fills, fill)
		}
	}
	if len(fills) != 4 || fills[0] != "#E8EEF5" || fills[2] != "#346C9B" {
		t.Fatalf("extremefinite range lost scale: %v", fills)
	}
}

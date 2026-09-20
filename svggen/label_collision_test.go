package svggen

import (
	"strings"
	"testing"
)

// TestGanttNamesEachTaskOnce pins the fix for go-slide-creator-kosq: the row
// label column and the bar both drew the task name, so every task was printed
// twice — and on a narrow bar the repeat was a truncated fragment of a name the
// reader had just read in full.
func TestGanttNamesEachTaskOnce(t *testing.T) {
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}
	req := &RequestEnvelope{
		Type:   "gantt",
		Output: OutputSpec{Width: 900, Height: 500},
		Data: map[string]any{
			"tasks": []any{
				map[string]any{"label": "Discovery", "start": "2026-01-01", "end": "2026-02-15"},
				map[string]any{"label": "Build wave one", "start": "2026-03-15", "end": "2026-07-01"},
			},
		},
	}
	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(doc.Content)
	for _, name := range []string{"Discovery", "Build wave one"} {
		if got := strings.Count(svg, name); got != 1 {
			t.Errorf("task %q appears %d times, want exactly one", name, got)
		}
	}
}

// TestTreemapParentLabelSurvivesItsChildren pins the header band: a parent's
// children are drawn on top of it, so a parent labelled in the middle of its
// own rect was painted over and the grouping had a name nobody could read.
func TestTreemapParentLabelSurvivesItsChildren(t *testing.T) {
	diagram := &TreemapDiagram{NewBaseDiagram("treemap")}
	req := &RequestEnvelope{
		Type:   "treemap",
		Output: OutputSpec{Width: 900, Height: 400},
		Data: map[string]any{
			"values": []any{
				map[string]any{"label": "Platform", "value": 60.0, "children": []any{
					map[string]any{"label": "Enterprise", "value": 40.0},
					map[string]any{"label": "SMB", "value": 20.0},
				}},
				map[string]any{"label": "Services", "value": 40.0, "children": []any{
					map[string]any{"label": "Professional", "value": 25.0},
					map[string]any{"label": "Training", "value": 15.0},
				}},
			},
		},
	}
	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(doc.Content)
	for _, name := range []string{"Platform", "Services", "Enterprise", "SMB", "Professional", "Training"} {
		if !strings.Contains(svg, name) {
			t.Errorf("label %q is missing from the treemap", name)
		}
	}
}

// TestTreemapHeaderHeight covers the band arithmetic: leaves get none, and a
// parent too short to spare the band keeps its full height for the children.
func TestTreemapHeaderHeight(t *testing.T) {
	style := DefaultStyleGuide()
	leaf := &TreemapNode{Label: "Leaf", Value: 10}
	parent := &TreemapNode{Label: "Parent", Children: []*TreemapNode{leaf}}

	if got := treemapHeaderHeight(leaf, Rect{W: 200, H: 200}, style); got != 0 {
		t.Errorf("a leaf needs no header band, got %.1f", got)
	}
	if got := treemapHeaderHeight(parent, Rect{W: 200, H: 200}, style); got <= 0 {
		t.Error("a parent with room should reserve a header band")
	}
	if got := treemapHeaderHeight(parent, Rect{W: 200, H: 20}, style); got != 0 {
		t.Errorf("a short parent should keep its height for the children, got %.1f", got)
	}
}

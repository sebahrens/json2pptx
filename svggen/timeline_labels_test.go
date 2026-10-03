package svggen

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// timelineExampleItems is examples/diagrams/timeline.json: six dated events,
// three of them within six weeks of each other, with 16–20 character labels.
func timelineExampleItems() []any {
	return []any{
		map[string]any{"date": "2024-01-15", "title": "UX Research & Design", "description": "User research and design phase"},
		map[string]any{"date": "2024-02-15", "title": "Backend Development", "description": "Core API and services"},
		map[string]any{"date": "2024-03-01", "title": "Frontend Development", "description": "UI implementation"},
		map[string]any{"date": "2024-05-01", "title": "Alpha Release", "description": "Internal testing milestone"},
		map[string]any{"date": "2024-06-15", "title": "Beta Release", "description": "External beta program"},
		map[string]any{"date": "2024-08-01", "title": "GA Launch", "description": "General availability"},
	}
}

func renderTimelineForLabels(t *testing.T, data map[string]any, w, h int) (string, []Finding) {
	t.Helper()
	req := &RequestEnvelope{Type: "timeline", Title: "Schedule", Data: data, Output: OutputSpec{Width: w, Height: h}}
	b, doc, err := (&Timeline{NewBaseDiagram("timeline")}).RenderWithBuilder(req)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(doc.String(), "&amp;", "&"), b.Findings()
}

func truncationFindings(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Code == FindingLabelTruncated {
			out = append(out, f)
		}
	}
	return out
}

// TestTimelineExampleLabelsShownInFull pins go-slide-creator-ze6qs: the
// example rendered "Frontend D…" and "Backend De…" beside an empty canvas.
// Every label is now drawn whole, at the frames a slide gives the diagram.
func TestTimelineExampleLabelsShownInFull(t *testing.T) {
	for _, size := range [][2]int{{800, 400}, {900, 420}, {1000, 380}, {720, 405}} {
		svg, findings := renderTimelineForLabels(t, map[string]any{"items": timelineExampleItems()}, size[0], size[1])
		for _, label := range []string{"UX Research & Design", "Backend Development", "Frontend Development", "Alpha Release", "Beta Release", "GA Launch"} {
			for _, word := range strings.Fields(label) {
				if !strings.Contains(svg, word+"<") && !strings.Contains(svg, word+" ") {
					t.Errorf("%dx%d: label %q is not drawn in full (lost %q)", size[0], size[1], label, word)
				}
			}
		}
		if strings.Contains(svg, "…") {
			t.Errorf("%dx%d: the timeline draws an ellipsis", size[0], size[1])
		}
		if got := truncationFindings(findings); len(got) > 0 {
			t.Errorf("%dx%d: unexpected truncation findings: %+v", size[0], size[1], got)
		}
	}
}

// TestTimelineLabelsWrapBeforeShrinkingOrTruncating: labels too wide for one
// line even staggered take two lines at the body size.
func TestTimelineLabelsWrapBeforeShrinkingOrTruncating(t *testing.T) {
	items := []any{
		map[string]any{"date": "2026-01-01", "title": "Regulatory submission filed", "type": "milestone"},
		map[string]any{"date": "2026-02-01", "title": "Supervisory board approval", "type": "milestone"},
		map[string]any{"date": "2026-03-01", "title": "Customer migration complete", "type": "milestone"},
		map[string]any{"date": "2026-04-01", "title": "Legacy platform retirement", "type": "milestone"},
		map[string]any{"date": "2026-05-01", "title": "Post-launch review signed", "type": "milestone"},
	}
	svg, findings := renderTimelineForLabels(t, map[string]any{"items": items}, 440, 320)
	if got := truncationFindings(findings); len(got) > 0 {
		t.Fatalf("labels were shortened although two lines fit: %+v", got)
	}
	if strings.Contains(svg, "…") {
		t.Fatal("the timeline draws an ellipsis")
	}
	// Every word survives, and at least one label is split over two lines.
	wrapped := false
	for _, it := range items {
		label := it.(map[string]any)["title"].(string)
		if strings.Contains(svg, ">"+label+"<") {
			continue
		}
		wrapped = true
		for _, word := range strings.Fields(label) {
			if !strings.Contains(svg, word) {
				t.Errorf("label %q lost the word %q", label, word)
			}
		}
	}
	if !wrapped {
		t.Error("expected at least one label on two lines at this width")
	}
}

// TestTimelineShortenedLabelBlocks: a label that cannot be shown whole by any
// placement is reported at a blocking severity with the item's path, not as an
// advisory note beside a silent ellipsis.
func TestTimelineShortenedLabelBlocks(t *testing.T) {
	var items []any
	for _, day := range []string{"01", "02", "03", "04", "05", "06", "07", "08"} {
		items = append(items, map[string]any{"date": "2026-01-" + day, "title": "Unbreakablemilestonelabelnumber" + day, "type": "milestone"})
	}
	svg, findings := renderTimelineForLabels(t, map[string]any{"items": items}, 400, 300)
	got := truncationFindings(findings)
	if len(got) == 0 {
		t.Fatal("no finding for labels that cannot be shown in full")
	}
	if !strings.Contains(svg, "…") {
		t.Error("expected the shortened labels to carry an ellipsis")
	}
	for _, f := range got {
		if f.Severity != core.SeverityShrinkOrSplit {
			t.Errorf("truncation finding has severity %q, want %q: %s", f.Severity, core.SeverityShrinkOrSplit, f.Message)
		}
		if !strings.HasPrefix(f.Field, "data.items[") {
			t.Errorf("truncation finding has field %q, want the item path", f.Field)
		}
	}

	// The same input reports the same findings without drawing: validate and
	// render agree.
	dry, err := DryRender(&RequestEnvelope{Type: "timeline", Title: "Schedule", Data: map[string]any{"items": items}, Output: OutputSpec{Width: 400, Height: 300}})
	if err != nil {
		t.Fatal(err)
	}
	if len(truncationFindings(dry)) != len(got) {
		t.Errorf("dry render reports %d truncations, render %d", len(truncationFindings(dry)), len(got))
	}
}

// TestTimelineSidePositionTruncationBlocks: the label positions the planner
// does not lay out report a shortened label at the same severity.
func TestTimelineSidePositionTruncationBlocks(t *testing.T) {
	items := []any{
		map[string]any{"title": "A very long workstream name that cannot fit beside its bar at all", "start": "2026-01-01", "end": "2026-12-20"},
		map[string]any{"title": "Another equally long workstream name that will not fit either", "start": "2026-01-05", "end": "2026-12-28"},
	}
	_, findings := renderTimelineForLabels(t, map[string]any{"items": items, "label_position": "right"}, 400, 300)
	got := truncationFindings(findings)
	if len(got) == 0 {
		t.Fatal("no finding for labels shortened beside their bars")
	}
	for _, f := range got {
		if f.Severity != core.SeverityShrinkOrSplit {
			t.Errorf("truncation finding has severity %q, want %q", f.Severity, core.SeverityShrinkOrSplit)
		}
	}
}

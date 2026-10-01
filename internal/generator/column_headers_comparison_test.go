package generator

import "testing"

// TestComparisonColumnsAlwaysGetHeaders pins go-slide-creator-gndpw: on a
// slide_type "comparison" slide the first bullet of each column is the option
// name and renders as a bold header even when the points under it are as
// short as the name — the length heuristic would leave them plain bullets.
func TestComparisonColumnsAlwaysGetHeaders(t *testing.T) {
	shapes := []shapeXML{columnShape("body", 400000), columnShape("body_2", 6000000)}
	content := []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Acquisition wins on speed; greenfield wins on control"},
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Greenfield", "Full process control", "Slower ramp"}},
		{PlaceholderID: "body_2", Type: ContentBullets, Value: []string{"Acquisition", "Existing customer base", "Integration risk"}},
	}
	if _, ok := markColumnHeaders(shapes, content)[1].Value.([]string); !ok {
		t.Fatal("precondition: the plain two-column heuristic should leave short columns unmarked")
	}
	content[1].ColumnHeader, content[2].ColumnHeader = true, true
	got := markColumnHeaders(shapes, content)
	for i, want := range map[int]string{1: "Greenfield", 2: "Acquisition"} {
		h, ok := got[i].Value.(columnHeaderBullets)
		if !ok || h.Header != want || len(h.Bullets) != 2 {
			t.Errorf("comparison column %d: got %#v, want header %q", i, got[i].Value, want)
		}
	}
}

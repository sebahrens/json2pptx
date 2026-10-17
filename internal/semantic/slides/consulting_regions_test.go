package slides

import (
	"encoding/json"
	"testing"
)

func TestRegionVerticalTimelinePreservesStops(t *testing.T) {
	r := map[string]any{"kind": "timeline", "orientation": "vertical", "milestones": []any{map[string]any{"label": "Align", "date": "Q1"}, map[string]any{"label": "Pilot", "date": "Q2"}, map[string]any{"label": "Scale", "date": "Q3"}}}
	b, err := regionTimeline(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(b.cell)
	var cell map[string]any
	_ = json.Unmarshal(raw, &cell)
	if got := RegionPatterns(map[string]any{"regions": []any{r}}); len(got) != 1 || got[0] != "timeline-vertical" {
		t.Fatalf("patterns %v", got)
	}
	if len(raw) == 0 || cell == nil {
		t.Fatal("empty native pattern cell")
	}
	r["orientation"] = "diagonal"
	if _, err := regionTimeline(r); err == nil {
		t.Fatal("invalid orientation degraded")
	}
	r["orientation"] = "vertical"
	r["milestones"] = []any{"Align", "Pilot", "Scale"}
	if _, err := regionTimeline(r); err == nil {
		t.Fatal("missing dates dropped")
	}
}

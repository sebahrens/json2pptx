package svggen

import "testing"

func TestTimelineBelowLabelsLeaveRoomForDescriptions(t *testing.T) {
	items := []any{
		map[string]any{"date": "2024-01-15", "title": "UX Research & Design", "description": "User research and design phase"},
		map[string]any{"date": "2024-02-15", "title": "Backend Development", "description": "Core API and services"},
		map[string]any{"date": "2024-03-01", "title": "Frontend Development", "description": "UI implementation"},
		map[string]any{"date": "2024-05-01", "title": "Alpha Release", "description": "Internal testing milestone"},
		map[string]any{"date": "2024-06-15", "title": "Beta Release", "description": "External beta program"},
		map[string]any{"date": "2024-08-01", "title": "GA Launch", "description": "General availability"},
	}
	findings, err := DryRender(&RequestEnvelope{Type: "timeline", Data: map[string]any{"items": items}})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if finding.Code == FindingDiagramTextOverlap {
			t.Errorf("timeline label/description collision: %+v", finding)
		}
	}
}

package svggen

import (
	"os"
	"testing"
)

func TestDebugTimelineRender(t *testing.T) {
	if os.Getenv("DEBUG_RENDER") == "" {
		t.Skip("set DEBUG_RENDER=1 to run")
	}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Implementation Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":        "2026 Q1",
					"title":       "Phase 1: Discovery",
					"description": "Requirements gathering and analysis",
				},
				map[string]any{
					"date":        "2026 Q2",
					"title":       "Phase 2: Development",
					"description": "Core feature implementation",
				},
				map[string]any{
					"date":        "2026 Q3",
					"title":       "Phase 3: Testing",
					"description": "QA and user acceptance testing",
				},
				map[string]any{
					"date":        "2026 Q4",
					"title":       "Phase 4: Launch",
					"description": "Production deployment and rollout",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	d := &Timeline{}
	builder, svgDoc, err := d.RenderWithBuilder(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	pngData, err := builder.RenderPNG(2.0)
	if err != nil {
		t.Fatalf("png: %v", err)
	}

	os.WriteFile("/tmp/timeline_test.png", pngData, 0644)
	os.WriteFile("/tmp/timeline_test.svg", svgDoc.Bytes(), 0644)
	t.Log("wrote /tmp/timeline_test.png and /tmp/timeline_test.svg")

	// Also test the overlapping activities + milestones variant
	req2 := &RequestEnvelope{
		Type: "timeline",
		Data: map[string]any{
			"title": "Product Roadmap",
			"activities": []any{
				map[string]any{"label": "Discovery", "start_date": "2024-01-01", "end_date": "2024-03-01"},
				map[string]any{"label": "Design", "start_date": "2024-02-15", "end_date": "2024-05-01"},
				map[string]any{"label": "Development", "start_date": "2024-04-01", "end_date": "2024-09-01"},
				map[string]any{"label": "Testing", "start_date": "2024-08-01", "end_date": "2024-10-15"},
				map[string]any{"label": "Launch Prep", "start_date": "2024-10-01", "end_date": "2024-12-01"},
			},
			"milestones": []any{
				map[string]any{"label": "Kickoff", "date": "2024-01-01"},
				map[string]any{"label": "Alpha", "date": "2024-06-01"},
				map[string]any{"label": "Beta", "date": "2024-09-01"},
				map[string]any{"label": "GA", "date": "2024-12-01"},
			},
		},
		Output: OutputSpec{Preset: "content_16x9"},
	}

	builder2, svgDoc2, err := d.RenderWithBuilder(req2)
	if err != nil {
		t.Fatalf("render multi-row: %v", err)
	}
	pngData2, err := builder2.RenderPNG(2.0)
	if err != nil {
		t.Fatalf("png multi-row: %v", err)
	}
	os.WriteFile("/tmp/timeline_multirow.png", pngData2, 0644)
	os.WriteFile("/tmp/timeline_multirow.svg", svgDoc2.Bytes(), 0644)
	t.Log("wrote /tmp/timeline_multirow.png and /tmp/timeline_multirow.svg")
}


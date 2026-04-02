package testrand

import (
	"encoding/json"
	"testing"
)

func TestDeterministic(t *testing.T) {
	seed := uint64(42)
	g1 := New(seed)
	g2 := New(seed)

	j1, err := g1.GenerateJSON()
	if err != nil {
		t.Fatalf("GenerateJSON (1): %v", err)
	}
	j2, err := g2.GenerateJSON()
	if err != nil {
		t.Fatalf("GenerateJSON (2): %v", err)
	}

	if string(j1) != string(j2) {
		t.Fatal("same seed produced different JSON — not deterministic")
	}
}

func TestDifferentSeeds(t *testing.T) {
	j1, _ := New(1).GenerateJSON()
	j2, _ := New(2).GenerateJSON()

	if string(j1) == string(j2) {
		t.Fatal("different seeds produced identical JSON")
	}
}

func TestValidJSON(t *testing.T) {
	seeds := []uint64{0, 1, 42, 100, 999, 12345, 99999}
	for _, seed := range seeds {
		t.Run("", func(t *testing.T) {
			data, err := New(seed).GenerateJSON()
			if err != nil {
				t.Fatalf("seed %d: GenerateJSON error: %v", seed, err)
			}

			var p PresentationInput
			if err := json.Unmarshal(data, &p); err != nil {
				t.Fatalf("seed %d: invalid JSON: %v", seed, err)
			}

			if p.Template == "" {
				t.Errorf("seed %d: empty template", seed)
			}
			if len(p.Slides) == 0 {
				t.Errorf("seed %d: no slides", seed)
			}
			if len(p.Slides) > 30 {
				t.Errorf("seed %d: too many slides: %d", seed, len(p.Slides))
			}

			// First slide must be title
			if p.Slides[0].SlideType != "title" {
				t.Errorf("seed %d: first slide type = %q, want title", seed, p.Slides[0].SlideType)
			}

			// Every slide must have content
			for i, s := range p.Slides {
				if len(s.Content) == 0 {
					t.Errorf("seed %d: slide %d has no content", seed, i)
				}
			}
		})
	}
}

func TestContentTypes(t *testing.T) {
	// Run enough seeds to see all content types
	types := make(map[string]bool)
	for seed := uint64(0); seed < 200; seed++ {
		p := New(seed).Generate()
		for _, s := range p.Slides {
			for _, c := range s.Content {
				types[c.Type] = true
			}
		}
	}

	expected := []string{"text", "bullets", "body_and_bullets", "bullet_groups", "table", "chart", "diagram", "image"}
	for _, e := range expected {
		if !types[e] {
			t.Errorf("content type %q never generated in 200 seeds", e)
		}
	}
}

func TestEdgeCasesAppear(t *testing.T) {
	// With 10% edge case chance, should see at least one in many seeds
	foundEdge := false
	for seed := uint64(0); seed < 100; seed++ {
		data, _ := New(seed).GenerateJSON()
		s := string(data)
		// Check for known edge case markers
		if contains(s, "日本語") || contains(s, "🚀") || contains(s, "<script>") ||
			contains(s, "مرحبا") || contains(s, "AAAAAAA") {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		t.Error("no edge case strings found in 100 seeds")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestChartDataStructure(t *testing.T) {
	// Verify chart data is valid for each chart type
	g := New(42)
	for _, ct := range chartTypes {
		data := g.chartData(ct)
		if len(data) == 0 {
			t.Errorf("chart type %q produced empty data", ct)
		}
	}
}

func TestDiagramDataStructure(t *testing.T) {
	g := New(42)
	for _, dt := range diagramTypes {
		data := g.diagramData(dt)
		if len(data) == 0 {
			t.Errorf("diagram type %q produced empty data", dt)
		}
	}
}

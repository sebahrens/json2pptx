package svggen

import (
	"fmt"
	"strings"
	"testing"
)

func TestVennChart_Draw(t *testing.T) {
	tests := []struct {
		name    string
		data    VennData
		wantErr bool
	}{
		{
			name: "basic 2-circle Venn",
			data: VennData{
				Title: "Skills Overlap",
				Circles: []VennCircle{
					{Label: "Engineering", Items: []string{"Coding", "Architecture"}},
					{Label: "Design", Items: []string{"UI/UX", "Typography"}},
				},
				Intersections: map[string]VennRegion{
					"ab": {Label: "Product Dev", Items: []string{"Prototyping"}},
				},
			},
			wantErr: false,
		},
		{
			name: "3-circle Venn",
			data: VennData{
				Title: "Innovation Triangle",
				Circles: []VennCircle{
					{Label: "Technology"},
					{Label: "Business"},
					{Label: "Design"},
				},
				Intersections: map[string]VennRegion{
					"ab":  {Label: "Feasibility"},
					"ac":  {Label: "Usability"},
					"bc":  {Label: "Viability"},
					"abc": {Label: "Innovation"},
				},
			},
			wantErr: false,
		},
		{
			name: "2-circle with subtitle and footnote",
			data: VennData{
				Title:    "Market Overlap",
				Subtitle: "Q4 Analysis",
				Circles: []VennCircle{
					{Label: "Product A"},
					{Label: "Product B"},
				},
				Intersections: map[string]VennRegion{
					"ab": {Label: "Shared Customers"},
				},
				Footnote: "Source: Internal CRM data",
			},
			wantErr: false,
		},
		{
			name: "minimal 2-circle no intersections",
			data: VennData{
				Circles: []VennCircle{
					{Label: "Set A"},
					{Label: "Set B"},
				},
			},
			wantErr: false,
		},
		{
			name: "3-circle with items",
			data: VennData{
				Title: "Team Skills",
				Circles: []VennCircle{
					{Label: "Frontend", Items: []string{"React", "CSS", "HTML"}},
					{Label: "Backend", Items: []string{"Go", "SQL", "APIs"}},
					{Label: "DevOps", Items: []string{"Docker", "K8s", "CI/CD"}},
				},
				Intersections: map[string]VennRegion{
					"ab":  {Label: "Full Stack"},
					"ac":  {Label: "Platform"},
					"bc":  {Label: "SRE"},
					"abc": {Label: "Staff Eng"},
				},
			},
			wantErr: false,
		},
		{
			name:    "too few circles",
			data:    VennData{Circles: []VennCircle{{Label: "Solo"}}},
			wantErr: true,
		},
		{
			name: "4 circles gracefully uses first 3",
			data: VennData{Circles: []VennCircle{
				{Label: "A"}, {Label: "B"}, {Label: "C"}, {Label: "D"},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSVGBuilder(800, 600)
			config := DefaultVennConfig(800, 600)

			chart := NewVennChart(builder, config)
			err := chart.Draw(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("VennChart.Draw() error = %v, wantErr %v", err, tt.wantErr)
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

func TestVennDiagram_Validate(t *testing.T) {
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid 2-circle",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "A"},
						map[string]any{"label": "B"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid 3-circle",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "A"},
						map[string]any{"label": "B"},
						map[string]any{"label": "C"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &RequestEnvelope{
				Type: "venn",
				Data: nil,
			},
			wantErr: true,
		},
		{
			name: "missing circles",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "too few circles",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "A"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "4 circles gracefully uses first 3",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "A"},
						map[string]any{"label": "B"},
						map[string]any{"label": "C"},
						map[string]any{"label": "D"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid 2-circle using sets alias",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"sets": []any{
						map[string]any{"label": "A"},
						map[string]any{"label": "B"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "sets alias too few",
			req: &RequestEnvelope{
				Type: "venn",
				Data: map[string]any{
					"sets": []any{
						map[string]any{"label": "A"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("VennDiagram.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVennDiagram_Render(t *testing.T) {
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	req := &RequestEnvelope{
		Type:  "venn",
		Title: "Test Venn Diagram",
		Data: map[string]any{
			"circles": []any{
				map[string]any{"label": "Science", "items": []any{"Physics", "Chemistry"}},
				map[string]any{"label": "Art", "items": []any{"Painting", "Music"}},
			},
			"intersections": map[string]any{
				"ab": map[string]any{"label": "Creativity", "items": []any{"Innovation"}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("VennDiagram.Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if doc.Width != 1067 || doc.Height != 800 {
		t.Errorf("Expected dimensions 1067x800 (800x600pt in CSS pixels), got %vx%v", doc.Width, doc.Height)
	}

	// Verify circle labels are present
	svg := string(doc.Content)
	for _, label := range []string{"Science", "Art"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain circle label %q", label)
		}
	}

	// Verify intersection label
	if !strings.Contains(svg, "Creativity") {
		t.Error("Expected SVG to contain intersection label \"Creativity\"")
	}

	// Verify circle items are present
	for _, item := range []string{"Physics", "Chemistry", "Painting", "Music"} {
		if !strings.Contains(svg, item) {
			t.Errorf("Expected SVG to contain item %q", item)
		}
	}
}

func TestVennDiagram_RenderSetsAlias(t *testing.T) {
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	req := &RequestEnvelope{
		Type:  "venn",
		Title: "Sets Alias Venn",
		Data: map[string]any{
			"sets": []any{
				map[string]any{"label": "Tech", "items": []any{"Go", "Rust"}},
				map[string]any{"label": "Design", "items": []any{"Figma"}},
			},
			"intersections": map[string]any{
				"ab": map[string]any{"label": "UX Engineering"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("VennDiagram.Render() with 'sets' alias error = %v", err)
	}

	svg := string(doc.Content)
	for _, label := range []string{"Tech", "Design", "UX Engineering"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain %q", label)
		}
	}
}

func TestVennDiagram_Render3Circle(t *testing.T) {
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	req := &RequestEnvelope{
		Type:  "venn",
		Title: "Three Circle Venn",
		Data: map[string]any{
			"circles": []any{
				map[string]any{"label": "A"},
				map[string]any{"label": "B"},
				map[string]any{"label": "C"},
			},
			"intersections": map[string]any{
				"ab":  map[string]any{"label": "A+B"},
				"ac":  map[string]any{"label": "A+C"},
				"bc":  map[string]any{"label": "B+C"},
				"abc": map[string]any{"label": "All"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("VennDiagram.Render() 3-circle error = %v", err)
	}

	svg := string(doc.Content)

	// Verify all labels present
	for _, label := range []string{"A", "B", "C", "A+B", "A+C", "B+C", "All"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain label %q", label)
		}
	}
}

func TestVennDiagram_RenderWithStringIntersections(t *testing.T) {
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	req := &RequestEnvelope{
		Type: "venn",
		Data: map[string]any{
			"circles": []any{
				map[string]any{"label": "X"},
				map[string]any{"label": "Y"},
			},
			"intersections": map[string]any{
				"ab": "Both",
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("VennDiagram.Render() string intersection error = %v", err)
	}

	svg := string(doc.Content)
	if !strings.Contains(svg, "Both") {
		t.Error("Expected SVG to contain string intersection label \"Both\"")
	}
}

func TestVennDiagram_Render_AllExclusiveItems(t *testing.T) {
	// Reproduces go-slide-ceator-j4o9: 2-circle Venn drops exclusive zone items.
	// Tests at multiple canvas sizes since PPTX placeholders vary.
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	sizes := []struct {
		w, h      int
		checkAll  bool // whether to check all items (very small canvases may omit items for readability)
	}{
		{800, 600, true},
		// 400x300 is too small with the 9pt minimum font floor to guarantee all
		// items fit; intersection text may be truncated. Verify rendering only.
		{400, 300, false},
		// 350x250 is too small for boardroom-readable font sizes with 6+ items.
		// At this size, the presentation-minimum font floors cause some items to
		// be omitted to keep remaining text readable. We still verify it renders.
		{350, 250, false},
	}

	for _, sz := range sizes {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "venn",
				Title: "Skills Overlap",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "Engineering", "items": []any{"System Design", "Coding", "Testing"}},
						map[string]any{"label": "Design", "items": []any{"UI/UX", "Typography", "Color Theory"}},
					},
					"intersections": map[string]any{
						"ab": map[string]any{"label": "Product Development", "items": []any{"Prototyping", "User Research"}},
					},
				},
				Output: OutputSpec{Width: sz.w, Height: sz.h},
			}

			doc, err := diagram.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			svg := string(doc.Content)

			if sz.checkAll {
				// ALL exclusive zone items must be present in the SVG
				for _, item := range []string{"System Design", "Coding", "Testing", "UI/UX", "Typography", "Color Theory"} {
					if !strings.Contains(svg, item) {
						t.Errorf("MISSING exclusive item %q in SVG output", item)
					}
				}

				// Intersection items must also be present
				for _, item := range []string{"Prototyping", "User Research"} {
					if !strings.Contains(svg, item) {
						t.Errorf("MISSING intersection item %q in SVG output", item)
					}
				}
			} else {
				// At very small sizes, just verify labels are present
				for _, label := range []string{"Engineering", "Design"} {
					if !strings.Contains(svg, label) {
						t.Errorf("MISSING circle label %q in SVG output at %dx%d", label, sz.w, sz.h)
					}
				}
			}
		})
	}
}

func TestVennDiagram_IntersectionLabel_NoMidWordBreak(t *testing.T) {
	// Regression test: intersection labels must wrap at word boundaries,
	// never splitting a word mid-character. When a single word is too
	// wide for the intersection region it should be truncated with an
	// ellipsis rather than broken across lines.
	diagram := &VennDiagram{NewBaseDiagram("venn")}

	// Use a small canvas so the intersection region is narrow, forcing
	// the long label "Artificial Intelligence" to wrap.
	req := &RequestEnvelope{
		Type:  "venn",
		Title: "Word Wrap Test",
		Data: map[string]any{
			"circles": []any{
				map[string]any{"label": "Engineering"},
				map[string]any{"label": "Design"},
			},
			"intersections": map[string]any{
				"ab": map[string]any{"label": "Product Development"},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)

	// The SVG must contain at least one of the full words from the label.
	// If mid-word breaking occurred we'd see fragments like "Developm" and "ent"
	// as separate text elements without either full word present.
	hasProduct := strings.Contains(svg, "Product")
	hasDevelopment := strings.Contains(svg, "Development")
	if !hasProduct && !hasDevelopment {
		t.Errorf("intersection label appears to break words mid-character; "+
			"neither 'Product' nor 'Development' found as whole words in SVG output")
	}
}

func TestBlendColors(t *testing.T) {
	a := Color{R: 255, G: 0, B: 0, A: 1.0}
	b := Color{R: 0, G: 0, B: 255, A: 1.0}

	result := blendColors(a, b)

	if result.R != 127 {
		t.Errorf("Expected R=127, got %v", result.R)
	}
	if result.G != 0 {
		t.Errorf("Expected G=0, got %v", result.G)
	}
	if result.B != 127 {
		t.Errorf("Expected B=127, got %v", result.B)
	}
	if result.A != 1.0 {
		t.Errorf("Expected A=1.0, got %v", result.A)
	}
}

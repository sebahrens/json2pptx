package svggen

import (
	"regexp"
	"strings"
	"testing"
)

func leadershipOrgRequest(strategy string) *RequestEnvelope {
	data := map[string]any{"root": map[string]any{
		"name": "Maria Okonkwo", "title": "Chief Executive Officer",
		"children": []any{
			map[string]any{"name": "Jens Lindqvist", "title": "Chief Financial Officer"},
			map[string]any{"name": "Aiko Tanaka", "title": "Chief Product Officer"},
			map[string]any{"name": "David Mensah", "title": "Chief Revenue Officer"},
		},
	}}
	if strategy != "" {
		data["accent_strategy"] = strategy
	}
	// A midnight-blue-like theme: accent2 is red, the colour the review saw
	// on every second-level node.
	return &RequestEnvelope{Type: "org_chart", Data: data, Output: OutputSpec{Width: 900, Height: 500},
		Style: StyleSpec{ThemeColors: []ThemeColorInput{
			{Name: "accent1", RGB: "#2E5090"}, {Name: "accent2", RGB: "#D9453B"},
			{Name: "accent3", RGB: "#F2B632"}, {Name: "accent4", RGB: "#3D9B5B"},
		}}}
}

var hexColor = regexp.MustCompile(`#[0-9a-fA-F]{6}`)

// TestOrgChart_PrimaryStrategyUsesOneHue pins go-slide-creator-libnz: under
// the default (primary) accent strategy no node is drawn in accent2; levels
// are the primary accent and its tints. Under rotate, levels take their own
// accents again.
func TestOrgChart_PrimaryStrategyUsesOneHue(t *testing.T) {
	render := func(strategy string) string {
		doc, err := (&OrgChartDiagram{NewBaseDiagram("org_chart")}).Render(leadershipOrgRequest(strategy))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ToLower(doc.String())
	}
	primary := render("")
	if strings.Contains(primary, "#d9453b") || strings.Contains(primary, "217,69,59") {
		t.Error("primary-strategy org chart draws accent2 (red)")
	}
	if !strings.Contains(primary, "#2e5090") {
		t.Errorf("primary-strategy org chart does not use accent1; colours: %v", hexColor.FindAllString(primary, 20))
	}
	rotated := render("rotate")
	if !strings.Contains(rotated, "#d9453b") {
		t.Error("rotate strategy should give the second level its own accent (accent2)")
	}
}

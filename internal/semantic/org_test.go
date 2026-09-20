package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/svggen"
)

func orgSpec(body map[string]any) *DeckSpec {
	return &DeckSpec{Meta: DeckMeta{Title: "Governance"}, Slides: []SlideSpec{{Kind: KindOrg, Body: body}}}
}

func orgNodes() []any {
	return []any{
		map[string]any{"id": "steer", "name": "Steering group", "title": "Decision owner"},
		map[string]any{"id": "platform", "name": "Platform lead", "title": "Architecture", "parent": "steer"},
		map[string]any{"id": "data", "name": "Data lead", "title": "Migration", "parent": "steer"},
		map[string]any{"id": "risk", "name": "Risk lead", "title": "Controls", "parent": "steer"},
	}
}

func TestOrgCompilesRegisteredDiagramWithoutPruning(t *testing.T) {
	body := map[string]any{"title": "Programme governance", "takeaway": "One group owns decisions.", "nodes": orgNodes()}
	plan := Normalize(orgSpec(body))
	if plan.Slides[0].Visual.Layout != "diagram" {
		t.Fatalf("plan = %+v", plan.Slides[0].Visual)
	}
	input, result, err := Compile(orgSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
	}
	slide := input.Slides[0]
	if slide.SlideType != "diagram" {
		t.Fatalf("slide type = %q", slide.SlideType)
	}
	var data map[string]any
	for _, c := range slide.Content {
		if c.DiagramValue != nil {
			if c.DiagramValue.Type != "org_chart" {
				t.Fatalf("diagram = %q", c.DiagramValue.Type)
			}
			data = c.DiagramValue.Data
		}
	}
	if data == nil {
		t.Fatal("no org diagram")
	}
	root := data["root"].(map[string]any)
	if root["name"] != "Steering group" || len(root["children"].([]any)) != 3 {
		t.Fatalf("root data = %+v", root)
	}
	findings, err := svggen.DryRender(&svggen.RequestEnvelope{Type: "org_chart", Data: data, Output: svggen.OutputSpec{Width: 1100, Height: 700}})
	if err != nil {
		t.Fatalf("org_chart not renderable: %v", err)
	}
	for _, f := range findings {
		if f.Code == svggen.FindingOrgChartDepthPruned {
			t.Fatalf("small org was pruned: %+v", f)
		}
	}
}

func TestOrgOverBudgetFallsBackWithoutLosingNames(t *testing.T) {
	nodes := orgNodes()
	for _, id := range []string{"legal", "finance", "ops", "security"} {
		nodes = append(nodes, map[string]any{"id": id, "name": strings.ToUpper(id), "parent": "steer"})
	}
	body := map[string]any{"nodes": nodes, "takeaway": "Eight bodies share the governance."}
	plan := Normalize(orgSpec(body))
	if plan.Slides[0].Visual.Layout != "content" {
		t.Fatalf("fallback layout = %q", plan.Slides[0].Visual.Layout)
	}
	input, result, err := Compile(orgSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Slides[0].SlideType != "content" {
		t.Fatalf("fallback slide type = %q", input.Slides[0].SlideType)
	}
	var bullets []string
	for _, c := range input.Slides[0].Content {
		if c.BulletsValue != nil {
			bullets = *c.BulletsValue
		}
	}
	if len(bullets) != len(nodes) {
		t.Fatalf("fallback retained %d of %d nodes", len(bullets), len(nodes))
	}
	if !strings.HasPrefix(bullets[1], "↳ ") {
		t.Fatalf("fallback lost hierarchy: %q", bullets[1])
	}
	if !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
		t.Fatalf("missing degradation: %+v", result.Diagnostics)
	}
}

func TestOrgMalformedTreeHasSemanticPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]any)
		path string
	}{
		{"missing parent", func(v []any) { v[1].(map[string]any)["parent"] = "unknown" }, "slides[0].nodes[1].parent"},
		{"duplicate id", func(v []any) { v[1].(map[string]any)["id"] = "steer" }, "slides[0].nodes[1].id"},
		{"wrong name type", func(v []any) { v[1].(map[string]any)["name"] = 42 }, "slides[0].nodes[1].name"},
		{"disconnected cycle", func(v []any) { v[1].(map[string]any)["parent"] = "data"; v[2].(map[string]any)["parent"] = "platform" }, "slides[0].nodes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := orgNodes()
			tc.edit(nodes)
			input, result, err := Compile(orgSpec(map[string]any{"nodes": nodes}), CompileOptions{})
			if err == nil || input != nil {
				t.Fatalf("malformed tree compiled: %+v", input)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Path == tc.path && d.Severity == diagnostics.SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing path %s: %+v", tc.path, result.Diagnostics)
			}
		})
	}
}

func TestOrgDiscoverySchema(t *testing.T) {
	ex := KindExample(KindOrg)
	if ex == nil || ex["kind"] != "org" {
		t.Fatalf("example = %+v", ex)
	}
	variant := KindItemSchema(KindOrg)
	if variant["additionalProperties"] != false {
		t.Fatal("open org schema")
	}
	props := variant["properties"].(map[string]any)
	item := props["nodes"].(map[string]any)["items"].(map[string]any)
	if item["additionalProperties"] != false {
		t.Fatal("open node schema")
	}
}

func TestOrgExplicitContentLayout(t *testing.T) {
	body := map[string]any{"nodes": orgNodes(), "layout": "content", "takeaway": "One group owns decisions."}
	plan := Normalize(orgSpec(body))
	if plan.Slides[0].Visual.Layout != "content" {
		t.Fatalf("plan layout = %q", plan.Slides[0].Visual.Layout)
	}
	input, result, err := Compile(orgSpec(body), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
	}
	if input.Slides[0].SlideType != "content" {
		t.Fatalf("compiled type = %q", input.Slides[0].SlideType)
	}
}

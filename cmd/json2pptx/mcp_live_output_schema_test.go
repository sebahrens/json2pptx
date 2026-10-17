package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// assertLiveOutputSchema validates the registered contract against the actual
// wire-shaped structured result, including the shared error-envelope branch.
// Callers separately assert success so a valid error cannot satisfy a success fixture.
func assertLiveOutputSchema(t *testing.T, s *server.MCPServer, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	registered, ok := s.ListTools()[tool]
	if !ok {
		t.Fatalf("tool %s is not registered", tool)
	}
	schema := compileToolInputSchema(t, tool+" output", registered.Tool.RawOutputSchema)
	result := callToolViaServer(t, s, tool, args)
	if result.StructuredContent == nil {
		t.Fatalf("%s returned no structured content: %s", tool, resultText(result))
	}
	if err := schema.Validate(roundTripJSON(t, result.StructuredContent)); err != nil {
		t.Fatalf("%s actual response violates advertised output schema: %v\n%s", tool, err, resultText(result))
	}
	return result
}

type liveOutputFixture struct {
	name, tool string
	args       map[string]any
}

func discoveryLiveOutputFixtures() []liveOutputFixture {
	return []liveOutputFixture{
		{"started", "get_started", map[string]any{}},
		{"unknown_task", "get_started", map[string]any{"task": "fix my deck"}},
		{"capabilities", "get_capabilities", map[string]any{"sections": []string{"runtime", "features", "deprecations", "tools", "vocabularies"}}},
		{"templates", "list_templates", map[string]any{"fields": "compact"}},
		{"input_schema", "get_input_schema", map[string]any{}},
		{"data_hints", "get_data_format_hints", map[string]any{}},
		{"charts", "get_chart_capabilities", map[string]any{}},
		{"diagrams", "get_diagram_capabilities", map[string]any{}},
		{"shapes", "get_shape_catalog", map[string]any{}},
		{"kinds", "list_slide_kinds", map[string]any{}},
		{"kind_budgets", "list_slide_kinds", map[string]any{"kinds": []string{"kpi_snapshot"}, "fields": []string{"budgets"}}},
		{"kind_brief", "list_slide_kinds", map[string]any{"kinds": []string{"table"}, "fields": []string{"brief"}}},
		{"kind_template", "list_slide_kinds", map[string]any{"kinds": []string{"chart_insight"}, "template": "midnight-blue"}},
		{"archetypes", "list_deck_archetypes", map[string]any{}},
		{"explain", "explain_deck_spec", map[string]any{"spec": validSemanticSpec}},
		{"finding", "describe_finding", map[string]any{"code": "VERTICAL_IMBALANCE"}},
		{"patterns", "list_patterns", map[string]any{"fields": "full"}},
		{"pattern", "show_pattern", map[string]any{"name": "cycle-figure-eight"}},
		{"recommend_pattern", "recommend_pattern", map[string]any{"intent": "compare two options"}},
		{"recommend_visual", "recommend_visual", map[string]any{"intent": "two loops beside explanatory text in half the slide"}},
		{"icons", "list_icons", map[string]any{"filter": "arrow"}},
		{"table_density", "table_density_guide", map[string]any{}},
		{"theme", "resolve_theme", map[string]any{"template_name": "midnight-blue"}},
		{"settings", "list_template_settings", map[string]any{"template_name": "midnight-blue"}},
		{"examine", "examine_template", map[string]any{"template_name": "midnight-blue"}},
		{"plan", "plan_deck", map[string]any{"brief": "An executive update for Northwind Logistics. Revenue GBP 412.6m up 6.8%. Ask: approve the routing pilot.", "format": "deckspec", "template": "midnight-blue"}},
	}
}

func TestMCPDiscoveryLiveOutputSchemas(t *testing.T) {
	s := newMCPServer(testMCPConfig(t))
	for _, fixture := range discoveryLiveOutputFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			result := assertLiveOutputSchema(t, s, fixture.tool, fixture.args)
			if result.IsError {
				t.Fatalf("success fixture returned an error: %s", resultText(result))
			}
		})
	}
	t.Run("error_envelope", func(t *testing.T) {
		result := assertLiveOutputSchema(t, s, "list_slide_kinds", map[string]any{"kinds": []string{"not-a-kind"}})
		if !result.IsError {
			t.Fatal("invalid kind must return an error envelope")
		}
	})
}

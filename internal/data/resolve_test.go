package data

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestResolveField_NoVars(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"name": "Alice"}}
	result, warnings := resolveField("no variables here", ctx, nil)
	if result != "no variables here" {
		t.Errorf("expected unchanged string, got %q", result)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveField_SimpleVar(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"company": "Acme Corp"}}
	result, warnings := resolveField("Welcome to {{ company }}!", ctx, nil)
	if result != "Welcome to Acme Corp!" {
		t.Errorf("expected 'Welcome to Acme Corp!', got %q", result)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveField_MultipleVars(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"name": "Bob", "year": 2026}}
	result, warnings := resolveField("{{name}} - {{year}}", ctx, nil)
	if result != "Bob - 2026" {
		t.Errorf("expected 'Bob - 2026', got %q", result)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveField_NestedPath(t *testing.T) {
	ctx := &Context{Vars: map[string]any{
		"revenue": map[string]any{
			"q4": 1500000,
		},
	}}
	result, warnings := resolveField("Q4 Revenue: {{ revenue.q4 }}", ctx, nil)
	if result != "Q4 Revenue: 1500000" {
		t.Errorf("expected 'Q4 Revenue: 1500000', got %q", result)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveField_ArrayIndex(t *testing.T) {
	ctx := &Context{Vars: map[string]any{
		"items": []any{"alpha", "beta", "gamma"},
	}}
	result, warnings := resolveField("Second: {{ items.1 }}", ctx, nil)
	if result != "Second: beta" {
		t.Errorf("expected 'Second: beta', got %q", result)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveField_UndefinedVar(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"name": "Alice"}}
	result, warnings := resolveField("Hello {{ unknown }}", ctx, nil)
	if result != "Hello {{ unknown }}" {
		t.Errorf("expected unchanged for undefined var, got %q", result)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] != `data: undefined variable "unknown"` {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestResolveField_FloatValue(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"rate": 3.14}}
	result, _ := resolveField("Rate: {{ rate }}", ctx, nil)
	if result != "Rate: 3.14" {
		t.Errorf("expected 'Rate: 3.14', got %q", result)
	}
}

func TestResolveField_BoolValue(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"active": true}}
	result, _ := resolveField("Active: {{ active }}", ctx, nil)
	if result != "Active: true" {
		t.Errorf("expected 'Active: true', got %q", result)
	}
}

func TestResolveField_WholeNumber(t *testing.T) {
	ctx := &Context{Vars: map[string]any{"count": float64(42)}}
	result, _ := resolveField("Count: {{ count }}", ctx, nil)
	if result != "Count: 42" {
		t.Errorf("expected 'Count: 42', got %q", result)
	}
}

func TestResolveVariables_FullPresentation(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{
			Title:  "{{ company }} Quarterly Report",
			Author: "{{ author }}",
		},
		Slides: []types.SlideDefinition{
			{
				Title: "{{ company }} Overview",
				Content: types.SlideContent{
					Bullets: []string{
						"Revenue: {{ revenue }}",
						"Growth: {{ growth }}%",
					},
				},
			},
		},
	}

	ctx := &Context{Vars: map[string]any{
		"company": "Acme Corp",
		"author":  "Jane Doe",
		"revenue": "$1.5M",
		"growth":  15,
	}}

	warnings := ResolveVariables(pres, ctx)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	if pres.Metadata.Title != "Acme Corp Quarterly Report" {
		t.Errorf("title not resolved: %q", pres.Metadata.Title)
	}
	if pres.Metadata.Author != "Jane Doe" {
		t.Errorf("author not resolved: %q", pres.Metadata.Author)
	}
	if pres.Slides[0].Title != "Acme Corp Overview" {
		t.Errorf("slide title not resolved: %q", pres.Slides[0].Title)
	}
	if pres.Slides[0].Content.Bullets[0] != "Revenue: $1.5M" {
		t.Errorf("bullet 0 not resolved: %q", pres.Slides[0].Content.Bullets[0])
	}
	if pres.Slides[0].Content.Bullets[1] != "Growth: 15%" {
		t.Errorf("bullet 1 not resolved: %q", pres.Slides[0].Content.Bullets[1])
	}
}

func TestResolveVariables_NilContext(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "{{ company }}"},
	}

	warnings := ResolveVariables(pres, nil)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil context, got %v", warnings)
	}
	if pres.Metadata.Title != "{{ company }}" {
		t.Errorf("expected unchanged title for nil context, got %q", pres.Metadata.Title)
	}
}

func TestResolveVariables_EmptyContext(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "{{ company }}"},
	}

	ctx := &Context{Vars: map[string]any{}}
	warnings := ResolveVariables(pres, ctx)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty context, got %v", warnings)
	}
}

func TestLookupPath_DeepNested(t *testing.T) {
	vars := map[string]any{
		"data": map[string]any{
			"metrics": map[string]any{
				"revenue": 42000,
			},
		},
	}

	val, ok := lookupPath(vars, "data.metrics.revenue")
	if !ok {
		t.Fatal("expected to find data.metrics.revenue")
	}
	if val != 42000 {
		t.Errorf("expected 42000, got %v", val)
	}
}

func TestLookupPath_DirectKey(t *testing.T) {
	vars := map[string]any{"simple": "value"}
	val, ok := lookupPath(vars, "simple")
	if !ok || val != "value" {
		t.Errorf("expected 'value', got %v (ok=%v)", val, ok)
	}
}

func TestLookupPath_NotFound(t *testing.T) {
	vars := map[string]any{"a": "b"}
	_, ok := lookupPath(vars, "x.y.z")
	if ok {
		t.Error("expected not found")
	}
}

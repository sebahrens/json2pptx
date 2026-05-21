package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/visualqa"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// trivialDeckJSON returns a one-slide deck that passes the deterministic gate
// on pass 1 with no repairs — keeps these tests focused on the visual_qa
// surface rather than convergence behavior.
func trivialDeckJSON() string {
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout1",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
				},
			},
		},
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

// TestVisualQA_DisabledByDefault asserts the default deterministic mode is
// truth-labeled and carries no visual_qa block — the opt-in must be invisible
// to callers that don't request it.
func TestVisualQA_DisabledByDefault(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation":    mustParseJSON(trivialDeckJSON()),
		"output_filename": "vqa_default.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if output.QualityMode != qualityModeDeterministic {
		t.Errorf("quality_mode = %q, want %q", output.QualityMode, qualityModeDeterministic)
	}
	if output.VisualQA != nil {
		t.Errorf("expected no visual_qa block when mode is disabled, got %+v", output.VisualQA)
	}
}

// TestVisualQA_EnabledReportsRequirementsAndMode asserts that enabling the mode
// truth-labels quality_mode, always reports requirements (model/API-key/cost),
// and degrades transparently: skipped+note when render tools are unavailable,
// or a real vision/heuristic mode otherwise. The test is tolerant of the
// environment so it is deterministic on CI (no render tools) and on dev
// machines (tools present, no API key → heuristic).
func TestVisualQA_EnabledReportsRequirementsAndMode(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation":    mustParseJSON(trivialDeckJSON()),
		"output_filename": "vqa_enabled.pptx",
		"visual_qa": map[string]any{
			"enabled": true,
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.QualityMode != qualityModeVisualQA {
		t.Errorf("quality_mode = %q, want %q", output.QualityMode, qualityModeVisualQA)
	}
	if output.VisualQA == nil {
		t.Fatal("expected visual_qa block when mode is enabled")
	}
	vqa := output.VisualQA
	if !vqa.Requested {
		t.Error("visual_qa.requested should be true")
	}
	if vqa.Requirements.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("requirements.api_key_env = %q, want ANTHROPIC_API_KEY", vqa.Requirements.APIKeyEnv)
	}
	if vqa.Requirements.DefaultModel == "" {
		t.Error("requirements.default_model should report the resolved model")
	}
	if vqa.Requirements.CostNote == "" {
		t.Error("requirements.cost_note should be populated so agents can weigh cost before enabling")
	}
	if len(vqa.Requirements.RenderDependencies) == 0 {
		t.Error("requirements.render_dependencies should list libreoffice + magick")
	}

	avail, _ := render.DependencyStatus()
	switch {
	case !avail:
		if vqa.InspectionMode != "skipped" {
			t.Errorf("inspection_mode = %q, want skipped when render tools absent", vqa.InspectionMode)
		}
		if len(vqa.Notes) == 0 {
			t.Error("expected a note explaining the skipped phase when render tools are unavailable")
		}
	default:
		if vqa.InspectionMode != "vision" && vqa.InspectionMode != "heuristic" {
			t.Errorf("inspection_mode = %q, want vision or heuristic when render tools present", vqa.InspectionMode)
		}
		if len(vqa.Passes) == 0 {
			t.Error("expected at least one pass recorded when render tools are present")
		}
		for _, p := range vqa.Passes {
			if p.VisualFindings == nil {
				t.Errorf("pass %d: visual_findings must be a non-nil array", p.Pass)
			}
			if p.ProposedRepairs == nil {
				t.Errorf("pass %d: proposed_repairs must be a non-nil array", p.Pass)
			}
			if p.RepairsApplied == nil {
				t.Errorf("pass %d: repairs_applied must be a non-nil array", p.Pass)
			}
		}
	}
}

// TestVisualQA_AuditPaletteRequested asserts that requesting the palette audit
// attaches a palette_audit block (available or transparently unavailable).
func TestVisualQA_AuditPaletteRequested(t *testing.T) {
	avail, _ := render.DependencyStatus()
	if !avail {
		t.Skip("render tools unavailable; visual_qa phase is skipped so palette audit never runs")
	}
	mc := repairMC(t)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation":    mustParseJSON(trivialDeckJSON()),
		"output_filename": "vqa_palette.pptx",
		"visual_qa": map[string]any{
			"enabled":       true,
			"audit_palette": true,
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if output.VisualQA == nil || output.VisualQA.PaletteAudit == nil {
		t.Fatal("expected palette_audit block when audit_palette=true")
	}
}

// TestMakeDeck_VisualQADefault asserts make_deck inherits the deterministic
// default truth-label and emits no visual_qa block unless requested.
func TestMakeDeck_VisualQADefault(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":         "Quarterly business review",
		"template":        "midnight-blue",
		"output_filename": "make_deck_vqa_default.pptx",
		"style_hints":     map[string]any{"slide_budget": float64(3)},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output makeDeckOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if output.QualityMode != qualityModeDeterministic {
		t.Errorf("quality_mode = %q, want %q", output.QualityMode, qualityModeDeterministic)
	}
	if output.VisualQA != nil {
		t.Errorf("expected no visual_qa block by default, got %+v", output.VisualQA)
	}
}

// TestExtractVisualQAConfig covers parsing, defaults, and clamping.
func TestExtractVisualQAConfig(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want visualQAConfig
	}{
		{
			name: "absent block disables mode",
			args: map[string]any{},
			want: visualQAConfig{Enabled: false},
		},
		{
			name: "enabled with defaults",
			args: map[string]any{"visual_qa": map[string]any{"enabled": true}},
			want: visualQAConfig{Enabled: true, MaxPasses: defaultVisualQAMaxPasses, Density: defaultVisualQADensity},
		},
		{
			name: "clamps max_passes high",
			args: map[string]any{"visual_qa": map[string]any{"enabled": true, "max_passes": float64(99)}},
			want: visualQAConfig{Enabled: true, MaxPasses: maxVisualQAMaxPasses, Density: defaultVisualQADensity},
		},
		{
			name: "clamps density low",
			args: map[string]any{"visual_qa": map[string]any{"enabled": true, "density": float64(1)}},
			want: visualQAConfig{Enabled: true, MaxPasses: defaultVisualQAMaxPasses, Density: minVisualQADensity},
		},
		{
			name: "model and audit_palette pass through",
			args: map[string]any{"visual_qa": map[string]any{"enabled": true, "model": "claude-x", "audit_palette": true}},
			want: visualQAConfig{Enabled: true, Model: "claude-x", AuditPalette: true, MaxPasses: defaultVisualQAMaxPasses, Density: defaultVisualQADensity},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractVisualQAConfig(makeRequest(tc.args))
			if got != tc.want {
				t.Errorf("extractVisualQAConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestActionableVisualFindings_FiltersBySeverity asserts only P0/P1 findings
// drive repairs — P2/P3 (and heuristic-only P3) are advisory.
func TestActionableVisualFindings_FiltersBySeverity(t *testing.T) {
	findings := []visualqa.Finding{
		{SlideIndex: 0, Severity: visualqa.SeverityP0, Category: "overlap"},
		{SlideIndex: 1, Severity: visualqa.SeverityP1, Category: "font_size"},
		{SlideIndex: 2, Severity: visualqa.SeverityP2, Category: "spacing"},
		{SlideIndex: 3, Severity: visualqa.SeverityP3, Category: "alignment"},
	}
	got := actionableVisualFindings(findings)
	if len(got) != 2 {
		t.Fatalf("expected 2 actionable (P0+P1) findings, got %d: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Severity != visualqa.SeverityP0 && f.Severity != visualqa.SeverityP1 {
			t.Errorf("actionable finding has severity %q, want P0/P1", f.Severity)
		}
	}
}

// TestVisualFindingMapping_AppliesRepair exercises the full deterministic
// mapping pipeline the visual_qa loop uses — visualqa.Finding →
// proposeRepairs → applyProposedRepairs — without rendering or an API key. A
// finding carrying an actionable reduce_text fix must trim the body.
func TestVisualFindingMapping_AppliesRepair(t *testing.T) {
	deck := PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "slideLayout2",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Heavy slide")},
					{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{
						"one", "two", "three", "four", "five", "six",
					}},
				},
			},
		},
	}

	findings := []visualqa.Finding{
		{
			SlideIndex:  0,
			SlideType:   "content",
			Severity:    visualqa.SeverityP1,
			Category:    "font_size",
			Description: "body text too dense",
			SuggestedFixes: []visualqa.SuggestedFix{
				{Kind: "reduce_text", Params: map[string]any{"max_items": float64(2)}},
			},
		},
	}

	actionable := actionableVisualFindings(findings)
	if len(actionable) != 1 {
		t.Fatalf("expected 1 actionable finding, got %d", len(actionable))
	}

	proposed := proposeRepairs(&deck, visualFindingsToProposeFindings(actionable))
	flat := flattenProposedRepairs(proposed)
	if len(flat) == 0 {
		t.Fatalf("expected proposed repairs from an actionable visual finding, got none")
	}
	foundReduce := false
	for _, p := range flat {
		if p.Kind == "reduce_text" && p.SlideIndex == 0 {
			foundReduce = true
		}
	}
	if !foundReduce {
		t.Errorf("expected a reduce_text directive on slide 0, got %+v", flat)
	}

	applied := applyProposedRepairs(&deck, proposed)
	if len(applied) == 0 {
		t.Fatalf("expected at least one applied repair, got none")
	}

	// The body must have been trimmed to 2 bullets.
	body := deck.Slides[0].Content[1]
	if body.BulletsValue == nil || len(*body.BulletsValue) != 2 {
		got := -1
		if body.BulletsValue != nil {
			got = len(*body.BulletsValue)
		}
		t.Errorf("expected body trimmed to 2 bullets, got %d", got)
	}
}

// TestVisualQAOutputSchemasValidJSON asserts the auto_repair / make_deck output
// schemas (which now embed the shared visual_qa fragment) remain valid JSON and
// advertise quality_mode + visual_qa.
func TestVisualQAOutputSchemasValidJSON(t *testing.T) {
	for name, schema := range map[string]json.RawMessage{
		"auto_repair": outputSchemaAutoRepair,
		"make_deck":   outputSchemaMakeDeck,
	} {
		var parsed map[string]any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("%s schema is not valid JSON: %v", name, err)
		}
		props, ok := parsed["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s schema missing properties", name)
		}
		if _, ok := props["quality_mode"]; !ok {
			t.Errorf("%s schema must advertise quality_mode", name)
		}
		if _, ok := props["visual_qa"]; !ok {
			t.Errorf("%s schema must advertise visual_qa", name)
		}
		req, _ := parsed["required"].([]any)
		hasQM := false
		for _, r := range req {
			if r == "quality_mode" {
				hasQM = true
			}
		}
		if !hasQM {
			t.Errorf("%s schema must require quality_mode (it is always present)", name)
		}
	}
}

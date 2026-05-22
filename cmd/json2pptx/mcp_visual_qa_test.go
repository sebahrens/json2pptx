package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/visualqa"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// heavyVisualRepairDeck returns a one-slide deck whose body carries six bullets,
// plus a P1 visual finding proposing a reduce_text(max_items:2) repair. The pair
// drives the deterministic visual-repair mapping (visualqa.Finding →
// proposeRepairs → applyProposedRepairs) without rendering or an API key — the
// foundation for exercising the staged apply/re-render/rollback contract.
func heavyVisualRepairDeck() (PresentationInput, proposeRepairsOutput) {
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
	proposed := proposeRepairs(&deck, visualFindingsToProposeFindings(actionableVisualFindings(findings)))
	return deck, proposed
}

func bulletCount(t *testing.T, deck PresentationInput) int {
	t.Helper()
	body := deck.Slides[0].Content[1]
	if body.BulletsValue == nil {
		return -1
	}
	return len(*body.BulletsValue)
}

// TestApplyAndReRenderVisualRepairs_RollsBackOnReRenderFailure forces a
// re-render failure AFTER a visual repair has mutated the deck and asserts the
// staged update rolls the mutation back: the in-memory deck (and therefore the
// marshaled final_presentation) returns to its pre-repair state so it stays
// consistent with the still-on-disk pre-repair PPTX. This is the core guard the
// bug demanded — repaired JSON must never point at an un-repaired PPTX.
func TestApplyAndReRenderVisualRepairs_RollsBackOnReRenderFailure(t *testing.T) {
	deck, proposed := heavyVisualRepairDeck()
	if got := bulletCount(t, deck); got != 6 {
		t.Fatalf("precondition: expected 6 bullets before repair, got %d", got)
	}

	rerenderErr := errors.New("simulated re-render failure")
	outcome := applyAndReRenderVisualRepairs(&deck, proposed, func() error {
		// The repair must already be applied to the deck by the time the
		// re-render runs — that is exactly the window where the bug lived.
		if got := bulletCount(t, deck); got != 2 {
			t.Errorf("re-render saw %d bullets, expected the repair (2) applied before re-render", got)
		}
		return rerenderErr
	})

	if !outcome.RolledBack {
		t.Errorf("outcome.RolledBack = false, want true after a re-render failure")
	}
	if !outcome.Consistent {
		t.Errorf("outcome.Consistent = false, want true (rollback restores consistency)")
	}
	if len(outcome.Applied) != 0 {
		t.Errorf("outcome.Applied = %v, want empty after rollback", outcome.Applied)
	}
	if !errors.Is(outcome.Err, rerenderErr) {
		t.Errorf("outcome.Err = %v, want the re-render error", outcome.Err)
	}
	if got := bulletCount(t, deck); got != 6 {
		t.Errorf("expected deck rolled back to 6 bullets, got %d — final_presentation would diverge from the PPTX", got)
	}
}

// TestApplyAndReRenderVisualRepairs_KeepsRepairsOnSuccess asserts the happy path
// is unchanged: when the re-render succeeds, the repair is kept and reported, and
// JSON + PPTX have advanced together.
func TestApplyAndReRenderVisualRepairs_KeepsRepairsOnSuccess(t *testing.T) {
	deck, proposed := heavyVisualRepairDeck()

	outcome := applyAndReRenderVisualRepairs(&deck, proposed, func() error { return nil })

	if outcome.RolledBack {
		t.Errorf("outcome.RolledBack = true, want false on a successful re-render")
	}
	if !outcome.Consistent {
		t.Errorf("outcome.Consistent = false, want true on success")
	}
	if len(outcome.Applied) == 0 {
		t.Errorf("outcome.Applied is empty, want the applied repair reported on success")
	}
	if outcome.Err != nil {
		t.Errorf("outcome.Err = %v, want nil on success", outcome.Err)
	}
	if got := bulletCount(t, deck); got != 2 {
		t.Errorf("expected repaired deck trimmed to 2 bullets, got %d", got)
	}
}

// TestApplyAndReRenderVisualRepairs_NoRepairsIsConsistent asserts that a pass
// that applies nothing reports a consistent, non-rolled-back outcome and never
// invokes the re-render closure (there is nothing new to render).
func TestApplyAndReRenderVisualRepairs_NoRepairsIsConsistent(t *testing.T) {
	deck := PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "slideLayout1",
				Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Hello")}},
			},
		},
	}

	rerendered := false
	outcome := applyAndReRenderVisualRepairs(&deck, proposeRepairsOutput{}, func() error {
		rerendered = true
		return nil
	})

	if rerendered {
		t.Error("re-render closure should not run when no repair was applied")
	}
	if outcome.RolledBack {
		t.Error("outcome.RolledBack = true, want false when nothing was applied")
	}
	if !outcome.Consistent {
		t.Error("outcome.Consistent = false, want true when nothing changed")
	}
	if len(outcome.Applied) != 0 {
		t.Errorf("outcome.Applied = %v, want empty when nothing was applied", outcome.Applied)
	}
}

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
	if !vqa.ArtifactConsistent {
		t.Error("visual_qa.artifact_consistent should be true on a normal run (no un-rollback-able re-render failure)")
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
			// Each pass must classify its inspection so a failed inspection is
			// distinguishable from a clean one.
			switch p.InspectionStatus {
			case inspectionStatusComplete, inspectionStatusPartial, inspectionStatusFailed:
			default:
				t.Errorf("pass %d: inspection_status = %q, want complete/partial/failed", p.Pass, p.InspectionStatus)
			}
			if p.FailedSlideCount < 0 {
				t.Errorf("pass %d: failed_slide_count = %d, want >= 0", p.Pass, p.FailedSlideCount)
			}
		}
		// A run with no per-slide inspection failures reports inspection_complete
		// and a zero failed count; a run with failures records both and explains
		// the inconclusive result in notes[].
		failures := 0
		for _, p := range vqa.Passes {
			failures += p.FailedSlideCount
		}
		if failures == 0 {
			if !vqa.InspectionComplete {
				t.Error("inspection_complete should be true when no slide inspection failed")
			}
			if vqa.FailedSlideCount != 0 {
				t.Errorf("failed_slide_count = %d, want 0 on a fully successful inspection", vqa.FailedSlideCount)
			}
		} else if vqa.InspectionComplete {
			t.Error("inspection_complete should be false when a pass had inspection failures")
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

// TestVisualQASchemaAdvertisesArtifactConsistent asserts the shared visual_qa
// fragment documents and requires the artifact_consistent guard, so MCP clients
// can rely on it being present on every visual_qa block.
func TestVisualQASchemaAdvertisesArtifactConsistent(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(visualQAResultSchema), &parsed); err != nil {
		t.Fatalf("visualQAResultSchema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("visual_qa schema missing properties")
	}
	if _, ok := props["artifact_consistent"]; !ok {
		t.Error("visual_qa schema must advertise artifact_consistent")
	}
	req, _ := parsed["required"].([]any)
	found := false
	for _, r := range req {
		if r == "artifact_consistent" {
			found = true
		}
	}
	if !found {
		t.Error("visual_qa schema must require artifact_consistent (it is always present)")
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

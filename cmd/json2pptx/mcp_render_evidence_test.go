package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// renderEvidenceFixture resolves the midnight-blue template and returns the
// pieces collectRenderFindings needs, so a test can drive the render pass
// directly and force individual failure stages.
func renderEvidenceFixture(t *testing.T) (layouts []types.LayoutMetadata, slideWidth, slideHeight int64, syntheticFiles map[string][]byte, metadata *types.TemplateMetadata, dataPalette []string) {
	t.Helper()
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err = template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts: %v", err)
	}
	slideWidth, slideHeight = template.ParseSlideDimensions(reader)
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        template.ParseTheme(reader),
	}
	template.SynthesizeIfNeeded(reader, analysis)
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	metadata, _ = template.ParseMetadata(reader)
	dataPalette = resolveDataPalette(metadata, analysis.Theme.Colors)
	return layouts, slideWidth, slideHeight, syntheticFiles, metadata, dataPalette
}

func renderEvidenceInput(t *testing.T) *PresentationInput {
	t.Helper()
	deckJSON := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
		map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []string{"one", "two"}},
	)
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(deckJSON), &input); err != nil {
		t.Fatalf("unmarshal deck: %v", err)
	}
	applyDefaults(&input)
	return &input
}

// TestCollectRenderFindings_GenerationFailureProducesEvidence forces the render
// pass to fail at the generation stage (bogus template path) and asserts the
// failure is reported as machine-readable evidence rather than swallowed as an
// empty finding set — the bug this change fixes.
func TestCollectRenderFindings_GenerationFailureProducesEvidence(t *testing.T) {
	mc := repairMC(t)
	layouts, w, h, synthetic, metadata, palette := renderEvidenceFixture(t)
	input := renderEvidenceInput(t)

	findings, evidence := mc.collectRenderFindings(
		context.Background(), input,
		filepath.Join(t.TempDir(), "does-not-exist.pptx"),
		layouts, w, h, synthetic, metadata, palette,
	)

	if evidence.Complete {
		t.Fatalf("expected evidence.Complete=false on generation failure, got complete=true")
	}
	if evidence.Stage != "generate" {
		t.Errorf("expected stage %q, got %q", "generate", evidence.Stage)
	}
	if evidence.Detail == "" {
		t.Errorf("expected non-empty evidence.Detail describing the failure")
	}
	if len(findings) != 0 {
		t.Errorf("expected no render findings on failure, got %d", len(findings))
	}
}

// TestCollectRenderFindings_SuccessIsComplete is the positive control: a valid
// template renders and the evidence is reported complete.
func TestCollectRenderFindings_SuccessIsComplete(t *testing.T) {
	mc := repairMC(t)
	layouts, w, h, synthetic, metadata, palette := renderEvidenceFixture(t)
	input := renderEvidenceInput(t)

	_, evidence := mc.collectRenderFindings(
		context.Background(), input,
		filepath.Join("..", "..", "templates", "midnight-blue.pptx"),
		layouts, w, h, synthetic, metadata, palette,
	)
	if !evidence.Complete {
		t.Fatalf("expected evidence.Complete=true on a valid render, got incomplete: stage=%q detail=%q", evidence.Stage, evidence.Detail)
	}
}

// TestRenderEvidenceFinding_ActionMapping locks the contract that an incomplete
// render is a blocking (refuse) finding by default and only drops to advisory
// (review) when degraded scoring was explicitly permitted.
func TestRenderEvidenceFinding_ActionMapping(t *testing.T) {
	blocking := renderEvidenceFinding(deterministic.RenderEvidence{Complete: false, Stage: "generate", Detail: "boom"})
	if blocking.Action != "refuse" {
		t.Errorf("default render-evidence finding action = %q, want refuse", blocking.Action)
	}
	if blocking.Code != renderEvidenceIncompleteCode {
		t.Errorf("finding code = %q, want %q", blocking.Code, renderEvidenceIncompleteCode)
	}

	degraded := renderEvidenceFinding(deterministic.RenderEvidence{Complete: false, Stage: "generate", Detail: "boom", Degraded: true})
	if degraded.Action != "review" {
		t.Errorf("degraded render-evidence finding action = %q, want review", degraded.Action)
	}
}

// TestEvaluateAutoRepairGate_RenderEvidenceFindingFailsGate asserts the
// synthetic refuse finding trips the P0 criterion so the auto_repair gate cannot
// pass when render evidence is incomplete.
func TestEvaluateAutoRepairGate_RenderEvidenceFindingFailsGate(t *testing.T) {
	gate := autoRepairGate{MinScore: 0, MaxP0Findings: 0, MaxP1Findings: 2}
	ds := &deterministic.DeckScore{OverallScore: 100}
	findings := []patterns.FitFinding{
		renderEvidenceFinding(deterministic.RenderEvidence{Complete: false, Stage: "generate", Detail: "boom"}),
	}
	reasons := evaluateAutoRepairGate(ds, findings, gate)
	if len(reasons) == 0 {
		t.Fatalf("expected the render-evidence refuse finding to produce a gate reason, got none")
	}

	// Under degraded scoring the finding is advisory and does not trip the P0 gate.
	degradedFindings := []patterns.FitFinding{
		renderEvidenceFinding(deterministic.RenderEvidence{Complete: false, Stage: "generate", Detail: "boom", Degraded: true}),
	}
	if reasons := evaluateAutoRepairGate(ds, degradedFindings, gate); len(reasons) != 0 {
		t.Errorf("expected no gate reasons under degraded scoring, got %v", reasons)
	}
}

// TestValidateAutoRepairFinalOutput_BlocksOnCorruptFile forces the final
// output-validation gate to fail by pointing it at a non-PPTX file. The result
// must be Ran=true, Valid=false with a blocking reason — never a silent pass.
func TestValidateAutoRepairFinalOutput_BlocksOnCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.pptx")
	if err := os.WriteFile(path, []byte("this is not a pptx archive"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	out := validateAutoRepairFinalOutput(path)
	if !out.Ran {
		t.Errorf("expected Ran=true even on a corrupt file")
	}
	if out.Valid {
		t.Errorf("expected Valid=false for a corrupt PPTX")
	}
	if len(out.Blocking) == 0 {
		t.Errorf("expected at least one blocking reason for a corrupt PPTX")
	}
}

// TestValidateAutoRepairFinalOutput_MissingFile treats an unreadable path as a
// blocking condition rather than an error the caller must handle.
func TestValidateAutoRepairFinalOutput_MissingFile(t *testing.T) {
	out := validateAutoRepairFinalOutput(filepath.Join(t.TempDir(), "missing.pptx"))
	if !out.Ran || out.Valid {
		t.Errorf("expected Ran=true, Valid=false for a missing file, got ran=%v valid=%v", out.Ran, out.Valid)
	}
}

// TestAutoRepair_HappyPathEvidenceComplete verifies that a normal converging run
// reports complete evidence: the render pass succeeded, final output validation
// passed, and no render_evidence warning block is attached.
func TestAutoRepair_HappyPathEvidenceComplete(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(autoRepairDeck(2)),
		"gate": map[string]any{
			"min_score":                  float64(50),
			"require_takeaway_on_charts": false,
		},
		"max_passes":      float64(2),
		"output_filename": "auto_repair_evidence_test.pptx",
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

	if output.OutputValidation == nil {
		t.Fatalf("expected output_validation to always be present")
	}
	if !output.OutputValidation.Ran || !output.OutputValidation.Valid {
		t.Errorf("expected final output validation to run and pass, got %+v", output.OutputValidation)
	}
	if !output.EvidenceComplete {
		t.Errorf("expected evidence_complete=true on a clean run")
	}
	if output.RenderEvidence != nil {
		t.Errorf("expected no render_evidence block on a complete render, got %+v", output.RenderEvidence)
	}
}

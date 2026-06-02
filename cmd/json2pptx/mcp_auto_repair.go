// mcp_auto_repair.go implements the auto_repair MCP tool — a server-side
// generate→inspect→repair convergence loop. Each pass collects fit findings
// (static + render-time), scores the deck deterministically, applies the
// proposed repairs, and re-runs. The loop stops when a configurable gate is
// satisfied or when max_passes is exhausted; the final deck is rendered to
// the output directory either way.
//
// The tool eliminates an entire orchestration burden from agents: instead of
// chaining generate_presentation → score_deck → propose_repairs →
// repair_slides_batch → generate_presentation in a hand-coded loop, the agent
// hands the engine a single call and gets back a trace plus a final PPTX.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// --- Defaults ---

const (
	defaultAutoRepairMinScore                = 75
	defaultAutoRepairMaxP0Findings           = 0
	defaultAutoRepairMaxP1Findings           = 2
	defaultAutoRepairRequireTakeawayOnCharts = true
	defaultAutoRepairMaxPasses               = 3
)

// contentProvenance labels where a facade's slide content came from. It is the
// deciding input for publishability: a deck filled with pattern exemplar values
// is a skeleton draft and is never publishable as-is, no matter how cleanly it
// scores. auto_repair runs on caller-authored content; make_deck cold-starts
// from exemplar placeholders.
type contentProvenance string

const (
	// contentProvenanceAuthorSupplied means the caller supplied the slide
	// content (auto_repair). Such a deck can be publishable once it also passes
	// the gate on complete evidence.
	contentProvenanceAuthorSupplied contentProvenance = "author_supplied"
	// contentProvenanceExemplarSkeleton means the content is pattern exemplar
	// placeholder values, not the caller's real content (make_deck cold start).
	// Always requires manual review and is never publishable as-is.
	contentProvenanceExemplarSkeleton contentProvenance = "exemplar_skeleton"
)

// --- Response types ---

// autoRepairOutput is the top-level response for auto_repair.
type autoRepairOutput struct {
	Path        string                 `json:"path,omitempty"`
	FinalScore  int                    `json:"final_score"`
	GatePassed  bool                   `json:"gate_passed"`
	Passes      int                    `json:"passes"`
	Trace       []autoRepairTraceEntry `json:"trace"`
	GateReasons []string               `json:"gate_reasons,omitempty"`
	// QualityMode truth-labels which inspection regime ACTUALLY ran:
	// "deterministic" (static + render-fit findings only — the default) or
	// "deterministic+visual_qa" (the deterministic loop followed by a visual
	// refinement phase that actually inspected slides). It is an alias of
	// Quality.Actual, preserved for backward compatibility: a requested visual-QA
	// phase that was skipped (e.g. render tools unavailable) reports
	// "deterministic" here, NOT "deterministic+visual_qa". final_score and trace
	// always describe the deterministic loop; visual-QA detail lives under
	// visual_qa.
	QualityMode string `json:"quality_mode"`
	// Quality separates the REQUESTED quality mode from the one that ACTUALLY ran,
	// so an agent never assumes vision/heuristic inspection happened when it was
	// skipped. Always present: requested is request-derived, actual mirrors
	// QualityMode, inspection_mode names the visual-QA backend, and
	// fallback_reasons explains any divergence (render tools unavailable, missing
	// API key heuristic fallback).
	Quality *qualityReport `json:"quality"`
	// VisualQA is present only when visual_qa mode was requested. It carries the
	// inspection mode, thumbnail paths, visual findings, proposed/applied
	// repairs, and optional palette audit. Any repairs it applied are also
	// reflected in final_presentation.
	VisualQA *visualQAResult `json:"visual_qa,omitempty"`
	// FinalPresentation is the full repaired deck JSON after the convergence
	// loop. Always present on a successful run, including zero-repair runs
	// (where it equals the resolved input). Lets agents continue editing,
	// diffing, patching, or re-validating by feeding it straight back into
	// validate_input / generate_presentation / repair_slide without
	// reconstructing state from the trace.
	FinalPresentation json.RawMessage `json:"final_presentation"`
	// IdempotentReplay is true when this response was served from the
	// idempotency cache instead of regenerated.
	IdempotentReplay bool `json:"idempotent_replay,omitempty"`
	// EvidenceComplete is the single authoritative flag that all required
	// validation evidence was collected AND clean: the render-time finding pass
	// for the scoring iteration completed, and final structural output
	// validation ran and found no blocking issues. gate_passed can only be true
	// on complete evidence unless the caller explicitly opted into degraded
	// scoring (in which case render_evidence.degraded labels the result).
	EvidenceComplete bool `json:"evidence_complete"`
	// RenderEvidence is present only when the render pass that backs the score
	// did not complete. complete=false means findings reflect static analysis
	// only; an explicit RENDER_EVIDENCE_INCOMPLETE finding accompanies it.
	RenderEvidence *deterministic.RenderEvidence `json:"render_evidence,omitempty"`
	// OutputValidation reports the final structural/output validation of the
	// rendered PPTX. Always present: a publishable / gate-passed result requires
	// ran=true and valid=true.
	OutputValidation *autoRepairOutputValidation `json:"output_validation,omitempty"`

	// --- Agent-native status fields (go-slide-creator-33oo) ---
	// These collapse the otherwise-scattered signals (transport success,
	// artifact existence, gate status, evidence completeness, content
	// provenance) into unambiguous machine-readable flags so an agent never
	// mistakes a draft, an exemplar skeleton, or a gate-failed result for a
	// publishable artifact.
	//
	// ArtifactStatus describes the rendered PPTX at Path: "generated" (written
	// and structurally valid) or "generated_invalid" (written but failed the
	// final structural output validation). The file always exists on a
	// non-error response — artifact existence alone never implies publishability.
	ArtifactStatus string `json:"artifact_status"`
	// ContentStatus and UsesExemplarContent label content provenance:
	// "author_supplied" (auto_repair — caller-authored slides) or
	// "exemplar_skeleton" (make_deck — pattern exemplar placeholder values).
	ContentStatus       string `json:"content_status"`
	UsesExemplarContent bool   `json:"uses_exemplar_content"`
	// ValidationStatus folds the deterministic gate and evidence completeness:
	// "passed" (gate met on complete evidence), "passed_degraded" (gate met but
	// evidence_complete=false — converged on degraded/static-only evidence), or
	// "failed" (gate not met).
	ValidationStatus string `json:"validation_status"`
	// Publishable is the single authoritative ship-as-is flag: gate passed AND
	// evidence complete AND structurally valid AND content author-supplied.
	// Exemplar skeletons, degraded passes, and gate-failed results are never
	// publishable. Equivalent to len(BlockingReasons)==0.
	Publishable bool `json:"publishable"`
	// ManualReviewRequired is the affirmative inverse of Publishable: true
	// whenever a human or agent must review before the deck ships (gate failed,
	// evidence incomplete, structurally invalid, or exemplar content). Provided
	// so agents gating on "show this to a human first?" don't have to negate
	// publishable.
	ManualReviewRequired bool `json:"manual_review_required"`
	// BlockingReasons enumerates every reason the deck is not publishable — the
	// unmet gate criteria (including final output validation), incomplete
	// evidence, and exemplar provenance. Empty iff Publishable is true. Superset
	// of GateReasons (which stays gate-only for backwards compatibility).
	BlockingReasons []string `json:"blocking_reasons,omitempty"`

	// NextState is the resumable per-pass state snapshot (go-slide-creator-yope):
	// completion status, a resume_token, the pass accounting, the remaining
	// findings, and a suggested next action. Always present. Pass
	// next_state.resume_token back as resume_token to continue the convergence
	// loop from this deck state without repeating completed passes.
	NextState *loopNextState `json:"next_state"`

	// checkpoint carries the server-side resumable snapshot from
	// runAutoRepairLoop to the handler, which finalizes the resume token and
	// persists it. Unexported so it never serializes into the response.
	checkpoint *loopCheckpoint `json:"-"`
}

// autoRepairOutputValidation summarizes the final pptx.ValidateOutputFile run on
// the rendered deck. The deck can only be reported gate-passed / publishable
// when Ran=true and Valid=true; Blocking lists the structural findings that
// forced the gate open otherwise.
type autoRepairOutputValidation struct {
	Ran      bool     `json:"ran"`
	Valid    bool     `json:"valid"`
	Blocking []string `json:"blocking,omitempty"`
}

// validateAutoRepairFinalOutput runs the unified output-validation suite on the
// rendered deck. It never returns an error: a read/parse failure is itself a
// blocking condition (Ran=true, Valid=false) so the caller refuses to mark the
// result publishable. This is the final evidence gate and runs regardless of
// degraded scoring — a structurally corrupt PPTX can never pass.
func validateAutoRepairFinalOutput(path string) autoRepairOutputValidation {
	report, err := pptx.ValidateOutputFile(path)
	if err != nil {
		return autoRepairOutputValidation{Ran: true, Valid: false, Blocking: []string{fmt.Sprintf("output validation could not run: %v", err)}}
	}
	blocking := report.Blocking()
	if len(blocking) == 0 {
		return autoRepairOutputValidation{Ran: true, Valid: true}
	}
	msgs := make([]string, len(blocking))
	for i, f := range blocking {
		msgs[i] = f.Error()
	}
	return autoRepairOutputValidation{Ran: true, Valid: false, Blocking: msgs}
}

// autoRepairTraceEntry is one iteration of the loop.
type autoRepairTraceEntry struct {
	Pass           int      `json:"pass"`
	Score          int      `json:"score"`
	FindingsCount  int      `json:"findings_count"`
	RepairsApplied []string `json:"repairs_applied"`
}

// autoRepairGate is the convergence gate config (all fields optional in the
// request; missing fields fall back to the defaults above).
type autoRepairGate struct {
	MinScore                int  `json:"min_score"`
	MaxP0Findings           int  `json:"max_p0_findings"`
	MaxP1Findings           int  `json:"max_p1_findings"`
	RequireTakeawayOnCharts bool `json:"require_takeaway_on_charts"`
}

// --- Tool definition ---

func mcpAutoRepairTool() mcp.Tool {
	return mcp.NewTool("auto_repair",
		mcp.WithDescription(`Run a server-side generate→inspect→repair convergence loop on a deck. Each pass renders the deck, collects fit findings, scores the deck deterministically, and applies the top-ranked repairs from propose_repairs. The loop stops when the configurable gate is satisfied or when max_passes is exhausted.

Quality mode (truth-labeled in the response as quality_mode):
- DEFAULT is "deterministic": the loop scores the deck from static + render-fit findings only. It never looks at a rendered pixel and needs no API key or render tools. This is fast and free but cannot catch purely visual defects (overlap, contrast, misalignment) the way a vision pass can.
- OPT-IN "deterministic+visual_qa": set visual_qa.enabled=true to additionally run the agent-grade visual refinement loop AFTER the deterministic loop — render thumbnails → inspect_slide_images → map visual findings → repair → re-render — and optionally the deterministic palette ΔE audit (audit_palette). This mode renders the deck (needs libreoffice + magick on PATH) and, when ANTHROPIC_API_KEY is set, issues one Claude vision call per slide (default model claude-haiku-4-5-20251001). It degrades transparently: missing render tools skip the phase with a note; a missing API key falls back to the free pure-Go heuristic inspector (advisory P3 findings). See the visual_qa response block's requirements field for the resolved preconditions and cost.

Replaces the agent's manual chain (generate_presentation → score_deck → propose_repairs → repair_slides_batch → generate_presentation) with a single tool call. The final PPTX is written to the server output directory either way; gate_passed reports whether convergence succeeded.

Gate fields (all optional, all defaulted) — these govern the DETERMINISTIC loop only:
- min_score (default 75): overall_score must be ≥ this value.
- max_p0_findings (default 0): max count of refuse-action findings tolerated.
- max_p1_findings (default 2): max count of shrink_or_split-action findings tolerated.
- require_takeaway_on_charts (default true): no takeaway_missing finding may remain.

Response shape: {path, final_score, gate_passed, passes, trace[], gate_reasons[], quality_mode, final_presentation, next_state, artifact_status, content_status, uses_exemplar_content, validation_status, publishable, manual_review_required, blocking_reasons[], evidence_complete, output_validation, render_evidence?, visual_qa?}. trace[i] = {pass, score, findings_count, repairs_applied[]} records score progression so the agent can audit convergence behavior. final_presentation is the full repaired deck JSON (always present, including zero-repair runs; reflects any visual_qa repairs too) — feed it straight back into validate_input / generate_presentation / repair_slide to keep editing without rebuilding state from the trace. visual_qa is present only when the mode was requested.

Resumable per-pass state (next_state, always present): {completion, resumable, resume_token, next_action, passes_run, next_pass?, max_passes, artifact_path, remaining_findings[]}. completion classifies how the loop stopped — "converged" (gate met on complete evidence; not resumable), "converged_degraded", "max_passes_exhausted", "no_progress", or "render_incomplete" — so a partial or degraded result is never mistaken for a converged one. When resumable is true, call auto_repair again with resume_token to continue from the saved post-repair deck WITHOUT repeating completed passes; gate and max_passes may be overridden on that call (e.g. relaxed bounds or a larger budget) while presentation is ignored. next_action is the suggested move; remaining_findings echoes the still-open findings (capped).

Publishability is reported explicitly so a successful transport response is never mistaken for a publishable deck: publishable is true ONLY when the gate passed on complete evidence AND the content is caller-authored (auto_repair always reports content_status="author_supplied"). When publishable is false, blocking_reasons enumerates every cause; manual_review_required is its affirmative inverse. artifact_status distinguishes a structurally valid PPTX ("generated") from one written but failed validation ("generated_invalid"); validation_status is "passed" / "passed_degraded" / "failed".

When gate_passed is false (max_passes exhausted), gate_reasons (and the superset blocking_reasons) list every unmet criterion so the agent can decide whether to call the tool again with relaxed bounds, switch templates, or escalate to human review.`),
		mcp.WithRawOutputSchema(outputSchemaAutoRepair),
		mcp.WithObject("presentation",
			mcp.Description(`Full presentation definition. Same schema as generate_presentation. Required for a fresh run; ignored (and not needed) when resume_token is supplied.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithString("template",
			mcp.Description("Template name override. If omitted, uses the template field from the presentation object."),
		),
		mcp.WithObject("gate",
			mcp.Description("Convergence gate configuration. All fields optional; omitted fields fall back to engine defaults (min_score=75, max_p0_findings=0, max_p1_findings=2, require_takeaway_on_charts=true)."),
			mcp.Properties(map[string]any{
				"min_score":                  map[string]any{"type": "integer", "description": "Minimum acceptable overall_score (default 75)."},
				"max_p0_findings":            map[string]any{"type": "integer", "description": "Maximum refuse-action findings tolerated (default 0)."},
				"max_p1_findings":            map[string]any{"type": "integer", "description": "Maximum shrink_or_split-action findings tolerated (default 2)."},
				"require_takeaway_on_charts": map[string]any{"type": "boolean", "description": "Require takeaway on chart/matrix slides (default true)."},
			}),
		),
		mcp.WithNumber("max_passes",
			mcp.Description("Maximum number of generate→inspect→repair iterations (default 3). Clamped to [1, 10]."),
		),
		mcp.WithObject("visual_qa",
			mcp.Description("Opt-in visual-QA mode. When enabled=true, runs a vision/heuristic visual refinement phase AFTER the deterministic loop (render thumbnails → inspect_slide_images → repair → re-render). Disabled by default. Requires libreoffice + magick on PATH; vision inspection additionally requires ANTHROPIC_API_KEY (otherwise falls back to the heuristic inspector). See the visual_qa response block for resolved requirements and cost."),
			mcp.Properties(map[string]any{
				"enabled":       map[string]any{"type": "boolean", "description": "Enable the visual-QA phase (default false)."},
				"model":         map[string]any{"type": "string", "description": "Claude vision model override (default claude-haiku-4-5-20251001)."},
				"audit_palette": map[string]any{"type": "boolean", "description": "Also run the deterministic palette ΔE audit (audit_palette) on the final deck (default false)."},
				"max_passes":    map[string]any{"type": "integer", "description": "Max visual render→inspect→repair iterations (default 1). Clamped to [1, 3]."},
				"density":       map[string]any{"type": "integer", "description": "Thumbnail DPI for inspection (default 50). Clamped to [25, 150]."},
			}),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: auto_repair.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used as the root for resolving relative local-asset paths (image_value.path, background.image, shape_grid image/icon paths). Required when any slide references a relative path and the agent cannot guarantee the server CWD matches the JSON's authoring directory. When omitted, the server falls back to its process CWD (not portable). Must be an absolute path to an existing directory. Same contract as generate_presentation."),
		),
		mcp.WithBoolean("allow_degraded_scoring",
			mcp.Description("Permit the loop to proceed when a per-pass render fails (slide conversion, temp-dir creation, or generation). Default false: a render failure emits a blocking RENDER_EVIDENCE_INCOMPLETE finding (refuse action) so gate_passed cannot be true on static-only evidence. Set true to converge on static analysis alone — render_evidence.degraded is then set and evidence_complete stays false even when gate_passed is true. Final structural output validation still runs and still blocks regardless of this flag."),
		),
		resumeTokenToolParam(),
		idempotencyKeyToolParam(),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleAutoRepair(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idemKey := idempotencyKey(request)
	idemFingerprint := requestFingerprint(request)
	switch cached, original, status := mc.idempotency.Lookup("auto_repair", idemKey, idemFingerprint); status {
	case idempotencyHit:
		if out, ok := cached.(*autoRepairOutput); ok {
			clone := *out
			clone.IdempotentReplay = true
			return api.MCPSuccessResult(ctx, &clone)
		}
	case idempotencyConflict:
		return idempotencyConflictResult("auto_repair", idemKey, idemFingerprint, original), nil
	case idempotencyMiss:
		// fall through and repair.
	}

	// Resume path: continue a saved convergence session from where it stopped.
	// The deck, trace, and provenance come from the checkpoint; presentation is
	// ignored. gate / max_passes / output_filename may be overridden on this
	// call, the rest inherited.
	if tok := resumeToken(request); tok != "" {
		cp, errResult := mc.loadResumeCheckpoint("auto_repair", tok)
		if errResult != nil {
			return errResult, nil
		}
		output, errResult := mc.runLoopResume(ctx, request, cp)
		if errResult != nil {
			return errResult, nil
		}
		return mc.respondAutoRepair(ctx, output, idemKey, idemFingerprint)
	}

	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("auto_repair", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nextCallGetInputSchema()), nil
	}
	applyDefaults(&input)
	mc.resolveInputNamedSettings(&input)

	// Honor template override (mirrors score_deck).
	if override, err := request.RequireString("template"); err == nil && override != "" {
		input.Template = override
	}

	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Resolve base_dir up front so relative local-asset paths resolve with the
	// same contract as generate_presentation. A malformed base_dir short-circuits
	// before the loop; runAutoRepairLoop rewrites the relative paths to absolute
	// form once (before the first pass) so every render in the loop embeds the
	// same assets generate_presentation would.
	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return baseDirErr, nil
	}

	gate := extractAutoRepairGate(request)
	maxPasses := extractMaxPasses(request)
	vqa, vqaErr := extractVisualQAConfig(request)
	if vqaErr != nil {
		return vqaErr, nil
	}
	allowDegraded := extractAllowDegradedScoring(request)

	outputFilename := sanitizeOutputFilename(input.OutputFilename)
	if outputFilename == "" {
		outputFilename = "auto_repair.pptx"
	}
	if reqFilename, err := request.RequireString("output_filename"); err == nil && reqFilename != "" {
		outputFilename = sanitizeOutputFilename(reqFilename)
	}

	output, errResult := mc.runAutoRepairLoop(ctx, &input, baseDir, gate, maxPasses, vqa, outputFilename, allowDegraded, contentProvenanceAuthorSupplied, "auto_repair", nil)
	if errResult != nil {
		return errResult, nil
	}

	return mc.respondAutoRepair(ctx, output, idemKey, idemFingerprint)
}

// runLoopResume continues a saved convergence session (auto_repair or make_deck)
// from its checkpoint. gate, max_passes (or max_repair_passes for make_deck),
// and output_filename may be overridden on the resume call; the deck, base_dir,
// visual_qa, allow_degraded_scoring, and provenance are inherited from the
// checkpoint so the resumed loop behaves like the original. cp.Tool — already
// validated to match the calling tool — labels the new checkpoint.
func (mc *mcpConfig) runLoopResume(ctx context.Context, request mcp.CallToolRequest, cp *loopCheckpoint) (*autoRepairOutput, *mcp.CallToolResult) {
	gate := extractAutoRepairGateOver(request, cp.Gate)
	maxPasses := extractResumeMaxPasses(request, cp.MaxPasses)
	outputFilename := cp.OutputFilename
	if reqFilename, err := request.RequireString("output_filename"); err == nil && reqFilename != "" {
		outputFilename = sanitizeOutputFilename(reqFilename)
	}
	// Deep-copy the checkpoint's deck: runAutoRepairLoop mutates the input in
	// place (resolution + repairs), so resuming the live cp.Input would corrupt
	// the stored snapshot and make a second resume of the same token start from a
	// half-repaired deck. The copy keeps every resume of a token deterministic.
	deck, errResult := cloneResumeDeck(cp.Input)
	if errResult != nil {
		return nil, errResult
	}
	resume := &loopResumeState{StartPass: cp.NextPass, Trace: cp.Trace}
	return mc.runAutoRepairLoop(ctx, deck, cp.BaseDir, gate, maxPasses, cp.VQA, outputFilename, cp.AllowDegraded, cp.Provenance, cp.Tool, resume)
}

// cloneResumeDeck returns an independent copy of a checkpoint's deck via a JSON
// round-trip, so the convergence loop can mutate it without touching the stored
// checkpoint. Returns an INTERNAL error result on the (unexpected) marshal
// failure rather than aliasing the stored deck.
func cloneResumeDeck(src *PresentationInput) (*PresentationInput, *mcp.CallToolResult) {
	data, err := json.Marshal(src)
	if err != nil {
		return nil, api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to clone resume deck: %v", err))
	}
	var dst PresentationInput
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to clone resume deck: %v", err))
	}
	return &dst, nil
}

// respondAutoRepair finalizes an auto_repair output: it persists the resumable
// checkpoint (minting the resume token echoed in next_state), caches the result
// for idempotent replay, and wraps it as an MCP success response. Shared by the
// fresh and resume paths so both attach the resume token before caching.
func (mc *mcpConfig) respondAutoRepair(ctx context.Context, output *autoRepairOutput, idemKey, idemFingerprint string) (*mcp.CallToolResult, error) {
	mc.finalizeResumeToken(output)
	mc.idempotency.Set("auto_repair", idemKey, idemFingerprint, output)

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// finalizeResumeToken persists the loop checkpoint and writes the minted token
// into next_state.resume_token. Callers that need tool-specific checkpoint data
// (e.g. make_deck's plan) must set it on output.checkpoint before calling. A nil
// store or absent checkpoint leaves resume_token empty (resume simply not
// offered).
func (mc *mcpConfig) finalizeResumeToken(output *autoRepairOutput) {
	if output == nil || output.checkpoint == nil || output.NextState == nil {
		return
	}
	output.NextState.ResumeToken = mc.loopSessions.Save(output.checkpoint)
}

// runAutoRepairLoop encapsulates the template-resolution + convergence-loop +
// final-render core of auto_repair. Extracted so make_deck (the cold-start
// facade) can reuse the same loop after building a PresentationInput from a
// plan rather than receiving one directly from the caller.
//
// Returns either a populated autoRepairOutput (errResult=nil) or an MCP error
// result the caller should pass straight through. Callers must wrap the
// successful output in api.MCPSuccessResult themselves so they can attach
// facade-specific fields (e.g. the deck plan summary in make_deck).
//
// tool labels which facade is running ("auto_repair" / "make_deck") so the
// stored checkpoint can be resumed only by the tool that created it. resume is
// nil for a fresh run; on resume it carries the global start pass and the trace
// to extend, and input is the checkpoint's post-repair deck — so the loop
// continues from where it stopped without repeating completed passes. The
// function builds output.checkpoint and output.NextState (minus the resume
// token, which the handler mints when it persists the checkpoint).
func (mc *mcpConfig) runAutoRepairLoop(
	ctx context.Context,
	input *PresentationInput,
	baseDir string,
	gate autoRepairGate,
	maxPasses int,
	vqa visualQAConfig,
	outputFilename string,
	allowDegraded bool,
	provenance contentProvenance,
	tool string,
	resume *loopResumeState,
) (*autoRepairOutput, *mcp.CallToolResult) {
	// Resolve relative local-asset paths (icons, images, background) against
	// base_dir once, before the convergence loop, mirroring generate_presentation
	// / validate_input. Resolution rewrites paths to absolute form in place; the
	// loop's repair edits never touch asset paths and already-absolute paths pass
	// through unchanged, so resolving once up front is correct for every pass.
	// Error-severity findings short-circuit (the caller passes the result
	// straight through), matching handleGenerate's contract.
	if assetFindings := resolveLocalAssetPaths(input.Slides, baseDir); len(assetFindings) > 0 {
		if assetErrors := diagnostics.FilterBySeverity(assetFindings, diagnostics.SeverityError); len(assetErrors) > 0 {
			return nil, api.MCPDiagnosticsError(assetErrors)
		}
	}

	// Resolve template once — reused on every pass.
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_NOT_FOUND", templateNotFoundError(input.Template, mc.templatesDir), nextCallListTemplates())
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates())
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates())
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        template.ParseTheme(reader),
	}
	template.SynthesizeIfNeeded(reader, analysis)
	layouts = analysis.Layouts
	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	templateMetadata, _ := template.ParseMetadata(reader)
	dataPalette := resolveDataPalette(templateMetadata, analysis.Theme.Colors)

	// Resolve canonical layout names (e.g. "title", "blank") to concrete
	// slideLayoutN IDs once, before findings collection and generation. This
	// lets callers (make_deck, agent-authored JSON) ship the portable canonical
	// names without forcing them to hard-code template-specific IDs. Already-
	// resolved IDs pass through unchanged.
	resolveCanonicalLayoutIDs(input.Slides, layouts)

	outputPath := filepath.Join(mc.outputDir, outputFilename)

	// Convergence loop. On a fresh run it covers passes 1..maxPasses. On resume
	// it starts at the checkpoint's NextPass and runs up to maxPasses MORE passes
	// (global pass numbering continues), seeding the trace with the prior session
	// so len(trace) always equals the total passes run. Each call therefore grants
	// a fresh budget of maxPasses passes from wherever the session left off.
	startPass := 1
	trace := make([]autoRepairTraceEntry, 0, maxPasses)
	if resume != nil {
		startPass = resume.StartPass
		trace = append(trace, resume.Trace...)
	}
	endPass := startPass + maxPasses - 1
	var lastFindings []patterns.FitFinding
	var lastScore int
	var lastGateReasons []string
	var lastRenderEvidence deterministic.RenderEvidence
	gatePassed := false
	stalled := false
	passesRun := startPass - 1

	for pass := startPass; pass <= endPass; pass++ {
		passesRun = pass

		findings := collectFitFindings(input, layouts, slideWidth, slideHeight, &analysis.Theme)
		renderFindings, renderEvidence := mc.collectRenderFindings(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette)
		findings = append(findings, renderFindings...)

		// A failed render pass must not look like a clean one: surface a
		// RENDER_EVIDENCE_INCOMPLETE finding so the gate sees a refuse finding
		// (default) and cannot pass on static-only evidence. allow_degraded
		// downgrades it to advisory and labels the score degraded.
		if !renderEvidence.Complete {
			renderEvidence.Degraded = allowDegraded
			findings = append(findings, renderEvidenceFinding(renderEvidence))
		}
		lastRenderEvidence = renderEvidence

		ds := deterministic.ScoreFromFindings(findings, len(input.Slides))
		gateReasons := evaluateAutoRepairGate(ds, findings, gate)

		entry := autoRepairTraceEntry{
			Pass:           pass,
			Score:          ds.OverallScore,
			FindingsCount:  len(findings),
			RepairsApplied: []string{},
		}

		lastFindings = findings
		lastScore = ds.OverallScore
		lastGateReasons = gateReasons

		if len(gateReasons) == 0 {
			gatePassed = true
			trace = append(trace, entry)
			break
		}

		if pass >= endPass {
			trace = append(trace, entry)
			break
		}

		proposed := proposeRepairs(input, fitFindingsToProposeFindings(findings))
		applied := applyProposedRepairs(input, proposed)
		entry.RepairsApplied = applied
		trace = append(trace, entry)

		if len(applied) == 0 {
			// The loop stalled: no repair landed yet the gate is unmet. Another
			// automatic pass would re-derive the same findings, so stop and let
			// next_state report no_progress.
			stalled = true
			break
		}
	}

	finalPath, renderErr := mc.renderAutoRepairFinal(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette, outputPath)
	if renderErr != nil {
		return nil, api.MCPSimpleError("GENERATION_FAILED", fmt.Sprintf("final generation failed: %v", renderErr))
	}

	// Opt-in visual-QA phase. Runs only when enabled; it mutates input (applying
	// any visual repairs) and rewrites finalPath so the on-disk PPTX and the
	// marshaled final_presentation below both reflect the visual repairs. Repairs
	// are staged atomically: a re-render failure rolls back the in-memory mutation
	// so final_presentation never advances past the PPTX at finalPath (see
	// visual_qa.artifact_consistent). It never errors — failures degrade to
	// recorded notes — so the deterministic deck is always preserved.
	var vqaResult *visualQAResult
	if vqa.Enabled {
		vqaResult = mc.runVisualQALoop(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette, finalPath, vqa)
	}

	// Final structural/output validation — the last evidence gate. Runs on the
	// on-disk PPTX AFTER the visual-QA phase (which can re-render it) and
	// regardless of degraded scoring: a structurally corrupt deck can never be
	// reported gate-passed / publishable. Blocking findings reopen the gate even
	// if the convergence loop had satisfied it.
	outputValidation := validateAutoRepairFinalOutput(finalPath)
	if !outputValidation.Valid {
		gatePassed = false
		for _, b := range outputValidation.Blocking {
			lastGateReasons = append(lastGateReasons, "final output validation: "+b)
		}
	}

	// evidence_complete is true only when the render pass that backed the score
	// completed AND final output validation passed. It is independent of the
	// degraded opt-in: a degraded run can still report gate_passed=true, but
	// evidence_complete stays false so the result is never mistaken for a clean
	// pass.
	evidenceComplete := lastRenderEvidence.Complete && outputValidation.Valid

	// Marshal the final repaired deck. input reflects every repair applied
	// during the loop (and visual_qa phase) plus the up-front asset-path and
	// canonical-layout resolution, so the JSON round-trips back into
	// validate_input / generate_presentation as-is — agents never have to
	// rebuild it from trace.
	finalPresentation, err := json.Marshal(input)
	if err != nil {
		return nil, api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal final presentation: %v", err))
	}

	// Derive the agent-native publishability status block from the loop's
	// terminal state (see deriveFacadeStatus).
	status := deriveFacadeStatus(gatePassed, evidenceComplete, outputValidation.Valid, lastGateReasons, provenance)

	// Report requested-vs-actual quality from the resolved visual-QA result rather
	// than the request alone, so a skipped/heuristic phase is never mislabeled as
	// a completed vision pass. quality_mode aliases quality.actual.
	quality := buildQualityReport(vqa, vqaResult)

	outVal := outputValidation
	output := &autoRepairOutput{
		Path:              finalPath,
		FinalScore:        lastScore,
		GatePassed:        gatePassed,
		Passes:            passesRun,
		Trace:             trace,
		GateReasons:       lastGateReasons,
		QualityMode:       quality.Actual,
		Quality:           &quality,
		VisualQA:          vqaResult,
		FinalPresentation: finalPresentation,
		EvidenceComplete:  evidenceComplete,
		OutputValidation:  &outVal,

		ArtifactStatus:       status.ArtifactStatus,
		ContentStatus:        status.ContentStatus,
		UsesExemplarContent:  status.UsesExemplarContent,
		ValidationStatus:     status.ValidationStatus,
		Publishable:          status.Publishable,
		ManualReviewRequired: status.ManualReviewRequired,
		BlockingReasons:      status.BlockingReasons,
	}
	if !lastRenderEvidence.Complete {
		re := lastRenderEvidence
		output.RenderEvidence = &re
	}
	if gatePassed {
		output.GateReasons = nil
	}

	// Build the resumable per-pass state (go-slide-creator-yope). completion
	// classifies how the loop terminated so a partial/degraded result is never
	// mistaken for a converged one; next_state exposes the resume affordance and
	// the findings still open. The handler mints and attaches the resume token
	// when it persists the checkpoint.
	completion := deriveLoopCompletion(gatePassed, evidenceComplete, lastRenderEvidence.Complete, stalled)
	nextState := &loopNextState{
		Completion:        completion,
		Resumable:         completion != loopCompletionConverged,
		NextAction:        nextActionForCompletion(completion),
		PassesRun:         passesRun,
		MaxPasses:         maxPasses,
		ArtifactPath:      finalPath,
		RemainingFindings: summarizeRemainingFindings(lastFindings),
	}
	if nextState.Resumable {
		nextState.NextPass = passesRun + 1
	}
	output.NextState = nextState

	// Snapshot the post-repair deck from the marshaled JSON (not the live
	// pointer) so the stored checkpoint is exactly the returned
	// final_presentation, decoupled from any later mutation. A resume continues
	// from this deck at NextPass, so completed passes never repeat.
	var checkpointInput PresentationInput
	if err := json.Unmarshal(finalPresentation, &checkpointInput); err != nil {
		return nil, api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to snapshot deck for resume: %v", err))
	}
	output.checkpoint = &loopCheckpoint{
		Tool:            tool,
		Input:           &checkpointInput,
		Trace:           trace,
		NextPass:        passesRun + 1,
		LastScore:       lastScore,
		LastGateReasons: lastGateReasons,
		GatePassed:      gatePassed,
		Completion:      completion,
		Gate:            gate,
		MaxPasses:       maxPasses,
		VQA:             vqa,
		OutputFilename:  outputFilename,
		BaseDir:         baseDir,
		AllowDegraded:   allowDegraded,
		Provenance:      provenance,
	}
	return output, nil
}

// facadeStatus bundles the derived agent-native publishability fields so
// runAutoRepairLoop stays within complexity limits and the logic lives in one
// place.
type facadeStatus struct {
	ArtifactStatus       string
	ContentStatus        string
	UsesExemplarContent  bool
	ValidationStatus     string
	Publishable          bool
	ManualReviewRequired bool
	BlockingReasons      []string
}

// deriveFacadeStatus computes the publishability status block from the loop's
// terminal state. publishable is the single source of truth, defined as "no
// blocking reasons", so the boolean and the human-readable list can never
// disagree. blockingReasons folds together every distinct failure: the unmet
// gate criteria (gateReasons already includes final-output-validation failures),
// a degraded/incomplete-evidence pass that nonetheless satisfied the gate, and
// exemplar content provenance.
func deriveFacadeStatus(gatePassed, evidenceComplete, outputValid bool, gateReasons []string, provenance contentProvenance) facadeStatus {
	usesExemplar := provenance == contentProvenanceExemplarSkeleton
	blocking := append([]string(nil), gateReasons...)
	if gatePassed && !evidenceComplete {
		blocking = append(blocking,
			"validation evidence incomplete: gate passed on degraded/static-only evidence (evidence_complete=false); render evidence or final output validation did not complete cleanly")
	}
	if usesExemplar {
		blocking = append(blocking,
			"content is an exemplar skeleton (pattern placeholder values), not author-supplied; replace per-slide content via repair_slide before publishing")
	}
	publishable := len(blocking) == 0

	artifactStatus := "generated"
	if !outputValid {
		artifactStatus = "generated_invalid"
	}
	validationStatus := "failed"
	if gatePassed {
		validationStatus = "passed_degraded"
		if evidenceComplete {
			validationStatus = "passed"
		}
	}
	return facadeStatus{
		ArtifactStatus:       artifactStatus,
		ContentStatus:        string(provenance),
		UsesExemplarContent:  usesExemplar,
		ValidationStatus:     validationStatus,
		Publishable:          publishable,
		ManualReviewRequired: !publishable,
		BlockingReasons:      blocking,
	}
}

// --- Gate evaluation ---

// evaluateAutoRepairGate returns a list of unmet criteria. Empty result means
// the deck satisfies the gate. Order is deterministic: score → P0 → P1 →
// takeaway, so agents can pattern-match on the leading reason.
func evaluateAutoRepairGate(ds *deterministic.DeckScore, findings []patterns.FitFinding, gate autoRepairGate) []string {
	var reasons []string
	if ds.OverallScore < gate.MinScore {
		reasons = append(reasons, fmt.Sprintf("score %d < min_score %d", ds.OverallScore, gate.MinScore))
	}
	p0 := countFindingsByAction(findings, "refuse")
	if p0 > gate.MaxP0Findings {
		reasons = append(reasons, fmt.Sprintf("%d P0 (refuse) findings exceeds max_p0_findings %d", p0, gate.MaxP0Findings))
	}
	p1 := countFindingsByAction(findings, "shrink_or_split")
	if p1 > gate.MaxP1Findings {
		reasons = append(reasons, fmt.Sprintf("%d P1 (shrink_or_split) findings exceeds max_p1_findings %d", p1, gate.MaxP1Findings))
	}
	if gate.RequireTakeawayOnCharts {
		missing := countFindingsByCode(findings, patterns.ErrCodeTakeawayMissing)
		if missing > 0 {
			reasons = append(reasons, fmt.Sprintf("%d chart/matrix slide(s) missing takeaway", missing))
		}
	}
	return reasons
}

func countFindingsByAction(findings []patterns.FitFinding, action string) int {
	n := 0
	for _, f := range findings {
		if f.Action == action {
			n++
		}
	}
	return n
}

func countFindingsByCode(findings []patterns.FitFinding, code string) int {
	n := 0
	for _, f := range findings {
		if f.Code == code {
			n++
		}
	}
	return n
}

// --- Helpers ---

// extractAutoRepairGate reads the gate object from the request, filling in
// every field with the engine default when absent. The default for
// RequireTakeawayOnCharts is true, so we have to distinguish "explicit false"
// from "absent" — that requires a presence check rather than a zero-value
// fallback.
func extractAutoRepairGate(request mcp.CallToolRequest) autoRepairGate {
	gate := autoRepairGate{
		MinScore:                defaultAutoRepairMinScore,
		MaxP0Findings:           defaultAutoRepairMaxP0Findings,
		MaxP1Findings:           defaultAutoRepairMaxP1Findings,
		RequireTakeawayOnCharts: defaultAutoRepairRequireTakeawayOnCharts,
	}
	args := request.GetArguments()
	raw, ok := args["gate"]
	if !ok || raw == nil {
		return gate
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return gate
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return gate
	}
	if v, ok := parsed["min_score"]; ok {
		gate.MinScore = anyToInt(v, gate.MinScore)
	}
	if v, ok := parsed["max_p0_findings"]; ok {
		gate.MaxP0Findings = anyToInt(v, gate.MaxP0Findings)
	}
	if v, ok := parsed["max_p1_findings"]; ok {
		gate.MaxP1Findings = anyToInt(v, gate.MaxP1Findings)
	}
	if v, ok := parsed["require_takeaway_on_charts"]; ok {
		if b, ok := v.(bool); ok {
			gate.RequireTakeawayOnCharts = b
		}
	}
	return gate
}

func anyToInt(v any, fallback int) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return fallback
		}
		return int(i)
	case int:
		return x
	case int64:
		return int(x)
	}
	return fallback
}

// extractMaxPasses reads the max_passes parameter, clamping to [1, 10].
func extractMaxPasses(request mcp.CallToolRequest) int {
	max := defaultAutoRepairMaxPasses
	args := request.GetArguments()
	if raw, ok := args["max_passes"]; ok {
		max = anyToInt(raw, defaultAutoRepairMaxPasses)
	}
	if max < 1 {
		max = 1
	}
	if max > 10 {
		max = 10
	}
	return max
}

// fitFindingsToProposeFindings adapts FitFindings into the polymorphic shape
// proposeRepairs accepts. proposeRepairs's internal logic understands both
// fit-finding and visual-QA shapes; we feed it the fit-finding fields here.
func fitFindingsToProposeFindings(findings []patterns.FitFinding) []proposeRepairsFinding {
	out := make([]proposeRepairsFinding, len(findings))
	for i, f := range findings {
		out[i] = proposeRepairsFinding{
			Pattern: f.Pattern,
			Path:    f.Path,
			Code:    f.Code,
			Message: f.Message,
			Fix:     f.Fix,
			Action:  f.Action,
		}
	}
	return out
}

// applyProposedRepairs walks each proposed slide's directives and applies the
// FIRST applicable repair per slide. Applying every ranked directive on a
// single slide tends to double-up (e.g. two reduce_text calls on the same
// body) without improving the score; the propose_repairs ranking already
// surfaces the best repair first, so taking just the top directive per slide
// is the right convergence step. Returns a human-readable summary of each
// repair that actually landed, suitable for the trace.
func applyProposedRepairs(input *PresentationInput, proposed proposeRepairsOutput) []string {
	var applied []string
	for _, slide := range proposed.Slides {
		for _, dir := range slide.Directives {
			params := adaptAutoRepairParams(input, slide.SlideIndex, dir.Kind, dir.Params)
			result := applyRepairFix(input, slide.SlideIndex, repairFixInput{Kind: dir.Kind, Params: params})
			if result.Applied {
				applied = append(applied, fmt.Sprintf("%s on slide %d", dir.Kind, slide.SlideIndex))
				break
			}
		}
	}
	return applied
}

// adaptAutoRepairParams translates fit-finding fix params (which describe the
// PROBLEM — e.g. current_words=120, max_words=80) into repair_slide params
// (which describe the ACTION — e.g. max_items=4). The agent normally bridges
// this vocabulary mismatch by hand; auto_repair removes the agent from the
// loop, so it must do the translation itself.
//
// Today this only matters for reduce_text against BODY_TOO_LONG findings, but
// the function is structured so additional kind→params adapters can be added
// without changing the call site.
func adaptAutoRepairParams(input *PresentationInput, slideIdx int, kind string, params map[string]any) map[string]any {
	if kind != "reduce_text" || params == nil {
		return params
	}
	if hasActionableReduceTextParam(params) {
		return params
	}
	maxWords := anyToInt(params["max_words"], 0)
	if maxWords <= 0 {
		return params
	}
	if slideIdx < 0 || slideIdx >= len(input.Slides) {
		return params
	}

	body := pickBodyContentForReduce(&input.Slides[slideIdx])
	if body == nil {
		return params
	}
	return reduceTextParamsForBody(params, body, maxWords)
}

// hasActionableReduceTextParam returns true when params already carry a
// repair_slide-actionable directive (max_items or max_length), in which case
// we trust the caller's intent and skip translation.
func hasActionableReduceTextParam(params map[string]any) bool {
	if _, ok := params["max_items"]; ok {
		return true
	}
	if _, ok := params["max_length"]; ok {
		return true
	}
	return false
}

// pickBodyContentForReduce finds the content item that BODY_TOO_LONG most
// likely refers to. Preference order: a "body*" placeholder with reducible
// content; then any non-title content with bullets; then any non-title text.
func pickBodyContentForReduce(slide *SlideInput) *ContentInput {
	for i := range slide.Content {
		ci := &slide.Content[i]
		if !strings.HasPrefix(ci.PlaceholderID, "body") {
			continue
		}
		if isReducibleContent(ci) {
			return ci
		}
	}
	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID == "title" {
			continue
		}
		if ci.BulletsValue != nil || ci.BodyAndBulletsValue != nil {
			return ci
		}
	}
	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID == "title" {
			continue
		}
		if ci.TextValue != nil {
			return ci
		}
	}
	return nil
}

func isReducibleContent(ci *ContentInput) bool {
	return ci.BulletsValue != nil || ci.BodyAndBulletsValue != nil || ci.TextValue != nil
}

// reduceTextParamsForBody computes the {max_items} or {max_length} that
// brings the body's word count within maxWords. Returns the original params
// unchanged when no truncation is needed.
func reduceTextParamsForBody(params map[string]any, body *ContentInput, maxWords int) map[string]any {
	out := cloneStringMap(params)
	switch {
	case body.BulletsValue != nil:
		return assignBulletReduce(out, *body.BulletsValue, maxWords, params)
	case body.BodyAndBulletsValue != nil:
		return assignBulletReduce(out, body.BodyAndBulletsValue.Bullets, maxWords, params)
	case body.TextValue != nil:
		out["max_length"] = maxWords * 6
		return out
	}
	return params
}

// assignBulletReduce walks bullets and sets max_items / max_length on out
// according to where the running word total first crosses maxWords. Returns
// originalParams when every bullet fits — no repair needed.
func assignBulletReduce(out map[string]any, bullets []string, maxWords int, originalParams map[string]any) map[string]any {
	words := 0
	for i, b := range bullets {
		w := len(strings.Fields(b))
		if words+w > maxWords {
			if i == 0 {
				// Single bullet already over budget; truncate by char count
				// instead. ~6 chars per word is a workable approximation
				// (English averages ~5 chars/word + space).
				out["max_length"] = maxWords * 6
				return out
			}
			out["max_items"] = i
			return out
		}
		words += w
	}
	return originalParams
}

func cloneStringMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// renderAutoRepairFinal renders the converged (or best-effort) deck to the
// configured output path. Mirrors collectRenderFindings's pipeline assembly
// but writes to a stable location instead of a temp dir.
func (mc *mcpConfig) renderAutoRepairFinal(
	ctx context.Context,
	input *PresentationInput,
	templatePath string,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
	syntheticFiles map[string][]byte,
	templateMetadata *types.TemplateMetadata,
	dataPalette []string,
	outputPath string,
) (string, error) {
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		rhythmGrid = resolveGrid(input.Grid, layouts, slideWidth, slideHeight)
	}

	slideSpecs, _, _, err := convertPresentationSlides(input.Slides, layouts, slideWidth, slideHeight, templateMetadata, rhythmGrid, patterns.AccentStrategy(input.AccentStrategy), nil, false)
	if err != nil {
		return "", fmt.Errorf("convert slides: %w", err)
	}

	genReq := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		SVGStrategy:           string(mc.cfg.SVG.Strategy),
		SVGScale:              mc.cfg.SVG.Scale,
		SVGNativeCompat:       string(mc.cfg.SVG.NativeCompatibility),
		MaxPNGWidth:           mc.cfg.SVG.MaxPNGWidth,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
		StrictFit:             "warn",
		DataPalette:           dataPalette,
	}
	if input.Chrome != nil {
		genReq.Footer = chromeToFooterConfig(input.Chrome, len(slideSpecs))
		applyChromeSkip(slideSpecs, input.Chrome, input.Slides, layouts)
	} else if input.Footer != nil && input.Footer.Enabled {
		genReq.Footer = &generator.FooterConfig{
			Enabled:  true,
			LeftText: input.Footer.LeftText,
		}
	}
	if input.ThemeOverride != nil {
		genReq.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}

	if _, err := generator.Generate(ctx, genReq); err != nil {
		return "", err
	}
	return outputPath, nil
}

// mcp_visual_qa.go implements the opt-in visual_qa mode shared by auto_repair
// and make_deck. The default convergence loop (in mcp_auto_repair.go) is
// DETERMINISTIC: it scores a deck from static + render-fit findings without ever
// looking at a rendered pixel. visual_qa mode layers the agent-grade visual
// refinement loop on top — render thumbnails → inspect_slide_images → map visual
// findings to propose_repairs → apply → re-render — and can additionally run the
// deterministic palette ΔE audit (audit_palette).
//
// The mode is opt-in and disabled by default because it has real preconditions
// (LibreOffice + ImageMagick on PATH for rendering) and real cost (a Claude
// vision call per slide when ANTHROPIC_API_KEY is set). When those preconditions
// are absent the loop degrades transparently: missing render tools skip the
// phase with an explanatory note; a missing API key falls back to the pure-Go
// heuristic inspector (advisory P3 findings) instead of erroring.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa"
	"github.com/sebahrens/json2pptx/internal/visualqa/heuristic"
)

// --- Defaults ---

const (
	defaultVisualQAMaxPasses = 1
	maxVisualQAMaxPasses     = 3
	defaultVisualQADensity   = 50
	minVisualQADensity       = 25
	maxVisualQADensity       = 150
)

// qualityModeDeterministic and qualityModeVisualQA are the truth-labels the
// response carries so an agent can tell which inspection regime actually ran.
const (
	qualityModeDeterministic = "deterministic"
	qualityModeVisualQA      = "deterministic+visual_qa"
)

// qualityModeLabel returns the response truth-label for the resolved mode.
func qualityModeLabel(visualQAEnabled bool) string {
	if visualQAEnabled {
		return qualityModeVisualQA
	}
	return qualityModeDeterministic
}

// --- Config ---

// visualQAConfig is the parsed visual_qa request block. Every field is optional;
// the zero value (Enabled=false) keeps the deterministic-only behaviour.
type visualQAConfig struct {
	Enabled      bool
	Model        string
	AuditPalette bool
	MaxPasses    int
	Density      int
}

// extractVisualQAConfig parses the visual_qa object argument, applying defaults
// and clamping. A missing block returns the disabled zero value. A malformed
// block (present but not an object) is tolerated: it disables the mode rather
// than failing the whole call, so a typo never blocks deck generation.
func extractVisualQAConfig(request mcp.CallToolRequest) visualQAConfig {
	cfg := visualQAConfig{MaxPasses: defaultVisualQAMaxPasses, Density: defaultVisualQADensity}
	raw, ok := request.GetArguments()["visual_qa"]
	if !ok || raw == nil {
		return visualQAConfig{} // disabled
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return visualQAConfig{}
	}
	var parsed struct {
		Enabled      *bool  `json:"enabled"`
		Model        string `json:"model"`
		AuditPalette *bool  `json:"audit_palette"`
		MaxPasses    *int   `json:"max_passes"`
		Density      *int   `json:"density"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return visualQAConfig{}
	}
	if parsed.Enabled != nil {
		cfg.Enabled = *parsed.Enabled
	}
	cfg.Model = parsed.Model
	if parsed.AuditPalette != nil {
		cfg.AuditPalette = *parsed.AuditPalette
	}
	if parsed.MaxPasses != nil {
		cfg.MaxPasses = *parsed.MaxPasses
	}
	if parsed.Density != nil {
		cfg.Density = *parsed.Density
	}
	if cfg.MaxPasses < 1 {
		cfg.MaxPasses = 1
	}
	if cfg.MaxPasses > maxVisualQAMaxPasses {
		cfg.MaxPasses = maxVisualQAMaxPasses
	}
	if cfg.Density < minVisualQADensity {
		cfg.Density = minVisualQADensity
	}
	if cfg.Density > maxVisualQADensity {
		cfg.Density = maxVisualQADensity
	}
	return cfg
}

// --- Response types ---

// visualQAResult is the visual-QA phase report attached to the auto_repair /
// make_deck response when visual_qa mode is requested. It is omitted entirely
// when the mode is disabled, so deterministic-only callers see no change.
type visualQAResult struct {
	// Requested is always true (the struct is only built when the mode is on).
	Requested bool `json:"requested"`
	// ArtifactConsistent reports whether the returned final_presentation matches
	// the PPTX written at the response path. It is true on every normal run
	// (including transparent fallbacks): each visual-repair pass is staged —
	// applied, re-rendered, and rolled back in memory if the re-render fails — so
	// the JSON and the on-disk PPTX always advance together. It is false ONLY in
	// the defensive case where a re-render failed AND the in-memory repairs could
	// not be reverted; final_presentation then reflects changes the PPTX does not,
	// a blocking note explains the divergence, and the artifact must not be
	// shipped.
	ArtifactConsistent bool `json:"artifact_consistent"`
	// InspectionMode reports which backend actually ran: "vision" (Claude vision
	// API), "heuristic" (pure-Go fallback, no API key), or "skipped" (render
	// tools unavailable — no inspection happened at all).
	InspectionMode string `json:"inspection_mode"`
	// InspectionComplete is false when any inspected pass had per-slide
	// inspection failures (an API/transport/decode error in vision mode, or an
	// undecodable thumbnail in heuristic mode). When false, an empty
	// visual_findings list does NOT mean the deck is visually clean — the
	// inspection itself did not fully succeed, so visual QA is inconclusive.
	InspectionComplete bool `json:"inspection_complete"`
	// FailedSlideCount is the number of slides whose inspection failed in the
	// last pass that ran inspection. It is 0 on a fully successful inspection and
	// on a skipped phase.
	FailedSlideCount int                   `json:"failed_slide_count"`
	Model            string                `json:"model,omitempty"`
	Requirements     visualQARequirements  `json:"requirements"`
	Passes           []visualQAPassEntry   `json:"passes"`
	PaletteAudit     *visualQAPaletteAudit `json:"palette_audit,omitempty"`
	// Notes carries human-readable explanations for transparent fallbacks
	// (missing render tools, missing API key, re-render failures).
	Notes []string `json:"notes,omitempty"`
}

// visualQARequirements documents the preconditions and cost of vision-backed
// inspection so an agent can decide whether to enable the mode and what it will
// cost before paying for it.
type visualQARequirements struct {
	APIKeyEnv          string   `json:"api_key_env"`
	APIKeyPresent      bool     `json:"api_key_present"`
	DefaultModel       string   `json:"default_model"`
	RenderDependencies []string `json:"render_dependencies"`
	RenderAvailable    bool     `json:"render_available"`
	RenderMissing      []string `json:"render_missing,omitempty"`
	CostNote           string   `json:"cost_note"`
}

// visualQAPassEntry records one render→inspect→repair iteration.
type visualQAPassEntry struct {
	Pass           int    `json:"pass"`
	InspectionMode string `json:"inspection_mode"`
	// FailedSlideCount is the number of slides whose inspection failed during
	// this pass (SlideResult.Error set). InspectionStatus classifies the pass as
	// "complete", "partial", or "failed". When not "complete", an empty
	// visual_findings list reflects failed inspection, not a clean deck.
	FailedSlideCount int                      `json:"failed_slide_count"`
	InspectionStatus string                   `json:"inspection_status"`
	ThumbnailPaths   []string                 `json:"thumbnail_paths"`
	VisualFindings   []visualqa.Finding       `json:"visual_findings"`
	ProposedRepairs  []visualQAProposedRepair `json:"proposed_repairs"`
	RepairsApplied   []string                 `json:"repairs_applied"`
}

// visualQAProposedRepair is one ranked directive proposed for a slide from a
// visual finding, flattened for the trace.
type visualQAProposedRepair struct {
	SlideIndex int    `json:"slide_index"`
	Kind       string `json:"kind"`
	Category   string `json:"category,omitempty"`
}

// visualQAPaletteAudit holds the optional palette ΔE audit result.
type visualQAPaletteAudit struct {
	Available  bool                        `json:"available"`
	Violations int                         `json:"violations"`
	Findings   diagnostics.FindingEnvelope `json:"findings"`
	Note       string                      `json:"note,omitempty"`
}

// buildVisualQARequirements assembles the precondition/cost summary. It performs
// no I/O beyond an environment lookup and a PATH probe.
func buildVisualQARequirements(cfg visualQAConfig) visualQARequirements {
	renderAvail, renderMissing := render.DependencyStatus()
	model := cfg.Model
	if model == "" {
		model = visualqa.DefaultModel()
	}
	return visualQARequirements{
		APIKeyEnv:          "ANTHROPIC_API_KEY",
		APIKeyPresent:      os.Getenv("ANTHROPIC_API_KEY") != "",
		DefaultModel:       model,
		RenderDependencies: []string{"libreoffice", "magick"},
		RenderAvailable:    renderAvail,
		RenderMissing:      renderMissing,
		CostNote:           "Rendering needs libreoffice + magick on PATH. Vision inspection issues one Claude vision call per slide (default model claude-haiku-4-5-20251001) and requires ANTHROPIC_API_KEY; without the key the mode falls back to the free pure-Go heuristic inspector (advisory P3 findings).",
	}
}

// --- Loop ---

// runVisualQALoop renders the just-generated deck, inspects the thumbnails,
// maps visual findings to repairs, applies them, and re-renders. Each pass is
// staged atomically (see applyAndReRenderVisualRepairs): the in-memory mutation
// to input and the on-disk PPTX at outputPath advance together, or — if the
// re-render of the repaired deck fails — the mutation is rolled back so the
// caller's final_presentation stays consistent with the PPTX. result.Notes and
// result.ArtifactConsistent record any such failure.
//
// It never returns an error: every failure mode degrades to a recorded note so
// the deterministic deck the caller already produced is preserved. The returned
// *visualQAResult is always non-nil when the mode is enabled.
func (mc *mcpConfig) runVisualQALoop(
	ctx context.Context,
	input *PresentationInput,
	templatePath string,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
	syntheticFiles map[string][]byte,
	templateMetadata *types.TemplateMetadata,
	dataPalette []string,
	outputPath string,
	cfg visualQAConfig,
) *visualQAResult {
	result := &visualQAResult{
		Requested:          true,
		ArtifactConsistent: true,
		InspectionComplete: true,
		Requirements:       buildVisualQARequirements(cfg),
		Passes:             []visualQAPassEntry{},
	}

	if !result.Requirements.RenderAvailable {
		result.InspectionMode = "skipped"
		result.Notes = append(result.Notes, fmt.Sprintf(
			"visual_qa skipped: render tools unavailable (%v). Install libreoffice + magick to enable visual inspection.",
			result.Requirements.RenderMissing))
		return result
	}

	infos := visualQASlideInfos(input.Slides)
	mode := ""

	for pass := 1; pass <= cfg.MaxPasses; pass++ {
		deck, err := render.RenderDeckOpts(outputPath, cfg.Density, len(input.Slides), false)
		if err != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("visual_qa pass %d: render failed: %v", pass, err))
			break
		}

		images, paths := buildVisualQAImages(deck, infos)
		report, passMode := mc.inspectVisualQA(ctx, images, cfg.Model)
		mode = passMode

		failedSlides, inspectionStatus := inspectionFailureStats(report)
		entry := visualQAPassEntry{
			Pass:             pass,
			InspectionMode:   passMode,
			FailedSlideCount: failedSlides,
			InspectionStatus: inspectionStatus,
			ThumbnailPaths:   paths,
			VisualFindings:   flattenVisualFindings(report),
			ProposedRepairs:  []visualQAProposedRepair{},
			RepairsApplied:   []string{},
		}

		// A pass with inspection failures cannot vouch for the slides it failed to
		// inspect: mark the whole phase incomplete and record the most recent
		// failure count so an empty visual_findings list is never read as a clean
		// deck.
		if failedSlides > 0 {
			result.InspectionComplete = false
			result.FailedSlideCount = failedSlides
		}

		actionable := actionableVisualFindings(entry.VisualFindings)
		if len(actionable) == 0 {
			// Zero actionable findings from a pass whose inspection failed reflects
			// the failure, not a clean deck — surface it explicitly so the loop is
			// not mistaken for a successful convergence.
			if failedSlides > 0 {
				result.Notes = append(result.Notes, fmt.Sprintf(
					"visual_qa pass %d: inspection %s for %d/%d slide(s); zero actionable findings here reflects failed inspection, not a clean deck — treat visual QA as inconclusive (see VISION_INSPECTION_FAILED / HEURISTIC_INSPECTION_FAILED).",
					pass, inspectionStatus, failedSlides, len(report.Results)))
			}
			result.Passes = append(result.Passes, entry)
			break
		}

		proposed := proposeRepairs(input, visualFindingsToProposeFindings(actionable))
		entry.ProposedRepairs = flattenProposedRepairs(proposed)

		// Stage the repairs atomically: apply them, re-render, and roll back the
		// in-memory mutation if the re-render fails. This keeps the marshaled
		// final_presentation and the on-disk PPTX consistent — they advance
		// together or neither does, so a re-render failure can never leave the
		// caller with repaired JSON pointing at a pre-repair PPTX.
		outcome := applyAndReRenderVisualRepairs(input, proposed, func() error {
			_, rerr := mc.renderAutoRepairFinal(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette, outputPath)
			return rerr
		})
		entry.RepairsApplied = outcome.Applied
		if entry.RepairsApplied == nil {
			entry.RepairsApplied = []string{}
		}
		result.Passes = append(result.Passes, entry)

		switch {
		case outcome.RolledBack:
			result.Notes = append(result.Notes, fmt.Sprintf(
				"visual_qa pass %d: re-render after repair failed (%v); reverted the in-memory repair(s) so final_presentation stays consistent with the on-disk PPTX at the returned path.",
				pass, outcome.Err))
		case !outcome.Consistent:
			result.ArtifactConsistent = false
			result.Notes = append(result.Notes, fmt.Sprintf(
				"visual_qa pass %d: re-render after repair failed (%v) and the in-memory repairs could not be reverted; final_presentation reflects changes the on-disk PPTX at the returned path does NOT — artifact_consistent=false, do not ship this artifact.",
				pass, outcome.Err))
		}

		// Stop when nothing more landed, the repairs were reverted, or the
		// artifacts diverged. Otherwise the re-render succeeded and the next pass
		// inspects the freshly rendered deck.
		if len(outcome.Applied) == 0 || outcome.RolledBack || !outcome.Consistent {
			break
		}
	}

	if mode == "" {
		mode = "skipped"
	}
	result.InspectionMode = mode
	// Only vision mode actually used a model; heuristic/skipped leave it empty.
	if mode == "vision" {
		result.Model = result.Requirements.DefaultModel
	}

	if cfg.AuditPalette {
		result.PaletteAudit = runVisualQAPaletteAudit(outputPath)
	}

	return result
}

// visualRepairOutcome reports how one staged visual-repair pass resolved.
type visualRepairOutcome struct {
	// Applied lists the repairs that landed in the FINAL deck. It is empty when
	// no repair applied, or when repairs were applied but reverted after a
	// re-render failure (RolledBack).
	Applied []string
	// RolledBack is true when repairs were applied, the re-render failed, and the
	// in-memory mutation was reverted to keep the deck consistent with the
	// on-disk PPTX.
	RolledBack bool
	// Consistent is true when the in-memory deck matches the last successfully
	// rendered PPTX. It is false only when a re-render failed AND the rollback
	// could not be performed — leaving the JSON ahead of the PPTX.
	Consistent bool
	// Err is the re-render error, if any.
	Err error
}

// applyAndReRenderVisualRepairs applies proposed repairs to input, then invokes
// rerender so the on-disk PPTX reflects them. It snapshots the pre-repair deck
// first so a re-render failure can be rolled back: on failure it restores input
// to the snapshot, keeping the marshaled final_presentation consistent with the
// still-on-disk pre-repair PPTX. Repairs and the rendered artifact therefore
// advance atomically — both or neither. Consistency is lost only in the
// defensive case where the rollback itself cannot be performed.
func applyAndReRenderVisualRepairs(input *PresentationInput, proposed proposeRepairsOutput, rerender func() error) visualRepairOutcome {
	// Snapshot before mutating so we can roll back on a re-render failure.
	snapshot, snapErr := json.Marshal(input)

	applied := applyProposedRepairs(input, proposed)
	if len(applied) == 0 {
		// Nothing changed; the on-disk PPTX already matches input.
		return visualRepairOutcome{Consistent: true}
	}

	if rerr := rerender(); rerr != nil {
		// The repaired deck failed to re-render. Roll back so the returned
		// final_presentation matches the pre-repair PPTX still on disk.
		if snapErr == nil {
			var restored PresentationInput
			if rbErr := json.Unmarshal(snapshot, &restored); rbErr == nil {
				*input = restored
				return visualRepairOutcome{RolledBack: true, Consistent: true, Err: rerr}
			}
		}
		// Could not roll back: input reflects repairs the on-disk PPTX does not.
		return visualRepairOutcome{Applied: applied, Consistent: false, Err: rerr}
	}

	return visualRepairOutcome{Applied: applied, Consistent: true}
}

// inspectVisualQA runs the vision agent when ANTHROPIC_API_KEY is set, otherwise
// the pure-Go heuristic fallback. Mirrors handleInspectSlideImages's backend
// selection so both surfaces report the same mode semantics.
func (mc *mcpConfig) inspectVisualQA(ctx context.Context, images []visualqa.SlideImage, model string) (*visualqa.Report, string) {
	var opts []visualqa.Option
	if model != "" {
		opts = append(opts, visualqa.WithModel(model))
	}
	if agent, err := visualqa.NewAgent(opts...); err == nil {
		report := agent.InspectAll(ctx, images)
		report.Mode = "vision"
		for ri := range report.Results {
			for fi := range report.Results[ri].Findings {
				if report.Results[ri].Findings[fi].Source == "" {
					report.Results[ri].Findings[fi].Source = "vision"
				}
			}
		}
		return report, "vision"
	}
	report := heuristic.InspectAll(images)
	return report, "heuristic"
}

// runVisualQAPaletteAudit runs the deterministic palette ΔE audit on the final
// PPTX, degrading to an unavailable result (rather than an error) when the audit
// tooling is missing.
func runVisualQAPaletteAudit(outputPath string) *visualQAPaletteAudit {
	report, err := auditPalettePPTX(outputPath, auditOptions{
		MaxDeltaE: 5.0,
		ChromaMin: 25,
		Density:   150,
		TmpDir:    "",
		Keep:      false,
	})
	if err != nil {
		return &visualQAPaletteAudit{
			Available: false,
			Findings:  diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "audit_palette"}, nil),
			Note:      fmt.Sprintf("palette audit unavailable: %v", err),
		}
	}
	for i := range report.Slides {
		report.Slides[i].RenderImage = ""
	}
	return &visualQAPaletteAudit{
		Available:  true,
		Violations: report.Violations,
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand: "audit_palette",
		}, diagnosticsFromAuditReport(report)),
	}
}

// --- Helpers ---

// visualQASlideInfos derives per-slide inspection metadata (type + title) from
// the deck so the vision prompt can be tuned per slide. Type defaults to
// "content" to match inspect_slide_images's own default.
func visualQASlideInfos(slides []SlideInput) map[int]visualqa.SlideInfo {
	infos := make(map[int]visualqa.SlideInfo, len(slides))
	for i := range slides {
		infos[i] = visualqa.SlideInfo{
			Index: i,
			Type:  "content",
			Title: extractTitleFromSlide(&slides[i]),
		}
	}
	return infos
}

// buildVisualQAImages materializes each rendered thumbnail into inspection bytes
// plus a stable on-disk path for the trace. Slides that failed to render (no
// inline data and no path) are skipped; the rest carry their slide metadata.
func buildVisualQAImages(deck *render.DeckResult, infos map[int]visualqa.SlideInfo) ([]visualqa.SlideImage, []string) {
	images := make([]visualqa.SlideImage, 0, len(deck.Slides))
	paths := make([]string, 0, len(deck.Slides))
	for _, s := range deck.Slides {
		data, path := materializeThumbnail(s)
		if len(data) == 0 {
			continue
		}
		info := infos[s.Index]
		info.Index = s.Index
		if info.Type == "" {
			info.Type = "content"
		}
		images = append(images, visualqa.SlideImage{Info: info, Data: data})
		if path != "" {
			paths = append(paths, path)
		}
	}
	return images, paths
}

// materializeThumbnail returns the PNG bytes for a rendered slide plus a stable
// filesystem path. Small thumbnails arrive inline as base64 (decode + persist a
// content-addressed artifact for the path); large ones already carry a path.
func materializeThumbnail(img render.SlideImage) ([]byte, string) {
	switch {
	case img.PNG64 != "":
		data, err := base64.StdEncoding.DecodeString(img.PNG64)
		if err != nil || len(data) == 0 {
			return nil, ""
		}
		path, perr := render.WriteArtifact(data)
		if perr != nil {
			path = ""
		}
		return data, path
	case img.Path != "":
		data, err := os.ReadFile(img.Path)
		if err != nil {
			return nil, img.Path
		}
		return data, img.Path
	default:
		return nil, ""
	}
}

// flattenVisualFindings collects every per-slide finding from a report into a
// flat list in report order. Returns an empty (non-nil) slice for a clean deck.
func flattenVisualFindings(report *visualqa.Report) []visualqa.Finding {
	out := []visualqa.Finding{}
	if report == nil {
		return out
	}
	for _, sr := range report.Results {
		out = append(out, sr.Findings...)
	}
	return out
}

// actionableVisualFindings keeps only P0/P1 findings — the same severity
// threshold the visual-repair loop uses to trigger autofix. P2/P3 (cosmetic /
// nitpick) and heuristic-only P3 findings are advisory and never drive
// automatic repairs, which keeps the loop from churning on subjective polish.
func actionableVisualFindings(findings []visualqa.Finding) []visualqa.Finding {
	out := make([]visualqa.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Severity == visualqa.SeverityP0 || f.Severity == visualqa.SeverityP1 {
			out = append(out, f)
		}
	}
	return out
}

// visualFindingsToProposeFindings adapts visualqa.Findings into the polymorphic
// shape proposeRepairs accepts (the visual-QA branch keys on Category).
func visualFindingsToProposeFindings(findings []visualqa.Finding) []proposeRepairsFinding {
	out := make([]proposeRepairsFinding, 0, len(findings))
	for _, f := range findings {
		idx := f.SlideIndex
		out = append(out, proposeRepairsFinding{
			SlideIndex:     &idx,
			SlideType:      f.SlideType,
			Severity:       string(f.Severity),
			Category:       f.Category,
			Description:    f.Description,
			Location:       f.Location,
			SuggestedFixes: f.SuggestedFixes,
		})
	}
	return out
}

// flattenProposedRepairs flattens propose_repairs output into the trace shape.
func flattenProposedRepairs(proposed proposeRepairsOutput) []visualQAProposedRepair {
	out := []visualQAProposedRepair{}
	for _, slide := range proposed.Slides {
		for _, dir := range slide.Directives {
			out = append(out, visualQAProposedRepair{
				SlideIndex: slide.SlideIndex,
				Kind:       dir.Kind,
				Category:   dir.Source.Category,
			})
		}
	}
	return out
}

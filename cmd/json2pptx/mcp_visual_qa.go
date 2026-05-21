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
	// InspectionMode reports which backend actually ran: "vision" (Claude vision
	// API), "heuristic" (pure-Go fallback, no API key), or "skipped" (render
	// tools unavailable — no inspection happened at all).
	InspectionMode string                `json:"inspection_mode"`
	Model          string                `json:"model,omitempty"`
	Requirements   visualQARequirements  `json:"requirements"`
	Passes         []visualQAPassEntry   `json:"passes"`
	PaletteAudit   *visualQAPaletteAudit `json:"palette_audit,omitempty"`
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
	Pass            int                      `json:"pass"`
	InspectionMode  string                   `json:"inspection_mode"`
	ThumbnailPaths  []string                 `json:"thumbnail_paths"`
	VisualFindings  []visualqa.Finding       `json:"visual_findings"`
	ProposedRepairs []visualQAProposedRepair `json:"proposed_repairs"`
	RepairsApplied  []string                 `json:"repairs_applied"`
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
// maps visual findings to repairs, applies them, and re-renders. It mutates
// input in place (so the caller's final_presentation reflects visual repairs)
// and rewrites outputPath so the on-disk PPTX reflects the repaired deck.
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
		Requested:    true,
		Requirements: buildVisualQARequirements(cfg),
		Passes:       []visualQAPassEntry{},
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

		entry := visualQAPassEntry{
			Pass:            pass,
			InspectionMode:  passMode,
			ThumbnailPaths:  paths,
			VisualFindings:  flattenVisualFindings(report),
			ProposedRepairs: []visualQAProposedRepair{},
			RepairsApplied:  []string{},
		}

		actionable := actionableVisualFindings(entry.VisualFindings)
		if len(actionable) == 0 {
			result.Passes = append(result.Passes, entry)
			break
		}

		proposed := proposeRepairs(input, visualFindingsToProposeFindings(actionable))
		entry.ProposedRepairs = flattenProposedRepairs(proposed)
		entry.RepairsApplied = applyProposedRepairs(input, proposed)
		result.Passes = append(result.Passes, entry)

		if len(entry.RepairsApplied) == 0 {
			break
		}

		// Re-render so the on-disk PPTX (and the next pass's thumbnails) reflect
		// the repairs just applied. A re-render failure stops the loop but keeps
		// the repairs in input — the caller still marshals the repaired JSON.
		if _, rerr := mc.renderAutoRepairFinal(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette, outputPath); rerr != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("visual_qa pass %d: re-render after repair failed: %v", pass, rerr))
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

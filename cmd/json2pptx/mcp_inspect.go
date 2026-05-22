// mcp_inspect.go implements the inspect_slide_images MCP tool — a first-class
// entry point to the visualqa Claude vision QA agent. Agents pass rendered
// slide images (path or base64 PNG) and optional per-slide metadata; the tool
// returns a structured visualqa.Report whose findings include suggested_fixes
// mapped to repair_slide fix kinds (via visualqa.SuggestedFixesForCategory),
// so the agent can pipe findings → repair_slide without an extra round-trip.
//
// This is the canonical surface for the visual refinement loop. score_deck
// rejects mode="with_heuristics" with UNSUPPORTED_MODE pointing here, so any
// vision-based QA flows through inspect_slide_images on rendered thumbnails.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/visualqa"
	"github.com/sebahrens/json2pptx/internal/visualqa/heuristic"
)

// --- Input types ---

// inspectSlideImageInput is one slide-image entry on the call.
type inspectSlideImageInput struct {
	Index     int    `json:"index"`
	Path      string `json:"path,omitempty"`
	PNGBase64 string `json:"png_base64,omitempty"`
	SlideType string `json:"slide_type,omitempty"`
	Title     string `json:"title,omitempty"`
}

// inspectSlideInfo is optional per-slide metadata supplied alongside images,
// indexed by SlideIndex. Used when callers prefer to pass slide_info as a
// separate list from the image data (e.g. when images come from disk).
type inspectSlideInfo struct {
	Index     int    `json:"index"`
	SlideType string `json:"slide_type,omitempty"`
	Title     string `json:"title,omitempty"`
}

// --- Tool definition ---

func mcpInspectSlideImagesTool() mcp.Tool {
	return mcp.NewTool("inspect_slide_images",
		mcp.WithDescription(`Run vision-based visual QA on rendered slide images. Returns a structured visualqa.Report with per-slide findings: {severity (P0-P3), category, description, location, suggested_fixes}. suggested_fixes are pre-mapped to repair_slide fix kinds (via SuggestedFixesForCategory), so agents can pipe findings directly into repair_slide with {kind: "autofix_visual", params: {category: "<finding.category>"}}.

This is the canonical entry point for the visual refinement loop:
  generate_presentation → render_deck_thumbnails → inspect_slide_images → repair_slide.

Each slide_images[] entry must include "index" (0-based) and one of "path" (absolute filesystem path to a .png/.jpg) or "png_base64" (raw base64-encoded image bytes, no data: URL prefix). Optional per-slide "slide_type" (title/content/section/chart/diagram/...) and "title" tune the prompt for that slide.

When ANTHROPIC_API_KEY is set, vision-backed checks run via Claude (Report.mode="vision"). When unset, the tool falls back to a deterministic heuristic pass — pure-Go image checks for blank slides, edge-band overflow, and aspect ratio (Report.mode="heuristic", findings tagged source="heuristic", severity P3). Heuristic findings are advisory and may have higher false-positive rates than vision-backed checks.

Image source policy: paths must be absolute and end in .png/.jpg/.jpeg. Path traversal (..) is rejected. For images already in memory (e.g. just-rendered thumbnails), prefer png_base64 to avoid disk round-trips.`),
		mcp.WithRawOutputSchema(outputSchemaInspectSlideImages),
		mcp.WithArray("slide_images",
			mcp.Required(),
			mcp.Description(`Slide images to inspect. Each entry: {index: int, path?: string, png_base64?: string, slide_type?: string, title?: string}. Exactly one of path or png_base64 must be set per entry.`),
		),
		mcp.WithArray("slide_info",
			mcp.Description(`Optional per-slide metadata, indexed by "index". Use to supply slide_type/title without re-encoding the slide_images array. Entries here override slide_images[].slide_type / .title when both are present.`),
		),
		mcp.WithObject("deck_metadata",
			mcp.Description(`Optional deck-level metadata. Recognized keys: template (string). Echoed back on the report.template field.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string"},
			}),
		),
		mcp.WithString("model",
			mcp.Description(`Optional Claude model override. Default: claude-haiku-4-5-20251001. Use a stronger model when the default produces too many false positives.`),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleInspectSlideImages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	images, err := extractSlideImages(request)
	if err != nil {
		return argInvalidValue("inspect_slide_images", "INVALID_PARAMETER", "slide_images", err.Error(), "array", []any{map[string]any{"index": 0, "path": "/tmp/slide-0.jpg"}}, nil), nil
	}
	if len(images) == 0 {
		return argMissing("inspect_slide_images", "slide_images", "array", []any{map[string]any{"index": 0, "path": "/tmp/slide-0.jpg"}}, nil), nil
	}

	// Optional slide_info overrides.
	infoByIdx, err := extractSlideInfoOverrides(request)
	if err != nil {
		return argInvalidValue("inspect_slide_images", "INVALID_PARAMETER", "slide_info", err.Error(), "array", []any{}, nil), nil
	}

	// Optional deck metadata (template echo).
	template := ""
	if obj, ok := request.GetArguments()["deck_metadata"].(map[string]any); ok {
		if t, ok := obj["template"].(string); ok {
			template = t
		}
	}

	// Load image bytes + assemble visualqa.SlideImage list. pathByIndex carries
	// each entry's source path (when supplied) so a per-slide inspection failure
	// can name the offending image in its diagnostic.
	slideImages := make([]visualqa.SlideImage, 0, len(images))
	pathByIndex := make(map[int]string, len(images))
	for _, entry := range images {
		data, err := loadInspectImage(entry)
		if err != nil {
			return api.MCPSimpleError("INVALID_IMAGE", fmt.Sprintf("slide_images[%d]: %v", entry.Index, err)), nil
		}
		if entry.Path != "" {
			pathByIndex[entry.Index] = entry.Path
		}
		info := visualqa.SlideInfo{
			Index: entry.Index,
			Type:  entry.SlideType,
			Title: entry.Title,
		}
		if override, ok := infoByIdx[entry.Index]; ok {
			if override.SlideType != "" {
				info.Type = override.SlideType
			}
			if override.Title != "" {
				info.Title = override.Title
			}
		}
		if info.Type == "" {
			info.Type = "content"
		}
		slideImages = append(slideImages, visualqa.SlideImage{Info: info, Data: data})
	}

	// Build agent. If construction fails (e.g. ANTHROPIC_API_KEY unset),
	// fall back to the deterministic heuristic checker so callers still
	// get an actionable — if coarser — visual QA pass instead of an
	// INSPECT_DISABLED error.
	var opts []visualqa.Option
	if m, ok := request.GetArguments()["model"].(string); ok && m != "" {
		opts = append(opts, visualqa.WithModel(m))
	}

	var report *visualqa.Report
	if agent, err := visualqa.NewAgent(opts...); err == nil {
		report = agent.InspectAll(ctx, slideImages)
		report.Mode = "vision"
		for ri := range report.Results {
			for fi := range report.Results[ri].Findings {
				if report.Results[ri].Findings[fi].Source == "" {
					report.Results[ri].Findings[fi].Source = "vision"
				}
			}
		}
	} else {
		report = heuristic.InspectAll(slideImages)
	}
	report.Template = template

	failedSlides, inspectionStatus := inspectionFailureStats(report)
	output := inspectOutput{
		Report:           report,
		FailedSlideCount: failedSlides,
		InspectionStatus: inspectionStatus,
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand: "inspect_slide_images",
			Template:   report.Template,
		}, diagnosticsFromVisualQAReport(report, pathByIndex)),
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Finding envelope projection ---

// inspectOutput is the inspect_slide_images response. It promotes the full
// visualqa.Report (mode, results[], and the p0..p3 rollups agents branch on) to
// the top level and adds a FindingEnvelope projection of every per-slide finding
// under "findings", so an agent can branch on findings.ok without losing the
// visual-QA detail. The envelope is always present; findings.findings is empty
// when the report is clean. See docs/AGENT_DIAGNOSTICS.md.
//
// FailedSlideCount and InspectionStatus disambiguate a clean inspection (zero
// findings because every slide was inspected and passed) from a backend failure
// (zero findings because the inspection itself failed): a backend/transport/
// decode error on a slide projects to an error-severity finding (so findings.ok
// is false), and these two fields let an agent tell the cases apart even in
// mode="vision" where SlideResult.Error is set but no visual defects returned.
type inspectOutput struct {
	*visualqa.Report
	// FailedSlideCount is the number of slides whose inspection failed
	// (SlideResult.Error set) rather than completing.
	FailedSlideCount int `json:"failed_slide_count"`
	// InspectionStatus is "complete" (no slide errors), "partial" (some slides
	// failed), or "failed" (every slide failed). When not "complete", an empty
	// findings list does NOT indicate a clean deck.
	InspectionStatus string                      `json:"inspection_status"`
	Findings         diagnostics.FindingEnvelope `json:"findings"`
}

// Inspection status values for inspectOutput.InspectionStatus and the per-pass
// visual_qa entries.
const (
	inspectionStatusComplete = "complete" // every slide inspected without error
	inspectionStatusPartial  = "partial"  // some slides failed inspection
	inspectionStatusFailed   = "failed"   // every slide failed inspection
)

// inspectionFailureStats counts the slides whose inspection failed (their
// SlideResult.Error is set) and classifies the run as complete/partial/failed.
// A nil or slide-less report is reported complete with zero failures.
func inspectionFailureStats(report *visualqa.Report) (failed int, status string) {
	if report == nil || len(report.Results) == 0 {
		return 0, inspectionStatusComplete
	}
	for _, sr := range report.Results {
		if sr.Error != "" {
			failed++
		}
	}
	switch {
	case failed == 0:
		return 0, inspectionStatusComplete
	case failed == len(report.Results):
		return failed, inspectionStatusFailed
	default:
		return failed, inspectionStatusPartial
	}
}

// visualCategoryNamespace maps a visualqa finding category onto a finding-
// envelope namespace. Content-overflow categories project to FIT (matching the
// fit-report findings an agent already handles); every other visual defect
// projects to RENDER. Categories absent from this map fall through to RENDER.
var visualCategoryNamespace = map[string]diagnostics.Namespace{
	"text_overflow":   diagnostics.NamespaceFit,
	"text_truncation": diagnostics.NamespaceFit,
}

// diagnosticsFromVisualQAReport flattens every per-slide visualqa finding into a
// transport-neutral diagnostic, in report order (by slide, then finding), so
// BuildEnvelope can project them into the shared FindingEnvelope shape. A slide
// whose inspection FAILED (SlideResult.Error set — an API/transport/decode error
// in vision mode, or an undecodable image in heuristic mode) is projected as an
// error-severity diagnostic so findings.ok reports false: a backend failure can
// never masquerade as a clean inspection. pathByIndex supplies the source image
// path per slide index when one is known, so the failure diagnostic can name it.
func diagnosticsFromVisualQAReport(report *visualqa.Report, pathByIndex map[int]string) []diagnostics.Diagnostic {
	if report == nil {
		return nil
	}
	var ds []diagnostics.Diagnostic
	for _, sr := range report.Results {
		if sr.Error != "" {
			ds = append(ds, diagnosticFromVisualQASlideError(sr, report.Mode, pathByIndex[sr.SlideIndex]))
		}
		for _, f := range sr.Findings {
			ds = append(ds, diagnosticFromVisualQAFinding(f))
		}
	}
	return ds
}

// diagnosticFromVisualQASlideError adapts a failed per-slide inspection
// (SlideResult.Error) into an error-severity Diagnostic. It picks the code from
// the failure mode — VISION_TIMEOUT for a vision deadline, otherwise
// VISION_INSPECTION_FAILED for a vision backend/transport/decode failure or
// HEURISTIC_INSPECTION_FAILED for a heuristic decode failure — keeping the
// vision/heuristic source distinction so a degraded heuristic run is never
// mistaken for a vision-backed defect. The slide index drives where.slide and
// the image path (when known) rides in evidence.
func diagnosticFromVisualQASlideError(sr visualqa.SlideResult, mode, path string) diagnostics.Diagnostic {
	source := mode
	if source == "" {
		source = "vision"
	}
	code := diagnostics.CodeVisionInspectionFailed
	switch {
	case strings.HasPrefix(sr.Error, visualqa.VisionTimeoutCode):
		code = diagnostics.CodeVisionTimeout
	case mode == "heuristic":
		code = diagnostics.CodeHeuristicInspectionFailed
	}
	d := diagnostics.Diagnostic{
		Code:     code,
		Message:  sr.Error,
		Path:     fmt.Sprintf("slides[%d]", sr.SlideIndex),
		Severity: diagnostics.SeverityError,
		Details: map[string]any{
			"inspection_failed": true,
			"source":            source,
		},
	}
	if sr.SlideType != "" {
		d.Details["slide_type"] = sr.SlideType
	}
	if path != "" {
		d.Details["image_path"] = path
	}
	return d
}

// diagnosticFromVisualQAFinding adapts one visualqa.Finding into a Diagnostic:
// it severity-maps the P0..P3 level, namespaces the visual category (FIT for
// overflow, RENDER otherwise), carries the slide location (so the envelope's
// where.slide is populated), preserves the precise P-level/category/location/
// source in evidence, and lifts the first suggested repair_slide fix into a
// remediation.
func diagnosticFromVisualQAFinding(f visualqa.Finding) diagnostics.Diagnostic {
	ns := diagnostics.NamespaceRender
	if mapped, ok := visualCategoryNamespace[f.Category]; ok {
		ns = mapped
	}
	d := diagnostics.Diagnostic{
		Code:     diagnostics.DottedCode(ns, f.Category),
		Message:  f.Description,
		Path:     fmt.Sprintf("slides[%d]", f.SlideIndex),
		Severity: visualSeverityToDiagnosticSeverity(f.Severity),
		Details: map[string]any{
			"visual_severity": string(f.Severity),
			"visual_category": f.Category,
		},
	}
	if f.Location != "" {
		d.Details["location"] = f.Location
	}
	if f.Source != "" {
		d.Details["source"] = f.Source
	}
	if len(f.SuggestedFixes) > 0 {
		d.Fix = &diagnostics.Fix{
			Kind:   f.SuggestedFixes[0].Kind,
			Params: f.SuggestedFixes[0].Params,
		}
	}
	return d
}

// visualSeverityToDiagnosticSeverity projects a visualqa P-severity onto the
// three-level diagnostic vocabulary. P0 (catastrophic) and P1 (major) become
// errors so findings.ok reports false on a deck that still needs repair — the
// same P0/P1 threshold the visual-repair loop uses to trigger autofix; P2
// (minor/cosmetic) is a warning; P3 (nitpick) and any unknown value are
// advisory info. The precise P-level is preserved in evidence.visual_severity
// and the report's p0..p3 rollups.
func visualSeverityToDiagnosticSeverity(s visualqa.Severity) diagnostics.Severity {
	switch s {
	case visualqa.SeverityP0, visualqa.SeverityP1:
		return diagnostics.SeverityError
	case visualqa.SeverityP2:
		return diagnostics.SeverityWarning
	default:
		return diagnostics.SeverityInfo
	}
}

// --- Input parsing ---

// extractSlideImages reads the slide_images array, re-marshaling through JSON
// so MCP's untyped maps decode into typed inspectSlideImageInput entries.
func extractSlideImages(request mcp.CallToolRequest) ([]inspectSlideImageInput, error) {
	args := request.GetArguments()
	raw, ok := args["slide_images"]
	if !ok {
		return nil, fmt.Errorf("slide_images is required")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("slide_images: %w", err)
	}
	var out []inspectSlideImageInput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("slide_images must be an array of {index, path?, png_base64?, slide_type?, title?} objects: %w", err)
	}
	return out, nil
}

// extractSlideInfoOverrides reads the optional slide_info array.
func extractSlideInfoOverrides(request mcp.CallToolRequest) (map[int]inspectSlideInfo, error) {
	args := request.GetArguments()
	raw, ok := args["slide_info"]
	if !ok || raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("slide_info: %w", err)
	}
	var list []inspectSlideInfo
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("slide_info must be an array of {index, slide_type?, title?} objects: %w", err)
	}
	out := make(map[int]inspectSlideInfo, len(list))
	for _, e := range list {
		out[e.Index] = e
	}
	return out, nil
}

// loadInspectImage resolves an inspectSlideImageInput entry to raw image bytes,
// validating the source. Exactly one of Path or PNGBase64 must be set.
func loadInspectImage(entry inspectSlideImageInput) ([]byte, error) {
	hasPath := entry.Path != ""
	hasB64 := entry.PNGBase64 != ""
	switch {
	case hasPath && hasB64:
		return nil, fmt.Errorf("set exactly one of path or png_base64, not both")
	case !hasPath && !hasB64:
		return nil, fmt.Errorf("set exactly one of path or png_base64")
	case hasB64:
		// Tolerate data: URL prefix even though we document its absence.
		s := entry.PNGBase64
		if i := strings.Index(s, ","); i >= 0 && strings.HasPrefix(s, "data:") {
			s = s[i+1:]
		}
		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("png_base64 is not valid base64: %w", err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("png_base64 decoded to zero bytes")
		}
		return data, nil
	default: // hasPath
		if err := validateInspectImagePath(entry.Path); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("read image: %w", err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("image file is empty")
		}
		return data, nil
	}
}

// validateInspectImagePath enforces an absolute path with an image extension
// and no traversal segments. Mirrors api.ValidatePptxPath's policy.
func validateInspectImagePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return fmt.Errorf("path contains traversal segment")
		}
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("path must be absolute (got %q)", path)
	}
	lower := strings.ToLower(clean)
	if !(strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")) {
		return fmt.Errorf("path must end in .png/.jpg/.jpeg")
	}
	return nil
}

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
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}
	if len(images) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "slide_images must contain at least one entry"), nil
	}

	// Optional slide_info overrides.
	infoByIdx, err := extractSlideInfoOverrides(request)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	// Optional deck metadata (template echo).
	template := ""
	if obj, ok := request.GetArguments()["deck_metadata"].(map[string]any); ok {
		if t, ok := obj["template"].(string); ok {
			template = t
		}
	}

	// Load image bytes + assemble visualqa.SlideImage list.
	slideImages := make([]visualqa.SlideImage, 0, len(images))
	for _, entry := range images {
		data, err := loadInspectImage(entry)
		if err != nil {
			return api.MCPSimpleError("INVALID_IMAGE", fmt.Sprintf("slide_images[%d]: %v", entry.Index, err)), nil
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

	mcpResult, err := api.MCPSuccessResult(ctx, report)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
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

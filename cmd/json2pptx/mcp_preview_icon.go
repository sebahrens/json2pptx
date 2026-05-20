// mcp_preview_icon.go implements the preview_icon MCP tool: a lightweight,
// deckless rendering of a single IconInput so agents can verify a bundled
// name, custom .svg path, remote URL, or inline SVG markup before committing
// it to a slide. Without it, an agent's only feedback path was a full
// generate + re-render cycle.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

const (
	previewIconDefaultSize = 128
	previewIconMinSize     = 16
	previewIconMaxSize     = 1024
)

// previewIconResponse is the JSON shape returned by preview_icon. Fields are
// kept deliberately small (no embedded base64 unless rasterization succeeded)
// so agents can preview many icons cheaply.
type previewIconResponse struct {
	SVGData       string   `json:"svg_data"`
	PNGBase64     string   `json:"png_base64,omitempty"`
	Alt           string   `json:"alt"`
	SourceKind    string   `json:"source_kind"`
	QualifiedName string   `json:"qualified_name,omitempty"`
	Width         int      `json:"width,omitempty"`
	Height        int      `json:"height,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

func mcpPreviewIconTool() mcp.Tool {
	return mcp.NewTool("preview_icon",
		mcp.WithDescription(`Render a single icon spec (bundled name, custom .svg path, remote URL, or inline SVG markup) to SVG bytes + a PNG preview. Use to verify an IconInput before committing it to a deck — no template load, no full generation cycle.

Exactly one of name|path|url|svg_data must be set. Optional fill (hex like "#FF0000") recolors bundled and path/URL-based SVGs; it is ignored for inline svg_data (the agent supplies pre-styled markup).

Response fields:
- svg_data: the SVG markup (Fill applied when present)
- png_base64: base64-encoded PNG preview (best-effort; absent if rasterization fails)
- alt: alt text — uses IconInput.alt when set, otherwise derived from name/path
- source_kind: one of "bundled", "path", "url", "inline"
- qualified_name: canonical "<set>:<name>" form for bundled icons (use directly as icon.name in deck JSON)
- width / height: PNG dimensions in pixels

Failure codes:
- INVALID_PARAMETER: icon has zero or multiple sources, or base_dir is not absolute
- ICON_BUNDLED_NAME_UNKNOWN: bundled name not in the embedded registry (suggestions returned)
- URL_FETCH_FAILED: URL download failed or content is not SVG
- ICON_PATH_RESOLUTION: path resolution failed (missing, traversal, wrong extension)`),
		mcp.WithRawOutputSchema(outputSchemaPreviewIcon),
		mcp.WithObject("icon",
			mcp.Required(),
			mcp.Description(`IconInput shape. Exactly one of: name (bundled, e.g. "filled:chart-pie"), path (.svg on disk; relative paths resolve against base_dir), url (HTTPS .svg), svg_data (inline markup). Optional: fill (hex color override; ignored for inline svg_data), alt (accessibility text).`),
			mcp.Properties(map[string]any{
				"name":     map[string]any{"type": "string", "description": "Bundled icon name (e.g. 'chart-pie' or 'filled:chart-pie'). Discover via list_icons."},
				"path":     map[string]any{"type": "string", "description": "Filesystem path to a .svg file. Relative paths resolve against base_dir."},
				"url":      map[string]any{"type": "string", "description": "HTTP/HTTPS URL to a remote .svg. Downloaded once and validated as SVG."},
				"svg_data": map[string]any{"type": "string", "description": "Inline SVG markup. No I/O; fill is ignored."},
				"alt":      map[string]any{"type": "string", "description": "Optional alt text. Falls back to name or path when empty."},
				"fill":     map[string]any{"type": "string", "description": "Optional hex color override (e.g. '#FF0000'). Applied to bundled and path/URL-based SVGs."},
			}),
		),
		mcp.WithNumber("size_px",
			mcp.Description("Target pixel size for the longer side of the PNG preview. Default 128. Clamped to [16, 1024]."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used to resolve a relative icon.path. Must be an absolute path to an existing directory. Ignored for name/url/svg_data sources."),
		),
	)
}

// handlePreviewIcon resolves the icon spec into SVG bytes, rasterizes a PNG
// preview, and returns the combined response. URL fetching uses an SSRF-safe
// resolver derived from mcpConfig.resolverOpts (tests inject an HTTP client).
func (mc *mcpConfig) handlePreviewIcon(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	iconRaw, ok := args["icon"]
	if !ok || iconRaw == nil {
		return api.MCPSimpleError(diagnostics.CodeMissingParameter, "icon is required"), nil
	}

	icon, parseErr := parsePreviewIconInput(iconRaw)
	if parseErr != nil {
		return api.MCPSimpleError(diagnostics.CodeInvalidParameter, fmt.Sprintf("icon: %v", parseErr)), nil
	}

	sourceKind, srcErr := previewIconSourceKind(icon)
	if srcErr != nil {
		return api.MCPDiagnosticsError([]diagnostics.Diagnostic{*srcErr}), nil
	}

	sizePx := previewIconDefaultSize
	if v, ok := args["size_px"].(float64); ok {
		sizePx = clampInt(int(v), previewIconMinSize, previewIconMaxSize)
	}

	// base_dir only matters for relative paths; resolveBaseDir returns a
	// usable absolute dir (or an error envelope) for every path-based call.
	var baseDir string
	if sourceKind == "path" {
		var errResult *mcp.CallToolResult
		baseDir, errResult = resolveBaseDir(request)
		if errResult != nil {
			return errResult, nil
		}
	}

	svgBytes, warnings, resolveResult := mc.resolvePreviewIcon(icon, sourceKind, baseDir)
	if resolveResult != nil {
		return resolveResult, nil
	}

	resp := previewIconResponse{
		SVGData:    string(svgBytes),
		Alt:        previewIconAlt(icon, sourceKind),
		SourceKind: sourceKind,
		Warnings:   warnings,
	}
	if sourceKind == "bundled" {
		resp.QualifiedName = qualifiedIconName(icon.Name)
	}

	if pngB64, w, h, ok := rasterizePreviewIcon(svgBytes, sizePx); ok {
		resp.PNGBase64 = pngB64
		resp.Width = w
		resp.Height = h
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeInternal, fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// parsePreviewIconInput unmarshals the raw `icon` argument into an IconInput.
// The MCP SDK delivers the value as map[string]any, so we round-trip through
// JSON to reuse the existing struct tags.
func parsePreviewIconInput(raw any) (*IconInput, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var icon IconInput
	if err := json.Unmarshal(data, &icon); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &icon, nil
}

// previewIconSourceKind returns the canonical source kind for the icon and a
// diagnostic when the spec is ambiguous (multiple sources) or empty.
func previewIconSourceKind(icon *IconInput) (string, *diagnostics.Diagnostic) {
	var set []string
	if icon.Name != "" {
		set = append(set, "name")
	}
	if icon.Path != "" {
		set = append(set, "path")
	}
	if icon.URL != "" {
		set = append(set, "url")
	}
	if icon.SVGData != "" {
		set = append(set, "svg_data")
	}
	if len(set) == 0 {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconMissing,
			Path:     "icon",
			Message:  "icon must have one of 'name', 'path', 'url', or 'svg_data'",
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"remediation": "set exactly one of name (bundled), path (.svg file), url (remote .svg), or svg_data (inline markup)",
			},
		}
	}
	if len(set) > 1 {
		return "", &diagnostics.Diagnostic{
			Code:     diagnostics.CodeIconAmbiguous,
			Path:     "icon",
			Message:  fmt.Sprintf("icon has conflicting sources %v; exactly one is allowed", set),
			Severity: diagnostics.SeverityError,
			Details: map[string]any{
				"conflicting_fields": set,
				"remediation":        "keep one of " + strings.Join(set, ", ") + " and remove the others",
			},
		}
	}
	switch set[0] {
	case "name":
		return "bundled", nil
	case "path":
		return "path", nil
	case "url":
		return "url", nil
	case "svg_data":
		return "inline", nil
	}
	return "", nil
}

// resolvePreviewIcon dispatches by source kind and returns the SVG bytes
// (with Fill applied for non-inline sources) along with any advisory
// warnings. On failure it returns an MCP error result so the caller can
// short-circuit.
func (mc *mcpConfig) resolvePreviewIcon(icon *IconInput, sourceKind, baseDir string) ([]byte, []string, *mcp.CallToolResult) {
	var warnings []string

	switch sourceKind {
	case "inline":
		// Inline SVG: bytes pass through. Fill is ignored — surface as warning
		// so the agent realizes the override was dropped.
		if icon.Fill != "" {
			warnings = append(warnings, fmt.Sprintf("icon.fill %q ignored when svg_data is set; pre-style the inline SVG or switch to name/path/url", icon.Fill))
		}
		return []byte(icon.SVGData), warnings, nil

	case "bundled":
		if !icons.Exists(icon.Name) {
			return nil, nil, api.MCPDiagnosticsError([]diagnostics.Diagnostic{
				buildBundledIconNameFinding(icon.Name, 0, "icon/name"),
			})
		}
		data, err := icons.Lookup(icon.Name)
		if err != nil {
			return nil, nil, api.MCPSimpleError(diagnostics.CodeIconNotFound, fmt.Sprintf("icon %q: %v", icon.Name, err))
		}
		if icon.Fill != "" {
			data = applyIconFill(data, icon.Fill)
		}
		return data, warnings, nil

	case "path":
		// Reuse the production path resolver so all of its security checks
		// (traversal, symlink eval, extension, env expansion) apply here.
		// resolveIconInputPath rewrites icon.Path to an absolute resolved
		// form on success.
		diags := resolveIconInputPath(icon, baseDir, 0, "icon")
		if hasErrorDiagnostic(diags) {
			return nil, nil, api.MCPDiagnosticsError(diags)
		}
		data, err := os.ReadFile(icon.Path)
		if err != nil {
			return nil, nil, api.MCPSimpleError(diagnostics.CodeIconPath, fmt.Sprintf("read icon %q: %v", icon.Path, err))
		}
		if icon.Fill != "" {
			data = applyIconFill(data, icon.Fill)
		}
		// Collect non-error diagnostics (e.g. fill-on-inline warnings, which
		// shouldn't fire here but the same helper may grow new warnings).
		for _, d := range diags {
			if d.Severity == diagnostics.SeverityWarning {
				warnings = append(warnings, d.Message)
			}
		}
		return data, warnings, nil

	case "url":
		resolver, err := resource.NewResolver(mc.resolverOpts)
		if err != nil {
			return nil, nil, api.MCPSimpleError(diagnostics.CodeURLResolverInit, fmt.Sprintf("resource resolver: %v", err))
		}
		defer resolver.Close()
		path, err := resolver.ResolveSVG(icon.URL)
		if err != nil {
			return nil, nil, api.MCPDiagnosticsError([]diagnostics.Diagnostic{
				urlFetchDiagnostic("icon/url", "icon", "svg", icon.URL, 0, err),
			})
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, api.MCPSimpleError(diagnostics.CodeURLFetchFailed, fmt.Sprintf("read cached icon: %v", err))
		}
		if icon.Fill != "" {
			data = applyIconFill(data, icon.Fill)
		}
		return data, warnings, nil
	}

	return nil, nil, api.MCPSimpleError(diagnostics.CodeInternal, fmt.Sprintf("unknown icon source kind %q", sourceKind))
}

// rasterizePreviewIcon renders SVG bytes to a PNG and returns the base64
// payload plus dimensions. Returns ok=false when canvas can't parse the SVG
// (the response still carries svg_data, so the failure is non-fatal).
func rasterizePreviewIcon(svgBytes []byte, sizePx int) (b64 string, width, height int, ok bool) {
	img := svggen.LoadIcon(string(svgBytes), sizePx)
	if img == nil {
		return "", 0, 0, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", 0, 0, false
	}
	bounds := img.Bounds()
	return base64.StdEncoding.EncodeToString(buf.Bytes()), bounds.Dx(), bounds.Dy(), true
}

// previewIconAlt mirrors the deck-time alt fallback: explicit alt > derived
// from the source-specific identifier > literal "icon".
func previewIconAlt(icon *IconInput, sourceKind string) string {
	if icon.Alt != "" {
		return icon.Alt
	}
	switch sourceKind {
	case "bundled":
		return icon.Name
	case "path":
		return icon.Path
	case "url":
		return icon.URL
	}
	return "icon"
}

// qualifiedIconName returns the canonical "<set>:<name>" form for a bundled
// icon, mirroring list_icons' qualified_name semantics.
func qualifiedIconName(name string) string {
	name = strings.TrimSpace(name)
	if strings.IndexByte(name, ':') >= 0 {
		return name
	}
	return icons.DefaultSet + ":" + name
}

// hasErrorDiagnostic returns true if any diagnostic in the slice has error
// severity (i.e. must abort the request).
func hasErrorDiagnostic(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

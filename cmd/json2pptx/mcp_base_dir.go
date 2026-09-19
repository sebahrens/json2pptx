// mcp_base_dir.go centralizes resolution of the optional `base_dir` parameter
// across MCP tool handlers that resolve local-asset paths (icons, images,
// background images, shape_grid image/icon cells).
//
// Without an explicit base_dir, the server has no portable reference frame for
// relative paths in JSON: the same JSON works or fails depending on how the
// MCP server was launched. Agents reasonably send relative paths and expect
// them to resolve against the same directory the agent considers "current"
// (the directory the JSON lives in, an authored workspace, etc.). Letting the
// server's process CWD decide silently is "poison for 1-call -> publishable
// deck."
//
// Contract:
//   - When `base_dir` is supplied, it MUST be an absolute path to an existing
//     directory. Symlinks are evaluated. A relative `base_dir` or a missing /
//     non-directory path returns a structured INVALID_PARAMETER diagnostic.
//   - When `base_dir` is absent, the server falls back to its process CWD
//     (preserving historical behaviour for callers that haven't yet adopted
//     base_dir). Capability advertisement signals the new parameter so agents
//     know to start sending it.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// resolveBaseDir returns the directory MCP asset-path resolution should treat
// as the root for relative paths. The lookup order is:
//
//  1. `base_dir` argument on the request (must be absolute, existing dir).
//  2. Process CWD (legacy fallback).
//
// On a malformed `base_dir`, returns ("", errResult). The caller MUST return
// errResult unchanged. On success returns the cleaned, symlink-evaluated
// absolute path. The empty string is only returned alongside a non-nil
// errResult; callers should not treat ("", nil) as a valid state.
func resolveBaseDir(request mcp.CallToolRequest) (string, *mcp.CallToolResult) {
	dir, d := resolveBaseDirDiag(request)
	if d != nil {
		return "", api.MCPDiagnosticsError([]diagnostics.Diagnostic{*d})
	}
	return dir, nil
}

// resolveBaseDirDiag is resolveBaseDir's diagnostic form, for callers that fold
// the failure into an envelope of their own rather than returning it as the
// whole response (go-slide-creator-ydbk: validate_input reports a bad base_dir
// alongside the deck's other boundary diagnostics).
func resolveBaseDirDiag(request mcp.CallToolRequest) (string, *diagnostics.Diagnostic) {
	args := request.GetArguments()
	raw, _ := args["base_dir"].(string)

	bad := func(msg, remediation string, withInput bool) (string, *diagnostics.Diagnostic) {
		details := map[string]any{"remediation": remediation}
		if withInput {
			details["input_value"] = raw
		}
		return "", &diagnostics.Diagnostic{
			Code:         diagnostics.CodeInvalidParameter,
			Path:         "base_dir",
			Message:      msg,
			Severity:     diagnostics.SeverityError,
			ExpectedType: "string",
			ExampleValue: "/Users/you/decks",
			Details:      details,
		}
	}

	if raw == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return bad(fmt.Sprintf("base_dir not supplied and process CWD unavailable: %v", err),
				"pass an absolute directory as base_dir on this tool call", false)
		}
		return cwd, nil
	}

	if !filepath.IsAbs(raw) {
		return bad(fmt.Sprintf("base_dir must be an absolute path, got %q", raw),
			"pass an absolute path (e.g. /Users/you/decks) so the server can resolve relative asset references portably across CWDs", true)
	}

	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return bad(fmt.Sprintf("base_dir %q: %v", raw, err),
			"verify base_dir exists and is reachable from the server process", true)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return bad(fmt.Sprintf("base_dir %q: %v", raw, err),
			"verify base_dir exists and is reachable from the server process", true)
	}
	if !info.IsDir() {
		return bad(fmt.Sprintf("base_dir %q is not a directory", raw),
			"pass the path to an existing directory, not a file", true)
	}

	return filepath.Clean(resolved), nil
}

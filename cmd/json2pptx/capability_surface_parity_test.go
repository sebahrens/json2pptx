package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/svggen"
)

// These tests are the cross-surface drift gates required by
// go-slide-creator-4wxp: they keep the CLI runtime block, the chart/diagram CLI
// counterpart claim, the HTTP capability list, and the README HTTP table aligned
// with the canonical MCP catalog instead of letting each surface drift on its
// own.

// parseCapabilities decodes a buildCapabilitiesResult into the typed response,
// failing the test on any transport- or tool-level error.
func parseCapabilities(t *testing.T, result *mcp.CallToolResult, err error) capabilitiesResponse {
	t.Helper()
	if err != nil {
		t.Fatalf("buildCapabilitiesResult error: %v", err)
	}
	if result.IsError {
		t.Fatalf("buildCapabilitiesResult tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var resp capabilitiesResponse
	if uerr := json.Unmarshal([]byte(text), &resp); uerr != nil {
		t.Fatalf("parse capabilities response: %v", uerr)
	}
	return resp
}

// getHTTPCapabilities calls the HTTP CapabilitiesHandler and returns the parsed
// response so the cmd tests can assert against the same struct the server emits.
func getHTTPCapabilities(t *testing.T) api.CapabilitiesResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	w := httptest.NewRecorder()
	api.CapabilitiesHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/capabilities returned %d, want 200", w.Code)
	}
	var resp api.CapabilitiesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse HTTP capabilities response: %v", err)
	}
	return resp
}

// TestCapabilitiesRuntimeReportsResolvedDirs proves the runtime block reports a
// concrete templates/output directory when context is supplied (the CLI now
// passes ./templates and ./output by default), and documents the absence
// explicitly when it is not — never a silent empty string.
func TestCapabilitiesRuntimeReportsResolvedDirs(t *testing.T) {
	// With real directories, both fields resolve to concrete values.
	resolvedResult, resolvedErr := buildCapabilitiesResult(context.Background(), "../../templates", "./output")
	resolved := parseCapabilities(t, resolvedResult, resolvedErr)
	if resolved.Runtime.TemplatesDir == "" || resolved.Runtime.TemplatesDir == runtimeDirUnconfigured {
		t.Errorf("templates_dir should resolve to a concrete path or (embedded), got %q", resolved.Runtime.TemplatesDir)
	}
	if resolved.Runtime.OutputDir != "./output" {
		t.Errorf("output_dir = %q, want ./output", resolved.Runtime.OutputDir)
	}

	// With no context, the absence is documented explicitly.
	unsetResult, unsetErr := buildCapabilitiesResult(context.Background(), "", "")
	unset := parseCapabilities(t, unsetResult, unsetErr)
	if unset.Runtime.TemplatesDir != runtimeDirUnconfigured {
		t.Errorf("empty templates dir should report %q, got %q", runtimeDirUnconfigured, unset.Runtime.TemplatesDir)
	}
	if unset.Runtime.OutputDir != runtimeDirUnconfigured {
		t.Errorf("empty output dir should report %q, got %q", runtimeDirUnconfigured, unset.Runtime.OutputDir)
	}
}

// TestChartDiagramCapabilitiesCLICounterpartIsSkillInfo proves the corrected CLI
// counterpart for the chart/diagram capability tools is real: skill-info inlines
// the same detailed capability arrays the MCP tools return, while `capabilities`
// carries only the type-name registry. This is the drift gate for the metadata
// fix — if skill-info ever stops emitting these arrays, the counterpart claim
// becomes a lie and this test fails.
func TestChartDiagramCapabilitiesCLICounterpartIsSkillInfo(t *testing.T) {
	classes := toolClassifications()
	for _, tool := range []string{"get_chart_capabilities", "get_diagram_capabilities"} {
		if got := classes[tool].CLICounterpart; got != "skill-info" {
			t.Errorf("%s cli_counterpart = %q, want skill-info", tool, got)
		}
	}

	// skill-info must inline the same arrays the MCP tools return.
	st := buildSupportedTypes()
	if got, want := len(st.ChartCapabilities), len(svggen.ChartCapabilities()); got == 0 || got != want {
		t.Errorf("skill-info chart_capabilities count %d != get_chart_capabilities source count %d", got, want)
	}
	if got, want := len(st.DiagramCapabilities), len(svggen.DiagramCapabilitiesReady()); got == 0 || got != want {
		t.Errorf("skill-info diagram_capabilities count %d != get_diagram_capabilities default source count %d", got, want)
	}
}

// TestHTTPCapabilities_MCPOnlyFeatures_AreRealMCPTools ties the HTTP capability
// surface to the canonical MCP catalog: every tool the HTTP API names as
// "MCP-only" must actually be a registered MCP tool, so the HTTP doc cannot
// advertise a renamed or removed tool.
func TestHTTPCapabilities_MCPOnlyFeatures_AreRealMCPTools(t *testing.T) {
	resp := getHTTPCapabilities(t)
	if len(resp.MCPOnlyFeatures) == 0 {
		t.Fatal("HTTP mcp_only_features is empty")
	}
	registered := make(map[string]bool)
	for _, n := range mcpToolNames() {
		registered[n] = true
	}
	for _, feat := range resp.MCPOnlyFeatures {
		// Each entry is "<tool_name> (explanation)".
		fields := strings.Fields(feat)
		if len(fields) == 0 {
			t.Errorf("HTTP mcp_only_features has an empty entry")
			continue
		}
		name := fields[0]
		if !registered[name] {
			t.Errorf("HTTP mcp_only_features lists %q but it is not a registered MCP tool (mcpToolNames())", name)
		}
	}
}

// TestREADME_HTTPEndpoints_MatchHTTPCapabilities ties the README HTTP API table
// to the endpoints the HTTP /api/v1/capabilities response advertises (which the
// internal/api tests in turn pin to setupRoutes). This is the gate that would
// have caught README omitting /api/v1/capabilities.
func TestREADME_HTTPEndpoints_MatchHTTPCapabilities(t *testing.T) {
	resp := getHTTPCapabilities(t)
	doc, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(doc)
	for _, ep := range resp.AvailableEndpoints {
		fields := strings.Fields(ep)
		if len(fields) != 2 {
			t.Fatalf("malformed endpoint string %q", ep)
		}
		path := fields[1]
		if !strings.Contains(readme, path) {
			t.Errorf("README.md does not document HTTP endpoint %q (advertised in /api/v1/capabilities.available_endpoints)", path)
		}
	}
}

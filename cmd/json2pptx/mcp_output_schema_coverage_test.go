package main

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestMCPOutputSchemaCoverage is the doctor test for go-slide-creator-oa6i:
// every tool registered via registerMCPTools MUST publish a JSON output
// schema (RawOutputSchema). Without one, structured-output MCP clients
// (e.g. Codex) cannot validate responses locally and must trust ad-hoc
// fields.
//
// The test is intentionally driven off the same registration helper that
// runMCP uses, so it cannot drift: adding a tool to registerMCPTools makes
// it appear here automatically. Adding a tool without
// mcp.WithRawOutputSchema(...) will fail this test with the exact tool name.
//
// Stronger than the hardcoded `covered` map in
// TestMCPOutputSchemas_AllToolsCovered: that test only verifies the tool
// name is listed in a manually maintained map. This one verifies the
// mcp.Tool struct produced by the constructor actually carries the
// schema after registration — catching the case where someone declares
// `outputSchemaFoo` but forgets to wire it via WithRawOutputSchema in
// mcpFooTool().
func TestMCPOutputSchemaCoverage(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	s := server.NewMCPServer("json2pptx-doctor", "test")
	registerMCPTools(s, mc)

	tools := s.ListTools()
	if len(tools) == 0 {
		t.Fatal("registerMCPTools registered zero tools — coverage check is meaningless")
	}

	var missing []string
	for name, st := range tools {
		if st == nil {
			t.Errorf("ListTools returned a nil ServerTool for %q", name)
			continue
		}
		if len(st.Tool.RawOutputSchema) == 0 {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d MCP tool(s) registered without RawOutputSchema (agents cannot validate their responses):\n  %s\n\nAttach a schema in mcp_output_schemas.go and reference it via mcp.WithRawOutputSchema(...) in the tool factory.",
			len(missing), strings.Join(missing, ", "))
	}
}

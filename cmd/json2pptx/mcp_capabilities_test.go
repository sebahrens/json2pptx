package main

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPGetCapabilities(t *testing.T) {
	t.Run("returns valid capabilities", func(t *testing.T) {
		result, err := handleGetCapabilities(context.Background(), makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp capabilitiesResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.SchemaVersion == "" {
			t.Error("expected non-empty schema_version")
		}
		if resp.ToolVersion == "" {
			t.Error("expected non-empty tool_version")
		}
		if len(resp.MCPToolsAvailable) == 0 {
			t.Error("expected non-empty mcp_tools_available")
		}
		if len(resp.DeprecatedFields) == 0 {
			t.Error("expected non-empty deprecated_fields")
		}
	})

	t.Run("schema_version matches constant", func(t *testing.T) {
		result, _ := handleGetCapabilities(context.Background(), makeRequest(map[string]any{}))
		text := result.Content[0].(mcp.TextContent).Text
		var resp capabilitiesResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.SchemaVersion != SchemaVersion {
			t.Errorf("schema_version mismatch: got %q, want %q", resp.SchemaVersion, SchemaVersion)
		}
	})

	t.Run("mcp_tools_available is sorted", func(t *testing.T) {
		names := mcpToolNames()
		for i := 1; i < len(names); i++ {
			if names[i] < names[i-1] {
				t.Errorf("mcp_tools_available not sorted: %q before %q", names[i-1], names[i])
			}
		}
	})

	t.Run("mcp_tools_available includes get_capabilities itself", func(t *testing.T) {
		names := mcpToolNames()
		found := false
		for _, n := range names {
			if n == "get_capabilities" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected get_capabilities in mcp_tools_available")
		}
	})

	t.Run("features has strict_fit ladder", func(t *testing.T) {
		result, _ := handleGetCapabilities(context.Background(), makeRequest(map[string]any{}))
		text := result.Content[0].(mcp.TextContent).Text
		var resp capabilitiesResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if len(resp.Features.StrictFit) != 3 {
			t.Errorf("expected 3 strict_fit levels, got %d", len(resp.Features.StrictFit))
		}
	})

	t.Run("vocabularies are populated", func(t *testing.T) {
		result, _ := handleGetCapabilities(context.Background(), makeRequest(map[string]any{}))
		text := result.Content[0].(mcp.TextContent).Text
		var resp capabilitiesResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		v := resp.Vocabularies
		if len(v.RepairFixKinds) == 0 {
			t.Error("expected non-empty repair_fix_kinds")
		}
		if len(v.FitFindingCodes) == 0 {
			t.Error("expected non-empty fit_finding_codes")
		}
		if len(v.ContentTypes) == 0 {
			t.Error("expected non-empty content_types")
		}
		if len(v.SlideTransitions) == 0 {
			t.Error("expected non-empty slide_transitions")
		}
		if len(v.ChartTypes) == 0 {
			t.Error("expected non-empty chart_types")
		}
		if len(v.DiagramTypes) == 0 {
			t.Error("expected non-empty diagram_types")
		}
		if len(v.PlaceholderAliases) == 0 {
			t.Error("expected non-empty placeholder_aliases")
		}
		if len(v.PatternNames) == 0 {
			t.Error("expected non-empty pattern_names")
		}
	})

	t.Run("repair_fix_kinds matches applyRepairFix switch cases", func(t *testing.T) {
		// Parse mcp_repair.go and extract case labels from the applyRepairFix switch.
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "mcp_repair.go", nil, 0)
		if err != nil {
			t.Fatalf("failed to parse mcp_repair.go: %v", err)
		}

		var switchCases []string
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "applyRepairFix" {
				return true
			}
			// Walk the function body to find the switch statement.
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cc, ok := n.(*ast.CaseClause)
				if !ok || cc.List == nil {
					return true
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if ok && lit.Kind == token.STRING {
						// Strip quotes.
						switchCases = append(switchCases, lit.Value[1:len(lit.Value)-1])
					}
				}
				return true
			})
			return false
		})

		sort.Strings(switchCases)
		advertised := repairFixKinds()

		if len(switchCases) != len(advertised) {
			t.Fatalf("applyRepairFix has %d cases %v but repairFixKinds advertises %d %v",
				len(switchCases), switchCases, len(advertised), advertised)
		}
		for i := range switchCases {
			if switchCases[i] != advertised[i] {
				t.Errorf("mismatch at index %d: switch has %q, advertised has %q", i, switchCases[i], advertised[i])
			}
		}
	})
}

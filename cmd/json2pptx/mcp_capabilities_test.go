package main

import (
	"context"
	"encoding/json"
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
}

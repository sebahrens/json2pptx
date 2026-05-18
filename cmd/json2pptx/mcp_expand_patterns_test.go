package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// decodeBatchResponse pulls the batchExpansionResponse out of an MCP text
// content envelope; fatals on any decode mishap so test bodies stay flat.
func decodeBatchResponse(t *testing.T, result *mcp.CallToolResult) batchExpansionResponse {
	t.Helper()
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var resp batchExpansionResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, text)
	}
	return resp
}

func TestMCPExpandPatternsBatch(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
	}

	t.Run("expands multiple patterns with per-pattern content", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"theme_template": "midnight-blue",
			"names":          []any{"kpi-3up", "kpi-4up"},
			"content": mustParseJSON(`{
				"kpi-3up": {"values": [{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]},
				"kpi-4up": {"values": [{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"},{"big":"42","small":"NPS"}]}
			}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp := decodeBatchResponse(t, result)
		if resp.BoundsSource != "template" {
			t.Errorf("bounds_source = %q, want %q", resp.BoundsSource, "template")
		}
		if len(resp.Results) != 2 {
			t.Fatalf("got %d results, want 2", len(resp.Results))
		}
		for _, r := range resp.Results {
			if r.Error != nil {
				t.Errorf("pattern %q: unexpected error %+v", r.Pattern, r.Error)
				continue
			}
			if r.UsedExemplar {
				t.Errorf("pattern %q: used_exemplar=true, want false (content supplied)", r.Pattern)
			}
			if r.Result == nil {
				t.Errorf("pattern %q: result is nil", r.Pattern)
				continue
			}
			if r.Result.ShapeGrid == nil {
				t.Errorf("pattern %q: shape_grid is nil", r.Pattern)
			}
			if r.Result.BoundsSource != "template" {
				t.Errorf("pattern %q: per-result bounds_source = %q, want template", r.Pattern, r.Result.BoundsSource)
			}
		}
	})

	t.Run("falls back to exemplar values when content entry is missing", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"names": []any{"kpi-3up"},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp := decodeBatchResponse(t, result)
		if len(resp.Results) != 1 {
			t.Fatalf("got %d results, want 1", len(resp.Results))
		}
		r := resp.Results[0]
		if r.Error != nil {
			t.Fatalf("unexpected per-pattern error: %+v", r.Error)
		}
		if !r.UsedExemplar {
			t.Errorf("expected used_exemplar=true when content entry omitted")
		}
		if r.Result == nil || r.Result.ShapeGrid == nil {
			t.Errorf("exemplar fallback should still produce a shape_grid; got %+v", r.Result)
		}
	})

	t.Run("unknown pattern name surfaces per-entry error without aborting batch", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"names": []any{"kpi-3up", "definitely-not-a-pattern"},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp := decodeBatchResponse(t, result)
		if len(resp.Results) != 2 {
			t.Fatalf("got %d results, want 2", len(resp.Results))
		}
		// First should succeed via exemplar fallback.
		if resp.Results[0].Error != nil {
			t.Errorf("kpi-3up entry: unexpected error %+v", resp.Results[0].Error)
		}
		// Second should be a structured error.
		bad := resp.Results[1]
		if bad.Error == nil {
			t.Fatalf("unknown pattern entry should carry an error")
		}
		if bad.Result != nil {
			t.Errorf("unknown pattern entry should not carry a result")
		}
	})

	t.Run("invalid values produce a per-pattern error but other entries still expand", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"names": []any{"kpi-3up", "kpi-4up"},
			"content": mustParseJSON(`{
				"kpi-3up": {"values": [{"big":"only one","small":"x"}]},
				"kpi-4up": {"values": [{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"},{"big":"42","small":"NPS"}]}
			}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp := decodeBatchResponse(t, result)
		if len(resp.Results) != 2 {
			t.Fatalf("got %d results, want 2", len(resp.Results))
		}
		if resp.Results[0].Error == nil {
			t.Errorf("kpi-3up entry should have an error for too-few values")
		}
		if resp.Results[1].Error != nil {
			t.Errorf("kpi-4up entry should still succeed; got error %+v", resp.Results[1].Error)
		}
		if resp.Results[1].Result == nil {
			t.Errorf("kpi-4up entry should carry a populated result")
		}
	})

	t.Run("missing names parameter returns a tool error", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"theme_template": "midnight-blue",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected tool error when names is missing")
		}
	})

	t.Run("empty names array returns a tool error", func(t *testing.T) {
		result, err := mc.handleExpandPatterns(context.Background(), makeRequest(map[string]any{
			"names": []any{},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected tool error when names is empty")
		}
	})
}

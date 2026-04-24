package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestMCPListPatterns(t *testing.T) {
	result, err := handleListPatterns(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	// Should return a JSON array
	text := result.Content[0].(mcp.TextContent).Text
	var entries []skillPatternCompact
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one pattern")
	}

	// Verify kpi-3up is present
	found := false
	for _, e := range entries {
		if e.Name == "kpi-3up" {
			found = true
			if e.Cells != "3" {
				t.Errorf("kpi-3up cells = %q, want %q", e.Cells, "3")
			}
		}
	}
	if !found {
		t.Error("kpi-3up not found in list_patterns output")
	}
}

func TestMCPShowPattern(t *testing.T) {
	t.Run("known pattern", func(t *testing.T) {
		result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{
			"name": "kpi-3up",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var entry skillPatternFull
		if err := json.Unmarshal([]byte(text), &entry); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if entry.Name != "kpi-3up" {
			t.Errorf("name = %q, want kpi-3up", entry.Name)
		}
		if entry.Version < 1 {
			t.Errorf("version = %d, want >= 1", entry.Version)
		}
		if len(entry.Schema) == 0 {
			t.Error("schema is empty")
		}
		// Schema should be valid JSON
		var schema map[string]any
		if err := json.Unmarshal(entry.Schema, &schema); err != nil {
			t.Errorf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("unknown pattern", func(t *testing.T) {
		result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{
			"name": "nonexistent-pattern",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected tool error for unknown pattern")
		}
	})
}

func TestMCPValidatePattern(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		values := `[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": values,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.OK {
			t.Errorf("expected ok=true, got response: %s", text)
		}
	})

	t.Run("invalid input - wrong count", func(t *testing.T) {
		values := `[{"big":"$1.2M","small":"Revenue"}]`
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": values,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error (should be structured validation): %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.OK {
			t.Error("expected ok=false for invalid input")
		}
		if len(resp.Errors) == 0 {
			t.Error("expected at least one structured error")
		}
	})

	t.Run("multiple errors split", func(t *testing.T) {
		// card-grid with columns=0 + rows=0 produces 2 joined errors;
		// D10 requires they appear as separate entries.
		values := `{"columns":0,"rows":0,"cells":[]}`
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "card-grid",
			"values": values,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.OK {
			t.Error("expected ok=false")
		}
		if len(resp.Errors) < 2 {
			t.Errorf("expected at least 2 separate errors, got %d", len(resp.Errors))
		}
		if len(resp.Errors) > 0 && resp.Errors[0].Field != "columns" {
			t.Errorf("expected first error field='columns', got %q", resp.Errors[0].Field)
		}
	})

	t.Run("unknown pattern", func(t *testing.T) {
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "nonexistent",
			"values": "{}",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected tool error for unknown pattern")
		}
	})

	t.Run("callout unsupported pattern", func(t *testing.T) {
		// matrix-2x2 does not support callout; validate_pattern should reject it
		values := `{"x_axis_label":"X","y_axis_label":"Y","top_left":{"header":"A"},"top_right":{"header":"B"},"bottom_left":{"header":"C"},"bottom_right":{"header":"D"}}`
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":    "matrix-2x2",
			"values":  values,
			"callout": `{"text":"This should fail"}`,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected structured error, got tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.OK {
			t.Fatal("expected ok=false for callout on unsupported pattern")
		}
		if len(resp.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d", len(resp.Errors))
		}
		if resp.Errors[0].Code != "callout_unsupported" {
			t.Errorf("expected code=callout_unsupported, got %q", resp.Errors[0].Code)
		}
		if resp.Errors[0].Fix == nil {
			t.Fatal("expected fix suggestion")
		}
		if resp.Errors[0].Fix.Kind != "remove_field_or_switch_pattern" {
			t.Errorf("expected fix kind=remove_field_or_switch_pattern, got %q", resp.Errors[0].Fix.Kind)
		}
		supported, ok := resp.Errors[0].Fix.Params["supports_callout_patterns"]
		if !ok {
			t.Fatal("fix params missing supports_callout_patterns")
		}
		// Should be a non-empty array
		arr, ok := supported.([]any)
		if !ok || len(arr) == 0 {
			t.Errorf("expected non-empty supports_callout_patterns, got %v", supported)
		}
	})

	t.Run("callout supported pattern ok", func(t *testing.T) {
		values := `{"columns":2,"rows":1,"cells":[{"header":"A","body":"B"},{"header":"C","body":"D"}]}`
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":    "card-grid",
			"values":  values,
			"callout": `{"text":"Valid callout"}`,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.OK {
			t.Errorf("expected ok=true for callout on supported pattern, got: %s", text)
		}
	})
}

func TestMCPExpandPattern(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
	}

	t.Run("expand without template", func(t *testing.T) {
		values := `[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`
		result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": values,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp struct {
			Pattern   string         `json:"pattern"`
			Version   int            `json:"version"`
			ShapeGrid map[string]any `json:"shape_grid"`
		}
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Pattern != "kpi-3up" {
			t.Errorf("pattern = %q, want kpi-3up", resp.Pattern)
		}
		if resp.ShapeGrid == nil {
			t.Error("shape_grid is nil")
		}
		// Should have rows
		if _, ok := resp.ShapeGrid["rows"]; !ok {
			t.Error("shape_grid missing 'rows' key")
		}
	})

	t.Run("expand with template", func(t *testing.T) {
		values := `[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`
		result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
			"name":           "kpi-3up",
			"values":         values,
			"theme_template": "midnight-blue",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("unexpected tool error: %s", text)
		}
	})

	t.Run("invalid values", func(t *testing.T) {
		result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": `[{"big":"only one","small":"x"}]`,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected tool error for invalid values")
		}
	})
}

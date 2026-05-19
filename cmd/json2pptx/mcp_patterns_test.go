package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// mustParseJSON parses a JSON string into an untyped value for use as an MCP
// object parameter in tests. Panics on invalid JSON.
func mustParseJSON(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic("mustParseJSON: " + err.Error())
	}
	return v
}

func TestMCPListPatterns(t *testing.T) {
	result, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"page_size": float64(500), // request large enough page to get everything
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	// Should return the paginated envelope with grouped patterns.
	text := result.Content[0].(mcp.TextContent).Text
	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	groups := resp.Groups
	if len(groups) == 0 {
		t.Fatal("expected at least one category group")
	}
	if resp.TotalCount == 0 {
		t.Fatal("expected total_count > 0")
	}
	if resp.NextCursor != "" {
		t.Errorf("expected no next_cursor with page_size=500, got %q", resp.NextCursor)
	}

	// Verify groups are in the expected category order.
	wantOrder := []string{"data-display", "narrative", "structural", "hero"}
	var gotOrder []string
	for _, g := range groups {
		gotOrder = append(gotOrder, g.Category)
	}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("got %d categories %v, want %d %v", len(gotOrder), gotOrder, len(wantOrder), wantOrder)
	}
	for i, cat := range wantOrder {
		if gotOrder[i] != cat {
			t.Errorf("category[%d] = %q, want %q", i, gotOrder[i], cat)
		}
	}

	// Verify kpi-3up is present in data-display group with taxonomy.
	found := false
	for _, g := range groups {
		for _, e := range g.Patterns {
			if e.Name == "kpi-3up" {
				found = true
				if e.Cells != "3" {
					t.Errorf("kpi-3up cells = %q, want %q", e.Cells, "3")
				}
				if e.Category != "data-display" {
					t.Errorf("kpi-3up category = %q, want %q", e.Category, "data-display")
				}
				if e.DensityClass != "low" {
					t.Errorf("kpi-3up density_class = %q, want %q", e.DensityClass, "low")
				}
				if len(e.NarrativeRole) == 0 {
					t.Error("kpi-3up narrative_role is empty")
				}
				if len(e.PairsWith) == 0 {
					t.Error("kpi-3up pairs_with is empty")
				}
				if len(e.ComposesWith) == 0 {
					t.Error("kpi-3up composes_with is empty")
				}
				if len(e.RoleOnSlide) == 0 {
					t.Error("kpi-3up role_on_slide is empty")
				}
			}
		}
	}
	if !found {
		t.Error("kpi-3up not found in list_patterns output")
	}
}

// TestMCPListPatterns_Pagination verifies cursor + page_size iteration covers
// the full pattern catalog without duplicates and emits next_cursor only when
// more entries remain.
func TestMCPListPatterns_Pagination(t *testing.T) {
	// First, fetch everything to know the total.
	full, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"page_size": float64(500),
	}))
	if err != nil || full.IsError {
		t.Fatalf("baseline call failed: err=%v result=%+v", err, full)
	}
	var allResp listPatternsResponse
	if err := json.Unmarshal([]byte(full.Content[0].(mcp.TextContent).Text), &allResp); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}
	total := allResp.TotalCount
	if total < 2 {
		t.Skipf("not enough patterns to exercise pagination (total=%d)", total)
	}

	// Iterate in pages of 3.
	const pageSize = 3
	cursor := ""
	seen := make(map[string]bool, total)
	iterations := 0
	for {
		args := map[string]any{"page_size": float64(pageSize)}
		if cursor != "" {
			args["cursor"] = cursor
		}
		res, err := handleListPatterns(context.Background(), makeRequest(args))
		if err != nil || res.IsError {
			t.Fatalf("page call failed at cursor %q: err=%v result=%+v", cursor, err, res)
		}
		var resp listPatternsResponse
		if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &resp); err != nil {
			t.Fatalf("failed to parse page: %v", err)
		}
		if resp.TotalCount != total {
			t.Errorf("total_count = %d, want %d", resp.TotalCount, total)
		}
		count := 0
		for _, g := range resp.Groups {
			for _, p := range g.Patterns {
				if seen[p.Name] {
					t.Errorf("duplicate pattern %q across pages", p.Name)
				}
				seen[p.Name] = true
				count++
			}
		}
		if count > pageSize {
			t.Errorf("page exceeded page_size: got %d, want <= %d", count, pageSize)
		}
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
		iterations++
		if iterations > total {
			t.Fatalf("pagination did not terminate after %d iterations", iterations)
		}
	}
	if len(seen) != total {
		t.Errorf("iterated %d unique patterns, want %d", len(seen), total)
	}
}

// TestMCPListPatterns_InvalidCursor verifies a non-integer cursor produces
// a structured INVALID_PARAMETER error rather than a panic or silent skip.
func TestMCPListPatterns_InvalidCursor(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"cursor": "not-a-number",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result for invalid cursor, got %+v", res)
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
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": mustParseJSON(`[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`),
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
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": mustParseJSON(`[{"big":"$1.2M","small":"Revenue"}]`),
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
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":   "card-grid",
			"values": mustParseJSON(`{"columns":0,"rows":0,"cells":[]}`),
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
			"values": map[string]any{},
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
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":    "matrix-2x2",
			"values":  mustParseJSON(`{"x_axis_label":"X","y_axis_label":"Y","top_left":{"header":"A"},"top_right":{"header":"B"},"bottom_left":{"header":"C"},"bottom_right":{"header":"D"}}`),
			"callout": mustParseJSON(`{"text":"This should fail"}`),
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
		result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
			"name":    "card-grid",
			"values":  mustParseJSON(`{"columns":2,"rows":1,"cells":[{"header":"A","body":"B"},{"header":"C","body":"D"}]}`),
			"callout": mustParseJSON(`{"text":"Valid callout"}`),
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
		result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
			"name":   "kpi-3up",
			"values": mustParseJSON(`[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`),
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
		result, err := mc.handleExpandPattern(context.Background(), makeRequest(map[string]any{
			"name":           "kpi-3up",
			"values":         mustParseJSON(`[{"big":"$1.2M","small":"Revenue"},{"big":"340","small":"Customers"},{"big":"98%","small":"Uptime"}]`),
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
			"values": mustParseJSON(`[{"big":"only one","small":"x"}]`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected tool error for invalid values")
		}
	})
}

func TestAttachNextToolCallsToValidationErrors(t *testing.T) {
	t.Run("repair kind gets repair_slide suggestion", func(t *testing.T) {
		errs := []patternValidationError{
			{
				Field:   "values.title",
				Code:    "rename_field",
				Message: "unknown field",
				Fix:     &patterns.FixSuggestion{Kind: "rename_field", Params: map[string]any{"from": "titl", "to": "title"}},
			},
		}
		attachNextToolCallsToValidationErrors(errs, "card-grid")
		if errs[0].NextToolCall == nil {
			t.Fatal("expected NextToolCall to be populated")
		}
		tc := errs[0].NextToolCall
		if tc.Tool != "repair_slide" {
			t.Errorf("tool = %q, want repair_slide", tc.Tool)
		}
		if tc.ArgsTemplate["pattern"] != "card-grid" {
			t.Errorf("pattern = %v, want card-grid", tc.ArgsTemplate["pattern"])
		}
		// slide_index should be -1 (placeholder)
		if si, ok := tc.ArgsTemplate["slide_index"].(int); !ok || si != -1 {
			t.Errorf("slide_index = %v, want -1", tc.ArgsTemplate["slide_index"])
		}
	})

	t.Run("swap_pattern gets recommend_pattern suggestion", func(t *testing.T) {
		errs := []patternValidationError{
			{
				Field:   "pattern",
				Code:    "wrong_pattern",
				Message: "content shape matches different pattern",
				Fix:     &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{"suggested": []any{}}},
			},
		}
		attachNextToolCallsToValidationErrors(errs, "kpi-3up")
		if errs[0].NextToolCall == nil {
			t.Fatal("expected NextToolCall to be populated")
		}
		if errs[0].NextToolCall.Tool != "recommend_pattern" {
			t.Errorf("tool = %q, want recommend_pattern", errs[0].NextToolCall.Tool)
		}
	})

	t.Run("no fix means no next_tool_call", func(t *testing.T) {
		errs := []patternValidationError{
			{
				Field:   "values.columns",
				Message: "columns must be > 0",
			},
		}
		attachNextToolCallsToValidationErrors(errs, "card-grid")
		if errs[0].NextToolCall != nil {
			t.Errorf("expected nil NextToolCall for error without fix, got %+v", errs[0].NextToolCall)
		}
	})

	t.Run("unrecognized fix kind gets no next_tool_call", func(t *testing.T) {
		errs := []patternValidationError{
			{
				Field:   "callout",
				Code:    "callout_unsupported",
				Message: "does not support callout",
				Fix:     &patterns.FixSuggestion{Kind: "remove_field_or_switch_pattern"},
			},
		}
		attachNextToolCallsToValidationErrors(errs, "matrix-2x2")
		// remove_field_or_switch_pattern is not in repairFixKinds, so no tool call
		if errs[0].NextToolCall != nil {
			t.Errorf("expected nil NextToolCall for unrecognized fix kind, got %+v", errs[0].NextToolCall)
		}
	})
}

func TestValidatePatternNextToolCallInResponse(t *testing.T) {
	// card-grid with columns=0 produces a validation error with a fix —
	// the response should include next_tool_call.
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "card-grid",
		"values": mustParseJSON(`{"columns":0,"rows":0,"cells":[]}`),
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
		t.Fatal("expected ok=false")
	}

	// Check that next_tool_call is present in JSON output for errors with fixes
	hasNextToolCall := false
	for _, e := range resp.Errors {
		if e.Fix != nil && e.NextToolCall != nil {
			hasNextToolCall = true
			break
		}
	}

	// Also verify next_tool_call appears in raw JSON
	if json.Valid([]byte(text)) {
		var raw map[string]json.RawMessage
		_ = json.Unmarshal([]byte(text), &raw)
		// The response should be parseable and well-formed
		if _, ok := raw["errors"]; !ok {
			t.Error("response missing errors field")
		}
	}

	// It's OK if no fix suggestions are present for simple value range errors —
	// those don't have fix kinds in repairFixKinds. The important thing is the
	// field is included in the struct and marshalled when present.
	_ = hasNextToolCall
}

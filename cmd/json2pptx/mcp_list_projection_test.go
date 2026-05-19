package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestListPatterns_FieldsCompact verifies that fields=compact strips the
// optional taxonomy/descriptor fields (so they drop from the wire via
// omitempty) while keeping name + category + cells + use_when +
// supports_callout. This is the token-economy default for downstream releases.
func TestListPatterns_FieldsCompact(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"fields":    "compact",
		"page_size": float64(500),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text

	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("fields=compact should suppress deprecation warning, got %v", resp.Warnings)
	}
	if resp.TotalCount == 0 {
		t.Fatal("expected non-zero total_count")
	}
	// Raw JSON inspection — `omitempty` should drop the heavy fields.
	for _, suppressed := range []string{
		`"not_when"`, `"narrative_role"`, `"pairs_with"`,
		`"composes_with"`, `"role_on_slide"`, `"density_class"`,
		`"accent_weight"`, `"estimated_prompt_size_bytes"`,
	} {
		if strings.Contains(text, suppressed) {
			t.Errorf("fields=compact response unexpectedly contains %s field: %s", suppressed, text)
		}
	}
	// Identity / decision fields must still be present.
	for _, expected := range []string{`"name"`, `"category"`, `"cells"`, `"use_when"`} {
		if !strings.Contains(text, expected) {
			t.Errorf("fields=compact response missing %s field", expected)
		}
	}
}

// TestListPatterns_FieldsFull asserts that fields=full preserves the legacy
// taxonomy payload (callers who explicitly opt in keep the rich detail).
func TestListPatterns_FieldsFull(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"fields":    "full",
		"page_size": float64(500),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text

	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("explicit fields=full should suppress deprecation warning, got %v", resp.Warnings)
	}
	// Pick a known pattern that has full taxonomy populated.
	var found bool
	for _, g := range resp.Groups {
		for _, p := range g.Patterns {
			if p.Name == "kpi-3up" {
				found = true
				if len(p.NarrativeRole) == 0 {
					t.Error("kpi-3up narrative_role missing in fields=full mode")
				}
				if p.DensityClass == "" {
					t.Error("kpi-3up density_class missing in fields=full mode")
				}
			}
		}
	}
	if !found {
		t.Fatal("kpi-3up not in response — taxonomy assertions skipped")
	}
}

// TestListPatterns_FieldsUnsetDeprecation verifies the legacy default still
// returns the full payload but emits a deprecation hint nudging callers to be
// explicit.
func TestListPatterns_FieldsUnsetDeprecation(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"page_size": float64(500),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected deprecation warning when fields is omitted")
	}
	// Full payload is still produced (omitempty kept the optional fields off
	// the wire only when empty — kpi-3up's taxonomy is non-empty).
	var saw bool
	for _, g := range resp.Groups {
		for _, p := range g.Patterns {
			if p.Name == "kpi-3up" && len(p.NarrativeRole) > 0 {
				saw = true
			}
		}
	}
	if !saw {
		t.Error("expected full taxonomy when fields is omitted (legacy default)")
	}
}

// TestListPatterns_FieldsInvalid verifies an unknown fields value surfaces a
// structured INVALID_PARAMETER error.
func TestListPatterns_FieldsInvalid(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"fields": "verbose",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result for invalid fields, got %+v", res)
	}
}

// TestListPatterns_Filter verifies the case-insensitive filter applies before
// pagination and only returns matching pattern names.
func TestListPatterns_Filter(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"filter":    "KPI",
		"page_size": float64(500),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount == 0 {
		t.Fatal("expected at least one KPI pattern to match filter")
	}
	count := 0
	for _, g := range resp.Groups {
		for _, p := range g.Patterns {
			count++
			if !strings.Contains(strings.ToLower(p.Name), "kpi") {
				t.Errorf("filter=KPI returned non-matching pattern %q", p.Name)
			}
		}
	}
	if count != resp.TotalCount {
		t.Errorf("page count (%d) != total_count (%d) with large page_size", count, resp.TotalCount)
	}
}

// TestListPatterns_FilterEmpty verifies a filter with no matches returns an
// empty (but well-formed) envelope.
func TestListPatterns_FilterEmpty(t *testing.T) {
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"filter": "nothing-matches-this-string-zzz",
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listPatternsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount != 0 {
		t.Errorf("expected total_count=0 for unmatched filter, got %d", resp.TotalCount)
	}
	if len(resp.Groups) != 0 {
		t.Errorf("expected no groups for unmatched filter, got %d", len(resp.Groups))
	}
}

// TestListTemplates_FieldsCompact verifies that fields=compact drops the
// heavy per-template detail (theme_colors, color_roles, layout_summaries) and
// leaves only identity + capacity counts.
func TestListTemplates_FieldsCompact(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{
		"fields":    "compact",
		"page_size": float64(200),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp skillInfo
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("fields=compact should suppress deprecation warning, got %v", resp.Warnings)
	}
	if len(resp.Templates) == 0 {
		t.Fatal("expected non-empty templates list")
	}
	for _, tmpl := range resp.Templates {
		if tmpl.Name == "" {
			t.Error("template entry missing name")
		}
		if tmpl.ThemeColors != nil {
			t.Errorf("template %q: theme_colors should be omitted in compact mode, got %v", tmpl.Name, tmpl.ThemeColors)
		}
		if tmpl.ColorRoles != nil {
			t.Errorf("template %q: color_roles should be omitted in compact mode", tmpl.Name)
		}
		if len(tmpl.LayoutSummaries) != 0 {
			t.Errorf("template %q: layout_summaries should be omitted in compact mode", tmpl.Name)
		}
		if tmpl.TitleFont != "" {
			t.Errorf("template %q: title_font should be omitted in compact mode, got %q", tmpl.Name, tmpl.TitleFont)
		}
	}
}

// TestListTemplates_FieldsUnsetDeprecation verifies the legacy default
// produces a non-empty payload AND a deprecation warning nudging callers to
// pick a projection.
func TestListTemplates_FieldsUnsetDeprecation(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp skillInfo
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected deprecation warning when fields is omitted (and mode untouched)")
	}
	// Default mode=compact still populates theme detail.
	if len(resp.Templates) == 0 || resp.Templates[0].ThemeColors == nil {
		t.Error("expected legacy compact mode to populate theme_colors")
	}
}

// TestListTemplates_ModeExplicitSuppressesWarning verifies that the
// deprecation hint is NOT emitted when the caller has pinned `mode` — they
// already made an explicit choice and don't need the nudge.
func TestListTemplates_ModeExplicitSuppressesWarning(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{
		"mode": "list",
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp skillInfo
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("mode=list (explicit) should suppress deprecation warning, got %v", resp.Warnings)
	}
}

// TestListTemplates_FieldsInvalid verifies an unknown fields value surfaces a
// structured INVALID_PARAMETER error.
func TestListTemplates_FieldsInvalid(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{
		"fields": "verbose",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result for invalid fields, got %+v", res)
	}
}

// TestListTemplates_Filter verifies the case-insensitive filter narrows the
// templates list before pagination.
func TestListTemplates_Filter(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{
		"filter":    "MIDNIGHT",
		"mode":      "list",
		"page_size": float64(200),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp skillInfo
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount == 0 {
		t.Fatal("expected midnight-blue (or similar) to match filter")
	}
	for _, tmpl := range resp.Templates {
		if !strings.Contains(strings.ToLower(tmpl.Name), "midnight") {
			t.Errorf("filter=MIDNIGHT returned non-matching template %q", tmpl.Name)
		}
	}
}

// TestListIcons_FieldsCompact verifies fields=compact drops the redundant
// per-set `icons[]` qualified-name array while keeping `names[]`.
func TestListIcons_FieldsCompact(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"fields":    "compact",
		"page_size": float64(50),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listIconsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("fields=compact should suppress deprecation warning, got %v", resp.Warnings)
	}
	if len(resp.Sets) == 0 {
		t.Fatal("expected non-empty sets list")
	}
	for _, s := range resp.Sets {
		if len(s.Icons) != 0 {
			t.Errorf("set %q: icons[] should be empty in compact mode, got %d entries", s.Set, len(s.Icons))
		}
		if len(s.Names) == 0 {
			t.Errorf("set %q: names should still be populated", s.Set)
		}
	}
	// The icons key should not appear on the wire (omitempty).
	if strings.Contains(text, `"icons":`) {
		t.Errorf("fields=compact response unexpectedly contains an icons[] array: %s", text)
	}
}

// TestListIcons_FilterOverridesSearch verifies the canonical `filter`
// parameter takes precedence over the legacy `search` alias when both are
// supplied.
func TestListIcons_FilterOverridesSearch(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"set":       "outline",
		"filter":    "arrow",
		"search":    "chart",
		"page_size": float64(500),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listIconsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount == 0 {
		t.Fatal("expected at least one arrow icon to match filter")
	}
	for _, s := range resp.Sets {
		for _, n := range s.Names {
			if !strings.Contains(strings.ToLower(n), "arrow") {
				t.Errorf("filter=arrow returned non-matching icon %q (search=chart should be ignored)", n)
			}
		}
	}
}

// TestListIcons_FieldsUnsetDeprecation verifies the legacy default emits a
// deprecation hint and still ships the icons[] dual array.
func TestListIcons_FieldsUnsetDeprecation(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"set":       "outline",
		"page_size": float64(10),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var resp listIconsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected deprecation warning when fields is omitted")
	}
	if len(resp.Sets) == 0 || len(resp.Sets[0].Icons) == 0 {
		t.Error("expected legacy default to populate icons[] dual array")
	}
}

// TestListIcons_FieldsInvalid verifies an unknown fields value surfaces a
// structured INVALID_PARAMETER error.
func TestListIcons_FieldsInvalid(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"fields": "verbose",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result for invalid fields, got %+v", res)
	}
}

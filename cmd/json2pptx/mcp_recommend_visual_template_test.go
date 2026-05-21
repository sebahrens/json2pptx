package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// callRecommendVisual invokes the recommend_visual handler with args and parses
// the typed result. It fails the test on any error or tool error.
func callRecommendVisual(t *testing.T, mc *mcpConfig, args map[string]any) patterns.RecommendVisualResult {
	t.Helper()
	result, err := mc.handleRecommendVisual(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var rec patterns.RecommendVisualResult
	if err := json.Unmarshal([]byte(text), &rec); err != nil {
		t.Fatalf("failed to parse recommend_visual result: %v\n%s", err, text)
	}
	return rec
}

// TestMCPRecommendVisual_NoTemplate_NoSupport confirms that without template
// context, candidates carry no template_support (backward compatible).
func TestMCPRecommendVisual_NoTemplate_NoSupport(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent": "show our Q3 revenue trend",
	})
	if len(rec.Candidates) == 0 {
		t.Fatal("expected candidates")
	}
	for _, c := range rec.Candidates {
		if c.TemplateSupport != nil {
			t.Errorf("candidate %q: template_support should be nil without template context", c.Name)
		}
	}
}

// TestMCPRecommendVisual_WithTemplate_AnnotatesSupport confirms that passing a
// bundled template annotates every candidate with a valid template_support.
func TestMCPRecommendVisual_WithTemplate_AnnotatesSupport(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent":   "show our Q3 revenue trend",
		"template": "midnight-blue",
	})
	if len(rec.Candidates) == 0 {
		t.Fatal("expected candidates")
	}
	valid := map[string]bool{
		patterns.TemplateSupportSupported:   true,
		patterns.TemplateSupportRisky:       true,
		patterns.TemplateSupportUnsupported: true,
	}
	for _, c := range rec.Candidates {
		if c.TemplateSupport == nil {
			t.Errorf("candidate %q: template_support not set with template context", c.Name)
			continue
		}
		if !valid[c.TemplateSupport.Status] {
			t.Errorf("candidate %q: invalid status %q", c.Name, c.TemplateSupport.Status)
		}
		if len(c.TemplateSupport.Reasons) == 0 {
			t.Errorf("candidate %q: template_support has no reasons", c.Name)
		}
	}
}

// TestMCPRecommendVisual_WithTemplate_TopIsNotUnsupported confirms that
// support-aware demotion keeps the top recommendation feasible for the template
// (it is not "unsupported" when any supported/risky candidate exists).
func TestMCPRecommendVisual_WithTemplate_TopIsNotUnsupported(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent":   "show our Q3 revenue trend",
		"template": "midnight-blue",
	})
	if len(rec.Candidates) == 0 {
		t.Fatal("expected candidates")
	}

	hasFeasible := false
	for _, c := range rec.Candidates {
		if c.TemplateSupport != nil && c.TemplateSupport.Status != patterns.TemplateSupportUnsupported {
			hasFeasible = true
			break
		}
	}
	if hasFeasible {
		top := rec.Candidates[0]
		if top.TemplateSupport != nil && top.TemplateSupport.Status == patterns.TemplateSupportUnsupported {
			t.Errorf("top candidate %q is unsupported while a feasible candidate exists — demotion failed", top.Name)
		}
	}
}

// TestMCPRecommendVisual_UnknownTemplate errors with TEMPLATE_NOT_FOUND.
func TestMCPRecommendVisual_UnknownTemplate(t *testing.T) {
	mc := testMCPConfig(t)
	result, err := mc.handleRecommendVisual(context.Background(), makeRequest(map[string]any{
		"intent":   "show a chart",
		"template": "no-such-template-xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for unknown template, got success: %v", result.Content)
	}
}

// TestMCPRecommendVisual_CandidatesMode_WithTemplate confirms candidates mode
// still returns every supplied name, each annotated with template_support.
func TestMCPRecommendVisual_CandidatesMode_WithTemplate(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent":     "compare top KPIs",
		"template":   "midnight-blue",
		"candidates": []any{"kpi-3up", "bar", "two-column"},
	})
	if len(rec.Candidates) != 3 {
		t.Fatalf("candidates mode should return all 3 supplied names, got %d: %+v", len(rec.Candidates), rec.Candidates)
	}
	for _, c := range rec.Candidates {
		if c.TemplateSupport == nil {
			t.Errorf("candidate %q: template_support not set in candidates mode", c.Name)
		}
	}
}

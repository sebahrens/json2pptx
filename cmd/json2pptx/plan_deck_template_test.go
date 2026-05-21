package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// callPlanDeck invokes the plan_deck handler with args and parses the typed
// result. It fails the test on any error or tool error.
func callPlanDeck(t *testing.T, mc *mcpConfig, args map[string]any) planDeckResult {
	t.Helper()
	result, err := mc.handlePlanDeck(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var plan planDeckResult
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		t.Fatalf("failed to parse plan_deck result: %v\n%s", err, text)
	}
	return plan
}

// TestMCPPlanDeck_NoTemplate_NoSupport confirms that without template context,
// no slide (or alternative) carries template_support and template is empty —
// the plan is template-agnostic and backward compatible.
func TestMCPPlanDeck_NoTemplate_NoSupport(t *testing.T) {
	mc := testMCPConfig(t)
	plan := callPlanDeck(t, mc, map[string]any{
		"brief":        "quarterly business review for the leadership team",
		"slide_budget": 8.0,
	})
	if plan.Template != "" {
		t.Errorf("template should be empty without template context, got %q", plan.Template)
	}
	if len(plan.Slides) == 0 {
		t.Fatal("expected slides")
	}
	for _, s := range plan.Slides {
		if s.TemplateSupport != nil {
			t.Errorf("slide %d: template_support should be nil without template context", s.SlideIndex)
		}
		for _, alt := range s.Alternatives {
			if alt.TemplateSupport != nil {
				t.Errorf("slide %d: alternative %q has template_support without template context", s.SlideIndex, alt.PatternName)
			}
		}
	}
}

// TestMCPPlanDeck_WithTemplate_AnnotatesSupport confirms that passing a bundled
// template annotates every slide (and alternative) with a valid template_support
// and echoes the template name.
func TestMCPPlanDeck_WithTemplate_AnnotatesSupport(t *testing.T) {
	mc := testMCPConfig(t)
	plan := callPlanDeck(t, mc, map[string]any{
		"brief":        "investor pitch for an AI infrastructure company",
		"slide_budget": 10.0,
		"template":     "midnight-blue",
	})
	if plan.Template != "midnight-blue" {
		t.Errorf("template echo = %q, want midnight-blue", plan.Template)
	}
	if len(plan.Slides) == 0 {
		t.Fatal("expected slides")
	}
	valid := map[string]bool{
		patterns.TemplateSupportSupported:   true,
		patterns.TemplateSupportRisky:       true,
		patterns.TemplateSupportUnsupported: true,
	}
	for _, s := range plan.Slides {
		if s.TemplateSupport == nil {
			t.Errorf("slide %d: template_support not set with template context", s.SlideIndex)
			continue
		}
		if !valid[s.TemplateSupport.Status] {
			t.Errorf("slide %d: invalid status %q", s.SlideIndex, s.TemplateSupport.Status)
		}
		if len(s.TemplateSupport.Reasons) == 0 {
			t.Errorf("slide %d: template_support has no reasons", s.SlideIndex)
		}
		for _, alt := range s.Alternatives {
			if alt.TemplateSupport == nil {
				t.Errorf("slide %d: alternative %q missing template_support", s.SlideIndex, alt.PatternName)
				continue
			}
			if !valid[alt.TemplateSupport.Status] {
				t.Errorf("slide %d: alternative %q invalid status %q", s.SlideIndex, alt.PatternName, alt.TemplateSupport.Status)
			}
		}
	}
}

// TestMCPPlanDeck_WithTemplate_NoUnsupportedRecommendation confirms the plan
// never leaves a slide recommending a pattern the template cannot host (the
// swap guarantee). On bundled templates every named pattern is hostable, so
// this also documents the common-case expectation.
func TestMCPPlanDeck_WithTemplate_NoUnsupportedRecommendation(t *testing.T) {
	mc := testMCPConfig(t)
	for _, tmpl := range []string{"midnight-blue", "forest-green", "warm-coral", "modern-template"} {
		plan := callPlanDeck(t, mc, map[string]any{
			"brief":        "product strategy update with metrics and roadmap",
			"slide_budget": 12.0,
			"template":     tmpl,
		})
		for _, s := range plan.Slides {
			if s.TemplateSupport == nil {
				t.Fatalf("%s slide %d: expected template_support", tmpl, s.SlideIndex)
			}
			if s.TemplateSupport.Status == patterns.TemplateSupportUnsupported {
				t.Errorf("%s slide %d: recommended pattern %q is unsupported by the template", tmpl, s.SlideIndex, s.RecommendedPattern)
			}
		}
	}
}

// TestMCPPlanDeck_AgreesWithRecommendVisual is the cross-tool agreement check:
// for the same template, plan_deck's per-slide template_support.status for a
// pattern must match the status recommend_visual reports for that same pattern
// (both go through the shared generator.TemplateSupportContext helper).
func TestMCPPlanDeck_AgreesWithRecommendVisual(t *testing.T) {
	mc := testMCPConfig(t)
	const tmpl = "midnight-blue"

	plan := callPlanDeck(t, mc, map[string]any{
		"brief":        "go-to-market plan with KPIs, comparison, and a closing quote",
		"slide_budget": 9.0,
		"template":     tmpl,
	})

	// Collect each distinct recommended pattern and its plan-reported status.
	planStatus := map[string]string{}
	for _, s := range plan.Slides {
		if s.TemplateSupport == nil {
			t.Fatalf("slide %d: missing template_support", s.SlideIndex)
		}
		planStatus[s.RecommendedPattern] = s.TemplateSupport.Status
	}

	for pattern, status := range planStatus {
		rec := callRecommendVisual(t, mc, map[string]any{
			"intent":     "build this slide",
			"template":   tmpl,
			"candidates": []any{pattern},
		})
		if len(rec.Candidates) != 1 {
			t.Fatalf("recommend_visual candidates mode returned %d for %q", len(rec.Candidates), pattern)
		}
		got := rec.Candidates[0].TemplateSupport
		if got == nil {
			t.Fatalf("recommend_visual returned no template_support for %q", pattern)
		}
		if got.Status != status {
			t.Errorf("pattern %q: plan_deck status %q != recommend_visual status %q", pattern, status, got.Status)
		}
	}
}

// TestMCPPlanDeck_UnknownTemplate errors with a tool error rather than producing
// a plan, matching recommend_visual's contract.
func TestMCPPlanDeck_UnknownTemplate(t *testing.T) {
	mc := testMCPConfig(t)
	result, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{
		"brief":    "some deck",
		"template": "no-such-template-xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for unknown template, got success: %v", result.Content)
	}
}

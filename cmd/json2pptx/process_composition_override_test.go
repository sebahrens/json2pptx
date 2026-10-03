package main

import (
	"context"
	"strings"
	"testing"
)

// disputeProcessSlide is the consulting benchmark's process slide
// (go-slide-creator-vj549): four described control points whose default
// numbered strip renders below the readable floor on the modern template.
func disputeProcessSlide(override map[string]any) map[string]any {
	slide := map[string]any{
		"kind":  "process",
		"title": "Four control points give each dispute one owner and an auditable path to closure",
		"steps": []any{
			map[string]any{"label": "Capture", "description": disputeDescriptions[0]},
			map[string]any{"label": "Assign", "description": disputeDescriptions[1]},
			map[string]any{"label": "Resolve", "description": disputeDescriptions[2]},
			map[string]any{"label": "Close", "description": disputeDescriptions[3]},
		},
		"takeaway": "Automate routing and evidence capture; retain human approval for commercial exceptions.",
		"source":   "Illustrative Helios case; management assumptions, Oct 2026",
	}
	for k, v := range override {
		slide[k] = v
	}
	return slide
}

var disputeDescriptions = []string{
	"Finance creates one case record, links the invoice and assigns a reason code.",
	"The market lead names a resolver; cases without an owner escalate after 24 hours.",
	"The resolver agrees the action; credits above EUR25k require a second approval.",
	"Finance verifies cash or credit, closes the case and updates recurring-cause rules.",
}

func renderDisputeProcess(t *testing.T, mc *mcpConfig, override map[string]any) renderDeckSpecResponse {
	t.Helper()
	spec := map[string]any{
		"meta":   map[string]any{"template": "modern", "title": "Dispute control"},
		"slides": []any{disputeProcessSlide(override)},
	}
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil {
		t.Fatal(err)
	}
	var render renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &render)
	return render
}

// The default strip fails on modern; the advertised layout:content alternative
// then renders as native bullets with no pattern, keeps all four descriptions,
// and the explanation says exactly that. Before the fix the explanation said
// layout content while the render was still the refused strip.
func TestProcessContentOverrideRendersWhatItReports(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := refusalTestConfig(t)

	refused := renderDisputeProcess(t, mc, nil)
	if refused.OK {
		t.Fatal("the default numbered strip was expected to be refused on modern; the regression no longer exercises the alternative")
	}
	if !strings.Contains(refused.Error, "numbered-step-strip") {
		t.Errorf("refusal does not name the default strip: %s", refused.Error)
	}

	fixed := renderDisputeProcess(t, mc, map[string]any{"layout": "content"})
	if !fixed.OK {
		t.Fatalf("layout:content did not render: %s %+v", fixed.Error, fixed.Diagnostics)
	}
	if strings.Contains(fixed.Error, "numbered-step-strip") {
		t.Errorf("layout:content still compiled the strip: %s", fixed.Error)
	}
	if fixed.Explanation == nil || len(fixed.Explanation.Slides) != 1 {
		t.Fatalf("no explanation_summary: %+v", fixed.Explanation)
	}
	if got := fixed.Explanation.Slides[0]; got.Layout != "content" || got.Pattern != "" {
		t.Errorf("explanation = pattern %q layout %q, want no pattern on layout content", got.Pattern, got.Layout)
	}
	slides := readPPTXSlides(t, fixed.PptxPath)
	if len(slides) != 1 {
		t.Fatalf("rendered %d slides, want 1", len(slides))
	}
	for _, d := range disputeDescriptions {
		if !strings.Contains(slides[0], d) {
			t.Errorf("rendered slide lost the description %q", d)
		}
	}
}

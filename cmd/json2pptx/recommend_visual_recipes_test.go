package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/svggen"
)

// recommendVisualFor calls recommend_visual and decodes the structured result.
func recommendVisualFor(t *testing.T, mc *mcpConfig, args map[string]any) patterns.RecommendVisualResult {
	t.Helper()
	res, err := mc.handleRecommendVisual(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("recommend_visual: %v", err)
	}
	if res.IsError {
		t.Fatalf("recommend_visual error: %s", resultText(res))
	}
	var rec patterns.RecommendVisualResult
	structuredInto(t, res.StructuredContent, &rec)
	return rec
}

// renderRecipe runs a candidate's next_tool_call through render_deck_spec and
// returns the verdict.
func renderRecipe(t *testing.T, mc *mcpConfig, c patterns.VisualCandidate) renderDeckSpecResponse {
	t.Helper()
	if c.NextToolCall == nil {
		t.Fatalf("%s %q has no next_tool_call", c.Category, c.Name)
	}
	if c.NextToolCall.Tool != "render_deck_spec" {
		t.Fatalf("%s next_tool_call.tool = %q, want render_deck_spec", c.Name, c.NextToolCall.Tool)
	}
	if !toolInProfile(t, toolProfileDeckSpec, c.NextToolCall.Tool) {
		t.Fatalf("next_tool_call %q is not advertised by the default profile", c.NextToolCall.Tool)
	}
	// Round-trip through JSON exactly as an agent would receive and resend it.
	raw, err := json.Marshal(c.NextToolCall.ArgsTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("render_deck_spec: %v", err)
	}
	var verdict renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &verdict)
	return verdict
}

func toolInProfile(t *testing.T, profile, tool string) bool {
	t.Helper()
	names := deckSpecToolNames
	if profile == toolProfileCore {
		names = coreToolNames
	}
	for _, n := range names {
		if n == tool {
			return true
		}
	}
	return false
}

// TestRecommendVisualRecipesRenderForEveryType is the acceptance test for
// go-slide-creator-b7qqg.24: every chart and diagram type recommend_visual can
// rank carries a data contract and a render_deck_spec next call on the default
// profile, and that call renders the visual without errors.
func TestRecommendVisualRecipesRenderForEveryType(t *testing.T) {
	if testing.Short() {
		t.Skip("renders one deck per chart / diagram type")
	}
	mc := semanticTestConfig(t)

	type entry struct {
		cat  patterns.VisualCategory
		name string
	}
	var types []entry
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			types = append(types, entry{patterns.VisualCategoryChart, c.Type})
		}
	}
	for _, d := range svggen.DiagramCapabilitiesReady() {
		types = append(types, entry{patterns.VisualCategoryDiagram, d.Type})
	}

	for _, e := range types {
		name := e.name
		t.Run(name, func(t *testing.T) {
			// Candidates mode resolves a few names (timeline, pyramid) to the
			// named pattern of the same name, so attach to the chart /
			// diagram candidate directly — the same call the handler makes.
			rec := patterns.RecommendVisualResult{Candidates: []patterns.VisualCandidate{{Category: e.cat, Name: name}}}
			attachVisualRecipes(&rec, "midnight-blue")
			c := rec.Candidates[0]
			if c.DataContract == nil || c.DataContract.FieldPath == "" {
				t.Fatalf("%s (%s) has no data_contract", name, c.Category)
			}
			verdict := renderRecipe(t, mc, c)
			if !verdict.Success {
				t.Fatalf("recipe for %s did not render: error=%q diags=%+v", name, verdict.Error, verdict.Diagnostics)
			}
			for _, d := range verdict.Diagnostics {
				if d.Severity == "error" {
					t.Errorf("recipe for %s raised error diagnostic %+v", name, d)
				}
			}
			for _, w := range verdict.Warnings {
				lw := strings.ToLower(w)
				if strings.Contains(lw, "fail") || strings.Contains(lw, "invalid") || strings.Contains(lw, "dropped") {
					t.Errorf("recipe for %s warns: %s", name, w)
				}
			}
			for _, d := range verdict.Diagnostics {
				t.Logf("diagnostic: %s %s %s", d.Severity, d.Code, d.Message)
			}
			for _, w := range verdict.Warnings {
				t.Logf("warning: %s", w)
			}
			assertDeckHasVisual(t, verdict.PptxPath)
		})
	}
}

// TestRecommendVisualRankingIntentGetsHorizontalBars covers
// go-slide-creator-oocqj: a ranking intent hands the bar candidate a
// horizontal, sorted recipe, and that recipe renders.
func TestRecommendVisualRankingIntentGetsHorizontalBars(t *testing.T) {
	if !rankingIntent("Rank the top 5 markets by revenue") || !rankingIntent("Germany is the largest market") {
		t.Fatal("ranking intents not recognised")
	}
	if rankingIntent("Revenue grew every quarter") {
		t.Fatal("a trend intent was read as a ranking")
	}
	rec := patterns.RecommendVisualResult{
		QueryUnderstood: "rank the top 5 markets by revenue",
		Candidates:      []patterns.VisualCandidate{{Category: patterns.VisualCategoryChart, Name: "bar"}},
	}
	attachVisualRecipes(&rec, "midnight-blue")
	c := rec.Candidates[0]
	raw, _ := json.Marshal(c.NextToolCall.ArgsTemplate)
	if !strings.Contains(string(raw), `"orientation":"horizontal"`) {
		t.Fatalf("ranking recipe is not horizontal: %s", raw)
	}
	if testing.Short() {
		return
	}
	verdict := renderRecipe(t, semanticTestConfig(t), c)
	if !verdict.Success {
		t.Fatalf("ranked bar recipe did not render: %q %+v", verdict.Error, verdict.Diagnostics)
	}
}

// assertDeckHasVisual checks slide 1 embeds a rendered chart/diagram (an SVG /
// PNG picture or native diagram shapes beyond the title).
func assertDeckHasVisual(t *testing.T, pptxPath string) {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()
	var slide string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, oErr := f.Open()
			if oErr != nil {
				t.Fatal(oErr)
			}
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, io.LimitReader(rc, 16<<20))
			_ = rc.Close()
			slide = buf.String()
		}
	}
	if !strings.Contains(slide, "<p:pic>") && strings.Count(slide, "<p:sp>") < 3 {
		t.Errorf("slide 1 carries no rendered visual (no picture, %d shapes)", strings.Count(slide, "<p:sp>"))
	}
}

// TestRecommendVisualDiagramIntentsReachRunnableSpec follows the review's
// default-profile journey: recommend_visual for a gantt / venn / pestel intent
// ranks the diagram and hands back a runnable spec for it.
func TestRecommendVisualDiagramIntentsReachRunnableSpec(t *testing.T) {
	mc := semanticTestConfig(t)
	for intent, want := range map[string]string{
		"Show implementation tasks with start and end dates, overlapping workstreams and dependencies in a project schedule.": "gantt",
		"Venn diagram of the overlap between customer need and product strengths":                                             "venn",
		"PESTEL analysis of the political, economic, social, technological, environmental and legal context":                  "pestel",
	} {
		t.Run(want, func(t *testing.T) {
			rec := recommendVisualFor(t, mc, map[string]any{"intent": intent, "template": "midnight-blue"})
			var found *patterns.VisualCandidate
			for i := range rec.Candidates {
				if rec.Candidates[i].Name == want {
					found = &rec.Candidates[i]
				}
				c := rec.Candidates[i]
				if (c.Category == patterns.VisualCategoryChart || c.Category == patterns.VisualCategoryDiagram) && c.NextToolCall == nil {
					t.Errorf("%s candidate %q has no next_tool_call", c.Category, c.Name)
				}
			}
			if found == nil {
				t.Fatalf("recommend_visual did not rank %s for %q: %+v", want, intent, rec.Candidates)
			}
			spec, _ := found.NextToolCall.ArgsTemplate["spec"].(map[string]any)
			meta, _ := spec["meta"].(map[string]any)
			if meta["template"] != "midnight-blue" {
				t.Errorf("recipe spec template = %v, want the requested midnight-blue", meta["template"])
			}
			// Validate (fast, no render) with the default-profile validator.
			raw, _ := json.Marshal(found.NextToolCall.ArgsTemplate)
			var args map[string]any
			_ = json.Unmarshal(raw, &args)
			res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(args))
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError || strings.Contains(resultText(res), `"severity":"error"`) {
				t.Errorf("recipe spec for %s does not validate: %s", want, resultText(res))
			}
		})
	}
}

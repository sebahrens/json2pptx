package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// composeRecipeTemplates are the template families compose recipes are
// rendered on; the local, untracked p-style joins when present.
func composeRecipeTemplates() []string {
	tpls := []string{"midnight-blue", "warm-coral"}
	if _, err := os.Stat("../../templates/p-style.pptx"); err == nil {
		tpls = append(tpls, "p-style")
	}
	return tpls
}

// composeRecipeCases are the emitted compose candidates the recipes cover: a
// pattern pair from normal mode, a pattern pair resolved from the shortlist,
// and the heterogeneous three-region composition.
var composeRecipeCases = []struct {
	id   string
	args map[string]any
	want string
}{
	{"panels+quote", map[string]any{"intent": "panels and quote side by side on one slide"}, "compose:pull-quote+stylish-panels"},
	{"kpi+process", map[string]any{"intent": "kpi metrics with a process flow combined", "candidates": []any{"compose:kpi-3up+process-flow", "kpi-3up"}}, "compose:kpi-3up+process-flow"},
	{"chart+kpi+timeline", map[string]any{
		"intent":        "line chart left 65%, KPI upper-right, timeline lower-right on one slide",
		"content_hints": map[string]any{"item_count": 3, "has_metrics": true, "has_chart": true, "columns": 2},
	}, "compose:chart:line+diagram:timeline+stat-hero"},
}

// TestRecommendVisualComposeRecipes is the acceptance test for
// go-slide-creator-okg00 / -lp0o8: the top compose candidate carries a
// runnable render_deck_spec recipe — raw_json2pptx slide on the canonical
// blank-title layout with a title and compose.segments — plus per-region
// segment paths and contracts, and that recipe validates and renders on the
// default profile on two template families (and p-style when present).
func TestRecommendVisualComposeRecipes(t *testing.T) {
	mc := semanticTestConfig(t)
	for _, tpl := range composeRecipeTemplates() {
		for _, tc := range composeRecipeCases {
			t.Run(tpl+"/"+tc.id, func(t *testing.T) {
				args := map[string]any{"template": tpl}
				for k, v := range tc.args {
					args[k] = v
				}
				rec := recommendVisualFor(t, mc, args)
				top := rec.Candidates[0]
				if top.Category != patterns.VisualCategoryCompose || top.Name != tc.want {
					t.Fatalf("top candidate = %s %q, want compose %q: %+v", top.Category, top.Name, tc.want, rec.Candidates)
				}
				assertComposeRecipeShape(t, top, tpl)
				assertRecipeValidates(t, mc, top)
				if testing.Short() {
					return
				}
				verdict := renderRecipe(t, mc, top)
				if !verdict.Success {
					t.Fatalf("recipe did not render: error=%q diags=%+v", verdict.Error, verdict.Diagnostics)
				}
				for _, d := range verdict.Diagnostics {
					if isPlaceholderFinding(d) {
						continue // the recipe's own scaffolding: see TestRecipesVerbatimAreBlockedAsExemplarContent
					}
					if d.Severity == "error" || d.Severity == "warning" {
						t.Errorf("recipe raised %s %s: %s", d.Severity, d.Code, d.Message)
					}
				}
				if tc.id == "chart+kpi+timeline" {
					assertThreeRegionContentVisible(t, verdict.PptxPath)
				}
			})
		}
	}
}

// assertComposeRecipeShape checks the recipe's spec and the region contracts.
func assertComposeRecipeShape(t *testing.T, c patterns.VisualCandidate, tpl string) {
	t.Helper()
	if c.NextToolCall == nil || c.NextToolCall.Tool != "render_deck_spec" {
		t.Fatalf("compose candidate has no render_deck_spec next_tool_call: %+v", c.NextToolCall)
	}
	if !toolInProfile(t, toolProfileDeckSpec, c.NextToolCall.Tool) {
		t.Fatal("render_deck_spec is not on the default profile")
	}
	spec, _ := c.NextToolCall.ArgsTemplate["spec"].(map[string]any)
	meta, _ := spec["meta"].(map[string]any)
	if meta["template"] != tpl {
		t.Errorf("recipe template = %v, want %s", meta["template"], tpl)
	}
	slides, _ := spec["slides"].([]any)
	if len(slides) != 1 {
		t.Fatalf("recipe has %d slides, want 1", len(slides))
	}
	s0, _ := slides[0].(map[string]any)
	if s0["kind"] != "raw_json2pptx" {
		t.Errorf("recipe slide kind = %v, want raw_json2pptx", s0["kind"])
	}
	slide, _ := s0["slide"].(map[string]any)
	if slide["layout_id"] != composeRecipeLayoutID {
		t.Errorf("recipe layout_id = %v, want %s", slide["layout_id"], composeRecipeLayoutID)
	}
	content, _ := slide["content"].([]any)
	if len(content) != 1 || content[0].(map[string]any)["placeholder_id"] != "title" {
		t.Errorf("recipe content should carry exactly the title: %+v", content)
	}
	compose, _ := slide["compose"].(map[string]any)
	if compose["direction"] != c.Composition.Direction {
		t.Errorf("recipe direction %v != composition direction %s", compose["direction"], c.Composition.Direction)
	}
	if c.Composition.Instructions == "" {
		t.Error("composition has no adoption instructions")
	}
	raw, _ := json.Marshal(spec)
	var round any
	_ = json.Unmarshal(raw, &round)
	for _, leaf := range c.Composition.Leaves() {
		if leaf.SegmentPath == "" || leaf.DataContract == nil || leaf.DataContract.FieldPath == "" {
			t.Errorf("region %s %q lacks segment_path / data_contract: %+v", leaf.Category, leaf.Name, leaf)
			continue
		}
		if !strings.HasPrefix(leaf.DataContract.FieldPath, leaf.SegmentPath+".") {
			t.Errorf("field_path %q is not inside segment %q", leaf.DataContract.FieldPath, leaf.SegmentPath)
		}
		if v := lookupSpecPath(round, strings.TrimPrefix(leaf.DataContract.FieldPath, "")); v == nil {
			t.Errorf("field_path %q does not resolve in the recipe spec", leaf.DataContract.FieldPath)
		}
		if len(leaf.DataContract.RequiredKeys) == 0 && len(leaf.DataContract.OptionalKeys) == 0 {
			t.Errorf("region %q contract lists no keys", leaf.Name)
		}
	}
}

// lookupSpecPath resolves a "slides[0].slide.compose.segments[1].x" path in
// a JSON-decoded spec, returning nil when any step is missing.
func lookupSpecPath(root any, path string) any {
	cur := root
	for _, part := range strings.Split(path, ".") {
		name, idx := part, -1
		if i := strings.IndexByte(part, '['); i >= 0 {
			name = part[:i]
			n := 0
			for _, r := range part[i+1 : len(part)-1] {
				n = n*10 + int(r-'0')
			}
			idx = n
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[name]
		if idx >= 0 {
			arr, ok := cur.([]any)
			if !ok || idx >= len(arr) {
				return nil
			}
			cur = arr[idx]
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// assertRecipeValidates runs the recipe through validate_deck_spec.
func assertRecipeValidates(t *testing.T, mc *mcpConfig, c patterns.VisualCandidate) {
	t.Helper()
	raw, _ := json.Marshal(c.NextToolCall.ArgsTemplate)
	var args map[string]any
	_ = json.Unmarshal(raw, &args)
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	assertOnlyPlaceholdersBlock(t, "recipe", res)
}

// assertThreeRegionContentVisible checks the three-region render keeps every
// chart category and series, the KPI value and label, and every timeline
// phase and milestone.
func assertThreeRegionContentVisible(t *testing.T, pptxPath string) {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()
	var slide, svgs strings.Builder
	for _, f := range zr.File {
		isSlide := f.Name == "ppt/slides/slide1.xml"
		isSVG := strings.HasPrefix(f.Name, "ppt/media/") && strings.HasSuffix(f.Name, ".svg")
		if !isSlide && !isSVG {
			continue
		}
		rc, oErr := f.Open()
		if oErr != nil {
			t.Fatal(oErr)
		}
		dst := &svgs
		if isSlide {
			dst = &slide
		}
		_, _ = io.Copy(dst, io.LimitReader(rc, 16<<20))
		_ = rc.Close()
	}
	for _, want := range []string{"$2.4B", "Addressable AI consulting market by FY27"} {
		if !strings.Contains(slide.String(), want) {
			t.Errorf("KPI text %q missing from slide 1", want)
		}
	}
	for _, want := range []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Mobile app", "Web", "Discovery", "Build", "Rollout", "Go-live"} {
		if !strings.Contains(svgs.String(), want) {
			t.Errorf("chart / timeline text %q missing from the rendered SVGs", want)
		}
	}
}

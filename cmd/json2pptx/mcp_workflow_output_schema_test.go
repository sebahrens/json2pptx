package main

import (
	"encoding/base64"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

type workflowSchemaContext struct {
	server   *server.MCPServer
	deck     map[string]any
	rendered renderDeckSpecResponse
}

type workflowSchemaFixture struct {
	tool           string
	externalRender bool
	run            func(*testing.T, *workflowSchemaContext)
}

func liveSchemaSuccess(t *testing.T, s *server.MCPServer, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result := assertLiveOutputSchema(t, s, tool, args)
	if result.IsError {
		t.Fatalf("%s success fixture returned error: %s", tool, resultText(result))
	}
	return result
}

// Each entry owns a runnable successful request, rather than a schema-presence
// assertion. The completeness guard below shares this exact execution inventory.
func workflowLiveOutputFixtures() []workflowSchemaFixture {
	simple := func(tool string, render bool, args func(*workflowSchemaContext) map[string]any) workflowSchemaFixture {
		return workflowSchemaFixture{tool, render, func(t *testing.T, c *workflowSchemaContext) { liveSchemaSuccess(t, c.server, tool, args(c)) }}
	}
	pres := func(c *workflowSchemaContext) map[string]any { return map[string]any{"presentation": c.deck} }
	path := func(c *workflowSchemaContext) map[string]any { return map[string]any{"pptx_path": c.rendered.PptxPath} }
	kpis := []any{map[string]any{"big": "42%", "small": "Growth"}, map[string]any{"big": "98%", "small": "Retention"}}
	pattern := func(*workflowSchemaContext) map[string]any {
		return map[string]any{"name": "kpi-2up", "values": kpis, "theme_template": "midnight-blue"}
	}
	return []workflowSchemaFixture{
		simple("generate_presentation", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "output_filename": "schema-raw.pptx"}
		}),
		simple("validate_input", false, pres),
		simple("validate_pattern", false, pattern),
		simple("expand_pattern", false, pattern),
		simple("expand_patterns", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"names": []string{"kpi-2up"}, "theme_template": "midnight-blue", "content": map[string]any{"kpi-2up": map[string]any{"values": kpis}}}
		}),
		simple("preview_icon", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"icon": map[string]any{"name": "filled:chart-pie"}, "size_px": 64}
		}),
		simple("preview_presentation_plan", false, pres),
		simple("preview_slide_wireframe", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "slide_index": 0, "format": "svg"}
		}),
		simple("score_deck", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "mode": "deterministic"}
		}),
		simple("score_candidates", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "slide_index": 0, "candidates": []any{c.deck["slides"].([]map[string]any)[0]}}
		}),
		simple("analyze_deck_rhythm", false, pres),
		simple("repair_slide", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": mustParseJSON(threeSlideHeavyDeck()), "slide_index": 0, "fixes": []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 2}}}}
		}),
		simple("repair_slides_batch", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": mustParseJSON(threeSlideHeavyDeck()), "fixes": []any{map[string]any{"slide_index": 0, "kind": "reduce_text", "params": map[string]any{"max_items": 2}}}}
		}),
		simple("propose_repairs", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "findings": []any{map[string]any{"path": "/slides/0/content/1", "code": "BODY_TOO_LONG", "action": "shrink_or_split", "fix": map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 2}}}}}
		}),
		simple("apply_deck_patch", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": mustParseJSON(threeSlideDeck()), "ops": []any{map[string]any{"op": "remove_slide", "index": 1}}}
		}),
		simple("auto_repair", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"presentation": c.deck, "max_passes": 1, "output_filename": "schema-repair.pptx"}
		}),
		simple("make_deck", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"outline": "Series B funding: revenue grew 18% to $42M, gross margin 61%, retention 112%. Approve the routing pilot.", "template": "midnight-blue", "style_hints": map[string]any{"slide_budget": 3}, "max_repair_passes": 1, "output_filename": "schema-made.pptx"}
		}),
		simple("read_presentation", false, path),
		simple("export_deck", false, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"pptx_path": c.rendered.PptxPath, "format": "notes"}
		}),
		simple("validate_presentation_output", false, func(c *workflowSchemaContext) map[string]any { return map[string]any{"path": c.rendered.PptxPath} }),
		simple("compile_deck_spec", false, func(*workflowSchemaContext) map[string]any { return map[string]any{"spec": validSemanticSpec} }),
		{"validate_deck_spec", false, func(t *testing.T, c *workflowSchemaContext) {
			first := liveSchemaSuccess(t, c.server, "validate_deck_spec", map[string]any{"spec": validSemanticSpec})
			var out deckSpecEnvelopeResponse
			structuredInto(t, first.StructuredContent, &out)
			if out.DeckID == "" {
				t.Fatal("validated spec did not enter the handle store")
			}
			patched := liveSchemaSuccess(t, c.server, "validate_deck_spec", map[string]any{"deck_id": out.DeckID, "patch": []any{map[string]any{"op": "replace", "path": "/slides/1/title", "value": "Approve the pilot"}}})
			var updated deckSpecEnvelopeResponse
			structuredInto(t, patched.StructuredContent, &updated)
			if updated.DeckID != out.DeckID || len(updated.ChangedSlides) != 1 {
				t.Fatalf("store patch lost identity/change metadata: %+v", updated)
			}
		}},
		simple("render_deck_spec", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"spec": validSemanticSpec, "output_filename": "schema-semantic.pptx"}
		}),
		{"inspect_slide_images", false, func(t *testing.T, c *workflowSchemaContext) {
			liveSchemaSuccess(t, c.server, "inspect_slide_images", map[string]any{"slide_images": []any{map[string]any{"index": 0, "png_base64": base64.StdEncoding.EncodeToString(makeUpperHeavyPNG(t))}}})
		}},
		simple("register_template_setting", false, func(*workflowSchemaContext) map[string]any {
			return map[string]any{"template_name": "midnight-blue", "kind": "table_styles", "name": "schema-style", "definition": map[string]any{"style_id": "@template-default"}}
		}),
		{"delete_template_setting", false, func(t *testing.T, c *workflowSchemaContext) {
			liveSchemaSuccess(t, c.server, "register_template_setting", map[string]any{"template_name": "midnight-blue", "kind": "table_styles", "name": "delete-schema-style", "definition": map[string]any{"style_id": "@template-default"}})
			liveSchemaSuccess(t, c.server, "delete_template_setting", map[string]any{"template_name": "midnight-blue", "kind": "table_styles", "name": "delete-schema-style"})
		}},
		simple("purge_render_cache", false, func(*workflowSchemaContext) map[string]any { return map[string]any{} }),
		simple("render_slide_image", true, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"pptx_path": c.rendered.PptxPath, "slide_index": 0, "density": 50}
		}),
		simple("render_slide_image_from_json", true, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"slide": c.deck["slides"].([]map[string]any)[0], "template": "midnight-blue", "density": 50}
		}),
		simple("render_deck_thumbnails", true, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"pptx_path": c.rendered.PptxPath, "density": 50}
		}),
		simple("audit_palette", true, func(c *workflowSchemaContext) map[string]any {
			return map[string]any{"pptx_path": c.rendered.PptxPath, "density": 50}
		}),
		{"submit_visual_review", true, func(t *testing.T, c *workflowSchemaContext) {
			thumbs := liveSchemaSuccess(t, c.server, "render_deck_thumbnails", map[string]any{"pptx_path": c.rendered.PptxPath, "density": 50})
			var images renderedDeckThumbnailsResponse
			structuredInto(t, thumbs.StructuredContent, &images)
			if len(images.Slides) != c.rendered.SlideCount {
				t.Fatal("review fixture lacks complete rendered images")
			}
			slides := make([]any, len(images.Slides))
			for i, im := range images.Slides {
				slides[i] = map[string]any{"index": im.Index, "image_path": im.Path, "verdict": "approved"}
			}
			liveSchemaSuccess(t, c.server, "submit_visual_review", map[string]any{"pptx_path": c.rendered.PptxPath, "pptx_revision": sha256OfFile(t, c.rendered.PptxPath), "slides": slides})
		}},
	}
}

func runWorkflowOutputSchemas(t *testing.T, externalRender bool) {
	t.Helper()
	// Template setting writes and render-cache sweeping are confined to test-owned roots.
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")
	t.Setenv("ANTHROPIC_API_KEY", "")
	mc := settingsTestConfig(t)
	mc.loopSessions = newLoopSessionStore(loopSessionTTL)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	c := &workflowSchemaContext{server: newMCPServer(mc), deck: minimalScoreDeck(2)}
	c.rendered = renderDeckSpec(t, mc, map[string]any{"spec": validSemanticSpec, "output_filename": "schema-seed.pptx"})
	for _, fixture := range workflowLiveOutputFixtures() {
		if fixture.externalRender != externalRender {
			continue
		}
		t.Run(fixture.tool, func(t *testing.T) {
			fixture.run(t, c)
		})
	}
}

func TestMCPWorkflowLiveOutputSchemas(t *testing.T) { runWorkflowOutputSchemas(t, false) }
func TestMCPWorkflowRenderLiveOutputSchemas(t *testing.T) {
	testutil.RequireRenderTools(t)
	runWorkflowOutputSchemas(t, true)
}

func TestMCPLiveOutputSchemaFixturesCoverRegistry(t *testing.T) {
	covered := map[string]bool{}
	for _, f := range discoveryLiveOutputFixtures() {
		covered[f.tool] = true
	}
	for _, f := range workflowLiveOutputFixtures() {
		if covered[f.tool] {
			t.Errorf("duplicate tool fixture ownership: %s", f.tool)
		}
		covered[f.tool] = true
	}
	registered := newMCPServer(testMCPConfig(t)).ListTools()
	var missing []string
	for name := range registered {
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("registered tools lack a runnable live-success output fixture: %v", missing)
	}
	for name := range covered {
		if _, ok := registered[name]; !ok {
			t.Errorf("fixture references unregistered tool %s", name)
		}
	}
}

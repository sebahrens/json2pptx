package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-dykl. list_templates{} measured 153,266 B on the wire and
// then WARNED that the caller should have asked for compact; list_patterns{}
// measured 69,692 B and did the same. get_started tells the agent to call
// list_templates with no arguments and the error-path next_tool_call
// suggestions use args_template {}, so the expensive default was exactly the
// one agents hit. The server paid first and advised afterwards.

func structuredJSON(t *testing.T, res *mcpgo.CallToolResult, v any) {
	t.Helper()
	if res == nil || res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	text := res.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(text), v); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

func TestListTemplatesDefaultIsCompact(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	var def skillInfo
	res, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	structuredJSON(t, res, &def)

	if len(def.Warnings) != 0 {
		t.Errorf("the default carries a warning: %v", def.Warnings)
	}
	if def.SupportedTypes != nil {
		t.Error("the default carries supported_types — 14,388 B of static per-server data")
	}
	if len(def.Templates) == 0 {
		t.Fatal("expected templates")
	}
	if def.Templates[0].ThemeColors != nil || len(def.Templates[0].Layouts) != 0 {
		t.Error("the default carries per-template detail; that is the full projection")
	}
	if len(def.Templates[0].CanonicalLayoutIDs) == 0 {
		t.Error("the default omitted concrete canonical layout IDs")
	}

	// The default must equal the explicit compact projection, byte for byte.
	explicit, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{"fields": "compact"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Content[0].(mcpgo.TextContent).Text != explicit.Content[0].(mcpgo.TextContent).Text {
		t.Error("the default and fields=\"compact\" produced different payloads")
	}

	// And fields="full" still restores everything.
	var full skillInfo
	fullRes, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{"fields": "full"}))
	if err != nil {
		t.Fatal(err)
	}
	structuredJSON(t, fullRes, &full)
	if full.SupportedTypes == nil {
		t.Error("fields=\"full\" lost supported_types")
	}
	if len(full.Templates) == 0 || full.Templates[0].ThemeColors == nil {
		t.Error("fields=\"full\" lost the per-template detail")
	}

	// The whole point: the default is dramatically smaller.
	defSize := len(res.Content[0].(mcpgo.TextContent).Text)
	fullSize := len(fullRes.Content[0].(mcpgo.TextContent).Text)
	if defSize >= fullSize/4 {
		t.Errorf("default %d B is not meaningfully smaller than full %d B", defSize, fullSize)
	}
	// Concrete canonical IDs add a small per-template map to the formerly
	// 10 KB list projection; keep the full all-template response under 16 KB.
	if defSize > 16*1024 {
		t.Errorf("default payload is %d B, over the 16 KB budget", defSize)
	}
}

func TestListPatternsDefaultIsCompact(t *testing.T) {
	var def listPatternsResponse
	res, err := handleListPatterns(context.Background(), makeRequest(map[string]any{"page_size": float64(500)}))
	if err != nil {
		t.Fatal(err)
	}
	structuredJSON(t, res, &def)

	if len(def.Warnings) != 0 {
		t.Errorf("the default carries a warning: %v", def.Warnings)
	}
	if len(def.Groups) == 0 {
		t.Fatal("expected pattern groups")
	}
	for _, g := range def.Groups {
		// dogfood-b: "each group object has no name/label field I could find,
		// so grouping conveys nothing".
		if g.Category == "" {
			t.Error("a pattern group carries no category")
		}
		for _, p := range g.Patterns {
			if len(p.NarrativeRole) > 0 || len(p.PairsWith) > 0 || p.DensityClass != "" {
				t.Errorf("pattern %q carries the full taxonomy in the compact default", p.Name)
			}
			if p.Name == "" || p.Category == "" {
				t.Errorf("compact entry lost an identifying field: %+v", p)
			}
		}
	}

	var full listPatternsResponse
	fullRes, err := handleListPatterns(context.Background(), makeRequest(map[string]any{
		"page_size": float64(500), "fields": "full",
	}))
	if err != nil {
		t.Fatal(err)
	}
	structuredJSON(t, fullRes, &full)
	var sawTaxonomy bool
	for _, g := range full.Groups {
		for _, p := range g.Patterns {
			if len(p.NarrativeRole) > 0 {
				sawTaxonomy = true
			}
		}
	}
	if !sawTaxonomy {
		t.Error("fields=\"full\" lost the taxonomy payload")
	}

	defSize := len(res.Content[0].(mcpgo.TextContent).Text)
	fullSize := len(fullRes.Content[0].(mcpgo.TextContent).Text)
	if defSize >= fullSize {
		t.Errorf("default %d B is not smaller than full %d B", defSize, fullSize)
	}
}

func TestListIconsDefaultIsCompact(t *testing.T) {
	var def listIconsResponse
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{"page_size": float64(200)}))
	if err != nil {
		t.Fatal(err)
	}
	structuredJSON(t, res, &def)

	if len(def.Warnings) != 0 {
		t.Errorf("the default carries a warning: %v", def.Warnings)
	}
	for _, s := range def.Sets {
		if len(s.Icons) != 0 {
			t.Errorf("set %q carries the redundant icons[] dual array in the compact default", s.Set)
		}
		if len(s.Names) == 0 {
			t.Errorf("set %q lost its names[]", s.Set)
		}
	}
}

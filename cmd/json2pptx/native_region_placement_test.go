package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/svggen"
)

// nativeRegionFixtures are short fixtures for every native diagram type —
// the ten canonical types the 2026-10-02 sweep found failing in a region,
// SWOT and five forces, and the three panel aliases — with one authored
// label each that must reach the slide.
var nativeRegionFixtures = map[string]struct {
	data  map[string]any
	label string
}{
	"business_model_canvas": {map[string]any{"key_partners": []any{"Partners"}, "key_activities": []any{"Develop"}, "key_resources": []any{"People"}, "value_proposition": []any{"ValueProp"}, "customer_relationships": []any{"Support"}, "channels": []any{"Sales"}, "customer_segments": []any{"Teams"}, "cost_structure": []any{"Payroll"}, "revenue_streams": []any{"Licences"}}, "ValueProp"},
	"heatmap":               {map[string]any{"row_labels": []any{"North", "South"}, "col_labels": []any{"Q1", "Q2"}, "values": []any{[]any{1, 2}, []any{3, 4}}}, "North"},
	"house_diagram":         {map[string]any{"roof": "Vision", "foundation": "People", "sections": []any{map[string]any{"label": "Growth", "items": []any{"Scale"}}}}, "Growth"},
	"kpi_dashboard":         {map[string]any{"metrics": []any{map[string]any{"label": "Revenue", "value": "12M"}, map[string]any{"label": "Margin", "value": "42%"}}}, "12M"},
	"nine_box_talent":       {map[string]any{"employees": []any{map[string]any{"name": "Alice", "performance": 2, "potential": 2}}}, "Alice"},
	"panel_layout":          {map[string]any{"layout": "columns", "panels": []any{map[string]any{"title": "Growth", "body": "Scale"}, map[string]any{"title": "Control", "body": "Manage"}}}, "Manage"},
	"pestel":                {map[string]any{"political": []any{"Trade"}, "economic": []any{"Growth"}, "social": []any{"Demand"}, "technological": []any{"AI"}, "environmental": []any{"Energy"}, "legal": []any{"Privacy"}}, "Privacy"},
	"porters_five_forces":   {map[string]any{"forces": []any{map[string]any{"type": "rivalry", "label": "Rivalry", "factors": []any{"RivalItem"}}, map[string]any{"type": "new_entrants", "label": "New Entrants", "factors": []any{"x"}}, map[string]any{"type": "substitutes", "label": "Substitutes", "factors": []any{"x"}}, map[string]any{"type": "suppliers", "label": "Suppliers", "factors": []any{"x"}}, map[string]any{"type": "buyers", "label": "Buyers", "factors": []any{"x"}}}}, "RivalItem"},
	"process_flow":          {map[string]any{"steps": []any{"Design", "Build", "Launch"}}, "Launch"},
	"pyramid":               {map[string]any{"levels": []any{map[string]any{"label": "Strategy"}, map[string]any{"label": "Delivery"}, map[string]any{"label": "Operations"}}}, "Operations"},
	"swot":                  {map[string]any{"strengths": []any{"Talent"}, "weaknesses": []any{"Scale"}, "opportunities": []any{"Growth"}, "threats": []any{"Rivals"}}, "Rivals"},
	"value_chain":           {map[string]any{"primary": []any{map[string]any{"name": "Inbound", "items": []any{"Receive"}}, map[string]any{"name": "Operations", "items": []any{"Build"}}, map[string]any{"name": "Outbound", "items": []any{"Ship"}}}, "support": []any{map[string]any{"name": "People", "items": []any{"Train"}}}}, "Outbound"},
	"icon_columns":          {map[string]any{"panels": []any{map[string]any{"title": "Growth", "body": "Scale"}, map[string]any{"title": "Control", "body": "Manage"}}}, "Manage"},
	"icon_rows":             {map[string]any{"panels": []any{map[string]any{"title": "Growth", "body": "Scale"}, map[string]any{"title": "Control", "body": "Manage"}}}, "Manage"},
	"stat_cards":            {map[string]any{"panels": []any{map[string]any{"title": "Revenue", "value": "12M"}, map[string]any{"title": "Margin", "value": "42%"}}}, "12M"},
}

// go-slide-creator-6x9bt / -3grgs: every diagram type get_diagram_capabilities
// advertises with a native_ooxml shape_grid placement generates through that
// placement — as a shape_grid cell beside commentary and as a compose segment
// — as an editable native group (no picture) carrying its authored text, on
// a tracked template and on the local p-style template when present.
func TestAdvertisedNativeRegionPlacementsGenerate(t *testing.T) {
	var advertised []string
	for _, c := range generator.ApplyPlacementMetadata(svggen.DiagramCapabilitiesReady()) {
		for _, p := range c.Placements {
			if p.Context == "shape_grid" && p.Pipeline == "native_ooxml" {
				advertised = append(advertised, c.Type)
			}
		}
	}
	sort.Strings(advertised)
	if len(advertised) < len(nativeRegionFixtures) {
		t.Errorf("only %d native types advertise a shape_grid placement: %v", len(advertised), advertised)
	}
	var slides []any
	for _, typ := range advertised {
		fx, ok := nativeRegionFixtures[typ]
		if !ok {
			t.Fatalf("%s advertises a native shape_grid placement but has no render fixture", typ)
		}
		diagram := map[string]any{"type": typ, "data": fx.data, "alt": "Fixture " + typ}
		title := []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": typ}}
		slides = append(slides,
			map[string]any{"layout_id": "blank-title", "content": title, "shape_grid": map[string]any{
				"columns": []any{70, 30},
				"rows": []any{map[string]any{"cells": []any{
					map[string]any{"diagram": diagram},
					map[string]any{"shape": map[string]any{"geometry": "rect", "fill": "none", "text": "Margin 42%"}},
				}}},
			}},
			map[string]any{"layout_id": "blank-title", "content": title, "compose": map[string]any{
				"direction": "horizontal",
				"segments": []any{
					map[string]any{"diagram": diagram, "size_pct": 70},
					map[string]any{"pattern": map[string]any{"name": "stat-hero", "values": map[string]any{"value": "42%", "label": "Margin"}}, "size_pct": 30},
				},
			}})
	}

	templates := []string{"midnight-blue"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			data, err := json.Marshal(map[string]any{"template": tpl, "output_filename": "native.pptx", "slides": slides})
			if err != nil {
				t.Fatal(err)
			}
			in := filepath.Join(dir, "input.json")
			if err := os.WriteFile(in, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := runJSONMode(in, filepath.Join(dir, "result.json"), "../../templates", dir,
				"", false, false, tpl, "off", false, "off", "", false); err != nil {
				t.Fatalf("generate: %v", err)
			}
			parts := readPPTXSlides(t, filepath.Join(dir, "native.pptx"))
			for i, typ := range advertised {
				for j, how := range []string{"shape_grid", "compose"} {
					xml := parts[2*i+j]
					if !strings.Contains(xml, `descr="Fixture `+typ+`"`) {
						t.Errorf("%s via %s: no native group carrying the diagram's alt text", typ, how)
					}
					if !strings.Contains(xml, nativeRegionFixtures[typ].label) && !strings.Contains(xml, strings.ToUpper(nativeRegionFixtures[typ].label)) {
						t.Errorf("%s via %s: authored %q missing", typ, how, nativeRegionFixtures[typ].label)
					}
				}
			}
		})
	}
}

// readPPTXSlides returns the slide XML of a deck in slide order.
func readPPTXSlides(t *testing.T, path string) []string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	byNum := map[int]string{}
	for _, f := range zr.File {
		var n int
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		if _, err := fmt.Sscanf(f.Name, "ppt/slides/slide%d.xml", &n); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		byNum[n] = string(b)
	}
	out := make([]string, 0, len(byNum))
	for i := 1; i <= len(byNum); i++ {
		out = append(out, byNum[i])
	}
	return out
}

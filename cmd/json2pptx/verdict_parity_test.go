package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// One verdict (go-slide-creator-9k5fh): every surface that accepts a raw deck
// answers the same question the same way. A deck is valid for all of them or
// for none, and one they refuse is refused with the same error findings — the
// same codes at the same paths.

// surfaceVerdict is one surface's answer for one deck.
type surfaceVerdict struct {
	Valid  bool
	Errors []string // "CODE @ path" of each error finding, sorted, deduplicated
}

func (v surfaceVerdict) String() string {
	if v.Valid {
		return "valid"
	}
	return "invalid [" + strings.Join(v.Errors, "; ") + "]"
}

// verdictOf reads an answer's findings envelope. findings is the envelope's
// "findings" list as decoded JSON.
func verdictOf(valid bool, findings []any) surfaceVerdict {
	seen := map[string]bool{}
	v := surfaceVerdict{Valid: valid}
	for _, raw := range findings {
		f, _ := raw.(map[string]any)
		if f == nil || f["severity"] != "error" {
			continue
		}
		path, _ := f["path"].(string)
		if ev, ok := f["evidence"].(map[string]any); ok && path == "" {
			path, _ = ev["path"].(string)
		}
		// An MCP tool addresses a deck it cannot parse by its argument name;
		// the CLI has the deck itself.
		if path == "presentation" {
			path = ""
		}
		key := fmt.Sprintf("%v @ %s", f["code"], path)
		if !seen[key] {
			seen[key] = true
			v.Errors = append(v.Errors, key)
		}
	}
	sort.Strings(v.Errors)
	return v
}

func envelopeFindings(t *testing.T, doc map[string]any) []any {
	t.Helper()
	switch fs := doc["findings"].(type) {
	case []any:
		return fs
	case map[string]any:
		inner, _ := fs["findings"].([]any)
		return inner
	}
	return nil
}

// verdictSurfaces runs one deck file through every raw-deck surface and
// returns each one's verdict by name.
func verdictSurfaces(t *testing.T, mc *mcpConfig, deckPath string, generate bool) map[string]surfaceVerdict {
	t.Helper()
	data, err := os.ReadFile(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	var presentation any
	if err := json.Unmarshal(data, &presentation); err != nil {
		t.Fatal(err)
	}
	out := map[string]surfaceVerdict{}
	decode := func(name, text string) map[string]any {
		var doc map[string]any
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			t.Fatalf("%s: answer is not JSON: %v\n%s", name, err, text)
		}
		return doc
	}
	baseDir := filepath.Dir(deckPath)

	// validate_input (what `json2pptx validate` runs).
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": presentation, "fit_report": false, "base_dir": baseDir,
	}))
	if err != nil {
		t.Fatal(err)
	}
	doc := decode("validate_input", textContent(res))
	out["validate"] = verdictOf(!res.IsError, envelopeFindings(t, doc))

	// generate --dry-run.
	var dryErr error
	dry := captureStdout(t, func() {
		dryErr = runJSONDryRun(deckPath, "", mc.templatesDir, "", "", false)
	})
	doc = decode("generate --dry-run", dry)
	valid, _ := doc["valid"].(bool)
	if valid != (dryErr == nil) {
		t.Errorf("generate --dry-run: valid=%t but error=%v", valid, dryErr)
	}
	out["dry-run"] = verdictOf(valid, envelopeFindings(t, doc))

	if !generate {
		return out
	}

	// generate (CLI).
	report := filepath.Join(t.TempDir(), "report.json")
	genErr := runJSONMode(deckPath, report, mc.templatesDir, t.TempDir(), "", false, false, "", "warn", false, "strict", "", false)
	reportData, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("generate wrote no report (error %v): %v", genErr, err)
	}
	doc = decode("generate", string(reportData))
	success, _ := doc["success"].(bool)
	if !success && genErr == nil && len(verdictOf(false, envelopeFindings(t, doc)).Errors) > 0 {
		t.Errorf("generate: refused the deck but returned no error")
	}
	out["generate"] = verdictOf(success, envelopeFindings(t, doc))

	// generate_presentation (MCP).
	res, err = mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": presentation, "base_dir": baseDir,
	}))
	if err != nil {
		t.Fatal(err)
	}
	doc = decode("generate_presentation", textContent(res))
	success = !res.IsError
	if reported, ok := doc["success"].(bool); ok && !res.IsError {
		success = reported
	}
	out["generate_presentation"] = verdictOf(success, envelopeFindings(t, doc))
	return out
}

// assertOneVerdict fails when the surfaces disagree.
func assertOneVerdict(t *testing.T, verdicts map[string]surfaceVerdict) surfaceVerdict {
	t.Helper()
	names := make([]string, 0, len(verdicts))
	for name := range verdicts {
		names = append(names, name)
	}
	sort.Strings(names)
	ref := verdicts["validate"]
	agree := true
	for _, name := range names {
		if verdicts[name].String() != ref.String() {
			agree = false
		}
	}
	if !agree {
		var b strings.Builder
		for _, name := range names {
			fmt.Fprintf(&b, "\n  %-22s %s", name, verdicts[name])
		}
		t.Errorf("surfaces disagree:%s", b.String())
	}
	return ref
}

// verdictCover opens every mutation deck; verdictSlides are the valid slides a
// mutation breaks, one per deck so each case costs two slides.
const verdictCover = `{"layout_id": "title", "content": [
  {"placeholder_id": "title", "type": "text", "text_value": "Quarterly review"},
  {"placeholder_id": "subtitle", "type": "text", "text_value": "Results and outlook"}]}`

var verdictSlides = map[string]string{
	"bullets": `{"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Revenue grew in every region"},
      {"placeholder_id": "body", "type": "bullets", "bullets_value": ["North up 12%", "South up 8%", "West up 5%"]}]}`,
	"chart": `{"layout_id": "content", "source": "Finance, FY25", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Revenue rose each quarter"},
      {"placeholder_id": "body", "type": "chart", "chart_value": {"type": "bar", "title": "Revenue",
        "data": {"Q1": 10, "Q2": 12, "Q3": 15}}}]}`,
	"table": `{"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three options were compared"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["Option", "Cost"],
        "rows": [["Build", "High"], ["Buy", "Low"]]}}]}`,
	"grid": `{"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Two steps lead to launch"}],
      "shape_grid": {"rows": [{"cells": [
        {"shape": {"geometry": "rect", "text": "Plan"}},
        {"shape": {"geometry": "rect", "text": "Launch"}}]}]},
      "overlays": [{"kind": "arrow",
        "from": {"anchor_cell": {"row": 0, "col": 0}}, "to": {"anchor_cell": {"row": 0, "col": 1}}}]}`,
	"pattern": `{"layout_id": "blank-title", "source": "Finance, FY25", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three numbers tell the story"}],
      "pattern": {"name": "kpi-3up", "values": [
        {"big": "12%", "small": "Growth"}, {"big": "94", "small": "NPS"}, {"big": "3", "small": "Launches"}]}}`,
}

// verdictMutation is one single-fault edit of a valid two-slide deck: the
// cover and the named slide.
type verdictMutation struct {
	name  string
	slide string
	// invalid says the fault is one generation refuses; false marks a control
	// that every surface must keep accepting.
	invalid bool
	// apply edits the deck; target is the named slide (slides[1]).
	apply func(deck, target map[string]any)
}

func verdictJSON(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic(err)
	}
	return v
}

// verdictMutations covers each structural refusal generation makes — the error
// constructions in json_mode.go, overlays.go, pattern_resolve.go, structure.go
// and the shape-grid resolver — and each one validation makes.
func verdictMutations() []verdictMutation {
	set := func(key, value string) func(_, target map[string]any) {
		return func(_, target map[string]any) { target[key] = verdictJSON(value) }
	}
	content := func(target map[string]any, item int) map[string]any {
		return target["content"].([]any)[item].(map[string]any)
	}
	setContent := func(key, value string) func(_, target map[string]any) {
		return func(_, target map[string]any) { content(target, 1)[key] = verdictJSON(value) }
	}
	delContent := func(key string) func(_, target map[string]any) {
		return func(_, target map[string]any) { delete(content(target, 1), key) }
	}
	retype := func(drop, kind, field, value string) func(_, target map[string]any) {
		return func(_, target map[string]any) {
			c := content(target, 1)
			delete(c, drop)
			c["type"] = kind
			c[field] = verdictJSON(value)
		}
	}
	top := func(key, value string) func(deck, _ map[string]any) {
		return func(deck, _ map[string]any) { deck[key] = verdictJSON(value) }
	}
	const kpis = `[{"big": "1", "small": "a"}, {"big": "2", "small": "b"}, {"big": "3", "small": "c"}]`
	const cellB = `{"shape": {"geometry": "rect", "text": "b"}}`
	return []verdictMutation{
		{"control: unchanged", "bullets", false, func(_, _ map[string]any) {}},
		{"control: every slide kind", "bullets", false, func(deck, _ map[string]any) {
			for _, name := range []string{"chart", "table", "grid", "pattern"} {
				deck["slides"] = append(deck["slides"].([]any), verdictJSON(verdictSlides[name]))
			}
		}},
		{"control: unknown top-level key", "bullets", false, top("flavour", `"mint"`)},

		// Deck level.
		{"template and template_path", "bullets", true, top("template_path", `"other.pptx"`)},
		{"no template", "bullets", true, func(deck, _ map[string]any) { delete(deck, "template") }},
		{"unknown template", "bullets", true, top("template", `"no-such-template"`)},
		{"template_path that is not there", "bullets", true, func(deck, _ map[string]any) {
			delete(deck, "template")
			deck["template_path"] = "no-such-template.pptx"
		}},
		{"no slides", "bullets", true, top("slides", `[]`)},
		{"structure and slides", "bullets", true, top("structure", `{"cover": {"title": "A"}, "sections": [{"title": "S", "slides": []}]}`)},
		{"grid config out of range", "bullets", true, top("grid", `{"columns": 99}`)},
		{"emoji in text", "bullets", true, func(_, target map[string]any) {
			content(target, 0)["text_value"] = "Revenue grew \U0001F680"
		}},
		{"hex colour in constrained mode", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"shape": {"geometry": "rect", "text": "a", "fill": "#FF0000"}}]}]}`)},
		{"unknown transition", "bullets", true, set("transition", `"teleport"`)},
		{"unknown background fit", "bullets", true, set("background", `{"color": "accent1", "fit": "squash"}`)},

		// Layout.
		{"unknown layout_id", "bullets", true, set("layout_id", `"slideLayout99"`)},
		{"no layout_id and no slide_type", "bullets", true, func(_, target map[string]any) { delete(target, "layout_id") }},
		{"pattern on a layout that cannot host it", "pattern", true, set("layout_id", `"title"`)},

		// Placeholder content.
		{"content without placeholder_id", "bullets", true, delContent("placeholder_id")},
		{"content without type", "bullets", true, delContent("type")},
		{"unknown content type", "bullets", true, setContent("type", `"poem"`)},
		{"unknown placeholder_id", "bullets", true, setContent("placeholder_id", `"sidebar"`)},
		{"value of another type", "bullets", true, setContent("type", `"text"`)},
		{"chart without type", "chart", true, setContent("chart_value", `{"data": {}}`)},
		{"chart of unknown type", "chart", true, setContent("chart_value", `{"type": "squiggle", "data": {"a": 1}}`)},
		{"diagram without type", "chart", true, retype("chart_value", "diagram", "diagram_value", `{"data": {}}`)},
		{"diagram data key the renderer does not read", "chart", true, retype("chart_value", "diagram", "diagram_value",
			`{"type": "timeline", "data": {"evnts": [{"label": "a", "date": "2025"}]}}`)},
		{"table row wider than its headers", "table", true, setContent("table_value",
			`{"headers": ["A", "B"], "rows": [["1", "2", "3"]]}`)},
		{"table style_id that is not a GUID", "table", true, setContent("table_value",
			`{"headers": ["A", "B"], "rows": [["1", "2"]], "style": {"style_id": "<nope>"}}`)},
		{"image without path", "bullets", true, retype("bullets_value", "image", "image_value", `{"alt": "x"}`)},
		{"image file that does not exist", "bullets", true, retype("bullets_value", "image", "image_value", `{"path": "no-such-file.png"}`)},

		// Pattern / compose / shape_grid.
		{"pattern and shape_grid", "pattern", true, set("shape_grid", `{"rows": [{"cells": [`+cellB+`]}]}`)},
		{"pattern and compose", "pattern", true, set("compose",
			`{"layout": "stack", "segments": [{"pattern": {"name": "kpi-2up", "values": [{"big": "1", "small": "a"}, {"big": "2", "small": "b"}]}}]}`)},
		{"unknown pattern", "pattern", true, set("pattern", `{"name": "no-such-pattern", "values": []}`)},
		{"pattern values of the wrong shape", "pattern", true, set("pattern", `{"name": "kpi-3up", "values": "nope"}`)},
		{"pattern with too few values", "pattern", true, set("pattern", `{"name": "kpi-3up", "values": [{"big": "1", "small": "a"}]}`)},
		{"pattern vertical_align outside its set", "pattern", true, set("pattern",
			`{"name": "kpi-3up", "vertical_align": "sideways", "values": `+kpis+`}`)},
		{"pattern override it does not have", "pattern", true, set("pattern",
			`{"name": "kpi-3up", "overrides": {"no_such": 1}, "values": `+kpis+`}`)},
		{"grid vertical_align outside its set", "grid", true, set("shape_grid",
			`{"vertical_align": "sideways", "rows": [{"cells": [{"shape": {"geometry": "rect", "text": "a"}}, `+cellB+`]}]}`)},
		{"grid cell with two payloads", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"shape": {"geometry": "rect", "text": "a"}, "table": {"headers": ["A"], "rows": [["1"]]}}, `+cellB+`]}]}`)},
		{"grid cell pattern next to a grid", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"pattern": {"name": "kpi-2up", "values": [{"big": "1", "small": "a"}, {"big": "2", "small": "b"}]}, "grid": {"rows": []}}, `+cellB+`]}]}`)},
		{"grid cell diagram without type", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"diagram": {"data": {}}}, `+cellB+`]}]}`)},
		{"grid cell icon that is not bundled", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"icon": {"name": "no-such-icon-anywhere"}}, `+cellB+`]}]}`)},
		{"grid columns that are not numbers", "grid", true, set("shape_grid",
			`{"columns": "wide", "rows": [{"cells": [{"shape": {"geometry": "rect", "text": "a"}}, `+cellB+`]}]}`)},
		{"grid with unknown geometry", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"shape": {"geometry": "dodecahedron", "text": "a"}}, `+cellB+`]}]}`)},

		// Overlays.
		{"overlay of unknown kind", "grid", true, set("overlays", `[{"kind": "squiggle"}]`)},
		{"overlay without kind", "grid", true, set("overlays", `[{"from": {"x_pct": 1, "y_pct": 1}, "to": {"x_pct": 9, "y_pct": 9}}]`)},
		{"overlay anchored to a cell that is not there", "grid", true, set("overlays",
			`[{"kind": "arrow", "from": {"anchor_cell": {"row": 9, "col": 9}}, "to": {"anchor_cell": {"row": 0, "col": 0}}}]`)},
		{"overlay arrow without an end", "grid", true, set("overlays",
			`[{"kind": "arrow", "from": {"anchor_cell": {"row": 0, "col": 0}}}]`)},
		{"overlay link on an arrow", "grid", true, set("overlays",
			`[{"kind": "arrow", "link": {"url": "https://example.com"}, "from": {"anchor_cell": {"row": 0, "col": 0}}, "to": {"anchor_cell": {"row": 0, "col": 1}}}]`)},
		{"overlay on a slide without a grid, anchored to a cell", "bullets", true, set("overlays",
			`[{"kind": "arrow", "from": {"anchor_cell": {"row": 0, "col": 0}}, "to": {"x_pct": 50, "y_pct": 50}}]`)},
		{"overlay badge without text", "grid", true, set("overlays", `[{"kind": "badge", "at": {"anchor_cell": {"row": 0, "col": 0}}}]`)},
		{"overlay callout without a target", "grid", true, set("overlays", `[{"kind": "callout", "text": "Look here"}]`)},

		// Links.
		{"content link to a slide that is not there", "bullets", true, setContent("link", `{"slide": 99}`)},
		{"content link with a URL and a slide", "bullets", true, setContent("link", `{"url": "https://example.com", "slide": 1}`)},
		{"content link that is not http(s)", "bullets", true, setContent("link", `{"url": "javascript:alert(1)"}`)},
		{"content link on a chart", "chart", true, setContent("link", `{"url": "https://example.com"}`)},
		{"source_link to a slide that is not there", "chart", true, set("source_link", `{"slide": 99}`)},
		{"shape link to a slide that is not there", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"shape": {"geometry": "rect", "text": "a", "link": {"slide": 99}}}, `+cellB+`]}]}`)},
		{"overlay badge link that is not http(s)", "grid", true, set("overlays",
			`[{"kind": "badge", "text": "1", "link": {"url": "ftp://example.com"}, "at": {"anchor_cell": {"row": 0, "col": 0}}}]`)},
		{"control: a link the deck can follow", "bullets", false, setContent("link", `{"slide": 1}`)},

		// More of each family: faults every surface refuses at the parser,
		// the enum table, the structure block and the grid resolver.
		{"unknown slide_type without a layout", "bullets", true, func(_, target map[string]any) {
			delete(target, "layout_id")
			target["slide_type"] = "poem"
		}},
		{"structure with a section that has no title", "bullets", true, func(deck, _ map[string]any) {
			delete(deck, "slides")
			deck["structure"] = verdictJSON(`{"sections": [{"slides": [{"layout_id": "content"}]}]}`)
		}},
		{"structure that expands to no slides", "bullets", true, func(deck, _ map[string]any) {
			delete(deck, "slides")
			deck["structure"] = verdictJSON(`{"sections": []}`)
		}},
		{"chart data of the wrong shape", "chart", true, setContent("chart_value", `{"type": "bar", "data": "nope"}`)},
		{"chart with no data", "chart", true, setContent("chart_value", `{"type": "bar", "data": {}}`)},
		{"image fit outside its set", "bullets", true, retype("bullets_value", "image", "image_value", `{"path": "pixel.png", "fit": "squash"}`)},
		{"background image that does not exist", "bullets", true, set("background", `{"image": "no-such.png"}`)},
		{"grid without rows", "grid", true, func(_, target map[string]any) {
			target["shape_grid"] = verdictJSON(`{"rows": []}`)
			delete(target, "overlays")
		}},
		{"grid row without cells", "grid", true, func(_, target map[string]any) {
			target["shape_grid"] = verdictJSON(`{"rows": [{"cells": []}]}`)
			delete(target, "overlays")
		}},
		{"grid cell spanning past the columns", "grid", true, set("shape_grid",
			`{"columns": 2, "rows": [{"cells": [{"col_span": 5, "shape": {"geometry": "rect", "text": "a"}}, `+cellB+`]}]}`)},
		{"grid cell table wider than its headers", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"table": {"headers": ["A"], "rows": [["1", "2"]]}}, `+cellB+`]}]}`)},
		{"grid row height that is not a number", "grid", true, set("shape_grid",
			`{"rows": [{"height": "tall", "cells": [{"shape": {"geometry": "rect", "text": "a"}}, `+cellB+`]}]}`)},
		{"grid cell with an unknown pattern", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"pattern": {"name": "no-such", "values": []}}, `+cellB+`]}]}`)},
		{"grid shape text of the wrong shape", "grid", true, set("shape_grid",
			`{"rows": [{"cells": [{"shape": {"geometry": "rect", "text": 12}}, `+cellB+`]}]}`)},
		{"compose with an unknown layout", "pattern", true, func(_, target map[string]any) {
			delete(target, "pattern")
			target["compose"] = verdictJSON(`{"layout": "spiral", "segments": [{"pattern": {"name": "kpi-2up", "values": [{"big": "1", "small": "a"}, {"big": "2", "small": "b"}]}}]}`)
		}},
		{"compose without segments", "pattern", true, func(_, target map[string]any) {
			delete(target, "pattern")
			target["compose"] = verdictJSON(`{"segments": []}`)
		}},
		{"pattern callout it does not support", "pattern", true, set("pattern",
			`{"name": "kpi-3up", "callout": {"text": "So what"}, "values": `+kpis+`}`)},
		{"pattern cell_overrides key that is not an index", "pattern", true, set("pattern",
			`{"name": "kpi-3up", "cell_overrides": {"x": {}}, "values": `+kpis+`}`)},
		{"theme_override colour that is not a colour", "bullets", true, top("theme_override", `{"colors": {"accent1": "notacolor"}}`)},
		{"unknown accent_strategy", "bullets", true, top("accent_strategy", `"sparkle"`)},
		{"unknown viewing_mode", "bullets", true, top("viewing_mode", `"squint"`)},
		{"unknown design_mode", "bullets", true, top("design_mode", `"wild"`)},
		{"unknown type_scale", "bullets", true, top("type_scale", `"huge"`)},
		{"unknown build", "bullets", true, set("build", `"explode"`)},
		{"unknown transition_speed", "bullets", true, set("transition_speed", `"warp"`)},
		{"column split layout out of range", "bullets", true, set("layout_id", `"two-column-99-1"`)},
		{"text_value of the wrong JSON type", "bullets", true, func(_, target map[string]any) { content(target, 0)["text_value"] = 12 }},
		{"legacy value of the wrong shape", "bullets", true, func(_, target map[string]any) {
			c := content(target, 1)
			delete(c, "bullets_value")
			c["value"] = "not a list"
		}},
		{"legacy chart value without type", "chart", true, func(_, target map[string]any) {
			c := content(target, 1)
			delete(c, "chart_value")
			c["value"] = verdictJSON(`{"data": {"a": 1}}`)
		}},
		{"control: a headline on a canvas", "grid", false, set("headline", `"A headline"`)},
		{"control: bullet_groups without groups", "bullets", false, retype("bullets_value", "bullet_groups", "bullet_groups_value", `{"groups": []}`)},
	}
}

// verdictCoreMutations are the cases the -short run keeps: the faults
// go-slide-creator-9k5fh was filed on and one of each refusal family. CI's
// race shards run -short; the integration corpus job runs every case.
var verdictCoreMutations = map[string]bool{
	"control: unchanged":                           true,
	"template and template_path":                   true,
	"template_path that is not there":              true,
	"no slides":                                    true,
	"structure and slides":                         true,
	"structure that expands to no slides":          true,
	"grid config out of range":                     true,
	"emoji in text":                                true,
	"hex colour in constrained mode":               true,
	"unknown layout_id":                            true,
	"pattern on a layout that cannot host it":      true,
	"content without placeholder_id":               true,
	"content without type":                         true,
	"unknown content type":                         true,
	"unknown placeholder_id":                       true,
	"chart without type":                           true,
	"chart of unknown type":                        true,
	"table row wider than its headers":             true,
	"image file that does not exist":               true,
	"pattern and shape_grid":                       true,
	"unknown pattern":                              true,
	"pattern values of the wrong shape":            true,
	"grid cell pattern next to a grid":             true,
	"grid cell icon that is not bundled":           true,
	"grid without rows":                            true,
	"overlay of unknown kind":                      true,
	"overlay anchored to a cell that is not there": true,
	"content link to a slide that is not there":    true,
}

// TestVerdictParityMutationCorpus applies each single fault to a valid deck
// and asserts that validate_input, generate --dry-run, generate and
// generate_presentation give one verdict for it, with the same error codes at
// the same paths.
func TestVerdictParityMutationCorpus(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()
	// An image the decks can name, so an image fault is the only fault.
	var pixel bytes.Buffer
	if err := png.Encode(&pixel, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), pixel.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	mutations := verdictMutations()
	names := map[string]bool{}
	for _, m := range mutations {
		names[m.name] = true
	}
	for name := range verdictCoreMutations {
		if !names[name] {
			t.Errorf("core case %q names no mutation", name)
		}
	}
	for i, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			if testing.Short() && !verdictCoreMutations[m.name] {
				t.Skip("-short runs the core cases; the integration corpus job runs all")
			}
			target := verdictJSON(verdictSlides[m.slide]).(map[string]any)
			deck := map[string]any{
				"template": "midnight-blue",
				"slides":   []any{verdictJSON(verdictCover), target},
			}
			m.apply(deck, target)
			data, err := json.Marshal(deck)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, fmt.Sprintf("deck-%02d.json", i))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true))
			t.Logf("verdict: %s", verdict)
			if verdict.Valid == m.invalid {
				t.Errorf("verdict = %s, want invalid=%t", verdict, m.invalid)
			}
			if !verdict.Valid && len(verdict.Errors) == 0 {
				t.Errorf("an invalid verdict names no error finding")
			}
		})
	}
}

// TestStructuralVerdictLeavesDeckUnchanged: validation reads the verdict by
// running generation's own conversion, and generation reads it before it
// renders; neither may rewrite the deck it was handed.
func TestStructuralVerdictLeavesDeckUnchanged(t *testing.T) {
	slides := []any{verdictJSON(verdictCover)}
	for _, name := range []string{"bullets", "chart", "table", "grid", "pattern"} {
		slides = append(slides, verdictJSON(verdictSlides[name]))
	}
	data, err := json.Marshal(map[string]any{"template": "midnight-blue", "accent_strategy": "rotate", "slides": slides})
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	mc := testMCPConfig(t)
	templatePath, cleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	analysis, err := getOrAnalyzeTemplate(templatePath, mc.cache)
	if err != nil {
		t.Fatal(err)
	}
	resolveCanonicalLayoutIDs(input.Slides, analysis.Layouts)
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	out := dryRunOutput{Valid: true}
	deckStructuralDiagnostics(&out, &input, analysis)
	if !out.Valid {
		t.Fatalf("the base deck is refused: %+v", out.Diagnostics)
	}
	if refusal := deckStructuralRefusal(&input, analysis.Layouts, analysis.Theme, analysis.Metadata, analysis.SlideWidth, analysis.SlideHeight, nil); refusal != nil {
		t.Fatalf("generation's side refuses the base deck: %v", refusal)
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the verdict rewrote the deck:\nbefore %s\nafter  %s", before, after)
	}
}

// TestCLIValidateResolvesTemplatePathAsGenerate: on the CLI a deck may name a
// template_path outside its own directory. generate renders it, so validate
// and generate --dry-run accept it (go-slide-creator-c8335).
func TestCLIValidateResolvesTemplatePathAsGenerate(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	deck := map[string]any{"template_path": templatePath, "slides": []any{verdictJSON(verdictCover)}}
	data, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "deck.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if res := validateJSONFile(path, "../../templates", "", false, ""); !res.Valid {
		t.Errorf("validate rejects a template_path generate renders: %v", res.Errors)
	}
	var dryErr error
	captureStdout(t, func() { dryErr = runJSONDryRun(path, "", "../../templates", "", "", false) })
	if dryErr != nil {
		t.Errorf("generate --dry-run rejects it: %v", dryErr)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	if err := runJSONMode(path, report, "../../templates", t.TempDir(), "", false, false, "", "warn", false, "strict", "", false); err != nil {
		t.Errorf("generate refuses it: %v", err)
	}
}

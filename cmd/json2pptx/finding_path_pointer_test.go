package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every finding a raw-deck surface reports addresses the deck the author sent
// with a JSON Pointer (RFC 6901): it starts with "/", its segments are
// separated by "/" alone, and it resolves in that deck
// (go-slide-creator-7fshg). That holds for a deck whose slides the engine
// expands (split_slide, structure: go-slide-creator-9564b, -2v8me) and for a
// finding about something the engine built or wrote (a cell of an expanded
// pattern or compose grid, slide chrome, a written shape:
// go-slide-creator-o45pn) — the engine's own locator is debug.locator. The
// one path that does not resolve names a field to add, and its parent does.

// missingFieldCodes are the findings about a field the deck does not have:
// the path names the field to add and its parent resolves.
var missingFieldCodes = map[string]bool{
	"required": true, "REQUIRED": true, "takeaway_missing": true, "DATA_WITHOUT_SOURCE": true,
}

// findingPathProblem returns what is wrong with one finding's path for deck,
// or "".
func findingPathProblem(deck any, code, path string) string {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "[]") {
		return "is not a JSON Pointer"
	}
	if pointerResolves(deck, path) {
		return ""
	}
	cut := strings.LastIndexByte(path, '/')
	parent, last := path[:cut], path[cut+1:]
	bare := code[strings.LastIndexByte(code, '.')+1:]
	// A field to add.
	if _, index := strconv.Atoi(last); index != nil && pointerResolves(deck, parent) {
		if missingFieldCodes[bare] || (bare == "INVALID_PARAMETER" && strings.HasSuffix(last, "_value")) {
			return ""
		}
	}
	return "does not resolve in the deck"
}

// renderedSlideCount is how many slides the deck renders, 0 when the test
// cannot tell (a structure deck's dividers and agenda).
func renderedSlideCount(deck any) int {
	doc, _ := deck.(map[string]any)
	if doc == nil || doc["structure"] != nil {
		return 0
	}
	slides, _ := doc["slides"].([]any)
	for _, s := range slides {
		if m, _ := s.(map[string]any); m != nil && m["type"] == "split_slide" {
			return 0
		}
	}
	return len(slides)
}

// assertFindingPointers checks every finding of one answer against the deck
// that produced it: the path resolves there, a finding on a slide says which
// rendered slide (slide_number), and a locator the engine kept is under debug.
func assertFindingPointers(t *testing.T, label string, deck any, findings []any) {
	t.Helper()
	slides := renderedSlideCount(deck)
	for _, raw := range findings {
		f, _ := raw.(map[string]any)
		if f == nil {
			continue
		}
		path, _ := f["path"].(string)
		if ev, ok := f["evidence"].(map[string]any); ok && path == "" {
			path, _ = ev["path"].(string)
		}
		// "" is the deck; "presentation" is the tool argument that carries a
		// deck the tool could not read.
		if path == "" || path == "presentation" {
			continue
		}
		code, _ := f["code"].(string)
		if problem := findingPathProblem(deck, code, path); problem != "" {
			t.Errorf("%s: %s path %q %s (debug %v)", label, code, path, problem, f["debug"])
		}
		onSlide := strings.HasPrefix(path, "/slides/") || strings.HasPrefix(path, "/structure/")
		number, hasNumber := f["slide_number"].(float64)
		switch {
		case strings.HasPrefix(path, "/slides/") && !hasNumber:
			t.Errorf("%s: %s at %q has no slide_number", label, code, path)
		case hasNumber && !onSlide:
			t.Errorf("%s: %s at %q has slide_number %v but names no slide", label, code, path, number)
		case hasNumber && (number < 1 || (slides > 0 && int(number) > slides)):
			t.Errorf("%s: %s at %q has slide_number %v, deck renders %d slides", label, code, path, number, slides)
		case hasNumber && slides > 0 && !strings.HasPrefix(path, fmt.Sprintf("/slides/%d", int(number)-1)):
			t.Errorf("%s: %s at %q has slide_number %v", label, code, path, number)
		}
		if debug, ok := f["debug"].(map[string]any); ok {
			if locator, _ := debug["locator"].(string); locator == path {
				t.Errorf("%s: %s debug.locator repeats the path %q", label, code, path)
			}
		}
	}
}

// answerFindings is every finding an answer carries: the findings envelope
// and the fit_findings list beside it.
func answerFindings(t *testing.T, doc map[string]any) []any {
	t.Helper()
	out := append([]any(nil), envelopeFindings(t, doc)...)
	fit, _ := doc["fit_findings"].([]any)
	return append(out, fit...)
}

func TestFindingPathProblem(t *testing.T) {
	deck := verdictJSON(`{"template": "t", "slides": [
	  {"content": [{"type": "table", "table_value": {"rows": [["a", {"content": "b"}]]}}]},
	  {"pattern": {"name": "kpi-3up", "values": []}},
	  {"compose": {"segments": []}}]}`)
	for _, tc := range []struct {
		code, path string
		ok         bool
	}{
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0/table_value/rows", true},
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0/table_value/rows/0/1/content", true},
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0.rows", false},
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0/rows", false},
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0/table_value/rows[0][1]", false},
		{"POLICY.no_emoji_violation", "slides[0].content[0].text_value", false},
		{"INPUT.REQUIRED", "template", false},
		{"INPUT.REQUIRED", "/template", true},
		{"INPUT.required", "/slides/0/layout_id", true},
		{"INPUT.max_length", "/slides/0/content/0/text", false},
		{"INPUT.INVALID_PARAMETER", "/slides/0/content/0/text_value", true},
		{"INPUT.takeaway_missing", "/slides/3/takeaway", false},
		// The engine's locators are not places in the deck.
		{"FIT.fit_overflow", "/slides/1/pattern/rows/0/cells/1/shape/text", false},
		{"FIT.fit_overflow", "/slides/1/pattern/values", true},
		{"INPUT.contrast_predicted", "/slides/1/chrome", false},
		{"INPUT.TEXT_BELOW_READABLE_MIN", "/slides/1/rendered_shapes/200/paragraphs/0", false},
		{"GRID.grid_violation", "/slides/2/shape_grid", false},
		{"GRID.grid_violation", "/slides/2/compose", true},
	} {
		if got := findingPathProblem(deck, tc.code, tc.path); (got == "") != tc.ok {
			t.Errorf("%s @ %s: problem %q, want ok=%t", tc.code, tc.path, got, tc.ok)
		}
	}
}

// faultyTableDeck carries one of each table fault validation reports, in a
// placeholder table, a grid-cell table and the deck defaults.
const faultyTableDeck = `{"template": "midnight-blue",
  "defaults": {"table_style": {"style_id": "not-a-guid"}},
  "slides": [
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three options were compared"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A", "B"],
        "style": {"style_id": "also-not-a-guid"},
        "rows": [["1", "2", "3"],
                 [{"content": "4", "conditional": {"rule": "bogus", "fill": "accent2"}},
                  {"content": "5", "conditional": {"rule": "gte", "threshold": "x", "fill": "#12"}}]]}}]},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "The default style applies here"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A", "B"], "rows": [["1", "2"]]}}]},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "And here"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A", "B"], "rows": [["1", "2"]]}}]},
    {"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "A table sits in a grid cell"}],
      "shape_grid": {"rows": [{"cells": [{"table": {"headers": ["A", "B"],
        "style": {"style_id": "nor-this"},
        "rows": [["1", "2", "3"], [{"content": "4", "conditional": {"rule": "bogus"}}, "5"]]}}]}]}}]}`

// faultyDefaultsDeck sets values the template owns in the deck defaults and
// in cells of its own, and misspells a key in each defaults block.
const faultyDefaultsDeck = `{"template": "midnight-blue", "viewing_mode": "sideways", "flavour": "x",
  "defaults": {"table_style": {"header_background": "#123456", "qq": 1}, "cell_style": {"fill": "#654321", "zz": 1}},
  "slides": [
    {"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Grid with hand-set values"}],
      "shape_grid": {"rows": [{"connector": {"color": "#111111"}, "cells": [
        {"shape": {"geometry": "rect", "fill": "#654321",
                   "text": {"content": "Plan", "size": 9, "paragraphs": [{"content": "p", "color": "#555555"}]}}},
        {"shape": {"geometry": "rect", "text": "No fill of its own"}},
        {"shape": {"geometry": "rect", "text": "Nor this one"}},
        {"table": {"headers": ["A", "B"], "rows": [["1", {"content": "2", "conditional": {"rule": "always", "fill": "#888888"}}]]}}]}]}},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "A diagram with its own colours"},
      {"placeholder_id": "body", "type": "diagram", "diagram_value": {"type": "pyramid", "style": {"colors": ["accent1", "#FF0000"]},
        "data": {"levels": [{"label": "A"}, {"label": "B"}, {"label": "C"}]}}}]}]}`

// faultyTextDeck carries the text-policy and icon faults the fit report
// reports: one string each, in a placeholder, a pattern value and a diagram.
const faultyTextDeck = `{"template": "midnight-blue", "slides": [
    {"layout_id": "title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Quarterly <blink>review</blink>"},
      {"placeholder_id": "subtitle", "type": "text", "text_value": "Results and outlook"}]},
    {"layout_id": "blank-title", "source": "Finance, FY25", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three numbers tell the story"}],
      "pattern": {"name": "kpi-3up", "values": [
        {"big": "12%", "small": "Growth <zz>q</zz>"}, {"big": "94", "small": "NPS"}, {"big": "3", "small": "Launches"}]}},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Two panels describe the plan"},
      {"placeholder_id": "body", "type": "diagram", "diagram_value": {"type": "panel_layout", "alt": "Two panels", "data": {"panels": [
        {"title": "One", "body": "Text", "icon": "no-such-icon-xyz"}, {"title": "Two", "body": "Text"}]}}}]}]}`

// TestFaultyDeckFindingPaths pins, family by family, where a fault is
// reported: at the JSON Pointer of the field the author wrote, once. A table
// fault sits under table_value for a placeholder table and under the cell's
// table for a grid table (go-slide-creator-7fshg, -dln5i); a value adopted
// from the deck defaults sits under /defaults (go-slide-creator-wj70r); a
// deck-level field is /field (go-slide-creator-yhi2e); a text-policy or icon
// fault is the string's own pointer (go-slide-creator-pcjob, -3sm9n).
func TestFaultyDeckFindingPaths(t *testing.T) {
	mc := testMCPConfig(t)
	for _, tc := range []struct {
		name      string
		deck      string
		fitReport bool
		want      []string // "CODE @ path", each reported exactly once
		never     []string // path prefixes no finding may have ...
		neverCode string   // ... when its code is this one ("" for any code)
	}{
		{name: "tables", deck: faultyTableDeck, want: []string{
			"INPUT.INVALID_PARAMETER @ /slides/0/content/1/table_value/rows",
			"INPUT.INVALID_PARAMETER @ /slides/0/content/1/table_value/style/style_id",
			"INPUT.INVALID_PARAMETER @ /slides/0/content/1/table_value/rows/1/0/conditional/rule",
			"INPUT.INVALID_PARAMETER @ /slides/0/content/1/table_value/rows/1/1/conditional/threshold",
			"INPUT.INVALID_PARAMETER @ /slides/0/content/1/table_value/rows/1/1/conditional/fill",
			"INPUT.INVALID_PARAMETER @ /defaults/table_style/style_id",
			"INPUT.INVALID_PARAMETER @ /slides/3/shape_grid/rows/0/cells/0/table/rows",
			"INPUT.INVALID_PARAMETER @ /slides/3/shape_grid/rows/0/cells/0/table/style/style_id",
			"INPUT.INVALID_PARAMETER @ /slides/3/shape_grid/rows/0/cells/0/table/rows/1/0/conditional/rule",
		}, never: []string{"/slides/1/content/1/table_value/style", "/slides/2/content/1/table_value/style"}},
		{name: "defaults and deck fields", deck: faultyDefaultsDeck, want: []string{
			"INPUT.UNKNOWN_ENUM @ /viewing_mode",
			"INPUT.unknown_key @ /flavour",
			"INPUT.unknown_key @ /defaults/table_style/qq",
			"INPUT.unknown_key @ /defaults/cell_style/zz",
			"INPUT.design_mode_violation @ /defaults/table_style/header_background",
			"INPUT.design_mode_violation @ /defaults/cell_style/fill",
			"INPUT.design_mode_violation @ /slides/0/shape_grid/rows/0/connector/color",
			"INPUT.design_mode_violation @ /slides/0/shape_grid/rows/0/cells/0/shape/fill",
			"INPUT.design_mode_violation @ /slides/0/shape_grid/rows/0/cells/0/shape/text/size",
			"INPUT.design_mode_violation @ /slides/0/shape_grid/rows/0/cells/0/shape/text/paragraphs/0/color",
			"INPUT.design_mode_violation @ /slides/0/shape_grid/rows/0/cells/3/table/rows/0/1/conditional/fill",
			"INPUT.design_mode_violation @ /slides/1/content/1/diagram_value/style/colors/1",
		}, never: []string{"/slides/0/shape_grid/rows/0/cells/1/shape/fill", "/slides/0/shape_grid/rows/0/cells/3/table/style"}},
		{name: "text and icons", deck: faultyTextDeck, fitReport: true, want: []string{
			"INPUT.UNSUPPORTED_INLINE_MARKUP @ /slides/0/content/0/text_value",
			"INPUT.UNSUPPORTED_INLINE_MARKUP @ /slides/1/pattern/values/0/small",
			"INPUT.ICON_BUNDLED_NAME_UNKNOWN @ /slides/2/content/1/diagram_value/data/panels/0/icon",
		}, never: []string{"/slides/1/pattern/rows", "/slides/1/shape_grid"}, neverCode: "INPUT.UNSUPPORTED_INLINE_MARKUP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := verdictJSON(tc.deck)
			res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": tc.fitReport}))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("answer is not JSON: %v", err)
			}
			findings := envelopeFindings(t, doc)
			assertFindingPointers(t, "validate_input", deck, findings)
			count := map[string]int{}
			var keys []string
			for _, raw := range findings {
				f, _ := raw.(map[string]any)
				ev, _ := f["evidence"].(map[string]any)
				path, _ := ev["path"].(string)
				key := fmt.Sprintf("%v @ %s", f["code"], path)
				count[key]++
				keys = append(keys, key)
				for _, prefix := range tc.never {
					if strings.HasPrefix(path, prefix) && (tc.neverCode == "" || f["code"] == tc.neverCode) {
						t.Errorf("%s: the author wrote nothing there", key)
					}
				}
			}
			for _, want := range tc.want {
				if count[want] != 1 {
					t.Errorf("%s reported %d times, want once", want, count[want])
				}
			}
			if t.Failed() {
				sort.Strings(keys)
				t.Logf("findings:\n  %s", strings.Join(keys, "\n  "))
			}
		})
	}
}

// TestValidateInputTemplatePathNamesWhatTheCallerWrote: a template the deck
// names is the deck's field; the tool's own template argument replaces it and
// is then the thing to correct.
func TestValidateInputTemplatePathNamesWhatTheCallerWrote(t *testing.T) {
	mc := testMCPConfig(t)
	deck := verdictJSON(`{"template": "no-such-template", "slides": [` + verdictCover + `]}`)
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"deck field", map[string]any{"presentation": deck, "fit_report": false}, "/template"},
		{"tool argument", map[string]any{"presentation": deck, "fit_report": false, "template": "nor-this-one"}, "template"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := mc.handleValidate(context.Background(), makeRequest(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("answer is not JSON: %v", err)
			}
			var paths []string
			for _, raw := range envelopeFindings(t, doc) {
				f, _ := raw.(map[string]any)
				if code, _ := f["code"].(string); strings.HasSuffix(code, "TEMPLATE_NOT_FOUND") {
					ev, _ := f["evidence"].(map[string]any)
					path, _ := ev["path"].(string)
					paths = append(paths, path)
				}
			}
			if len(paths) != 1 || paths[0] != tc.want {
				t.Errorf("TEMPLATE_NOT_FOUND at %q, want [%q]", paths, tc.want)
			}
		})
	}
}

// shortExampleDecks are the examples -short measures, each on the template
// that gives it the findings of one path family: expanded pattern cells,
// table cells, split_slide pages, slide chrome.
var shortExampleDecks = map[string]string{
	"patterns-smoke.json":             "midnight-blue",
	"table-style-demo.json":           "modern-template",
	"split-slide-vendor-matrix.json":  "midnight-blue",
	"basic-deck.json":                 "modern-template",
	"comparison-2col-connectors.json": "midnight-blue",
}

// TestExampleCorpusFindingPathsAreAuthoredPointers runs the fit report of the
// example decks and holds each finding path to the rule. The corpus job
// measures every deck on two templates; -short measures shortExampleDecks.
func TestExampleCorpusFindingPathsAreAuthoredPointers(t *testing.T) {
	mc := testMCPConfig(t)
	absDir, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(absDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	checked, decks := 0, 0
	for _, file := range files {
		templates := []string{"midnight-blue", "modern-template"}
		if testing.Short() {
			tmpl, ok := shortExampleDecks[filepath.Base(file)]
			if !ok {
				continue
			}
			templates = []string{tmpl}
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		decks++
		for _, tmpl := range templates {
			var deck map[string]any
			if err := json.Unmarshal(data, &deck); err != nil {
				t.Fatal(err)
			}
			deck["template"] = tmpl
			delete(deck, "template_path")
			res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
				"presentation": deck, "fit_report": true, "base_dir": absDir,
			}))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("%s: answer is not JSON: %v", file, err)
			}
			findings := envelopeFindings(t, doc)
			checked += len(findings)
			assertFindingPointers(t, filepath.Base(file)+" on "+tmpl, deck, findings)

			// The render reports the same deck's findings beside the file it
			// writes; the five family decks are rendered too.
			if shortExampleDecks[filepath.Base(file)] != tmpl {
				continue
			}
			res, err = mc.handleGenerate(context.Background(), makeRequest(map[string]any{
				"presentation": deck, "fit_report": true, "base_dir": absDir,
			}))
			if err != nil {
				t.Fatal(err)
			}
			doc = nil
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("%s: generate answer is not JSON: %v", file, err)
			}
			assertFindingPointers(t, filepath.Base(file)+" on "+tmpl+" (generate_presentation)", deck, answerFindings(t, doc))
		}
	}
	if testing.Short() && decks != len(shortExampleDecks) {
		t.Errorf("measured %d of the %d decks -short names: one was renamed or removed", decks, len(shortExampleDecks))
	}
	if checked == 0 {
		t.Error("the examples produced no findings: the check ran on nothing")
	}
}

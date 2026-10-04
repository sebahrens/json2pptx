package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// hexTable is one table with the two colours a table can set: a header
// background and a conditional cell fill.
const hexTable = `{"headers": ["A", "B"], "style": {"header_background": "#112233"},
  "rows": [["1", {"content": "2", "conditional": {"rule": "always", "fill": "#888888"}}]]}`

const hexDiagram = `{"type": "bar_chart", "data": {"categories": ["a", "b"], "series": [{"name": "s", "values": [1, 2]}]},
  "style": {"colors": ["accent1", "#00FF00"], "background": "#101010"}}`

const hexPattern = `{"name": "card-grid", "values": {"cells": [{"header": "One", "body": "First"}, {"header": "Two", "body": "Second"}]},
  "overrides": {"card_fill": "#FFF5ED"}}`

// designModeHostsDeck puts the same raw hex values in every position the deck
// format can hold them: a table in a placeholder, under the legacy "value"
// key, in a grid cell and in a nested sub-grid; a diagram likewise, and in a
// composite cell and a compose segment; a pattern override on a slide, in a
// grid cell and in a nested compose envelope; and a table that adopts the deck
// default in each of its two hosts.
const designModeHostsDeck = `{"template": "midnight-blue",
  "defaults": {"table_style": {"header_background": "#123456"}},
  "slides": [
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "A table in a placeholder"},
      {"placeholder_id": "body", "type": "table", "table_value": ` + hexTable + `}]},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "The same table under the legacy key"},
      {"placeholder_id": "body", "type": "table", "value": ` + hexTable + `}]},
    {"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "The same table in a grid"}],
      "shape_grid": {"rows": [{"cells": [
        {"table": ` + hexTable + `},
        {"grid": {"rows": [{"cells": [
          {"table": ` + hexTable + `},
          {"shape": {"geometry": "rect", "fill": "#FF0000", "text": {"content": "x", "size": 9}}}]}]}},
        {"composite": {"text": {"geometry": "rect", "fill": "#FF0000", "text": "42"}, "sub_diagram": ` + hexDiagram + `}},
        {"pattern": ` + hexPattern + `}]}]}},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "A legacy diagram and a legacy chart"},
      {"placeholder_id": "body", "type": "diagram", "value": ` + hexDiagram + `},
      {"placeholder_id": "body", "type": "chart", "value": {"type": "bar", "data": {"Q1": 4}, "style": {"colors": ["#00FF00"]}}}]},
    {"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "A compose envelope"}],
      "compose": {"direction": "horizontal", "segments": [
        {"diagram": ` + hexDiagram + `},
        {"compose": {"direction": "vertical", "segments": [{"pattern": ` + hexPattern + `}]}}]}},
    {"layout_id": "content", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Tables that adopt the default"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A"], "rows": [["1"]]}}],
      "shape_grid": {"rows": [{"cells": [{"table": {"headers": ["A"], "rows": [["1"]]}}]}]}}]}`

// TestDesignModeOneRuleWhereverTheValueSits: constrained mode refuses a raw
// hex colour by what it is, not by where it sits. A table in a placeholder
// answers to the rule a grid-cell table does (go-slide-creator-gpbjx), and so
// do the legacy "value" key, a nested sub-grid, a composite cell, a pattern
// nested in a cell and every compose segment. Each violation is the JSON
// Pointer of the field the author wrote; a value adopted from the deck
// defaults is reported there, once.
func TestDesignModeOneRuleWhereverTheValueSits(t *testing.T) {
	want := []string{
		"/defaults/table_style/header_background",
		"/slides/0/content/1/table_value/rows/0/1/conditional/fill",
		"/slides/0/content/1/table_value/style/header_background",
		"/slides/1/content/1/value/rows/0/1/conditional/fill",
		"/slides/1/content/1/value/style/header_background",
		"/slides/2/shape_grid/rows/0/cells/0/table/rows/0/1/conditional/fill",
		"/slides/2/shape_grid/rows/0/cells/0/table/style/header_background",
		"/slides/2/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/table/rows/0/1/conditional/fill",
		"/slides/2/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/table/style/header_background",
		"/slides/2/shape_grid/rows/0/cells/1/grid/rows/0/cells/1/shape/fill",
		"/slides/2/shape_grid/rows/0/cells/1/grid/rows/0/cells/1/shape/text/size",
		"/slides/2/shape_grid/rows/0/cells/2/composite/sub_diagram/style/background",
		"/slides/2/shape_grid/rows/0/cells/2/composite/sub_diagram/style/colors/1",
		"/slides/2/shape_grid/rows/0/cells/2/composite/text/fill",
		"/slides/2/shape_grid/rows/0/cells/3/pattern/overrides/card_fill",
		"/slides/3/content/1/value/style/background",
		"/slides/3/content/1/value/style/colors/1",
		"/slides/3/content/2/value/style/colors/0",
		"/slides/4/compose/segments/0/diagram/style/background",
		"/slides/4/compose/segments/0/diagram/style/colors/1",
		"/slides/4/compose/segments/1/compose/segments/0/pattern/overrides/card_fill",
	}

	violations := func(t *testing.T, deck string) []string {
		t.Helper()
		var input PresentationInput
		if err := json.Unmarshal([]byte(deck), &input); err != nil {
			t.Fatal(err)
		}
		applyDefaults(&input)
		var paths []string
		for _, f := range validateDesignMode(&input) {
			if f.Code != "design_mode_violation" || f.Action != "refuse" {
				t.Errorf("%s: code %q action %q, want a design_mode_violation refusal", f.Path, f.Code, f.Action)
			}
			paths = append(paths, f.Path)
		}
		sort.Strings(paths)
		return paths
	}

	t.Run("constrained", func(t *testing.T) {
		var raw any
		if err := json.Unmarshal([]byte(designModeHostsDeck), &raw); err != nil {
			t.Fatal(err)
		}
		got := violations(t, designModeHostsDeck)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("violations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		for _, path := range got {
			if !pointerResolves(raw, path) {
				t.Errorf("%s does not resolve in the deck", path)
			}
		}
	})
	t.Run("free", func(t *testing.T) {
		deck := strings.Replace(designModeHostsDeck, `{"template"`, `{"design_mode": "free", "template"`, 1)
		if got := violations(t, deck); len(got) != 0 {
			t.Errorf("free mode reports %v", got)
		}
	})
	t.Run("scheme colours", func(t *testing.T) {
		deck := designModeHostsDeck
		for _, hex := range []string{"#112233", "#888888", "#00FF00", "#101010", "#FFF5ED", "#123456", "#FF0000"} {
			deck = strings.ReplaceAll(deck, hex, "accent2")
		}
		deck = strings.Replace(deck, `, "size": 9`, "", 1)
		if got := violations(t, deck); len(got) != 0 {
			t.Errorf("a deck on scheme colours reports %v", got)
		}
	})
}

// TestDesignModeNestedGridInheritsExpanderSizes: the sizes of a pattern
// expansion are the engine's at every depth of the grid it produced, and an
// author's own nested grid gets no such waiver.
func TestDesignModeNestedGridInheritsExpanderSizes(t *testing.T) {
	const nested = `{"rows": [{"cells": [{"grid": {"rows": [{"cells": [
      {"shape": {"geometry": "rect", "text": {"content": "12%", "size": 40}}}]}]}}]}]`
	for _, tc := range []struct {
		name, grid string
		want       int
	}{
		{"expansion", nested + `, "source": "pattern:kpi-3up"}`, 0},
		{"authored", nested + `}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var grid ShapeGridInput
			if err := json.Unmarshal([]byte(tc.grid), &grid); err != nil {
				t.Fatal(err)
			}
			got := validateSlideDesignMode(&SlideInput{ShapeGrid: &grid}, 1, nil)
			if len(got) != tc.want {
				t.Errorf("%d violation(s) %v, want %d", len(got), got, tc.want)
			}
		})
	}
}

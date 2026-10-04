package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// go-slide-creator-xxeal: validate predicts against the colours the deck
// renders in. The fit collector is handed the template's theme; the deck's
// theme_override is applied to it there, as generation applies it. Before,
// white text on an accent the deck overrides to pale yellow drew no
// contrast_predicted, and generation swapped it.
func TestValidateAppliesThemeOverride(t *testing.T) {
	const deck = `{
		"template": "midnight-blue",
		"theme_override": {"colors": {"accent1": "#FFE680"}},
		"slides": [{"slide_type": "content",
			"content": [{"placeholder_id": "title", "type": "text", "text_value": "Override"}],
			"shape_grid": {"columns": 2, "rows": [{"cells": [
				{"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "North region grew fastest", "color": "lt1"}}},
				{"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "South region held flat", "color": "lt1"}}}
			]}]}}]
	}`
	layouts, theme, width, height := fitReportGeometry("midnight-blue", testutil.TemplatesDir())
	if theme == nil {
		t.Fatal("cannot analyze midnight-blue")
	}
	predictions := func(mutate func(*PresentationInput)) []string {
		var input PresentationInput
		if err := json.Unmarshal([]byte(deck), &input); err != nil {
			t.Fatal(err)
		}
		mutate(&input)
		applyDefaults(&input)
		resolveCanonicalLayoutIDs(input.Slides, layouts)
		var out []string
		for _, f := range contrastPredictions(collectFitFindings(&input, layouts, width, height, theme)) {
			out = append(out, f.Message)
		}
		return out
	}

	overridden := predictions(func(*PresentationInput) {})
	if len(overridden) == 0 || !strings.Contains(strings.ToUpper(strings.Join(overridden, "\n")), "ON #FFE680") {
		t.Errorf("no contrast_predicted against the overridden accent #FFE680: %q", overridden)
	}
	if plain := predictions(func(in *PresentationInput) { in.ThemeOverride = nil }); len(plain) != 0 {
		t.Errorf("without the override the template accent carries white text; got %q", plain)
	}
	for _, c := range theme.Colors {
		if c.Name == "accent1" && strings.EqualFold(c.RGB, "#FFE680") {
			t.Error("the caller's theme was overridden in place")
		}
	}
}

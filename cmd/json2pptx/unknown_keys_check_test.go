package main

import (
	"encoding/json"
	"testing"
)

// go-slide-creator-z72f: chart_value.style and chart_value.chart_style were not
// walked, so a deck that set style.palette (svggen's name for what the engine
// calls colors), style.font_size (inert) or style.show_grid got silence even
// under --strict-unknown-keys, and the agent had no way to learn that the knob
// it reached for does not exist.
func TestUnknownKeys_ChartStyleBlocks(t *testing.T) {
	raw := json.RawMessage(`{"template":"t","slides":[{"slide_type":"chart","content":[
		{"placeholder_id":"body","type":"chart","chart_value":{"type":"bar",
			"data":{"categories":["Q1"],"series":[{"name":"Rev","values":[1]}]},
			"style":{"show_values":true,"palette":"vibrant","font_size":18,"show_grid":false},
			"chart_style":{"show_single_series_legend":true,"show_gridlines":true}}}]}]}`)

	got := map[string]bool{}
	for _, w := range checkInputUnknownKeys(raw) {
		got[w.Path] = true
	}
	for _, want := range []string{
		"/slides/0/content/0/chart_value/style/palette",
		"/slides/0/content/0/chart_value/style/font_size",
		"/slides/0/content/0/chart_value/style/show_grid",
		"/slides/0/content/0/chart_value/chart_style/show_gridlines",
	} {
		if !got[want] {
			t.Errorf("unknown key %s not reported; got %v", want, got)
		}
	}
	// The keys that work must not be reported.
	for _, ok := range []string{
		"/slides/0/content/0/chart_value/style/show_values",
		"/slides/0/content/0/chart_value/chart_style/show_single_series_legend",
	} {
		if got[ok] {
			t.Errorf("working key %s reported as unknown", ok)
		}
	}
}

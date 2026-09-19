package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// Two agent-facing surfaces describe the same payload — get_capabilities'
// chart/diagram capability entries and get_data_format_hints — and they were
// written independently, so they disagreed. timeline's capability said
// required_fields ["values"] while the hints said "events"; the hints were
// right, and an agent that trusted the capability built a payload the renderer
// dropped. stat_cards / icon_columns / icon_rows had no hints entry at all, so
// their shape had to be guessed from optional_fields
// (go-slide-creator-umji).
func TestEveryVisualTypeHasAgreeingMetadata(t *testing.T) {
	hints := buildDataFormatHints()

	type entry struct {
		kind string // "chart" or "diagram"
		// required is the capability's own required_fields. Only diagram
		// capabilities carry them; a chart's payload shape is described by the
		// hints alone, so there is nothing to disagree with.
		required    []string
		hasRequired bool
	}
	declared := map[string]entry{}
	for _, c := range svggen.ChartCapabilities() {
		if c.Status != "ready" {
			continue
		}
		declared[c.Type] = entry{kind: "chart"}
	}
	for _, d := range svggen.DiagramCapabilitiesReady() {
		declared[d.Type] = entry{kind: "diagram", required: d.RequiredFields, hasRequired: true}
	}
	if len(declared) == 0 {
		t.Fatal("no ready chart or diagram capabilities")
	}

	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cap := declared[name]
		t.Run(name, func(t *testing.T) {
			hint, ok := hints[name]
			if !ok {
				t.Fatalf("%s %q is advertised in get_capabilities but has no data_format_hints entry, "+
					"so its payload shape has to be guessed", cap.kind, name)
			}
			if cap.hasRequired && !sameKeySet(cap.required, hint.RequiredKeys) {
				t.Errorf("required fields disagree for %s %q: capability says %v, data_format_hints says %v — "+
					"an agent that trusts one builds a payload the other rejects",
					cap.kind, name, cap.required, hint.RequiredKeys)
			}
		})
	}
}

// Every hints entry must describe something the engine advertises: a hint for a
// type nobody can use is a different kind of lie.
func TestEveryHintDescribesAnAdvertisedType(t *testing.T) {
	advertised := map[string]bool{}
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			advertised[c.Type] = true
		}
	}
	for _, d := range svggen.DiagramCapabilitiesReady() {
		advertised[d.Type] = true
	}
	for name := range buildDataFormatHints() {
		if !advertised[name] {
			t.Errorf("data_format_hints describes %q, which is not in chart_types or diagram_types", name)
		}
	}
}

// sameKeySet compares two key lists ignoring order and empty/nil difference.
func sameKeySet(a, b []string) bool {
	norm := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, s := range in {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	}
	x, y := norm(a), norm(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

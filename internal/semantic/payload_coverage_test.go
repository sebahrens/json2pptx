package semantic

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// This file holds the per-kind payload-field round-trip coverage gate
// (go-slide-creator-s4hs). TestExplainCompileParity proves each kind compiles
// to the pattern explain advertises, but it does NOT prove every *documented*
// payload field actually reaches the rendered output — and that gap is what let
// fields ship silently dropped (the 2.1/2.2/2.3 field-test regressions). This
// gate closes it: for every documented field of every kind it injects a unique
// sentinel and asserts the sentinel survives into the compiled deckinput slide,
// and it fails the build if the kind registry documents a field that has no
// round-trip probe — so a newly documented field cannot ship without coverage.

// fieldProbe exercises one documented per-kind payload field. inject returns an
// otherwise-minimal, compile-valid body for the kind with sentinel placed in
// the field under test (and nowhere else). rendered records whether that
// content is expected to survive into the compiled slide; a documented field
// with rendered=false is one the compiler intentionally does not emit, and why
// records the deliberate reason so the exemption is a record, not an accident.
type fieldProbe struct {
	inject   func(sentinel string) map[string]any
	rendered bool
	why      string
}

// covChart builds a minimal typed, data-bearing chart payload (the shape
// CompileChartInsight requires to emit the chart panel), optionally carrying a
// sentinel as the chart title.
func covChart(title string) map[string]any {
	c := map[string]any{
		"type": "bar_chart",
		"data": map[string]any{
			"categories": []any{"Q1"},
			"series":     []any{map[string]any{"name": "S", "values": []any{1}}},
		},
	}
	if title != "" {
		c["title"] = title
	}
	return c
}

// covKPIs builds three KPI cells (a count the kpi-Nup family supports), placing
// firstBig in the first cell's value.
func covKPIs(firstBig string) []any {
	return []any{
		map[string]any{"value": firstBig, "label": "ARR"},
		map[string]any{"value": "118%", "label": "NRR"},
		map[string]any{"value": "41d", "label": "Cycle"},
	}
}

// covColumns builds two balanced comparison columns (the shape comparison-2col
// requires), placing leftItem as the left column's single item.
func covColumns(leftItem string) []any {
	return []any{
		map[string]any{"title": "L", "items": []any{leftItem}},
		map[string]any{"title": "R", "items": []any{"Right"}},
	}
}

// covSteps builds three process steps (the count process-flow requires),
// placing first as the first step's label.
func covSteps(first string) []any {
	return []any{first, "Build", "Review"}
}

// covPhases builds three roadmap phases (the count phase-roadmap requires),
// placing firstName as the first phase's name.
func covPhases(firstName string) []any {
	return []any{
		map[string]any{"name": firstName, "date_label": "Q1", "description": "Prove value"},
		map[string]any{"name": "Expand", "date_label": "Q2"},
		map[string]any{"name": "GA", "date_label": "Q3"},
	}
}

// payloadFieldCoverage maps every slide kind to a probe for each of its
// documented payload fields (RequiredFields ∪ TypicalFields from the kind
// registry). TestSemanticPayloadFieldCoverage asserts this table stays in exact
// sync with the registry, so a documented field without a round-trip probe
// fails the build.
var payloadFieldCoverage = map[SlideKind]map[string]fieldProbe{
	KindTitle: {
		"title":    {inject: func(s string) map[string]any { return map[string]any{"title": s} }, rendered: true},
		"subtitle": {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "subtitle": s} }, rendered: true},
		"eyebrow":  {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "eyebrow": s} }, rendered: true},
	},
	KindSection: {
		"title": {inject: func(s string) map[string]any { return map[string]any{"title": s} }, rendered: true},
		"subtitle": {
			inject:   func(s string) map[string]any { return map[string]any{"title": "Filler", "subtitle": s} },
			rendered: false,
			why:      "section dividers reserve body placeholders for decorative section numbers; CompileSection (slides/structural.go) intentionally does not emit subtitle",
		},
	},
	KindExecutiveSummary: {
		"title":     {inject: func(s string) map[string]any { return map[string]any{"title": s} }, rendered: true},
		"points":    {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "points": []any{s}} }, rendered: true},
		"takeaways": {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "takeaways": []any{s}} }, rendered: true},
		"takeaway":  {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "takeaway": s} }, rendered: true},
	},
	KindKPISnapshot: {
		"kpis":     {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs("$48M"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs("$48M"), "takeaway": s} }, rendered: true},
	},
	KindChartInsight: {
		"chart":    {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(s), "insights": []any{"Grew 41%"}} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "title": s} }, rendered: true},
		"insights": {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(""), "insights": []any{s}} }, rendered: true},
		"source":   {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "source": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "takeaway": s} }, rendered: true},
	},
	KindComparison: {
		"columns":  {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns("Left"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns("Left"), "takeaway": s} }, rendered: true},
	},
	KindProcess: {
		"steps":    {inject: func(s string) map[string]any { return map[string]any{"steps": covSteps(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"steps": covSteps("Discover"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"steps": covSteps("Discover"), "takeaway": s} }, rendered: true},
	},
	KindRoadmap: {
		"phases":   {inject: func(s string) map[string]any { return map[string]any{"phases": covPhases(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"phases": covPhases("Pilot"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"phases": covPhases("Pilot"), "takeaway": s} }, rendered: true},
	},
	KindDecision: {
		"title":          {inject: func(s string) map[string]any { return map[string]any{"title": s} }, rendered: true},
		"options":        {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "options": []any{s}} }, rendered: true},
		"recommendation": {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "recommendation": s} }, rendered: true},
		"takeaway":       {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "takeaway": s} }, rendered: true},
	},
	KindClosing: {
		"title":    {inject: func(s string) map[string]any { return map[string]any{"title": s} }, rendered: true},
		"subtitle": {inject: func(s string) map[string]any { return map[string]any{"title": "Filler", "subtitle": s} }, rendered: true},
	},
	KindRawJSON2pptx: {
		"slide": {inject: func(s string) map[string]any {
			return map[string]any{"slide": map[string]any{
				"slide_type": "title",
				"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": s}},
			}}
		}, rendered: true},
	},
}

// documentedFields returns the deduplicated, sorted union of a kind's required
// and typical payload fields — the set every probe table must cover.
func documentedFields(info KindInfo) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range append(append([]string{}, info.RequiredFields...), info.TypicalFields...) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// TestSemanticPayloadFieldCoverage is the round-trip coverage gate: for every
// documented per-kind payload field it injects a unique sentinel and asserts the
// sentinel reaches (or, for intentional drops, stays out of) the compiled
// deckinput slide. It also asserts the probe table is in exact sync with the
// kind registry, so a newly documented field without a probe fails the build.
func TestSemanticPayloadFieldCoverage(t *testing.T) {
	for _, kind := range AllSlideKinds() {
		info, ok := LookupKind(kind)
		if !ok {
			t.Errorf("kind %q is not registered", kind)
			continue
		}
		documented := documentedFields(info)
		probes, ok := payloadFieldCoverage[kind]
		if !ok {
			t.Errorf("kind %q has no payload-field coverage probes; add an entry to payloadFieldCoverage covering fields %v", kind, documented)
			continue
		}

		// Sync gate: probes must cover exactly the documented fields — no missing
		// (uncovered field), no extra (probe for an undocumented field).
		docSet := map[string]bool{}
		for _, f := range documented {
			docSet[f] = true
			if _, has := probes[f]; !has {
				t.Errorf("%s: documented field %q has no coverage probe; add it to payloadFieldCoverage", kind, f)
			}
		}
		for f := range probes {
			if !docSet[f] {
				t.Errorf("%s: coverage probe for %q is not a documented field (RequiredFields/TypicalFields); remove the probe or document the field", kind, f)
			}
		}

		for field, probe := range probes {
			if !docSet[field] {
				continue // already reported as an extra probe above
			}
			t.Run(string(kind)+"/"+field, func(t *testing.T) {
				sentinel := "ZQSENTINEL_" + string(kind) + "_" + field
				spec := &DeckSpec{
					Meta:   DeckMeta{Title: "Coverage Deck", Template: "midnight-blue"},
					Slides: []SlideSpec{{Kind: kind, Body: probe.inject(sentinel)}},
				}
				input, _, err := Compile(spec, CompileOptions{})
				if err != nil {
					t.Fatalf("compile %s with field %q populated: %v", kind, field, err)
				}
				if len(input.Slides) != 1 {
					t.Fatalf("expected exactly 1 emitted slide, got %d", len(input.Slides))
				}
				encoded, err := json.Marshal(input.Slides[0])
				if err != nil {
					t.Fatalf("marshal compiled slide: %v", err)
				}
				present := strings.Contains(string(encoded), sentinel)
				switch {
				case probe.rendered && !present:
					t.Errorf("documented field %q did not reach the compiled slide (silent drop)\ncompiled slide: %s", field, encoded)
				case !probe.rendered && present:
					t.Errorf("field %q is marked as intentionally not rendered (%s) but its content reached the compiled slide; update the probe or the compiler\ncompiled slide: %s",
						field, probe.why, encoded)
				}
			})
		}
	}
}

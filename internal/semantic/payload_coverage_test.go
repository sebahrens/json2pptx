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

// covTiers builds three architecture tiers (the count arch-stack requires),
// placing firstLabel on the first tier and exercising the items->detail join.
// covMembers builds three team members (inside team-bios' card limit), the
// first carrying the probed text.
func covMembers(firstName string) []any {
	return []any{
		map[string]any{"name": firstName, "role": "Engagement partner", "bio": "Led the settlement migration."},
		map[string]any{"name": "Jonas Weber", "role": "Delivery lead", "bio": "Runs the cutover rehearsals."},
		map[string]any{"name": "Priya Raman", "role": "Data lead", "bio": "Owns the reconciliation model."},
	}
}

// covSections builds four agenda sections (inside both agenda patterns'
// counts), the first carrying the probed text.
func covSections(firstTitle string) []any {
	return []any{
		map[string]any{"title": firstTitle, "subtitle": "Q3 against the plan"},
		map[string]any{"title": "What we found", "subtitle": "Three findings"},
		map[string]any{"title": "What we recommend", "subtitle": "The decision"},
		map[string]any{"title": "What happens next", "subtitle": "First ninety days"},
	}
}

func covTiers(firstLabel string) []any {
	return []any{
		map[string]any{"label": firstLabel, "items": []any{"Web console", "Mobile"}},
		map[string]any{"label": "Services", "description": "Orders, pricing"},
		map[string]any{"label": "Platform", "description": "Kubernetes"},
	}
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
		// go-slide-creator-ku6t: the ask rides the pattern's bottom-line bar, and
		// the bullet fallback appends it rather than dropping it.
		"bottom_line": {inject: func(s string) map[string]any {
			return map[string]any{"title": "Filler", "points": []any{"a", "b", "c"}, "bottom_line": s}
		}, rendered: true},
	},
	// go-slide-creator-6o1r: every documented option_matrix field must reach the
	// rendered slide — a criterion label, an option name and detail, a score, the
	// highlight badge, and the two fields that resolve a highlight by name.
	KindOptionMatrix: {
		"title": {inject: func(s string) map[string]any { return covOptionMatrix(map[string]any{"title": s}) }, rendered: true},
		"criteria": {inject: func(s string) map[string]any {
			return covOptionMatrix(map[string]any{"criteria": []any{s, "Payback"}})
		}, rendered: true},
		"options": {inject: func(s string) map[string]any {
			return covOptionMatrix(map[string]any{"options": []any{
				map[string]any{"name": s, "scores": []any{1, 2}},
				map[string]any{"name": "Hub", "scores": []any{3, 4}},
			}})
		}, rendered: true},
		"scale": {inject: func(string) map[string]any { return covOptionMatrix(map[string]any{"scale": "harvey"}) }},
		"recommended": {inject: func(string) map[string]any {
			return covOptionMatrix(map[string]any{"recommended": "Hub"})
		}},
		"decisive_criterion": {inject: func(string) map[string]any {
			return covOptionMatrix(map[string]any{"decisive_criterion": "Payback"})
		}},
		// The harness sentinel is 39 chars, past table-highlight's 24-char badge
		// budget, so this probe exercises the drop rule rather than the badge.
		// TestOptionMatrixKeepsBadgeWithinBudget covers a badge that fits.
		"highlight_label": {inject: func(s string) map[string]any {
			return covOptionMatrix(map[string]any{"recommended": "Hub", "highlight_label": s})
		}, why: "a badge over the pattern's 24-char budget is dropped rather than costing the whole matrix; validation warns"},
		"takeaway": {inject: func(s string) map[string]any { return covOptionMatrix(map[string]any{"takeaway": s}) }, rendered: true},
	},
	// go-slide-creator-e4h1: a header, a cell, an alignment and the two emphasis
	// fields all have to reach the compiled table.
	KindTable: {
		"title":   {inject: func(s string) map[string]any { return covTable(map[string]any{"title": s}) }, rendered: true},
		"headers": {inject: func(s string) map[string]any { return covTable(map[string]any{"headers": []any{s, "FY26"}}) }, rendered: true},
		"rows": {inject: func(s string) map[string]any {
			return covTable(map[string]any{"rows": []any{[]any{s, "1"}, []any{"SMB", "2"}}})
		}, rendered: true},
		"column_alignments": {inject: func(string) map[string]any {
			return covTable(map[string]any{"column_alignments": []any{"left", "right"}})
		}},
		"highlight_column": {inject: func(string) map[string]any {
			return covTable(map[string]any{"highlight_column": "FY26"})
		}},
		"totals_row": {inject: func(string) map[string]any { return covTable(map[string]any{"totals_row": true}) }},
		"takeaway":   {inject: func(s string) map[string]any { return covTable(map[string]any{"takeaway": s}) }, rendered: true},
	},
	KindKPISnapshot: {
		"kpis":     {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs("$48M"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"kpis": covKPIs("$48M"), "takeaway": s} }, rendered: true},
	},
	KindChartInsight: {
		"chart": {inject: func(s string) map[string]any {
			return map[string]any{"chart": covChart(s), "insights": []any{"Grew 41%"}}
		}, rendered: true},
		"title": {inject: func(s string) map[string]any {
			return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "title": s}
		}, rendered: true},
		"insights": {inject: func(s string) map[string]any { return map[string]any{"chart": covChart(""), "insights": []any{s}} }, rendered: true},
		"source": {inject: func(s string) map[string]any {
			return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "source": s}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any {
			return map[string]any{"chart": covChart(""), "insights": []any{"Grew 41%"}, "takeaway": s}
		}, rendered: true},
	},
	KindComparison: {
		"columns":  {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns(s)} }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns("Left"), "title": s} }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"columns": covColumns("Left"), "takeaway": s} }, rendered: true},
	},
	// go-slide-creator-162os: a tier's label, its items (joined into the detail
	// line) and the cross-cutting rails all have to reach the rendered stack.
	KindArchitecture: {
		"tiers": {inject: func(s string) map[string]any { return map[string]any{"tiers": covTiers(s)} }, rendered: true},
		"title": {inject: func(s string) map[string]any { return map[string]any{"tiers": covTiers("Experience"), "title": s} }, rendered: true},
		"rails": {inject: func(s string) map[string]any {
			return map[string]any{"tiers": covTiers("Experience"), "rails": []any{s}}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return map[string]any{"tiers": covTiers("Experience"), "takeaway": s} }, rendered: true},
	},
	// go-slide-creator-3rvk: a section's title and its subtitle both have to
	// reach the rendered agenda, and the current-section marker with them.
	KindAgenda: {
		"sections": {inject: func(s string) map[string]any { return map[string]any{"sections": covSections(s)} }, rendered: true},
		"title": {inject: func(s string) map[string]any {
			return map[string]any{"sections": covSections("Where we are"), "title": s}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any {
			return map[string]any{"sections": covSections("Where we are"), "takeaway": s}
		}, rendered: true},
		// current selects a row rather than contributing text of its own, so it
		// is checked for reaching the slide, not for its literal value.
		"current": {inject: func(string) map[string]any {
			return map[string]any{"sections": covSections("Where we are"), "current": 2.0}
		}, rendered: false},
	},
	KindQuote: {
		"quotes": {inject: func(s string) map[string]any {
			return map[string]any{"quotes": []any{map[string]any{"text": s, "name": "J. Lin"}}}
		}, rendered: true},
		"title": {inject: func(s string) map[string]any {
			return map[string]any{"quote": "Cycle time fell", "attribution": "J. Lin", "title": s}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any {
			return map[string]any{"quote": "Cycle time fell", "attribution": "J. Lin", "takeaway": s}
		}, rendered: true},
		"attribution": {inject: func(s string) map[string]any {
			return map[string]any{"quote": "Cycle time fell", "attribution": s}
		}, rendered: true},
		"role": {inject: func(s string) map[string]any {
			return map[string]any{"quote": "Cycle time fell", "attribution": "J. Lin", "role": s}
		}, rendered: true},
	},
	KindBridge: {
		"columns": {inject: func(s string) map[string]any {
			return map[string]any{"columns": []any{
				map[string]any{"label": s, "type": "total", "value": 120},
				map[string]any{"label": "COGS", "type": "delta", "value": -45},
				map[string]any{"label": "EBITDA", "type": "subtotal"},
			}}
		}, rendered: true},
		"title":    {inject: func(s string) map[string]any { b := bridgeCoverageBody(); b["title"] = s; return b }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { b := bridgeCoverageBody(); b["takeaway"] = s; return b }, rendered: true},
		"unit":     {inject: func(s string) map[string]any { b := bridgeCoverageBody(); b["unit"] = s; return b }, rendered: true},
		"caption":  {inject: func(s string) map[string]any { b := bridgeCoverageBody(); b["caption"] = s; return b }, rendered: true},
	},
	KindPillars: {
		"pillars": {inject: func(s string) map[string]any {
			b := pillarsCoverageBody()
			b["pillars"].([]any)[0].(map[string]any)["title"] = s
			return b
		}, rendered: true},
		"title":    {inject: func(s string) map[string]any { b := pillarsCoverageBody(); b["title"] = s; return b }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { b := pillarsCoverageBody(); b["takeaway"] = s; return b }, rendered: true},
		"objective": {inject: func(s string) map[string]any {
			b := pillarsCoverageBody()
			b["objective"] = s
			b["foundation"] = "Data"
			return b
		}, rendered: true},
		"foundation": {inject: func(s string) map[string]any {
			b := pillarsCoverageBody()
			b["objective"] = "Grow trust"
			b["foundation"] = s
			return b
		}, rendered: true},
		"roof_badges": {inject: func(s string) map[string]any {
			b := pillarsCoverageBody()
			b["objective"] = "Grow trust"
			b["foundation"] = "Data"
			b["roof_badges"] = []any{s}
			return b
		}, rendered: true},
	},
	// go-slide-creator-13lj: a person's name, role and bio all have to reach
	// the rendered card.
	KindTeam: {
		"members": {inject: func(s string) map[string]any { return map[string]any{"members": covMembers(s)} }, rendered: true},
		"title": {inject: func(s string) map[string]any {
			return map[string]any{"members": covMembers("Amara Okafor"), "title": s}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any {
			return map[string]any{"members": covMembers("Amara Okafor"), "takeaway": s}
		}, rendered: true},
	},
	// go-slide-creator-2hkc: the number, the words beneath it, and the context
	// and source around them all have to reach the rendered slide. The gate's
	// sentinels are longer than the hero's own text budgets, so these probes
	// travel the degrade path — which is exactly where a field is most likely to
	// be dropped on the floor.
	KindStat: {
		"value":    {inject: func(s string) map[string]any { return covStat(map[string]any{"value": s}) }, rendered: true},
		"label":    {inject: func(s string) map[string]any { return covStat(map[string]any{"label": s}) }, rendered: true},
		"unit":     {inject: func(s string) map[string]any { return covStat(map[string]any{"unit": s}) }, rendered: true},
		"context":  {inject: func(s string) map[string]any { return covStat(map[string]any{"context": s}) }, rendered: true},
		"source":   {inject: func(s string) map[string]any { return covStat(map[string]any{"source": s}) }, rendered: true},
		"title":    {inject: func(s string) map[string]any { return covStat(map[string]any{"title": s}) }, rendered: true},
		"takeaway": {inject: func(s string) map[string]any { return covStat(map[string]any{"takeaway": s}) }, rendered: true},
	},
	// go-slide-creator-wrsb: a milestone's label, its date and its body all have
	// to reach the rendered line.
	KindTimeline: {
		"milestones": {inject: func(s string) map[string]any { return map[string]any{"milestones": covMilestones(s)} }, rendered: true},
		"title": {inject: func(s string) map[string]any {
			return map[string]any{"milestones": covMilestones("Mandate published"), "title": s}
		}, rendered: true},
		"takeaway": {inject: func(s string) map[string]any {
			return map[string]any{"milestones": covMilestones("Mandate published"), "takeaway": s}
		}, rendered: true},
	},
	// go-slide-creator-ykjh: both axes, both axis ends and every quadrant's
	// header and body have to reach the rendered 2x2.
	KindMatrix2x2: {
		"quadrants": {inject: func(s string) map[string]any { return covMatrix(map[string]any{"quadrants": covQuadrants(s)}) }, rendered: true},
		"x_axis":    {inject: func(s string) map[string]any { return covMatrix(map[string]any{"x_axis": s}) }, rendered: true},
		"y_axis":    {inject: func(s string) map[string]any { return covMatrix(map[string]any{"y_axis": s}) }, rendered: true},
		"x_low":     {inject: func(s string) map[string]any { return covMatrix(map[string]any{"x_low": s}) }, rendered: true},
		"x_high":    {inject: func(s string) map[string]any { return covMatrix(map[string]any{"x_high": s}) }, rendered: true},
		"y_low":     {inject: func(s string) map[string]any { return covMatrix(map[string]any{"y_low": s}) }, rendered: true},
		"y_high":    {inject: func(s string) map[string]any { return covMatrix(map[string]any{"y_high": s}) }, rendered: true},
		"title":     {inject: func(s string) map[string]any { return covMatrix(map[string]any{"title": s}) }, rendered: true},
		"takeaway":  {inject: func(s string) map[string]any { return covMatrix(map[string]any{"takeaway": s}) }, rendered: true},
	},
	// go-slide-creator-anzx: every part of the framework has to reach the
	// rendered visual.
	KindFramework: {
		"sections":  {inject: func(s string) map[string]any { return covFramework(map[string]any{"sections": covSWOT(s)}) }, rendered: true},
		"framework": {inject: func(string) map[string]any { return covFramework(nil) }, rendered: false, why: "framework names the visual rather than contributing text of its own"},
		"title":     {inject: func(s string) map[string]any { return covFramework(map[string]any{"title": s}) }, rendered: true},
		"takeaway":  {inject: func(s string) map[string]any { return covFramework(map[string]any{"takeaway": s}) }, rendered: true},
	},
	// go-slide-creator-q31s: the story, its bullets, its result metrics and the
	// picture's caption all have to reach the rendered slide.
	KindImageCase: {
		"body":    {inject: func(s string) map[string]any { return covImageCase(map[string]any{"body": s}) }, rendered: true},
		"bullets": {inject: func(s string) map[string]any { return covImageCase(map[string]any{"bullets": []any{s}}) }, rendered: true},
		"metrics": {inject: func(s string) map[string]any {
			return covImageCase(map[string]any{"metrics": []any{map[string]any{"value": "2", "label": s}}})
		}, rendered: true},
		"eyebrow":     {inject: func(s string) map[string]any { return covImageCase(map[string]any{"eyebrow": s}) }, rendered: true},
		"heading":     {inject: func(s string) map[string]any { return covImageCase(map[string]any{"heading": s}) }, rendered: true},
		"caption":     {inject: func(s string) map[string]any { return covImageCase(map[string]any{"caption": s}) }, rendered: true},
		"image_label": {inject: func(s string) map[string]any { return covImageCase(map[string]any{"image_label": s}) }, rendered: true},
		"image": {inject: func(s string) map[string]any {
			return covImageCase(map[string]any{"image": map[string]any{"path": "/tmp/" + s + ".png", "alt": s}})
		}, rendered: true},
		"image_side": {inject: func(string) map[string]any { return covImageCase(map[string]any{"image_side": "right"}) }, rendered: false, why: "image_side picks a layout rather than contributing text"},
		"title":      {inject: func(s string) map[string]any { return covImageCase(map[string]any{"title": s}) }, rendered: true},
		"takeaway":   {inject: func(s string) map[string]any { return covImageCase(map[string]any{"takeaway": s}) }, rendered: true},
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

func bridgeCoverageBody() map[string]any {
	return map[string]any{"columns": []any{
		map[string]any{"label": "Revenue", "type": "total", "value": 120},
		map[string]any{"label": "COGS", "type": "delta", "value": -45},
		map[string]any{"label": "EBITDA", "type": "subtotal"},
	}}
}

func pillarsCoverageBody() map[string]any {
	return map[string]any{"pillars": []any{
		map[string]any{"title": "Trust", "body": []any{"Reliability"}},
		map[string]any{"title": "Speed", "body": []any{"Delivery"}},
		map[string]any{"title": "Scale", "body": []any{"Growth"}},
	}}
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

// covOptionMatrix returns a minimal valid option_matrix payload with the given
// fields overlaid, so each probe exercises one field against a matrix that
// otherwise reaches the table-highlight pattern.
func covOptionMatrix(overlay map[string]any) map[string]any {
	body := map[string]any{
		"title":    "Options",
		"criteria": []any{"Capex", "Payback"},
		"options": []any{
			map[string]any{"name": "Automate", "scores": []any{1, 2}},
			map[string]any{"name": "Hub", "scores": []any{3, 4}},
		},
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

// covMilestones builds three timeline milestones (the count timeline-horizontal
// requires), the first carrying the probed text.
func covMilestones(firstLabel string) []any {
	return []any{
		map[string]any{"label": firstLabel, "date": "Mar 2024", "body": "The regulator sets the date."},
		map[string]any{"label": "Programme approved", "date": "Sep 2024"},
		map[string]any{"label": "Wave 1 live", "date": "Jun 2025"},
	}
}

// covQuadrants builds the four quadrants a 2x2 needs, the first carrying the
// probed text.
func covQuadrants(firstHeader string) []any {
	return []any{
		map[string]any{"header": firstHeader, "body": "Reconciliation alerts."},
		map[string]any{"header": "Plan properly", "body": "Platform migration."},
		map[string]any{"header": "Defer", "body": "Reporting refresh."},
		map[string]any{"header": "Fill the gaps", "body": "Runbook tidy-up."},
	}
}

// covMatrix returns a minimal valid matrix_2x2 payload with the given fields
// overlaid.
func covMatrix(overlay map[string]any) map[string]any {
	body := map[string]any{
		"x_axis":    "Effort",
		"y_axis":    "Impact",
		"quadrants": covQuadrants("Do first"),
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

// covSWOT builds the four SWOT quadrants, the first carrying the probed text.
func covSWOT(firstStrength string) map[string]any {
	return map[string]any{
		"strengths":     []any{firstStrength},
		"weaknesses":    []any{"Reconciliation is manual"},
		"opportunities": []any{"The T+1 mandate"},
		"threats":       []any{"A competitor is already live"},
	}
}

// covFramework returns a minimal valid framework payload with the given fields
// overlaid.
func covFramework(overlay map[string]any) map[string]any {
	body := map[string]any{
		"framework": "swot",
		"sections":  covSWOT("Two clearers already migrated"),
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

// covImageCase returns a minimal valid image_case payload with the given fields
// overlaid.
func covImageCase(overlay map[string]any) map[string]any {
	body := map[string]any{
		"body": "Northbank ran the cutover on the rehearsed plan.",
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

// covStat returns a minimal valid stat payload with the given fields overlaid.
func covStat(overlay map[string]any) map[string]any {
	body := map[string]any{
		"value": "$2.4B",
		"label": "Addressable market by FY27",
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

// covTable returns a minimal valid table payload with the given fields overlaid.
func covTable(overlay map[string]any) map[string]any {
	body := map[string]any{
		"title":   "Segments",
		"headers": []any{"Segment", "FY26"},
		"rows": []any{
			[]any{"Enterprise", "$41M"},
			[]any{"SMB", "$9M"},
		},
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

package patterns

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/svggen"
)

// Compose candidates (go-slide-creator-okg00, -cny8c, -lp0o8).
//
// A compose candidate names its constituent visuals in its canonical name and
// carries the region layout in Composition: the direction, the size_pct share
// of every region, and nested envelopes. Composition is the ONE representation
// of a recommended multi-region slide: pattern pairs (stylish-panels +
// pull-quote), heterogeneous chart / KPI / diagram regions and shortlist
// round-trips all produce it, and the MCP layer turns it into a runnable
// compose.segments recipe (cmd/json2pptx attachComposeRecipe).
//
// Canonical names: "compose:" + the constituent tokens, sorted and joined with
// "+". A named pattern is its bare registry name; a chart or diagram region is
// "chart:<type>" / "diagram:<type>", so a diagram that shares a pattern's name
// (pyramid, timeline) stays unambiguous. Examples:
//
//	compose:pull-quote+stylish-panels
//	compose:chart:line+diagram:timeline+stat-hero

// VisualComposition is the region layout of a compose candidate: the
// compose.direction and the ordered regions (compose.segments).
type VisualComposition struct {
	// Direction is "horizontal" (regions left to right) or "vertical"
	// (regions top to bottom), the compose envelope's direction.
	Direction string         `json:"direction"`
	Regions   []VisualRegion `json:"regions"`
	// Instructions says how to adopt the recipe: where the sample content
	// sits and what to keep. Set by the MCP layer on the top-level
	// composition only.
	Instructions string `json:"instructions,omitempty"`
}

// VisualRegion is one compose segment: a leaf visual (named_pattern, chart or
// diagram) or a nested composition (category "compose").
type VisualRegion struct {
	Category VisualCategory `json:"category"`
	// Name is the pattern / chart / diagram type; empty for a nested region.
	Name string `json:"name,omitempty"`
	// SizePct is the region's share of its envelope's axis (the segment's
	// size_pct); shares of one envelope sum to 100.
	SizePct float64 `json:"size_pct"`
	// Position is where the region sits on the slide ("left", "upper-right",
	// "bottom", ...).
	Position string `json:"position,omitempty"`
	// Compose is the nested envelope of a category "compose" region.
	Compose *VisualComposition `json:"compose,omitempty"`
	// SegmentPath is this region's segment in next_tool_call's spec
	// (e.g. slides[0].slide.compose.segments[1].compose.segments[0]).
	// Set by the MCP layer.
	SegmentPath string `json:"segment_path,omitempty"`
	// DataContract is the leaf's content contract: the data keys of a chart /
	// diagram, or the values keys of a pattern, with field_path pointing at the
	// sample content to replace. Set by the MCP layer.
	DataContract *VisualDataContract `json:"data_contract,omitempty"`
}

// Leaves returns the leaf regions of the composition in document order.
func (c *VisualComposition) Leaves() []*VisualRegion {
	if c == nil {
		return nil
	}
	var out []*VisualRegion
	for i := range c.Regions {
		r := &c.Regions[i]
		if r.Compose != nil {
			out = append(out, r.Compose.Leaves()...)
			continue
		}
		out = append(out, r)
	}
	return out
}

// composeLeaf is one constituent visual of a compose candidate.
type composeLeaf struct {
	category VisualCategory
	name     string
}

func (l composeLeaf) token() string {
	switch l.category {
	case VisualCategoryChart:
		return "chart:" + l.name
	case VisualCategoryDiagram:
		return "diagram:" + l.name
	default:
		return l.name
	}
}

// composeCandidateName is the canonical, order-independent name of a compose
// candidate with the given leaves.
func composeCandidateName(leaves []composeLeaf) string {
	tokens := make([]string, len(leaves))
	for i, l := range leaves {
		tokens[i] = l.token()
	}
	sort.Strings(tokens)
	return "compose:" + strings.Join(tokens, "+")
}

// composeMaxRecommendedLeaves bounds the regions recommend_visual composes on
// one slide; past four the regions are too small to read.
const composeMaxRecommendedLeaves = 4

// parseComposeName splits a canonical compose name into its leaves, checking
// each against the catalogs. It returns a precise error for a malformed name
// or an unknown / unsupported constituent.
func parseComposeName(reg *Registry, name string) ([]composeLeaf, error) {
	body := strings.TrimPrefix(name, "compose:")
	if body == "" {
		return nil, fmt.Errorf("compose name %q lists no constituents; use compose:<a>+<b> as recommend_visual emits it", name)
	}
	parts := strings.Split(body, "+")
	if len(parts) < 2 {
		return nil, fmt.Errorf("compose name %q has one constituent; a composition needs at least two (compose:<a>+<b>)", name)
	}
	if len(parts) > composeMaxRecommendedLeaves {
		return nil, fmt.Errorf("compose name %q has %d constituents; recommend_visual composes at most %d regions on one slide", name, len(parts), composeMaxRecommendedLeaves)
	}
	readyCharts := readyChartTypes()
	readyDiagrams := readyDiagramTypes()
	seen := make(map[string]bool, len(parts))
	leaves := make([]composeLeaf, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("compose name %q has an empty constituent (doubled or trailing \"+\")", name)
		}
		if seen[p] {
			return nil, fmt.Errorf("compose name %q repeats constituent %q", name, p)
		}
		seen[p] = true
		switch {
		case strings.HasPrefix(p, "chart:"):
			t := strings.TrimPrefix(p, "chart:")
			if !readyCharts[t] {
				return nil, fmt.Errorf("compose constituent %q: %q is not a ready chart type", p, t)
			}
			leaves = append(leaves, composeLeaf{VisualCategoryChart, t})
		case strings.HasPrefix(p, "diagram:"):
			t := strings.TrimPrefix(p, "diagram:")
			if !readyDiagrams[t] {
				return nil, fmt.Errorf("compose constituent %q: %q is not a ready diagram type", p, t)
			}
			leaves = append(leaves, composeLeaf{VisualCategoryDiagram, t})
		default:
			if reg == nil {
				return nil, fmt.Errorf("compose constituent %q: no pattern registry", p)
			}
			if _, ok := reg.Get(p); !ok {
				hint := ""
				if readyCharts[p] {
					hint = fmt.Sprintf("; write the chart as \"chart:%s\"", p)
				} else if readyDiagrams[p] {
					hint = fmt.Sprintf("; write the diagram as \"diagram:%s\"", p)
				}
				return nil, fmt.Errorf("compose constituent %q is not a named pattern%s", p, hint)
			}
			leaves = append(leaves, composeLeaf{VisualCategoryPattern, p})
		}
	}
	return leaves, nil
}

func readyChartTypes() map[string]bool {
	out := make(map[string]bool)
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			out[c.Type] = true
		}
	}
	return out
}

func readyDiagramTypes() map[string]bool {
	out := make(map[string]bool)
	for _, d := range svggen.DiagramCapabilitiesReady() {
		out[d.Type] = true
	}
	return out
}

// composePlacementFor is the placement guidance every compose candidate
// carries: the leaves' names in composable_with.
func composePlacementFor(leaves []composeLeaf) *PlacementGuidance {
	with := make([]string, len(leaves))
	for i, l := range leaves {
		with[i] = l.name
	}
	return &PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "pattern_expansion",
		GridEmbeddable:     false,
		RenderPipeline:     "native_ooxml",
		ComposableWith:     with,
	}
}

// ---------------------------------------------------------------------------
// Pattern-pair layout
// ---------------------------------------------------------------------------

// sideBySideIntentWords mark a request to place the pair left and right.
var sideBySideIntentWords = []string{"side by side", "side-by-side", "alongside", "next to", "beside", "left", "right", "columns"}

func intentWantsSideBySide(intentLower string) bool {
	for _, w := range sideBySideIntentWords {
		if strings.Contains(intentLower, w) {
			return true
		}
	}
	return false
}

// bandRank orders a pattern within a stacked pair from its first declared
// on-slide role: a banner sits on top (0), a body pattern in the middle (1),
// a callout / foundation below (2). Band patterns take the smaller share.
func bandRank(reg *Registry, name string) (rank int, band bool) {
	p, ok := reg.Get(name)
	if !ok {
		return 1, false
	}
	roles := p.Taxonomy().RoleOnSlide
	if len(roles) == 0 {
		return 1, false
	}
	switch roles[0] {
	case "banner":
		return 0, true
	case "callout", "foundation":
		return 2, true
	default:
		return 1, false
	}
}

// pairComposition lays out a two-pattern compose candidate. Side-by-side
// intents get a horizontal envelope with the body pattern on the left; all
// others stack vertically, banner on top and callout / foundation below. A
// band pattern (kpi row, quote, icon row) takes 35% of the width beside a
// body pattern; stacked, both halves get 50% (a KPI row squeezed below half
// the height shrinks its labels under the readable minimum).
func pairComposition(reg *Registry, a, b, intentLower string) *VisualComposition {
	rankA, bandA := bandRank(reg, a)
	rankB, bandB := bandRank(reg, b)
	first, second := a, b
	firstBand, secondBand := bandA, bandB
	horizontal := intentWantsSideBySide(intentLower)
	if horizontal {
		// Body on the left, band on the right.
		if bandA && !bandB {
			first, second, firstBand, secondBand = b, a, bandB, bandA
		}
	} else if rankB < rankA {
		first, second, firstBand, secondBand = b, a, bandB, bandA
	}
	share := func(band, otherBand bool) float64 {
		switch {
		case !horizontal || band == otherBand:
			return 50
		case band:
			return 35
		default:
			return 65
		}
	}
	dir, p1, p2 := "vertical", "top", "bottom"
	if horizontal {
		dir, p1, p2 = "horizontal", "left", "right"
	}
	return &VisualComposition{
		Direction: dir,
		Regions: []VisualRegion{
			{Category: VisualCategoryPattern, Name: first, SizePct: share(firstBand, secondBand), Position: p1},
			{Category: VisualCategoryPattern, Name: second, SizePct: share(secondBand, firstBand), Position: p2},
		},
	}
}

// pairAffinity reports whether either pattern declares the other in its
// ComposesWith axis — the condition scoreCompose emits a pair under.
func pairAffinity(reg *Registry, a, b string) bool {
	pa, okA := reg.Get(a)
	pb, okB := reg.Get(b)
	if !okA || !okB {
		return false
	}
	return sliceContains(pa.Taxonomy().ComposesWith, b) || sliceContains(pb.Taxonomy().ComposesWith, a)
}

// ---------------------------------------------------------------------------
// Heterogeneous region intents (go-slide-creator-lp0o8)
// ---------------------------------------------------------------------------

// regionIntent is one view the user asked to place in its own region.
type regionIntent struct {
	leaf composeLeaf
	// kind is the requested view: "chart", "kpi", "diagram" or "quote".
	kind string
	// h is "left" / "right" / ""; v is "top" / "bottom" / "".
	h, v string
	// pct is an explicit share ("65%"), 0 when none was given.
	pct float64
}

// compoundIntent is a parsed same-slide multi-region request.
type compoundIntent struct {
	regions []regionIntent
	// unsupported names requested views a compose region cannot host.
	unsupported []string
}

var (
	pctRE = regexp.MustCompile(`(\d{1,2}(?:\.\d+)?)\s*(?:%|percent\b|pct\b)`)
	// relationRE finds "X above Y"-style relations inside one clause;
	// relationPositions rewrites them into two positioned clauses.
	relationRE    = regexp.MustCompile(`\b(?:above|on top of|below|beneath|underneath|to the left of|left of|to the right of|right of)\b`)
	clauseSplitRE = regexp.MustCompile(`[,;:.()]+|\band\b|\bwith\b|\bplus\b|\balongside\b|\bbeside\b|\bnext to\b`)
)

// sameSlideIntentWords mark that the views belong together on one slide.
var sameSlideIntentWords = []string{
	"one slide", "single slide", "same slide", "a slide", "this slide",
	"region", "regions", "divided", "split", "panel", "quadrant", "dashboard",
	"together", "side by side", "side-by-side", "combine", "combined", "compose",
}

var kpiCountWords = map[string]int{"two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6}

// parseCompoundIntent finds an explicit request for two or more different
// views in their own regions of one slide ("line chart left 65%, KPI
// upper-right, timeline lower-right"). It returns nil when the intent names
// fewer than two supported views or gives no sign that they share a slide
// (no positions, no same-slide wording), so whole-slide intents keep their
// singleton recommendations.
func parseCompoundIntent(reg *Registry, intentLower string, hints *VisualHints) *compoundIntent {
	var ci compoundIntent
	positioned := false
	usedKinds := map[string]bool{}
	for _, clause := range clauseSplitRE.Split(relationRE.ReplaceAllStringFunc(intentLower, relationPositions), -1) {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		words := intentWords(clause)
		kind, leaf, ok := regionViewFor(reg, clause, words, hints)
		if !ok {
			if intentHasAny(words, "table", "tabular") {
				ci.unsupported = append(ci.unsupported, "table")
			}
			continue
		}
		r := regionIntent{leaf: leaf, kind: kind}
		r.h, r.v = clausePosition(words)
		if r.h != "" || r.v != "" {
			positioned = true
		}
		if m := pctRE.FindStringSubmatch(clause); m != nil {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil && f > 0 && f < 100 {
				r.pct = f
			}
		}
		ci.regions = append(ci.regions, r)
		usedKinds[kind] = true
		if len(ci.regions) == composeMaxRecommendedLeaves {
			break
		}
	}
	if len(ci.regions) < 2 || len(usedKinds) < 2 {
		return nil
	}
	if !positioned {
		together := false
		for _, w := range sameSlideIntentWords {
			if strings.Contains(intentLower, w) {
				together = true
				break
			}
		}
		if !together {
			return nil
		}
	}
	// Two regions of the same leaf collapse to one composition token; keep
	// the composition well-formed by dropping exact duplicates.
	seen := map[string]bool{}
	dedup := ci.regions[:0]
	for _, r := range ci.regions {
		if seen[r.leaf.token()] {
			continue
		}
		seen[r.leaf.token()] = true
		dedup = append(dedup, r)
	}
	ci.regions = dedup
	if len(ci.regions) < 2 {
		return nil
	}
	return &ci
}

// relationPositions turns a relation word into "<first position>, <second
// position>": "kpi row above a process flow" reads as "kpi row top, bottom a
// process flow".
func relationPositions(rel string) string {
	switch rel {
	case "above", "on top of":
		return " top , bottom "
	case "below", "beneath", "underneath":
		return " bottom , top "
	case "to the left of", "left of":
		return " left , right "
	default:
		return " right , left "
	}
}

// clausePosition reads left/right and top/bottom words in one clause;
// "top-right" splits into both.
func clausePosition(words []string) (h, v string) {
	for _, w := range words {
		switch w {
		case "left", "lefthand":
			h = "left"
		case "right", "righthand":
			h = "right"
		case "top", "upper", "above", "header":
			v = "top"
		case "bottom", "lower", "below", "footer":
			v = "bottom"
		}
	}
	return h, v
}

// regionViewFor maps one clause to the view it requests and the leaf that
// renders it: a chart (best chart rule for the clause), a KPI pattern, a
// quote, or a diagram (best diagram rule). Tables and free text have no
// compose segment and return ok=false.
func regionViewFor(reg *Registry, clause string, words []string, hints *VisualHints) (kind string, leaf composeLeaf, ok bool) {
	has := func(ws ...string) bool { return intentHasAny(words, ws...) }
	switch {
	case has("chart", "graph", "plot"):
		return "chart", composeLeaf{VisualCategoryChart, bestChartFor(clause, hints)}, true
	case has("kpi", "metric", "stat", "tile", "scorecard") ||
		strings.Contains(clause, "big number") || strings.Contains(clause, "headline number"):
		name := kpiPatternFor(words)
		if reg != nil {
			if _, found := reg.Get(name); !found {
				return "", composeLeaf{}, false
			}
		}
		return "kpi", composeLeaf{VisualCategoryPattern, name}, true
	case has("line", "bar", "pie", "donut", "waterfall", "scatter", "funnel", "area"):
		return "chart", composeLeaf{VisualCategoryChart, bestChartFor(clause, hints)}, true
	case has("quote", "testimonial"):
		if reg != nil {
			if _, found := reg.Get("pull-quote"); found {
				return "quote", composeLeaf{VisualCategoryPattern, "pull-quote"}, true
			}
		}
		return "", composeLeaf{}, false
	}
	if name := bestDiagramFor(clause); name != "" {
		return "diagram", composeLeaf{VisualCategoryDiagram, name}, true
	}
	return "", composeLeaf{}, false
}

// kpiPatternFor picks the KPI pattern a clause asks for: one tile is a
// stat-hero, "three KPIs" a kpi-3up, plural without a count a kpi-3up.
func kpiPatternFor(words []string) string {
	for i, w := range words {
		n, isCount := kpiCountWords[w]
		if !isCount || i+1 >= len(words) {
			continue
		}
		next := words[i+1]
		if strings.HasPrefix(next, "kpi") || strings.HasPrefix(next, "metric") || strings.HasPrefix(next, "tile") || strings.HasPrefix(next, "stat") {
			return fmt.Sprintf("kpi-%dup", n)
		}
	}
	for _, w := range words {
		switch w {
		case "kpis", "metrics", "tiles", "stats", "scorecard", "row", "strip", "dashboard", "cards":
			return "kpi-3up"
		}
	}
	return "stat-hero"
}

// bestChartFor scores the chart rules against one clause; a bare "chart"
// with no type keyword is a bar chart.
func bestChartFor(clause string, hints *VisualHints) string {
	ready := readyChartTypes()
	best, bestScore := "bar", 0.0
	for _, r := range chartRules {
		if !ready[r.chartType] {
			continue
		}
		s := scoreChartRule(r, clause, hints)
		if strings.Contains(clause, strings.ReplaceAll(r.chartType, "_", " ")) {
			s += 0.2 // the clause names the type outright
		}
		if s > bestScore {
			best, bestScore = r.chartType, s
		}
	}
	return best
}

// regionDiagramExcluded lists diagrams that duplicate another region kind
// (KPI tiles, panels) and so never stand for a diagram region.
var regionDiagramExcluded = map[string]bool{"kpi_dashboard": true, "stat_cards": true, "panel_layout": true, "icon_columns": true, "icon_rows": true}

// bestDiagramFor scores the diagram rules against one clause.
func bestDiagramFor(clause string) string {
	ready := readyDiagramTypes()
	best, bestScore := "", 0.0
	for _, r := range diagramRules {
		if !ready[r.diagramType] || regionDiagramExcluded[r.diagramType] {
			continue
		}
		if s := scoreKeywords(r.keywords, clause, r.baseScore); s > bestScore {
			best, bestScore = r.diagramType, s
		}
	}
	return best
}

// regionWeight is a region's default share weight: a single KPI tile or a
// quote needs less room than a chart or diagram; a KPI row (kpi-Nup) needs
// as much, or its labels shrink below the readable minimum.
func regionWeight(r regionIntent) float64 {
	switch {
	case r.kind == "quote", r.kind == "kpi" && r.leaf.name == "stat-hero":
		return 2
	default:
		return 3
	}
}

// shareOut assigns size_pct to regions of one envelope: explicit shares are
// kept, the rest of 100 is split by weight. Explicit shares that overflow
// 100 are dropped in favour of weights.
func shareOut(pcts, weights []float64) []float64 {
	out := make([]float64, len(pcts))
	explicit, wRest := 0.0, 0.0
	for i, p := range pcts {
		if p > 0 {
			explicit += p
		} else {
			wRest += weights[i]
		}
	}
	if explicit >= 100 || (explicit > 0 && wRest == 0 && explicit != 100) {
		// Inconsistent explicit shares: fall back to weights for all.
		for i := range pcts {
			pcts[i] = 0
		}
		explicit, wRest = 0, 0
		for _, w := range weights {
			wRest += w
		}
	}
	rest := 100 - explicit
	for i, p := range pcts {
		if p > 0 {
			out[i] = p
			continue
		}
		out[i] = math.Round(rest * weights[i] / wRest)
	}
	// Absorb rounding drift in the last weighted region.
	sum := 0.0
	for _, v := range out {
		sum += v
	}
	if sum != 100 {
		for i := len(out) - 1; i >= 0; i-- {
			if pcts[i] == 0 {
				out[i] += 100 - sum
				break
			}
		}
	}
	return out
}

// buildRegionComposition arranges region intents into a (possibly nested)
// composition. Left / right words make the outer envelope horizontal with
// each side stacked top-to-bottom; top / bottom words alone make it vertical
// with each band laid out left-to-right. Unpositioned intents put the chart
// (else the first view) on the left at 60% and stack the rest on the right.
func buildRegionComposition(regions []regionIntent) *VisualComposition {
	anyH, anyV := false, false
	for _, r := range regions {
		anyH = anyH || r.h != ""
		anyV = anyV || r.v != ""
	}
	switch {
	case anyH:
		return splitComposition(regions, "horizontal", func(r regionIntent) string { return r.h }, "left", "right",
			func(r regionIntent) string { return r.v }, "top", "bottom")
	case anyV:
		return splitComposition(regions, "vertical", func(r regionIntent) string { return r.v }, "top", "bottom",
			func(r regionIntent) string { return r.h }, "left", "right")
	}
	// No positions: a KPI row and one other view stack, the row on top;
	// otherwise the lead view goes left and the others stack on the right.
	if len(regions) == 2 {
		for i, r := range regions {
			if r.kind == "kpi" && r.leaf.name != "stat-hero" {
				other := regions[1-i]
				return leafEnvelope([]regionIntent{r, other}, "vertical", "top", "bottom")
			}
		}
	}
	lead := 0
	for i, r := range regions {
		if r.kind == "chart" {
			lead = i
			break
		}
	}
	placed := make([]regionIntent, len(regions))
	copy(placed, regions)
	for i := range placed {
		if i == lead {
			placed[i].h = "left"
		} else {
			placed[i].h = "right"
		}
	}
	return splitComposition(placed, "horizontal", func(r regionIntent) string { return r.h }, "left", "right",
		func(r regionIntent) string { return r.v }, "top", "bottom")
}

// splitComposition groups regions on the outer axis (first / second side),
// nests each multi-region side on the cross axis, and assigns shares.
func splitComposition(regions []regionIntent, dir string, outer func(regionIntent) string, firstSide, secondSide string,
	inner func(regionIntent) string, innerFirst, innerSecond string,
) *VisualComposition {
	var first, second, loose []regionIntent
	for _, r := range regions {
		switch outer(r) {
		case firstSide:
			first = append(first, r)
		case secondSide:
			second = append(second, r)
		default:
			loose = append(loose, r)
		}
	}
	// Unplaced views join the emptier side (the second side on a tie).
	for _, r := range loose {
		if len(first) < len(second) {
			first = append(first, r)
		} else {
			second = append(second, r)
		}
	}
	if len(first) == 0 || len(second) == 0 {
		// Everything on one side: lay that side out on the outer axis.
		all := append(first, second...)
		sortByInner(all, inner, innerFirst, innerSecond)
		return leafEnvelope(all, dir, firstSide, secondSide)
	}
	innerDir := "vertical"
	if dir == "vertical" {
		innerDir = "horizontal"
	}
	side := func(group []regionIntent, pos string) VisualRegion {
		if len(group) == 1 {
			r := group[0]
			return VisualRegion{Category: r.leaf.category, Name: r.leaf.name, SizePct: r.pct, Position: pos}
		}
		sortByInner(group, inner, innerFirst, innerSecond)
		nested := leafEnvelope(group, innerDir, innerFirst, innerSecond)
		for i := range nested.Regions {
			nested.Regions[i].Position = joinPosition(nested.Regions[i].Position, pos, dir)
		}
		return VisualRegion{Category: VisualCategoryCompose, Compose: nested, Position: pos}
	}
	r1, r2 := side(first, firstSide), side(second, secondSide)
	// Outer shares: an explicit share of a single-region side wins; the side
	// holding the chart otherwise takes 60%.
	pcts := []float64{r1.SizePct, r2.SizePct}
	weights := []float64{groupWeight(first), groupWeight(second)}
	shares := shareOut(pcts, weights)
	r1.SizePct, r2.SizePct = shares[0], shares[1]
	return &VisualComposition{Direction: dir, Regions: []VisualRegion{r1, r2}}
}

// groupWeight weights an outer side: a single view weighs as itself (a
// chart or diagram 3, a KPI or quote 2), a stacked side of several views 2,
// so a lead chart beside a stacked column takes 60%.
func groupWeight(group []regionIntent) float64 {
	if len(group) == 1 {
		return regionWeight(group[0])
	}
	return 2
}

func sortByInner(group []regionIntent, inner func(regionIntent) string, first, second string) {
	rank := func(r regionIntent) int {
		switch inner(r) {
		case first:
			return 0
		case second:
			return 2
		default:
			return 1
		}
	}
	sort.SliceStable(group, func(i, j int) bool { return rank(group[i]) < rank(group[j]) })
}

// leafEnvelope lays leaf regions out along dir with weighted / explicit
// shares; positions read first, (middle,) last.
func leafEnvelope(group []regionIntent, dir, firstPos, lastPos string) *VisualComposition {
	pcts := make([]float64, len(group))
	weights := make([]float64, len(group))
	for i, r := range group {
		pcts[i] = r.pct
		weights[i] = regionWeight(r)
	}
	shares := shareOut(pcts, weights)
	out := &VisualComposition{Direction: dir}
	for i, r := range group {
		pos := "middle"
		switch i {
		case 0:
			pos = firstPos
		case len(group) - 1:
			pos = lastPos
		}
		out.Regions = append(out.Regions, VisualRegion{Category: r.leaf.category, Name: r.leaf.name, SizePct: shares[i], Position: pos})
	}
	return out
}

// joinPosition names a nested region's slide position: "upper-right" for the
// top of the right column, "bottom-left" for the left of the bottom band.
func joinPosition(innerPos, outerPos, outerDir string) string {
	if outerDir == "horizontal" {
		switch innerPos {
		case "top":
			return "upper-" + outerPos
		case "bottom":
			return "lower-" + outerPos
		}
		return innerPos + "-" + outerPos
	}
	return outerPos + "-" + innerPos
}

// compoundLeaves returns the composition's leaves for naming.
func compoundLeaves(ci *compoundIntent) []composeLeaf {
	out := make([]composeLeaf, len(ci.regions))
	for i, r := range ci.regions {
		out[i] = r.leaf
	}
	return out
}

// sameLeafSet reports whether two leaf lists hold the same tokens.
func sameLeafSet(a, b []composeLeaf) bool {
	return composeCandidateName(a) == composeCandidateName(b)
}

// compoundRationale explains a heterogeneous composition.
func compoundRationale(ci *compoundIntent, comp *VisualComposition) string {
	var parts []string
	for _, l := range comp.Leaves() {
		parts = append(parts, fmt.Sprintf("%s %s %s (%g%%)", l.Position, l.Category, l.Name, l.SizePct))
	}
	msg := "Composition with one region per requested view — " + strings.Join(parts, ", ") +
		". The intent asks for these views together on one slide, so this ranks above any single view."
	if len(ci.unsupported) > 0 {
		msg += " No compose region hosts a " + strings.Join(ci.unsupported, "/") + "; put it on its own slide or in a pattern."
	}
	return msg
}

// scoreCompoundCompose emits the heterogeneous composition for an explicit
// same-slide compound intent and demotes every other candidate below it: they
// each show only part of what the user asked to see together.
func scoreCompoundCompose(reg *Registry, intentLower string, hints *VisualHints, all []VisualCandidate) []VisualCandidate {
	ci := parseCompoundIntent(reg, intentLower, hints)
	if ci == nil {
		return all
	}
	comp := buildRegionComposition(ci.regions)
	leaves := compoundLeaves(ci)
	top := 0.0
	for _, c := range all {
		top = math.Max(top, c.Score)
	}
	score := roundScore(math.Min(1, math.Max(top, 0.9)+0.02))
	name := composeCandidateName(leaves)
	all = demoteBelow(all, score, name)
	return append(all, VisualCandidate{
		Category:       VisualCategoryCompose,
		Name:           name,
		Score:          score,
		Rationale:      compoundRationale(ci, comp),
		ConfidenceBand: confidenceBand(score),
		Placement:      composePlacementFor(leaves),
		Composition:    comp,
	})
}

// demoteBelow caps every candidate other than keep strictly below score, so
// the full composition outranks partial views; the candidate of the same name
// (a pattern-pair compose with the same leaves) is dropped as a duplicate.
func demoteBelow(all []VisualCandidate, score float64, keep string) []VisualCandidate {
	out := all[:0]
	for _, c := range all {
		if c.Name == keep {
			continue
		}
		if c.Score >= score {
			c.Score = roundScore(math.Max(0, score-0.01))
			c.ConfidenceBand = confidenceBand(c.Score)
			c.Rationale += "; shows only part of the requested same-slide composition"
		}
		out = append(out, c)
	}
	return out
}

// ---------------------------------------------------------------------------
// Shortlist resolution (go-slide-creator-cny8c)
// ---------------------------------------------------------------------------

// scoreComposeShortlistCandidate resolves a "compose:..." name in shortlist
// mode the same way normal mode emits it: a pattern pair is scored as the
// average of its patterns (+0.10 for an explicit compose intent) with the
// pair layout; a heterogeneous name takes the intent's region layout when the
// intent requests exactly those views, else a default layout. A malformed
// name or an unknown constituent returns category compose with score 0 and
// the precise reason.
func scoreComposeShortlistCandidate(reg *Registry, intentLower string, hints *VisualHints, name string, scoreLeaf func(composeLeaf) VisualCandidate) VisualCandidate {
	reject := func(reason string) VisualCandidate {
		return VisualCandidate{
			Category:       VisualCategoryCompose,
			Name:           name,
			Score:          0,
			Rationale:      "Invalid compose candidate: " + reason,
			ConfidenceBand: confidenceBand(0),
		}
	}
	leaves, err := parseComposeName(reg, name)
	if err != nil {
		return reject(err.Error())
	}
	allPatterns := true
	for _, l := range leaves {
		allPatterns = allPatterns && l.category == VisualCategoryPattern
	}
	if allPatterns && len(leaves) == 2 {
		a, b := leaves[0].name, leaves[1].name
		if !pairAffinity(reg, a, b) {
			return reject(fmt.Sprintf("neither %q nor %q declares the other in its compose affinity (taxonomy composes_with), so recommend_visual never pairs them", a, b))
		}
		score := (scoreLeaf(leaves[0]).Score + scoreLeaf(leaves[1]).Score) / 2
		rationale := fmt.Sprintf("Compose envelope combining %q and %q — the pair declares compose-affinity.", a, b)
		if explicitComposeIntent(intentLower) {
			score += 0.10
			rationale = fmt.Sprintf("Compose envelope combining %q and %q — the intent requests a multi-pattern layout and the pair declares compose-affinity.", a, b)
		}
		score = math.Min(1, score)
		return VisualCandidate{
			Category:       VisualCategoryCompose,
			Name:           composeCandidateName(leaves),
			Score:          roundScore(score),
			Rationale:      rationale,
			ConfidenceBand: confidenceBand(score),
			Placement:      composePlacementFor(leaves),
			Composition:    pairComposition(reg, a, b, intentLower),
		}
	}

	canonical := composeCandidateName(leaves)
	if ci := parseCompoundIntent(reg, intentLower, hints); ci != nil && sameLeafSet(compoundLeaves(ci), leaves) {
		comp := buildRegionComposition(ci.regions)
		return VisualCandidate{
			Category:    VisualCategoryCompose,
			Name:        canonical,
			Rationale:   compoundRationale(ci, comp),
			Placement:   composePlacementFor(leaves),
			Composition: comp,
			// Scored by rankShortlistCompound: just above every partial view.
			intentCompound: true,
		}
	}
	// Not the intent's own composition: score as the mean of its views and
	// lay it out with defaults.
	regions := make([]regionIntent, len(leaves))
	sum := 0.0
	for i, l := range leaves {
		kind := "diagram"
		switch {
		case l.category == VisualCategoryChart:
			kind = "chart"
		case l.name == "pull-quote":
			kind = "quote"
		case strings.HasPrefix(l.name, "kpi-") || l.name == "stat-hero":
			kind = "kpi"
		}
		regions[i] = regionIntent{leaf: l, kind: kind}
		sum += scoreLeaf(l).Score
	}
	score := sum / float64(len(leaves))
	return VisualCandidate{
		Category:       VisualCategoryCompose,
		Name:           canonical,
		Score:          roundScore(score),
		Rationale:      "Composition of the listed views with a default region layout (scored as the mean of its views; the intent does not ask for exactly these regions).",
		ConfidenceBand: confidenceBand(score),
		Placement:      composePlacementFor(leaves),
		Composition:    buildRegionComposition(regions),
	}
}

// explicitComposeIntent reports whether the intent asks to combine views.
func explicitComposeIntent(intentLower string) bool {
	for _, kw := range composeIntentKeywords {
		if strings.Contains(intentLower, kw) {
			return true
		}
	}
	return false
}

// rankShortlistCompound places the intent's own heterogeneous composition
// just above every other shortlist entry, mirroring normal mode.
func rankShortlistCompound(out []VisualCandidate) []VisualCandidate {
	idx := -1
	for i, c := range out {
		if c.intentCompound {
			idx = i
			break
		}
	}
	if idx < 0 {
		return out
	}
	top := 0.0
	for i, c := range out {
		if i != idx {
			top = math.Max(top, c.Score)
		}
	}
	score := roundScore(math.Min(1, math.Max(top, 0.9)+0.02))
	keep := out[idx]
	keep.intentCompound = false
	keep.Score = score
	keep.ConfidenceBand = confidenceBand(score)
	rest := make([]VisualCandidate, 0, len(out)-1)
	rest = append(rest, out[:idx]...)
	rest = append(rest, out[idx+1:]...)
	rest = demoteBelow(rest, score, keep.Name)
	return append(rest, keep)
}

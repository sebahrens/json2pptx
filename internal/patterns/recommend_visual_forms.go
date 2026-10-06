package patterns

import (
	"math"
	"strings"
)

// Forms of one visual (go-slide-creator-7sqof, go-slide-creator-bdvhj).
//
// Several visuals exist twice: as a named pattern (native, editable shapes)
// and as an svggen chart / diagram of nearly the same name, with different
// field names and different abilities. recommend_visual scored both from the
// same keywords, so they tied (value-chain 0.96 = value_chain 0.96) and
// nothing said that only one of them could do what was asked. visualTwins is
// the one table of those pairs: it feeds each candidate's also_as
// cross-reference, the differs_by line on a near tie, and the cue routing
// that breaks the tie when the intent names an ability only one form has.

// visualTwin is one visual drawn by two renderers.
type visualTwin struct {
	pattern string
	// other is the chart / diagram form.
	other    string
	otherCat VisualCategory
	// patternOnly / otherOnly say what only that form does, phrased to
	// complete "only the pattern …" / "only the diagram …".
	patternOnly string
	otherOnly   string
	// patternFields / otherFields are each form's data shape in one line.
	patternFields string
	otherFields   string
	// patternCues / otherCues are intent phrases that ask for an ability only
	// that form has.
	patternCues []string
	otherCues   []string
}

var visualTwins = []visualTwin{
	{
		pattern: "matrix-2x2", other: "matrix_2x2", otherCat: VisualCategoryDiagram,
		patternOnly:   "holds a header and body text per quadrant as native editable shapes",
		otherOnly:     "plots labelled points at x/y positions",
		patternFields: "x_axis_label, y_axis_label, top_left / top_right / bottom_left / bottom_right {header, body} (or quadrants[4] in that order)",
		otherFields:   "x_axis_label, y_axis_label and points[{label, x, y}] on a 0–100 scale, or quadrants[{position, title, items}]",
		patternCues:   []string{"labelled", "labeled", "named", "quadrant labels", "initiative"},
		otherCues:     []string{"plotted", "plot", "plots", "points", "dots", "bubbles", "coordinates", "scatter"},
	},
	{
		pattern: "value-chain", other: "value_chain", otherCat: VisualCategoryDiagram,
		patternOnly:   "highlights a step (steps[].highlight) and gives each of 4–10 steps a description",
		otherOnly:     "draws Porter's layout: primary activities over support activities with a margin",
		patternFields: "steps[{label, description, highlight}]",
		otherFields:   "primary[{label, description, items}], support[{label, …}], margin_label",
		patternCues:   []string{"highlight", "highlighted", "highlighting", "emphasised", "emphasized", "description", "descriptions"},
		otherCues:     []string{"support activities", "primary activities", "primary and support", "margin"},
	},
	{
		pattern: "process-flow", other: "process_flow", otherCat: VisualCategoryDiagram,
		patternOnly:   "draws 3–8 steps in one lane as native editable shapes",
		otherOnly:     "takes explicit connections, so it can loop back and run vertically",
		patternFields: "steps[{label, type, highlight}]",
		otherFields:   "steps[{id, label, type}], connections[{from, to, label}], direction",
	},
	{
		pattern: "pyramid", other: "pyramid", otherCat: VisualCategoryDiagram,
		patternOnly:   "draws 3–5 tiers as native editable trapezoids",
		otherOnly:     "takes a description per level and more than five levels",
		patternFields: "tiers[string]",
		otherFields:   "levels[{label, description}]",
	},
	{
		pattern: "timeline-horizontal", other: "timeline", otherCat: VisualCategoryDiagram,
		patternOnly:   "sets 3–7 milestones on a line, each with a date and a line of detail",
		otherOnly:     "draws date-scaled range bars with milestone markers",
		patternFields: "values is a list of {label, date, body, end_date}",
		otherFields:   "events[{label, start_date, end_date}], milestones[{label, date}]",
		otherCues:     []string{"durations", "duration", "date range", "date ranges", "start and end", "overlapping"},
	},
	{
		pattern: "bmc-canvas", other: "business_model_canvas", otherCat: VisualCategoryDiagram,
		patternOnly:   "draws the nine blocks as native editable shapes, each with its own header",
		otherOnly:     "takes each block as a plain string list",
		patternFields: "nine keys, each {header, bullets[]}",
		otherFields:   "the same nine keys, each a string[]",
	},
	{
		pattern: "strategy-house", other: "house_diagram", otherCat: VisualCategoryDiagram,
		patternOnly:   "draws a gabled roof, 3–5 pillars and 1–3 foundation levels as native editable shapes",
		otherOnly:     "takes any number of floors under the pillar row",
		patternFields: "objective, pillars[{title, body[]}], foundation (a string, or 1–3 levels each a string or a row of cells), beam, roof_badges",
		otherFields:   "roof, sections[{label, items}], floors[], foundation",
	},
	{
		pattern: "waterfall-bridge", other: "waterfall", otherCat: VisualCategoryChart,
		patternOnly:   "computes subtotals itself and draws 3–10 native editable bars",
		otherOnly:     "takes explicit increase / decrease / total points on a chart axis",
		patternFields: "columns[{label, type: total|delta|subtotal, value}], unit, caption",
		otherFields:   "points[{label, value, type: increase|decrease|total}]",
	},
	{
		pattern: "capability-heatmap", other: "heatmap", otherCat: VisualCategoryDiagram,
		patternOnly:   "rates named activities per function in 2–4 tiers with a legend",
		otherOnly:     "colours a numeric matrix on a continuous scale",
		patternFields: "tiers[{label}], columns[{header, cells[{text, tier}]}]",
		otherFields:   "values[][] numbers, row_labels[], col_labels[]",
		otherCues:     []string{"numeric", "numbers", "correlation", "intensity", "density"},
	},
	{
		pattern: "roadmap-phased", other: "gantt", otherCat: VisualCategoryDiagram,
		patternOnly:   "lays out workstreams × phases as a grid of text cells with no date scale",
		otherOnly:     "draws task bars on a date axis with milestones",
		patternFields: "phases[string], workstreams[{name, items[]}] (one item per phase)",
		otherFields:   "tasks[{id, label, start_date, end_date, group}], milestones[{id, label, date}]",
		otherCues:     []string{"gantt", "start and end dates", "dependencies", "overlapping", "schedule"},
	},
	{
		pattern: "swimlane", other: "process_flow", otherCat: VisualCategoryDiagram,
		patternOnly:   "gives every actor a lane and places each step in the lane of its owner",
		otherOnly:     "draws one flow with explicit connections and no lanes",
		patternFields: "lanes[{actor, steps[]}] (one step per column; an empty string leaves the cell empty), flow[[from, to]]",
		otherFields:   "steps[{id, label, type}], connections[{from, to, label}]",
	},
	{
		pattern: "team-bios", other: "org_chart", otherCat: VisualCategoryDiagram,
		patternOnly:   "shows 1–8 people as cards with a role and a short bio, without reporting lines",
		otherOnly:     "draws the reporting tree",
		patternFields: "members[{name, role, bio, photo}]",
		otherFields:   "root {name, title, children[…]}",
	},
}

// twinFor returns the twin entry a candidate belongs to and whether the
// candidate is its pattern side. A pattern in several entries (none today)
// resolves to the first.
func twinsFor(cat VisualCategory, name string) (out []visualTwin, patternSide bool) {
	for _, tw := range visualTwins {
		switch {
		case cat == VisualCategoryPattern && tw.pattern == name:
			out, patternSide = append(out, tw), true
		case cat == tw.otherCat && tw.other == name:
			out = append(out, tw)
		}
	}
	return out, patternSide
}

func categoryForm(cat VisualCategory) string {
	switch cat {
	case VisualCategoryPattern:
		return "pattern"
	case VisualCategoryChart:
		return "chart"
	case VisualCategoryDiagram:
		return "diagram"
	case VisualCategoryKind:
		return "kind"
	case VisualCategoryPlaceholder:
		return "layout"
	default:
		return string(cat)
	}
}

// sameNamedTwin reports whether the twin's two forms carry the same name once
// separators are ignored (matrix-2x2 / matrix_2x2) — the pairs an agent takes
// for one thing. Related-but-different pairs (swimlane / process_flow) inform
// differs_by only.
func sameNamedTwin(tw visualTwin) bool {
	return sameVisual(tw.pattern, tw.other) || tw.pattern == "strategy-house" || tw.pattern == "waterfall-bridge"
}

// VisualAlsoAs returns the other pattern / chart / diagram forms of the same
// visual with what each takes and does differently. The MCP layer adds the
// DeckSpec kind.
func VisualAlsoAs(cat VisualCategory, name string) []VisualFormRef {
	twins, patternSide := twinsFor(cat, name)
	var out []VisualFormRef
	for _, tw := range twins {
		if !sameNamedTwin(tw) {
			continue
		}
		if patternSide {
			out = append(out, VisualFormRef{
				Form: categoryForm(tw.otherCat), Name: tw.other,
				Differs: "takes " + tw.otherFields + "; only it " + tw.otherOnly,
			})
			continue
		}
		out = append(out, VisualFormRef{
			Form: "pattern", Name: tw.pattern,
			Differs: "takes " + tw.patternFields + "; only it " + tw.patternOnly,
		})
	}
	return out
}

// VisualTwinPatterns returns, for a chart / diagram, the same-named patterns.
func VisualTwinPatterns(cat VisualCategory, name string) []string {
	twins, patternSide := twinsFor(cat, name)
	if patternSide {
		return nil
	}
	var out []string
	for _, tw := range twins {
		if sameNamedTwin(tw) {
			out = append(out, tw.pattern)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// differs_by
// ---------------------------------------------------------------------------

// differsByMargin is the score distance under which two candidates count as a
// near tie and must say what sets them apart.
const differsByMargin = 0.02

// differsTraitMax caps one side of a generic differs_by line, in characters.
const differsTraitMax = 70

// AnnotateDiffersBy sets differs_by on every candidate that sits within
// differsByMargin of another one, and clears it elsewhere. It is recomputed
// whenever the ranking changes (template-support reordering, truncation).
func AnnotateDiffersBy(reg *Registry, candidates []VisualCandidate) {
	for i := range candidates {
		c := &candidates[i]
		c.DiffersBy = ""
		if c.Score <= 0 {
			continue
		}
		// The nearest other candidate; on equal distance the higher-ranked.
		best, bestGap := -1, math.MaxFloat64
		for j := range candidates {
			if j == i || candidates[j].Score <= 0 {
				continue
			}
			gap := math.Abs(c.Score - candidates[j].Score)
			if gap > differsByMargin+1e-9 {
				continue
			}
			if gap < bestGap-1e-9 {
				best, bestGap = j, gap
			}
		}
		if best < 0 {
			continue
		}
		c.DiffersBy = differsByLine(reg, *c, candidates[best])
	}
}

// differsByLine is one line contrasting c with its near tie o.
func differsByLine(reg *Registry, c, o VisualCandidate) string {
	vs := "vs " + o.Name + " (" + categoryForm(o.Category) + "): "
	twins, patternSide := twinsFor(c.Category, c.Name)
	for _, tw := range twins {
		if patternSide && o.Category == tw.otherCat && o.Name == tw.other {
			return vs + "only this pattern " + tw.patternOnly + "; only the " + categoryForm(tw.otherCat) + " " + tw.otherOnly
		}
		if !patternSide && o.Category == VisualCategoryPattern && o.Name == tw.pattern {
			return vs + "only this " + categoryForm(tw.otherCat) + " " + tw.otherOnly + "; only the pattern " + tw.patternOnly
		}
	}
	return vs + "this is " + candidateTrait(reg, c) + "; that is " + candidateTrait(reg, o)
}

// candidateTrait is a short phrase saying what a candidate is for, taken from
// catalog truth rather than the (already shown) scoring rationale.
func candidateTrait(reg *Registry, c VisualCandidate) string {
	switch c.Category {
	case VisualCategoryPattern:
		if reg != nil {
			if p, ok := reg.Get(c.Name); ok {
				return clip(lowerFirst(firstClause(p.UseWhen())), differsTraitMax)
			}
		}
	case VisualCategoryChart:
		for _, r := range chartRules {
			if r.chartType == c.Name {
				return clip(lowerFirst(r.rationale), differsTraitMax)
			}
		}
	case VisualCategoryDiagram:
		for _, r := range diagramRules {
			if r.diagramType == c.Name {
				return clip(lowerFirst(r.rationale), differsTraitMax)
			}
		}
	case VisualCategoryPlaceholder:
		for _, r := range placeholderRules {
			if r.slideType == c.Name {
				return clip(lowerFirst(firstClause(r.rationale)), differsTraitMax)
			}
		}
	case VisualCategoryCompose:
		if c.Composition != nil {
			var names []string
			for _, l := range c.Composition.Leaves() {
				names = append(names, l.Name)
			}
			return "a raw compose slide of " + strings.Join(names, " + ")
		}
		return "a raw compose slide of several patterns"
	case VisualCategoryKind:
		return "one DeckSpec slide of 2–3 typed regions"
	}
	return "a " + categoryForm(c.Category)
}

// firstClause cuts a sentence at its first ";", " — " or ". ".
func firstClause(s string) string {
	for _, sep := range []string{";", " — ", ". ", " (", ":"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	// Keep acronyms and proper nouns ("SWOT", "KPI", "Porter's").
	if len(r) > 1 && r[1] >= 'A' && r[1] <= 'Z' {
		return s
	}
	for _, keep := range []string{"Porter", "Gantt", "Venn", "Maslow", "Harvey", "Fishbone", "Ishikawa"} {
		if strings.HasPrefix(s, keep) {
			return s
		}
	}
	return strings.ToLower(string(r[0])) + string(r[1:])
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := string(r[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:") + "…"
}

// ---------------------------------------------------------------------------
// Literal-name routing
// ---------------------------------------------------------------------------

// visualNameAliases lists, per candidate, the phrases that name it outright
// besides its own name. The key is "<category>:<name>". Both forms of a twin
// carry the shared word, so naming it lifts both and keeps their order.
var visualNameAliases = map[string][]string{
	"named_pattern:swimlane":               {"swim lane", "swimlanes", "swim lanes"},
	"named_pattern:matrix-2x2":             {"2x2", "2x2 matrix", "two by two", "2 by 2"},
	"diagram:matrix_2x2":                   {"2x2", "2x2 matrix", "two by two", "2 by 2"},
	"named_pattern:strategy-house":         {"strategy house", "strategic house", "house of strategy"},
	"diagram:house_diagram":                {"house diagram", "temple diagram"},
	"named_pattern:timeline-horizontal":    {"timeline"},
	"named_pattern:capability-heatmap":     {"heatmap", "heat map"},
	"named_pattern:risk-heatmap":           {"risk heat map", "risk heatmap", "risk matrix"},
	"diagram:heatmap":                      {"heat map"},
	"named_pattern:bmc-canvas":             {"business model canvas", "bmc"},
	"named_pattern:waterfall-bridge":       {"waterfall", "bridge chart"},
	"named_pattern:journey-maturity-model": {"maturity model", "maturity ladder", "maturity curve"},
	"named_pattern:value-chain":            {"value chain"},
	"named_pattern:driver-tree":            {"driver tree", "value driver tree", "issue tree"},
	"named_pattern:scqa-summary":           {"scqa"},
	"named_pattern:exec-summary":           {"executive summary", "exec summary"},
	"named_pattern:before-after":           {"before after", "before and after"},
	"named_pattern:cycle-ring":             {"cycle ring", "segmented cycle", "pdca", "flywheel"},
	"diagram:gantt":                        {"gantt chart", "gantt plan"},
	"diagram:org_chart":                    {"org chart", "organisation chart", "organization chart", "organigram"},
	"diagram:porters_five_forces":          {"five forces", "porter s five forces"},
	"diagram:venn":                         {"venn diagram"},
	"diagram:fishbone":                     {"ishikawa"},
	"diagram:swot":                         {"swot analysis"},
	"diagram:pestel":                       {"pestel analysis", "pestle"},
	"chart:funnel":                         {"funnel"},
	"chart:waterfall":                      {"waterfall"},
	"chart:scatter":                        {"scatter", "scatter plot", "scatterplot"},
	"chart:treemap":                        {"treemap", "tree map"},
	"chart:donut":                          {"donut", "doughnut"},
	"chart:gauge":                          {"gauge"},
	"chart:radar":                          {"radar chart", "spider chart"},
	"chart:small_multiples":                {"small multiples"},
}

// namedOutright reports whether the intent names the candidate itself: its
// own name (separators as spaces), "<type> chart" / "<type> graph" for a
// chart, or one of its aliases. Generic layout names (content, image, table)
// are excluded — their keyword rules already score them.
func namedOutright(c VisualCandidate, words []string) bool {
	switch c.Category {
	case VisualCategoryPattern, VisualCategoryDiagram:
		own := strings.NewReplacer("-", " ", "_", " ").Replace(c.Name)
		// kpi-3up and friends are count variants of one visual, not a name.
		if !strings.HasPrefix(c.Name, "kpi-") && intentContainsPhrase(words, intentWords(own)) {
			return true
		}
	case VisualCategoryChart:
		own := strings.ReplaceAll(c.Name, "_", " ")
		for _, suffix := range []string{" chart", " graph", " plot"} {
			if intentContainsPhrase(words, intentWords(own+suffix)) {
				return true
			}
		}
	default:
		return false
	}
	for _, alias := range visualNameAliases[string(c.Category)+":"+c.Name] {
		if intentContainsPhrase(words, intentWords(alias)) {
			return true
		}
	}
	return false
}

// applyLiteralNameRouting lifts the candidates the intent names outright above
// every candidate that matched only an incidental keyword: "a swimlane
// process across three teams" is a swimlane, not the team-bios page its word
// "teams" scored (go-slide-creator-bdvhj). Named candidates move by one
// shared amount, so two named forms keep their order; nothing is added.
func applyLiteralNameRouting(all []VisualCandidate, intentLower string) {
	words := intentWords(intentLower)
	named := make([]bool, len(all))
	bestNamed, bestOther := 0.0, 0.0
	any := false
	for i, c := range all {
		if c.Score <= 0 {
			continue
		}
		if namedOutright(c, words) && !shadowedChartName(c, all, words) {
			named[i], any = true, true
			bestNamed = math.Max(bestNamed, c.Score)
		} else if c.Category != VisualCategoryCompose && c.Category != VisualCategoryKind {
			bestOther = math.Max(bestOther, c.Score)
		}
	}
	if !any || bestNamed >= bestOther+differsByMargin+0.01 {
		return
	}
	// Named candidates rise together to clear the best unnamed one by more
	// than a near tie; when the scale's top stops them, the unnamed ones are
	// capped below instead.
	target := math.Min(1, bestOther+differsByMargin+0.01)
	shift := target - bestNamed
	for i := range all {
		if !named[i] {
			continue
		}
		all[i].Score = roundScore(math.Min(1, all[i].Score+shift))
		all[i].Rationale += "; the intent names this visual"
		all[i].ConfidenceBand = confidenceBand(all[i].Score)
	}
	ceiling := roundScore(target - differsByMargin - 0.01)
	for i := range all {
		if named[i] || all[i].Category == VisualCategoryCompose || all[i].Category == VisualCategoryKind || all[i].Score <= ceiling {
			continue
		}
		all[i].Score = ceiling
		all[i].ConfidenceBand = confidenceBand(ceiling)
	}
}

// shadowedChartName reports whether a chart is named only as part of a longer
// chart name the intent also names: "a stacked bar chart" names stacked_bar,
// not bar.
func shadowedChartName(c VisualCandidate, all []VisualCandidate, words []string) bool {
	if c.Category != VisualCategoryChart {
		return false
	}
	for _, o := range all {
		if o.Category == VisualCategoryChart && o.Name != c.Name && o.Score > 0 &&
			strings.Contains(o.Name, c.Name) && namedOutright(o, words) {
			return true
		}
	}
	return false
}

// applyTwinCueRouting breaks a tie between the two forms of one visual when
// the intent asks for something only one of them does ("one highlighted
// step" → the value-chain pattern; "6 plotted items" → the matrix_2x2
// diagram).
func applyTwinCueRouting(all []VisualCandidate, intentLower string) {
	words := intentWords(intentLower)
	cs := candidateSet(all)
	for _, tw := range visualTwins {
		p := cs.find(VisualCategoryPattern, tw.pattern)
		o := cs.find(tw.otherCat, tw.other)
		if p == nil || o == nil || p.Score <= 0 || o.Score <= 0 {
			continue
		}
		wantsPattern := len(tw.patternCues) > 0 && intentHasAny(words, tw.patternCues...)
		wantsOther := len(tw.otherCues) > 0 && intentHasAny(words, tw.otherCues...)
		if wantsPattern == wantsOther {
			continue
		}
		win, lose, only := p, o, "only the pattern "+tw.patternOnly
		if wantsOther {
			win, lose, only = o, p, "only the "+categoryForm(tw.otherCat)+" "+tw.otherOnly
		}
		// The winner clears the loser by more than a near tie.
		gap := differsByMargin + 0.01
		if win.Score < lose.Score+gap {
			if lose.Score+gap <= 1 {
				setCandidateScore(win, lose.Score+gap, only)
			} else {
				setCandidateScore(win, 1, only)
				setCandidateScore(lose, 1-gap, "")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Chart type spellings
// ---------------------------------------------------------------------------

// chartTypeSynonyms maps other words for a chart type onto the canonical one.
var chartTypeSynonyms = map[string]string{
	"column":      "bar",
	"doughnut":    "donut",
	"bar-stacked": "stacked_bar",
	"spider":      "radar",
}

// CanonicalChartType returns the canonical name of a chart type and whether
// the name is a chart type at all. The canonical spelling is the short one
// get_chart_capabilities lists ("bar", "line", "stacked_bar"); the "_chart"
// spelling ("bar_chart", "line_chart") and hyphens are accepted everywhere and
// resolve to it.
func CanonicalChartType(name string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.TrimSuffix(n, "_chart")
	if syn, ok := chartTypeSynonyms[n]; ok {
		n = syn
	}
	if syn, ok := chartTypeSynonyms[strings.ReplaceAll(n, "_", "-")]; ok {
		n = syn
	}
	for _, r := range chartRules {
		if r.chartType == n {
			return n, true
		}
	}
	return "", false
}

// canonicalCandidateName rewrites a caller-supplied candidate name to the
// spelling the catalogs use: a chart type in its "_chart" spelling becomes the
// canonical short name, inside compose names too ("compose:chart:line_chart+…").
// A name that is a pattern, a diagram or a layout is left alone.
func canonicalCandidateName(reg *Registry, name string) string {
	if strings.HasPrefix(name, "compose:") {
		parts := strings.Split(strings.TrimPrefix(name, "compose:"), "+")
		for i, p := range parts {
			if t, ok := strings.CutPrefix(strings.TrimSpace(p), "chart:"); ok {
				if canon, isChart := CanonicalChartType(t); isChart {
					parts[i] = "chart:" + canon
				}
			}
		}
		return "compose:" + strings.Join(parts, "+")
	}
	if reg != nil {
		if _, ok := reg.Get(name); ok {
			return name
		}
	}
	for _, r := range diagramRules {
		if r.diagramType == name {
			return name
		}
	}
	for _, r := range placeholderRules {
		if r.slideType == name {
			return name
		}
	}
	if canon, ok := CanonicalChartType(name); ok {
		return canon
	}
	return name
}

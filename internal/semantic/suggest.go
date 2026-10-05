package semantic

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/svggen"
)

// Nearest-match help for an unknown kind or key (go-slide-creator-t0c1m).
//
// The agent journey review wrote kind "funnel" and was handed the list of all
// 27 kinds; "takeway" got a did_you_mean but "bulets" (for items) and "stages"
// (for steps) only the list of accepted keys. An unknown name now always gets
// the nearest registered one when there is a close match: by spelling, by the
// chart or diagram type it names, or by the word family it belongs to.

// KindSuggestion says what an unregistered slide kind most likely means.
type KindSuggestion struct {
	// DidYouMean is the kind to use instead, or "" when nothing is close.
	DidYouMean SlideKind
	// HostedType is set when the name is a chart or diagram type rather than a
	// kind: the type as the hosting kind's payload spells it.
	HostedType string
	// HostedAs is "chart" or "diagram" for a HostedType.
	HostedAs string
	// Hint is the sentence a finding carries: which kind to use, and for a
	// hosted type, where the type goes.
	Hint string
}

// diagramHostKinds maps a diagram type to the slide kind that draws it. A type
// without an entry has no kind of its own and goes through raw_json2pptx.
var diagramHostKinds = map[string]SlideKind{
	"org_chart":             KindOrg,
	"timeline":              KindTimeline,
	"matrix_2x2":            KindMatrix2x2,
	"process_flow":          KindProcess,
	"house_diagram":         KindPillars,
	"gantt":                 KindRoadmap,
	"kpi_dashboard":         KindKPISnapshot,
	"stat_cards":            KindKPISnapshot,
	"swot":                  KindFramework,
	"porters_five_forces":   KindFramework,
	"business_model_canvas": KindFramework,
}

// chartHostKinds maps a chart type to a kind that draws it as more than a
// chart; every other chart type is hosted by chart_insight.
var chartHostKinds = map[string]SlideKind{
	"waterfall": KindBridge,
}

// kindWordHints maps a word an author reaches for onto the kind that carries
// it, beyond the registered spellings in kindSpelling.
var kindWordHints = map[string]SlideKind{
	"kpi": KindKPISnapshot, "kpis": KindKPISnapshot, "metrics": KindKPISnapshot,
	"summary": KindExecutiveSummary, "exec_summary": KindExecutiveSummary,
	"steps": KindProcess, "flow": KindProcess, "workflow": KindProcess,
	"chart": KindChartInsight, "graph": KindChartInsight,
	"cover": KindTitle, "divider": KindSection,
	"options": KindOptionMatrix, "matrix": KindMatrix2x2,
	"heatmap": KindRiskHeatmap, "heat_map": KindRiskHeatmap, "risk_matrix": KindRiskHeatmap, "risks": KindRiskHeatmap,
	"image": KindImageCase, "case_study": KindImageCase,
	"people": KindTeam, "house": KindPillars, "milestones": KindTimeline,
	"thank_you": KindClosing, "end": KindClosing,
}

// SuggestKind returns the nearest-match help for an unregistered kind.
func SuggestKind(kind string) KindSuggestion {
	name := strings.ToLower(strings.TrimSpace(kind))
	name = strings.NewReplacer("-", "_", " ", "_").Replace(name)
	if name == "" {
		return KindSuggestion{}
	}
	if SlideKind(name).Valid() {
		// A registered kind written with other case, spaces or hyphens.
		return KindSuggestion{DidYouMean: SlideKind(name), Hint: fmt.Sprintf("use %q", name)}
	}
	if canonical, ok := kindSpelling[SlideKind(name)]; ok {
		return KindSuggestion{DidYouMean: canonical, Hint: fmt.Sprintf("use %q", canonical)}
	}
	if s, ok := hostedTypeSuggestion(name); ok {
		return s
	}
	if k, ok := kindWordHints[name]; ok {
		return KindSuggestion{DidYouMean: k, Hint: fmt.Sprintf("did you mean %q?", k)}
	}
	if k := nearestKind(name); k != "" {
		return KindSuggestion{DidYouMean: k, Hint: fmt.Sprintf("did you mean %q?", k)}
	}
	return KindSuggestion{}
}

// hostedTypeSuggestion resolves a name that is a chart or diagram type.
func hostedTypeSuggestion(name string) (KindSuggestion, bool) {
	bare := strings.TrimSuffix(strings.TrimSuffix(name, "_chart"), "_diagram")
	for _, c := range svggen.ChartCapabilities() {
		if c.Status != "ready" || (c.Type != name && c.Type != bare) {
			continue
		}
		if host, ok := chartHostKinds[c.Type]; ok {
			return KindSuggestion{DidYouMean: host, HostedType: c.Type, HostedAs: "chart",
				Hint: fmt.Sprintf("%q is a chart type, not a slide kind: use kind %q, which draws it", c.Type, host)}, true
		}
		shape, _ := chartDataShape(c.Type)
		return KindSuggestion{DidYouMean: KindChartInsight, HostedType: c.Type, HostedAs: "chart",
			Hint: fmt.Sprintf("%q is a chart type, not a slide kind: use kind %q with chart {type: %q, data: %s}", c.Type, KindChartInsight, c.Type, shape)}, true
	}
	for _, d := range svggen.DiagramCapabilitiesReady() {
		match := d.Type == name || d.Type == bare
		for _, alias := range d.Aliases {
			match = match || alias == name
		}
		if !match {
			continue
		}
		if host, ok := diagramHostKinds[d.Type]; ok {
			return KindSuggestion{DidYouMean: host, HostedType: d.Type, HostedAs: "diagram",
				Hint: fmt.Sprintf("%q is a diagram type, not a slide kind: use kind %q, which draws it", d.Type, host)}, true
		}
		return KindSuggestion{DidYouMean: KindRawJSON2pptx, HostedType: d.Type, HostedAs: "diagram",
			Hint: fmt.Sprintf("%q is a diagram type with no slide kind of its own: use kind %q with slide.content [{placeholder_id: \"body\", type: \"diagram\", diagram_value: {type: %q, data: {…}}}]", d.Type, KindRawJSON2pptx, d.Type)}, true
	}
	return KindSuggestion{}, false
}

// nearestKind returns the registered kind closest in spelling, the one kind
// the name is a word of ("snapshot" → kpi_snapshot), or "".
func nearestKind(name string) SlideKind {
	best, bestDist := SlideKind(""), -1
	var wordOf []SlideKind
	for _, k := range AllSlideKinds() {
		d := editDistance(name, string(k))
		if bestDist < 0 || d < bestDist {
			best, bestDist = k, d
		}
		for _, word := range strings.Split(string(k), "_") {
			if word == name && len(name) >= 4 {
				wordOf = append(wordOf, k)
			}
		}
	}
	limit := 2
	if len(name) >= 8 {
		limit = 3
	}
	if bestDist >= 0 && bestDist <= limit {
		return best
	}
	if len(wordOf) == 1 {
		return wordOf[0]
	}
	return ""
}

// unknownKindMessage is the one wording of an unknown-kind finding; the parser
// and the validator both use it, so the two passes collapse to one finding.
// With a suggestion the message leads with it; the full list of kinds is in the
// finding's evidence either way.
func unknownKindMessage(kind string) string {
	if s := SuggestKind(kind); s.Hint != "" {
		return fmt.Sprintf("unknown slide kind %q; %s", kind, s.Hint)
	}
	return fmt.Sprintf("unknown slide kind %q; expected one of %s", kind, joinKinds())
}

// fieldFamilies groups the names authors use interchangeably for one idea. An
// unknown key that belongs to a family is most likely the accepted key of the
// same family; each family is listed in order of preference.
var fieldFamilies = [][]string{
	{"items", "bullets", "points", "entries", "lines", "list", "content"},
	{"steps", "stages", "phases", "milestones", "activities"},
	{"kpis", "metrics", "stats", "numbers", "figures"},
	{"title", "header", "heading", "headline", "name", "label"},
	{"description", "detail", "details", "body", "text", "desc", "summary", "caption"},
	{"takeaway", "conclusion", "so_what", "key_message", "message", "insight"},
	{"source", "sources", "citation", "footnote"},
	{"columns", "cols", "sides"},
	{"owner", "who", "responsible", "assignee", "lead"},
	{"date", "when", "due", "deadline", "timing"},
	{"image", "photo", "picture", "img"},
	{"members", "people", "team"},
	{"options", "choices", "alternatives"},
	{"value", "number", "figure", "amount", "metric"},
	{"delta", "change", "trend", "comparator"},
}

// suggestKey returns the accepted key an unknown key most likely means: the
// nearest in spelling, else the accepted key of the same word family (after
// correcting the unknown key's own spelling against the family words), else "".
func suggestKey(key string, known []string) string {
	if sug := closestKey(key, known); sug != "" {
		return sug
	}
	lower := strings.ToLower(key)
	for _, family := range fieldFamilies {
		if !inFamily(lower, family) {
			continue
		}
		for _, word := range family {
			if word != lower && containsKey(known, word) {
				return word
			}
		}
	}
	return ""
}

// inFamily reports whether key is one of the family's words, allowing one typo
// in a word of six letters or more ("bulets" for bullets).
func inFamily(key string, family []string) bool {
	for _, word := range family {
		if word == key || (len(word) >= 6 && editDistance(key, word) <= 1) {
			return true
		}
	}
	return false
}

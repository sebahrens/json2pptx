package deckplan

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// Same-slide spatial requirements (go-slide-creator-vae7f).
//
// A brief that says where things go on ONE slide — "left two-thirds a line
// chart of quarterly revenue; upper right a 32% gross margin KPI; lower right
// a three-step launch timeline" — used to be planned as narrative slots: the
// region structure vanished, the KPI fact went unplaced and the revenue facts
// landed on the closing slide. The planner now reads those clauses with a
// small, documented cue grammar (not general language understanding) and
// drafts one mixed-region slide: a DeckSpec `regions` slide, or the raw
// shape_grid that kind compiles to.
//
// The grammar: the brief is cut into segments at sentence ends, semicolons,
// colons and newlines (never inside brackets). A region clause is a segment
// that STARTS with a position — left, right, top, bottom, upper/lower/top/
// bottom left/right, centre/middle, optionally "-hand", "side", "half",
// "column", "panel", … — optionally followed by a share ("two-thirds",
// "half", "a third", "40%"), and names one visual: chart (line / bar / column
// / area / pie / donut / stacked bar), KPI / metric / stat, KPIs, timeline /
// milestones / roadmap, table, image / photo / screenshot, or text / narrative
// / commentary / bullets. Two or more region clauses whose positions form a
// supported arrangement make a request; a segment just before them that talks
// about regions, panels or dividing the slide is its lead-in. Everything else
// in the brief is planned as before.

// RegionSlot is one region of a planned mixed-region slide: where it sits,
// its role in the slide's hierarchy, the region kind drafted for it, and the
// brief clause that asked for it.
type RegionSlot struct {
	// Path is the region's location in the draft: deck_spec
	// "slides[1].regions[0]", or the raw skeleton's
	// "shape_grid.rows[0].cells[0]".
	Path string `json:"path"`
	// Position is the brief's placement ("left", "upper right", …).
	Position string `json:"position"`
	// Role is "main" for the region the others are arranged around (or the
	// first of equal columns / rows) and "supporting" for the rest.
	Role string `json:"role"`
	// Kind is the region kind drafted: chart, stat, kpis, table, timeline,
	// image or text.
	Kind string `json:"kind"`
	// Visual is the brief's own words for the visual ("line chart").
	Visual string `json:"visual"`
	// SizePct is the share the brief gave the region, when it gave one.
	SizePct float64 `json:"size_pct,omitempty"`
	// Facts are the brief clauses that belong in this region, verbatim.
	Facts []string `json:"facts"`
}

// UnsupportedRegion is a region requirement the plan could not draft as
// asked: an unknown visual or an arrangement outside the supported set.
type UnsupportedRegion struct {
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// regionRequest is a parsed same-slide spatial request.
type regionRequest struct {
	arrangement string
	regions     []regionReq // in regions-kind order: main first
	unsupported []UnsupportedRegion
	// remainder is the brief with the request (and its lead-in) cut out, so
	// its facts are not routed a second time to other slides.
	remainder string
}

type regionReq struct {
	pos       string // L R T B TL TR BL BR C
	position  string // brief wording, normalised
	kind      string
	visual    string
	chartType string
	size      float64
	count     int // a "three-step" / "3 milestones" count, 0 when unstated
	text      string
}

var (
	regionPosition = regexp.MustCompile(`(?i)^(?:(?:in|on|at|across)\s+)?(?:the\s+)?((?:upper|top|lower|bottom)[- ]?(?:left|right)|left|right|top|bottom|upper|lower|cent(?:re|er)|middle)(?:[- ]hand)?(?:\s+(?:side|column|panel|region|area|corner|band|part|section))?\b`)
	// regionShare reads the share right after the position: a fraction word
	// ("two-thirds", "a third", "half"), or a bare percent followed by a
	// boundary or an article ("left 60%, a chart", "left 60% a chart") — "a
	// 32% gross margin KPI" is the KPI's value, not the region's share.
	regionShare   = regexp.MustCompile(`(?i)^[\s,]*(?:(?:the|a|an|one)\s+)?(two[- ]thirds|three[- ]quarters|third|half|quarter)\b|^[\s,]*((\d{1,2})\s*%)(?:$|\s*[,(]|\s+(?:of|width|height|share|wide|tall|a|an|the)\b)`)
	regionIntro   = regexp.MustCompile(`(?i)\b(?:regions?|panels?|divide[sd]?|split|canvas|quadrants?|side by side|same slide|one slide|layout)\b`)
	regionSegment = regexp.MustCompile(`[;\n]+|[.!?]+(?:\s+|$)|:\s+`)
	regionCount   = regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|[2-7])[- ](?:step|stage|phase|milestone|point|item)s?\b|\b(two|three|four|five|six|seven|[2-7])\s+(?:steps|stages|phases|milestones|metrics|kpis)\b`)
)

// regionCue maps a visual word to the region kind that draws it. An empty
// kind marks a visual no region kind draws.
type regionCue struct {
	re   *regexp.Regexp
	kind string
}

var regionCues = []regionCue{
	{regexp.MustCompile(`(?i)\b(?:org(?:anisation|anization|anizational|anisational)?\s+chart|maps?|videos?|gauges?|funnels?|heat\s?maps?|word\s+clouds?|venn(?:\s+diagram)?|diagrams?|matrix)\b`), ""},
	{regexp.MustCompile(`(?i)\b(?:(?:line|bar|column|area|pie|donut|doughnut|stacked\s+bar|waterfall)\s+)?(?:charts?|graphs?|plots?)\b|\btrend\s?lines?\b`), slides.RegionChart},
	{regexp.MustCompile(`(?i)\b(?:kpis|kpi\s+(?:tiles|cards|strip|row)|metrics|scorecard)\b`), slides.RegionKPIs},
	{regexp.MustCompile(`(?i)\b(?:kpi|metric|stat|statistic|big\s+number|headline\s+number|hero\s+number)\b`), slides.RegionStat},
	{regexp.MustCompile(`(?i)\b(?:timeline|milestones|roadmap|schedule)\b`), slides.RegionTimeline},
	{regexp.MustCompile(`(?i)\btables?\b`), slides.RegionTable},
	{regexp.MustCompile(`(?i)\b(?:image|photo|picture|screenshot|logo)\b`), slides.RegionImage},
	{regexp.MustCompile(`(?i)\b(?:text|narrative|commentary|bullets|bullet\s+points|summary|insights?|takeaways|explanation|notes|callout)\b`), slides.RegionText},
}

var chartTypeWords = map[string]string{
	"line": "line_chart", "bar": "bar_chart", "column": "bar_chart", "waterfall": "bar_chart",
	"area": "area_chart", "pie": "pie_chart", "donut": "donut_chart", "doughnut": "donut_chart",
	"stacked bar": "stacked_bar_chart",
}

var countWords = map[string]int{"two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7}

// regionArrangements maps a sorted position set to its arrangement and the
// position order of its regions (main first).
var regionArrangements = map[string]struct {
	arrangement string
	order       []string
}{
	"L,R":     {slides.ArrangeColumns, []string{"L", "R"}},
	"C,L,R":   {slides.ArrangeColumns, []string{"L", "C", "R"}},
	"B,T":     {slides.ArrangeRows, []string{"T", "B"}},
	"B,C,T":   {slides.ArrangeRows, []string{"T", "C", "B"}},
	"BR,L,TR": {slides.ArrangeMainLeft, []string{"L", "TR", "BR"}},
	"BL,R,TL": {slides.ArrangeMainRight, []string{"R", "TL", "BL"}},
	"BL,BR,T": {slides.ArrangeMainTop, []string{"T", "BL", "BR"}},
	"B,TL,TR": {slides.ArrangeMainBottom, []string{"B", "TL", "TR"}},
}

// parseRegionRequest reads a same-slide spatial request from the brief, or
// returns nil when the brief places fewer than two visuals.
func parseRegionRequest(brief string) *regionRequest {
	segs := regionSegments(brief)
	type found struct {
		seg int
		req regionReq
	}
	var hits []found
	for i := 0; i < len(segs); i++ {
		stub := strings.TrimSpace(brief[segs[i][0]:segs[i][1]])
		r, ok := parseRegionClause(stub)
		if !ok && i+1 < len(segs) && len(strings.Fields(stub)) <= 4 && regionPosition.MatchString(stub) {
			// "left two-thirds: a line chart …" — the colon cut the position
			// from its visual.
			merged := [2]int{segs[i][0], segs[i+1][1]}
			if r, ok = parseRegionClause(brief[merged[0]:merged[1]]); ok {
				segs[i] = merged
				segs = append(segs[:i+1], segs[i+2:]...)
			}
		}
		if ok {
			hits = append(hits, found{i, r})
		}
	}
	if len(hits) < 2 {
		return nil
	}

	req := &regionRequest{}
	cut := map[int]bool{}
	if first := hits[0].seg; first > 0 && regionIntro.MatchString(brief[segs[first-1][0]:segs[first-1][1]]) {
		cut[first-1] = true
	}

	byPos := map[string]regionReq{}
	var keys []string
	for _, h := range hits {
		if _, dup := byPos[h.req.pos]; dup {
			// Reported, and left in the brief so its facts are still planned.
			req.unsupported = append(req.unsupported, UnsupportedRegion{Text: h.req.text,
				Reason: fmt.Sprintf("a second region at %q: each position holds one region", h.req.position)})
			continue
		}
		cut[h.seg] = true
		byPos[h.req.pos] = h.req
		keys = append(keys, h.req.pos)
	}
	arr, ok := regionArrangements[sortedJoin(keys)]
	if !ok {
		// The positions do not form a supported arrangement: report every
		// clause and leave the brief whole, so its facts are planned as before
		// rather than lost.
		placed := make([]string, len(hits))
		for i, h := range hits {
			placed[i] = h.req.position
		}
		for _, h := range hits {
			req.unsupported = append(req.unsupported, UnsupportedRegion{Text: h.req.text,
				Reason: "the placed regions (" + strings.Join(placed, ", ") + ") are not a supported arrangement: two or three side by side, two or three stacked, or one main region beside, above or below a stack of two. Plan the slide yourself with kind: regions or a raw_json2pptx compose"})
		}
		req.remainder = brief
		return req
	}
	req.arrangement = arr.arrangement
	for _, p := range arr.order {
		r := byPos[p]
		if r.kind == "" {
			req.unsupported = append(req.unsupported, UnsupportedRegion{Text: r.text,
				Reason: fmt.Sprintf("no region kind draws a %s (regions hold %s); drafted as a text region holding the clause — replace it with a supported kind, or author the slide as a raw_json2pptx compose", r.visual, strings.Join(slides.RegionKinds, ", "))})
			r.kind = slides.RegionText
		}
		req.regions = append(req.regions, r)
	}
	var rest strings.Builder
	last := 0
	for i, s := range segs {
		if !cut[i] {
			continue
		}
		rest.WriteString(brief[last:s[0]])
		rest.WriteString(" ")
		last = s[1]
	}
	rest.WriteString(brief[last:])
	req.remainder = strings.TrimSpace(rest.String())
	return req
}

// regionSegments cuts the brief at sentence ends, semicolons, colons and
// newlines outside brackets, returning each segment's [start, end) span with
// the separator excluded.
func regionSegments(brief string) [][2]int {
	depths := bracketDepths(brief)
	var out [][2]int
	start := 0
	for _, loc := range regionSegment.FindAllStringIndex(brief, -1) {
		if loc[0] > 0 && depths[loc[0]-1] > 0 {
			continue
		}
		out = append(out, [2]int{start, loc[0]})
		start = loc[1]
	}
	if start < len(brief) {
		out = append(out, [2]int{start, len(brief)})
	}
	return out
}

// parseRegionClause reads one segment as a region clause.
func parseRegionClause(seg string) (regionReq, bool) {
	text := strings.TrimSpace(seg)
	m := regionPosition.FindStringSubmatchIndex(text)
	if m == nil {
		return regionReq{}, false
	}
	position := strings.ToLower(strings.ReplaceAll(text[m[2]:m[3]], "-", " "))
	r := regionReq{pos: positionCode(position), position: position, text: text}
	rest := text[m[1]:]
	if sm := regionShare.FindStringSubmatch(rest); sm != nil {
		r.size = shareOf(sm)
	}
	start, end, kind := -1, -1, ""
	for _, cue := range regionCues {
		loc := cue.re.FindStringIndex(rest)
		if loc == nil {
			continue
		}
		if start < 0 || loc[0] < start || (loc[0] == start && loc[1] > end) {
			start, end, kind = loc[0], loc[1], cue.kind
		}
	}
	if start < 0 {
		return regionReq{}, false
	}
	r.visual = strings.ToLower(rest[start:end])
	r.kind = kind
	if kind == slides.RegionChart {
		r.chartType = "bar_chart"
		for word, ct := range chartTypeWords {
			if strings.HasPrefix(r.visual, word+" ") {
				r.chartType = ct
			}
		}
	}
	if cm := regionCount.FindStringSubmatch(rest); cm != nil {
		word := strings.ToLower(cm[1] + cm[2])
		if n, ok := countWords[word]; ok {
			r.count = n
		} else if n, err := strconv.Atoi(word); err == nil {
			r.count = n
		}
	}
	return r, true
}

func positionCode(p string) string {
	p = strings.ReplaceAll(p, " ", "")
	switch p {
	case "left":
		return "L"
	case "right":
		return "R"
	case "top", "upper":
		return "T"
	case "bottom", "lower":
		return "B"
	case "centre", "center", "middle":
		return "C"
	}
	vert := "B"
	if strings.HasPrefix(p, "upper") || strings.HasPrefix(p, "top") {
		vert = "T"
	}
	if strings.HasSuffix(p, "left") {
		return vert + "L"
	}
	return vert + "R"
}

func shareOf(sm []string) float64 {
	if sm[3] != "" {
		n, _ := strconv.Atoi(sm[3])
		return float64(n)
	}
	switch strings.ToLower(strings.ReplaceAll(sm[1], "-", " ")) {
	case "two thirds":
		return 67
	case "three quarters":
		return 75
	case "third":
		return 33
	case "half":
		return 50
	case "quarter":
		return 25
	}
	return 0
}

func sortedJoin(keys []string) string {
	out := append([]string(nil), keys...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return strings.Join(out, ",")
}

// draftable reports whether the request carries a region slide to draft.
func (q *regionRequest) draftable() bool {
	return q != nil && len(q.regions) > 0
}

// regionsGuidance is the slot guidance of a drafted regions slide.
const regionsGuidance = "The one slide the brief laid out in regions: fill each region's fields (list_slide_kinds kinds:[\"regions\"]) from its facts, keep the arrangement and size_pct, and title the slide with the claim the regions make together."

// draftBody builds the regions slide body: the arrangement and one fillable
// region per requirement, with the shares the brief gave.
func (q *regionRequest) draftBody() map[string]any {
	regions := make([]any, len(q.regions))
	main := slides.IsMainArrangement(q.arrangement)
	for i, r := range q.regions {
		region := draftRegion(r)
		if r.size > 0 && (i == 0 || !main) {
			region["size_pct"] = clampShare(r.size)
		}
		regions[i] = region
	}
	return map[string]any{
		"kind":        "regions",
		"title":       patterns.FillPlaceholder,
		"arrangement": q.arrangement,
		"regions":     regions,
		"takeaway":    patterns.FillPlaceholder,
	}
}

func clampShare(v float64) float64 {
	return max(float64(slides.RegionMinSizePct), min(float64(slides.RegionMaxSizePct), v))
}

// draftRegion is one fillable region of the given kind.
func draftRegion(r regionReq) map[string]any {
	fill := patterns.FillPlaceholder
	switch r.kind {
	case slides.RegionChart:
		return map[string]any{"kind": r.kind, "heading": fill, "chart": map[string]any{
			"type": r.chartType,
			"data": map[string]any{"categories": []any{fill}, "series": []any{map[string]any{"name": fill, "values": []any{fill}}}},
		}}
	case slides.RegionStat:
		return map[string]any{"kind": r.kind, "value": fill, "label": fill}
	case slides.RegionKPIs:
		n := min(max(r.count, slides.RegionKPIMin), slides.RegionKPIMax)
		kpis := make([]any, n)
		for i := range kpis {
			kpis[i] = map[string]any{"value": fill, "label": fill}
		}
		return map[string]any{"kind": r.kind, "kpis": kpis}
	case slides.RegionTable:
		return map[string]any{"kind": r.kind, "headers": []any{fill, fill}, "rows": []any{[]any{fill, fill}}}
	case slides.RegionTimeline:
		n := min(max(r.count, 3), 7)
		stops := make([]any, n)
		for i := range stops {
			stops[i] = map[string]any{"label": fill, "date": fill}
		}
		return map[string]any{"kind": r.kind, "milestones": stops}
	case slides.RegionImage:
		return map[string]any{"kind": r.kind, "image": map[string]any{"path": fill, "alt": fill}}
	default:
		return map[string]any{"kind": slides.RegionText, "body": fill}
	}
}

// regionSlots annotates the drafted regions: their roles, kinds, shares and
// facts. pathOf maps a region index to its location in the draft.
func (q *regionRequest) regionSlots(pathOf func(int) string) []RegionSlot {
	out := make([]RegionSlot, len(q.regions))
	for i, r := range q.regions {
		role := "supporting"
		if i == 0 {
			role = "main"
		}
		out[i] = RegionSlot{
			Path: pathOf(i), Position: r.position, Role: role, Kind: r.kind,
			Visual: r.visual, SizePct: r.size, Facts: []string{r.text},
		}
	}
	return out
}

// regionSlotsFromFields annotates the regions of a drafted regions body (an
// outline item's) when no parsed request is behind it: kind and role per
// region, the main region first, every region carrying the slot's facts.
func regionSlotsFromFields(fields map[string]any, facts []string, pathOf func(int) string) []RegionSlot {
	regions, _ := fields["regions"].([]any)
	arrangement, _ := fields["arrangement"].(string)
	out := make([]RegionSlot, 0, len(regions))
	for i, r := range regions {
		region, _ := r.(map[string]any)
		kind, _ := region["kind"].(string)
		role, position := "supporting", "right"
		switch {
		case i == 0:
			role, position = "main", "left"
		case arrangement == slides.ArrangeMainLeft && i == 1:
			position = "upper right"
		case arrangement == slides.ArrangeMainLeft && i == 2:
			position = "lower right"
		}
		size, _ := region["size_pct"].(float64)
		out = append(out, RegionSlot{Path: pathOf(i), Position: position, Role: role, Kind: kind, Visual: kind, SizePct: size, Facts: append([]string{}, facts...)})
	}
	return out
}

// facts are every region clause, verbatim.
func (q *regionRequest) facts() []string {
	out := make([]string, len(q.regions))
	for i, r := range q.regions {
		out[i] = r.text
	}
	return out
}

// rawRegionsSlide compiles the regions draft to the raw slide it renders as
// (the skeleton of a raw plan) and maps each region to its cell path.
func (q *regionRequest) rawRegionsSlide() (json.RawMessage, func(int) string, error) {
	body := q.draftBody()
	delete(body, "kind")
	in := slides.Input{OutputIndex: 0, Title: patterns.FillPlaceholder, Takeaway: patterns.FillPlaceholder, Body: body}
	slide, links, err := slides.CompileRegions(in)
	if err != nil {
		return nil, nil, err
	}
	paths := map[int]string{}
	for _, l := range links {
		var k int
		var tail string
		if n, _ := fmt.Sscanf(l.SemanticPath, "slides[0].regions[%d]%s", &k, &tail); n == 1 {
			if _, seen := paths[k]; !seen {
				paths[k] = strings.TrimPrefix(l.RawPath, "slides[0].")
			}
		}
	}
	raw, err := json.Marshal(slide)
	if err != nil {
		return nil, nil, err
	}
	return raw, func(i int) string { return paths[i] }, nil
}

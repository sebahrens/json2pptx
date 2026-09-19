package generator

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Alt text for charts, diagrams and tables (go-slide-creator-6e8h).
//
// Every chart was embedded with descr="Revenue ($M) (bar_chart)" — the title
// and the type name, which is exactly what a screen reader then announced: the
// shape of the picture and nothing in it. Native diagram groups and table
// frames announced nothing at all, because their cNvPr carried no descr.
//
// The generated sentence is a fallback. An authored `alt` on the chart,
// diagram or table always wins: the agent writing the deck knows what the
// visual is FOR, and no derivation can.

// altMaxLen caps a generated description. Screen readers announce the whole
// string, so a paragraph is worse than a sentence.
const altMaxLen = 220

// altMaxSeriesNames is how many series a description names before it stops
// listing and lets the count speak for the rest.
const altMaxSeriesNames = 3

// DiagramAltTextFor returns the description for a diagram or chart — the
// author's own alt when they wrote one, else a sentence derived from the data.
// Charts reach the renderer as a DiagramSpec (ChartSpec.ToDiagramSpec carries
// the alt across), so this one function covers both.
func DiagramAltTextFor(spec *types.DiagramSpec) string {
	if spec == nil {
		return "Diagram"
	}
	if alt := strings.TrimSpace(spec.Alt); alt != "" {
		return truncateAlt(alt)
	}
	return truncateAlt(describeDiagram(spec))
}

// TableAltText returns the description for a table frame.
func TableAltText(spec *types.TableSpec) string {
	if spec == nil {
		return "Table"
	}
	if alt := strings.TrimSpace(spec.Alt); alt != "" {
		return truncateAlt(alt)
	}
	return truncateAlt(describeTable(spec))
}

// describeDiagram builds a sentence from whatever the payload actually says.
// For a chart that is "Bar chart, Quarterly revenue ($M). 4 categories, 1
// series (Revenue), values from 34 to 48."; for a structural diagram it is
// "Process flow, How the migration runs. 5 steps."
func describeDiagram(spec *types.DiagramSpec) string {
	head := humanTypeName(spec.Type)
	if t := strings.TrimSpace(spec.Title); t != "" {
		head += ", " + t
	}

	facts := chartFacts(spec.Data)
	if len(facts) == 0 {
		if n, noun := diagramItemCount(spec); n > 0 {
			facts = append(facts, pluralCount(n, noun, noun+"s"))
		}
	}
	if len(facts) == 0 {
		return head + "."
	}
	return head + ". " + strings.Join(facts, ", ") + "."
}

// describeTable builds "Table, 4 columns by 6 rows. Columns: Segment, FY25,
// FY26, Growth." — its size and what the columns are.
func describeTable(spec *types.TableSpec) string {
	cols := len(spec.Headers)
	rows := len(spec.Rows)
	if cols == 0 && rows == 0 {
		return "Table."
	}
	head := fmt.Sprintf("Table, %s by %s",
		pluralCount(cols, "column", "columns"), pluralCount(rows, "row", "rows"))
	if cols == 0 {
		return head + "."
	}
	return head + ". Columns: " + strings.Join(spec.Headers, ", ") + "."
}

// chartShape is what a chart payload says about itself: how many categories it
// has, what its series are called, and the range its values span.
type chartShape struct {
	categories  int
	seriesNames []string
	min, max    float64
	haveRange   bool
}

// empty reports a payload that said nothing measurable, so the caller falls
// back to the structural item count.
func (c chartShape) empty() bool {
	return c.categories == 0 && len(c.seriesNames) == 0 && !c.haveRange
}

// chartFacts describes a chart-shaped payload: how many categories and series
// it has, what the series are called, and the range the values span. It
// returns nil when the payload is not chart-shaped, so a structural diagram
// falls through to its item count instead.
func chartFacts(data map[string]any) []string {
	if fact, ok := gaugeFact(data); ok {
		return []string{fact}
	}
	shape, ok := chartDataShape(data)
	if !ok || shape.empty() {
		return nil
	}

	var facts []string
	if shape.categories > 0 {
		facts = append(facts, pluralCount(shape.categories, "category", "categories"))
	}
	if len(shape.seriesNames) > 0 {
		facts = append(facts, seriesFact(shape.seriesNames))
	}
	if shape.haveRange {
		facts = append(facts, fmt.Sprintf("values from %s to %s",
			formatAltNumber(shape.min), formatAltNumber(shape.max)))
	}
	return facts
}

// altPlaceholderSeries is the name ChartSpec.ToDiagramSpec gives the lone
// series it synthesizes from flat label→value data. It is the engine's own
// label, not the author's, so a description that reads it back adds nothing.
const altPlaceholderSeries = "Data"

// seriesFact renders "2 series (Revenue, Cost)", dropping the names when they
// are unnamed placeholders and trimming the list when it is long.
func seriesFact(names []string) string {
	fact := pluralCount(len(names), "series", "series")
	if len(names) == 1 && names[0] == altPlaceholderSeries {
		return fact
	}
	listed := make([]string, 0, altMaxSeriesNames)
	for _, n := range names {
		if n == "" {
			continue
		}
		if len(listed) == altMaxSeriesNames {
			break
		}
		listed = append(listed, n)
	}
	if len(listed) == 0 {
		return fact
	}
	if len(listed) < len(names) {
		return fact + " (" + strings.Join(listed, ", ") + ", …)"
	}
	return fact + " (" + strings.Join(listed, ", ") + ")"
}

// chartDataShape reads a chart payload for its shape. The payload is
// deliberately loose — a flat label→value map for simple charts, categories +
// series for multi-series ones, a list of labelled points for funnels and
// waterfalls — and it reaches here either decoded from JSON or built in Go by
// ChartSpec.ToDiagramSpec, so every list is read through altList rather than
// asserted to []any. The readers are tried in order and the first that
// recognises the payload wins.
func chartDataShape(data map[string]any) (chartShape, bool) {
	if len(data) == 0 {
		return chartShape{}, false
	}
	for _, read := range []func(map[string]any) (chartShape, bool){
		categorySeriesShape, labelledPointShape, seriesPointShape, flatNumericShape,
	} {
		if shape, ok := read(data); ok {
			return shape, true
		}
	}
	return chartShape{}, false
}

// categorySeriesShape reads {categories: [...], series: [{name, values}]} and
// its single-series variant {categories: [...], values: [...]} (pie, donut).
func categorySeriesShape(data map[string]any) (chartShape, bool) {
	cats, ok := altList(data["categories"])
	if !ok {
		return chartShape{}, false
	}
	shape := chartShape{categories: len(cats)}
	if series, ok := altList(data["series"]); ok {
		for _, entry := range series {
			m, isMap := altMap(entry)
			if !isMap {
				shape.seriesNames = append(shape.seriesNames, "")
				continue
			}
			name, _ := m["name"].(string)
			shape.seriesNames = append(shape.seriesNames, strings.TrimSpace(name))
			if values, isList := altList(m["values"]); isList {
				shape.min, shape.max, shape.haveRange = extendRange(values, shape.min, shape.max, shape.haveRange)
			}
		}
	}
	if values, ok := altList(data["values"]); ok {
		shape.seriesNames = append(shape.seriesNames, "")
		shape.min, shape.max, shape.haveRange = extendRange(values, shape.min, shape.max, shape.haveRange)
	}
	return shape, true
}

// labelledPointShape reads {values: [{label, value}]} (funnel, treemap) and
// {points: [{label, value, type}]} (waterfall): the points are the categories
// and their values are the range.
func labelledPointShape(data map[string]any) (chartShape, bool) {
	for _, key := range []string{"values", "points"} {
		points, ok := altList(data[key])
		if !ok {
			continue
		}
		n, values := labelledPoints(points)
		if n == 0 {
			continue
		}
		shape := chartShape{categories: n}
		shape.min, shape.max, shape.haveRange = extendRange(values, 0, 0, false)
		return shape, true
	}
	return chartShape{}, false
}

// seriesPointShape reads {series: [{name, points: [{x, y}]}]} (scatter,
// bubble), where the point count stands in for the category count.
func seriesPointShape(data map[string]any) (chartShape, bool) {
	series, ok := altList(data["series"])
	if !ok {
		return chartShape{}, false
	}
	var shape chartShape
	for _, entry := range series {
		m, isMap := altMap(entry)
		if !isMap {
			continue
		}
		name, _ := m["name"].(string)
		shape.seriesNames = append(shape.seriesNames, strings.TrimSpace(name))
		points, isList := altList(m["points"])
		if !isList {
			continue
		}
		if len(points) > shape.categories {
			shape.categories = len(points)
		}
		for _, pt := range points {
			if pm, isMap := altMap(pt); isMap {
				shape.min, shape.max, shape.haveRange = extendRange([]any{pm["y"]}, shape.min, shape.max, shape.haveRange)
			}
		}
	}
	return shape, shape.categories > 0 || shape.haveRange
}

// flatNumericShape reads the flat form — {label: value} or
// {label: [v1, v2, …]}. Only numeric entries count, so a structural payload
// ({steps: [...]}) reports nothing and falls through to the item count.
func flatNumericShape(data map[string]any) (chartShape, bool) {
	var shape chartShape
	widest := 0
	for _, k := range sortedDataKeys(data) {
		if list, ok := altList(data[k]); ok {
			if !allNumeric(list) {
				continue
			}
			shape.categories++
			if len(list) > widest {
				widest = len(list)
			}
			shape.min, shape.max, shape.haveRange = extendRange(list, shape.min, shape.max, shape.haveRange)
			continue
		}
		if f, ok := altNumber(data[k]); ok {
			shape.categories++
			if widest < 1 {
				widest = 1
			}
			shape.min, shape.max, shape.haveRange = extendRange([]any{f}, shape.min, shape.max, shape.haveRange)
		}
	}
	shape.seriesNames = make([]string, widest)
	return shape, shape.categories > 0
}

// labelledPoints reads a list of {label, value} objects, returning how many
// there are and the values they carry. It reports zero for a list that is not
// in that shape, so the caller can try the next one.
func labelledPoints(points []any) (int, []any) {
	values := make([]any, 0, len(points))
	for _, p := range points {
		m, ok := altMap(p)
		if !ok {
			return 0, nil
		}
		if _, hasLabel := m["label"]; !hasLabel {
			return 0, nil
		}
		values = append(values, m["value"])
	}
	return len(points), values
}

// gaugeFact describes a dial payload — {value, min, max} — as "72 of 100",
// which is the only thing on the chart. The min/max siblings are required so a
// flat chart that happens to have a category named "value" is not mistaken for
// a dial.
func gaugeFact(data map[string]any) (string, bool) {
	value, ok := altNumber(data["value"])
	if !ok {
		return "", false
	}
	maximum, hasMax := altNumber(data["max"])
	if _, hasMin := altNumber(data["min"]); !hasMin && !hasMax {
		return "", false
	}
	if hasMax && maximum != 0 {
		return fmt.Sprintf("%s of %s", formatAltNumber(value), formatAltNumber(maximum)), true
	}
	return "value " + formatAltNumber(value), true
}

// altList normalises any slice the payload might hold — []any from JSON,
// []string / []float64 / []map[string]any from the Go-side chart conversion —
// into a []any the description code can walk.
func altList(v any) ([]any, bool) {
	switch list := v.(type) {
	case nil:
		return nil, false
	case []any:
		return list, len(list) > 0
	case []string:
		out := make([]any, len(list))
		for i, s := range list {
			out[i] = s
		}
		return out, len(out) > 0
	case []float64:
		out := make([]any, len(list))
		for i, f := range list {
			out[i] = f
		}
		return out, len(out) > 0
	case []int:
		out := make([]any, len(list))
		for i, n := range list {
			out[i] = n
		}
		return out, len(out) > 0
	case []map[string]any:
		out := make([]any, len(list))
		for i, m := range list {
			out[i] = m
		}
		return out, len(out) > 0
	}
	return nil, false
}

// altMap normalises a payload entry that should be an object.
func altMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// sortedDataKeys orders a payload's keys so a derived description is
// byte-identical across runs.
func sortedDataKeys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// allNumeric reports whether every entry in a list is a number — the test that
// separates a multi-series value array from a list of labels or step objects.
func allNumeric(values []any) bool {
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if _, ok := altNumber(v); !ok {
			return false
		}
	}
	return true
}

// diagramItemCount returns how many things a structural diagram draws and what
// to call them, for the payload keys that say so plainly.
func diagramItemCount(spec *types.DiagramSpec) (int, string) {
	for _, probe := range []struct {
		key  string
		noun string
	}{
		{"steps", "step"}, {"stages", "stage"}, {"phases", "phase"},
		{"levels", "level"}, {"tiers", "tier"}, {"nodes", "node"},
		{"forces", "force"}, {"tasks", "task"}, {"milestones", "milestone"},
		{"events", "event"}, {"items", "item"}, {"cards", "card"},
		{"rows", "row"}, {"points", "point"},
	} {
		if list, ok := spec.Data[probe.key].([]any); ok && len(list) > 0 {
			return len(list), probe.noun
		}
	}
	// A framework payload names its own sections rather than holding one list
	// (a SWOT carries strengths / weaknesses / opportunities / threats), so
	// count the populated ones. Without this a SWOT announced only "Swot
	// diagram." — the type name the alt text exists to replace.
	if n := populatedSections(spec.Data); n > 0 {
		return n, "section"
	}
	return 0, ""
}

// populatedSections counts the top-level payload keys holding a non-empty list
// — the shape a framework diagram uses for its named sections.
func populatedSections(data map[string]any) int {
	n := 0
	for _, k := range sortedDataKeys(data) {
		if list, ok := data[k].([]any); ok && len(list) > 0 {
			n++
		}
	}
	return n
}

// extendRange folds a value list into a running min/max.
func extendRange(values []any, min, max float64, have bool) (float64, float64, bool) {
	for _, v := range values {
		f, ok := altNumber(v)
		if !ok {
			continue
		}
		switch {
		case !have:
			min, max, have = f, f, true
		case f < min:
			min = f
		case f > max:
			max = f
		}
	}
	return min, max, have
}

// altNumber reads a JSON number in any of the shapes a decoded payload uses.
// A numeric string counts: flat chart payloads routinely carry "1200" rather
// than 1200.
func altNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// altAcronyms are the diagram-type words that are acronyms, not words. Without
// them "swot" reads back as "Swot" — a word no listener recognises.
var altAcronyms = map[string]string{
	"swot": "SWOT", "pestel": "PESTEL", "pest": "PEST", "bmc": "BMC",
	"kpi": "KPI", "rag": "RAG", "scqa": "SCQA", "okr": "OKR", "raci": "RACI",
}

// humanTypeName turns "stacked_bar_chart" into "Stacked bar chart" and
// "process_flow" into "Process flow diagram" — a noun a listener can picture.
func humanTypeName(t string) string {
	name := strings.TrimSpace(strings.ReplaceAll(t, "_", " "))
	if name == "" {
		return "Diagram"
	}
	words := strings.Split(name, " ")
	for i, w := range words {
		if full, ok := altAcronyms[strings.ToLower(w)]; ok {
			words[i] = full
		}
	}
	name = strings.Join(words, " ")
	for _, suffix := range []string{"chart", "diagram", "flow", "map", "matrix", "timeline", "canvas"} {
		if strings.HasSuffix(name, suffix) {
			return capitalizeFirst(name)
		}
	}
	return capitalizeFirst(name + " diagram")
}

// capitalizeFirst upper-cases the first rune, leaving the rest alone so
// "PESTEL analysis" is not mangled.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// pluralCount renders "1 category" / "6 categories".
func pluralCount(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// formatAltNumber prints a value the way a reader would say it.
func formatAltNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// truncateAlt keeps a description to one announceable sentence's worth.
func truncateAlt(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= altMaxLen {
		return s
	}
	return string([]rune(s)[:altMaxLen-1]) + "…"
}

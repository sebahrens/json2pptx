package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Regions slides (go-slide-creator-fn2ka).
//
// One slide, one title, two or three typed regions side by side or stacked —
// the chart on the left with the margin KPI and the launch timeline on the
// right. Before this kind a DeckSpec slide carried exactly one kind, so the
// mixed slide every executive review has needed raw_json2pptx and left typed
// validation and semantic repair behind.
//
// The arrangement is bounded on purpose: four named arrangements over two or
// three regions with explicit proportions, not a layout optimizer. Every
// region compiles onto a primitive the raw engine already renders inside a
// shape_grid cell — an svggen diagram, a nested stat-hero / kpi-Nup /
// timeline-horizontal pattern, a native table, a picture or a text box — so
// there is no second layout engine, only a grid the compiler writes.

// Region kinds a regions slide can hold. Each one's fields are the ones the
// like-named slide kind uses (stat ↔ stat, table ↔ table, timeline ↔ timeline,
// kpis ↔ kpi_snapshot.kpis, chart ↔ chart_insight.chart, image ↔
// image_case.image).
const (
	RegionChart    = "chart"
	RegionStat     = "stat"
	RegionKPIs     = "kpis"
	RegionTable    = "table"
	RegionTimeline = "timeline"
	RegionImage    = "image"
	RegionText     = "text"
)

// RegionKinds lists the region kinds in a stable order.
var RegionKinds = []string{RegionChart, RegionStat, RegionKPIs, RegionTable, RegionTimeline, RegionImage, RegionText}

// Region arrangements.
const (
	ArrangeColumns    = "columns"     // 2–3 regions left to right; size_pct is each one's width
	ArrangeRows       = "rows"        // 2–3 regions top to bottom; size_pct is each one's height
	ArrangeMainLeft   = "main_left"   // regions[0] fills the left column; regions[1..2] stack on the right
	ArrangeMainRight  = "main_right"  // regions[0] fills the right column; regions[1..2] stack on the left
	ArrangeMainTop    = "main_top"    // regions[0] fills the top band; regions[1..2] sit side by side beneath it
	ArrangeMainBottom = "main_bottom" // regions[0] fills the bottom band; regions[1..2] sit side by side above it
)

// RegionArrangements lists the arrangements in a stable order.
var RegionArrangements = []string{ArrangeColumns, ArrangeRows, ArrangeMainLeft, ArrangeMainRight, ArrangeMainTop, ArrangeMainBottom}

// Region bounds shared by validation and compile.
const (
	// RegionMinSizePct / RegionMaxSizePct bound one region's share: a sliver
	// below 15% cannot hold a chart or a number, and past 85% the others
	// cannot.
	RegionMinSizePct = 15
	RegionMaxSizePct = 85
	// RegionDefaultMainPct is the main region's share when it sets none.
	RegionDefaultMainPct = 60
	// RegionHeadingMax caps a region heading: one short line above the region.
	RegionHeadingMax = 60
	// RegionKPIMin / RegionKPIMax bound a kpis region: one number is a stat,
	// and past four cards a third of the slide holds no readable number.
	RegionKPIMin = 2
	RegionKPIMax = 4
	// RegionTextBodyMax / RegionTextBulletsMax bound a text region.
	RegionTextBodyMax    = 400
	RegionTextBulletsMax = 6
	// RegionTableMaxColumns / RegionTableMaxRows bound a table region: half a
	// slide holds fewer columns and rows than the whole one.
	RegionTableMaxColumns = 4
	RegionTableMaxRows    = 6
	// regionHeadingRowPt / regionCaptionRowPt are the fixed heights of a
	// region's heading and caption rows (one bold line; up to two italic
	// ones). An auto-height row's estimate was generous, and in a stacked
	// region it took the height the visual beneath it needed.
	regionHeadingRowPt = 20
	regionCaptionRowPt = 34
)

// RegionArrangementOf returns the payload's arrangement, defaulting to columns.
func RegionArrangementOf(body map[string]any) string {
	if a := strField(body, "arrangement"); a != "" {
		return a
	}
	return ArrangeColumns
}

// IsMainArrangement reports whether the arrangement has one main region and a
// two-region stack.
func IsMainArrangement(arrangement string) bool {
	switch arrangement {
	case ArrangeMainLeft, ArrangeMainRight, ArrangeMainTop, ArrangeMainBottom:
		return true
	}
	return false
}

// RegionCountBounds returns the region count an arrangement takes.
func RegionCountBounds(arrangement string) (lo, hi int) {
	if IsMainArrangement(arrangement) {
		return 3, 3
	}
	return 2, 3
}

// RegionList returns the payload's regions in order. A non-object entry is
// returned as nil so indices stay aligned with the authored list.
func RegionList(body map[string]any) []map[string]any {
	raw, ok := body["regions"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, len(raw))
	for i, e := range raw {
		out[i], _ = e.(map[string]any)
	}
	return out
}

// RegionSizePct returns a region's authored size_pct, or 0 when unset.
func RegionSizePct(region map[string]any) float64 {
	switch v := region["size_pct"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

// regionWeight is a region kind's claim on space a share was not authored
// for: a stat needs less than the timeline or table stacked with it, and a
// chart more than either (go-slide-creator-vae7f).
var regionWeight = map[string]float64{RegionStat: 2, RegionChart: 4}

func regionWeightOf(r map[string]any) float64 {
	if w, ok := regionWeight[strField(r, "kind")]; ok {
		return w
	}
	return 3
}

// ResolveShares fills unset shares (0) with what the set ones leave, split in
// proportion to weights (nil: equally), so the group sums to 100. It reports
// an error when an unset share would fall under RegionMinSizePct or, with
// every share set, the shares do not sum to 100 (±1 for rounding).
func ResolveShares(shares, weights []float64) ([]float64, error) {
	out := append([]float64(nil), shares...)
	sum, unsetWeight := 0.0, 0.0
	weight := func(i int) float64 {
		if i < len(weights) && weights[i] > 0 {
			return weights[i]
		}
		return 1
	}
	for i, s := range shares {
		if s > 0 {
			sum += s
		} else {
			unsetWeight += weight(i)
		}
	}
	if unsetWeight == 0 {
		if math.Abs(sum-100) > 1 {
			return nil, fmt.Errorf("the size_pct values sum to %g; they must sum to 100", sum)
		}
		return out, nil
	}
	rest := 100 - sum
	for i := range out {
		if out[i] > 0 {
			continue
		}
		out[i] = math.Round(rest*weight(i)/unsetWeight*100) / 100
		if out[i] < RegionMinSizePct {
			return nil, fmt.Errorf("the set size_pct values sum to %g, leaving %g%% for the region(s) without one; each region needs at least %d%%", sum, math.Max(rest, 0), RegionMinSizePct)
		}
	}
	return out, nil
}

// regionMinHeightPct is the least share of a vertical group — the rows of a
// rows arrangement, a main_left / main_right stack, the main row and the
// band beneath or above it in main_top / main_bottom — a region reads in on
// the shortest shipped content area (modern: about 215pt under a title and
// takeaway). The values are measured with validate_deck_spec on every
// shipped template (go-slide-creator-umev3): a stat needs 30% (35% with a
// context line), a three-stop timeline 55%, a table 15% per line including
// the header, and a heading row another 10%. A stat weighted under a
// timeline or table used to fall below these on modern, and drafted regions
// slides were refused.
func regionMinHeightPct(r map[string]any) float64 {
	var pct float64
	switch strField(r, "kind") {
	case RegionStat:
		pct = 30
		if strField(r, "context") != "" {
			pct = 35
		}
	case RegionTimeline:
		pct = 55
	case RegionTable:
		rows, _ := r["rows"].([]any)
		pct = 15 * float64(len(rows)+1)
	case RegionKPIs, RegionChart:
		pct = 30
	case RegionImage:
		pct = 25
	default:
		pct = RegionMinSizePct
	}
	if RegionHeading(r) != "" {
		pct += 10
	}
	return math.Min(pct, RegionMaxSizePct)
}

// raiseToMinimums lifts every unset share below its minimum to it, taking
// the difference from the other unset shares in proportion to what they
// hold above their own minimums. Authored shares are kept. When the others
// cannot give enough, each short share gets the same fraction of its
// deficit, and validation reports the region that still does not read.
func raiseToMinimums(shares []float64, unset []bool, mins []float64) []float64 {
	out := append([]float64(nil), shares...)
	need, avail := 0.0, 0.0
	for i := range out {
		if !unset[i] {
			continue
		}
		if out[i] < mins[i] {
			need += mins[i] - out[i]
		} else {
			avail += out[i] - mins[i]
		}
	}
	if need == 0 || avail == 0 {
		return out
	}
	take := math.Min(need, avail)
	for i := range out {
		if !unset[i] {
			continue
		}
		if out[i] < mins[i] {
			out[i] += (mins[i] - out[i]) * take / need
		} else {
			out[i] -= (out[i] - mins[i]) * take / avail
		}
		out[i] = math.Round(out[i]*100) / 100
	}
	return out
}

// RegionGroupShares resolves the shares of the two size groups of a regions
// payload: the main axis (columns / rows: every region; main_*: the main
// region and the stack) and, for main_* arrangements, the stack's own split.
// Unset shares are split by region kind (see regionWeight), then any unset
// share of a vertical group is raised to its kind's readable minimum (see
// regionMinHeightPct).
func RegionGroupShares(body map[string]any) (axis, stack []float64, err error) {
	regions := RegionList(body)
	arrangement := RegionArrangementOf(body)
	if !IsMainArrangement(arrangement) {
		shares := make([]float64, len(regions))
		weights := make([]float64, len(regions))
		unset := make([]bool, len(regions))
		mins := make([]float64, len(regions))
		for i, r := range regions {
			shares[i] = RegionSizePct(r)
			weights[i] = regionWeightOf(r)
			unset[i] = shares[i] <= 0
			mins[i] = regionMinHeightPct(r)
		}
		axis, err = ResolveShares(shares, weights)
		if err == nil && arrangement == ArrangeRows {
			axis = raiseToMinimums(axis, unset, mins)
		}
		return axis, nil, err
	}
	if len(regions) != 3 {
		return nil, nil, fmt.Errorf("%s takes exactly 3 regions", arrangement)
	}
	main := RegionSizePct(regions[0])
	mainUnset := main <= 0
	if mainUnset {
		main = RegionDefaultMainPct
	}
	axis = []float64{main, 100 - main}
	stackShares := []float64{RegionSizePct(regions[1]), RegionSizePct(regions[2])}
	stack, err = ResolveShares(stackShares,
		[]float64{regionWeightOf(regions[1]), regionWeightOf(regions[2])})
	if err != nil {
		return axis, stack, err
	}
	switch arrangement {
	case ArrangeMainLeft, ArrangeMainRight:
		// The stack splits the column's height.
		stack = raiseToMinimums(stack, []bool{stackShares[0] <= 0, stackShares[1] <= 0},
			[]float64{regionMinHeightPct(regions[1]), regionMinHeightPct(regions[2])})
	default:
		// The main region and the band split the height; the band is as
		// tall as its taller-needing region.
		band := math.Max(regionMinHeightPct(regions[1]), regionMinHeightPct(regions[2]))
		axis = raiseToMinimums(axis, []bool{mainUnset, mainUnset}, []float64{regionMinHeightPct(regions[0]), band})
	}
	return axis, stack, err
}

// regionBuild is one region compiled to a grid cell, with the source links its
// content emitted relative to the cell (RawPath is a suffix such as
// ".pattern.values.value", "" for the cell itself).
type regionBuild struct {
	cell  *deckinput.GridCellInput
	links []SourceLink
}

// CompileRegions compiles a regions payload onto one blank-title slide whose
// shape_grid holds every region at its allocated share. It returns an error
// only for a payload validation already refuses (an unknown arrangement, a
// region count or share outside the bounds, an unknown region kind): a regions
// slide has no single-kind fallback to degrade to, so it never compiles a
// half-formed grid.
func CompileRegions(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	arrangement := RegionArrangementOf(in.Body)
	regions := RegionList(in.Body)
	lo, hi := RegionCountBounds(arrangement)
	if !containsStr(RegionArrangements, arrangement) {
		return nil, nil, fmt.Errorf("unknown arrangement %q", arrangement)
	}
	if len(regions) < lo || len(regions) > hi {
		return nil, nil, fmt.Errorf("%s takes %d–%d regions, got %d", arrangement, lo, hi, len(regions))
	}
	axis, stack, err := RegionGroupShares(in.Body)
	if err != nil {
		return nil, nil, err
	}

	builds := make([]regionBuild, len(regions))
	for i, r := range regions {
		b, berr := compileRegion(in, i, r)
		if berr != nil {
			return nil, nil, fmt.Errorf("regions[%d]: %w", i, berr)
		}
		builds[i] = b
	}

	grid, cellPaths := regionGrid(arrangement, axis, stack, builds)
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title", ShapeGrid: grid}
	links := titleLink(slide, in)
	for i, b := range builds {
		cellRaw := in.rawSlide() + ".shape_grid" + cellPaths[i]
		regionSem := fmt.Sprintf("%s.regions[%d]", in.semSlide(), i)
		links = append(links, SourceLink{RawPath: cellRaw, SemanticPath: regionSem})
		for _, l := range b.links {
			links = append(links, SourceLink{RawPath: cellRaw + l.RawPath, SemanticPath: regionSem + l.SemanticPath})
		}
	}

	if src, field := regionsSourceLine(in.Body); src != "" {
		slide.Source = src
		links = append(links, SourceLink{RawPath: in.rawSlide() + ".source", SemanticPath: in.semSlide() + field})
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// regionsSourceLine joins the slide's own source and each region's, dropping
// repeats, into the one attribution line the slide renders. The field is the
// semantic path suffix of the first source.
func regionsSourceLine(body map[string]any) (string, string) {
	var parts []string
	field := ""
	add := func(s, f string) {
		if s == "" {
			return
		}
		for _, p := range parts {
			if strings.EqualFold(p, s) {
				return
			}
		}
		if field == "" {
			field = f
		}
		parts = append(parts, s)
	}
	add(strField(body, "source"), ".source")
	for i, r := range RegionList(body) {
		add(strField(r, "source"), fmt.Sprintf(".regions[%d].source", i))
	}
	return strings.Join(parts, "; "), field
}

// regionGrid lays the compiled regions out for the arrangement. It returns the
// grid and each region's cell path relative to the grid
// (".rows[0].cells[1].grid.rows[0].cells[0]").
func regionGrid(arrangement string, axis, stack []float64, builds []regionBuild) (*deckinput.ShapeGridInput, []string) {
	paths := make([]string, len(builds))
	cellAt := func(r, c int) string { return fmt.Sprintf(".rows[%d].cells[%d]", r, c) }
	switch arrangement {
	case ArrangeRows:
		g := &deckinput.ShapeGridInput{Columns: json.RawMessage("1")}
		for i, b := range builds {
			g.Rows = append(g.Rows, deckinput.GridRowInput{Height: axis[i], Cells: []*deckinput.GridCellInput{b.cell}})
			paths[i] = cellAt(i, 0)
		}
		return g, paths
	case ArrangeMainLeft, ArrangeMainRight:
		side := &deckinput.ShapeGridInput{Columns: json.RawMessage("1"), Rows: []deckinput.GridRowInput{
			{Height: stack[0], Cells: []*deckinput.GridCellInput{builds[1].cell}},
			{Height: stack[1], Cells: []*deckinput.GridCellInput{builds[2].cell}},
		}}
		mainCol, sideCol := 0, 1
		cols := axis
		if arrangement == ArrangeMainRight {
			mainCol, sideCol = 1, 0
			cols = []float64{axis[1], axis[0]}
		}
		cells := make([]*deckinput.GridCellInput, 2)
		cells[mainCol] = builds[0].cell
		cells[sideCol] = &deckinput.GridCellInput{Grid: side}
		paths[0] = cellAt(0, mainCol)
		paths[1] = cellAt(0, sideCol) + ".grid" + cellAt(0, 0)
		paths[2] = cellAt(0, sideCol) + ".grid" + cellAt(1, 0)
		return &deckinput.ShapeGridInput{Columns: shareColumns(cols), Rows: []deckinput.GridRowInput{{Cells: cells}}}, paths
	case ArrangeMainTop, ArrangeMainBottom:
		side := &deckinput.ShapeGridInput{Columns: shareColumns(stack), Rows: []deckinput.GridRowInput{
			{Cells: []*deckinput.GridCellInput{builds[1].cell, builds[2].cell}},
		}}
		mainRow, sideRow := 0, 1
		heights := axis
		if arrangement == ArrangeMainBottom {
			mainRow, sideRow = 1, 0
			heights = []float64{axis[1], axis[0]}
		}
		rows := make([]deckinput.GridRowInput, 2)
		rows[mainRow] = deckinput.GridRowInput{Height: heights[mainRow], Cells: []*deckinput.GridCellInput{builds[0].cell}}
		rows[sideRow] = deckinput.GridRowInput{Height: heights[sideRow], Cells: []*deckinput.GridCellInput{{Grid: side}}}
		paths[0] = cellAt(mainRow, 0)
		paths[1] = cellAt(sideRow, 0) + ".grid" + cellAt(0, 0)
		paths[2] = cellAt(sideRow, 0) + ".grid" + cellAt(0, 1)
		return &deckinput.ShapeGridInput{Columns: json.RawMessage("1"), Rows: rows}, paths
	default: // columns
		cells := make([]*deckinput.GridCellInput, len(builds))
		for i, b := range builds {
			cells[i] = b.cell
			paths[i] = cellAt(0, i)
		}
		return &deckinput.ShapeGridInput{Columns: shareColumns(axis), Rows: []deckinput.GridRowInput{{Cells: cells}}}, paths
	}
}

// shareColumns encodes percentage shares as a grid columns array.
func shareColumns(shares []float64) json.RawMessage {
	rounded := make([]float64, len(shares))
	for i, s := range shares {
		rounded[i] = math.Round(s*100) / 100
	}
	b, _ := json.Marshal(rounded)
	return b
}

// compileRegion builds one region's cell: its content, under its heading when
// it has one (and an image's caption beneath it).
func compileRegion(in Input, idx int, r map[string]any) (regionBuild, error) {
	if r == nil {
		return regionBuild{}, fmt.Errorf("a region must be an object")
	}
	kind := strField(r, "kind")
	var content regionBuild
	var err error
	switch kind {
	case RegionChart:
		content, err = regionChart(in, r)
	case RegionStat:
		content, err = regionStat(r)
	case RegionKPIs:
		content, err = regionKPIs(r)
	case RegionTable:
		content, err = regionTable(in, r)
	case RegionTimeline:
		content, err = regionTimeline(r)
	case RegionImage:
		content, err = regionImage(r)
	case RegionText:
		content, err = regionText(r)
	default:
		return regionBuild{}, fmt.Errorf("unknown region kind %q", kind)
	}
	if err != nil {
		return regionBuild{}, err
	}

	heading := RegionHeading(r)
	caption := ""
	if kind == RegionImage {
		caption = strField(r, "caption")
	}
	if heading == "" && caption == "" {
		return content, nil
	}
	// A heading or caption is its own content-sized row, so the region's
	// visual keeps the rest of the cell instead of sharing a text box with it.
	g := &deckinput.ShapeGridInput{Columns: json.RawMessage("1"), RowGap: 4}
	var links []SourceLink
	if heading != "" {
		g.Rows = append(g.Rows, deckinput.GridRowInput{MinHeight: regionHeadingRowPt, MaxHeight: regionHeadingRowPt, Cells: []*deckinput.GridCellInput{
			labelCell(regionParagraph{Content: heading, Bold: true}, "b"),
		}})
		links = append(links, SourceLink{RawPath: fmt.Sprintf(".grid.rows[%d].cells[0]", len(g.Rows)-1), SemanticPath: RegionHeadingField(r)})
	}
	contentRow := len(g.Rows)
	g.Rows = append(g.Rows, deckinput.GridRowInput{Cells: []*deckinput.GridCellInput{content.cell}})
	prefix := fmt.Sprintf(".grid.rows[%d].cells[0]", contentRow)
	links = append(links, SourceLink{RawPath: prefix, SemanticPath: ""})
	for _, l := range content.links {
		links = append(links, SourceLink{RawPath: prefix + l.RawPath, SemanticPath: l.SemanticPath})
	}
	if caption != "" {
		g.Rows = append(g.Rows, deckinput.GridRowInput{MinHeight: regionCaptionRowPt, MaxHeight: regionCaptionRowPt, Cells: []*deckinput.GridCellInput{
			labelCell(regionParagraph{Content: caption, Italic: true}, "t"),
		}})
		links = append(links, SourceLink{RawPath: fmt.Sprintf(".grid.rows[%d].cells[0]", len(g.Rows)-1), SemanticPath: ".caption"})
	}
	return regionBuild{cell: &deckinput.GridCellInput{Grid: g}, links: links}, nil
}

// RegionHeading returns a region's heading line: its heading, with a chart's
// unit appended ("Quarterly revenue (€m)"), or the unit alone.
func RegionHeading(r map[string]any) string {
	heading := strField(r, "heading")
	if strField(r, "kind") == RegionChart {
		if unit := strField(r, "unit"); unit != "" {
			if heading == "" {
				return "Values in " + unit
			}
			return heading + " (" + unit + ")"
		}
	}
	return heading
}

// RegionHeadingField names the semantic field a region's heading row came from.
func RegionHeadingField(r map[string]any) string {
	if strField(r, "heading") == "" {
		return ".unit"
	}
	return ".heading"
}

// regionParagraph is one paragraph of a compiled text cell. No size is set:
// the deck's type scale sizes compiled text, and a constrained deck refuses
// hand-set sizes.
type regionParagraph struct {
	Content string `json:"content"`
	Bold    bool   `json:"bold,omitempty"`
	Italic  bool   `json:"italic,omitempty"`
	Bullet  bool   `json:"bullet,omitempty"`
}

// textCell builds an unfilled, left-aligned text box cell.
func textCell(paras []regionParagraph, vAlign string) *deckinput.GridCellInput {
	return textCellInset(paras, vAlign, nil)
}

// labelCell is a one-line heading / caption cell with no vertical inset, so
// its fixed row is spent on the text rather than on padding.
func labelCell(p regionParagraph, vAlign string) *deckinput.GridCellInput {
	zero := 0.0
	return textCellInset([]regionParagraph{p}, vAlign, &zero)
}

func textCellInset(paras []regionParagraph, vAlign string, vInset *float64) *deckinput.GridCellInput {
	text, _ := json.Marshal(struct {
		Paragraphs    []regionParagraph `json:"paragraphs"`
		Align         string            `json:"align"`
		VerticalAlign string            `json:"vertical_align"`
		InsetTop      *float64          `json:"inset_top,omitempty"`
		InsetBottom   *float64          `json:"inset_bottom,omitempty"`
	}{paras, "l", vAlign, vInset, vInset})
	return &deckinput.GridCellInput{Shape: &deckinput.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     text,
	}}
}

func regionChart(in Input, r map[string]any) (regionBuild, error) {
	chart := chartSpec(r)
	if chart == nil {
		return regionBuild{}, fmt.Errorf("chart region needs chart {type, data}")
	}
	chart.Alt = firstNonEmpty(strField(r, "alt"), RegionHeading(r), chart.Title, visualAltText(in))
	return regionBuild{
		cell:  &deckinput.GridCellInput{Diagram: chart},
		links: []SourceLink{{RawPath: ".diagram", SemanticPath: ".chart"}},
	}, nil
}

func regionStat(r map[string]any) (regionBuild, error) {
	v := statValues(r)
	if v.Value == "" {
		return regionBuild{}, fmt.Errorf("stat region needs a value")
	}
	// The region's source joins the slide's attribution line; stat-hero would
	// print it a second time inside the region.
	v.Source = ""
	encoded, err := json.Marshal(v)
	if err != nil {
		return regionBuild{}, fmt.Errorf("marshal stat-hero values: %w", err)
	}
	// No size overrides: stat-hero sizes its figure, label and context to
	// the region's cell (go-slide-creator-hidji). The fixed 36/16/14pt set
	// it used to carry was shrunk below the 12pt floor in a short region.
	return regionBuild{
		cell: patternCell(deckinput.PatternInput{Name: "stat-hero", Values: encoded}),
		links: []SourceLink{
			{RawPath: ".pattern.values.value", SemanticPath: "." + statValueField(r)},
			{RawPath: ".pattern.values.label", SemanticPath: ".label"},
			{RawPath: ".pattern.values.unit", SemanticPath: ".unit"},
			{RawPath: ".pattern.values.context", SemanticPath: ".context"},
		},
	}, nil
}

func regionKPIs(r map[string]any) (regionBuild, error) {
	cells, field := kpiCells(r)
	if len(cells) < RegionKPIMin || len(cells) > RegionKPIMax {
		return regionBuild{}, fmt.Errorf("kpis region takes %d–%d metrics, got %d", RegionKPIMin, RegionKPIMax, len(cells))
	}
	encoded, err := json.Marshal(cells)
	if err != nil {
		return regionBuild{}, fmt.Errorf("marshal kpi values: %w", err)
	}
	b := regionBuild{cell: patternCell(deckinput.PatternInput{Name: fmt.Sprintf("kpi-%dup", len(cells)), Values: encoded})}
	for k := range cells {
		b.links = append(b.links, SourceLink{RawPath: fmt.Sprintf(".pattern.values[%d]", k), SemanticPath: fmt.Sprintf(".%s[%d]", field, k)})
	}
	return b, nil
}

func regionTable(in Input, r map[string]any) (regionBuild, error) {
	headers := tableHeaders(r)
	rows := tableRows(r, len(headers))
	if len(headers) == 0 || len(rows) == 0 {
		return regionBuild{}, fmt.Errorf("table region needs headers and at least one row")
	}
	table := &jsonschema.TableInput{Headers: headers, Rows: rows, Alt: firstNonEmpty(RegionHeading(r), visualAltText(in))}
	if alignments := tableColumnAlignments(r, len(headers)); len(alignments) > 0 {
		table.ColumnAlignments = alignments
	}
	if style := tableStyle(r, headers); style != nil {
		table.Style = style
	}
	return regionBuild{
		cell: &deckinput.GridCellInput{Table: table},
		links: []SourceLink{
			{RawPath: ".table.headers", SemanticPath: "." + tableHeadersField(r)},
			{RawPath: ".table.rows", SemanticPath: ".rows"},
		},
	}, nil
}

func regionTimeline(r map[string]any) (regionBuild, error) {
	stops := TimelineStops(r)
	if !timelineFits(stops) {
		return regionBuild{}, fmt.Errorf("timeline region %s", firstNonEmpty(TimelineOverBudget(r), "needs 3–7 milestones"))
	}
	encoded, err := json.Marshal(stops)
	if err != nil {
		return regionBuild{}, fmt.Errorf("marshal timeline-horizontal values: %w", err)
	}
	p := deckinput.PatternInput{Name: "timeline-horizontal", Values: encoded}
	if timelineHasRanges(stops) {
		p.Overrides, err = json.Marshal(timelineOverrides{Style: "gantt"})
		if err != nil {
			return regionBuild{}, fmt.Errorf("marshal timeline-horizontal overrides: %w", err)
		}
	}
	return regionBuild{
		cell:  patternCell(p),
		links: []SourceLink{{RawPath: ".pattern.values", SemanticPath: "." + timelineStopsField(r)}},
	}, nil
}

func regionImage(r map[string]any) (regionBuild, error) {
	img := imageCaseImageFrom(r)
	if img == nil {
		return regionBuild{}, fmt.Errorf("image region needs image {path|url}")
	}
	return regionBuild{
		cell: &deckinput.GridCellInput{Image: &deckinput.GridImageInput{
			Path: img.Path, URL: img.URL, Alt: firstNonEmpty(img.Alt, strField(r, "caption"), strField(r, "heading")), Fit: img.Fit,
		}},
		links: []SourceLink{{RawPath: ".image", SemanticPath: ".image"}},
	}, nil
}

func regionText(r map[string]any) (regionBuild, error) {
	var paras []regionParagraph
	var links []SourceLink
	if body := strField(r, "body"); body != "" {
		for _, p := range strings.Split(body, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				paras = append(paras, regionParagraph{Content: p})
			}
		}
		links = append(links, SourceLink{RawPath: ".shape.text", SemanticPath: ".body"})
	}
	bullets, _ := stringList(r, "bullets")
	for _, b := range bullets {
		paras = append(paras, regionParagraph{Content: b, Bullet: true})
	}
	if len(bullets) > 0 && len(links) == 0 {
		links = append(links, SourceLink{RawPath: ".shape.text", SemanticPath: ".bullets"})
	}
	if len(paras) == 0 {
		return regionBuild{}, fmt.Errorf("text region needs a body or bullets")
	}
	return regionBuild{cell: textCell(paras, "t"), links: links}, nil
}

// patternCell hosts a named pattern in a grid cell; the engine expands it
// inside the cell's own rectangle.
func patternCell(p deckinput.PatternInput) *deckinput.GridCellInput {
	raw, _ := json.Marshal(p)
	return &deckinput.GridCellInput{Pattern: raw}
}

// RegionPatterns returns the named patterns a regions payload compiles to, in
// region order ("" for a region without one). The explain planner and the
// raw preflight read it.
func RegionPatterns(body map[string]any) []string {
	regions := RegionList(body)
	out := make([]string, len(regions))
	for i, r := range regions {
		switch strField(r, "kind") {
		case RegionStat:
			out[i] = "stat-hero"
		case RegionTimeline:
			out[i] = "timeline-horizontal"
		case RegionKPIs:
			if cells, _ := kpiCells(r); len(cells) > 0 {
				out[i] = fmt.Sprintf("kpi-%dup", len(cells))
			}
		}
	}
	return out
}

// RegionKPIBudgetReason explains why a kpis region's metrics break the
// kpi-Nup budgets, or "" when they fit.
func RegionKPIBudgetReason(r map[string]any) string {
	cells, _ := kpiCells(r)
	if len(cells) < RegionKPIMin || len(cells) > RegionKPIMax {
		return ""
	}
	values, err := json.Marshal(cells)
	if err != nil {
		return "the metrics cannot be encoded as pattern values"
	}
	name := fmt.Sprintf("kpi-%dup", len(cells))
	if err := deckinput.ValidatePattern(&deckinput.PatternInput{Name: name, Values: values}, patterns.Default()); err != nil {
		return kpiBudgetReason(name, err)
	}
	return ""
}

// RegionKPICount returns the usable metric count of a kpis region.
func RegionKPICount(r map[string]any) int {
	cells, _ := kpiCells(r)
	return len(cells)
}

// RegionChartSpec returns a chart region's renderable chart, or nil.
func RegionChartSpec(r map[string]any) *types.DiagramSpec { return chartSpec(r) }

// RegionHasImage reports whether an image region names a picture.
func RegionHasImage(r map[string]any) bool { return imageCaseImageFrom(r) != nil }

// RegionTextCounts returns a text region's body length and bullet count.
func RegionTextCounts(r map[string]any) (bodyRunes, bullets int) {
	list, _ := stringList(r, "bullets")
	return runeLen(strField(r, "body")), len(list)
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

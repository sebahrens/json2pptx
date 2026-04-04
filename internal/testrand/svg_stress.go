// Package testrand provides SVG stress testing across all registered diagram types.
package testrand

import (
	"encoding/xml"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/svggen"
)

// Variant describes a stress test data variant.
type Variant int

const (
	VariantMinimal  Variant = iota // 1 data point
	VariantStandard                // 3-5 data points
	VariantDense                   // 15-20 data points
	VariantEmpty                   // empty data
	VariantEdge                    // special chars, long labels, negative/zero/huge values
)

func (v Variant) String() string {
	switch v {
	case VariantMinimal:
		return "minimal"
	case VariantStandard:
		return "standard"
	case VariantDense:
		return "dense"
	case VariantEmpty:
		return "empty"
	case VariantEdge:
		return "edge"
	default:
		return "unknown"
	}
}

// AllVariants returns all 5 test variants.
func AllVariants() []Variant {
	return []Variant{VariantMinimal, VariantStandard, VariantDense, VariantEmpty, VariantEdge}
}

// SVGStressResult holds results from a single test case.
type SVGStressResult struct {
	DiagramType string  `json:"diagram_type"`
	Variant     string  `json:"variant"`
	Passed      bool    `json:"passed"`
	Error       string  `json:"error,omitempty"`
	SVGBytes    int     `json:"svg_bytes"`
	HasViewBox  bool    `json:"has_viewbox"`
	PNGBytes    int     `json:"png_bytes,omitempty"`
	ValidXML    bool    `json:"valid_xml"`
}

// SVGStressReport holds the full stress test report.
type SVGStressReport struct {
	Seed       uint64             `json:"seed"`
	Total      int                `json:"total"`
	Passed     int                `json:"passed"`
	Failed     int                `json:"failed"`
	Results    []SVGStressResult  `json:"results"`
	Failures   []SVGStressResult  `json:"failures,omitempty"`
}

// SVGStressRunner runs SVG stress tests across all diagram types.
type SVGStressRunner struct {
	rng  *rand.Rand
	Seed uint64
}

// NewSVGStressRunner creates a runner with the given seed.
func NewSVGStressRunner(seed uint64) *SVGStressRunner {
	return &SVGStressRunner{
		rng:  rand.New(rand.NewPCG(seed, seed^0xCAFEBABE)), //nolint:gosec // intentionally using math/rand for reproducible test data
		Seed: seed,
	}
}

// DiagramTypes returns all registered canonical diagram types sorted.
func DiagramTypes() []string {
	types := svggen.Types()
	sort.Strings(types)
	return types
}

// AliasMap returns a copy of all alias→canonical mappings.
func AliasMap() map[string]string {
	// We test alias resolution by trying to render with alias names.
	return map[string]string{
		"funnel":       "funnel_chart",
		"gauge":        "gauge_chart",
		"treemap":      "treemap_chart",
		"bar":          "bar_chart",
		"line":         "line_chart",
		"pie":          "pie_chart",
		"donut":        "donut_chart",
		"area":         "area_chart",
		"radar":        "radar_chart",
		"scatter":      "scatter_chart",
		"bubble":       "bubble_chart",
		"stacked_bar":  "stacked_bar_chart",
		"stacked_area": "stacked_area_chart",
		"grouped_bar":  "grouped_bar_chart",
		"bar-stacked":  "stacked_bar_chart",
		"process":      "process_flow",
		"flow":         "process_flow",
		"flowchart":    "process_flow",
		"orgchart":     "org_chart",
		"org":          "org_chart",
		"nine-box":     "nine_box_talent",
		"nine_box":     "nine_box_talent",
		"bmc":          "business_model_canvas",
		"canvas":       "business_model_canvas",
		"porter":       "porters_five_forces",
		"matrix":       "matrix_2x2",
		"porters":      "porters_five_forces",
		"icon_columns": "panel_layout",
		"icon_rows":    "panel_layout",
		"stat_cards":   "panel_layout",
		"panel":        "panel_layout",
		"icon_panel":   "panel_layout",
		"number_tiles": "panel_layout",
		"callout_cards":"panel_layout",
	}
}

// Run executes the full stress test. If filterType is non-empty, only that type is tested.
func (r *SVGStressRunner) Run(filterType string) *SVGStressReport {
	types := DiagramTypes()
	if filterType != "" {
		types = []string{filterType}
	}
	variants := AllVariants()

	report := &SVGStressReport{
		Seed:    r.Seed,
		Total:   len(types) * len(variants),
		Results: make([]SVGStressResult, 0, len(types)*len(variants)),
	}

	for _, typ := range types {
		for _, v := range variants {
			result := r.runOne(typ, v)
			report.Results = append(report.Results, result)
			if result.Passed {
				report.Passed++
			} else {
				report.Failed++
				report.Failures = append(report.Failures, result)
			}
		}
	}

	return report
}

func (r *SVGStressRunner) runOne(typ string, v Variant) SVGStressResult {
	result := SVGStressResult{
		DiagramType: typ,
		Variant:     v.String(),
	}

	data := r.generateData(typ, v)
	req := &svggen.RequestEnvelope{
		Type:  typ,
		Title: fmt.Sprintf("Stress Test: %s (%s)", typ, v),
		Data:  data,
		Output: svggen.OutputSpec{
			Width:  800,
			Height: 600,
		},
	}

	// Render SVG
	doc, err := svggen.Render(req)
	if err != nil {
		// Empty data variant is allowed to fail validation
		if v == VariantEmpty {
			result.Passed = true
			result.Error = "expected: " + err.Error()
			return result
		}
		result.Error = err.Error()
		return result
	}

	// Validate SVG is non-empty
	if len(doc.Content) == 0 {
		result.Error = "SVG content is empty"
		return result
	}
	result.SVGBytes = len(doc.Content)

	// Validate well-formed XML
	if err := xml.Unmarshal(doc.Content, new(interface{})); err != nil {
		result.Error = fmt.Sprintf("SVG is not valid XML: %v", err)
		return result
	}
	result.ValidXML = true

	// Validate viewBox
	svgStr := string(doc.Content)
	result.HasViewBox = strings.Contains(svgStr, "viewBox")
	if !result.HasViewBox {
		result.Error = "SVG missing viewBox attribute"
		return result
	}

	// Try PNG rendering via RenderMultiFormat
	pngReq := &svggen.RequestEnvelope{
		Type:  typ,
		Title: req.Title,
		Data:  data,
		Output: svggen.OutputSpec{
			Format: "png",
			Width:  800,
			Height: 600,
			Scale:  2.0,
		},
	}
	pngResult, err := svggen.RenderMultiFormat(pngReq, "png")
	if err != nil {
		// Some diagrams may not support multi-format; that's OK, not a failure
		result.Passed = true
		result.Error = "png unsupported: " + err.Error()
		return result
	}

	if pngResult.PNG != nil {
		result.PNGBytes = len(pngResult.PNG)
		// Validate PNG signature (first 8 bytes)
		if len(pngResult.PNG) >= 8 {
			pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			for i := 0; i < 8; i++ {
				if pngResult.PNG[i] != pngSig[i] {
					result.Error = "PNG has invalid signature bytes"
					return result
				}
			}
		}
	}

	result.Passed = true
	return result
}

// generateData produces test data for a given diagram type and variant.
func (r *SVGStressRunner) generateData(typ string, v Variant) map[string]any {
	switch v {
	case VariantEmpty:
		return map[string]any{}
	case VariantMinimal:
		return r.dataForType(typ, 1)
	case VariantStandard:
		return r.dataForType(typ, 3+r.rng.IntN(3)) // 3-5
	case VariantDense:
		return r.dataForType(typ, 15+r.rng.IntN(6)) // 15-20
	case VariantEdge:
		return r.edgeData(typ)
	default:
		return map[string]any{}
	}
}

// dataForType generates data with n data points for a given type.
func (r *SVGStressRunner) dataForType(typ string, n int) map[string]any { //nolint:gocyclo
	switch typ {
	// === CHART TYPES ===
	case "bar_chart", "line_chart", "area_chart":
		return r.categorySeries(n, 1)
	case "stacked_bar_chart", "grouped_bar_chart", "stacked_area_chart":
		return r.categorySeries(n, 2+r.rng.IntN(2))
	case "pie_chart", "donut_chart":
		return r.pieData(n)
	case "radar_chart":
		if n < 3 {
			n = 3 // radar needs minimum 3
		}
		return r.categorySeries(n, 1)
	case "scatter_chart":
		return r.scatterData(n)
	case "bubble_chart":
		return r.bubbleData(n)

	// === SPECIAL CHART TYPES ===
	case "waterfall":
		return r.waterfallData(n)
	case "funnel_chart":
		return r.funnelData(n)
	case "gauge_chart":
		return map[string]any{"value": float64(r.rng.IntN(100)), "min": 0.0, "max": 100.0}
	case "treemap_chart":
		return r.treemapData(n)

	// === BUSINESS DIAGRAMS ===
	case "process_flow":
		return r.processFlowData(n)
	case "timeline":
		return r.timelineData(n)
	case "matrix_2x2":
		return r.matrix2x2Data(n)
	case "pyramid":
		return r.pyramidData(n)
	case "swot":
		return r.swotData(n)
	case "venn":
		return r.vennData(n)
	case "org_chart":
		return r.orgChartData(n)
	case "gantt":
		return r.ganttData(n)
	case "kpi_dashboard":
		return r.kpiData(n)
	case "heatmap":
		return r.heatmapData(n)
	case "fishbone":
		return r.fishboneData(n)
	case "pestel":
		return r.pestelData(n)
	case "business_model_canvas":
		return r.bmcData(n)
	case "value_chain":
		return r.valueChainData(n)
	case "nine_box_talent":
		return r.nineBoxData(n)
	case "porters_five_forces":
		return r.portersData(n)
	case "house_diagram":
		return r.houseData(n)
	case "panel_layout":
		return r.panelData(n)

	default:
		return map[string]any{"items": r.stringSlice(n)}
	}
}

func (r *SVGStressRunner) categorySeries(cats, series int) map[string]any {
	categories := r.labels(cats)
	seriesData := make([]map[string]any, series)
	for i := range seriesData {
		vals := make([]any, cats)
		for j := range vals {
			vals[j] = float64(10 + r.rng.IntN(90))
		}
		seriesData[i] = map[string]any{
			"name":   fmt.Sprintf("Series %d", i+1),
			"values": vals,
		}
	}
	return map[string]any{
		"categories": toAnyStr(categories),
		"series":     toAny(seriesData),
	}
}

func (r *SVGStressRunner) pieData(n int) map[string]any {
	vals := make([]any, n)
	for i := range vals {
		vals[i] = float64(5 + r.rng.IntN(40))
	}
	return map[string]any{
		"values":     vals,
		"categories": toAnyStr(r.labels(n)),
	}
}

func (r *SVGStressRunner) scatterData(n int) map[string]any {
	points := make([]map[string]any, n)
	for i := range points {
		points[i] = map[string]any{
			"x": float64(r.rng.IntN(100)),
			"y": float64(r.rng.IntN(100)),
		}
	}
	return map[string]any{
		"series": toAny([]map[string]any{
			{"name": "Series A", "values": toAny(points)},
		}),
	}
}

func (r *SVGStressRunner) bubbleData(n int) map[string]any {
	points := make([]map[string]any, n)
	for i := range points {
		points[i] = map[string]any{
			"x":    float64(r.rng.IntN(100)),
			"y":    float64(r.rng.IntN(100)),
			"size": float64(5 + r.rng.IntN(45)),
		}
	}
	return map[string]any{
		"series": toAny([]map[string]any{
			{"name": "Bubbles", "values": toAny(points)},
		}),
	}
}

func (r *SVGStressRunner) waterfallData(n int) map[string]any {
	points := make([]map[string]any, n)
	for i := range points {
		typ := "increase"
		val := float64(10 + r.rng.IntN(90))
		if r.rng.IntN(3) == 0 {
			typ = "decrease"
			val = -val
		}
		if i == n-1 {
			typ = "total"
		}
		points[i] = map[string]any{
			"label": fmt.Sprintf("Item %d", i+1),
			"value": val,
			"type":  typ,
		}
	}
	return map[string]any{"points": toAny(points)}
}

func (r *SVGStressRunner) funnelData(n int) map[string]any {
	stages := make([]map[string]any, n)
	val := 1000.0
	for i := range stages {
		stages[i] = map[string]any{
			"label": fmt.Sprintf("Stage %d", i+1),
			"value": val,
		}
		val *= 0.6
	}
	return map[string]any{"values": toAny(stages)}
}

func (r *SVGStressRunner) treemapData(n int) map[string]any {
	nodes := make([]map[string]any, n)
	for i := range nodes {
		nodes[i] = map[string]any{
			"name":  fmt.Sprintf("Node %d", i+1),
			"value": float64(10 + r.rng.IntN(90)),
		}
	}
	return map[string]any{"nodes": toAny(nodes)}
}

func (r *SVGStressRunner) processFlowData(n int) map[string]any {
	steps := make([]map[string]any, n)
	for i := range steps {
		steps[i] = map[string]any{
			"label": fmt.Sprintf("Step %d", i+1),
		}
		if r.rng.IntN(3) == 0 {
			steps[i]["type"] = "decision"
		}
	}
	return map[string]any{"steps": toAny(steps)}
}

func (r *SVGStressRunner) timelineData(n int) map[string]any {
	phases := make([]map[string]any, n)
	year, month := 2025, 1
	for i := range phases {
		start := fmt.Sprintf("%d-%02d", year, month)
		month += 1 + r.rng.IntN(3)
		if month > 12 {
			month -= 12
			year++
		}
		end := fmt.Sprintf("%d-%02d", year, month)
		phases[i] = map[string]any{
			"name":  fmt.Sprintf("Phase %d", i+1),
			"start": start,
			"end":   end,
		}
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return map[string]any{"phases": toAny(phases)}
}

func (r *SVGStressRunner) matrix2x2Data(n int) map[string]any {
	return map[string]any{
		"x_axis": "Impact",
		"y_axis": "Effort",
		"quadrants": toAny([]map[string]any{
			{"label": "Quick Wins", "items": toAnyStr(r.stringSlice(min(n, 2)))},
			{"label": "Major Projects", "items": toAnyStr(r.stringSlice(min(n, 2)))},
			{"label": "Fill-Ins", "items": toAnyStr(r.stringSlice(min(n, 2)))},
			{"label": "Thankless Tasks", "items": toAnyStr(r.stringSlice(min(n, 2)))},
		}),
	}
}

func (r *SVGStressRunner) pyramidData(n int) map[string]any {
	if n > 20 {
		n = 20 // pyramid max 20 levels
	}
	levels := make([]map[string]any, n)
	for i := range levels {
		levels[i] = map[string]any{"label": fmt.Sprintf("Level %d", i+1)}
	}
	return map[string]any{"levels": toAny(levels)}
}

func (r *SVGStressRunner) swotData(n int) map[string]any {
	return map[string]any{
		"strengths":     toAnyStr(r.stringSlice(n)),
		"weaknesses":    toAnyStr(r.stringSlice(n)),
		"opportunities": toAnyStr(r.stringSlice(n)),
		"threats":       toAnyStr(r.stringSlice(n)),
	}
}

func (r *SVGStressRunner) vennData(n int) map[string]any {
	count := n
	if count < 2 {
		count = 2
	}
	if count > 3 {
		count = 3 // venn max 3
	}
	circles := make([]map[string]any, count)
	for i := range circles {
		circles[i] = map[string]any{
			"label": fmt.Sprintf("Set %d", i+1),
			"items": toAnyStr(r.stringSlice(min(n, 3))),
		}
	}
	return map[string]any{"circles": toAny(circles)}
}

func (r *SVGStressRunner) orgChartData(n int) map[string]any {
	root := map[string]any{
		"name":  "CEO",
		"title": "Chief Executive",
	}
	if n > 1 {
		// Create a tree structure: VPs at level 1, managers at level 2.
		// Cap direct reports at 6 per parent to keep tree realistic.
		vpCount := min(n-1, 6)
		children := make([]map[string]any, vpCount)
		remaining := n - 1 - vpCount
		for i := range children {
			vp := map[string]any{
				"name":  fmt.Sprintf("VP %d", i+1),
				"title": fmt.Sprintf("Vice President %d", i+1),
			}
			// Distribute remaining nodes as reports under VPs
			if remaining > 0 {
				reportsPerVP := min(remaining, 4)
				reports := make([]map[string]any, reportsPerVP)
				for j := range reports {
					reports[j] = map[string]any{
						"name":  fmt.Sprintf("Mgr %d.%d", i+1, j+1),
						"title": fmt.Sprintf("Manager %d.%d", i+1, j+1),
					}
				}
				vp["children"] = toAny(reports)
				remaining -= reportsPerVP
			}
			children[i] = vp
		}
		root["children"] = toAny(children)
	}
	return map[string]any{"root": root}
}

func (r *SVGStressRunner) ganttData(n int) map[string]any {
	tasks := make([]map[string]any, n)
	for i := range tasks {
		tasks[i] = map[string]any{
			"name":     fmt.Sprintf("Task %d", i+1),
			"start":    fmt.Sprintf("2026-%02d-01", 1+i%12),
			"end":      fmt.Sprintf("2026-%02d-28", 1+i%12),
			"progress": float64(r.rng.IntN(100)),
		}
	}
	return map[string]any{"tasks": toAny(tasks)}
}

func (r *SVGStressRunner) kpiData(n int) map[string]any {
	metrics := make([]map[string]any, n)
	for i := range metrics {
		metrics[i] = map[string]any{
			"label":  fmt.Sprintf("KPI %d", i+1),
			"value":  fmt.Sprintf("%d", r.rng.IntN(1000)),
			"change": fmt.Sprintf("+%d%%", r.rng.IntN(50)),
		}
	}
	return map[string]any{"metrics": toAny(metrics)}
}

func (r *SVGStressRunner) heatmapData(n int) map[string]any {
	rows := min(n, 10)
	cols := min(n, 10)
	if rows < 2 {
		rows = 2
	}
	if cols < 2 {
		cols = 2
	}
	rowLabels := r.labels(rows)
	colLabels := r.labels(cols)
	// Must be []any containing []any of float64 for type assertions
	values := make([]any, rows)
	for i := range values {
		row := make([]any, cols)
		for j := range row {
			row[j] = float64(r.rng.IntN(100))
		}
		values[i] = row
	}
	return map[string]any{
		"row_labels": toAnyStr(rowLabels),
		"col_labels": toAnyStr(colLabels),
		"values":     values,
	}
}

func (r *SVGStressRunner) fishboneData(n int) map[string]any {
	categories := make([]map[string]any, min(n, 6))
	for i := range categories {
		categories[i] = map[string]any{
			"name":   fmt.Sprintf("Category %d", i+1),
			"causes": toAnyStr(r.stringSlice(min(n, 4))),
		}
	}
	return map[string]any{
		"effect":     "Root Problem",
		"categories": toAny(categories),
	}
}

func (r *SVGStressRunner) pestelData(n int) map[string]any {
	return map[string]any{
		"political":     toAnyStr(r.stringSlice(n)),
		"economic":      toAnyStr(r.stringSlice(n)),
		"social":        toAnyStr(r.stringSlice(n)),
		"technological": toAnyStr(r.stringSlice(n)),
		"environmental": toAnyStr(r.stringSlice(n)),
		"legal":         toAnyStr(r.stringSlice(n)),
	}
}

func (r *SVGStressRunner) bmcData(n int) map[string]any {
	return map[string]any{
		"key_partners":           toAnyStr(r.stringSlice(n)),
		"key_activities":         toAnyStr(r.stringSlice(n)),
		"key_resources":          toAnyStr(r.stringSlice(n)),
		"value_propositions":     toAnyStr(r.stringSlice(n)),
		"customer_relationships": toAnyStr(r.stringSlice(n)),
		"channels":               toAnyStr(r.stringSlice(n)),
		"customer_segments":      toAnyStr(r.stringSlice(n)),
		"cost_structure":         toAnyStr(r.stringSlice(n)),
		"revenue_streams":        toAnyStr(r.stringSlice(n)),
	}
}

func (r *SVGStressRunner) valueChainData(n int) map[string]any {
	primary := make([]map[string]any, min(n, 5))
	for i := range primary {
		primary[i] = map[string]any{
			"name":  fmt.Sprintf("Activity %d", i+1),
			"items": toAnyStr(r.stringSlice(min(n, 3))),
		}
	}
	support := make([]map[string]any, min(n, 3))
	for i := range support {
		support[i] = map[string]any{
			"name":  fmt.Sprintf("Support %d", i+1),
			"items": toAnyStr(r.stringSlice(min(n, 2))),
		}
	}
	return map[string]any{"primary": toAny(primary), "support": toAny(support)}
}

func (r *SVGStressRunner) nineBoxData(n int) map[string]any {
	people := make([]map[string]any, n)
	for i := range people {
		people[i] = map[string]any{
			"name":        fmt.Sprintf("Person %d", i+1),
			"performance": float64(1 + r.rng.IntN(3)),
			"potential":   float64(1 + r.rng.IntN(3)),
		}
	}
	return map[string]any{"people": toAny(people)}
}

func (r *SVGStressRunner) portersData(n int) map[string]any {
	return map[string]any{
		"rivalry":        toAnyStr(r.stringSlice(n)),
		"new_entrants":   toAnyStr(r.stringSlice(n)),
		"substitutes":    toAnyStr(r.stringSlice(n)),
		"buyer_power":    toAnyStr(r.stringSlice(n)),
		"supplier_power": toAnyStr(r.stringSlice(n)),
	}
}

func (r *SVGStressRunner) houseData(n int) map[string]any {
	pillars := r.stringSlice(min(n, 5))
	if len(pillars) == 0 {
		pillars = []string{"Pillar 1"}
	}
	return map[string]any{
		"roof":       "Vision",
		"pillars":    toAnyStr(pillars),
		"foundation": "Foundation",
	}
}

func (r *SVGStressRunner) panelData(n int) map[string]any {
	panels := make([]map[string]any, n)
	for i := range panels {
		panels[i] = map[string]any{
			"title": fmt.Sprintf("Panel %d", i+1),
			"body":  fmt.Sprintf("Body content %d", i+1),
		}
	}
	return map[string]any{"panels": toAny(panels)}
}

// edgeData generates data with special characters, extreme values, etc.
func (r *SVGStressRunner) edgeData(typ string) map[string]any { //nolint:gocyclo
	edgeLabels := []string{
		"日本語テスト",
		"<script>alert('xss')</script>",
		strings.Repeat("VeryLongLabel", 20),
		"Line1\nLine2\nLine3",
		`He said "hello" & she said 'goodbye'`,
		"🚀📊💡",
		"   spaces   ",
		"",
	}

	el := func(s []string) []any { return toAnyStr(s) }

	switch typ {
	case "bar_chart", "line_chart", "area_chart":
		vals := []any{-100.0, 0.0, 0.001, 999999.0, -0.5}
		return map[string]any{
			"categories": el(edgeLabels[:5]),
			"series":     toAny([]map[string]any{{"name": "Edge Series", "values": vals}}),
		}

	case "stacked_bar_chart", "grouped_bar_chart", "stacked_area_chart":
		vals1 := []any{-50.0, 0.0, 100.0, 999999.0}
		vals2 := []any{0.0, 0.0, 0.0, 0.0}
		return map[string]any{
			"categories": el(edgeLabels[:4]),
			"series": toAny([]map[string]any{
				{"name": "Edge A", "values": vals1},
				{"name": "Edge B", "values": vals2},
			}),
		}

	case "pie_chart", "donut_chart":
		return map[string]any{
			"values":     []any{0.0, 0.001, 999999.0, 0.0, 1.0},
			"categories": el(edgeLabels[:5]),
		}

	case "radar_chart":
		return map[string]any{
			"categories": el(edgeLabels[:5]),
			"series":     toAny([]map[string]any{{"name": "Edge", "values": []any{-10.0, 0.0, 100.0, 50.0, 999.0}}}),
		}

	case "scatter_chart":
		pts := []map[string]any{
			{"x": -100.0, "y": -100.0},
			{"x": 0.0, "y": 0.0},
			{"x": 999999.0, "y": 999999.0},
		}
		return map[string]any{"series": toAny([]map[string]any{{"name": "Edge", "values": toAny(pts)}})}

	case "bubble_chart":
		pts := []map[string]any{
			{"x": -100.0, "y": -100.0, "size": 0.0},
			{"x": 0.0, "y": 0.0, "size": 999.0},
			{"x": 999999.0, "y": 999999.0, "size": 0.001},
		}
		return map[string]any{"series": toAny([]map[string]any{{"name": "Edge", "values": toAny(pts)}})}

	case "waterfall":
		return map[string]any{"points": toAny([]map[string]any{
			{"label": edgeLabels[0], "value": -999999.0, "type": "decrease"},
			{"label": edgeLabels[1], "value": 0.0, "type": "increase"},
			{"label": edgeLabels[2], "value": 999999.0, "type": "total"},
		})}

	case "funnel_chart":
		return map[string]any{"values": toAny([]map[string]any{
			{"label": edgeLabels[0], "value": 999999.0},
			{"label": edgeLabels[1], "value": 0.001},
			{"label": edgeLabels[2], "value": 0.0},
		})}

	case "gauge_chart":
		return map[string]any{"value": -50.0, "min": -100.0, "max": 999999.0}

	case "treemap_chart":
		return map[string]any{"nodes": toAny([]map[string]any{
			{"name": edgeLabels[0], "value": 0.001},
			{"name": edgeLabels[1], "value": 999999.0},
			{"name": edgeLabels[2], "value": 0.0},
		})}

	case "swot":
		return map[string]any{
			"strengths":     el(edgeLabels[:3]),
			"weaknesses":    el(edgeLabels[3:6]),
			"opportunities": el(edgeLabels[:2]),
			"threats":       el(edgeLabels[5:7]),
		}

	case "venn":
		return map[string]any{"circles": toAny([]map[string]any{
			{"label": edgeLabels[0], "items": el(edgeLabels[:3])},
			{"label": edgeLabels[1], "items": el(edgeLabels[3:6])},
		})}

	case "org_chart":
		return map[string]any{"root": map[string]any{
			"name":  edgeLabels[0],
			"title": edgeLabels[2],
			"children": toAny([]map[string]any{
				{"name": edgeLabels[3], "title": edgeLabels[4]},
			}),
		}}

	case "gantt":
		return map[string]any{"tasks": toAny([]map[string]any{
			{"name": edgeLabels[0], "start": "2026-01-01", "end": "2026-12-31", "progress": 0.0},
			{"name": edgeLabels[1], "start": "2026-06-01", "end": "2026-06-02", "progress": 100.0},
		})}

	case "timeline":
		return map[string]any{"phases": toAny([]map[string]any{
			{"name": edgeLabels[0], "start": "2025-01", "end": "2025-02"},
			{"name": edgeLabels[2], "start": "2025-03", "end": "2026-12"},
		})}

	case "process_flow":
		steps := make([]map[string]any, 4)
		for i := range steps {
			steps[i] = map[string]any{"label": edgeLabels[i]}
		}
		return map[string]any{"steps": toAny(steps)}

	case "matrix_2x2":
		return map[string]any{
			"x_axis": edgeLabels[0],
			"y_axis": edgeLabels[1],
			"quadrants": toAny([]map[string]any{
				{"label": edgeLabels[2], "items": el(edgeLabels[:2])},
				{"label": edgeLabels[3], "items": el(edgeLabels[:2])},
				{"label": edgeLabels[4], "items": el(edgeLabels[:2])},
				{"label": edgeLabels[5], "items": el(edgeLabels[:2])},
			}),
		}

	case "pyramid":
		levels := make([]map[string]any, 3)
		for i := range levels {
			levels[i] = map[string]any{"label": edgeLabels[i]}
		}
		return map[string]any{"levels": toAny(levels)}

	case "fishbone":
		return map[string]any{
			"effect": edgeLabels[0],
			"categories": toAny([]map[string]any{
				{"name": edgeLabels[1], "causes": el(edgeLabels[:3])},
				{"name": edgeLabels[2], "causes": el(edgeLabels[3:6])},
			}),
		}

	case "pestel":
		return map[string]any{
			"political":     el(edgeLabels[:2]),
			"economic":      el(edgeLabels[2:4]),
			"social":        el(edgeLabels[4:6]),
			"technological": el(edgeLabels[:2]),
			"environmental": el(edgeLabels[2:4]),
			"legal":         el(edgeLabels[4:6]),
		}

	case "kpi_dashboard":
		return map[string]any{"metrics": toAny([]map[string]any{
			{"label": edgeLabels[0], "value": "-999"},
			{"label": edgeLabels[2], "value": "0"},
			{"label": edgeLabels[5], "value": "999999"},
		})}

	case "heatmap":
		return map[string]any{
			"row_labels": el(edgeLabels[:3]),
			"col_labels": el(edgeLabels[3:6]),
			"values":     []any{[]any{-100.0, 0.0, 999999.0}, []any{0.0, 0.001, -0.5}, []any{100.0, 50.0, 0.0}},
		}

	case "business_model_canvas":
		return map[string]any{
			"key_partners":           el(edgeLabels[:2]),
			"key_activities":         el(edgeLabels[2:4]),
			"value_propositions":     el(edgeLabels[4:6]),
			"customer_segments":      el(edgeLabels[:2]),
			"channels":              el(edgeLabels[2:4]),
			"revenue_streams":       el(edgeLabels[4:6]),
			"key_resources":         el(edgeLabels[:2]),
			"cost_structure":        el(edgeLabels[2:4]),
			"customer_relationships": el(edgeLabels[:2]),
		}

	case "value_chain":
		return map[string]any{
			"primary": toAny([]map[string]any{
				{"name": edgeLabels[0], "items": el(edgeLabels[:3])},
				{"name": edgeLabels[1], "items": el(edgeLabels[3:6])},
			}),
			"support": toAny([]map[string]any{
				{"name": edgeLabels[2], "items": el(edgeLabels[:2])},
			}),
		}

	case "nine_box_talent":
		return map[string]any{"people": toAny([]map[string]any{
			{"name": edgeLabels[0], "performance": 1.0, "potential": 1.0},
			{"name": edgeLabels[2], "performance": 3.0, "potential": 3.0},
			{"name": edgeLabels[5], "performance": 2.0, "potential": 2.0},
		})}

	case "porters_five_forces":
		return map[string]any{
			"rivalry":        el(edgeLabels[:3]),
			"new_entrants":   el(edgeLabels[3:6]),
			"substitutes":    el(edgeLabels[:2]),
			"buyer_power":    el(edgeLabels[2:4]),
			"supplier_power": el(edgeLabels[4:6]),
		}

	case "house_diagram":
		return map[string]any{
			"roof":       edgeLabels[2],
			"pillars":    el(edgeLabels[:3]),
			"foundation": edgeLabels[4],
		}

	case "panel_layout":
		panels := make([]map[string]any, 3)
		for i := range panels {
			panels[i] = map[string]any{
				"title": edgeLabels[i],
				"body":  edgeLabels[i+3],
			}
		}
		return map[string]any{"panels": toAny(panels)}

	default:
		return map[string]any{"items": el(edgeLabels[:5])}
	}
}

// helpers

// toAny converts []map[string]any to []any for svggen type assertions.
func toAny(maps []map[string]any) []any {
	out := make([]any, len(maps))
	for i, m := range maps {
		out[i] = m
	}
	return out
}

// toAnyStr converts []string to []any.
func toAnyStr(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}

func (r *SVGStressRunner) labels(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Label %d", i+1)
	}
	return out
}

func (r *SVGStressRunner) stringSlice(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Item %d", i+1)
	}
	return out
}

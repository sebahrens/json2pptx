package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// The family-wide split-layout gate (go-slide-creator-27b0c): every circular
// pattern, alone on a slide and in each split placement an agent reaches for,
// on every template, at its smallest and largest item count. Each pattern's
// own matrix test pins its counts, budgets and styles; this one pins what the
// family promises together — a circle stays a circle wherever it is placed,
// its labels stay readable and clear of it and of each other, and validate
// and generate say the same thing about the slide.

// circularMember is one pattern of the family with the copy it carries in the
// matrix: short labels, no descriptions (the copy a ring holds beside a
// second zone).
type circularMember struct {
	pattern  string
	min, max int
	values   func(n int) string   // the pattern's values as JSON
	labels   func(n int) []string // every label the slide must show
	// lobes is the number of ring squares the pattern draws (2 for the figure
	// eight).
	lobes int
	// wideOnly marks the pattern that refuses a narrow area with a
	// fit_overflow finding instead of drawing in it.
	wideOnly bool
}

func circularLabelObjects(key string, labels []string) string {
	items := make([]string, len(labels))
	for i, l := range labels {
		items[i] = fmt.Sprintf(`{"label":%q}`, l)
	}
	return fmt.Sprintf(`%q:[%s]`, key, strings.Join(items, ","))
}

var (
	circularRingLabels    = []string{"Forecast", "Review results", "Decide", "Commit funding", "Execute", "Track benefits", "Close the books", "Reset targets"}
	circularNodeLabels    = []string{"Plan", "Do", "Check", "Act", "Share", "Audit", "Review", "Reset"}
	circularIntakeLabels  = []string{"Attract", "Convert", "Onboard"}
	circularLoopLabels    = []string{"Activate", "Engage", "Renew", "Expand", "Refer", "Assess", "Reprice", "Recommit"}
	circularEightLabels   = []string{"Plan", "Code", "Build", "Test", "Release", "Deploy", "Operate", "Monitor"}
	circularSpokeLabels   = []string{"Marketing", "Sales", "Service", "Finance", "Risk", "Product", "Partners", "Data science"}
	circularLayerLabels   = []string{"Sponsor", "Leadership team", "Function heads", "Front-line teams", "Customers"}
	circularIntakeByLoops = map[int]int{3: 1, 8: 3} // the smallest and the largest combination
)

var circularFamily = []circularMember{
	{pattern: "cycle-ring", min: 4, max: 8, lobes: 1,
		values: func(n int) string { return "{" + circularLabelObjects("phases", circularRingLabels[:n]) + "}" },
		labels: func(n int) []string { return circularRingLabels[:n] }},
	{pattern: "cycle-nodes", min: 3, max: 8, lobes: 1,
		values: func(n int) string { return "{" + circularLabelObjects("steps", circularNodeLabels[:n]) + "}" },
		labels: func(n int) []string { return circularNodeLabels[:n] }},
	{pattern: "cycle-intake", min: 3, max: 8, lobes: 1,
		values: func(n int) string {
			return "{" + circularLabelObjects("intake", circularIntakeLabels[:circularIntakeByLoops[n]]) + "," + circularLabelObjects("loop", circularLoopLabels[:n]) + "}"
		},
		labels: func(n int) []string {
			return append(append([]string(nil), circularIntakeLabels[:circularIntakeByLoops[n]]...), circularLoopLabels[:n]...)
		}},
	{pattern: "cycle-figure-eight", min: 4, max: 8, lobes: 2, wideOnly: true,
		values: func(n int) string {
			return `{"left_label":"Build","right_label":"Run",` + circularLabelObjects("phases", circularEightLabels[:n]) + "}"
		},
		labels: func(n int) []string { return circularEightLabels[:n] }},
	{pattern: "radial-hub", min: 4, max: 8, lobes: 1,
		values: func(n int) string {
			return `{"center":{"label":"Core platform"},` + circularLabelObjects("spokes", circularSpokeLabels[:n]) + "}"
		},
		labels: func(n int) []string { return append([]string{"Core platform"}, circularSpokeLabels[:n]...) }},
	{pattern: "concentric-rings", min: 3, max: 5, lobes: 1,
		values: func(n int) string { return "{" + circularLabelObjects("layers", circularLayerLabels[:n]) + "}" },
		labels: func(n int) []string { return circularLayerLabels[:n] }},
}

// circularPlacement puts a pattern on a slide. narrow marks the placements
// that give the circle at most 60% of the content width, half those that give
// it at most 50%. skip are the neighbour's small-role texts (a KPI strip's
// captions), which are not the circle's and keep their own floor.
type circularPlacement struct {
	name         string
	narrow, half bool
	slide        func(pattern string) string // the slide's pattern / compose / shape_grid member
	// valuesPath is the JSON pointer of the pattern's values on the slide.
	valuesPath string
	skip       []string
}

const (
	circularBullets  = `{"name":"labeled-rows","values":{"rows":[{"label":"WHY","body":"Each cycle starts from last quarter's results"},{"label":"WHO","body":"One owner per phase, named in the charter"},{"label":"HOW","body":"The loop closes before the board meets"}]}}`
	circularTakeaway = `{"name":"stat-hero","values":{"value":"88%","label":"Actions closed on time","context":"14 of 16; two slipped by a week"}}`
	circularKPIRow   = `{"name":"kpi-inline","values":["71% | Revenue retained","112% | Net retention","1 in 5 | Refer a peer"]}`
)

func circularCompose(direction string, circleFirst bool, circlePct int, neighbour string) func(string) string {
	return func(pattern string) string {
		circle := fmt.Sprintf(`{"size_pct":%d,"pattern":%s}`, circlePct, pattern)
		other := fmt.Sprintf(`{"size_pct":%d,"pattern":%s}`, 100-circlePct, neighbour)
		if !circleFirst {
			circle, other = other, circle
		}
		return fmt.Sprintf(`"compose":{"direction":%q,"gap":12,"segments":[%s,%s]}`, direction, circle, other)
	}
}

var circularPlacements = []circularPlacement{
	{name: "alone", valuesPath: "/slides/0/pattern/values",
		slide: func(pattern string) string { return `"pattern":` + pattern }},
	{name: "h50-bullets", narrow: true, half: true, valuesPath: "/slides/0/compose/segments/0/pattern/values",
		slide: circularCompose("horizontal", true, 50, circularBullets)},
	{name: "h60-takeaway", narrow: true, valuesPath: "/slides/0/compose/segments/0/pattern/values",
		slide: circularCompose("horizontal", true, 60, circularTakeaway)},
	{name: "h40-60-right", narrow: true, valuesPath: "/slides/0/compose/segments/1/pattern/values",
		slide: circularCompose("horizontal", false, 60, circularBullets)},
	{name: "v70-kpi-row", valuesPath: "/slides/0/compose/segments/0/pattern/values",
		slide: circularCompose("vertical", true, 70, circularKPIRow),
		skip:  []string{"Revenue retained", "Net retention", "Refer a peer"}},
	{name: "nested-cell", narrow: true, half: true, valuesPath: "/slides/0/shape_grid/rows/0/cells/0/pattern/values",
		slide: func(pattern string) string {
			return `"shape_grid":{"columns":[50,50],"gap":12,"rows":[{"cells":[{"pattern":` + pattern +
				`},{"shape":{"geometry":"rect","fill":"none","text":{"content":"The loop closes 14 of 16 actions on time","align":"l"}}}]}]}`
		}},
}

func circularMatrixDeck(tpl string, m circularMember, p circularPlacement, n int) string {
	pattern := fmt.Sprintf(`{"name":%q,"values":%s}`, m.pattern, m.values(n))
	return fmt.Sprintf(`{"template":%q,"slides":[{"layout_id":"content","source":"Illustrative","content":[{"placeholder_id":"title","type":"text","text_value":"The operating rhythm closes the loop every quarter"}],%s}]}`,
		tpl, p.slide(pattern))
}

// circularMatrixResult is one cell of the pass table.
type circularMatrixResult struct {
	verdict string  // ok | refused | FAIL
	sidePt  float64 // the smallest ring square across templates and counts
}

// circularSmallestSidePt is the readable floor of a ring in any placement:
// under it the badges and nodes cannot hold a 12pt numeral.
const circularSmallestSidePt = 96.0

// TestCircularSplitMatrixAcrossTemplates is the gate. Per cell:
//
//	(a) no refuse-class finding — or, for the wide-only figure eight in a
//	    narrow placement, its documented fit_overflow refusal whose fix swaps
//	    to cycle-ring;
//	(b) every round layer of the resolved grid is a square, one ring square
//	    per lobe, the lobes equal and apart;
//	(c) no text cell overlaps a round layer or another text cell, every label
//	    is on the slide, and every run of the grid is written at 12pt or above
//	    with no stored shrink under it;
//	(d) validate_input (fit report) and generate_presentation give the same
//	    verdict, with the same error findings.
//
// -short runs (a)-(c) from the resolved grid on two templates at the largest
// count; the corpus job runs everything, generation included, on every
// template (the local p-style when present).
func TestCircularSplitMatrixAcrossTemplates(t *testing.T) {
	templates := testutil.AllTestTemplateNames()
	if testing.Short() {
		templates = []string{"abstract", "midnight-blue"}
	}
	mc := testMCPConfig(t)
	var (
		mu      sync.Mutex
		results = map[[2]string]*circularMatrixResult{}
	)
	record := func(m circularMember, p circularPlacement, failed, refused bool, sidePt float64) {
		mu.Lock()
		defer mu.Unlock()
		key := [2]string{m.pattern, p.name}
		r := results[key]
		if r == nil {
			r = &circularMatrixResult{verdict: "ok"}
			results[key] = r
		}
		switch {
		case failed:
			r.verdict = "FAIL"
		case refused && r.verdict == "ok":
			r.verdict = "refused"
		}
		if sidePt > 0 && (r.sidePt == 0 || sidePt < r.sidePt) {
			r.sidePt = sidePt
		}
	}

	t.Run("cells", func(t *testing.T) {
		for _, tplName := range templates {
			tpl := loadCircularTemplate(t, tplName)
			for _, m := range circularFamily {
				counts := []int{m.min, m.max}
				if testing.Short() {
					counts = counts[1:]
				}
				for _, p := range circularPlacements {
					for _, n := range counts {
						t.Run(fmt.Sprintf("%s/%s/%s/%d", tplName, m.pattern, p.name, n), func(t *testing.T) {
							t.Parallel()
							sidePt, refused := 0.0, false
							defer func() { record(m, p, t.Failed(), refused, sidePt) }()
							sidePt, refused = assertCircularCell(t, mc, tpl, circularMatrixDeck(tplName, m, p, n), m, p, n)
						})
					}
				}
			}
		}
	})

	// The pass table: pattern x placement, with the smallest ring the cell
	// drew on any template at either count.
	var b strings.Builder
	fmt.Fprintf(&b, "\n%-20s", "pattern")
	for _, p := range circularPlacements {
		fmt.Fprintf(&b, " %-14s", p.name)
	}
	for _, m := range circularFamily {
		fmt.Fprintf(&b, "\n%-20s", m.pattern)
		for _, p := range circularPlacements {
			cell := "-"
			if r := results[[2]string{m.pattern, p.name}]; r != nil {
				cell = r.verdict
				if r.sidePt > 0 {
					cell += fmt.Sprintf(" %.0fpt", r.sidePt)
				}
			}
			fmt.Fprintf(&b, " %-14s", cell)
		}
	}
	t.Log(b.String())
}

// assertCircularCell checks one cell of the matrix and returns the side of
// the smallest ring square it drew, in points, and whether the cell is the
// wide-only pattern's documented refusal.
func assertCircularCell(t *testing.T, mc *mcpConfig, tpl *circularTemplate, deckJSON string, m circularMember, p circularPlacement, n int) (sidePt float64, refused bool) {
	t.Helper()
	input := circularDeck(t, deckJSON)

	// What generation writes, without the file.
	specs, _, _, convertErr := convertPresentationSlides(circularDeck(t, deckJSON).Slides, tpl.layouts, tpl.width, tpl.height, nil, nil, "", &GridDiagramContext{
		ThemeColors: tpl.theme.Colors,
		FontFamily:  tpl.theme.BodyFont,
		TitleFont:   tpl.theme.TitleFont,
		ViewingMode: tokens.ParseViewingMode(input.ViewingMode),
	}, false)
	// The wide-only figure refuses an area under 580pt wide: every half-width
	// placement on every template, and a 60% share on all but the widest
	// content areas. Where it is not refused it is held to the same checks as
	// the rest of the family.
	refused = m.wideOnly && p.narrow && convertErr != nil && strings.Contains(convertErr.Error(), "580pt wide")
	if m.wideOnly && p.half && !refused {
		t.Errorf("%s in %s is narrower than the pattern draws in: want the 580pt refusal, got %v", m.pattern, p.name, convertErr)
	}

	// (d) One verdict from validate and generate — the corpus run only: it
	// writes a deck per cell.
	var writtenSlide string
	if !testing.Short() {
		writtenSlide = assertCircularVerdict(t, mc, deckJSON, p, refused)
	}
	if refused {
		return 0, true
	}
	if convertErr != nil {
		t.Fatalf("generation refuses the slide: %v", convertErr)
	}

	// (a) No refuse-class finding.
	for _, f := range collectFitFindings(input, tpl.layouts, tpl.width, tpl.height, &tpl.theme) {
		if matrixBlockingFitFinding(f) {
			t.Errorf("refuse-class finding %s at %s: %s", f.Code, f.Path, f.Message)
			continue
		}
		// Nor a text-fit advisory, for the circle or its neighbour: the copy
		// is short and the placement must hold it. The one documented limit
		// is cycle-intake's eight list rows in the upper 70% of a short
		// content area, which the pattern reports instead of shrinking.
		if f.Code == patterns.ErrCodeBodyTooLong || f.Code == "TEXT_EXCEEDS_SHAPE" {
			if m.pattern == "cycle-intake" && n == m.max && p.name == "v70-kpi-row" && strings.Contains(f.Message, "cycle-intake loop[") && strings.Contains(f.Message, "list row") {
				continue
			}
			t.Errorf("text-fit finding %s (%s) at %s: %s", f.Code, f.Action, f.Path, f.Message)
		}
	}

	// (b) Circles are circles in the resolved grid.
	d := circularDrawingOf(circularResolvedSlide(t, input, tpl, 0))
	if len(d.round) == 0 {
		t.Fatal("the resolved grid carries no round layer")
	}
	for _, c := range d.round {
		if diff := c.Bounds.CX - c.Bounds.CY; diff < -2 || diff > 2 || c.Bounds.CX <= 0 {
			t.Errorf("%s layer %q resolves to %d x %d EMU: not round", c.ShapeSpec.Geometry, c.LayerName, c.Bounds.CX, c.Bounds.CY)
		}
	}
	if len(d.squares) != m.lobes {
		t.Fatalf("%d ring cells in the resolved grid, want %d", len(d.squares), m.lobes)
	}
	smallest := 0.0
	if testing.Verbose() {
		t.Logf("ring squares %v, %d round layers, %d text cells", d.squares, len(d.round), len(d.texts))
	}
	for _, sq := range d.squares {
		side := float64(minI64(sq.CX, sq.CY)) / 12700
		if smallest == 0 || side < smallest {
			smallest = side
		}
		if side < circularSmallestSidePt {
			t.Errorf("the ring is %.0fpt across (%s): under the %.0fpt a readable ring needs", side, circularRectPt(sq), circularSmallestSidePt)
		}
	}
	if m.lobes == 2 {
		left, right := d.squares[0], d.squares[1]
		if diff := left.CX - right.CX; diff < -2 || diff > 2 {
			t.Errorf("the lobes are %d and %d EMU wide, want equal", left.CX, right.CX)
		}
		if circularOverlapPt(left, right) > 1 {
			t.Errorf("the lobes overlap: %s and %s", circularRectPt(left), circularRectPt(right))
		}
	}

	// (c) Text cells clear the circle and each other.
	for i, a := range d.texts {
		for _, c := range d.round {
			if over := circularOverlapPt(a.bounds, c.Bounds); over > 1 {
				t.Errorf("text %q (%s) overlaps the %s layer %q (%s) by %.1fpt", a.text, circularRectPt(a.bounds), c.ShapeSpec.Geometry, c.LayerName, circularRectPt(c.Bounds), over)
				break
			}
		}
		for _, b := range d.texts[i+1:] {
			if over := circularOverlapPt(a.bounds, b.bounds); over > 1 && !a.interlocks(b) {
				t.Errorf("text %q (%s) overlaps text %q (%s) by %.1fpt", a.text, circularRectPt(a.bounds), b.text, circularRectPt(b.bounds), over)
			}
		}
	}
	var inMemory strings.Builder
	for _, spec := range specs {
		for _, fragment := range spec.RawShapeXML {
			inMemory.Write(fragment)
		}
	}
	for surface, xml := range map[string]string{"resolved grid": inMemory.String(), "written slide": writtenSlide} {
		if xml == "" {
			continue
		}
		// The source line is the slide's chrome, set by the template.
		for _, low := range circularWrittenTextBelow(xml, 12, append([]string{"Source: Illustrative"}, p.skip...)...) {
			t.Errorf("%s: text under 12pt: %s", surface, low)
		}
		for _, label := range m.labels(n) {
			if !strings.Contains(xml, ">"+label+"</a:t>") {
				t.Errorf("%s: label %q is not on the slide", surface, label)
			}
		}
	}
	return smallest, false
}

// assertCircularVerdict runs the deck through validate_input (with the fit
// report) and generate_presentation, requires one verdict, and returns the
// written slide's XML ("" for a refused deck).
func assertCircularVerdict(t *testing.T, mc *mcpConfig, deckJSON string, p circularPlacement, wantRefusal bool) string {
	t.Helper()
	var presentation any
	if err := json.Unmarshal([]byte(deckJSON), &presentation); err != nil {
		t.Fatal(err)
	}
	decode := func(name, text string) map[string]any {
		var doc map[string]any
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			t.Fatalf("%s: answer is not JSON: %v\n%.600s", name, err, text)
		}
		return doc
	}
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": presentation, "fit_report": true}))
	if err != nil {
		t.Fatal(err)
	}
	validateText := textContent(res)
	validateDoc := decode("validate_input", validateText)
	validate := verdictOf(!res.IsError, envelopeFindings(t, validateDoc))

	gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": presentation}))
	if err != nil {
		t.Fatal(err)
	}
	genText := textContent(gen)
	genDoc := decode("generate_presentation", genText)
	success := !gen.IsError
	if reported, ok := genDoc["success"].(bool); ok && !gen.IsError {
		success = reported
	}
	generate := verdictOf(success, envelopeFindings(t, genDoc))
	if validate.String() != generate.String() {
		t.Errorf("validate and generate disagree:\n  validate_input        %s\n  generate_presentation %s", validate, generate)
	}

	if wantRefusal {
		if validate.Valid || generate.Valid {
			t.Errorf("a wide-only figure in %s was approved (validate %s, generate %s)", p.name, validate, generate)
		}
		for surface, text := range map[string]string{"validate_input": validateText, "generate_presentation": genText} {
			for _, want := range []string{"fit_overflow", "content area at least 580pt wide", "use cycle-ring", p.valuesPath, `"swap_pattern"`, `"to":"cycle-ring"`} {
				if !strings.Contains(text, want) {
					t.Errorf("%s refusal does not carry %q: %.700s", surface, want, text)
				}
			}
		}
		return ""
	}
	if !validate.Valid || !generate.Valid {
		t.Errorf("the slide is refused: validate %s, generate %s", validate, generate)
	}
	// The fit report itself asks for no refusal either.
	var refusing []string
	for _, raw := range answerFindings(t, validateDoc) {
		f, _ := raw.(map[string]any)
		ev, _ := f["evidence"].(map[string]any)
		if action, _ := ev["action"].(string); action == "refuse" || f["action"] == "refuse" {
			refusing = append(refusing, fmt.Sprintf("%v: %v", f["code"], f["message"]))
		}
	}
	sort.Strings(refusing)
	for _, r := range refusing {
		t.Errorf("validate_input reports a refuse-class finding: %s", r)
	}
	path, _ := genDoc["output_path"].(string)
	if path == "" {
		t.Fatalf("generate_presentation returned no output_path: %.400s", genText)
	}
	return readSlideXML(t, path, "ppt/slides/slide1.xml")
}

// The matrix covers each member of the registry's circular family: a seventh
// pattern has to join it (or state here why it does not).
func TestCircularSplitMatrixCoversTheFamily(t *testing.T) {
	covered := map[string]bool{}
	for _, m := range circularFamily {
		covered[m.pattern] = true
		if _, ok := patterns.Default().Get(m.pattern); !ok {
			t.Errorf("%s is in the matrix and not in the registry", m.pattern)
		}
	}
	for _, p := range patterns.Default().List() {
		name := p.Name()
		if (strings.HasPrefix(name, "cycle-") || name == "radial-hub" || name == "concentric-rings") && !covered[name] {
			t.Errorf("%s is a circular pattern the split matrix does not cover: add it to circularFamily", name)
		}
	}
}

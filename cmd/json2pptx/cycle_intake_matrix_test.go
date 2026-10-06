package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// cycleIntakeBudgets are the loop copy budgets pinned per (intake, loop)
// combination: label and description characters every phase can carry at once,
// written unshrunk at 12pt or above on every shipped template and on the local
// p-style (abstract's 687 x 294pt body is the binding one). Description 0 =
// labels only (eight phases: the list holds a description line only where the
// body is about 320pt tall or more). The intake's budgets are its schema
// maxima at every count: label 24 (words of about nine letters with three steps), description
// 60.
var cycleIntakeBudgets = map[[2]int]struct{ label, description int }{
	{1, 3}: {26, 70}, {2, 3}: {26, 70}, {3, 3}: {26, 70},
	{1, 4}: {26, 70}, {2, 4}: {26, 70}, {3, 4}: {22, 65},
	{1, 5}: {26, 70}, {2, 5}: {26, 60}, {3, 5}: {22, 45},
	{1, 6}: {26, 40}, {2, 6}: {26, 30}, {3, 6}: {22, 22},
	{1, 7}: {26, 40}, {2, 7}: {26, 30}, {3, 7}: {22, 22},
	{1, 8}: {26, 0}, {2, 8}: {26, 0}, {3, 8}: {22, 0},
}

const (
	cycleIntakeStepLabelBudget = 24
	cycleIntakeStepDescBudget  = 60
)

// cycleIntakeProbeCopy is length characters of short words, the way real copy
// wraps (an intake arrow of a three-step lane holds a word of about nine letters).
func cycleIntakeProbeCopy(length int) string {
	if length <= 0 {
		return ""
	}
	return strings.TrimSpace(budgetProbeCopy(length))
}

// cycleIntakeBudgetValues is nIntake steps at the intake budgets feeding
// nLoop phases whose labels and descriptions are labelLen and descLen long.
func cycleIntakeBudgetValues(nIntake, nLoop, labelLen, descLen int) *patterns.CycleIntakeValues {
	v := &patterns.CycleIntakeValues{}
	for i := 0; i < nIntake; i++ {
		v.Intake = append(v.Intake, patterns.CycleIntakeStep{
			Label:       cycleIntakeProbeCopy(cycleIntakeStepLabelBudget),
			Description: cycleIntakeProbeCopy(cycleIntakeStepDescBudget),
		})
	}
	for i := 0; i < nLoop; i++ {
		v.Loop = append(v.Loop, patterns.CycleIntakePhase{Label: cycleIntakeProbeCopy(labelLen), Description: cycleIntakeProbeCopy(descLen)})
	}
	return v
}

func cycleIntakeSlide(t *testing.T, v *patterns.CycleIntakeValues, style string) SlideInput {
	t.Helper()
	return SlideInput{SlideType: "content", LayoutID: "blank-title", Pattern: cycleIntakePattern(t, v, style)}
}

func cycleIntakePattern(t *testing.T, v *patterns.CycleIntakeValues, style string) *PatternInput {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := &PatternInput{Name: "cycle-intake", Values: encoded}
	if style != "" {
		p.Overrides = json.RawMessage(fmt.Sprintf(`{"loop_style":%q}`, style))
	}
	return p
}

// cycleIntakeMatrixTemplates is every shipped template plus the local p-style;
// -short keeps the smallest body, a typical one and p-style.
func cycleIntakeMatrixTemplates() []string {
	names := testutil.AllTestTemplateNames()
	if !testing.Short() {
		return names
	}
	return slices.DeleteFunc(names, func(name string) bool {
		return name != "abstract" && name != "midnight-blue" && name != "p-style"
	})
}

// assertCycleIntakeClean fails on any readability finding, any run written
// below its role floor, any refusal and any cycle-intake copy warning.
func assertCycleIntakeClean(t *testing.T, input *PresentationInput, geom schemaMaximaGeometry) {
	t.Helper()
	for _, f := range collectReadabilityFindings(input, geom.layouts, geom.width, geom.height) {
		t.Errorf("readability finding: %s %s", f.Code, f.Message)
	}
	if n := writtenRoleViolations(t, input, geom); n > 0 {
		t.Errorf("%d runs written below their role floor", n)
	}
	for _, f := range collectFitFindings(input, geom.layouts, geom.width, geom.height, nil) {
		if f.Action == "refuse" || (f.Code == patterns.ErrCodeBodyTooLong && strings.Contains(f.Message, "cycle-intake")) {
			t.Errorf("fit finding: %s (%s) %s", f.Code, f.Action, f.Message)
		}
	}
}

// Run with JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE=1 to measure, per combination,
// the readable loop description budget with every other field at its pinned
// length, through the fit-report collector on every template.
func TestCycleIntakeBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE=1")
	}
	for nI := 1; nI <= 3; nI++ {
		for nL := 3; nL <= 8; nL++ {
			label := cycleIntakeBudgets[[2]int{nI, nL}].label
			budget := probeReadableBudget(t, "cycle-intake", 70, func(length int) any {
				return cycleIntakeBudgetValues(nI, nL, label, length)
			})
			t.Logf("intake=%d loop=%d label=%d description budget=%d (pinned %d)", nI, nL, label, budget, cycleIntakeBudgets[[2]int{nI, nL}].description)
		}
	}
}

// TestCycleIntakeMatrixAcrossTemplates is the intake x loop x template wall:
// every combination at its pinned copy budget, in both loop styles, renders on
// every template (abstract's smallest body and the local p-style included)
// with no readability finding, no run written below its role floor, no refusal
// and no pattern warning. 3 intake steps x 8 phases is the hard requirement.
func TestCycleIntakeMatrixAcrossTemplates(t *testing.T) {
	for _, templateName := range cycleIntakeMatrixTemplates() {
		geom := loadSchemaMaximaGeometry(t, templateName)
		for nI := 1; nI <= 3; nI++ {
			for _, nL := range []int{3, 4, 5, 6, 7, 8} {
				for _, style := range []string{"segments", "nodes"} {
					t.Run(fmt.Sprintf("%s/%d+%d/%s", templateName, nI, nL, style), func(t *testing.T) {
						b := cycleIntakeBudgets[[2]int{nI, nL}]
						values := cycleIntakeBudgetValues(nI, nL, b.label, b.description)
						values.Loop[nL-1].Highlight = true
						values.Center = &patterns.CycleIntakeCenter{Label: "Each month"}
						input := &PresentationInput{Template: geom.name, Slides: []SlideInput{cycleIntakeSlide(t, values, style)}}
						assertCycleIntakeClean(t, input, geom)
					})
				}
			}
		}
	}
}

var (
	cycleIntakeShapeRE = regexp.MustCompile(`(?s)<a:off x="(-?\d+)" y="(-?\d+)"/>\s*<a:ext cx="(\d+)" cy="(\d+)"/>.*?<a:prstGeom prst="(\w+)">`)
)

// cycleIntakeWritten is a written preset shape: its box in EMU.
type cycleIntakeWritten struct {
	geometry     string
	x, y, cx, cy int64
}

// cycleIntakeWrittenShapes returns every preset shape the slide writes.
func cycleIntakeWrittenShapes(t *testing.T, input *PresentationInput, geom schemaMaximaGeometry) []cycleIntakeWritten {
	t.Helper()
	specs, _, _, err := convertPresentationSlides(input.Slides, geom.layouts, geom.width, geom.height, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var out []cycleIntakeWritten
	for _, spec := range specs {
		for _, fragment := range spec.RawShapeXML {
			for _, raw := range writtenShapeRE.FindAllString(string(fragment), -1) {
				m := cycleIntakeShapeRE.FindStringSubmatch(raw)
				if m == nil {
					continue
				}
				n := func(s string) int64 { v, _ := strconv.ParseInt(s, 10, 64); return v }
				out = append(out, cycleIntakeWritten{geometry: m[5], x: n(m[1]), y: n(m[2]), cx: n(m[3]), cy: n(m[4])})
			}
		}
	}
	return out
}

// assertCycleIntakeDrawn checks what the slide writes: the lane's arrows, one
// entry arrow whose tip touches the loop, and a loop that is still round
// (square segment frames, or circular nodes), at least 120pt across.
func assertCycleIntakeDrawn(t *testing.T, shapes []cycleIntakeWritten, nIntake, nLoop int, style string) {
	t.Helper()
	count := map[string]int{}
	var entry cycleIntakeWritten
	var loopX0, loopY0, loopX1, loopY1 int64
	first := true
	grow := func(s cycleIntakeWritten) {
		if first {
			loopX0, loopY0, loopX1, loopY1, first = s.x, s.y, s.x+s.cx, s.y+s.cy, false
			return
		}
		if s.x < loopX0 {
			loopX0 = s.x
		}
		if s.y < loopY0 {
			loopY0 = s.y
		}
		if s.x+s.cx > loopX1 {
			loopX1 = s.x + s.cx
		}
		if s.y+s.cy > loopY1 {
			loopY1 = s.y + s.cy
		}
	}
	for _, s := range shapes {
		count[s.geometry]++
		switch s.geometry {
		case "rightArrow", "downArrow":
			entry = s
		case "blockArc":
			if d := s.cx - s.cy; d < -2 || d > 2 {
				t.Errorf("a ring segment's frame is %d x %d EMU: the ring is not round", s.cx, s.cy)
			}
			grow(s)
		case "ellipse":
			if d := s.cx - s.cy; d < -2 || d > 2 {
				t.Errorf("a circle is %d x %d EMU", s.cx, s.cy)
			}
			if float64(s.cx)/12700 < 18 {
				t.Errorf("a circle is only %.1fpt across", float64(s.cx)/12700)
			}
			if style == "nodes" {
				grow(s)
			}
		}
	}
	if got := count["homePlate"] + count["chevron"]; got != nIntake || count["homePlate"] != 1 {
		t.Errorf("%d intake arrows (%d pentagons), want %d led by one pentagon", got, count["homePlate"], nIntake)
	}
	if got := count["rightArrow"] + count["downArrow"]; got != 1 {
		t.Fatalf("%d entry arrows, want 1", got)
	}
	if style == "nodes" {
		if count["ellipse"] != nLoop || count["circularArrow"] != nLoop || count["blockArc"] != 0 {
			t.Errorf("nodes: %d circles, %d links, %d segments; want %d, %d, 0", count["ellipse"], count["circularArrow"], count["blockArc"], nLoop, nLoop)
		}
	} else if count["blockArc"] != nLoop || count["ellipse"] != nLoop {
		t.Errorf("segments: %d segments and %d badges, want %d each", count["blockArc"], count["ellipse"], nLoop)
	}
	if side := float64(loopX1-loopX0) / 12700; side < 119 {
		t.Errorf("the loop is %.0fpt across, under the 120pt floor", side)
	}
	// Nodes do not reach the square's top and bottom edge at every count; the
	// arrow's own axis is the one that has to meet the loop.
	gap := float64(loopX0-(entry.x+entry.cx)) / 12700
	if entry.geometry == "downArrow" {
		gap = float64(loopY0-(entry.y+entry.cy)) / 12700
	}
	if gap < -1 || gap > 1 {
		t.Errorf("the %s ends %.2fpt from the loop; it must touch it within 1pt", entry.geometry, gap)
	}
}

// The 3 + 8 maximum (and the other corners) as written on every template: the
// lane, the arrow and a round loop of at least 120pt are all there, and the
// arrow touches the loop.
func TestCycleIntakeWrittenGeometryAcrossTemplates(t *testing.T) {
	for _, templateName := range cycleIntakeMatrixTemplates() {
		geom := loadSchemaMaximaGeometry(t, templateName)
		for _, combo := range [][2]int{{1, 3}, {1, 8}, {2, 5}, {3, 3}, {3, 8}} {
			for _, style := range []string{"segments", "nodes"} {
				t.Run(fmt.Sprintf("%s/%d+%d/%s", templateName, combo[0], combo[1], style), func(t *testing.T) {
					b := cycleIntakeBudgets[combo]
					values := cycleIntakeBudgetValues(combo[0], combo[1], b.label, b.description)
					input := &PresentationInput{Template: geom.name, Slides: []SlideInput{cycleIntakeSlide(t, values, style)}}
					shapes := cycleIntakeWrittenShapes(t, input, geom)
					assertCycleIntakeDrawn(t, shapes, combo[0], combo[1], style)
					var arrow cycleIntakeWritten
					for _, s := range shapes {
						if s.geometry == "rightArrow" {
							arrow = s
						}
					}
					if arrow.geometry == "" {
						t.Fatal("a full-width slide must enter the loop from the left (rightArrow)")
					}
				})
			}
		}
	}
}

// cycleIntakeSplitValues is the copy a loop carries beside a second zone:
// short labels, no descriptions.
func cycleIntakeSplitValues(nIntake, nLoop int) *patterns.CycleIntakeValues {
	steps := []patterns.CycleIntakeStep{{Label: "Attract"}, {Label: "Convert"}, {Label: "Onboard"}}
	phases := []patterns.CycleIntakePhase{
		{Label: "Activate"}, {Label: "Engage"}, {Label: "Renew"}, {Label: "Expand"},
		{Label: "Refer"}, {Label: "Review"}, {Label: "Reprice"}, {Label: "Recommit"},
	}
	v := &patterns.CycleIntakeValues{Intake: steps[:nIntake], Loop: phases[:nLoop]}
	v.Loop[1].Highlight = true
	return v
}

// TestCycleIntakeSplitLayoutsAcrossTemplates is the split wall: in a 50% and a
// 60% horizontal compose segment beside bullets-like content, in a vertical
// 65% segment above a kpi-inline row and in a nested shape-grid cell the loop
// stays round, the lane keeps all its arrows at 12pt, the entry arrow still
// touches the loop (from the left, or from above in the stacked layout a
// narrow segment takes), and nothing is refused, shrunk or warned about.
func TestCycleIntakeSplitLayoutsAcrossTemplates(t *testing.T) {
	bullets := PatternInput{Name: "labeled-rows", Values: json.RawMessage(`{"rows":[{"label":"WHY","body":"Renewals carry most of the revenue"},{"label":"HOW","body":"One loop owned by the account team"}]}`)}
	kpi := PatternInput{Name: "kpi-inline", Values: json.RawMessage(`["71% | Revenue retained","112% | Net retention","1 in 5 | Refer a peer"]`)}
	type layout struct {
		name    string
		maxLoop int // phases the segment holds without a warning on every template
		slide   func(ci PatternInput) SlideInput
	}
	compose := func(direction string, share float64, other PatternInput) func(PatternInput) SlideInput {
		return func(ci PatternInput) SlideInput {
			return SlideInput{SlideType: "content", LayoutID: "blank-title", Compose: &ComposeInput{Direction: direction, Segments: []SegmentInput{
				{SizePct: share, Pattern: ci}, {SizePct: 100 - share, Pattern: other},
			}}}
		}
	}
	layouts := []layout{
		{"horizontal-50", 8, compose("horizontal", 50, bullets)},
		{"horizontal-60", 8, compose("horizontal", 60, bullets)},
		{"vertical-65", 6, compose("vertical", 65, kpi)},
		{"nested-cell", 8, func(ci PatternInput) SlideInput {
			nested, _ := json.Marshal(ci)
			return SlideInput{SlideType: "content", LayoutID: "blank-title", ShapeGrid: &ShapeGridInput{
				Columns: json.RawMessage(`[50,50]`),
				Rows: []GridRowInput{{Cells: []*GridCellInput{
					{Pattern: nested},
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: json.RawMessage(`{"content":"The loop renews seven in ten accounts","size":18}`)}},
				}}},
			}}
		}},
	}
	for _, templateName := range cycleIntakeMatrixTemplates() {
		geom := loadSchemaMaximaGeometry(t, templateName)
		for _, l := range layouts {
			for _, combo := range [][2]int{{1, 4}, {2, 5}, {3, 6}, {3, 8}} {
				for _, style := range []string{"segments", "nodes"} {
					if combo[1] > l.maxLoop {
						continue
					}
					t.Run(fmt.Sprintf("%s/%s/%d+%d/%s", templateName, l.name, combo[0], combo[1], style), func(t *testing.T) {
						ci := cycleIntakePattern(t, cycleIntakeSplitValues(combo[0], combo[1]), style)
						input := &PresentationInput{Template: geom.name, Slides: []SlideInput{l.slide(*ci)}}
						assertCycleIntakeClean(t, input, geom)
						var own []cycleIntakeWritten
						for _, s := range cycleIntakeWrittenShapes(t, input, geom) {
							switch s.geometry {
							case "homePlate", "chevron", "rightArrow", "downArrow", "blockArc", "ellipse", "circularArrow":
								own = append(own, s)
							}
						}
						assertCycleIntakeDrawn(t, own, combo[0], combo[1], style)
					})
				}
			}
		}
	}
}

// Eight phases in the upper half of a vertical split cannot all keep a 12pt
// row on the shortest body: the pattern says so (the documented fallback)
// instead of shrinking silently.
func TestCycleIntakeShortSegmentReportsTheRowsItCannotHold(t *testing.T) {
	geom := loadSchemaMaximaGeometry(t, "abstract")
	kpi := PatternInput{Name: "kpi-inline", Values: json.RawMessage(`["71% | Revenue retained","112% | Net retention","1 in 5 | Refer a peer"]`)}
	ci := cycleIntakePattern(t, cycleIntakeSplitValues(3, 8), "")
	input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{SlideType: "content", LayoutID: "blank-title",
		Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{SizePct: 45, Pattern: *ci}, {SizePct: 55, Pattern: kpi}}}}}}
	found := false
	for _, f := range collectFitFindings(input, geom.layouts, geom.width, geom.height, nil) {
		if f.Code == patterns.ErrCodeBodyTooLong && strings.Contains(f.Message, "cycle-intake loop[") && strings.Contains(f.Message, "list row") {
			found = true
		}
	}
	if !found {
		t.Error("eight list rows in a 45% vertical segment on abstract: want BODY_TOO_LONG naming the list rows")
	}
}

// A description longer than the combination's budget reaches the fit report
// on every template, naming the phase.
func TestCycleIntakeDenseDescriptionWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := cycleIntakeBudgetValues(3, 6, 12, 60)
	assertBudgetFindingAcrossTemplates(t, "cycle-intake", values, "loop[0].description", "with 3 intake steps and 6 phases use about 22")
}

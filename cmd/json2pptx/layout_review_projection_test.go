package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"math"
	"testing"
)

func TestDetachedLabelUsesNestedGridGapAndRelatedSlot(t *testing.T) {
	for _, tc := range []struct {
		gap, space float64
		want       int
	}{{8, 13, 1}, {20, 20, 0}, {20, 31, 1}} {
		frame := pptx.RectEmu{CX: 100 * 12700, CY: 40 * 12700}
		figure := layoutReviewText{path: "/grid/rows/0/cells/0/grid/rows/0/cells/0", pattern: "kpi-3up", gridGap: tc.gap, frame: frame, ink: pptx.RectEmu{CX: 50 * 12700, CY: 30 * 12700}, paragraphs: []geomParagraph{{text: "Unlimited", sizePt: 24}}, figure: true}
		label := layoutReviewText{path: "/grid/rows/0/cells/0/grid/rows/1/cells/0", pattern: "kpi-3up", gridGap: tc.gap, frame: frame, ink: pptx.RectEmu{Y: int64((30 + tc.space) * 12700), CX: 50 * 12700, CY: 12 * 12700}, paragraphs: []geomParagraph{{text: "Capacity", sizePt: 12}}}
		a := geomAccumulator{reviewText: []layoutReviewText{figure, label}}
		if got := len(a.detachedLabelFindings()); got != tc.want {
			t.Fatalf("gap%v space%v findings%d want%d", tc.gap, tc.space, got, tc.want)
		}
		a.reviewText[1].path = "/grid/rows/0/cells/1/grid/rows/1/cells/0"
		if len(a.detachedLabelFindings()) != 0 {
			t.Fatal("unrelated adjacent pane treated as figure parent")
		}
	}
}
func TestGridHierarchyExcludesFiguresAndCardSubheads(t *testing.T) {
	text := func(content string, size float64, bold bool) layoutReviewText {
		return layoutReviewText{path: "/grid/rows/0/cells/0", paragraphs: []geomParagraph{{text: content, sizePt: size, bold: bold}}}
	}
	for _, tc := range []struct {
		heading float64
		pattern string
		want    int
	}{{18, "", 1}, {14, "", 0}, {18, "kpi-3up", 0}, {24, "", 0}} {
		h, b := text("Heading", tc.heading, true), text("Prose", 18, false)
		h.pattern, b.pattern = tc.pattern, tc.pattern
		a := geomAccumulator{reviewText: []layoutReviewText{h, b}}
		if got := len(a.gridHierarchyFindings()); got != tc.want {
			t.Fatalf("%+v got%d", tc, got)
		}
	}
}
func TestRhythmRecursiveComposeNativeLeaves(t *testing.T) {
	inner := &ComposeInput{Direction: "vertical", Segments: []SegmentInput{composeTextLeaf("A"), {Image: &jsonschema.GridImageInput{Path: "evidence.png"}, SizePct: 75}, composeTextLeaf("B")}}
	inner.Segments[0].SizePct = 10
	inner.Segments[2].SizePct = 15
	outer := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{Compose: inner, SizePct: 60}, composeTextLeaf("C")}}
	var rs rhythm.Slide
	projectComposeGeometry(outer, &rs)
	if rs.PaneCount != 4 || rs.VisualSide != "left" || math.Abs(rs.VisualAreaShare-.45) > 1e-9 {
		t.Fatalf("recursive projection %+v", rs)
	}
	outer.SmartCompose = true
	projectComposeGeometry(outer, &rs)
	if rs.VisualAreaShare != 0 || rs.SplitPct != 0 {
		t.Fatalf("unknown measured smart shares presented as measured: %+v", rs)
	}
}
func TestRhythmRecursiveRawGridCountsNativeLeaves(t *testing.T) {
	raw := &ShapeGridInput{Columns: json.RawMessage(`2`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Grid: &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Image: &jsonschema.GridImageInput{Path: "evidence.png"}}}}, {Cells: []*GridCellInput{composePaneText("Explanation", 12, false)}}}}}, composePaneText("Argument", 12, false)}}}}
	rs := rhythm.Slide{Grid: buildDensityGrid(raw)}
	projectRhythmRawGrid(raw, &rs)
	if rs.PaneCount != 3 || rs.VisualSide != "left" || rs.VisualAreaShare <= 0 || rs.VisualAreaShare >= .5 {
		t.Fatalf("raw nested projection %+v", rs)
	}
}

func TestBaselinePeersRespectKnownCenteredBlockRoles(t *testing.T) {
	for _, pattern := range []string{"contact-directory", "state-shift-hub", "swimlane", "kpi-3up", ""} {
		for _, anchor := range []string{"", "top"} {
			frame := pptx.RectEmu{CX: 100 * 12700, CY: 80 * 12700}
			a := geomAccumulator{reviewText: []layoutReviewText{
				{path: "/grid/rows/0/cells/0", pattern: pattern, anchor: anchor, geometry: "rect", frame: frame, baseline: 20, paragraphs: []geomParagraph{{text: "First label", sizePt: 12}}},
				{path: "/grid/rows/0/cells/1", pattern: pattern, anchor: anchor, geometry: "rect", frame: frame, baseline: 30, paragraphs: []geomParagraph{{text: "Second label", sizePt: 12}, {text: "Wrapped line", sizePt: 12}}},
			}}
			if pattern == "state-shift-hub" {
				a.reviewText[0].paragraphs = append(a.reviewText[0].paragraphs, geomParagraph{text: "Annotation", sizePt: 12})
			}
			want := 1
			if anchor == "" && pattern != "" && pattern != "kpi-3up" {
				want = 0
			}
			if got := len(a.baselineReviewFindings()); got != want {
				t.Fatalf("pattern%q anchor%q got%d want%d", pattern, anchor, got, want)
			}
		}
	}
}
func TestOrphanChecksHeadingRolesInsteadOfBoldMessages(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		size    float64
		want    bool
	}{{"text-sidebar", 18, false}, {"state-shift-hub", 18, false}, {"", 14, false}, {"", 18, true}} {
		text := layoutReviewText{pattern: tc.pattern, paragraphs: []geomParagraph{{text: "A bold message", sizePt: tc.size, bold: true}}}
		if got := layoutReviewIsHeading(text); got != tc.want {
			t.Fatalf("%+v heading=%v", tc, got)
		}
	}
}

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// findCode returns the first finding with the given code, or nil.
func findCode(fs []patterns.FitFinding, code string) *patterns.FitFinding {
	for i := range fs {
		if fs[i].Code == code {
			return &fs[i]
		}
	}
	return nil
}

// --- OVERTALL_FLOW_LANE (retired) ------------------------------------------

// go-slide-creator-0l7dr: the finding said a timeline's "boxes stretch
// vertically". No timeline renders that way — uncapped it is sized to its
// text, capped its rows spread while its marks keep their size — so no flow,
// capped or not, draws a finding that says so.
func TestNoFlowFindingSaysBoxesStretch(t *testing.T) {
	seven := json.RawMessage(`[{"label":"A"},{"label":"B"},{"label":"C"},{"label":"D"},{"label":"E"},{"label":"F"},{"label":"G"}]`)
	four := json.RawMessage(`[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Scale"}]`)
	for _, p := range []*PatternInput{
		{Name: "timeline-horizontal", Values: seven},
		{Name: "timeline-horizontal", MaxHeightPct: 60, Values: seven},
		{Name: "timeline-horizontal", MaxHeightPct: 60, Values: four},
		{Name: "timeline-horizontal", Values: four},
		{Name: "process-flow", MaxHeightPct: 60, Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B"},{"label":"C"},{"label":"D"},{"label":"E"},{"label":"F"},{"label":"G"}]}`)},
	} {
		in := patternSlide(p)
		for _, f := range collectFitFindings(&in, nil, 9144000, 6858000, nil) {
			if f.Code == "OVERTALL_FLOW_LANE" || strings.Contains(f.Message, "stretch") {
				t.Errorf("%s (max_height_pct %v): %s: %s", p.Name, p.MaxHeightPct, f.Code, f.Message)
			}
		}
	}
	if _, ok := patterns.GetFindingMeta("OVERTALL_FLOW_LANE"); ok {
		t.Error("the retired OVERTALL_FLOW_LANE is still described")
	}
}

// --- FLOW_DIAMOND_NO_CONTENT ----------------------------------------------

func TestFlowDiamondNoContent_DecisionStep_Fires(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Request"},{"label":"Review","type":"decision"},{"label":"Deploy"}]}`),
	})
	f := findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeFlowDiamondNoContent)
	if f == nil {
		t.Fatal("expected FLOW_DIAMOND_NO_CONTENT for a process-flow with a decision diamond")
	}
	if f.Fix == nil || f.Fix.Params["diamond_count"] != 1 {
		t.Errorf("fix params = %+v, want diamond_count 1", f.Fix)
	}
}

func TestFlowDiamondNoContent_NoDecision_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}`),
	})
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeFlowDiamondNoContent) != nil {
		t.Error("FLOW_DIAMOND_NO_CONTENT should not fire without a decision step")
	}
}

// Compose-embedded flows are exempt (a second zone explains the branch); the
// detector reads slide.Pattern, which is unset for compose slides.
func TestFlowDiamondNoContent_ComposeExempt(t *testing.T) {
	in := PresentationInput{Slides: []SlideInput{{Compose: &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "process-flow", Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B","type":"decision"},{"label":"C"}]}`)}},
			{Pattern: PatternInput{Name: "pull-quote", Values: json.RawMessage(`{"quote":"x","attribution":"y"}`)}},
		},
	}}}}
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeFlowDiamondNoContent) != nil {
		t.Error("FLOW_DIAMOND_NO_CONTENT should not fire for a compose-embedded flow")
	}
}

// --- TOC_FLOWCHART_VOCAB ---------------------------------------------------

func TestTocFlowchartVocab_AgendaTitleFlowPattern_Fires(t *testing.T) {
	agenda := "Agenda"
	in := PresentationInput{Slides: []SlideInput{{
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &agenda}},
		Pattern: &PatternInput{Name: "process-flow", Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B"},{"label":"C"}]}`)},
	}}}
	f := findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeTocFlowchartVocab)
	if f == nil {
		t.Fatal("expected TOC_FLOWCHART_VOCAB for an Agenda title drawn with process-flow")
	}
	if f.Fix == nil || f.Fix.Params["reason"] != "toc_as_flowchart" {
		t.Errorf("fix = %+v", f.Fix)
	}
}

func TestTocFlowchartVocab_NonTocTitle_NoFire(t *testing.T) {
	title := "Our Delivery Process"
	in := PresentationInput{Slides: []SlideInput{{
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		Pattern: &PatternInput{Name: "process-flow", Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B"},{"label":"C"}]}`)},
	}}}
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeTocFlowchartVocab) != nil {
		t.Error("TOC_FLOWCHART_VOCAB should not fire for a non-agenda title")
	}
}

func TestTocFlowchartVocab_AgendaWithAgendaPattern_NoFire(t *testing.T) {
	agenda := "Table of Contents"
	in := PresentationInput{Slides: []SlideInput{{
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &agenda}},
		Pattern: &PatternInput{Name: "agenda", Values: json.RawMessage(`{"items":[{"title":"A"},{"title":"B"}]}`)},
	}}}
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeTocFlowchartVocab) != nil {
		t.Error("TOC_FLOWCHART_VOCAB should not fire when the agenda uses the agenda pattern")
	}
}

// --- MATRIX_AXIS_IMBALANCE -------------------------------------------------

func TestMatrixAxisImbalance_RotatedSpanningBand_Fires(t *testing.T) {
	grid := &ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{{
				RowSpan: 2,
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Rotation: 270,
					Text:     json.RawMessage(`{"content":"Market Growth"}`),
				},
			}},
		}},
	}
	in := PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}
	f := findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeMatrixAxisImbalance)
	if f == nil {
		t.Fatal("expected MATRIX_AXIS_IMBALANCE for a rotated row-spanning text band")
	}
	if patterns.FindingClass(f.Code) != patterns.FindingClassRendering {
		t.Errorf("class = %q, want rendering", patterns.FindingClass(f.Code))
	}
}

// The post-fix matrix-2x2 geometry (rotation 0, vert270 text) must not fire.
func TestMatrixAxisImbalance_UnrotatedBand_NoFire(t *testing.T) {
	grid := &ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{{
				RowSpan: 2,
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Text:     json.RawMessage(`{"content":"Market Growth","vert":"vert270"}`),
				},
			}},
		}},
	}
	in := PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeMatrixAxisImbalance) != nil {
		t.Error("MATRIX_AXIS_IMBALANCE should not fire for an unrotated vert270 band")
	}
}

// A rotated single (non-spanning) cell is not an axis band — no finding.
func TestMatrixAxisImbalance_RotatedNonSpanning_NoFire(t *testing.T) {
	grid := &ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Rotation: 90,
					Text:     json.RawMessage(`{"content":"x"}`),
				},
			}},
		}},
	}
	in := PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeMatrixAxisImbalance) != nil {
		t.Error("MATRIX_AXIS_IMBALANCE should not fire for a rotated non-spanning cell")
	}
}

// --- Pipeline wiring -------------------------------------------------------

func TestPatternChoiceSmells_SurfaceViaCollectFitFindings(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Draft"},{"label":"Approved?","type":"decision"},{"label":"Ship"}]}`),
	})
	if findCode(collectFitFindings(&in, nil, 9144000, 6858000, nil), patterns.ErrCodeFlowDiamondNoContent) == nil {
		t.Fatal("FLOW_DIAMOND_NO_CONTENT should surface through collectFitFindings")
	}
}

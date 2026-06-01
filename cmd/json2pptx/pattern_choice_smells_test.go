package main

import (
	"encoding/json"
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

// --- OVERTALL_FLOW_LANE ----------------------------------------------------

// A 7-step process-flow with short labels is over-tall but outside the
// SPARSE_SINGLE_ROW_FLOW item range (3–6), so OVERTALL covers it.
func TestOvertallFlowLane_SevenSteps_Fires(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B"},{"label":"C"},{"label":"D"},{"label":"E"},{"label":"F"},{"label":"G"}]}`),
	})
	fs := collectPatternChoiceFindings(&in)
	f := findCode(fs, patterns.ErrCodeOvertallFlowLane)
	if f == nil {
		t.Fatal("expected OVERTALL_FLOW_LANE for a 7-step short-label process-flow")
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "swap_pattern" || f.Fix.Params["reason"] != "overtall_flow_lane" {
		t.Errorf("fix = %+v", f.Fix)
	}
	// Must NOT also emit SPARSE — the two are complementary.
	if findSparseFlow(fs) != nil {
		t.Error("SPARSE_SINGLE_ROW_FLOW should not fire alongside OVERTALL_FLOW_LANE")
	}
}

// A max_height_pct cap that is still too tall (60%) trips OVERTALL even with
// only 4 steps (where SPARSE is exempt because a cap is present).
func TestOvertallFlowLane_CapStillTooTall_Fires(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:         "process-flow",
		MaxHeightPct: 60,
		Values:       json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Scale"}]}`),
	})
	fs := collectPatternChoiceFindings(&in)
	if findCode(fs, patterns.ErrCodeOvertallFlowLane) == nil {
		t.Fatal("expected OVERTALL_FLOW_LANE for a capped-but-tall (60%) flow")
	}
}

// A reasonable cap (~35%) is the recommended remedy and must not fire.
func TestOvertallFlowLane_ReasonableCap_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:         "process-flow",
		MaxHeightPct: 35,
		Values:       json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Scale"},{"label":"Review"},{"label":"Iterate"},{"label":"Done"}]}`),
	})
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeOvertallFlowLane) != nil {
		t.Error("OVERTALL_FLOW_LANE should not fire when max_height_pct caps the lane to 35%")
	}
}

// An uncapped 3–6 step flow is SPARSE's territory; OVERTALL must defer.
func TestOvertallFlowLane_DefersToSparse(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}`),
	})
	fs := collectPatternChoiceFindings(&in)
	if findCode(fs, patterns.ErrCodeOvertallFlowLane) != nil {
		t.Error("OVERTALL_FLOW_LANE should defer to SPARSE_SINGLE_ROW_FLOW for an uncapped 3–6 step flow")
	}
}

// Long labels carry enough text to justify the height — no over-tall finding.
func TestOvertallFlowLane_LongLabels_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name: "process-flow",
		Values: json.RawMessage(`{"steps":[
			{"label":"Gather detailed requirements from every stakeholder team"},
			{"label":"Design the system architecture and review with security"},
			{"label":"Implement the core services and integration test suite"},
			{"label":"Roll out progressively to production with monitoring"},
			{"label":"Measure adoption and iterate on the rough edges found"},
			{"label":"Hand off to the operations team with full runbooks"},
			{"label":"Close out the project and capture the lessons learned"}
		]}`),
	})
	if findCode(collectPatternChoiceFindings(&in), patterns.ErrCodeOvertallFlowLane) != nil {
		t.Error("OVERTALL_FLOW_LANE should not fire for a long-label flow")
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
		Values: json.RawMessage(`{"steps":[{"label":"A"},{"label":"B"},{"label":"C"},{"label":"D"},{"label":"E"},{"label":"F"},{"label":"G"}]}`),
	})
	if findCode(collectFitFindings(&in, nil, 9144000, 6858000, nil), patterns.ErrCodeOvertallFlowLane) == nil {
		t.Fatal("OVERTALL_FLOW_LANE should surface through collectFitFindings")
	}
}

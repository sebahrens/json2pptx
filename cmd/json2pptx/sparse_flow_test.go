package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// findSparseFlow returns the first SPARSE_SINGLE_ROW_FLOW finding in fs, or nil.
func findSparseFlow(fs []patterns.FitFinding) *patterns.FitFinding {
	for i := range fs {
		if fs[i].Code == patterns.ErrCodeSparseSingleRowFlow {
			return &fs[i]
		}
	}
	return nil
}

func patternSlide(p *PatternInput) PresentationInput {
	return PresentationInput{Slides: []SlideInput{{Pattern: p}}}
}

// --- Positive cases -------------------------------------------------------

func TestSparseFlow_ProcessFlowShortLabels_Fires(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Review"}]}`),
	})
	f := findSparseFlow(collectSparseSingleRowFlowFindings(&in))
	if f == nil {
		t.Fatal("expected SPARSE_SINGLE_ROW_FLOW for a short-label process-flow with no bounds")
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "swap_pattern" {
		t.Errorf("fix kind = %+v, want swap_pattern", f.Fix)
	}
	if got := f.Path; got != "/slides/0/pattern" {
		t.Errorf("path = %q, want /slides/0/pattern", got)
	}
}

func TestSparseFlow_TimelineDotsNoBody_Fires(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "timeline-horizontal",
		Values: json.RawMessage(`[{"label":"Q1","date":"2025"},{"label":"Q2","date":"2025"},{"label":"Q3","date":"2025"}]`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) == nil {
		t.Fatal("expected SPARSE_SINGLE_ROW_FLOW for a label+date-only dots timeline")
	}
}

// --- Negative cases -------------------------------------------------------

func TestSparseFlow_MaxHeightCapped_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:         "process-flow",
		MaxHeightPct: 35,
		Values:       json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("max_height_pct cap should suppress SPARSE_SINGLE_ROW_FLOW")
	}
}

func TestSparseFlow_ExplicitBounds_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Bounds: &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 40},
		Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("explicit bounds should suppress SPARSE_SINGLE_ROW_FLOW")
	}
}

func TestSparseFlow_TimelineChevronStyle_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:      "timeline-horizontal",
		Overrides: json.RawMessage(`{"style":"chevron"}`),
		Values:    json.RawMessage(`[{"label":"Q1","date":"2025"},{"label":"Q2","date":"2025"},{"label":"Q3","date":"2025"}]`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("chevron timeline is multi-row; should not fire SPARSE_SINGLE_ROW_FLOW")
	}
}

func TestSparseFlow_TimelineGanttStyle_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:      "timeline-horizontal",
		Overrides: json.RawMessage(`{"style":"gantt"}`),
		Values:    json.RawMessage(`[{"label":"Q1","date":"2025","end_date":"2025"},{"label":"Q2","date":"2025","end_date":"2025"},{"label":"Q3","date":"2025","end_date":"2025"}]`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("gantt timeline is one row per stop; should not fire SPARSE_SINGLE_ROW_FLOW")
	}
}

func TestSparseFlow_DenseLabels_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name: "process-flow",
		Values: json.RawMessage(`{"steps":[` +
			`{"label":"Collect detailed customer requirements and constraints"},` +
			`{"label":"Design the target reference architecture end to end"},` +
			`{"label":"Build, integrate and validate every subsystem fully"}` +
			`]}`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("a process-flow with long descriptive labels is dense; should not fire")
	}
}

func TestSparseFlow_TimelineWithBodies_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name: "timeline-horizontal",
		Values: json.RawMessage(`[` +
			`{"label":"Q1","date":"2025","body":"Discovery and planning across all workstreams"},` +
			`{"label":"Q2","date":"2025","body":"Implementation of the core platform capabilities"},` +
			`{"label":"Q3","date":"2025","body":"Rollout, training and change management activities"}` +
			`]`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("a dots timeline carrying bodies is dense; should not fire")
	}
}

func TestSparseFlow_TooManyItems_NoFire(t *testing.T) {
	// 7 short stops: enough columns that each cell is narrow — outside the guard range.
	in := patternSlide(&PatternInput{
		Name:   "timeline-horizontal",
		Values: json.RawMessage(`[{"label":"Q1"},{"label":"Q2"},{"label":"Q3"},{"label":"Q4"},{"label":"Q5"},{"label":"Q6"},{"label":"Q7"}]`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("7-stop timeline is outside the sparse-flow item range; should not fire")
	}
}

func TestSparseFlow_NonFlowPattern_NoFire(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "kpi-3up",
		Values: json.RawMessage(`{"kpis":[{"value":"1","label":"a"},{"value":"2","label":"b"},{"value":"3","label":"c"}]}`),
	})
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("kpi-3up is not a single-row flow; should not fire")
	}
}

func TestSparseFlow_ComposeSegment_NoFire(t *testing.T) {
	// A process-flow embedded in a compose envelope (second zone absorbs the
	// height) carries slide.Pattern == nil, so the guard never sees it.
	in := PresentationInput{Slides: []SlideInput{{
		Compose: &ComposeInput{
			Direction: "vertical",
			Segments: []SegmentInput{
				{Pattern: PatternInput{
					Name:   "process-flow",
					Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"}]}`),
				}},
				{Pattern: PatternInput{
					Name:   "pull-quote",
					Values: json.RawMessage(`{"quote":"It works.","attribution":"A. User"}`),
				}},
			},
		},
	}}}
	if findSparseFlow(collectSparseSingleRowFlowFindings(&in)) != nil {
		t.Error("compose-embedded process-flow should not fire SPARSE_SINGLE_ROW_FLOW")
	}
}

// TestSparseFlow_SurfacesViaCollectFitFindings proves the guard is wired into
// the comprehensive fit-finding pipeline (the source for MCP fit_report,
// preview, plan, and the structured validate preflight), not just the
// standalone sub-collector.
func TestSparseFlow_SurfacesViaCollectFitFindings(t *testing.T) {
	in := patternSlide(&PatternInput{
		Name:   "process-flow",
		Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Ship"},{"label":"Review"}]}`),
	})
	findings := collectFitFindings(&in, nil, 9144000, 6858000, nil)
	if findSparseFlow(findings) == nil {
		t.Fatal("SPARSE_SINGLE_ROW_FLOW should surface through collectFitFindings")
	}
}

func TestSparseFlow_ConstructorFieldsWellFormed(t *testing.T) {
	f := patterns.SparseSingleRowFlow("process-flow", "/slides/2/pattern", 2, 4, 6.5)
	if f.Code != patterns.ErrCodeSparseSingleRowFlow {
		t.Errorf("code = %q", f.Code)
	}
	if f.Pattern != "process-flow" {
		t.Errorf("pattern = %q", f.Pattern)
	}
	if f.Fix == nil || f.Fix.Params["reason"] != "single_row_sparse" {
		t.Errorf("fix params = %+v", f.Fix)
	}
	if _, ok := f.Fix.Params["suggested"].([]any); !ok {
		t.Errorf("fix.params.suggested should be a list, got %T", f.Fix.Params["suggested"])
	}
}

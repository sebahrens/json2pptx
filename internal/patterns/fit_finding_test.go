package patterns

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestActionRank(t *testing.T) {
	tests := []struct {
		action string
		want   int
	}{
		{"info", 0},
		{"review", 1},
		{"shrink_or_split", 2},
		{"refuse", 3},
		{"unknown", -1},
		{"", -1},
	}
	for _, tt := range tests {
		got := ActionRank(tt.action)
		if got != tt.want {
			t.Errorf("ActionRank(%q) = %d, want %d", tt.action, got, tt.want)
		}
	}
}

func TestSortCanonical(t *testing.T) {
	stubSlideIndex := func(path string) int {
		// Mirror slidepath.SlideIndex's contract: "/slides/N/..." → N, else -1.
		if len(path) > 8 && path[:8] == "/slides/" {
			return int(path[8] - '0')
		}
		return -1
	}

	t.Run("severity descending, then slide ascending, then code ascending", func(t *testing.T) {
		findings := []FitFinding{
			{ValidationError: ValidationError{Path: "/slides/0", Code: "zzz"}, Action: "info"},
			{ValidationError: ValidationError{Path: "/slides/2", Code: "alpha"}, Action: "refuse"},
			{ValidationError: ValidationError{Path: "/slides/5", Code: "abc"}, Action: "review"},
			{ValidationError: ValidationError{Path: "/slides/0", Code: "beta"}, Action: "refuse"},
			{ValidationError: ValidationError{Path: "/slides/2", Code: "alpha"}, Action: "shrink_or_split"},
		}

		SortCanonical(findings, stubSlideIndex)

		wantOrder := []struct {
			action string
			path   string
			code   string
		}{
			// refuse (rank 3) — sorted by slide asc, then code asc.
			{"refuse", "/slides/0", "beta"},
			{"refuse", "/slides/2", "alpha"},
			// shrink_or_split (rank 2).
			{"shrink_or_split", "/slides/2", "alpha"},
			// review (rank 1).
			{"review", "/slides/5", "abc"},
			// info (rank 0).
			{"info", "/slides/0", "zzz"},
		}
		if len(findings) != len(wantOrder) {
			t.Fatalf("findings length changed: got %d, want %d", len(findings), len(wantOrder))
		}
		for i, want := range wantOrder {
			got := findings[i]
			if got.Action != want.action || got.Path != want.path || got.Code != want.code {
				t.Errorf("findings[%d] = {action:%q path:%q code:%q}, want {%q %q %q}",
					i, got.Action, got.Path, got.Code, want.action, want.path, want.code)
			}
		}
	})

	t.Run("deck-level paths (slide_index -1) sort before slide 0 at same severity", func(t *testing.T) {
		findings := []FitFinding{
			{ValidationError: ValidationError{Path: "/slides/0", Code: "x"}, Action: "info"},
			{ValidationError: ValidationError{Path: "/deck/duplicate_title", Code: "x"}, Action: "info"},
		}

		SortCanonical(findings, stubSlideIndex)

		if findings[0].Path != "/deck/duplicate_title" {
			t.Errorf("expected deck-level path first, got %q", findings[0].Path)
		}
	})

	t.Run("empty and single-element slices are no-ops", func(t *testing.T) {
		SortCanonical(nil, stubSlideIndex)
		single := []FitFinding{{ValidationError: ValidationError{Path: "/slides/0", Code: "x"}, Action: "info"}}
		SortCanonical(single, stubSlideIndex)
		if single[0].Code != "x" {
			t.Errorf("single-element slice was mutated: %+v", single)
		}
	})
}

func TestContentDropped(t *testing.T) {
	f := ContentDropped("/slides/3", "slide 4", "skipped in partial mode: invalid layout_id")

	if f.Code != ErrCodeContentDropped {
		t.Errorf("Code = %q, want %q", f.Code, ErrCodeContentDropped)
	}
	if f.Action != "review" {
		t.Errorf("Action = %q, want review (content-drop diagnostics must never block)", f.Action)
	}
	if f.Path != "/slides/3" {
		t.Errorf("Path = %q, want /slides/3", f.Path)
	}
	if f.Fix == nil || f.Fix.Kind != "review" {
		t.Fatalf("Fix = %+v, want kind=review (no deterministic auto-fix)", f.Fix)
	}
	if got := f.Fix.Params["locator"]; got != "slide 4" {
		t.Errorf("Fix.Params[locator] = %v, want \"slide 4\"", got)
	}
	if got, _ := f.Fix.Params["reason"].(string); got == "" {
		t.Error("Fix.Params[reason] should carry the drop reason")
	}

	// The code must be registered so AllFitFindingCodes + the finding-meta
	// drift guarantee cover it (errors.Is + describe_finding rely on this).
	if !errors.Is(&f.ValidationError, ErrContentDropped) {
		t.Error("ContentDropped finding does not unwrap to ErrContentDropped sentinel")
	}
	if _, ok := GetFindingMeta(ErrCodeContentDropped); !ok {
		t.Error("CONTENT_DROPPED missing from findingMetaRegistry")
	}
}

func TestActionRankOrdering(t *testing.T) {
	// Verify refuse > shrink_or_split > review > info.
	ordered := []string{"info", "review", "shrink_or_split", "refuse"}
	for i := 1; i < len(ordered); i++ {
		if ActionRank(ordered[i]) <= ActionRank(ordered[i-1]) {
			t.Errorf("ActionRank(%q) should be > ActionRank(%q)", ordered[i], ordered[i-1])
		}
	}
}

func TestFitFindingJSON(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Pattern: "table",
			Path:    "slides[0].content.rows[3]",
			Code:    ErrCodeFitOverflow,
			Message: "row 3 overflows cell height",
			Fix:     &FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 3}},
		},
		Action:        "shrink_or_split",
		Measured:      &Extent{WidthEMU: 914400, HeightEMU: 1828800},
		Allowed:       &Extent{WidthEMU: 914400, HeightEMU: 1371600},
		OverflowRatio: 1.33,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Embedded ValidationError fields must appear at top level (not nested).
	for _, key := range []string{"pattern", "path", "code", "message", "fix"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected top-level key %q from embedded ValidationError", key)
		}
	}

	// FitFinding-specific fields must also be present.
	for _, key := range []string{"action", "measured", "allowed", "overflow_ratio"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected top-level key %q from FitFinding", key)
		}
	}

	// Verify no nested "ValidationError" wrapper.
	if _, ok := m["ValidationError"]; ok {
		t.Error("embedded ValidationError should not appear as a named key")
	}
}

func TestRepairToolCall(t *testing.T) {
	tests := []struct {
		name     string
		fix      *FixSuggestion
		slideIdx int
		wantNil  bool
		wantTool string
		wantKind string
	}{
		{
			name:     "reduce_text",
			fix:      &FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_items": 5}},
			slideIdx: 2,
			wantTool: "repair_slide",
			wantKind: "reduce_text",
		},
		{
			name:     "split_at_row",
			fix:      &FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 3}},
			slideIdx: 0,
			wantTool: "repair_slide",
			wantKind: "split_at_row",
		},
		{
			name:    "nil fix",
			fix:     nil,
			wantNil: true,
		},
		{
			name:    "unknown kind",
			fix:     &FixSuggestion{Kind: "adopt_pattern"},
			wantNil: true,
		},
		{
			name:     "no params",
			fix:      &FixSuggestion{Kind: "shorten_title"},
			slideIdx: 1,
			wantTool: "repair_slide",
			wantKind: "shorten_title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RepairToolCall(tt.slideIdx, tt.fix)
			if tt.wantNil {
				if got != nil {
					t.Errorf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want non-nil")
			}
			if got.Tool != tt.wantTool {
				t.Errorf("Tool = %q, want %q", got.Tool, tt.wantTool)
			}
			fixes, ok := got.ArgsTemplate["fixes"].([]any)
			if !ok || len(fixes) != 1 {
				t.Fatalf("ArgsTemplate[fixes] = %v, want 1-element slice", got.ArgsTemplate["fixes"])
			}
			fixMap, ok := fixes[0].(map[string]any)
			if !ok {
				t.Fatalf("fixes[0] is %T, want map[string]any", fixes[0])
			}
			if fixMap["kind"] != tt.wantKind {
				t.Errorf("fixes[0].kind = %v, want %q", fixMap["kind"], tt.wantKind)
			}
			if idx, ok := got.ArgsTemplate["slide_index"].(int); !ok || idx != tt.slideIdx {
				t.Errorf("slide_index = %v, want %d", got.ArgsTemplate["slide_index"], tt.slideIdx)
			}
		})
	}
}

func TestAttachNextToolCalls(t *testing.T) {
	stubSlideIndex := func(path string) int {
		// Simple stub: extract number after "slides/"
		if len(path) > 8 && path[:8] == "/slides/" {
			return int(path[8] - '0')
		}
		return 0
	}

	findings := []FitFinding{
		{
			ValidationError: ValidationError{
				Path: "/slides/2/content/body",
				Code: ErrCodeFitOverflow,
				Fix:  &FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_items": 5}},
			},
			Action: "refuse",
		},
		{
			ValidationError: ValidationError{
				Path: "/slides/1/shape_grid",
				Code: ErrCodeSparseLayout,
				Fix:  &FixSuggestion{Kind: "adopt_pattern", Params: map[string]any{"filled_slots": 3}},
			},
			Action: "review",
		},
		{
			ValidationError: ValidationError{
				Path: "/slides/0/content/body",
				Code: ErrCodeFitOverflow,
			},
			Action: "info", // should be skipped
		},
		{
			ValidationError: ValidationError{
				Path: "/slides/3/table",
				Code: ErrCodeDensityExceeded,
				Fix:  &FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 4}},
			},
			Action: "shrink_or_split",
		},
		// Non-text grid finding: grid_diagram_narrow with reshape_grid fix.
		{
			ValidationError: ValidationError{
				Path: "/slides/4/shape_grid/rows/0/cells/1/diagram",
				Code: ErrCodeGridDiagramNarrow,
				Fix:  &FixSuggestion{Kind: "reshape_grid", Params: map[string]any{"columns": 1}},
			},
			Action: "review",
		},
		// Render-time finding: diagram_clamped with swap_layout fix.
		{
			ValidationError: ValidationError{
				Path: "/slides/5/content/body",
				Code: ErrCodeDiagramClamped,
				Fix:  &FixSuggestion{Kind: "swap_layout", Params: map[string]any{"dimension": "width"}},
			},
			Action: "review",
		},
		// Render-time finding: diagram_render_failed with review fix (no auto-fix).
		{
			ValidationError: ValidationError{
				Path: "/slides/6/content/body",
				Code: ErrCodeDiagramRenderFailed,
				Fix:  &FixSuggestion{Kind: "review", Params: map[string]any{"reason": "SVG parse error"}},
			},
			Action: "review",
		},
	}

	AttachNextToolCalls(findings, stubSlideIndex)

	// Finding 0: refuse + reduce_text → repair_slide
	if findings[0].NextToolCall == nil {
		t.Fatal("findings[0].NextToolCall is nil")
	}
	if findings[0].NextToolCall.Tool != "repair_slide" {
		t.Errorf("findings[0] tool = %q, want repair_slide", findings[0].NextToolCall.Tool)
	}

	// Finding 1: review + adopt_pattern → recommend_pattern
	if findings[1].NextToolCall == nil {
		t.Fatal("findings[1].NextToolCall is nil")
	}
	if findings[1].NextToolCall.Tool != "recommend_pattern" {
		t.Errorf("findings[1] tool = %q, want recommend_pattern", findings[1].NextToolCall.Tool)
	}

	// Finding 2: info → skipped
	if findings[2].NextToolCall != nil {
		t.Errorf("findings[2].NextToolCall should be nil for action=info, got %+v", findings[2].NextToolCall)
	}

	// Finding 3: shrink_or_split + split_at_row → repair_slide
	if findings[3].NextToolCall == nil {
		t.Fatal("findings[3].NextToolCall is nil")
	}
	if findings[3].NextToolCall.Tool != "repair_slide" {
		t.Errorf("findings[3] tool = %q, want repair_slide", findings[3].NextToolCall.Tool)
	}

	// Finding 4: grid_diagram_narrow + reshape_grid → repair_slide
	if findings[4].NextToolCall == nil {
		t.Fatal("findings[4].NextToolCall is nil (grid_diagram_narrow should map to repair_slide)")
	}
	if findings[4].NextToolCall.Tool != "repair_slide" {
		t.Errorf("findings[4] tool = %q, want repair_slide", findings[4].NextToolCall.Tool)
	}

	// Finding 5: diagram_clamped + swap_layout → repair_slide
	if findings[5].NextToolCall == nil {
		t.Fatal("findings[5].NextToolCall is nil (diagram_clamped should map to repair_slide)")
	}
	if findings[5].NextToolCall.Tool != "repair_slide" {
		t.Errorf("findings[5] tool = %q, want repair_slide", findings[5].NextToolCall.Tool)
	}

	// Finding 6: diagram_render_failed + review fix → no NextToolCall
	// "review" is not a repair kind, so RepairToolCall returns nil.
	if findings[6].NextToolCall != nil {
		t.Errorf("findings[6].NextToolCall should be nil for review-only fix, got %+v", findings[6].NextToolCall)
	}
}

func TestNextToolCallJSON(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Path:    "/slides/0/content/body",
			Code:    ErrCodeFitOverflow,
			Message: "text overflows",
			Fix:     &FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_items": 5}},
		},
		Action: "refuse",
		NextToolCall: &ToolCallSuggestion{
			Tool: "repair_slide",
			ArgsTemplate: map[string]any{
				"slide_index": 0,
				"fixes":       []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 5}}},
			},
		},
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	ntc, ok := m["next_tool_call"].(map[string]any)
	if !ok {
		t.Fatal("next_tool_call missing or not an object")
	}
	if ntc["tool"] != "repair_slide" {
		t.Errorf("tool = %v, want repair_slide", ntc["tool"])
	}
	args, ok := ntc["args_template"].(map[string]any)
	if !ok {
		t.Fatal("args_template missing or not an object")
	}
	if args["slide_index"] != float64(0) {
		t.Errorf("slide_index = %v, want 0", args["slide_index"])
	}
}

func TestNextToolCallOmittedWhenNil(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Path:    "/slides/0/content/body",
			Code:    ErrCodeFitOverflow,
			Message: "text overflows",
		},
		Action: "info",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := m["next_tool_call"]; ok {
		t.Error("nil NextToolCall should be omitted from JSON")
	}
}

func TestFitFindingJSONOmitsNilExtents(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Pattern: "shape_grid",
			Path:    "slides[1]",
			Code:    ErrCodeDensityExceeded,
			Message: "too dense",
		},
		Action: "review",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := m["measured"]; ok {
		t.Error("nil Measured should be omitted from JSON")
	}
	if _, ok := m["allowed"]; ok {
		t.Error("nil Allowed should be omitted from JSON")
	}
	if _, ok := m["overflow_ratio"]; ok {
		t.Error("zero OverflowRatio should be omitted from JSON")
	}
}

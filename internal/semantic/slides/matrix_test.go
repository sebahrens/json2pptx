package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func quadrant(header, body string) map[string]any {
	m := map[string]any{"header": header}
	if body != "" {
		m["body"] = body
	}
	return m
}

func matrixBody(overlay map[string]any) map[string]any {
	body := map[string]any{
		"title":  "Where to spend the next two quarters",
		"x_axis": "Effort to deliver",
		"y_axis": "Impact on settlement risk",
		"quadrants": []any{
			quadrant("Do first", "Reconciliation alerts."),
			quadrant("Plan properly", "Platform migration."),
			quadrant("Defer", "Reporting refresh."),
			quadrant("Fill the gaps", "Runbook tidy-up."),
		},
	}
	for k, v := range overlay {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return body
}

// Two axes and four quadrants is one of the shapes every strategy deck reaches
// for, and the spec had no kind for it (go-slide-creator-ykjh).
func TestCompileMatrixUsesTheQuadrantsWhenItFits(t *testing.T) {
	body := matrixBody(map[string]any{"x_low": "Low effort", "y_high": "High impact"})
	if got := MatrixPattern(body); got != "matrix-2x2" {
		t.Fatalf("MatrixPattern = %q, want matrix-2x2", got)
	}
	slide, _, err := CompileMatrix(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "matrix-2x2" {
		t.Fatalf("pattern = %+v, want matrix-2x2", slide.Pattern)
	}
	var values matrixValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatalf("values %s: %v", slide.Pattern.Values, err)
	}
	// The list is read clockwise from the top left, which is how a 2x2 is read.
	if values.TopLeft.Header != "Do first" || values.TopRight.Header != "Plan properly" ||
		values.BottomRight.Header != "Defer" || values.BottomLeft.Header != "Fill the gaps" {
		t.Errorf("quadrants landed in the wrong corners: %+v", values)
	}
	if values.XAxisLabel != "Effort to deliver" || values.YAxisLabel != "Impact on settlement risk" {
		t.Errorf("axes = %q / %q", values.XAxisLabel, values.YAxisLabel)
	}
	if values.XLow != "Low effort" || values.YHigh != "High impact" {
		t.Errorf("axis ends = %+v", values)
	}
}

func TestMatrixAxisEndBudgetRejectsTextThatShrinksBelowReadableSize(t *testing.T) {
	fit := matrixBody(map[string]any{"y_high": "Underserved"}) // 11 chars
	if reason := MatrixOverBudget(fit); reason != "" {
		t.Fatalf("11-character end label rejected: %s", reason)
	}
	tooLong := matrixBody(map[string]any{"y_high": "Fast, under-served"})
	if reason := MatrixOverBudget(tooLong); !strings.Contains(reason, "an axis end holds 11") {
		t.Errorf("18-character end label should trigger fallback, got %q", reason)
	}
}

// Quadrants can be written by position instead of as a list.
func TestMatrixAcceptsNamedPositions(t *testing.T) {
	body := map[string]any{
		"x_axis": "Effort", "y_axis": "Impact",
		"top_left":     quadrant("Do first", ""),
		"top_right":    quadrant("Plan properly", ""),
		"bottom_left":  quadrant("Fill the gaps", ""),
		"bottom_right": quadrant("Defer", ""),
	}
	if got := MatrixPattern(body); got != "matrix-2x2" {
		t.Fatalf("MatrixPattern = %q, want matrix-2x2", got)
	}
	quads := MatrixQuadrants(body)
	if quads["bottom_left"].Header != "Fill the gaps" {
		t.Errorf("bottom_left = %+v", quads["bottom_left"])
	}
}

// A quadrant is also writable as a bare string or "Header: body".
func TestMatrixQuadrantStringForms(t *testing.T) {
	body := map[string]any{
		"x_axis": "Effort", "y_axis": "Impact",
		"quadrants": []any{"Do first: act this quarter", "Plan properly", "Defer", "Fill the gaps"},
	}
	quads := MatrixQuadrants(body)
	if quads["top_left"].Header != "Do first" || quads["top_left"].Body != "act this quarter" {
		t.Errorf("top_left = %+v", quads["top_left"])
	}
	if quads["top_right"].Header != "Plan properly" || quads["top_right"].Body != "" {
		t.Errorf("top_right = %+v", quads["top_right"])
	}
}

// Short of four headed quadrants and two named axes it degrades, and the
// finding says what broke.
func TestMatrixDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "three quadrants",
			body:   matrixBody(map[string]any{"quadrants": []any{quadrant("A", ""), quadrant("B", ""), quadrant("C", "")}}),
			reason: "has 3 usable quadrants",
		},
		{
			name:   "a headerless quadrant",
			body:   matrixBody(map[string]any{"quadrants": []any{quadrant("A", ""), quadrant("B", ""), quadrant("C", ""), map[string]any{"body": "orphan"}}}),
			reason: "has 3 usable quadrants",
		},
		{
			name:   "an unnamed axis",
			body:   matrixBody(map[string]any{"y_axis": nil}),
			reason: "missing an axis label",
		},
		{
			name:   "an axis written as a sentence",
			body:   matrixBody(map[string]any{"x_axis": strings.Repeat("a", 70)}),
			reason: "x axis label is 70 characters",
		},
		{
			name:   "an over-long axis end",
			body:   matrixBody(map[string]any{"x_high": strings.Repeat("a", 25)}),
			reason: "x_high is 25 characters",
		},
		{
			name: "an over-long quadrant body",
			body: matrixBody(map[string]any{"quadrants": []any{
				quadrant("A", strings.Repeat("a", 210)), quadrant("B", ""), quadrant("C", ""), quadrant("D", ""),
			}}),
			reason: "top left body is 210 characters",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatrixPattern(c.body); got != "" {
				t.Errorf("MatrixPattern = %q, want the bullet fallback", got)
			}
			if over := MatrixOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("MatrixOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileMatrix(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Error("the fallback lost the quadrants entirely")
			}
		})
	}
}

// A quadrant stripped of where it sits on the axes has lost the point of the
// slide, so the fallback names both and every position.
func TestMatrixFallbackKeepsThePositionsAndAxes(t *testing.T) {
	body := matrixBody(map[string]any{
		"x_low": "Low effort", "x_high": "High effort",
		"y_low": "Low impact", "y_high": "High impact",
		"quadrants": []any{quadrant("A", ""), quadrant("B", ""), quadrant("C", "")},
	})
	slide, _, err := CompileMatrix(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{
		"Impact on settlement risk (Low impact to High impact)",
		"Effort to deliver (Low effort to High effort)",
		"Top left — A", "Top right — B", "Bottom right — C",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the fallback dropped %q: %s", want, encoded)
		}
	}
}

// An empty payload is the required-field gate's business, not the budget's.
func TestMatrixWithNoQuadrantsReportsNothing(t *testing.T) {
	if over := MatrixOverBudget(map[string]any{"quadrants": []any{}}); over != "" {
		t.Errorf("an empty matrix reported %q", over)
	}
	if n := UsableMatrixQuadrantCount(map[string]any{"title": "x"}); n != 0 {
		t.Errorf("UsableMatrixQuadrantCount = %d, want 0", n)
	}
}

// The spellings an author reaches for all resolve.
func TestMatrixAliases(t *testing.T) {
	for _, field := range []string{"quadrants", "cells", "boxes"} {
		body := map[string]any{field: []any{"A", "B", "C", "D"}}
		if n := len(MatrixQuadrants(body)); n != 4 {
			t.Errorf("%s: resolved %d quadrants, want 4", field, n)
		}
	}
	for _, key := range []string{"x_axis", "x_axis_label"} {
		if x, _ := matrixAxes(map[string]any{key: "Effort"}); x != "Effort" {
			t.Errorf("%s: x = %q", key, x)
		}
	}
	for _, key := range []string{"y_axis", "y_axis_label"} {
		if _, y := matrixAxes(map[string]any{key: "Impact"}); y != "Impact" {
			t.Errorf("%s: y = %q", key, y)
		}
	}
}

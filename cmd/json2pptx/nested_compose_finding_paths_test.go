package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// go-slide-creator-epch2: a sub-grid is generated as if it were the slide's
// grid, so a native diagram in a nested compose segment reported
// TEXT_EXCEEDS_SHAPE a second time at a top-level cell path validate never
// names. Generation also indexed cells by resolved column, and validate's
// diagram dry render looked bounds up by the cell's index in its row, so a
// segment after a multi-column one (every horizontal compose segment after
// the first spans its columns) was never dry-rendered. Validate and generation
// now report each generate-time grid finding at one path — the authored JSON
// pointer — at any nesting depth.

func nestedComposeKPISegment(pct int) map[string]any {
	return narrowRegionSegment(pct, "pattern", map[string]any{"name": "kpi-3up", "values": []any{
		map[string]any{"big": "41%", "small": "Share"}, map[string]any{"big": "$2.1B", "small": "Revenue"}, map[string]any{"big": "12", "small": "Markets"},
	}})
}

func nestedComposeSegment(pct int, direction string, segments ...any) map[string]any {
	return map[string]any{"size_pct": pct, "compose": map[string]any{"direction": direction, "segments": segments}}
}

func nestedComposeInput(t *testing.T, template string, slides []any) *PresentationInput {
	t.Helper()
	data, err := json.Marshal(map[string]any{"template": template, "slides": slides})
	if err != nil {
		t.Fatal(err)
	}
	var in PresentationInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	return &in
}

func findingPathsWithCode(fs []patterns.FitFinding, code string) map[string]string {
	out := map[string]string{}
	for _, f := range fs {
		if f.Code == code {
			out[f.Path] = f.Message
		}
	}
	return out
}

func TestNestedComposeNativeDiagramFindingPathsMatchValidate(t *testing.T) {
	narrowPyramid := func(pct int) map[string]any {
		return narrowRegionSegment(pct, "diagram", narrowRegionPyramid("Transformation", 3, false))
	}
	cases := []struct {
		name  string
		slide map[string]any
		want  string // validate's path on midnight-blue
		// authored is where a report places it: the authored segment.
		authored string
	}{
		{
			name: "first segment",
			slide: narrowRegionSlide("Nested first", nestedComposeSegment(50, "horizontal", narrowPyramid(45), narrowRegionHero(55)),
				narrowRegionHero(50)),
			want:     "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/grid/rows/0/cells/0/diagram",
			authored: "/slides/0/compose/segments/0/compose/segments/0/diagram",
		},
		{
			name: "after a multi-column segment",
			slide: narrowRegionSlide("Nested after KPIs", nestedComposeKPISegment(50),
				nestedComposeSegment(50, "horizontal", narrowPyramid(45), narrowRegionHero(55))),
			want:     "/slides/0/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/grid/rows/0/cells/0/diagram",
			authored: "/slides/0/compose/segments/1/compose/segments/0/diagram",
		},
		{
			name: "vertical inner compose",
			slide: narrowRegionSlide("Nested vertical", nestedComposeKPISegment(78),
				nestedComposeSegment(22, "vertical", narrowRegionHero(50), narrowPyramid(50))),
			want:     "/slides/0/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/grid/rows/0/cells/0/diagram",
			authored: "/slides/0/compose/segments/1/compose/segments/1/diagram",
		},
	}
	for _, tpl := range narrowRegionTemplates() {
		for _, tc := range cases {
			t.Run(tpl+"/"+tc.name, func(t *testing.T) {
				slides := []any{tc.slide}
				input := nestedComposeInput(t, tpl, slides)
				_, _, validate := gateFor(t, input)
				predicted := findingPathsWithCode(validate, patterns.ErrCodeTextExceedsShape)
				if _, ok := predicted[tc.want]; tpl == "midnight-blue" && !ok {
					t.Fatalf("validate did not predict the mid-word break at %s: %v", tc.want, predicted)
				}
				// The report addresses the authored envelope: the segment's
				// diagram, not the cell of the grid it merges into.
				authored := newAuthoredPaths(input)
				if got := authored.address(tc.want, "").Path; got != tc.authored {
					t.Errorf("%s is reported at %s, want %s", tc.want, got, tc.authored)
				}
				for path, msg := range predicted {
					delete(predicted, path)
					predicted[authored.address(path, "").Path] = msg
				}
				result, _ := generateNarrowRegionDeck(t, tpl, slides)
				reported := findingPathsWithCode(result.FitFindings, patterns.ErrCodeTextExceedsShape)
				for path, msg := range reported {
					if want, ok := predicted[path]; !ok || want != msg {
						t.Errorf("generate reports %s at %s (%q), which validate does not: %v", patterns.ErrCodeTextExceedsShape, path, msg, predicted)
					}
				}
				for path := range predicted {
					if _, ok := reported[path]; !ok {
						t.Errorf("generate report lacks the %s validate predicted at %s", patterns.ErrCodeTextExceedsShape, path)
					}
				}
			})
		}
	}
}

// A nested native diagram whose region is too small is refused with
// DIAGRAM_REGION_TOO_SMALL at the path validate predicted.
func TestNestedComposeRegionRefusalPathMatchesValidate(t *testing.T) {
	swot := narrowRegionSegment(15, "diagram", map[string]any{"type": "swot", "alt": "SWOT", "data": map[string]any{
		"strengths": []any{"Brand", "Scale"}, "weaknesses": []any{"Cost", "Legacy IT"},
		"opportunities": []any{"Asia"}, "threats": []any{"Entrants"},
	}})
	slides := []any{narrowRegionSlide("Nested refusal", nestedComposeKPISegment(70),
		nestedComposeSegment(30, "vertical", narrowRegionHero(85), swot))}
	const want = "/slides/0/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/grid/rows/0/cells/0/diagram"

	in := nestedComposeInput(t, "midnight-blue", slides)
	_, _, validate := gateFor(t, in)
	if _, ok := findingPathsWithCode(validate, patterns.ErrCodeDiagramRegionTooSmall)[want]; !ok {
		t.Fatalf("validate did not predict %s at %s: %+v", patterns.ErrCodeDiagramRegionTooSmall, want, validate)
	}

	in = nestedComposeInput(t, "midnight-blue", slides)
	applyDefaults(in)
	_, cleanup, err := RunPresentation(context.Background(), in, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off",
	})
	if cleanup != nil {
		defer cleanup()
	}
	var capacity *generator.NativeRegionCapacityError
	if !errors.As(err, &capacity) {
		t.Fatalf("generation must refuse the region, got err=%v", err)
	}
	if capacity.Finding.Path != want {
		t.Errorf("refusal path = %q, validate predicted %q", capacity.Finding.Path, want)
	}
}

// A grid table two sub-grids deep, after a column-spanning cell, refuses its
// dropped rows at the path validate predicted.
func TestNestedGridTableTruncationPathMatchesValidate(t *testing.T) {
	rows := make([]any, 9)
	for i := range rows {
		rows[i] = []any{"Segment", "€14m", "+4%"}
	}
	table := map[string]any{"table": map[string]any{"headers": []any{"Segment", "Revenue", "Growth"}, "rows": rows}}
	inner := map[string]any{"grid": map[string]any{"rows": []any{
		map[string]any{"height": 30, "cells": []any{table}},
		map[string]any{"cells": []any{map[string]any{"shape": map[string]any{"geometry": "rect", "text": "Note"}}}},
	}}}
	slides := []any{map[string]any{
		"layout_id": "content",
		"content":   []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Nested table"}},
		"shape_grid": map[string]any{"columns": 3, "rows": []any{map[string]any{"cells": []any{
			map[string]any{"col_span": 2, "shape": map[string]any{"geometry": "rect", "text": "Context"}},
			map[string]any{"grid": map[string]any{"rows": []any{map[string]any{"cells": []any{inner}}}}},
		}}}},
	}}
	const want = "/slides/0/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/grid/rows/0/cells/0/table"

	in := nestedComposeInput(t, "midnight-blue", slides)
	applyDefaults(in)
	geom := loadSchemaMaximaGeometry(t, "midnight-blue")
	predicted := findingPathsWithCode(collectFitFindings(in, geom.layouts, geom.width, geom.height, nil), patterns.ErrCodeTableRowsTruncated)
	if _, ok := predicted[want]; !ok {
		t.Fatalf("validate did not predict %s at %s: %v", patterns.ErrCodeTableRowsTruncated, want, predicted)
	}

	_, cleanup, err := RunPresentation(context.Background(), in, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	loss := generationRefusal(err)
	if loss == nil || loss.Code != patterns.ErrCodeTableRowsTruncated {
		t.Fatalf("generation must refuse the dropped rows, got err=%v", err)
	}
	if loss.Path != want {
		t.Errorf("refusal path = %q, validate predicted %q", loss.Path, want)
	}
}

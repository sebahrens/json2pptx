package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutReviewExhibitAfterSpanNamesAuthoredCell(t *testing.T) {
	input := geomSlides(t, `[{"shape_grid":{"columns":[1,1,1,1,1],"rows":[{"cells":[{"col_span":4,"shape":{"text":"Explanation"}},{"image":{"path":"proof.png"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeExhibitTooSmall)
	if len(fs) != 1 || fs[0].Path != "/slides/0/shape_grid/rows/0/cells/1" {
		t.Fatalf("sole exhibit must name authored index after span: %+v", fs)
	}
	assertFindingPointerReplaceable(t, input, fs[0].Path)
}
func TestLayoutReviewGeneratedPathsKeepAdvisoriesAtAuthoredContainers(t *testing.T) {
	input := geomSlides(t, `[{"pattern":{"name":"stat-hero","values":{"value":"32%","label":"Margin"}}},{"shape_grid":{"rows":[{"cells":[{"pattern":{"name":"stat-hero","values":{"value":"32%","label":"Margin"}}}]}]}}]`)
	codes := []string{patterns.ErrCodeExhibitTooSmall, patterns.ErrCodeBaselineMisalign, patterns.ErrCodePaneFillImbalance, patterns.ErrCodeTitleOrphan, patterns.ErrCodeDetachedLabel, patterns.ErrCodeTypeHierarchyFlat}
	for _, code := range codes {
		path := "/slides/0/pattern/rows/1/cells/4/shape/text"
		peer := "/slides/1/shape_grid/rows/0/cells/0/grid/rows/1/cells/4"
		fs := layoutReviewFindingsAtAuthoredPaths(input, []patterns.FitFinding{layoutReviewFinding(code, path, "review", "retain measured advisory", map[string]any{"cells": []string{path, peer}, "parent": peer})})
		if len(fs) != 1 || fs[0].Code != code || fs[0].Action != "review" || fs[0].Path != "/slides/0/pattern" {
			t.Fatalf("finding altered or not mapped: %+v", fs)
		}
		assertFindingPointerReplaceable(t, input, fs[0].Path)
		cells, ok := fs[0].Fix.Params["cells"].([]string)
		if !ok {
			t.Fatal("mapped peer cells missing")
		}
		for _, p := range cells {
			assertFindingPointerReplaceable(t, input, p)
		}
		parent, ok := fs[0].Fix.Params["parent"].(string)
		if !ok {
			t.Fatal("mapped parent missing")
		}
		assertFindingPointerReplaceable(t, input, parent)
	}
}

func TestTemplateQAExhibitPathsResolveOnReportedTemplates(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "examples", "template-qa-deck.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, template := range []string{"midnight-blue", "modern-template"} {
		t.Run(template, func(t *testing.T) {
			var input PresentationInput
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			input.Template = template
			layouts, w, h := schemaMaximaLayouts(t, template)
			fs := findingsByCode(collectGeometryFindings(&input, layouts, w, h, nil), patterns.ErrCodeExhibitTooSmall)
			found := false
			for _, f := range fs {
				assertFindingPointerReplaceable(t, &input, f.Path)
				if f.Path == "/slides/10/shape_grid/rows/1/cells/1" {
					found = true
				}
			}
			if !found {
				t.Fatalf("sole exhibit advisory lost or misattributed: %+v", fs)
			}
		})
	}
}

package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestExemplarGeometryHeadingAndBaselineRoles(t *testing.T) {
	checked := 0
	for _, pat := range patterns.Default().List() {
		ex, ok := pat.(patterns.Exemplar)
		if !ok {
			continue
		}
		values, err := json.Marshal(ex.ExemplarValues())
		if err != nil {
			t.Fatal(err)
		}
		ctx := measuredComposeContext()
		ctx.Theme = pStyleTheme()
		grid, _, err := expandPattern(&PatternInput{Name: pat.Name(), Values: values}, ctx, patterns.Default())
		if err != nil {
			t.Errorf("%s expansion: %v", pat.Name(), err)
			continue
		}
		input := &PresentationInput{Slides: []SlideInput{{LayoutID: "content", ShapeGrid: grid}}}
		checked++
		for _, f := range collectGeometryFindings(input, nil, ctx.SlideWidth, ctx.SlideHeight, nil) {
			if f.Code == patterns.ErrCodeBaselineMisalign || f.Code == patterns.ErrCodeTitleOrphan {
				t.Errorf("%s unexpected %s at %s: %s", pat.Name(), f.Code, f.Path, f.Message)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no exemplars checked")
	}
}

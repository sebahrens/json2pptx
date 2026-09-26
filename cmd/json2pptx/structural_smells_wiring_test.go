package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// The structural-smell detectors are part of the shared fit collection, so
// validate, dry-run and generate all report them on authored grids — and
// leave pattern expansions alone (go-slide-creator-s1uvj.13).
func TestCollectFitFindings_ReportsStructuralSmellsOnAuthoredGrids(t *testing.T) {
	r, err := template.OpenTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	p, err := template.BuildProfile(r)
	if err != nil {
		t.Fatal(err)
	}

	var input PresentationInput
	deck := `{"template":"midnight-blue","slides":[
	  {"layout_id":"blank","shape_grid":{"columns":4,"rows":[{"cells":[
	    {"shape":{"geometry":"rect","fill":"accent1","text":"A"}},
	    {"shape":{"geometry":"rect","fill":"accent2","text":"B"}},
	    {"shape":{"geometry":"rect","fill":"accent3","text":"C"}},
	    {"shape":{"geometry":"rect","fill":"#123456","text":"D"}}]}]}},
	  {"layout_id":"blank","shape_grid":{"columns":1,"row_gap":2,"rows":[
	    {"cells":[{"table":{"headers":["A"],"rows":[["1"]]}}]},
	    {"cells":[{"table":{"headers":["B"],"rows":[["2"]]}}]}]}}]}`
	if err := json.Unmarshal([]byte(deck), &input); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	for _, f := range collectFitFindings(&input, p.Layouts, p.SlideWidth, p.SlideHeight, nil) {
		switch f.Code {
		case patterns.ErrCodeAccentOverload, patterns.ErrCodeMixedFillScheme,
			patterns.ErrCodeStackedTables, patterns.ErrCodeDividerTooThin:
			got[f.Code] = f.Path
			if f.Action != "review" {
				t.Errorf("%s action = %q, want review", f.Code, f.Action)
			}
		}
	}
	for code, path := range map[string]string{
		patterns.ErrCodeAccentOverload:  "/slides/0/shape_grid",
		patterns.ErrCodeMixedFillScheme: "/slides/0/shape_grid",
		patterns.ErrCodeStackedTables:   "",
		patterns.ErrCodeDividerTooThin:  "",
	} {
		gotPath, ok := got[code]
		if !ok {
			t.Errorf("fit collection did not report %s; got %v", code, got)
			continue
		}
		if path != "" && gotPath != path {
			t.Errorf("%s path = %q, want %q", code, gotPath, path)
		}
	}
}

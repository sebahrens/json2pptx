package main

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// A short spanning card inherits a shrink from a single-row sibling. Editing
// the short card does nothing: both validate and generation must name the peer.
func TestSharedAutofitFindingNamesOverflowingPeer(t *testing.T) {
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns":2,"vertical_align":"top","rows":[
		{"min_height":40,"max_height":40,"cells":[
			{"row_span":3,"shape":{"geometry":"rect","text":{"content":"Short title","size":12,"bold":true}}},
			{"shape":{"geometry":"rect","text":{"content":"A much longer statement that wraps onto several lines and cannot fit in its one row even though the short card next to it has three rows of space.","size":12}}}]},
		{"min_height":40,"max_height":40,"cells":[{"shape":{"geometry":"rect","text":{"content":"Brief","size":12}}}]},
		{"min_height":40,"max_height":40,"cells":[{"shape":{"geometry":"rect","text":{"content":"Also brief","size":12}}}]}]}`), &grid); err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{ViewingMode: "read", Slides: []SlideInput{{ShapeGrid: &grid}}}
	w, h := int64(600*12700), int64(400*12700)
	findings := readabilityCodes(collectReadability(input, nil, w, h, nil, false))
	cause := "/slides/0/shape_grid/rows/0/cells/1"
	affected := "/slides/0/shape_grid/rows/0/cells/0"
	var inherited bool
	for _, f := range findings {
		if f.Fix == nil || f.Fix.Params["affected_cell_path"] != affected {
			continue
		}
		inherited = true
		if f.Path != cause+"/shape/text" || f.Fix.Params["cell_path"] != cause || !strings.Contains(f.Message, affected) || f.Fix.Params["paragraph_text"] == "Short title" {
			t.Fatalf("inherited shrink blamed the wrong cell: %+v", f)
		}
	}
	if !inherited {
		t.Fatalf("fixture must expose an inherited unreadable title: %+v", findings)
	}
	out, err := resolveShapeGrid(&grid, pptx.NewShapeIDAllocator(nil), nil, nil, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	var inheritedSource bool
	for _, src := range out.ShapeSources {
		if src.Path == affected+"/shape/text" {
			inheritedSource = len(src.SharedAutofitSources) == 1 && src.SharedAutofitSources[0] == cause+"/shape/text"
		}
	}
	if !inheritedSource {
		t.Fatalf("generation lost shared shrink provenance: %+v", out.ShapeSources)
	}
	// Nested pattern/source mapping must keep both the affected and cause paths.
	sources := nestedPatternCells{"/slides/0/shape_grid/rows/0/cells/0": "sample"}.shapeSources([]gridShapeSource{{
		Path:                 "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/shape/text",
		SharedAutofitSources: []string{"/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/1/shape/text"},
	}})
	if strings.Contains(sources[0].Path, "/grid/") || strings.Contains(sources[0].SharedAutofitSources[0], "/grid/") {
		t.Fatalf("nested shared shrink paths were not rerooted: %+v", sources)
	}
}

// Written sizes can shrink while stored autofit returns to 100%. Attribution
// and original KPI roles must survive that native writer optimization.
func TestSharedAutofitBakedKPIKeepsCause(t *testing.T) {
	const cause = "/slides/0/shape_grid/rows/0/cells/1"
	const affected = "/slides/0/shape_grid/rows/0/cells/0"
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns":2,"vertical_align":"top","rows":[{"min_height":60,"max_height":60,"cells":[{"row_span":2,"shape":{"geometry":"rect","type_scale":"compact","text":{"paragraphs":[{"content":"42%","size":24,"bold":true,"figure":true}]}}},{"shape":{"geometry":"rect","type_scale":"compact","text":{"paragraphs":[{"content":"USD 1234567890 1234567890 1234567890 1234567890","size":24,"bold":true,"figure":true}]}}}]},{"min_height":60,"max_height":60,"cells":[{"shape":{"geometry":"rect","type_scale":"compact","text":{"paragraphs":[{"content":"43%","size":24,"bold":true,"figure":true}]}}}]}]}`), &grid); err != nil {
		t.Fatal(err)
	}
	out, err := resolveShapeGrid(&grid, pptx.NewShapeIDAllocator(nil), nil, nil, 600*12700, 500*12700, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundSource := false
	for i, s := range out.ShapeSources {
		if s.Path != affected+"/shape/text" {
			continue
		}
		foundSource = true
		if len(s.SharedAutofitOriginalSizes) != 1 || s.SharedAutofitOriginalSizes[0] != 2400 {
			t.Fatalf("original KPI size lost: %+v", s)
		}
		var shape struct {
			Body struct {
				Properties struct {
					Autofit struct {
						Scale int `xml:"fontScale,attr"`
					} `xml:"normAutofit"`
				} `xml:"bodyPr"`
				Paragraphs []struct {
					Runs []struct {
						Properties struct {
							Size int `xml:"sz,attr"`
						} `xml:"rPr"`
					} `xml:"r"`
				} `xml:"p"`
			} `xml:"txBody"`
		}
		if err := xml.Unmarshal(out.Shapes[i], &shape); err != nil {
			t.Fatal(err)
		}
		if scale := shape.Body.Properties.Autofit.Scale; scale != 0 && scale != 100000 {
			t.Fatalf("fixture uses stored shrink %d", scale)
		}
		if len(shape.Body.Paragraphs) != 1 || len(shape.Body.Paragraphs[0].Runs) == 0 {
			t.Fatal("missing written KPI")
		}
		if size := shape.Body.Paragraphs[0].Runs[0].Properties.Size; size <= 0 || size >= 1800 {
			t.Fatalf("expected baked KPI below18pt, got %d", size)
		}
	}
	if !foundSource {
		t.Fatal("affected source missing")
	}
	input := &PresentationInput{ViewingMode: "read", Slides: []SlideInput{{ShapeGrid: &grid}}}
	for _, f := range readabilityCodes(collectReadability(input, nil, 600*12700, 500*12700, nil, false)) {
		if f.Fix == nil || f.Fix.Params["affected_cell_path"] != affected {
			continue
		}
		if f.Path != cause+"/shape/text" || f.Fix.Params["role"] != "kpi-value" || f.Fix.Params["paragraph_text"] != "USD 1234567890 1234567890 1234567890 1234567890" {
			t.Fatalf("wrong baked source: %+v", f)
		}
		return
	}
	t.Fatal("baked KPI finding missing")
}

func TestSharedAutofitMultipleCausesDoNotPromiseSingleFieldBudget(t *testing.T) {
	f := &patterns.FitFinding{ValidationError: patterns.ValidationError{Fix: &patterns.FixSuggestion{Kind: "reduce_cell_text", Params: map[string]any{"measurement_source": "predicted", "cell_path": "/slides/0/shape_grid/rows/0/cells/0", "shared_autofit_sources": []string{"first", "second"}, "max_chars": 99}}}}
	fieldRepairBudget(f, fieldRepairScope{authored: &PresentationInput{Slides: []SlideInput{{}}}, slideIdx: 0}, []cellParagraph{{text: "Overflowing first field"}})
	if f.Fix.Params["repair"] != "composition" || f.Fix.Params["field_max_chars"] != nil || f.Fix.Params["max_chars"] != nil {
		t.Fatalf("unsafe moving-cause budget: %+v", f.Fix)
	}
}

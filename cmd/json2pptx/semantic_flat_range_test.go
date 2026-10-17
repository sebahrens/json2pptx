package main

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"reflect"
	"strings"
	"testing"
)

func TestSemanticFlatRangeKeepsLocatedScaleAdvice(t *testing.T) {
	spec := map[string]any{"meta": map[string]any{"template": "midnight-blue", "title": "Service rate"}, "slides": []any{map[string]any{"kind": "chart_insight", "title": "Service rate varies within a narrow band", "chart": map[string]any{"type": "line_chart", "data": map[string]any{"categories": []string{"Q1", "Q2", "Q3"}, "series": []any{map[string]any{"name": "Rate", "values": []float64{90, 95, 92}}}, "y_min": 0}}, "insights": []string{"The rate stays above 90 while quarter 2 peaks at 95."}, "source": "Illustrative service records", "takeaway": "Track the variation while preserving the zero baseline comparison."}}}
	assertSemanticScaleResponse(t, spec, "/slides/0/chart/data/y_min")
}

func TestSemanticFlatRangeLocatesOneOfMultipleRegionCharts(t *testing.T) {
	chart := func(values []float64) map[string]any {
		return map[string]any{"kind": "chart", "chart": map[string]any{"type": "line_chart", "data": map[string]any{"categories": []string{"Q1", "Q2", "Q3"}, "series": []any{map[string]any{"name": "Rate", "values": values}}, "y_min": 0}}}
	}
	spec := map[string]any{"meta": map[string]any{"template": "midnight-blue", "title": "Service rate"}, "slides": []any{map[string]any{"kind": "regions", "title": "Service rate varies within a narrow band", "arrangement": "columns", "regions": []any{chart([]float64{10, 30, 20}), chart([]float64{90, 95, 92})}, "source": "Illustrative service records", "takeaway": "Track the variation while preserving the zero baseline comparison."}}}
	assertSemanticScaleResponse(t, spec, "/slides/0/regions/1/chart/data/y_min")
}

func assertSemanticScaleResponse(t *testing.T, spec map[string]any, wantPath string) {
	t.Helper()
	before, _ := json.Marshal(spec)
	verdict := deckSpecVerdicts(t, semanticTestConfig(t), map[string]any{"spec": spec})
	for name, structured := range map[string]any{"validate": verdict.ValidateStructured, "render": verdict.RenderStructured} {
		t.Run(name, func(t *testing.T) {
			var root any
			raw, err := json.Marshal(structured)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &root); err != nil {
				t.Fatal(err)
			}
			finding := flatRangeResponseFinding(root)
			if finding == nil {
				t.Fatalf("missing flat range finding: %s", raw)
			}
			b, _ := json.Marshal(finding)
			t.Log(string(b))
			var holder map[string]any
			if name == "validate" {
				holder = finding["remediation"].(map[string]any)["primary"].(map[string]any)
			} else {
				holder, _ = finding["recommended_edit"].(map[string]any)
			}
			params, _ := holder["params"].(map[string]any)
			want := map[string]any{"field": "data.y_min", "value": float64(89), "data_min": float64(90), "data_max": float64(95), "axis_min": float64(0), "axis_max": float64(100), "range_share": float64(.05), "fix_kind": "explicit_scale"}
			if finding["path"] != wantPath || !reflect.DeepEqual(params, want) {
				t.Fatalf("incorrect located scale advice: %s", b)
			}
			vocabulary := map[string]bool{}
			addScaleAdviceVocabulary(vocabulary, finding)
			for _, text := range adviceTexts(finding) {
				for _, word := range fieldLikeRE.FindAllString(text, -1) {
					if !vocabulary[word] && !remedyParamNames[word] {
						t.Errorf("published scale advice omitted from vocabulary catalog: %s", word)
					}
				}
			}
			if finding["next_tool_call"] != nil || finding["patch_verified"] == true {
				t.Fatal("scale advice became an automatic patch")
			}

		})
	}
	after, _ := json.Marshal(spec)
	if string(before) != string(after) {
		t.Fatal("semantic handlers mutated authored axis or chart data")
	}
}

func flatRangeResponseFinding(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		if code, _ := x["code"].(string); strings.HasSuffix(code, "chart.flat_range") {
			return x
		}
		for _, child := range x {
			if found := flatRangeResponseFinding(child); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range x {
			if found := flatRangeResponseFinding(child); found != nil {
				return found
			}
		}
	}
	return nil
}

func TestSemanticScaleAdviceDoesNotGuessBetweenCharts(t *testing.T) {
	sm := semantic.NewSourceMap()
	sm.Add("slides[0].pattern.values.left", "slides[0].regions[0].chart", 0)
	sm.Add("slides[0].pattern.values.right", "slides[0].regions[1].chart", 0)
	d := semanticDiagnostic{SemanticPath: "slides[0]"}
	f := patterns.FitFinding{ValidationError: patterns.ValidationError{Path: "/slides/0/pattern/rows/0/cells/0/diagram/data/y_max", Fix: &patterns.FixSuggestion{Kind: explicitScaleAdvice, Params: map[string]any{"field": "data.y_max", "value": -89}}}}
	locateScaleAdvice(&d, sm, nil, f)
	if d.SemanticPath != "slides[0]" {
		t.Fatal("ambiguous chart assigned an invented edit path")
	}
	if got := remedyFacts(map[string]any{"field": "raw_internal_field", "value": 89}); got != nil {
		t.Fatalf("scale-only forwarding broadened unrelated remedies: %v", got)
	}
}

func TestSemanticScaleAdviceLeavesOtherExplicitScaleContractsAlone(t *testing.T) {
	d := semanticDiagnostic{SemanticPath: "slides[0].chart"}
	fix := &patterns.FixSuggestion{Kind: explicitScaleAdvice, Params: map[string]any{"options": []string{"style.scale=log"}}}
	attachFixParams(&d, fix)
	if d.RecommendedEdit != nil {
		t.Fatal("non-bound advice acquired a fabricated numeric bound edit")
	}
	if got := remedyFacts(semanticFixParams(fix.Kind, fix.Params)); got != nil {
		t.Fatalf("unrelated explicit_scale remedy changed: %v", got)
	}
}

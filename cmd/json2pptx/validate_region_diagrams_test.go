package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// hdx2lAnalysis is a template with a title + body content layout and a
// title-only layout, enough for validateSlidesAgainstTemplate to accept a
// body-placeholder diagram, a shape_grid and a compose slide.
func hdx2lAnalysis() *types.TemplateAnalysis {
	return &types.TemplateAnalysis{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		Layouts: []types.LayoutMetadata{{
			ID: "content-slide", Name: "Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body", Type: types.PlaceholderBody},
			},
		}},
	}
}

// hdx2lSlide places diagram (raw JSON) in the given placement.
func hdx2lSlide(t *testing.T, placement, diagram string) SlideInput {
	t.Helper()
	var raw string
	switch placement {
	case "placeholder":
		raw = `{"layout_id": "content-slide", "content": [
			{"placeholder_id": "body", "type": "diagram", "diagram_value": ` + diagram + `}]}`
	case "grid":
		raw = `{"layout_id": "content-slide", "shape_grid": {"columns": [70, 30], "rows": [{"cells": [
			{"diagram": ` + diagram + `},
			{"shape": {"geometry": "rect", "fill": "none", "text": "Margin 42%"}}]}]}}`
	case "compose":
		raw = `{"layout_id": "content-slide", "compose": {"direction": "horizontal", "segments": [
			{"diagram": ` + diagram + `, "size_pct": 70},
			{"pattern": {"name": "stat-hero", "values": {"value": "42%", "label": "Margin"}}, "size_pct": 30}]}}`
	default:
		t.Fatalf("unknown placement %q", placement)
	}
	var s SlideInput
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal %s slide: %v", placement, err)
	}
	return s
}

func hdx2lValidate(slide SlideInput) []diagnostics.Diagnostic {
	out := dryRunOutput{Valid: true}
	validateSlidesAgainstTemplate(&out, []SlideInput{slide}, hdx2lAnalysis())
	var errs []diagnostics.Diagnostic
	for _, d := range out.Diagnostics {
		if d.Severity == diagnostics.SeverityError {
			errs = append(errs, d)
		}
	}
	return errs
}

// TestNativeDiagramWrongDataKeys_RefusedInEveryPlacement is the
// go-slide-creator-hdx2l repro: the layout-composition evidence fixtures wrote
// pyramid levels as {"title"} and house sections as {"title", "bullets"}. The
// builders read "label" / "items", so every label rendered empty in a body
// placeholder, a shape_grid cell and a compose segment while validate called
// the deck clean. validate and generate must now both refuse, naming the key
// the builder reads.
func TestNativeDiagramWrongDataKeys_RefusedInEveryPlacement(t *testing.T) {
	cases := []struct {
		name     string
		diagram  string
		wantKeys map[string]string // unknown key -> did-you-mean
	}{
		{
			name:     "pyramid",
			diagram:  `{"type": "pyramid", "data": {"levels": [{"title": "Strategy"}, {"title": "Delivery"}, {"title": "Operations"}]}}`,
			wantKeys: map[string]string{"title": "label"},
		},
		{
			name:     "house_diagram",
			diagram:  `{"type": "house_diagram", "data": {"roof": "Vision", "sections": [{"title": "Growth", "bullets": ["Scale"]}], "foundation": "People"}}`,
			wantKeys: map[string]string{"title": "label", "bullets": "items"},
		},
	}
	for _, tc := range cases {
		for _, placement := range []string{"placeholder", "grid", "compose"} {
			t.Run(tc.name+"/"+placement, func(t *testing.T) {
				slide := hdx2lSlide(t, placement, tc.diagram)
				errs := hdx2lValidate(slide)
				found := map[string]bool{}
				for _, d := range errs {
					if d.Code != patterns.ErrCodeUnknownKey {
						continue
					}
					for key, want := range tc.wantKeys {
						if !strings.HasSuffix(d.Path, "/"+key) {
							continue
						}
						found[key] = true
						if d.Fix == nil || d.Fix.Kind != "rename_field" || d.Fix.Params["to"] != want {
							t.Errorf("%s: fix = %+v, want rename_field to %q", d.Path, d.Fix, want)
						}
						if !strings.Contains(d.Message, fmt.Sprintf("did you mean %q", want)) {
							t.Errorf("message lacks did-you-mean %q: %s", want, d.Message)
						}
					}
				}
				for key := range tc.wantKeys {
					if !found[key] {
						t.Errorf("validate did not refuse key %q; errors: %+v", key, errs)
					}
				}

				err := generateErr(slide)
				if err == nil {
					t.Fatal("generate accepted a diagram whose labels would all render empty")
				}
				for key, want := range tc.wantKeys {
					if !strings.Contains(err.Error(), fmt.Sprintf("unknown key %q", key)) ||
						!strings.Contains(err.Error(), fmt.Sprintf("did you mean %q", want)) {
						t.Errorf("generate error does not name %q -> %q: %v", key, want, err)
					}
				}
			})
		}
	}
}

// TestNativeDiagramCorrectDataKeys_AcceptedInEveryPlacement: the same
// diagrams with the keys the builders read pass validate's data checks in
// every placement.
func TestNativeDiagramCorrectDataKeys_AcceptedInEveryPlacement(t *testing.T) {
	diagrams := map[string]string{
		"pyramid":       `{"type": "pyramid", "data": {"levels": [{"label": "Strategy"}, {"label": "Delivery", "description": "Ship it"}, "Operations"]}}`,
		"house_diagram": `{"type": "house_diagram", "data": {"roof": "Vision", "sections": [{"label": "Growth", "items": ["Scale"]}], "foundation": {"label": "People"}, "footnote": "Illustrative"}}`,
	}
	for name, diagram := range diagrams {
		for _, placement := range []string{"placeholder", "grid", "compose"} {
			t.Run(name+"/"+placement, func(t *testing.T) {
				for _, d := range hdx2lValidate(hdx2lSlide(t, placement, diagram)) {
					if d.Code == patterns.ErrCodeUnknownKey || d.Code == string(diagnostics.CodeInvalidSlide) {
						t.Errorf("unexpected data error: %+v", d)
					}
				}
			})
		}
	}
}

// TestRegionDiagramUnknownType_ValidateMatchesGenerate: a region diagram whose
// type no renderer owns aborts generate ("unknown diagram type"); validate
// used to skip it on the belief another check caught it — for a shape_grid
// cell and for a compose segment alike.
func TestRegionDiagramUnknownType_ValidateMatchesGenerate(t *testing.T) {
	diagram := `{"type": "process_flowz", "data": {"steps": [{"label": "a"}]}}`
	for _, placement := range []string{"grid", "compose"} {
		t.Run(placement, func(t *testing.T) {
			slide := hdx2lSlide(t, placement, diagram)
			var hit *diagnostics.Diagnostic
			errs := hdx2lValidate(slide)
			for i := range errs {
				if errs[i].Code == string(diagnostics.CodeUnknownEnum) && strings.HasSuffix(errs[i].Path, "/diagram/type") {
					hit = &errs[i]
				}
			}
			if hit == nil {
				t.Fatalf("validate did not refuse unknown region diagram type; errors: %+v", errs)
			}
			if hit.Fix == nil || hit.Fix.Params["did_you_mean"] != "process_flow" {
				t.Errorf("fix = %+v, want did_you_mean process_flow", hit.Fix)
			}
			if err := generateErr(slide); err == nil || !strings.Contains(err.Error(), "unknown diagram type") {
				t.Errorf("generate error = %v, want unknown diagram type", err)
			}
		})
	}
}

// TestComposeSvggenDiagramFailure_ValidateMatchesGenerate: a compose segment
// whose svggen data generate rejects is an error at validate too.
func TestComposeSvggenDiagramFailure_ValidateMatchesGenerate(t *testing.T) {
	slide := hdx2lSlide(t, "compose", `{"type": "bar_chart", "data": {"categories": ["a", "b"], "series": [{"name": "s", "values": [1, 2]}], "colour": "x"}}`)
	errs := hdx2lValidate(slide)
	ok := false
	for _, d := range errs {
		// An unread key is reported at the key with a did-you-mean
		// (go-slide-creator-x9s5i).
		if d.Code == patterns.ErrCodeUnknownKey && d.Path == "/slides/0/compose/segments/0/diagram/data/colour" &&
			d.Fix != nil && d.Fix.Params["to"] == "colors" {
			ok = true
		}
	}
	if !ok {
		t.Errorf("validate did not refuse the compose bar_chart; errors: %+v", errs)
	}
	if err := generateErr(slide); err == nil {
		t.Error("generate accepted the compose bar_chart validate refuses")
	}
}

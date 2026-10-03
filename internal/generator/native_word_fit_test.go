package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

const wordFitPt = int64(12700)

func wordFitPyramid(top string) *types.DiagramSpec {
	return &types.DiagramSpec{Type: "pyramid", Data: map[string]any{"levels": []any{
		map[string]any{"label": top, "description": "Short description"},
		map[string]any{"label": "Operating model", "description": "Short description"},
		map[string]any{"label": "Capabilities", "description": "Short description"},
		map[string]any{"label": "Foundations", "description": "Short description"},
	}}}
}

func wordFitValueChain() *types.DiagramSpec {
	return &types.DiagramSpec{Type: "value_chain", Data: map[string]any{
		"primary": []any{
			map[string]any{"label": "Inbound Logistics", "items": []any{"Sourcing", "Supplier mgmt"}},
			map[string]any{"label": "Operations", "items": []any{"Assembly", "QA"}},
			map[string]any{"label": "Outbound Logistics", "items": []any{"Warehousing", "Fulfilment"}},
			map[string]any{"label": "Marketing & Sales", "items": []any{"Brand", "Direct sales"}},
			map[string]any{"label": "Service", "items": []any{"Support", "Warranty"}},
		},
		"support": []any{map[string]any{"label": "Firm Infrastructure", "items": []any{"Finance"}}},
	}}
}

// go-slide-creator-v74wv: the 2026-10-02 sweep's narrow regions — a pyramid
// apex in a 33-50% compose column ("Stra / teg / y") and a native value chain
// at 70% ("Inboun / d", "Warehousi / ng") — now render every word whole: the
// trapezoid's real text rectangle and a bullet's marL reach the writer's
// margin clamp. The scan of the written group agrees.
func TestNativeNarrowRegionsKeepWordsWhole(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spec   *types.DiagramSpec
		bounds types.BoundingBox
	}{
		{"pyramid in a 33% column", wordFitPyramid("Strategy"), types.BoundingBox{Width: 285 * wordFitPt, Height: 300 * wordFitPt}},
		{"pyramid in a 40% column", wordFitPyramid("Strategy"), types.BoundingBox{Width: 340 * wordFitPt, Height: 300 * wordFitPt}},
		{"value chain at 70%", wordFitValueChain(), types.BoundingBox{Width: 600 * wordFitPt, Height: 300 * wordFitPt}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, font := range []string{"Calibri", "Arial", "Segoe UI"} {
				d, err := GenerateGridNativeDiagram(tc.spec, NativeDiagramRegion{Bounds: tc.bounds, FontName: font, Path: "/p"}, "alt",
					func(int) uint32 { return 100 })
				if err != nil {
					t.Fatalf("%s: %v", font, err)
				}
				if unfit := pptx.UnfitWords(d.XML, font); len(unfit) != 0 {
					t.Errorf("%s: words broken mid-word: %+v", font, unfit)
				}
				for _, f := range d.Findings {
					if f.Code == patterns.ErrCodeTextExceedsShape {
						t.Errorf("%s: unexpected %s", font, f.Message)
					}
				}
				if fs := NativeDiagramWordFitPreflight(tc.spec, tc.bounds, font, nil, "/p"); len(fs) != 0 {
					t.Errorf("%s: preflight reported %+v", font, fs)
				}
			}
		})
	}
}

// A word no native shape can hold at its fixed size is reported, not broken
// silently: validate's preflight and generation's own scan raise the same
// TEXT_EXCEEDS_SHAPE at the authored diagram.
func TestNativeWordTooWideIsReportedAtValidateAndGenerate(t *testing.T) {
	for _, tc := range []struct {
		name, word string
		spec       *types.DiagramSpec
		bounds     types.BoundingBox
	}{
		{"pyramid apex", "Transformation", wordFitPyramid("Transformation"), types.BoundingBox{Width: 240 * wordFitPt, Height: 360 * wordFitPt}},
		{"value chain", "Warehousing", wordFitValueChain(), types.BoundingBox{Width: 330 * wordFitPt, Height: 300 * wordFitPt}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const path = "/slides/0/shape_grid/rows/0/cells/0/diagram"
			pre := NativeDiagramWordFitPreflight(tc.spec, tc.bounds, "Calibri", nil, path)
			if len(pre) != 1 {
				t.Fatalf("preflight: want one TEXT_EXCEEDS_SHAPE, got %+v", pre)
			}
			f := pre[0]
			if f.Code != patterns.ErrCodeTextExceedsShape || f.Path != path || f.Pattern != tc.spec.Type ||
				f.Fix == nil || f.Fix.Kind != "widen_shape_text_area" {
				t.Fatalf("preflight finding: %+v", f)
			}
			if words, _ := f.Fix.Params["words"].([]string); !containsString(words, tc.word) {
				t.Errorf("want %q among the broken words, got %v", tc.word, words)
			}
			d, err := GenerateGridNativeDiagram(tc.spec, NativeDiagramRegion{Bounds: tc.bounds, FontName: "Calibri", Path: path}, "alt",
				func(int) uint32 { return 100 })
			if err != nil {
				t.Fatal(err)
			}
			var gen *patterns.FitFinding
			for i := range d.Findings {
				if d.Findings[i].Code == patterns.ErrCodeTextExceedsShape {
					gen = &d.Findings[i]
				}
			}
			if gen == nil || gen.Path != f.Path || gen.Message != f.Message || gen.Action != f.Action {
				t.Fatalf("generation must raise the finding validate predicts:\nvalidate: %+v\ngenerate: %+v", f, gen)
			}
		})
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

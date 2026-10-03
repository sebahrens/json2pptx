package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-v74wv: narrow-region builders broke single words mid-word
// with no finding — a pyramid apex in a compose column ("Stra / teg / y"), a
// native value chain at 70% ("Inboun / d", "Sourcin / g") and timeline-
// horizontal in a 33% column ("Desig / n", "Octob / er" on modern-yellow).

func narrowRegionSlide(title string, segments ...any) map[string]any {
	return map[string]any{
		"layout_id": "content",
		"content":   []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": title}},
		"compose":   map[string]any{"direction": "horizontal", "segments": segments},
	}
}

func narrowRegionSegment(pct int, kind string, v any) map[string]any {
	return map[string]any{"size_pct": pct, kind: v}
}

func narrowRegionHero(pct int) map[string]any {
	return narrowRegionSegment(pct, "pattern", map[string]any{"name": "stat-hero", "values": map[string]any{"value": "3.2x", "label": "Growth"}})
}

func narrowRegionPyramid(top string, levels int, describe bool) map[string]any {
	names := []string{top, "Operating model", "Capabilities", "Foundations", "Data platforms"}
	var ls []any
	for _, n := range names[:levels] {
		level := map[string]any{"label": n}
		if describe {
			level["description"] = "Short description"
		}
		ls = append(ls, level)
	}
	return map[string]any{"type": "pyramid", "alt": "Pyramid", "data": map[string]any{"levels": ls}}
}

func narrowRegionValueChain() map[string]any {
	return map[string]any{"type": "value_chain", "alt": "Value chain", "data": map[string]any{
		"primary": []any{
			map[string]any{"label": "Inbound Logistics", "items": []any{"Sourcing", "Supplier mgmt"}},
			map[string]any{"label": "Operations", "items": []any{"Assembly", "QA"}},
			map[string]any{"label": "Outbound Logistics", "items": []any{"Warehousing", "Fulfilment"}},
			map[string]any{"label": "Marketing & Sales", "items": []any{"Brand", "Direct sales"}},
			map[string]any{"label": "Service", "items": []any{"Support", "Warranty"}},
		},
		"support": []any{
			map[string]any{"label": "Firm Infrastructure", "items": []any{"Finance"}},
			map[string]any{"label": "Human Resources", "items": []any{"Hiring"}},
		},
	}}
}

func narrowRegionTemplates() []string {
	tpls := []string{"midnight-blue", "modern-yellow"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		tpls = append(tpls, "p-style")
	}
	return tpls
}

// generateNarrowRegionDeck generates slides on template and returns the
// generate result and the slide XML.
func generateNarrowRegionDeck(t *testing.T, template string, slides []any) (JSONOutput, []string) {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(map[string]any{"template": template, "output_filename": "deck.pptx", "slides": slides})
	if err != nil {
		t.Fatal(err)
	}
	in, report := filepath.Join(dir, "input.json"), filepath.Join(dir, "result.json")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		t.Fatal(err)
	}
	runErr := runJSONMode(in, report, "../../templates", dir, "", false, false, template, "off", false, "off", "", false)
	out, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("generate: %v", runErr)
	}
	var result JSONOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || result.OutputPath == "" {
		t.Fatalf("generate failed: %v %+v", runErr, result.FitFindings)
	}
	return result, readPPTXSlides(t, filepath.Join(dir, "deck.pptx"))
}

func TestNarrowRegionBuildersKeepWordsWhole(t *testing.T) {
	slides := []any{
		narrowRegionSlide("Pyramid in a 50% column", narrowRegionSegment(50, "diagram", narrowRegionPyramid("Strategy", 4, true)), narrowRegionHero(50)),
		narrowRegionSlide("Pyramid in a 40% column", narrowRegionSegment(40, "diagram", narrowRegionPyramid("Strategy", 5, true)), narrowRegionHero(60)),
		narrowRegionSlide("Pyramid in a 33% column", narrowRegionSegment(33, "diagram", narrowRegionPyramid("Strategy", 4, true)), narrowRegionHero(67)),
		narrowRegionSlide("Value chain at 70%", narrowRegionSegment(70, "diagram", narrowRegionValueChain()), narrowRegionHero(30)),
		narrowRegionSlide("Timeline in a 33% column", narrowRegionSegment(33, "pattern", map[string]any{"name": "timeline-horizontal", "values": []any{
			map[string]any{"date": "October", "label": "Design"}, map[string]any{"date": "November", "label": "Pilot"},
			map[string]any{"date": "December", "label": "Rollout"}, map[string]any{"date": "January", "label": "Scale"},
		}}), narrowRegionHero(67)),
	}
	for _, tpl := range narrowRegionTemplates() {
		t.Run(tpl, func(t *testing.T) {
			_, theme, _, _ := fitReportGeometry(tpl, filepath.Join("..", "..", "templates"))
			font := ""
			if theme != nil {
				font = theme.BodyFont
			}
			result, parts := generateNarrowRegionDeck(t, tpl, slides)
			for i, xml := range parts {
				if unfit := pptx.UnfitWords(xml, font); len(unfit) != 0 {
					t.Errorf("slide %d breaks words mid-word: %+v", i+1, unfit)
				}
			}
			for _, f := range result.FitFindings {
				if f.Code == patterns.ErrCodeTextExceedsShape {
					t.Errorf("unexpected %s at %s: %s", f.Code, f.Path, f.Message)
				}
			}
		})
	}
}

// What the builders cannot hold is reported — by validate first, and by the
// generate report — instead of breaking silently.
func TestNarrowRegionWordBreaksAreReported(t *testing.T) {
	slides := []any{
		narrowRegionSlide("Narrow pyramid", narrowRegionSegment(22, "diagram", narrowRegionPyramid("Transformation", 3, false)), narrowRegionHero(78)),
		narrowRegionSlide("Narrow value chain", narrowRegionSegment(45, "diagram", narrowRegionValueChain()), narrowRegionHero(55)),
	}
	const tpl = "midnight-blue"
	data, err := json.Marshal(map[string]any{"template": tpl, "slides": slides})
	if err != nil {
		t.Fatal(err)
	}
	var in PresentationInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	_, _, validate := gateFor(t, &in)
	predicted := map[string]patterns.FitFinding{}
	for _, f := range validate {
		if f.Code == patterns.ErrCodeTextExceedsShape {
			predicted[f.Path] = f
		}
	}
	for _, want := range []string{
		"/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/diagram",
		"/slides/1/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/diagram",
	} {
		f, ok := predicted[want]
		if !ok {
			t.Fatalf("validate did not predict the mid-word break at %s: %+v", want, predicted)
		}
		if f.Pattern == "" || f.Fix == nil || f.Fix.Kind != "widen_shape_text_area" {
			t.Errorf("finding at %s: %+v", want, f)
		}
	}
	result, _ := generateNarrowRegionDeck(t, tpl, slides)
	for path, want := range predicted {
		found := false
		for _, f := range result.FitFindings {
			if f.Code == want.Code && f.Path == path && f.Message == want.Message {
				found = true
			}
		}
		if !found {
			t.Errorf("generate report lacks the %s validate predicted at %s", want.Code, path)
		}
	}
}

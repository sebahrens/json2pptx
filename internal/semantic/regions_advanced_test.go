package semantic

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func advancedRegionsFixture(arrangement string) map[string]any {
	chart := map[string]any{"kind": "chart", "chart": map[string]any{"type": "pie_chart", "data": map[string]any{"categories": []any{"A", "B"}, "values": []any{60, 40}}}}
	text := map[string]any{"kind": "text", "body": "One owner is accountable."}
	regions := []any{chart, map[string]any{"kind": "stat", "value": "32%", "label": "Margin"}, map[string]any{"kind": "table", "headers": []any{"Team", "Owner"}, "rows": []any{[]any{"Pilot", "Finance"}}}, text}
	if arrangement == slides.ArrangeTilesOverPanels {
		regions = []any{map[string]any{"kind": "kpis", "kpis": []any{map[string]any{"value": "32%", "label": "Margin"}, map[string]any{"value": "12", "label": "Owners"}}}, chart, text}
	}
	return map[string]any{"title": "Owners turn data into decisions", "takeaway": "The pilot has an accountable owner.", "arrangement": arrangement, "regions": regions, "gap": 12, "divider": true}
}

func TestAdvancedRegionsArrangementsKeepEveryPane(t *testing.T) {
	for _, arrangement := range []string{slides.ArrangeQuad, slides.ArrangeMainLeft3, slides.ArrangeMainTop3, slides.ArrangeTilesOverPanels} {
		t.Run(arrangement, func(t *testing.T) {
			slide, _ := compileRegionsOK(t, advancedRegionsFixture(arrangement))
			encoded, _ := json.Marshal(slide.ShapeGrid)
			for _, text := range []string{"Finance", "One owner is accountable."} {
				if arrangement == slides.ArrangeTilesOverPanels && text == "Finance" {
					continue
				}
				if !strings.Contains(string(encoded), text) {
					t.Fatalf("pane lost %q: %s", text, encoded)
				}
			}
			if slide.ShapeGrid.Gap != 12 || string(slide.ShapeGrid.ColRule) != `"all"` {
				t.Fatalf("pane seams missing: %+v", slide.ShapeGrid)
			}
			switch arrangement {
			case slides.ArrangeQuad:
				if len(slide.ShapeGrid.Rows) != 2 || len(slide.ShapeGrid.Rows[1].Cells) != 2 {
					t.Fatal("quad does not have four cells")
				}
			case slides.ArrangeMainLeft3:
				if len(slide.ShapeGrid.Rows[0].Cells[1].Grid.Rows) != 3 {
					t.Fatal("main_left_3 does not have three stacked panes")
				}
			case slides.ArrangeMainTop3:
				if len(slide.ShapeGrid.Rows[1].Cells[0].Grid.Rows[0].Cells) != 3 {
					t.Fatal("main_top_3 does not have three lower panes")
				}
			}
		})
	}
}

func TestAdvancedRegionsRejectInvalidCountAndGap(t *testing.T) {
	for _, gap := range []any{-1, math.NaN(), math.Inf(1), "12"} {
		body := advancedRegionsFixture(slides.ArrangeQuad)
		body["gap"] = gap
		if _, _, err := Compile(regionsSpec(body), CompileOptions{Strict: StrictnessStrict}); err == nil {
			t.Fatalf("invalid gap accepted: %v", gap)
		}
	}
	body := advancedRegionsFixture(slides.ArrangeQuad)
	body["regions"] = append(body["regions"].([]any), map[string]any{"kind": "text", "body": "Fifth"})
	_, res, err := Compile(regionsSpec(body), CompileOptions{Strict: StrictnessStrict})
	if err == nil {
		t.Fatal("fifth pane accepted")
	}
	found := false
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, "quad") && strings.Contains(d.Message, "main_top_3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("count error hides alternatives: %+v", res.Diagnostics)
	}
	body = advancedRegionsFixture(slides.ArrangeTilesOverPanels)
	body["regions"].([]any)[0] = map[string]any{"kind": "text", "body": "Wrong"}
	if _, _, err := Compile(regionsSpec(body), CompileOptions{}); err == nil {
		t.Fatal("tiles over panels accepted a non-KPI band")
	}
}

func TestSectionShowChaptersUsesCurrentAgendaTracker(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Chapters", Template: "midnight-blue"}, Slides: []SlideSpec{{Kind: KindSection, Body: map[string]any{"title": "Diagnosis", "show_chapters": true}}, {Kind: KindSection, Body: map[string]any{"title": "Actions", "show_chapters": true}}}}
	input, res, err := Compile(spec, CompileOptions{Strict: StrictnessStrict})
	if err != nil {
		t.Fatalf("compile %v: %+v", err, res.Diagnostics)
	}
	for i, slide := range input.Slides {
		if len(slide.Content) != 1 || slide.Pattern == nil || slide.Pattern.Name != "agenda" || slide.SlideType != "content" || slide.SectionTitle == "" {
			t.Fatalf("chapter tracker missing: %+v", slide)
		}
		var overrides struct{ Highlight int }
		if err := json.Unmarshal(slide.Pattern.Overrides, &overrides); err != nil {
			t.Fatal(err)
		}
		if overrides.Highlight != i+1 {
			t.Fatalf("current chapter=%d, want %d", overrides.Highlight, i+1)
		}
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func TestSixStepProcessRegionsExpandAtTemplateGeometry(t *testing.T) {
	for _, templateName := range []string{"modern", "midnight-blue"} {
		t.Run(templateName, func(t *testing.T) {
			layouts, width, height := schemaMaximaLayouts(t, templateName)
			for _, arrangement := range []string{slides.ArrangeColumns, slides.ArrangeRows, slides.ArrangeMainLeft, slides.ArrangeMainRight, slides.ArrangeMainTop, slides.ArrangeMainBottom} {
				t.Run(arrangement, func(t *testing.T) {
					steps := []any{}
					for _, label := range []string{"Mail arrives", "System decides", "Person handles", "Staff judge", "Count", "Improve"} {
						steps = append(steps, map[string]any{"label": label, "description": "Name an owner"})
					}
					spec := &semantic.DeckSpec{Meta: semantic.DeckMeta{Title: "Accountable process", Template: templateName}, Slides: []semantic.SlideSpec{{Kind: semantic.KindRegions, Body: map[string]any{
						"title": "Each message reaches an owner", "takeaway": "Track every case to resolution.", "arrangement": arrangement,
						"regions": []any{map[string]any{"kind": "process", "steps": steps}, map[string]any{"kind": "stat", "value": "32%", "label": "Resolved"}, map[string]any{"kind": "text", "body": "One owner per case."}},
					}}}}
					input, res, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessStrict})
					if err != nil {
						t.Fatalf("compile: %v; %v", err, res.Diagnostics)
					}
					findings := collectFitFindings(input, layouts, width, height, nil)
					for _, f := range findings {
						if f.Action == "refuse" || strings.Contains(f.Message, "no readable body") {
							t.Errorf("six short described steps refused: %s: %s", f.Code, f.Message)
						}
					}
				})
			}
		})
	}
}

func TestProcessRegionRefusesDenseNativeLayers(t *testing.T) {
	layouts, width, height := schemaMaximaLayouts(t, "modern")
	steps := []any{}
	for i := 0; i < 6; i++ {
		steps = append(steps, map[string]any{"label": strings.Repeat("word ", 8), "description": strings.Repeat("word ", 16)})
	}
	spec := &semantic.DeckSpec{Meta: semantic.DeckMeta{Title: "Dense process", Template: "modern"}, Slides: []semantic.SlideSpec{{Kind: semantic.KindRegions, Body: map[string]any{
		"title": "Every step has a named owner", "takeaway": "Keep each step concise.", "arrangement": "columns",
		"regions": []any{map[string]any{"kind": "process", "steps": steps}, map[string]any{"kind": "stat", "value": "32%", "label": "Resolved"}, map[string]any{"kind": "text", "body": "One owner per case."}},
	}}}}
	input, res, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessStrict})
	if err != nil {
		t.Fatalf("bounded content must reach measured preflight: %v; %v", err, res.Diagnostics)
	}
	for _, f := range collectFitFindings(input, layouts, width, height, nil) {
		if f.Action == "refuse" {
			return
		}
	}
	t.Fatal("dense process text bypassed actual geometry refusal")
}

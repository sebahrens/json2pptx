package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// rawEscapeHatchSpec is a DeckSpec whose only slide is a raw_json2pptx payload
// carrying hand-set sizes and hex fills — the thing constrained mode exists to
// refuse.
const rawEscapeHatchSpec = `{
  "meta": {"title": "Raw escape hatch", "template": "midnight-blue"%s},
  "slides": [{
    "kind": "raw_json2pptx",
    "slide": {
      "slide_type": "content",
      "content": [{"placeholder_id": "title", "type": "text", "text_value": "Raw grid"}],
      "shape_grid": {"columns": 2, "rows": [{"cells": [
        {"shape": {"geometry": "rect", "fill": "#4472C4", "text": {"content": "Hand-set", "size": 18, "color": "#FFFFFF"}}},
        {"shape": {"geometry": "rect", "fill": "accent2", "text": {"content": "Also hand-set", "size": 22}}}
      ]}]}
    }
  }]
}`

// The same slide was refused by generate_presentation with blocking
// design_mode_violation findings and accepted in silence by render_deck_spec:
// the compiled deck is stamped constrained, but nothing checked it on that path
// (go-slide-creator-rs4h).
func TestDeckSpecEnforcesConstrainedDesignMode(t *testing.T) {
	data := []byte(strings.Replace(rawEscapeHatchSpec, "%s", "", 1))

	viaSpec := specDesignModeDiagnostics("spec.json", data, semantic.StrictnessWarn)
	if len(viaSpec) == 0 {
		t.Fatal("a raw slide with hand-set sizes and hex fills drew no design_mode_violation through the DeckSpec path")
	}
	for _, d := range viaSpec {
		if d.Code != "design_mode_violation" {
			t.Errorf("unexpected code %q", d.Code)
		}
	}

	// The raw path's verdict on the identical slide.
	var spec struct {
		Slides []struct {
			Slide json.RawMessage `json:"slide"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	var slide SlideInput
	if err := json.Unmarshal(spec.Slides[0].Slide, &slide); err != nil {
		t.Fatalf("decode raw slide: %v", err)
	}
	viaRaw := validateDesignMode(&PresentationInput{Template: "midnight-blue", Slides: []SlideInput{slide}})
	if len(viaRaw) != len(viaSpec) {
		t.Errorf("the same slide draws %d violations through generate and %d through the DeckSpec path",
			len(viaRaw), len(viaSpec))
	}
}

// meta.design_mode: "free" is the opt-out. Without it there was no way to say
// the raw values are deliberate on this path at all.
func TestDeckSpecDesignModeFreeOptsOut(t *testing.T) {
	data := []byte(strings.Replace(rawEscapeHatchSpec, "%s", `, "design_mode": "free"`, 1))
	if d := specDesignModeDiagnostics("spec.json", data, semantic.StrictnessWarn); len(d) != 0 {
		t.Errorf("design_mode \"free\" still drew %d violations: %+v", len(d), d)
	}

	// And it reaches the compiled deck rather than being dropped on the floor.
	spec, diags := semantic.Parse("spec.json", data)
	if spec == nil || diags.HasErrors() {
		t.Fatalf("parse: %+v", diags)
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.DesignMode != "free" {
		t.Errorf("compiled design_mode = %q, want free", input.DesignMode)
	}
}

// A spec that does not use the escape hatch compiles to constrained and draws
// nothing: the compiler's own output never hand-sets what the template owns.
func TestCompiledDeckIsConstrainedAndClean(t *testing.T) {
	data := []byte(`{
      "meta": {"title": "Ordinary deck", "template": "midnight-blue"},
      "slides": [
        {"kind": "title", "title": "Q3 review", "subtitle": "September 2026"},
        {"kind": "kpi_snapshot", "title": "Where we are", "kpis": [
          {"value": "71%", "label": "Gross margin"},
          {"value": "117%", "label": "NRR"},
          {"value": "6.2%", "label": "Churn"}]}
      ]
    }`)
	spec, diags := semantic.Parse("spec.json", data)
	if spec == nil || diags.HasErrors() {
		t.Fatalf("parse: %+v", diags)
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.DesignMode != "constrained" {
		t.Errorf("compiled design_mode = %q, want constrained", input.DesignMode)
	}
	if d := compiledDesignModeDiagnostics(input); len(d) != 0 {
		t.Errorf("the compiler's own output violates constrained mode: %+v", d)
	}
}

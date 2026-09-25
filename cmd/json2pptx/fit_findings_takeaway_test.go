package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestTakeawayFitRejectsWrappedChromeText(t *testing.T) {
	short := "Growth is on track."
	long := strings.Repeat("A longer strategic takeaway needs room to breathe. ", 5)
	input := &PresentationInput{Slides: []SlideInput{
		{LayoutID: "slideLayout2", Takeaway: short},
		{LayoutID: "slideLayout2", Takeaway: long},
	}}
	findings := collectTakeawayFitFindings(input, reserveTestLayouts(), shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
	if len(findings) != 1 {
		t.Fatalf("takeaway findings = %+v, want one long-text finding", findings)
	}
	f := findings[0]
	if f.Code != patterns.ErrCodeBodyTooLong || f.Action != "refuse" || f.Path != "/slides/1/takeaway" {
		t.Fatalf("wrong code/action/path: %+v", f)
	}
	if f.Measured == nil || f.Allowed == nil || f.Measured.HeightEMU <= f.Allowed.HeightEMU {
		t.Fatalf("takeaway finding lacks measured overflow: %+v", f)
	}
}

func TestSemanticTakeawayFindingMapsToSourceAndBlocksShipping(t *testing.T) {
	takeaway := strings.Repeat("A longer strategic takeaway needs room to breathe. ", 5)
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Results", Template: "midnight-blue"},
		Slides: []semantic.SlideSpec{{Kind: semantic.KindExecutiveSummary, Body: map[string]any{
			"title": "Results", "points": []any{"Revenue grew", "Costs held", "Margin improved"}, "takeaway": takeaway,
		}}},
	}
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatal(err)
	}
	applyDefaults(input)
	for _, f := range collectFitFindings(input, reserveTestLayouts(), shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU, nil) {
		if f.Code != patterns.ErrCodeBodyTooLong || !strings.HasSuffix(f.Path, "/takeaway") {
			continue
		}
		d := semanticDiagFromFit(result.SourceMap, f)
		if d.SemanticPath != "slides[0].takeaway" || d.Action != "refuse" {
			t.Fatalf("semantic takeaway diagnostic = %+v", d)
		}
		status := semanticPublicationStatus([]semanticDiagnostic{d}, nil, "")
		if status.Publishable || status.DeterministicReady || len(status.DeterministicBlockingReasons) == 0 {
			t.Fatalf("wrapped takeaway remained ready: %+v", status)
		}
		return
	}
	t.Fatal("compiled deck did not report a wrapped takeaway")
}

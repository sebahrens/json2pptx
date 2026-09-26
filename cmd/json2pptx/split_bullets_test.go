package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestRepairSplitBulletsPreservesSourceAndSiblings(t *testing.T) {
	left, right := []string{"L0", "\tL1 child", "L2", "L3", "L4"}, []string{"R0", "R1", "R2", "R3", "R4"}
	base := SlideInput{LayoutID: "slideLayout3", SpeakerNotes: "notes", Source: "source", SourceLink: &LinkInput{URL: "https://example.com"}, Content: []ContentInput{{Type: "bullets", PlaceholderID: "body", BulletsValue: &left}, {Type: "bullets", PlaceholderID: "body_2", BulletsValue: &right}}}
	before, after := SlideInput{LayoutID: "before"}, SlideInput{LayoutID: "after"}
	input := PresentationInput{Slides: []SlideInput{before, base, after}}
	result := applyRepairFix(&input, 1, repairFixInput{Kind: "split_bullets", Params: map[string]any{"max_items": 2}})
	if !result.Applied || len(input.Slides) != 5 || !reflect.DeepEqual(input.Slides[0], before) || !reflect.DeepEqual(input.Slides[4], after) {
		t.Fatalf("result=%+v pages=%d", result, len(input.Slides))
	}
	for col, original := range [][]string{left, right} {
		var joined []string
		for _, slide := range input.Slides[1:4] {
			if slide.LayoutID != base.LayoutID {
				t.Fatal("native layout changed")
			}
			joined = append(joined, (*slide.Content[col].BulletsValue)...)
		}
		if !reflect.DeepEqual(joined, original) {
			t.Fatal("source bullets changed")
		}
	}
}

func TestRepairSplitBulletsInvalidAndNoopAreAtomic(t *testing.T) {
	bullets := []string{"a", "b"}
	for _, params := range []map[string]any{{}, {"max_items": 0}, {"max_items": -1}, {"max_items": 1.5}, {"max_items": "1"}, {"max_items": 1, "path": "/slides/0/content/0"}, {"max_items": 10}} {
		input := PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{Type: "bullets", BulletsValue: &bullets}}}}}
		before, _ := json.Marshal(input)
		result := applySplitBullets(&input, 0, params)
		after, _ := json.Marshal(input)
		if result.Applied || string(before) != string(after) {
			t.Fatalf("no-op/failure mutated source: params=%v result=%+v", params, result)
		}
	}
}

func TestRepairSlideSplitBulletsMCPRoundTrip(t *testing.T) {
	deck := minimalDeck(map[string]any{"placeholder_id": "title", "type": "text", "value": "Source title"}, map[string]any{"placeholder_id": "body", "type": "bullets", "value": []string{"A", "\tA child", "B", "C", "D"}})
	result, err := repairMC(t).handleRepairSlide(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deck), "slide_index": float64(0), "fixes": []any{map[string]any{"kind": "split_bullets", "params": map[string]any{"max_items": float64(2)}}}}))
	if err != nil || result.IsError {
		t.Fatalf("MCP result=%+v err=%v", result, err)
	}
	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.AppliedFixes) != 1 || !output.AppliedFixes[0].Applied {
		t.Fatalf("repair not applied: %+v", output.AppliedFixes)
	}
	var patched PresentationInput
	if err := json.Unmarshal(output.PatchedDeck, &patched); err != nil {
		t.Fatal(err)
	}
	var joined []string
	for _, slide := range patched.Slides {
		if slide.LayoutID != "slideLayout2" {
			t.Fatal("MCP serialization lost layout")
		}
		joined = append(joined, (*slide.Content[1].BulletsValue)...)
	}
	if !reflect.DeepEqual(joined, []string{"A", "\tA child", "B", "C", "D"}) {
		t.Fatal("MCP round trip lost or rewrote source")
	}
}

func TestRepairSplitBulletsRefusesInternalNavigationAtomically(t *testing.T) {
	bullets := []string{"A", "B", "C"}
	for _, sibling := range []SlideInput{
		{SourceLink: &LinkInput{Slide: 2}},
		{Content: []ContentInput{{Type: "text", Link: &LinkInput{Slide: 2}}}},
		{Pattern: &PatternInput{Name: "card-grid", Values: json.RawMessage(`{"items":[{"link":{"slide":2}}]}`)}},
	} {
		input := PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{Type: "bullets", BulletsValue: &bullets}}}, sibling}}
		before, _ := json.Marshal(input)
		result := applySplitBullets(&input, 0, map[string]any{"max_items": 1})
		after, _ := json.Marshal(input)
		if result.Applied || result.Code != "navigation_requires_authoring" || string(before) != string(after) {
			t.Fatal("numeric destinations silently redirected or partial mutation")
		}
	}
}

func TestRepairFindingsIncludeEveryContinuationAndExcludeSiblings(t *testing.T) {
	var findings []patterns.FitFinding
	for _, path := range []string{"/slides/0/content/0", "/slides/1/content/0", "/slides/2/content/0", "/slides/3/content/0", "/slides/4/content/0", "/slides/10/content/0", "/template"} {
		findings = append(findings, patterns.FitFinding{ValidationError: patterns.ValidationError{Path: path}})
	}
	got := filterFindingsForRepairPages(findings, 1, 3)
	if !reflect.DeepEqual(got, findings[1:4]) {
		t.Fatal("continuation diagnostic scope incorrect")
	}
	if got := filterFindingsForRepairPages(findings, 1, 1); !reflect.DeepEqual(got, findings[1:2]) {
		t.Fatal("ordinary unsplit diagnostic scope changed")
	}
}

func TestRepairSlideReturnsUnsafeLaterBulletPageFindings(t *testing.T) {
	bullets := []string{"Short source bullet", strings.Repeat("Required unresolved handoffs are not optional. ", 200), "Final source bullet"}
	other := []string{strings.Repeat("Unrelated sibling source text. ", 200)}
	input := PresentationInput{Template: "modern", Slides: []SlideInput{
		{LayoutID: "slideLayout3", Content: []ContentInput{{Type: "bullets", PlaceholderID: "body", BulletsValue: &bullets}}},
		{LayoutID: "slideLayout3", Content: []ContentInput{{Type: "bullets", PlaceholderID: "body", BulletsValue: &other}}},
	}}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repairMC(t).handleRepairSlide(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(string(raw)), "slide_index": float64(0), "fixes": []any{map[string]any{"kind": "split_bullets", "params": map[string]any{"max_items": float64(1)}}}}))
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var output repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatal(err)
	}
	if !output.AppliedFixes[0].Applied {
		t.Fatalf("repair not applied: %+v", output.AppliedFixes)
	}
	foundLater := false
	for _, finding := range output.Findings.Findings {
		path, _ := finding.Evidence["path"].(string)
		if strings.HasPrefix(path, "/slides/1/") {
			foundLater = true
		}
		if strings.HasPrefix(path, "/slides/3/") {
			t.Fatal("unaffected sibling findings included")
		}
	}
	if !foundLater {
		t.Fatal("unsafe later continuation diagnostics hidden")
	}
}

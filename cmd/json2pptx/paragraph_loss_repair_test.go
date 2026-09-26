package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestParagraphRepairPreflightSurvivesEnvelopeAndPreservesSource(t *testing.T) {
	left, right := []string{"Parent", "\tRequired child", "Next"}, []string{"Peer", "\tPeer child", "Last"}
	for _, legacy := range []bool{false, true} {
		for _, code := range []string{patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed} {
			input := PresentationInput{Slides: []SlideInput{{LayoutID: "slideLayout3", Content: []ContentInput{{Type: "bullets", PlaceholderID: "body", BulletsValue: &left}, {Type: "bullets", PlaceholderID: "body_2", BulletsValue: &right}}}}}
			if legacy {
				for i := range input.Slides[0].Content {
					item := &input.Slides[0].Content[i]
					data, err := json.Marshal(*item.BulletsValue)
					if err != nil {
						t.Fatal(err)
					}
					item.Value, item.BulletsValue = data, nil
				}
			}
			before, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			findings := sourcePreservingPreflightParagraphRepairs([]patterns.FitFinding{{ValidationError: patterns.ValidationError{Code: code, Path: "/slides/0/content/0"}, Action: "refuse"}}, &input)
			after, err := json.Marshal(input)
			if err != nil || string(after) != string(before) {
				t.Fatal("preflight mutated input")
			}
			if findings[0].Action != "refuse" {
				t.Fatal("source-loss refusal weakened")
			}
			envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{}, diagnostics.FromFitFindings(findings))
			data, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			var wire diagnostics.FindingEnvelope
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.OK || len(wire.Findings) != 1 || wire.Findings[0].Remediation == nil {
				t.Fatalf("source-loss wire refusal lost: %+v", wire)
			}
			fix := wire.Findings[0].Remediation.Primary
			if fix == nil || fix.Action != diagnostics.ActionSplitSlide || fix.Params["kind"] != "split_bullets" || fix.Params["max_items"] != float64(2) {
				t.Fatalf("source-preserving wire action missing: %+v", fix)
			}
			result := applyRepairFix(&input, 0, repairFixInput{Kind: fix.Params["kind"].(string), Params: fix.Params})
			if !result.Applied || len(input.Slides) != 2 {
				t.Fatalf("wire split failed: %+v", result)
			}
			for ci, original := range [][]string{left, right} {
				var joined []string
				for _, slide := range input.Slides {
					value, err := slide.Content[ci].ResolveValue()
					if err != nil {
						t.Fatal(err)
					}
					joined = append(joined, value.([]string)...)
				}
				if !reflect.DeepEqual(joined, original) {
					t.Fatalf("wire split lost source: %v", joined)
				}
			}
		}
	}
}

func TestParagraphRepairPreflightUnsupportedSourceHasNoDestructiveFix(t *testing.T) {
	for _, item := range []ContentInput{
		{Type: "text"},
		{Type: "bullets"},
		{Type: "bullets", Value: json.RawMessage(`{"invalid":"list"}`)},
		{Type: "body_and_bullets", BodyAndBulletsValue: &BodyAndBulletsInput{Body: "Required heading", Bullets: []string{"A", "B"}}},
	} {
		input := PresentationInput{Slides: []SlideInput{{Content: []ContentInput{item}}}}
		got := sourcePreservingPreflightParagraphRepairs([]patterns.FitFinding{{ValidationError: patterns.ValidationError{Code: patterns.ErrCodeTextTrimmed, Path: "/slides/0/content/0", Fix: &patterns.FixSuggestion{Kind: "reduce_text"}}}}, &input)
		if got[0].Fix != nil {
			t.Fatalf("unsupported source offers destructive/unproved fix: %+v", got)
		}
	}
	table := &patterns.FixSuggestion{Kind: "split_at_row"}
	got := sourcePreservingPreflightParagraphRepairs([]patterns.FitFinding{{ValidationError: patterns.ValidationError{Code: patterns.ErrCodeTableRowsTruncated, Fix: table}}}, nil)
	if got[0].Fix != table {
		t.Fatal("unrelated table repair changed")
	}
	got = sourcePreservingPreflightParagraphRepairs([]patterns.FitFinding{{ValidationError: patterns.ValidationError{Code: patterns.ErrCodeTextTrimmed}}}, nil)
	if got[0].Fix != nil {
		t.Fatal("nil input invented split")
	}
}

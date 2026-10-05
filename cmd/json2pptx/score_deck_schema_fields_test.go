package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// TestScoreDeckSchemaNamesThePerSlideFields is the go-slide-creator-ano9i
// acceptance test: every key a per_slide entry is marshalled with is a
// property the score_deck output schema declares. The schema said slide_index
// while the response says index, so an agent reading the schema found no
// slide number.
func TestScoreDeckSchemaNamesThePerSlideFields(t *testing.T) {
	var schema struct {
		Properties struct {
			PerSlide struct {
				Items struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"items"`
			} `json:"per_slide"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(outputSchemaScoreDeck, &schema); err != nil {
		t.Fatalf("score_deck output schema: %v", err)
	}
	declared := schema.Properties.PerSlide.Items.Properties
	if len(declared) == 0 {
		t.Fatal("score_deck output schema declares no per_slide item properties")
	}

	raw, err := json.Marshal(deterministic.SlideScore{Findings: []deterministic.ScoreFinding{}})
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatal(err)
	}
	for key := range sent {
		if _, ok := declared[key]; !ok {
			t.Errorf("per_slide entries carry %q, which the output schema does not declare", key)
		}
	}
	for key := range declared {
		if _, ok := sent[key]; !ok {
			t.Errorf("the output schema declares per_slide[].%s, which no entry carries", key)
		}
	}
}

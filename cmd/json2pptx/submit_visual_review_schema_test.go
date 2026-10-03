package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// TestSubmitVisualReviewSlidesAreTyped pins the input schema for slides[].
// The parameter used to be an untyped array with a prose example, so an agent
// that read the schema sent [{index, verdict}] and learned about the image
// requirement from a rejection naming a field ("role") it never had to supply
// (go-slide-creator-g2mc).
func TestSubmitVisualReviewSlidesAreTyped(t *testing.T) {
	tool := mcpSubmitVisualReviewTool()
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal input schema: %v", err)
	}
	var schema struct {
		Properties struct {
			Slides struct {
				Items map[string]any `json:"items"`
			} `json:"slides"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	items := schema.Properties.Slides.Items
	if items == nil {
		t.Fatal("slides has no items schema: an agent cannot tell what an entry needs")
	}

	props, _ := items["properties"].(map[string]any)
	for _, field := range []string{"index", "verdict", "image_path", "image_sha256", "role", "findings"} {
		if _, ok := props[field]; !ok {
			t.Errorf("slides.items is missing the %q property", field)
		}
	}

	required, _ := items["required"].([]any)
	got := make(map[string]bool, len(required))
	for _, r := range required {
		got[r.(string)] = true
	}
	if !got["index"] || !got["verdict"] {
		t.Errorf("slides.items.required = %v, want index and verdict", required)
	}

	// The image is required, but either field satisfies it.
	anyOf, _ := items["anyOf"].([]any)
	if len(anyOf) != 2 {
		t.Fatalf("slides.items.anyOf = %v, want one branch per image field", anyOf)
	}
	branches := make(map[string]bool)
	for _, b := range anyOf {
		m := b.(map[string]any)
		for _, r := range m["required"].([]any) {
			branches[r.(string)] = true
		}
	}
	if !branches["image_path"] || !branches["image_sha256"] {
		t.Errorf("anyOf branches = %v, want image_path and image_sha256", branches)
	}
}

// TestAppendReviewSlidesRequiresAnImage checks the runtime error names the
// field the caller actually has to supply.
func TestAppendReviewSlidesRequiresAnImage(t *testing.T) {
	idx := 0
	record := &visualqa.ReviewRecord{}
	err := appendReviewSlides(record, []visualReviewSlideInput{
		{Index: &idx, Verdict: "approved"},
	}, "host")
	if err == nil {
		t.Fatal("a verdict with no image should be rejected")
	}
	if !errors.Is(err, errVisualReviewRejected) {
		t.Errorf("error should be a review rejection, got %v", err)
	}
	for _, want := range []string{"slides[0]", "image_path", "image_sha256"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "role") {
		t.Errorf("error should not blame role, which defaults: %q", err)
	}

	// An image_sha256 alone is enough — the reviewer may have hashed the PNG
	// rather than kept the file.
	record = &visualqa.ReviewRecord{}
	if err := appendReviewSlides(record, []visualReviewSlideInput{
		{Index: &idx, Verdict: "approved", ImageSHA256: "ABCDEF"},
	}, "host"); err != nil {
		t.Fatalf("image_sha256 alone should be accepted: %v", err)
	}
	if len(record.Slides) != 1 || record.Slides[0].ImageSHA256 != "abcdef" {
		t.Errorf("pixel hash not recorded (lowercased): %+v", record.Slides)
	}
	if record.Slides[0].Role != "slide" {
		t.Errorf("role should default to %q, got %q", "slide", record.Slides[0].Role)
	}
}

// TestSubmitVisualReviewFindingVocabulary pins go-slide-creator-lk37o: the
// findings item schema enumerates every severity and category the review
// accepts, each with a one-line meaning, and a value outside them is rejected
// with the list rather than recorded.
func TestSubmitVisualReviewFindingVocabulary(t *testing.T) {
	finding := visualReviewSlideItemSchema()["properties"].(map[string]any)["findings"].(map[string]any)["items"].(map[string]any)
	props := finding["properties"].(map[string]any)
	for field, vocabulary := range map[string][]visualqa.VocabularyEntry{
		"severity": visualqa.SeverityVocabulary(),
		"category": visualqa.CategoryVocabulary(),
	} {
		schema := props[field].(map[string]any)
		enum := schema["enum"].([]any)
		desc := schema["description"].(string)
		if len(enum) != len(vocabulary) {
			t.Errorf("%s enum has %d values, want %d", field, len(enum), len(vocabulary))
		}
		for i, e := range vocabulary {
			if i < len(enum) && enum[i] != e.Name {
				t.Errorf("%s enum[%d] = %v, want %s", field, i, enum[i], e.Name)
			}
			if e.Meaning == "" || !strings.Contains(desc, e.Name+": "+e.Meaning) {
				t.Errorf("%s description does not give the meaning of %q: %q", field, e.Name, desc)
			}
		}
	}

	idx := 0
	for _, tc := range []struct {
		name    string
		finding visualqa.Finding
		path    string
		lists   string
	}{
		{"unknown category", visualqa.Finding{Severity: visualqa.SeverityP2, Category: "whitespace"}, "slides[0].findings[0].category", "layout_balance"},
		{"unknown severity", visualqa.Finding{Severity: "minor", Category: "spacing"}, "slides[0].findings[0].severity", "P0, P1, P2, P3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := appendReviewSlides(&visualqa.ReviewRecord{}, []visualReviewSlideInput{
				{Index: &idx, Verdict: "approved", ImageSHA256: "abcdef", Findings: []visualqa.Finding{tc.finding}},
			}, "host")
			var rejection *visualReviewRejection
			if !errors.As(err, &rejection) {
				t.Fatalf("want a review rejection, got %v", err)
			}
			if rejection.Path != tc.path {
				t.Errorf("path = %q, want %q", rejection.Path, tc.path)
			}
			if !strings.Contains(rejection.Message, tc.lists) {
				t.Errorf("message does not list the allowed values: %q", rejection.Message)
			}
			if allowed, _ := rejection.Details["allowed"].([]string); len(allowed) == 0 {
				t.Errorf("details.allowed is empty: %v", rejection.Details)
			}
		})
	}

	record := &visualqa.ReviewRecord{}
	if err := appendReviewSlides(record, []visualReviewSlideInput{
		{Index: &idx, Verdict: "changes_requested", ImageSHA256: "abcdef", Findings: []visualqa.Finding{{Severity: visualqa.SeverityP3, Category: "layout_balance"}}},
	}, "host"); err != nil || len(record.Findings) != 1 {
		t.Errorf("a finding inside the vocabulary should be recorded: %v, %+v", err, record.Findings)
	}
}

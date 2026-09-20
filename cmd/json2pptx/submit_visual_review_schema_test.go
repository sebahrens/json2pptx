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

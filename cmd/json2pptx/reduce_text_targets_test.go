package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// gridSlideWithText builds a one-slide deck whose only text lives in a
// shape_grid cell (no content items to reduce).
func gridSlideWithText(t *testing.T, text string) PresentationInput {
	t.Helper()
	var input PresentationInput
	if err := json.Unmarshal([]byte(gridDeck(text)), &input); err != nil {
		t.Fatal(err)
	}
	// Drop the title content item so the slide's text is grid-only.
	input.Slides[0].Content = nil
	return input
}

func bulletSlideInput(bullets []string) PresentationInput {
	return PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID: "slideLayout2",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "bullets",
				BulletsValue:  &bullets,
			}},
		}},
	}
}

// go-slide-creator-9zof: BODY_TOO_LONG's own next_tool_call is
// repair_slide{reduce_text, {current_words, max_words}}. Applied verbatim it
// returned {applied:false, "no text content found to reduce on this slide"} for
// every slide, because reduce_text only honored max_items and max_length.
func TestReduceTextHonorsMaxWordsOnBullets(t *testing.T) {
	long := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta ", 5) // 40 words
	input := bulletSlideInput([]string{long, long, long})

	before := totalWords(*input.Slides[0].Content[0].BulletsValue)
	result := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text", Params: map[string]any{
		"current_words": before,
		"max_words":     30,
	}})
	if !result.Applied {
		t.Fatalf("reduce_text with max_words did not apply: code=%q message=%q", result.Code, result.Message)
	}
	after := totalWords(*input.Slides[0].Content[0].BulletsValue)
	if after >= before {
		t.Errorf("word count %d -> %d: nothing was trimmed", before, after)
	}
	// Every bullet survives: dropping bullets to hit a word budget loses points.
	if got := len(*input.Slides[0].Content[0].BulletsValue); got != 3 {
		t.Errorf("bullets = %d, want all 3 kept (shortened, not dropped)", got)
	}
	for i, b := range *input.Slides[0].Content[0].BulletsValue {
		if !strings.HasSuffix(b, "…") {
			t.Errorf("bullet %d was not marked as truncated: %q", i, b)
		}
		if strings.HasSuffix(strings.TrimSuffix(b, "…"), " ") {
			t.Errorf("bullet %d has a dangling space before the ellipsis: %q", i, b)
		}
	}
}

// A word budget also reaches body_and_bullets and bullet_groups.
func TestReduceTextHonorsMaxWordsOnCompositeBodies(t *testing.T) {
	long := strings.Repeat("alpha beta gamma delta ", 8) // 32 words
	input := PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID: "slideLayout2",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "body_and_bullets", BodyAndBulletsValue: &BodyAndBulletsInput{
					Body:    long,
					Bullets: []string{long, long},
				}},
				{PlaceholderID: "body_2", Type: "bullet_groups", BulletGroupsValue: &BulletGroupsInput{
					Groups: []BulletGroupInput{
						{Header: "One", Bullets: []string{long}},
						{Header: "Two", Bullets: []string{long}},
					},
				}},
			},
		}},
	}
	result := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text", Params: map[string]any{"max_words": 24}})
	if !result.Applied {
		t.Fatalf("not applied: code=%q message=%q", result.Code, result.Message)
	}
	bab := input.Slides[0].Content[0].BodyAndBulletsValue
	if len(strings.Fields(bab.Body)) > 24 {
		t.Errorf("body_and_bullets body still %d words", len(strings.Fields(bab.Body)))
	}
	if totalWords(bab.Bullets) >= 64 {
		t.Errorf("body_and_bullets bullets were not trimmed (%d words)", totalWords(bab.Bullets))
	}
	bg := input.Slides[0].Content[1].BulletGroupsValue
	if len(bg.Groups) != 2 {
		t.Errorf("bullet groups = %d, want both kept — a word budget must not delete a heading and its points", len(bg.Groups))
	}
	for i, g := range bg.Groups {
		if totalWords(g.Bullets) >= 32 {
			t.Errorf("group %d bullets were not trimmed (%d words)", i, totalWords(g.Bullets))
		}
	}
}

// The protected-fact guard still governs: trimming may not silently delete a
// number, unit, or negation.
func TestReduceTextRefusesWhenTrimWouldDropFacts(t *testing.T) {
	input := bulletSlideInput([]string{
		"Revenue did not grow in FY24 despite a 12% increase in marketing spend across every region we operate in today",
	})
	result := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text", Params: map[string]any{"max_words": 6}})
	if result.Applied {
		t.Fatal("trimming away '12%' / 'not' must not apply silently")
	}
	if result.Code != "semantic_review_required" {
		t.Errorf("code = %q, want semantic_review_required", result.Code)
	}
	// confirm_semantic_change is the documented deliberate override.
	confirmed := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text", Params: map[string]any{
		"max_words":               6,
		"confirm_semantic_change": true,
	}})
	if !confirmed.Applied {
		t.Errorf("confirmed trim did not apply: %q", confirmed.Message)
	}
}

// A directive with no budget at all used to report "no text content found",
// blaming the deck for a malformed call.
func TestReduceTextReportsMissingBudget(t *testing.T) {
	input := bulletSlideInput([]string{"one two three four five six"})
	result := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text"})
	if result.Applied {
		t.Fatal("a budget-less directive must not apply")
	}
	if !strings.Contains(result.Message, "max_words") {
		t.Errorf("message should name the missing budget params, got %q", result.Message)
	}
}

// On a shape_grid slide reduce_text cannot reach the text at all. Instead of a
// bare failure, the answer names the kind that can and hands back a ready
// directive.
func TestReduceTextOnGridSlideSuggestsReduceCellText(t *testing.T) {
	input := gridSlideWithText(t, "a very long cell text that overruns its box by a wide margin")

	result := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_text", Params: map[string]any{
		"path":      "/slides/0/shape_grid/rows/0/cells/0/shape/text",
		"max_chars": 30,
	}})
	if result.Applied {
		t.Fatal("reduce_text cannot edit a grid cell; it must not claim to")
	}
	if result.Code != "wrong_kind_for_target" {
		t.Errorf("code = %q, want wrong_kind_for_target", result.Code)
	}
	if result.DidYouMean != "reduce_cell_text" {
		t.Errorf("did_you_mean = %q, want reduce_cell_text", result.DidYouMean)
	}
	if result.NextToolCall == nil || result.NextToolCall.Tool != "repair_slide" {
		t.Fatalf("next_tool_call = %+v, want a repair_slide directive", result.NextToolCall)
	}
	fixes, _ := result.NextToolCall.ArgsTemplate["fixes"].([]any)
	if len(fixes) != 1 {
		t.Fatalf("next_tool_call carries %d fixes", len(fixes))
	}
	directive, _ := fixes[0].(map[string]any)
	params, _ := directive["params"].(map[string]any)
	if directive["kind"] != "reduce_cell_text" || params["cell_path"] != "/slides/0/shape_grid/rows/0/cells/0" {
		t.Errorf("suggested directive = %+v", directive)
	}
	if params["max_chars"] != 30 {
		t.Errorf("the character budget must survive the correction: %+v", params)
	}

	// And the suggested directive actually applies.
	applied := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_cell_text", Params: map[string]any{
		"cell_path": params["cell_path"],
		"max_chars": params["max_chars"],
	}})
	if !applied.Applied {
		t.Errorf("the suggested directive did not apply: %q", applied.Message)
	}
}

// propose_repairs must emit the reachable kind for grid-cell findings, which is
// how 60 of 60 directives came to apply nothing.
func TestProposeRepairsRetargetsGridCellText(t *testing.T) {
	input := gridSlideWithText(t, "a great deal of prose for one small cell to carry")
	findings := []proposeRepairsFinding{{
		Code:    "fit_overflow",
		Path:    "/slides/0/shape_grid/rows/0/cells/0/shape/text",
		Action:  "refuse",
		Message: "text needs 120 chars @ 12pt; cell allows 40",
		Fix:     &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_chars": 40}},
	}}
	out := proposeRepairs(&input, findings)
	if len(out.Slides) != 1 || len(out.Slides[0].Directives) != 1 {
		t.Fatalf("expected one directive, got %+v", out.Slides)
	}
	d := out.Slides[0].Directives[0]
	if d.Kind != "reduce_cell_text" {
		t.Errorf("directive kind = %q, want reduce_cell_text", d.Kind)
	}
	if got := d.Params["cell_path"]; got != "/slides/0/shape_grid/rows/0/cells/0" {
		t.Errorf("cell_path = %v", got)
	}
	if got := d.Params["max_chars"]; got != 40 {
		t.Errorf("max_chars = %v, want the finding's budget", got)
	}
}

func totalWords(items []string) int {
	n := 0
	for _, it := range items {
		n += len(strings.Fields(it))
	}
	return n
}

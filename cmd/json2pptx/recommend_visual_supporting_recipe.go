package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// A spectrum is a supporting axis, so its recommendation includes the argument
// the axis supports. The standalone semantic kind remains available to authors.
func attachSpectrumSupportingRecipe(c *patterns.VisualCandidate, templateName string, reg *patterns.Registry) bool {
	p, ok := reg.Get(c.Name)
	if !ok {
		return false
	}
	fit := patterns.SegmentFitFor(p)
	if !fit.SegmentOnly {
		return false
	}
	values, contract, err := patternRegionRecipe(reg, c.Name)
	if err != nil {
		return false
	}
	contract.Form = patterns.ContractFormRaw
	contract.FieldPath = recipeSlidePath + ".slide.compose.segments[0].pattern.values"
	contract.Description = "Supporting band: replace these spectrum values and the companion text at slides[0].slide.compose.segments[1].text; keep the two-pane composition. " + contract.Description
	c.DataContract = contract
	c.DeckSpec = deckSpecFormFor(semantic.KindRawJSON2pptx, "The supporting spectrum sits beside its argument in raw compose; the standalone spectrum kind remains available.")
	c.SegmentFit = fit
	slide := map[string]any{"kind": "raw_json2pptx", "slide": map[string]any{
		"layout_id": composeRecipeLayoutID,
		"content":   []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": placeholder.RecipeActionTitle("spectrum and argument")}},
		"compose": map[string]any{"direction": "horizontal", "gap": 20, "segments": []any{
			map[string]any{"size_pct": 40, "pattern": map[string]any{"name": c.Name, "values": values}},
			map[string]any{"size_pct": 60, "heading": "Decision implications", "text": map[string]any{
				"body": "Move routine approval decisions to accountable teams, while keeping risk standards and exception review central.",
				"bullets": []any{
					"Today, a central team reviews every change and creates a queue before work can start.",
					"Teams should decide within published limits and record the evidence behind each decision.",
					"Central assurance keeps ownership of policy, monitoring and exceptions above those limits.",
					"Pilot one workstream, measure turnaround time and review exceptions before scaling.",
				}}},
		}},
		"source": placeholder.RecipeSampleSource,
	}}
	c.NextToolCall = &patterns.ToolCallSuggestion{Tool: "render_deck_spec", ArgsTemplate: map[string]any{"spec": recipeSpec(strings.ReplaceAll(c.Name, "-", " "), templateName, slide)}}
	return true
}

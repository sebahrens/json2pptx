package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// routeImageHeroFields keeps every whole-slide attribution and conclusion on
// the same measured scrim as the headline, for raw and semantic authoring.
func routeImageHeroFields(input *PresentationInput) {
	for i := range input.Slides {
		slide := &input.Slides[i]
		if slide.Pattern == nil || slide.Pattern.Name != "image-hero" {
			continue
		}
		var values map[string]any
		if json.Unmarshal(slide.Pattern.Values, &values) != nil || values == nil {
			continue
		}
		if v, set := values["source"]; set {
			if _, ok := v.(string); !ok {
				continue
			}
		}
		if v, set := values["takeaway"]; set {
			if _, ok := v.(string); !ok {
				continue
			}
		}
		source, _ := values["source"].(string)
		source = patterns.MergeSourceNotes(source, slide.Source)
		if strings.TrimSpace(source) == "" {
			source = input.Source
		}
		takeaway, _ := values["takeaway"].(string)
		if authored := strings.TrimSpace(slide.Takeaway); authored != "" && authored != strings.TrimSpace(takeaway) {
			if takeaway != "" {
				takeaway += "\n"
			}
			takeaway += authored
		}
		if source != "" {
			values["source"] = source
		}
		if takeaway != "" {
			values["takeaway"] = takeaway
		}
		raw, err := json.Marshal(values)
		if err != nil {
			continue
		}
		slide.Pattern.Values = raw
		slide.Source = ""
		slide.Takeaway = ""
	}
}

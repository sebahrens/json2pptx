package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// CompileConsultingVisual emits the exact registered native pattern, preserving
// its closed field contract and measured refusal rather than degrading to text.
func CompileConsultingVisual(in Input, name string, values map[string]any) (*deckinput.SlideInput, []SourceLink, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s values: %w", name, err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title", Pattern: &deckinput.PatternInput{Name: name, Values: raw}}
	var links []SourceLink
	if name == "image-hero" {
		slide.LayoutID = "blank"
		slide.SlideType = "title"
		slide.Pattern.Bounds = &deckinput.GridBoundsInput{Width: 100, Height: 100}
	} else {
		links = titleLink(slide, in)
	}
	for key := range values {
		links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values." + key, SemanticPath: in.semSlide() + "." + key})
	}
	if name != "image-hero" {
		links = append(links, applyTakeaway(slide, in)...)
	}
	return slide, links, nil
}

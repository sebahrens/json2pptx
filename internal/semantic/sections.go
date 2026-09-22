package semantic

import (
	"encoding/json"
	"fmt"
)

type sourcedSlide struct {
	Slide        SlideSpec
	SourcePath   string
	SectionTitle string
}

// expandedSlides lowers chapter authoring into the same semantic slide stream
// the flat form uses. Generated agenda/divider slides retain paths to the
// structure fields that created them.
func expandedSlides(spec *DeckSpec) []sourcedSlide {
	if spec == nil {
		return nil
	}
	if spec.Structure == nil {
		out := make([]sourcedSlide, 0, len(spec.Slides))
		for i, slide := range spec.Slides {
			out = append(out, sourcedSlide{Slide: slide, SourcePath: fmt.Sprintf("slides[%d]", i)})
		}
		return out
	}
	st := spec.Structure
	var out []sourcedSlide
	if st.Cover != nil {
		out = append(out, sourcedSlide{Slide: *st.Cover, SourcePath: "structure.cover"})
	}
	if st.AutoAgenda && len(st.Sections) >= 2 {
		items := make([]any, 0, len(st.Sections))
		for _, section := range st.Sections {
			items = append(items, section.Title)
		}
		out = append(out, sourcedSlide{
			Slide:      SlideSpec{Kind: KindAgenda, Body: map[string]any{"title": "Agenda", "sections": items}},
			SourcePath: "structure.sections",
		})
	}
	for sectionIndex, section := range st.Sections {
		sectionPath := fmt.Sprintf("structure.sections[%d]", sectionIndex)
		out = append(out, sourcedSlide{
			Slide:      SlideSpec{Kind: KindSection, Body: map[string]any{"title": section.Title}},
			SourcePath: sectionPath,
		})
		for slideIndex, slide := range section.Slides {
			out = append(out, sourcedSlide{
				Slide: slide, SourcePath: fmt.Sprintf("%s.slides[%d]", sectionPath, slideIndex),
				SectionTitle: section.Title,
			})
		}
	}
	if st.Closing != nil {
		out = append(out, sourcedSlide{Slide: *st.Closing, SourcePath: "structure.closing"})
	}
	return out
}

// ExpandedSlidePayloads returns one stable semantic payload for every slide
// produced by the flat or structured authoring form. Generated agenda and
// divider slides are included, so render metadata has the same cardinality as
// the compiled presentation.
func ExpandedSlidePayloads(spec *DeckSpec) ([]json.RawMessage, error) {
	expanded := expandedSlides(spec)
	out := make([]json.RawMessage, len(expanded))
	for i := range expanded {
		payload, err := json.Marshal(expanded[i].Slide)
		if err != nil {
			return nil, fmt.Errorf("marshal expanded slide %d: %w", i, err)
		}
		out[i] = payload
	}
	return out, nil
}

// ExpandedSlideCount reports the number of slides the semantic structure
// produces without compiling or rendering it.
func ExpandedSlideCount(spec *DeckSpec) int { return len(expandedSlides(spec)) }

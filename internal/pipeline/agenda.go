package pipeline

import "github.com/sebahrens/json2pptx/internal/types"

// GenerateAgenda creates an agenda slide from section divider titles and inserts
// it after the title slide (index 0). If there are fewer than 2 sections, no
// agenda is generated because a single-section TOC adds no value.
//
// The generated slide uses SlideTypeContent with bullet points listing each
// section name. The rest of the pipeline handles layout selection and rendering
// as with any other content slide.
func GenerateAgenda(presentation *types.PresentationDefinition) {
	sections := extractSections(presentation.Slides)
	if len(sections) < 2 {
		return
	}

	agenda := types.SlideDefinition{
		Title: "Agenda",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: sections,
		},
	}

	// Insert after the title slide (position 1). If presentation has no slides
	// (unlikely), just prepend.
	insertAt := 0
	if len(presentation.Slides) > 0 {
		insertAt = 1
	}

	// Grow slice and insert
	slides := make([]types.SlideDefinition, 0, len(presentation.Slides)+1)
	slides = append(slides, presentation.Slides[:insertAt]...)
	slides = append(slides, agenda)
	slides = append(slides, presentation.Slides[insertAt:]...)

	// Reindex all slides
	for i := range slides {
		slides[i].Index = i
	}

	presentation.Slides = slides
}

// extractSections returns the titles of all section divider slides and any
// trailing title slides (e.g. "Discussion", "Q&A", "Thank You"). The first
// title slide is the opening/cover slide and is excluded; subsequent title
// slides act as section boundaries in consulting presentations.
func extractSections(slides []types.SlideDefinition) []string {
	var sections []string
	seenFirstTitle := false
	for _, slide := range slides {
		if slide.Type == types.SlideTypeTitle {
			if seenFirstTitle && slide.Title != "" {
				sections = append(sections, slide.Title)
			}
			seenFirstTitle = true
			continue
		}
		if slide.Type == types.SlideTypeSection && slide.Title != "" {
			sections = append(sections, slide.Title)
		}
	}
	return sections
}

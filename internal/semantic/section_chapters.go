package semantic

import "github.com/sebahrens/json2pptx/internal/diagnostics"

func validateSectionChapters(spec *DeckSpec, s *semDiags) {
	stream := expandedSlides(spec)
	count := 0
	for _, entry := range stream {
		appendix, _ := entry.Slide.Body["appendix"].(bool)
		if entry.Slide.Kind == KindSection && !appendix {
			count++
		}
	}
	for _, entry := range stream {
		show, _ := entry.Slide.Body["show_chapters"].(bool)
		if !show || entry.Slide.Kind != KindSection {
			continue
		}
		appendix, _ := entry.Slide.Body["appendix"].(bool)
		if count < 2 || count > 10 || appendix {
			s.hard(entry.SourcePath+".show_chapters", diagnostics.CodeSemanticDensity, "show_chapters requires 2–10 main chapters; appendix dividers are outside that tracker")
		}
	}
}

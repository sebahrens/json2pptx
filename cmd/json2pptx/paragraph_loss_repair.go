package main

import (
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Enrich only from authored bullet values, never from flattened fitted text.
// No media loading, layout conversion or input mutation is needed for this
// structural split proof. Non-bullet types stay present to reject compound
// sources that the plain-bullet repair cannot preserve safely.
func sourcePreservingPreflightParagraphRepairs(findings []patterns.FitFinding, input *PresentationInput) []patterns.FitFinding {
	var sources []generator.SlideSpec
	for i := range findings {
		if findings[i].Code != patterns.ErrCodeTextTrimmed && findings[i].Code != patterns.ErrCodeReadabilityTrimmed {
			continue
		}
		if sources == nil {
			sources = paragraphRepairSources(input)
		}
		findings[i].ValidationError = generator.SourcePreservingParagraphRepair(findings[i].ValidationError, sources)
	}
	return findings
}

func paragraphRepairSources(input *PresentationInput) []generator.SlideSpec {
	if input == nil {
		return nil
	}
	sources := make([]generator.SlideSpec, len(input.Slides))
	for si, slide := range input.Slides {
		for _, item := range slide.Content {
			content := generator.ContentItem{Type: generator.ContentType(item.Type)}
			if item.Type == "bullets" {
				if value, err := item.ResolveValue(); err == nil {
					content.Value = value
				}
			}
			sources[si].Content = append(sources[si].Content, content)
		}
	}
	return sources
}

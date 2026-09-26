package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

func authoredNativeImageFrames(slide *SlideInput, layout *types.LayoutMetadata) []types.BoundingBox {
	var frames []types.BoundingBox
	for _, content := range slide.Content {
		if content.Type != "image" {
			continue
		}
		image := content.ImageValue
		if image == nil && len(content.Value) > 0 {
			var legacy ImageInput
			if json.Unmarshal(content.Value, &legacy) != nil {
				continue
			}
			image = &legacy
		}
		if image == nil || (strings.TrimSpace(image.Path) == "" && strings.TrimSpace(image.URL) == "") || generator.ValidateImageFit(image.Fit) != nil {
			continue
		}
		if ph := findContrastPlaceholderByID(content.PlaceholderID, layout); ph != nil {
			frames = append(frames, ph.Bounds)
		}
	}
	return frames
}

func collectNativeImageContrastFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	sectionNumbers := contrastSectionNumbers(input, layouts)
	var findings []patterns.FitFinding
	for si := range input.Slides {
		slide := &input.Slides[si]
		layout := predicted[si]
		if layout == nil || (slide.ContrastCheck != nil && !*slide.ContrastCheck) {
			continue
		}
		frames := authoredNativeImageFrames(slide, layout)
		for ci, content := range injectSectionNumber(slide.Content, layout, sectionNumbers[si]) {
			if !nativeImageTextPopulated(&content) {
				continue
			}
			ph := findContrastPlaceholderByID(content.PlaceholderID, layout)
			if ph == nil || !generator.NativeImageOverlapsText(*ph, frames) {
				continue
			}
			path := slidepath.ContentIndex(si, ci)
			if ci >= len(slide.Content) {
				path = slidepath.Content(si, ph.ID)
			}
			findings = append(findings, generator.NativeImageContrastFinding(path, content.PlaceholderID))
		}
	}
	return findings
}

// ResolveValue preserves typed-field precedence and legacy input parity without
// fetching media. Only text-bearing content participates in this diagnostic.
func nativeImageTextPopulated(content *ContentInput) bool {
	var paragraphs []string
	value, err := content.ResolveValue()
	if err != nil {
		return false
	}
	switch content.Type {
	case "text":
		if text, ok := value.(string); ok {
			paragraphs = []string{text}
		}
	case "bullets":
		paragraphs, _ = value.([]string)
	case "body_and_bullets":
		if body, ok := value.(*BodyAndBulletsInput); ok && body != nil {
			paragraphs = append([]string{body.Body, body.TrailingBody}, body.Bullets...)
		}
	case "body_and_lead":
		if body, ok := value.(*BodyAndLeadInput); ok && body != nil {
			paragraphs = append([]string{body.Lead}, body.Bullets...)
		}
	case "bullet_groups":
		if body, ok := value.(*BulletGroupsInput); ok && body != nil {
			paragraphs = []string{body.Body, body.TrailingBody}
			for _, group := range body.Groups {
				paragraphs = append(paragraphs, group.Header)
				paragraphs = append(paragraphs, group.Bullets...)
			}
		}
	}
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) != "" {
			return true
		}
	}
	return false
}

package generator

import (
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/types"
)

// BuildContentItems creates content items for the generator based on layout content mappings.
// This function converts a SlideDefinition and its content mappings into a slice of
// ContentItem structs suitable for the generator.
//
// The function handles:
//   - Title, body, and bullet text content
//   - Combined body+bullets content when both map to the same placeholder
//   - Left/right column bullets for comparison layouts
//   - Image and chart content
func BuildContentItems(slide types.SlideDefinition, mappings []layout.ContentMapping) []ContentItem {
	items := make([]ContentItem, 0)

	// Track which placeholders have body mappings so we can combine body+bullets
	bodyMappings := make(map[string]string) // placeholderID -> body text
	for _, mapping := range mappings {
		if mapping.ContentField == "body" && slide.Content.Body != "" {
			bodyMappings[mapping.PlaceholderID] = slide.Content.Body
		}
	}

	// Detect title+body sharing a placeholder (e.g., closing slides where subtitle
	// placeholder is too small and body is routed to title placeholder instead).
	titleBodyShared := make(map[string]bool) // placeholderID -> true if title+body share it
	{
		titlePH := ""
		bodyPH := ""
		for _, m := range mappings {
			if m.ContentField == "title" {
				titlePH = m.PlaceholderID
			}
			if m.ContentField == "body" {
				bodyPH = m.PlaceholderID
			}
		}
		if titlePH != "" && titlePH == bodyPH {
			titleBodyShared[titlePH] = true
		}
	}

	for _, mapping := range mappings {
		var item ContentItem
		item.PlaceholderID = mapping.PlaceholderID

		switch mapping.ContentField {
		case "title":
			// Section divider titles use ContentSectionTitle so the generator
			// preserves the template's large font size instead of capping it
			// to 24pt. normAutofit scales the text to fill the placeholder.
			// Title slide titles use ContentTitleSlideTitle to preserve the
			// template's large ctrTitle font size, centered alignment, and
			// bold styling instead of capping to 24pt body-text size.
			if slide.Type == types.SlideTypeSection {
				item.Type = ContentSectionTitle
			} else if slide.Type == types.SlideTypeTitle {
				item.Type = ContentTitleSlideTitle
			} else {
				item.Type = ContentText
			}
			// When title and body share a placeholder (closing slides with
			// small subtitle), combine them so both render in the title area.
			if titleBodyShared[mapping.PlaceholderID] && slide.Content.Body != "" {
				item.Value = slide.Title + "\n" + slide.Content.Body
			} else {
				item.Value = slide.Title
			}

		case "body":
			// Skip body when it shares a placeholder with title — already combined above.
			if titleBodyShared[mapping.PlaceholderID] {
				continue
			}
			// If the slide has a pre-parsed standalone table, route it as ContentTable
			// instead of text. This handles content slides where the only
			// body content is a markdown table.
			if slide.Content.Table != nil {
				item.Type = ContentTable
				item.Value = slide.Content.Table
				break
			}
			// Skip body if bullets exist for the same placeholder - will be combined below
			if len(slide.Content.Bullets) > 0 {
				// Check if bullets map to the same placeholder
				skipBody := false
				for _, m := range mappings {
					if m.ContentField == "bullets" && m.PlaceholderID == mapping.PlaceholderID {
						skipBody = true
						break
					}
				}
				if skipBody {
					continue // Skip body, it will be prepended to bullets
				}
			}
			// Section divider body uses ContentSectionTitle to preserve
			// the template's large decorative font (e.g., 96pt "#" placeholder).
			// The pipeline populates this with a section number like "01".
			if slide.Type == types.SlideTypeSection {
				item.Type = ContentSectionTitle
			} else {
				item.Type = ContentText
			}
			item.Value = slide.Content.Body

		case "bullets":
			// Prefer bullet groups if available (hierarchical structure)
			if len(slide.Content.BulletGroups) > 0 {
				item.Type = ContentBulletGroups
				// Include body text if it exists and maps to the same placeholder
				bodyText := ""
				if body, hasBody := bodyMappings[mapping.PlaceholderID]; hasBody {
					bodyText = body
				}
				item.Value = BulletGroupsContent{
					Body:         bodyText,
					Groups:       slide.Content.BulletGroups,
					TrailingBody: slide.Content.BodyAfterBullets,
				}
			} else if len(slide.Content.Bullets) > 0 {
				// Check if there's body text for the same placeholder - use combined type
				if bodyText, hasBody := bodyMappings[mapping.PlaceholderID]; hasBody && bodyText != "" {
					item.Type = ContentBodyAndBullets
					item.Value = BodyAndBulletsContent{
						Body:         bodyText,
						Bullets:      slide.Content.Bullets,
						TrailingBody: slide.Content.BodyAfterBullets,
					}
				} else {
					item.Type = ContentBullets
					item.Value = slide.Content.Bullets
				}
			} else if len(slide.Content.Left) > 0 || len(slide.Content.Right) > 0 {
				// Fallback: two-column slide mapped to a single-placeholder layout.
				// Merge left/right columns into a single bullet list.
				combined := make([]string, 0, len(slide.Content.Left)+len(slide.Content.Right))
				combined = append(combined, slide.Content.Left...)
				combined = append(combined, slide.Content.Right...)
				item.Type = ContentBullets
				item.Value = combined
			}

		case "body_and_bullets":
			// Combined body text and bullets in a single mapping (from heuristic)
			item.Type = ContentBodyAndBullets
			item.Value = BodyAndBulletsContent{
				Body:         slide.Content.Body,
				Bullets:      slide.Content.Bullets,
				TrailingBody: slide.Content.BodyAfterBullets,
			}

		case "left":
			if len(slide.Content.Left) > 0 {
				item.Type = ContentBullets
				item.Value = slide.Content.Left
			}

		case "right":
			if len(slide.Content.Right) > 0 {
				item.Type = ContentBullets
				item.Value = slide.Content.Right
			}

		case "image":
			if slide.Content.ImagePath != "" {
				item.Type = ContentImage
				item.Value = ImageContent{
					Path: slide.Content.ImagePath,
					Alt:  "Slide image",
				}
			}

		case "chart", "infographic", "diagram":
			if slide.Content.DiagramSpec != nil {
				item.Type = ContentDiagram
				item.Value = slide.Content.DiagramSpec
			}

		default:
			// Skip unknown content fields
			continue
		}

		// Only add item if value is not empty
		if item.Value != nil {
			items = append(items, item)
		}
	}

	return items
}


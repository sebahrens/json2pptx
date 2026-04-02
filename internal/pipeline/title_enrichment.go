package pipeline

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// EnrichTitleSlides populates subtitle areas on title slides using
// frontmatter metadata (subtitle, author, date). This prevents title slides
// from rendering as sparse, empty layouts when the markdown only specifies
// a heading without body text.
//
// Behavior:
//   - Empty body: uses explicit subtitle field, or composes "author | date".
//   - Non-empty body: appends " | date" when date is in metadata but not
//     already present in the body text, ensuring the date is always visible.
//
// Priority: explicit subtitle field > composed author/date line.
func EnrichTitleSlides(pres *types.PresentationDefinition) {
	meta := pres.Metadata

	// Build the fallback subtitle from author and/or date
	var fallbackParts []string
	if meta.Author != "" {
		fallbackParts = append(fallbackParts, meta.Author)
	}
	if meta.Date != "" {
		fallbackParts = append(fallbackParts, meta.Date)
	}
	fallbackSubtitle := strings.Join(fallbackParts, " | ")

	// Determine which subtitle to use for empty-body slides
	subtitle := meta.Subtitle
	if subtitle == "" {
		subtitle = fallbackSubtitle
	}

	for i := range pres.Slides {
		slide := &pres.Slides[i]
		if slide.Type != types.SlideTypeTitle {
			continue
		}

		if slide.Content.Body == "" {
			// Empty body: inject the full subtitle
			if subtitle != "" {
				slide.Content.Body = subtitle
			}
			continue
		}

		// Non-empty body: append date if present in metadata but missing
		// from the existing body text.
		if meta.Date != "" && !strings.Contains(slide.Content.Body, meta.Date) {
			slide.Content.Body = slide.Content.Body + " | " + meta.Date
		}
	}
}

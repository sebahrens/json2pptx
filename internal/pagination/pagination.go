// Package pagination splits overflowing slides into continuation slides.
// When autopaginate is enabled in frontmatter, slides with more bullets
// than the template's layout capacity are split at natural break points
// (between bullet groups when available, otherwise at the threshold boundary).
// Continuation slides receive a title suffix like "Market Analysis (1/3)".
package pagination

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// DefaultMaxBullets is the fallback threshold when no template layout
// capacity is available.
const DefaultMaxBullets = 8

// PaginateWithLayouts splits overflowing slides using the template's actual
// layout capacity to determine the threshold. It finds the highest MaxBullets
// across all text-capable layouts and uses that as the split point.
// Falls back to DefaultMaxBullets if no layout capacity is available.
// Returns warnings and any fit findings (e.g. when using the default threshold).
func PaginateWithLayouts(pres *types.PresentationDefinition, layouts []types.LayoutMetadata) ([]string, []patterns.FitFinding) {
	threshold := effectiveMaxBullets(layouts)
	warnings := paginateWithThreshold(pres, threshold)

	// Site 9: emit hint when using default threshold (no template capacity).
	var findings []patterns.FitFinding
	if threshold == DefaultMaxBullets && usedDefaultThreshold(layouts) && len(warnings) > 0 {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Code:    patterns.ErrCodePaginationDefault,
				Message: fmt.Sprintf("pagination using default threshold of %d bullets (no template layout capacity available)", DefaultMaxBullets),
			},
			Action: "info",
		})
	}

	return warnings, findings
}

// usedDefaultThreshold returns true when no layout provided bullet capacity.
func usedDefaultThreshold(layouts []types.LayoutMetadata) bool {
	for _, l := range layouts {
		if !hasTextPlaceholder(l) {
			continue
		}
		if l.Capacity.MaxBullets > 0 {
			return false
		}
	}
	return true
}

// Paginate splits overflowing slides using the default threshold of 8 bullets.
// Prefer PaginateWithLayouts when template layouts are available.
func Paginate(pres *types.PresentationDefinition) []string {
	return paginateWithThreshold(pres, DefaultMaxBullets)
}

// paginateWithThreshold is the core pagination engine.
func paginateWithThreshold(pres *types.PresentationDefinition, maxBullets int) []string {
	if !pres.Metadata.AutopaginateEnabled() {
		return nil
	}

	var warnings []string
	var result []types.SlideDefinition

	for _, slide := range pres.Slides {
		if !shouldPaginate(slide, maxBullets) {
			result = append(result, slide)
			continue
		}

		pages := splitSlide(slide, maxBullets)
		if len(pages) > 1 {
			warnings = append(warnings, fmt.Sprintf(
				"slide %q auto-paginated into %d slides (threshold: %d bullets)",
				slide.Title, len(pages), maxBullets,
			))
		}
		result = append(result, pages...)
	}

	// Re-index slides
	for i := range result {
		result[i].Index = i
	}

	pres.Slides = result
	return warnings
}

// shouldPaginate returns true if the slide is a candidate for splitting.
func shouldPaginate(slide types.SlideDefinition, maxBullets int) bool {
	// Only paginate content slides with bullets
	totalBullets := countBullets(slide)
	if totalBullets <= maxBullets {
		return false
	}

	// Don't paginate slides with visual content
	if slide.Content.DiagramSpec != nil || slide.Content.ImagePath != "" {
		return false
	}

	// Don't paginate two-column or slot-based slides
	if len(slide.Content.Left) > 0 || len(slide.Content.Right) > 0 {
		return false
	}
	if slide.HasSlots() {
		return false
	}

	return true
}

// countBullets returns the total number of bullets in a slide.
func countBullets(slide types.SlideDefinition) int {
	return len(slide.Content.Bullets)
}

// splitSlide splits a slide into multiple continuation slides.
// It respects BulletGroup boundaries when available.
func splitSlide(slide types.SlideDefinition, maxBullets int) []types.SlideDefinition {
	if len(slide.Content.BulletGroups) > 0 {
		return splitByGroups(slide, maxBullets)
	}
	return splitByCount(slide, maxBullets)
}

// splitByGroups splits slides respecting BulletGroup boundaries.
// Each page gets as many groups as fit within the bullet limit.
func splitByGroups(slide types.SlideDefinition, maxBullets int) []types.SlideDefinition {
	groups := slide.Content.BulletGroups
	var pages [][]types.BulletGroup

	var currentPage []types.BulletGroup
	currentCount := 0

	for _, group := range groups {
		groupSize := len(group.Bullets)
		if groupSize == 0 {
			groupSize = 1 // header-only group counts as 1
		}

		// If adding this group would overflow AND we already have content,
		// start a new page
		if currentCount+groupSize > maxBullets && len(currentPage) > 0 {
			pages = append(pages, currentPage)
			currentPage = nil
			currentCount = 0
		}

		currentPage = append(currentPage, group)
		currentCount += groupSize
	}

	if len(currentPage) > 0 {
		pages = append(pages, currentPage)
	}

	if len(pages) <= 1 {
		return []types.SlideDefinition{slide}
	}

	return buildPagesFromGroups(slide, pages)
}

// splitByCount splits flat bullet lists at the threshold boundary.
func splitByCount(slide types.SlideDefinition, maxBullets int) []types.SlideDefinition {
	bullets := slide.Content.Bullets
	if len(bullets) <= maxBullets {
		return []types.SlideDefinition{slide}
	}

	var pages [][]string
	for i := 0; i < len(bullets); i += maxBullets {
		end := i + maxBullets
		if end > len(bullets) {
			end = len(bullets)
		}
		pages = append(pages, bullets[i:end])
	}

	return buildPagesFromBullets(slide, pages)
}

// buildPagesFromGroups creates continuation slides from grouped bullet pages.
func buildPagesFromGroups(original types.SlideDefinition, pages [][]types.BulletGroup) []types.SlideDefinition {
	total := len(pages)
	result := make([]types.SlideDefinition, total)

	for i, groups := range pages {
		s := types.SlideDefinition{
			SourceLine: original.SourceLine,
			Title:      paginatedTitle(original.Title, i+1, total),
			Type:       original.Type,
			RawContent: original.RawContent,
		}

		// First page gets body text; continuation pages don't
		if i == 0 {
			s.Content.Body = original.Content.Body
			s.SpeakerNotes = original.SpeakerNotes
			s.Source = original.Source
		}

		s.Content.BulletGroups = groups
		// Populate flat bullets for backward compatibility
		for _, g := range groups {
			s.Content.Bullets = append(s.Content.Bullets, g.Bullets...)
		}

		result[i] = s
	}

	return result
}

// buildPagesFromBullets creates continuation slides from flat bullet pages.
func buildPagesFromBullets(original types.SlideDefinition, pages [][]string) []types.SlideDefinition {
	total := len(pages)
	result := make([]types.SlideDefinition, total)

	for i, bullets := range pages {
		s := types.SlideDefinition{
			SourceLine: original.SourceLine,
			Title:      paginatedTitle(original.Title, i+1, total),
			Type:       original.Type,
			RawContent: original.RawContent,
		}

		// First page gets body text; continuation pages don't
		if i == 0 {
			s.Content.Body = original.Content.Body
			s.SpeakerNotes = original.SpeakerNotes
			s.Source = original.Source
		}

		s.Content.Bullets = bullets
		result[i] = s
	}

	return result
}

// paginatedTitle adds a page suffix to the slide title.
// Example: "Market Analysis" -> "Market Analysis (1/3)"
func paginatedTitle(title string, page, total int) string {
	if total <= 1 {
		return title
	}
	return fmt.Sprintf("%s (%d/%d)", title, page, total)
}

// effectiveMaxBullets computes the best bullet threshold from template layouts.
// It returns the highest MaxBullets across all layouts that have body/content
// placeholders (i.e., layouts that could be selected for bullet content).
// Returns DefaultMaxBullets if no layout has capacity info.
func effectiveMaxBullets(layouts []types.LayoutMetadata) int {
	best := 0
	for _, l := range layouts {
		// Only consider layouts with body/content placeholders
		if !hasTextPlaceholder(l) {
			continue
		}
		if l.Capacity.MaxBullets > best {
			best = l.Capacity.MaxBullets
		}
	}
	if best > 0 && best < DefaultMaxBullets {
		return best
	}
	return DefaultMaxBullets
}

// hasTextPlaceholder returns true if the layout has a body or content placeholder.
func hasTextPlaceholder(layout types.LayoutMetadata) bool {
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			return true
		}
	}
	return false
}

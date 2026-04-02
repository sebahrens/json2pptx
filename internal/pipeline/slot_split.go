// slot_split.go splits slides whose content cannot fit in any single template
// layout. This prevents silent content loss when slot-based slides (e.g.,
// ::slot1:: text + ::slot2:: chart) are placed on layouts with fewer content
// placeholders than slots.
//
// Two cases are handled:
//  1. Slot overflow: slide has N slots but max layout capacity is M (M < N).
//  2. Chart+body collision: standard-path slide has both Body text and
//     DiagramSpec, but no template layout has a dedicated chart placeholder.
package pipeline

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/sebahrens/json2pptx/internal/types"
)

// SplitContentOverflow splits slides that have more content items than any
// template layout can accommodate. Each excess content item becomes its own
// continuation slide. Returns warnings for each split.
func SplitContentOverflow(pres *types.PresentationDefinition, layouts []types.LayoutMetadata) []string {
	maxPH := maxContentPlaceholders(layouts)
	hasChartPH := anyLayoutHasChartPlaceholder(layouts)

	var warnings []string
	var result []types.SlideDefinition

	for _, slide := range pres.Slides {
		split, w := maybeSplitSlide(slide, maxPH, hasChartPH)
		warnings = append(warnings, w...)
		result = append(result, split...)
	}

	// Re-index all slides
	for i := range result {
		result[i].Index = i
	}

	pres.Slides = result
	return warnings
}

// maybeSplitSlide checks if a slide needs splitting and returns the result.
// If no split is needed, returns the original slide unchanged.
func maybeSplitSlide(slide types.SlideDefinition, maxPH int, hasChartPH bool) ([]types.SlideDefinition, []string) {
	// Case 1: Slot-based slides with more slots than any layout can hold
	if slide.HasSlots() && len(slide.Slots) > maxPH && maxPH > 0 {
		pages, warnings := splitBySlots(slide, maxPH)
		return pages, warnings
	}

	// Case 2: Standard-path slide with chart + body text, no chart placeholder
	if !slide.HasSlots() && !hasChartPH &&
		slide.Content.DiagramSpec != nil && slide.Content.Body != "" {
		pages, warnings := splitChartFromBody(slide)
		return pages, warnings
	}

	return []types.SlideDefinition{slide}, nil
}

// splitBySlots splits a slot-based slide into multiple slides, each with at
// most maxSlots slots. Slots are re-numbered starting from 1 on each page.
func splitBySlots(slide types.SlideDefinition, maxSlots int) ([]types.SlideDefinition, []string) {
	// Collect and sort slot numbers
	slotNums := make([]int, 0, len(slide.Slots))
	for num := range slide.Slots {
		slotNums = append(slotNums, num)
	}
	slices.Sort(slotNums)

	// Chunk slots into groups of maxSlots
	var chunks [][]int
	for i := 0; i < len(slotNums); i += maxSlots {
		end := i + maxSlots
		if end > len(slotNums) {
			end = len(slotNums)
		}
		chunks = append(chunks, slotNums[i:end])
	}

	if len(chunks) <= 1 {
		return []types.SlideDefinition{slide}, nil
	}

	total := len(chunks)
	pages := make([]types.SlideDefinition, total)

	for i, chunk := range chunks {
		page := types.SlideDefinition{
			SourceLine:      slide.SourceLine,
			Title:           paginateTitle(slide.Title, i+1, total),
			Type:            slide.Type,
			RawContent:      slide.RawContent,
			Transition:      slide.Transition,
			TransitionSpeed: slide.TransitionSpeed,
			Build:           slide.Build,
		}

		// Re-number slots starting from 1
		pageSlots := make(map[int]*types.SlotContent, len(chunk))
		for j, origNum := range chunk {
			slotCopy := *slide.Slots[origNum]
			slotCopy.SlotNumber = j + 1
			pageSlots[j+1] = &slotCopy
		}
		page.Slots = pageSlots

		// First page gets speaker notes and source
		if i == 0 {
			page.SpeakerNotes = slide.SpeakerNotes
			page.Source = slide.Source
		}

		pages[i] = page
	}

	slog.Info("split slot-overflow slide",
		slog.String("title", slide.Title),
		slog.Int("slot_count", len(slide.Slots)),
		slog.Int("max_capacity", maxSlots),
		slog.Int("result_pages", total),
	)

	return pages, []string{
		fmt.Sprintf("slide %q has %d slots but max layout capacity is %d; auto-split into %d slides",
			slide.Title, len(slide.Slots), maxSlots, total),
	}
}

// splitChartFromBody splits a standard-path slide that has both Body text and
// a DiagramSpec into two slides: one with just text, one with just the chart.
func splitChartFromBody(slide types.SlideDefinition) ([]types.SlideDefinition, []string) {
	// Slide 1: text content (body, bullets, etc.) — no chart
	textSlide := slide // shallow copy
	textSlide.Title = paginateTitle(slide.Title, 1, 2)
	textSlide.Content.DiagramSpec = nil

	// Slide 2: chart only — no body text
	chartSlide := types.SlideDefinition{
		SourceLine:      slide.SourceLine,
		Title:           paginateTitle(slide.Title, 2, 2),
		Type:            slide.Type,
		RawContent:      slide.RawContent,
		Transition:      slide.Transition,
		TransitionSpeed: slide.TransitionSpeed,
		Build:           slide.Build,
		Content: types.SlideContent{
			DiagramSpec: slide.Content.DiagramSpec,
		},
	}

	slog.Info("split chart-body collision",
		slog.String("title", slide.Title),
	)

	return []types.SlideDefinition{textSlide, chartSlide}, []string{
		fmt.Sprintf("slide %q has both body text and chart but no layout has a chart placeholder; auto-split into text + chart slides",
			slide.Title),
	}
}

// maxContentPlaceholders returns the maximum number of content placeholders
// (body, content, image, chart, table) across all template layouts.
func maxContentPlaceholders(layouts []types.LayoutMetadata) int {
	best := 0
	for _, l := range layouts {
		count := countContentPHs(l)
		if count > best {
			best = count
		}
	}
	return best
}

// anyLayoutHasChartPlaceholder returns true if any layout has a dedicated
// chart placeholder (separate from body/content).
func anyLayoutHasChartPlaceholder(layouts []types.LayoutMetadata) bool {
	for _, l := range layouts {
		for _, ph := range l.Placeholders {
			if ph.Type == types.PlaceholderChart {
				return true
			}
		}
	}
	return false
}

// countContentPHs counts placeholders that can serve as content slot targets.
// Mirrors generator.FilterContentPlaceholders and layout.countContentPlaceholders.
func countContentPHs(layout types.LayoutMetadata) int {
	count := 0
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderBody, types.PlaceholderContent,
			types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable:
			count++
		}
	}
	return count
}

// paginateTitle adds a page number suffix to a slide title.
func paginateTitle(title string, page, total int) string {
	if total <= 1 {
		return title
	}
	return fmt.Sprintf("%s (%d/%d)", title, page, total)
}

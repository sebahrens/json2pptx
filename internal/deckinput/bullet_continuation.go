package deckinput

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// SplitBulletSlide windows all plain bullet columns together. It is an explicit
// authoring primitive, not a fit estimate: callers must render and inspect the
// resulting pages. Strings (including markup and indentation) are never edited.
// Compound bullet content requires separate authoring because its lead/header
// semantics cannot be inferred safely. Non-bullet content repeats unchanged.
func SplitBulletSlide(base SlideInput, maxItems int) ([]SlideInput, error) {
	if maxItems <= 0 {
		return nil, fmt.Errorf("split_bullets: max_items must be positive")
	}
	columns := map[int][]string{}
	count := -1
	for i, item := range base.Content {
		switch item.Type {
		case "body_and_bullets", "bullet_groups", "body_and_lead":
			return nil, fmt.Errorf("split_bullets: compound content at index %d requires explicit sibling slides", i)
		case "bullets":
			var bullets []string
			if item.BulletsValue != nil {
				bullets = *item.BulletsValue
			} else if err := json.Unmarshal(item.Value, &bullets); err != nil {
				return nil, fmt.Errorf("split_bullets: content %d: %w", i, err)
			}
			if len(bullets) == 0 || (count >= 0 && len(bullets) != count) {
				return nil, fmt.Errorf("split_bullets: columns must be nonempty with matching item counts")
			}
			if depth, _ := pptx.BulletIndentDepth(bullets[0]); depth > 0 {
				return nil, fmt.Errorf("split_bullets: column %d starts with an orphan nested bullet", i)
			}
			for _, bullet := range bullets {
				if _, text := pptx.BulletIndentDepth(bullet); text == "" {
					return nil, fmt.Errorf("split_bullets: blank bullet cannot serve as a parent or page boundary; author sibling slides explicitly")
				}
			}
			columns[i], count = bullets, len(bullets)
		}
	}
	if count < 0 {
		return nil, fmt.Errorf("split_bullets: no plain bullet content")
	}
	windows, err := bulletContinuationWindows(columns, count, maxItems)
	if err != nil {
		return nil, err
	}
	pages := make([]SlideInput, len(windows))
	for p, window := range windows {
		page := base
		page.Content = append([]ContentInput(nil), base.Content...)
		for index, bullets := range columns {
			chunk := append([]string(nil), bullets[window[0]:window[1]]...)
			page.Content[index].BulletsValue = &chunk
			page.Content[index].Value = nil
		}
		// Match table continuation convention: notes and provenance stay on
		// page one, while authored title/layout/chrome remain on every page.
		if p > 0 {
			page.SpeakerNotes, page.Source, page.SourceLink = "", "", nil
		}
		pages[p] = page
	}
	return pages, nil
}

func bulletContinuationWindows(columns map[int][]string, count, maxItems int) ([][2]int, error) {
	var windows [][2]int
	for start := 0; start < count; {
		end := start + min(maxItems, count-start)
		// A page boundary must start a root bullet in every aligned column.
		// Back up instead of detaching a nested child from its parent.
		for end < count && end > start {
			safe := true
			for _, bullets := range columns {
				if depth, _ := pptx.BulletIndentDepth(bullets[end]); depth > 0 {
					safe = false
					break
				}
			}
			if safe {
				break
			}
			end--
		}
		if end == start {
			return nil, fmt.Errorf("split_bullets: a parent/child group exceeds max_items; increase the budget or author sibling slides")
		}
		windows = append(windows, [2]int{start, end})
		start = end
	}
	return windows, nil
}

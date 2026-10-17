package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// tableSplitOps retains every authored field and row. Only the final part
// keeps totals_row, since its last row is the original table's last row.
// The normal remedy trial validates both resulting slides before offering it.
func (rc *remedyContext) tableSplitOps(path string) []any {
	if !strings.HasSuffix(path, ".rows") && !strings.HasSuffix(path, "/rows") {
		return nil
	}
	container := slideContainer(path)
	pointer, node, ok := rc.nodeAt(container)
	slide, isSlide := node.(map[string]any)
	if !ok || !isSlide || slide["kind"] != "table" {
		return nil
	}
	rows, ok := slide["rows"].([]any)
	if !ok || len(rows) < 2 {
		return nil
	}
	first := deepCopyJSON(slide).(map[string]any)
	second := deepCopyJSON(slide).(map[string]any)
	cut := len(rows) / 2
	first["rows"], second["rows"] = rows[:cut], rows[cut:]
	delete(first, "totals_row")
	// The original slide retains its stable identity. The inserted one gets a
	// fresh identity from the normal DeckSpec storage path.
	delete(second, "id")
	if title, ok := slide["title"].(string); ok {
		first["title"], second["title"] = title+" (1/2)", title+" (2/2)"
	}
	index := slidepath.SlideIndex(pointer)
	if index < 0 {
		return nil
	}
	return []any{
		map[string]any{"op": "replace", "path": pointer, "value": first},
		map[string]any{"op": "add", "path": fmt.Sprintf("/slides/%d", index+1), "value": second},
	}
}

// Patches that insert slides cannot be batched with patches addressed to the
// original slide positions: the insertion changes those positions.
func hasSlideInsertion(targets []*remedyTarget) bool {
	for _, target := range targets {
		for _, candidate := range target.candidates {
			for _, raw := range candidate {
				op, ok := raw.(map[string]any)
				path, _ := op["path"].(string)
				if ok && op["op"] == "add" && strings.HasPrefix(path, "/slides/") && strings.Count(path, "/") == 2 {
					return true
				}
			}
		}
	}
	return false
}

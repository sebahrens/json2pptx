// Package layout provides layout selection logic for slides.
package layout

import "strings"

// LayoutAliases maps common names to generated layout IDs.
// These aliases allow users and the system to reference layouts using
// intuitive names that map to the programmatically generated layout IDs.
var LayoutAliases = map[string]string{
	// Two-column aliases
	"two-column":    "content-2-50-50",
	"2-col":         "content-2-50-50",
	"2col":          "content-2-50-50",
	"split":         "content-2-50-50",
	"sidebar-right": "content-2-70-30",
	"sidebar-left":  "content-2-30-70",
	"main-sidebar":  "content-2-70-30",
	"sidebar-main":  "content-2-30-70",

	// Multi-column aliases
	"three-column": "content-3",
	"3-col":        "content-3",
	"3col":         "content-3",
	"four-column":  "content-4",
	"4-col":        "content-4",
	"4col":         "content-4",
	"five-column":  "content-5",
	"5-col":        "content-5",
	"5col":         "content-5",

	// Grid aliases
	"2x2":    "grid-2x2",
	"grid":   "grid-2x2",
	"3x3":    "grid-3x3",
	"matrix": "grid-2x2",
	"2x3":    "grid-2x3",
	"3x2":    "grid-3x2",
	"4x2":    "grid-4x2",
	"4x3":    "grid-4x3",

	// Special layouts
	"agenda":   "agenda",
	"contents": "agenda",
	"outline":  "agenda",
	"toc":      "agenda",
}

// ResolveLayoutAlias resolves an alias to a layout ID.
// If the given string is a known alias, returns the corresponding layout ID.
// Otherwise, returns the input unchanged.
func ResolveLayoutAlias(requested string) string {
	normalized := strings.ToLower(strings.TrimSpace(requested))
	if alias, ok := LayoutAliases[normalized]; ok {
		return alias
	}
	return requested
}

// ClassifyGeneratedLayout returns appropriate tags for a generated layout
// based on its ID pattern. This enables the heuristic selector to properly
// score and select generated layouts.
func ClassifyGeneratedLayout(layoutID string) []string {
	var tags []string

	switch {
	case layoutID == "content-1":
		tags = []string{"content", "single-placeholder"}

	case strings.HasPrefix(layoutID, "content-2"):
		tags = []string{"content", "two-column", "comparison"}
		if strings.Contains(layoutID, "70-30") {
			tags = append(tags, "sidebar-right")
		} else if strings.Contains(layoutID, "30-70") {
			tags = append(tags, "sidebar-left")
		}

	case strings.HasPrefix(layoutID, "content-3"):
		tags = []string{"content", "three-column", "multi-column"}

	case strings.HasPrefix(layoutID, "content-4"):
		tags = []string{"content", "four-column", "multi-column"}

	case strings.HasPrefix(layoutID, "content-5"):
		tags = []string{"content", "five-column", "multi-column"}

	case strings.HasPrefix(layoutID, "grid-"):
		tags = []string{"content", "grid"}
		// Extract dimensions (e.g., "grid-2x2" -> "2x2")
		parts := strings.SplitN(layoutID, "-", 2)
		if len(parts) == 2 && parts[1] != "" {
			dims := parts[1]
			tags = append(tags, "grid-"+dims)
		}

	case layoutID == "agenda":
		tags = []string{"agenda", "outline", "list"}
	}

	return tags
}

// IsGeneratedLayout checks if a layout ID matches the pattern of
// programmatically generated layouts.
func IsGeneratedLayout(layoutID string) bool {
	// Check for content-N patterns
	if strings.HasPrefix(layoutID, "content-") {
		return true
	}
	// Check for grid patterns
	if strings.HasPrefix(layoutID, "grid-") {
		return true
	}
	// Check for special generated layouts
	if layoutID == "agenda" {
		return true
	}
	return false
}

// GetSlotCount returns the number of content slots in a generated layout.
// Returns 0 for non-generated or unrecognized layouts.
func GetSlotCount(layoutID string) int {
	switch layoutID {
	case "content-1":
		return 1
	case "content-2-50-50", "content-2-70-30", "content-2-30-70":
		return 2
	case "content-3":
		return 3
	case "content-4":
		return 4
	case "content-5":
		return 5
	case "grid-2x2":
		return 4
	case "grid-2x3", "grid-3x2":
		return 6
	case "grid-3x3":
		return 9
	case "grid-4x2":
		return 8
	case "grid-4x3":
		return 12
	case "agenda":
		return 1
	default:
		return 0
	}
}

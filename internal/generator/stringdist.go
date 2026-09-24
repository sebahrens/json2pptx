// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"sort"
	"strings"
)

// levenshteinDistance computes the Levenshtein edit distance between two strings.
// This is a standard dynamic programming implementation with O(m*n) time and O(min(m,n)) space.
func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Ensure a is the shorter string to minimize memory usage.
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}

	// Single-row DP: prev[j] holds the distance for the previous row.
	prev := make([]int, la+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= lb; i++ {
		curr := make([]int, la+1)
		curr[0] = i
		for j := 1; j <= la; j++ {
			cost := 1
			if b[i-1] == a[j-1] {
				cost = 0
			}
			curr[j] = min3(
				curr[j-1]+1,    // insertion
				prev[j]+1,      // deletion
				prev[j-1]+cost, // substitution
			)
		}
		prev = curr
	}
	return prev[la]
}

// min3 returns the minimum of three integers.
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// ClosestMatch finds the closest string to target from candidates using Levenshtein distance.
// Returns the closest match and its distance. Only suggests if distance <= maxDistance.
// Returns ("", -1) if no candidates exist or none are within maxDistance.
func ClosestMatch(target string, candidates []string, maxDistance int) (string, int) {
	if len(candidates) == 0 {
		return "", -1
	}

	bestMatch := ""
	bestDist := -1

	for _, c := range candidates {
		d := levenshteinDistance(target, c)
		if bestDist < 0 || d < bestDist {
			bestDist = d
			bestMatch = c
		}
	}

	if bestDist > maxDistance {
		return "", -1
	}
	return bestMatch, bestDist
}

// FormatAvailableIDs formats a sorted list of IDs for error messages.
// Example output: [layout1, layout2, layout3]
func FormatAvailableIDs(ids []string) string {
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	return "[" + strings.Join(sorted, ", ") + "]"
}

// LayoutNotFoundError builds a descriptive error message when a layout_id is not found.
// It lists available layouts and suggests the closest match if within distance 3.
func LayoutNotFoundError(layoutID string, availableLayouts []string) string {
	msg := fmt.Sprintf("layout_id %q not found in template", layoutID)

	if len(availableLayouts) > 0 {
		msg += fmt.Sprintf("; available layouts: %s", FormatAvailableIDs(availableLayouts))

		if match, _ := ClosestMatch(layoutID, availableLayouts, 3); match != "" {
			msg += fmt.Sprintf("; did you mean %q?", match)
		}
	}

	return msg
}

// SuggestedCanonicalLayout returns an available canonical alternative when
// one has a sensible meaning for the requested layout, then tries a close
// spelling match. It never suggests opaque slideLayoutN IDs.
func SuggestedCanonicalLayout(name string, available []string) string {
	available = append([]string(nil), available...)
	sort.Strings(available)
	preferred := map[string]string{
		"image-left":  "image-right",
		"image-right": "image-left",
		"agenda":      "content",
	}
	want := preferred[strings.ToLower(strings.TrimSpace(name))]
	for _, candidate := range available {
		if candidate == want {
			return candidate
		}
	}
	match, _ := ClosestMatch(name, available, 3)
	return match
}

// CanonicalLayoutNotFoundError names the template's actually resolvable
// canonical aliases. Raw slideLayoutN IDs are intentionally omitted: authors
// using a canonical name should be given the same vocabulary list_templates
// exposes, not implementation IDs that change between templates.
func CanonicalLayoutNotFoundError(name string, available []string) string {
	msg := fmt.Sprintf("layout_id %q is not available in this template; see list_templates.canonical_layout_availability (or canonical_layout_ids in detailed output)", name)
	if len(available) > 0 {
		msg += fmt.Sprintf("; available canonical_layout_ids: %s", FormatAvailableIDs(available))
		if suggestion := SuggestedCanonicalLayout(name, available); suggestion != "" {
			msg += fmt.Sprintf("; did_you_mean: %q", suggestion)
		}
	}
	return msg
}

// PlaceholderNotFoundError builds a descriptive error message when a placeholder_id is not found.
// It lists available placeholders in the layout and suggests the closest match if within distance 3.
func PlaceholderNotFoundError(placeholderID string, layoutID string, availablePlaceholders []string) string {
	msg := fmt.Sprintf("placeholder_id %q not found in layout %q", placeholderID, layoutID)
	if placeholderID == "subtitle" {
		return msg + "; no exact subtitle slot is available; use a layout with a subtitle or place supporting text in shape_grid (section body slots may hold decorative numbers)"
	}

	if len(availablePlaceholders) > 0 {
		msg += fmt.Sprintf("; available placeholders: %s", FormatAvailableIDs(availablePlaceholders))

		if match, _ := ClosestMatch(placeholderID, availablePlaceholders, 3); match != "" {
			msg += fmt.Sprintf("; did you mean %q?", match)
		}
	}

	return msg
}

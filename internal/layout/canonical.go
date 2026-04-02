package layout

import (
	"strings"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// canonicalRule defines how a canonical layout name maps to template layouts via tags.
type canonicalRule struct {
	requireTags []string // At least one must match
	excludeTags []string // If any present, skip the candidate
	nameHint    string   // Prefer layouts whose name or ID contains this substring
}

// canonicalNames maps human-friendly layout names to tag-based resolution rules.
var canonicalNames = map[string]canonicalRule{
	"title":                   {requireTags: []string{"title-slide"}, excludeTags: []string{"blank-title", "closing"}},
	"content":                 {requireTags: []string{"content"}, excludeTags: []string{"two-column", "section-header"}},
	"section":                 {requireTags: []string{"section-header"}},
	"closing":                 {requireTags: []string{"closing"}},
	"blank":                   {requireTags: []string{"blank-title", "blank"}},
	"two-column":              {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-wide-narrow":  {requireTags: []string{"two-column"}, nameHint: "60"},
	"two-column-narrow-wide":  {requireTags: []string{"two-column"}, nameHint: "40/60"},
	"image-left":              {requireTags: []string{"image-left"}},
	"image-right":             {requireTags: []string{"image-right"}},
	"quote":                   {requireTags: []string{"quote", "statement"}},
	"agenda":                  {requireTags: []string{"agenda"}},
}

// ResolveCanonicalLayoutID resolves a layout name to a concrete layout ID using
// tag-based matching against the available template layouts.
//
// Resolution order:
//  1. If the name is already a slideLayoutN ID, pass through unchanged.
//  2. If the name is a generated layout ID (content-2-50-50, grid-2x2, etc.), pass through.
//  3. If the name is a known alias (from LayoutAliases), resolve to the alias target.
//  4. If the name is a canonical name, find the best matching layout by tags.
//  5. Otherwise, return the name unchanged.
//
// The second return value indicates whether the name was successfully resolved.
func ResolveCanonicalLayoutID(name string, layouts []types.LayoutMetadata) (string, bool) {
	// Passthrough: slideLayoutN IDs are already concrete
	if strings.HasPrefix(name, "slideLayout") {
		return name, true
	}

	normalized := strings.ToLower(strings.TrimSpace(name))

	// Check canonical names first — prefer native template layouts over generated ones
	if rule, ok := canonicalNames[normalized]; ok {
		if id, found := findBestMatch(layouts, rule); found {
			return id, true
		}
	}

	// Passthrough: generated layout IDs (content-2-50-50, grid-2x2, etc.)
	if IsGeneratedLayout(name) {
		return name, true
	}

	// Fall back to layout aliases (2-col → content-2-50-50, etc.)
	if alias, ok := LayoutAliases[normalized]; ok {
		return alias, true
	}

	return name, false
}

// findBestMatch searches layouts for the best match for a canonical rule.
func findBestMatch(layouts []types.LayoutMetadata, rule canonicalRule) (string, bool) {
	var candidates []types.LayoutMetadata

	for _, l := range layouts {
		if matchesTags(l.Tags, rule.requireTags, rule.excludeTags) {
			candidates = append(candidates, l)
		}
	}

	if len(candidates) == 0 {
		return "", false
	}

	// If there's a name hint, prefer candidates whose name or ID contains it
	if rule.nameHint != "" {
		hint := strings.ToLower(rule.nameHint)
		for _, c := range candidates {
			if strings.Contains(strings.ToLower(c.Name), hint) ||
				strings.Contains(strings.ToLower(c.ID), hint) {
				return c.ID, true
			}
		}
	}

	return candidates[0].ID, true
}

// matchesTags checks if a layout's tags satisfy the require/exclude constraints.
func matchesTags(layoutTags, requireTags, excludeTags []string) bool {
	// Must have at least one required tag
	hasRequired := false
	for _, req := range requireTags {
		for _, lt := range layoutTags {
			if lt == req {
				hasRequired = true
				break
			}
		}
		if hasRequired {
			break
		}
	}
	if !hasRequired {
		return false
	}

	// Must not have any excluded tags
	for _, ex := range excludeTags {
		for _, lt := range layoutTags {
			if lt == ex {
				return false
			}
		}
	}

	return true
}

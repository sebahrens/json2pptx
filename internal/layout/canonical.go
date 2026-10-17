package layout

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// NOTE ON NAMING: the "canonical" in this file refers to user-facing canonical
// *layout names* (the JSON shorthands "title", "content", "section", …), not the
// canonical layout *taxonomy* in internal/template (types.CanonicalLayoutType).
// The mappings below stay tag-based on purpose: they depend on fine-grained
// structural tags (image-left, two-column variants, quote/statement, agenda,
// nameHints) that have no 1:1 equivalent in the coarse 7-type taxonomy, so they
// are intentionally NOT routed through types.CanonicalLayoutType. The heuristic
// scorer in heuristic.go is tag-based for the same reason. The one selection
// site with a clean 1:1 mapping — chrome page-number/footer skipping — does use
// the canonical taxonomy (see applyChromeSkip in cmd/json2pptx/json_mode.go).

// canonicalRule defines how a canonical layout name maps to template layouts via tags.
type canonicalRule struct {
	requireTags []string // At least one must match
	excludeTags []string // If any present, skip the candidate
	nameHint    string   // Prefer layouts whose name or ID contains this substring
}

// canonicalNames maps human-friendly layout names to tag-based resolution rules.
//
// Two distinct template roles share the word "blank" (see the "blank" /
// "blank-title" classifier tags in internal/template/classifier.go):
//   - "blank-canvas" targets the truly empty Blank layout (tag "blank", zero
//     content placeholders) — for slides whose entire content is a shape_grid or
//     pattern with no slide title.
//   - "blank-title" targets the Blank + Title layout (tag "blank-title", a title
//     placeholder and nothing else) — a title canvas used to host a shape_grid or
//     pattern below a slide title.
//
// "blank" is a LEGACY alias retained for backward compatibility. Historically it
// resolved to a title-bearing canvas (Blank + Title preferred via the "title"
// nameHint, falling back to the true Blank when no Blank + Title exists), and the
// bundled skeletons/skill examples still pass layout_id:"blank" for shape_grid
// slides that need a title. Its blank-title-preferred semantics are kept stable;
// author new decks with the unambiguous "blank-title" / "blank-canvas" IDs.
var canonicalNames = map[string]canonicalRule{
	"title":                  {requireTags: []string{"title-slide"}, excludeTags: []string{"blank-title", "closing"}},
	"content":                {requireTags: []string{"content"}, excludeTags: []string{"two-column", "section-header"}},
	"section":                {requireTags: []string{"section-header"}},
	"closing":                {requireTags: []string{"closing"}},
	"blank":                  {requireTags: []string{"blank-title", "blank"}, nameHint: "title"},
	"blank-title":            {requireTags: []string{"blank-title"}},
	"blank+title":            {requireTags: []string{"blank-title"}},
	"blank-canvas":           {requireTags: []string{"blank"}},
	"two-column":             {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-wide-narrow": {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-60-40":       {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-40-60":       {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-70-30":       {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-30-70":       {requireTags: []string{"two-column"}, nameHint: "50"},
	"stacked-60-40":          {requireTags: []string{"two-column"}, nameHint: "50"},
	"stacked-40-60":          {requireTags: []string{"two-column"}, nameHint: "50"},
	"stacked":                {requireTags: []string{"two-column"}, nameHint: "50"},
	"two-column-narrow-wide": {requireTags: []string{"two-column"}, nameHint: "50"},
	"image-left":             {requireTags: []string{"image-left"}},
	"image-right":            {requireTags: []string{"image-right"}},
	"quote":                  {requireTags: []string{"quote", "statement"}},
	"agenda":                 {requireTags: []string{"agenda"}},
}

// IsCanonicalName reports whether a name is one of the canonical LAYOUT names
// (case-insensitive) — the template-independent ids layout_id accepts, resolved
// per template by tag. It exists so a field that is not layout_id can tell an
// author "that value belongs in layout_id" instead of only listing its own
// vocabulary (go-slide-creator-ejh5u).
func IsCanonicalName(name string) bool {
	_, ok := canonicalNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// DerivedColumnLeftPercent returns the left column's requested share for the
// two asymmetric canonical names. Zero means use the template's native split.
func DerivedColumnLeftPercent(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "stacked":
		return 50
	case "stacked-60-40":
		return 60
	case "stacked-40-60":
		return 40
	case "two-column-60-40":
		return 60
	case "two-column-40-60":
		return 40
	case "two-column-70-30", "sidebar-right", "main-sidebar":
		return 70
	case "two-column-30-70", "sidebar-left", "sidebar-main":
		return 30
	case "two-column-wide-narrow":
		return 65
	case "two-column-narrow-wide":
		return 35
	default:
		return 0
	}
}

// CanonicalNames returns every canonical layout name, sorted.
func CanonicalNames() []string {
	out := make([]string, 0, len(canonicalNames))
	for name := range canonicalNames {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ResolveCanonicalLayoutID resolves a layout name to a concrete layout ID using
// tag-based matching against the available template layouts.
//
// Resolution order:
//  1. Pass through concrete slideLayoutN IDs.
//  2. Normalize portable aliases, then prefer native hosts by canonical tags.
//     Image-side roles may use a native two-column host.
//  3. Pass through explicit generated IDs, except agenda requires a real host.
//  4. Use a portable two-column alias fallback, or another alias whose target
//     ID actually exists in the template (including literal agenda).
//  5. Return the original name and false when no host resolves.
//
// The second return value indicates whether the name was successfully resolved.
func ResolveCanonicalLayoutID(name string, layouts []types.LayoutMetadata) (string, bool) {
	// Passthrough: slideLayoutN IDs are already concrete
	if strings.HasPrefix(name, "slideLayout") {
		return name, true
	}

	normalized := strings.ToLower(strings.TrimSpace(name))
	aliasName := normalized
	switch normalized {
	case "sidebar-right", "main-sidebar":
		normalized = "two-column-70-30"
	case "sidebar-left", "sidebar-main":
		normalized = "two-column-30-70"
	case "2-col", "2col", "split":
		normalized = "two-column"
	case "contents", "outline", "toc":
		normalized = "agenda"
	}

	// Check canonical names first — prefer native template layouts over generated ones
	if rule, ok := canonicalNames[normalized]; ok {
		if id, found := findBestMatch(layouts, rule); found {
			return id, true
		}
	}

	if normalized == "image-left" || normalized == "image-right" {
		if id, ok := findBestMatch(layouts, canonicalNames["two-column"]); ok {
			return id, true
		}
	}

	// Passthrough: generated layout IDs (content-2-50-50, grid-2x2, etc.)
	if IsGeneratedLayout(name) && normalized != "agenda" {
		return name, true
	}

	// Fall back to layout aliases (2-col → content-2-50-50, etc.)
	if alias, ok := LayoutAliases[aliasName]; ok {
		// Portable two-column aliases retain their generated fallback when no
		// native host exists; unsupported multi-column aliases need a real host.
		if strings.HasPrefix(alias, "content-2-") {
			return alias, true
		}
		for _, candidate := range layouts {
			if candidate.ID == alias {
				return alias, true
			}
		}
		return name, false
	}

	return name, false
}

// ResolveAllCanonicalLayouts resolves every canonical layout name against the
// given template layouts and returns a map of canonical name → concrete layout ID.
// Only names that successfully resolve are included in the result.
func ResolveAllCanonicalLayouts(layouts []types.LayoutMetadata) map[string]string {
	result := make(map[string]string, len(canonicalNames))
	for name := range canonicalNames {
		if id, found := ResolveCanonicalLayoutID(name, layouts); found {
			for _, candidate := range layouts {
				if candidate.ID == id {
					result[name] = id
					break
				}
			}
		}
	}
	return result
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

// UnsupportedNativeAliasHint explains how to author missing multi-pane roles.
func UnsupportedNativeAliasHint(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "three-column", "3-col", "3col", "four-column", "4-col", "4col", "five-column", "5-col", "5col":
		return "use regions columns for two or three panes, regions quad for four panes, or stylish-panels / comparison-2col for structured comparisons"
	}
	return ""
}

// IsStackedName identifies vertical native body splits before layout resolution.
func IsStackedName(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "stacked") && IsCanonicalName(name)
}

// ImageColumnFallback reports the side for a portable image+text native split.
func ImageColumnFallback(name string, layouts []types.LayoutMetadata) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized != "image-left" && normalized != "image-right" {
		return ""
	}
	if _, native := findBestMatch(layouts, canonicalNames[normalized]); native {
		return ""
	}
	return strings.TrimPrefix(normalized, "image-")
}

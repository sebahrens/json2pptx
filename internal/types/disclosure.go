package types

import "github.com/sebahrens/json2pptx/internal/placeholderrole"

// IsDisclosurePlaceholder recognizes explicit legal text slots, including
// hand-built metadata not yet carrying a parsed Role. Fonts are not semantics.
func IsDisclosurePlaceholder(ph PlaceholderInfo) bool {
	return placeholderrole.IsDisclosureAlias(ph.ID) && (ph.Type == PlaceholderSubtitle || ph.Type == PlaceholderBody || ph.Type == PlaceholderContent)
}

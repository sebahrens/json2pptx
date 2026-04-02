// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

// placeholderResolver provides flexible placeholder lookup by canonical name or
// OOXML placeholder index. It supports:
//   - Name-based lookup: "body", "body_2", "title"
//   - Index-based lookup: "idx:1", "idx:10" (targets OOXML ph idx attribute)
//   - Positional matching: when multiple shapes share a name, successive lookups
//     for the same name return shapes in visual order (left-to-right, top-to-bottom)
type placeholderResolver struct {
	byName   map[string][]int // shape name → list of shape indices (in order)
	byIdx    map[int]int      // OOXML placeholder idx → shape index
	nameUsed map[string]int   // tracks positional consumption for duplicate names
	warnings []string
	shapes   []shapeXML
	roleMap  map[int]PlaceholderSemanticRole // lazily-built semantic role map (see below)
}

// newPlaceholderResolver builds a resolver from the slide's shape tree.
func newPlaceholderResolver(shapes []shapeXML) *placeholderResolver {
	r := &placeholderResolver{
		byName:   make(map[string][]int),
		byIdx:    make(map[int]int),
		nameUsed: make(map[string]int),
		shapes:   shapes,
	}

	for i, shape := range shapes {
		name := shape.NonVisualProperties.ConnectionNonVisual.Name
		if name != "" {
			r.byName[name] = append(r.byName[name], i)
		}

		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph != nil && ph.Index != nil {
			r.byIdx[*ph.Index] = i
		}
	}

	// Warn about duplicate names (should not happen after normalization,
	// but provides a safety net for non-normalized templates).
	for name, indices := range r.byName {
		if len(indices) > 1 {
			r.warnings = append(r.warnings,
				fmt.Sprintf("ambiguous placeholder name %q matches %d shapes; use idx:N syntax to disambiguate", name, len(indices)))
		}
	}

	return r
}

// Resolve looks up a shape index by placeholder ID.
// Supports "idx:N" syntax for OOXML placeholder index targeting and
// name-based lookup with positional ordering for duplicate names.
func (r *placeholderResolver) Resolve(placeholderID string) (int, bool) {
	// Check for idx:N syntax (e.g., "idx:1", "idx:10")
	if strings.HasPrefix(placeholderID, "idx:") {
		if idx, err := strconv.Atoi(placeholderID[4:]); err == nil {
			if shapeIdx, ok := r.byIdx[idx]; ok {
				return shapeIdx, true
			}
		}
		return 0, false
	}

	// Name-based lookup
	indices, ok := r.byName[placeholderID]
	if !ok || len(indices) == 0 {
		return 0, false
	}

	// Single match — simple case
	if len(indices) == 1 {
		return indices[0], true
	}

	// Multiple matches — assign positionally (first caller gets first shape, etc.)
	usedCount := r.nameUsed[placeholderID]
	if usedCount < len(indices) {
		r.nameUsed[placeholderID]++
		return indices[usedCount], true
	}

	// All positions consumed — return last
	return indices[len(indices)-1], true
}

// Keys returns all registered placeholder names for error messages.
func (r *placeholderResolver) Keys() []string {
	keys := make([]string, 0, len(r.byName))
	for k := range r.byName {
		keys = append(keys, k)
	}
	return keys
}

// ResolveIDToName translates a placeholder ID (possibly "idx:N") to the
// canonical shape name. Returns the ID unchanged if it's already a name.
// This is used by clearUnmappedPlaceholders to track which shapes are populated.
func (r *placeholderResolver) ResolveIDToName(placeholderID string) string {
	if strings.HasPrefix(placeholderID, "idx:") {
		if idx, err := strconv.Atoi(placeholderID[4:]); err == nil {
			if shapeIdx, ok := r.byIdx[idx]; ok {
				return r.shapes[shapeIdx].NonVisualProperties.ConnectionNonVisual.Name
			}
		}
	}
	return placeholderID
}

// buildPlaceholderMap creates a lookup map from canonical shape names to indices.
// After normalization, each placeholder has a unique canonical name
// (e.g., "title", "body", "body_2", "image"), so only name-based lookup is needed.
func buildPlaceholderMap(shapes []shapeXML) map[string]int {
	m := make(map[string]int, len(shapes))
	for i, shape := range shapes {
		if name := shape.NonVisualProperties.ConnectionNonVisual.Name; name != "" {
			m[name] = i
		}
	}
	return m
}

// ResolutionTier indicates which level of the resolver matched.
type ResolutionTier int

const (
	// TierExact is a direct match by canonical name or idx:N syntax.
	TierExact ResolutionTier = iota + 1
	// TierSemantic matched via structural role classification.
	TierSemantic
	// TierFuzzy matched via Levenshtein distance on normalized names.
	TierFuzzy
	// TierPositional matched via slot/positional ordering.
	TierPositional
	// TierTopmost matched by picking the topmost placeholder (smallest Y offset).
	// Used as a last resort for "title" content when no title-type placeholder exists.
	TierTopmost
)

// String returns a human-readable tier name.
func (t ResolutionTier) String() string {
	switch t {
	case TierExact:
		return "exact"
	case TierSemantic:
		return "semantic"
	case TierFuzzy:
		return "fuzzy"
	case TierPositional:
		return "positional"
	case TierTopmost:
		return "topmost"
	default:
		return "unknown"
	}
}

// =============================================================================
// Semantic Role Classification
// =============================================================================

// PlaceholderSemanticRole classifies a placeholder by its structural purpose.
type PlaceholderSemanticRole int

const (
	RoleUnknown        PlaceholderSemanticRole = iota
	RoleTitle          // ph.Type == "title" or "ctrTitle"
	RoleSubtitle       // ph.Type == "subTitle"
	RoleBodyPrimary    // Largest body-type placeholder by area
	RoleBodySecondary  // Second largest body-type placeholder
	RoleBodyTertiary   // Third body-type placeholder
	RoleAccentLarge    // Body placeholder with very large font (Big Statement number)
	RoleCaption        // Small body placeholder (caption strip)
)

// semanticAliases maps common placeholder_id conventions to semantic roles.
var semanticAliases = map[string]PlaceholderSemanticRole{
	"Content Placeholder 1": RoleBodyPrimary,
	"Content Placeholder 2": RoleBodySecondary,
	"Content Placeholder 3": RoleBodyTertiary,
	"Text Placeholder 1":    RoleBodyPrimary,
	"Text Placeholder 2":    RoleBodySecondary,
	"Text Placeholder 3":    RoleBodyTertiary,
	"body":                  RoleBodyPrimary,
	"body_2":                RoleBodySecondary,
	"body_3":                RoleBodyTertiary,
	"subtitle":              RoleSubtitle,
	"title":                 RoleTitle,
}

// classifyShapeRole determines the semantic role of a shape based on its
// placeholder type and structural properties (position, size).
func classifyShapeRole(shape *shapeXML) PlaceholderSemanticRole {
	ph := shape.NonVisualProperties.NvPr.Placeholder
	if ph == nil {
		return RoleUnknown
	}

	switch ph.Type {
	case "title", "ctrTitle":
		return RoleTitle
	case "subTitle":
		return RoleSubtitle
	}

	// For body/obj/implicit types, classification requires context
	// (relative area comparison). Return RoleUnknown here; the resolver
	// builds a sorted role map from all shapes together.
	return RoleUnknown
}

// shapeArea returns the area of a shape in EMU² (or 0 if no transform).
func shapeArea(shape *shapeXML) int64 {
	if shape.ShapeProperties.Transform == nil {
		return 0
	}
	return shape.ShapeProperties.Transform.Extent.CX * shape.ShapeProperties.Transform.Extent.CY
}

// shapeXPosition returns the X offset of a shape (or 0 if no transform).
func shapeXPosition(shape *shapeXML) int64 {
	if shape.ShapeProperties.Transform == nil {
		return 0
	}
	return shape.ShapeProperties.Transform.Offset.X
}

// shapeYPosition returns the Y offset of a shape (or 0 if no transform).
func shapeYPosition(shape *shapeXML) int64 {
	if shape.ShapeProperties.Transform == nil {
		return 0
	}
	return shape.ShapeProperties.Transform.Offset.Y
}

// isContentPlaceholder returns true if the shape is a body, obj, or typeless
// (implicit) placeholder — the kinds that receive user content.
func isContentPlaceholder(shape *shapeXML) bool {
	ph := shape.NonVisualProperties.NvPr.Placeholder
	if ph == nil {
		return false
	}
	switch ph.Type {
	case "body", "obj", "":
		return true
	}
	return false
}

// buildSemanticRoleMap classifies body-type placeholders into semantic roles
// by sorting them by area (descending). The largest is RoleBodyPrimary, the
// second is RoleBodySecondary, etc.
func buildSemanticRoleMap(shapes []shapeXML) map[int]PlaceholderSemanticRole {
	roles := make(map[int]PlaceholderSemanticRole)

	// First pass: classify title/subtitle by placeholder type.
	for i := range shapes {
		if role := classifyShapeRole(&shapes[i]); role != RoleUnknown {
			roles[i] = role
		}
	}

	// Collect body-type placeholder indices and sort by area (largest first).
	type bodyShape struct {
		idx  int
		area int64
	}
	var bodies []bodyShape
	for i := range shapes {
		if _, already := roles[i]; already {
			continue
		}
		if isContentPlaceholder(&shapes[i]) {
			bodies = append(bodies, bodyShape{idx: i, area: shapeArea(&shapes[i])})
		}
	}

	sort.Slice(bodies, func(a, b int) bool {
		return bodies[a].area > bodies[b].area
	})

	// Assign roles by descending area.
	bodyRoles := []PlaceholderSemanticRole{RoleBodyPrimary, RoleBodySecondary, RoleBodyTertiary}
	for i, bs := range bodies {
		if i < len(bodyRoles) {
			roles[bs.idx] = bodyRoles[i]
		}
	}

	return roles
}

// =============================================================================
// Multi-tier Resolution Methods
// =============================================================================

// ResolveWithFallback looks up a shape index using the full five-tier resolution
// chain. Returns the shape index, which tier matched, and whether a match was found.
//
// Tiers (in order):
//  1. Exact: canonical name or idx:N syntax
//  2. Semantic: structural role classification
//  3. Fuzzy: Levenshtein distance on normalized names
//  4. Positional: slot-based or position-ordered content placeholders
//  5. Topmost: for "title" only — picks the placeholder with smallest Y offset
func (r *placeholderResolver) ResolveWithFallback(placeholderID string) (int, ResolutionTier, bool) {
	// Tier 1: Exact match (existing behavior)
	if idx, ok := r.Resolve(placeholderID); ok {
		return idx, TierExact, true
	}

	// Tier 2: Semantic role match
	if idx, ok := r.resolveBySemanticRole(placeholderID); ok {
		return idx, TierSemantic, true
	}

	// Tier 3: Fuzzy name match
	if idx, ok := r.resolveByFuzzyName(placeholderID); ok {
		return idx, TierFuzzy, true
	}

	// Tier 4: Positional fallback
	if idx, ok := r.resolveByPosition(placeholderID); ok {
		return idx, TierPositional, true
	}

	// Tier 5: Topmost placeholder fallback (title only).
	// Some layouts lack a title-type
	// placeholder entirely. Rather than silently dropping the title text,
	// route it to the topmost content placeholder so the text is visible.
	if placeholderID == "title" {
		if idx, ok := r.resolveByTopmostPlaceholder(); ok {
			return idx, TierTopmost, true
		}
	}

	return 0, 0, false
}

// resolveBySemanticRole maps a placeholder ID to a semantic role and then
// finds the shape with that role.
func (r *placeholderResolver) resolveBySemanticRole(placeholderID string) (int, bool) {
	targetRole, ok := semanticAliases[placeholderID]
	if !ok {
		return 0, false
	}

	roles := r.semanticRoles()
	for idx, role := range roles {
		if role == targetRole {
			return idx, true
		}
	}

	return 0, false
}

// semanticRoles returns or lazily builds the semantic role map.
func (r *placeholderResolver) semanticRoles() map[int]PlaceholderSemanticRole {
	if r.roleMap == nil {
		r.roleMap = buildSemanticRoleMap(r.shapes)
	}
	return r.roleMap
}

// resolveByFuzzyName uses Levenshtein distance to find a close match.
// Names are normalized before comparison: lowercased, "placeholder" removed,
// trailing digits stripped.
func (r *placeholderResolver) resolveByFuzzyName(placeholderID string) (int, bool) {
	normalized := normalizePlaceholderName(placeholderID)
	if normalized == "" {
		return 0, false
	}

	bestIdx := -1
	bestDist := 1<<31 - 1

	for i := range r.shapes {
		name := r.shapes[i].NonVisualProperties.ConnectionNonVisual.Name
		if name == "" {
			continue
		}
		normName := normalizePlaceholderName(name)
		dist := levenshteinDistance(normalized, normName)

		// Threshold: max(len/3, 2) — generous enough for "Content Placeholder 1"
		// → "body" type matches, but tight enough to avoid false positives.
		threshold := len(normalized) / 3
		if threshold < 2 {
			threshold = 2
		}
		if dist <= threshold && dist < bestDist {
			bestIdx = i
			bestDist = dist
		}
	}

	if bestIdx >= 0 {
		return bestIdx, true
	}
	return 0, false
}

// resolveByPosition interprets the placeholder ID as a slot reference
// (e.g., "slot1", "body_left", "body_right") and resolves by X-position
// among content placeholders.
func (r *placeholderResolver) resolveByPosition(placeholderID string) (int, bool) {
	slotIdx := parseSlotIndex(placeholderID)
	if slotIdx < 0 {
		return 0, false
	}

	contentShapes := r.contentPlaceholdersByXPosition()
	if slotIdx < len(contentShapes) {
		return contentShapes[slotIdx], true
	}

	return 0, false
}

// resolveByTopmostPlaceholder returns the content placeholder with the smallest
// Y offset (topmost in the slide). This is used as a last-resort fallback when
// no title-type placeholder exists in the layout. Rather than silently dropping
// the title text, routing it to the topmost content area keeps it visible.
func (r *placeholderResolver) resolveByTopmostPlaceholder() (int, bool) {
	bestIdx := -1
	var bestY int64 = 1<<62 - 1 // max int64 sentinel

	for i := range r.shapes {
		if !isContentPlaceholder(&r.shapes[i]) {
			continue
		}
		y := shapeYPosition(&r.shapes[i])
		if y < bestY {
			bestY = y
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		return bestIdx, true
	}
	return 0, false
}

// contentPlaceholdersByXPosition returns content placeholder shape indices
// sorted by X position (left to right).
func (r *placeholderResolver) contentPlaceholdersByXPosition() []int {
	type posShape struct {
		idx int
		x   int64
	}
	var content []posShape
	for i := range r.shapes {
		if isContentPlaceholder(&r.shapes[i]) {
			content = append(content, posShape{idx: i, x: shapeXPosition(&r.shapes[i])})
		}
	}

	sort.Slice(content, func(a, b int) bool {
		return content[a].x < content[b].x
	})

	result := make([]int, len(content))
	for i, cs := range content {
		result[i] = cs.idx
	}
	return result
}

// =============================================================================
// Name Normalization Utilities
// =============================================================================

// normalizePlaceholderName strips noise from a placeholder name for fuzzy
// comparison: lowercases, removes "placeholder", strips trailing digits,
// and collapses whitespace.
func normalizePlaceholderName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "placeholder", "")
	// Strip trailing digits (e.g., "15" from "Text Placeholder 15")
	s = strings.TrimRight(s, "0123456789")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// parseSlotIndex extracts a 0-based slot index from placeholder IDs like
// "slot1", "slot2", "body_left", "body_right".
// Returns -1 if the ID doesn't look like a slot reference.
func parseSlotIndex(id string) int {
	lower := strings.ToLower(id)

	// Direct slot references: "slot1" → 0, "slot2" → 1
	if strings.HasPrefix(lower, "slot") {
		rest := lower[4:]
		if len(rest) == 1 && rest[0] >= '1' && rest[0] <= '9' {
			return int(rest[0]-'0') - 1 // 1-indexed to 0-indexed
		}
	}

	// Positional aliases
	switch lower {
	case "body_left", "left":
		return 0
	case "body_right", "right":
		return 1
	case "body_center", "center":
		return 0 // For single-column, center maps to first
	}

	return -1
}

// =============================================================================
// Observability: Log Fallback Resolutions
// =============================================================================

// logFallbackResolution logs when a non-exact tier was used for resolution.
func logFallbackResolution(placeholderID string, resolvedTo string, tier ResolutionTier, layoutID string) {
	slog.Info("placeholder resolved via fallback",
		"requested", placeholderID,
		"resolved_to", resolvedTo,
		"tier", tier.String(),
		"layout_id", layoutID,
	)
}

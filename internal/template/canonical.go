package template

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/placeholderrole"
	"github.com/sebahrens/json2pptx/internal/types"
)

// This file is the single, authoritative entry point for the canonical layout
// and placeholder taxonomies that generation and preflight share.
//
//   - Layout taxonomy: ClassifyLayoutCanonical is a typed facade over
//     ClassifyCanonicalRole (internal/template/classifier.go). There is exactly
//     one layout classifier; this function only re-types its result so callers
//     can persist and compare it via types.CanonicalLayoutType. It deliberately
//     does NOT introduce a second set of rules beside ClassifyCanonicalRole.
//   - Placeholder taxonomy: ClassifyPlaceholderRole is the genuinely new
//     classifier. It refines the raw OOXML placeholder type into an
//     intent-level role (eyebrow vs title, section number vs body,
//     date/footer/page-number vs other).
//
// Both return enums defined in internal/types so they can be stored on
// types.LayoutMetadata / types.PlaceholderInfo without internal/template <->
// internal/types import cycles.

// The section-number font-size threshold and the placeholder-name alias / hint
// sets live in internal/placeholderrole, a stdlib-only leaf package shared with
// internal/generator's resolver. internal/template references them from there so
// the two classifiers never drift and internal/template never has to import
// internal/generator.

// ClassifyLayoutCanonical returns the canonical layout type and confidence for a
// layout. It delegates to ClassifyCanonicalRole so that the layout taxonomy has a
// single source of truth; the role string and types.CanonicalLayoutType share
// the same stable wire IDs.
func ClassifyLayoutCanonical(layout *types.LayoutMetadata) (types.CanonicalLayoutType, float64) {
	role, _, confidence := ClassifyCanonicalRole(layout)
	if role == "" {
		return types.CanonicalLayoutUnknown, confidence
	}
	return types.CanonicalLayoutType(role), confidence
}

// EffectiveCanonicalType returns a layout's canonical type, preferring the value
// persisted by ParseLayouts and falling back to on-the-fly classification when it
// is unset (e.g. hand-built or synthesized layouts that never passed through
// ParseLayouts). This keeps every consumer reading the same canonical taxonomy
// regardless of how the layout was constructed.
func EffectiveCanonicalType(layout *types.LayoutMetadata) types.CanonicalLayoutType {
	if layout == nil {
		return types.CanonicalLayoutUnknown
	}
	if layout.CanonicalType != types.CanonicalLayoutUnknown {
		return layout.CanonicalType
	}
	ct, _ := ClassifyLayoutCanonical(layout)
	return ct
}

// ClassifyPlaceholderRole assigns a canonical, intent-level role to a placeholder
// using its OOXML type, font size, name hints, and the section-number alias set.
// The layout argument is accepted for context-sensitive refinement and future
// use; classification today is placeholder-local.
//
// The returned confidence is 0.0–1.0: structural types (image, chart, subtitle,
// title) classify with high confidence, while chrome subtypes (date, footer,
// page number) and decorative bodies (eyebrow, section number) rely on name and
// font-size hints and report lower confidence.
func ClassifyPlaceholderRole(ph types.PlaceholderInfo, _ *types.LayoutMetadata) (types.PlaceholderRole, float64) {
	id := strings.ToLower(strings.TrimSpace(ph.ID))

	switch ph.Type {
	case types.PlaceholderImage:
		return types.PlaceholderRoleImage, 1.0
	case types.PlaceholderChart:
		return types.PlaceholderRoleChart, 1.0
	case types.PlaceholderTable:
		// Tables are content; treat them as body for placement purposes.
		return types.PlaceholderRoleBody, 0.6
	case types.PlaceholderSubtitle:
		return types.PlaceholderRoleSubtitle, 0.95
	case types.PlaceholderTitle:
		if placeholderrole.ContainsAnyHint(id, placeholderrole.EyebrowHints) {
			return types.PlaceholderRoleEyebrow, 0.75
		}
		return types.PlaceholderRoleTitle, 0.95
	case types.PlaceholderBody, types.PlaceholderContent:
		if conf, ok := sectionNumberConfidence(ph, id); ok {
			return types.PlaceholderRoleSectionNumber, conf
		}
		if placeholderrole.ContainsAnyHint(id, placeholderrole.EyebrowHints) {
			return types.PlaceholderRoleEyebrow, 0.7
		}
		if placeholderrole.ContainsAnyHint(id, placeholderrole.SubtitleHints) {
			return types.PlaceholderRoleSubtitle, 0.7
		}
		return types.PlaceholderRoleBody, 0.85
	case types.PlaceholderOther:
		return classifyChromeRole(id)
	}
	return types.PlaceholderRoleOther, 0.3
}

// sectionNumberConfidence reports whether a body/content placeholder is a
// decorative section number and, if so, the confidence. It triggers on an
// explicit alias ID, a "section … number" name, or a very large resolved font.
func sectionNumberConfidence(ph types.PlaceholderInfo, id string) (float64, bool) {
	if placeholderrole.IsSectionNumberAlias(id) {
		return 0.95, true
	}
	if strings.Contains(id, "section") && strings.Contains(id, "number") {
		return 0.9, true
	}
	if ph.FontSize >= placeholderrole.SectionNumberMinFontSize {
		return 0.85, true
	}
	return 0, false
}

// classifyChromeRole resolves a PlaceholderOther (date/footer/slide-number/header)
// into its specific role using name hints. The raw OOXML subtype is collapsed to
// PlaceholderOther upstream, so name is the only available signal.
func classifyChromeRole(id string) (types.PlaceholderRole, float64) {
	switch {
	case placeholderrole.ContainsAnyHint(id, placeholderrole.DateHints):
		return types.PlaceholderRoleDate, 0.85
	case placeholderrole.ContainsAnyHint(id, placeholderrole.PageNumHints):
		return types.PlaceholderRolePageNumber, 0.85
	case placeholderrole.ContainsAnyHint(id, placeholderrole.FooterHints):
		return types.PlaceholderRoleFooter, 0.85
	default:
		return types.PlaceholderRoleOther, 0.6
	}
}

// CanonicalFamilyCoverage returns, for each coarse layout family, the names of
// the layouts that provide it. Utility families (LayoutFamilyOther) are omitted.
// Callers use it to assert that a template covers the content-bearing families
// (title-slide, section-divider, one-content, qa-closing).
func CanonicalFamilyCoverage(layouts []types.LayoutMetadata) map[types.CanonicalLayoutFamily][]string {
	coverage := make(map[types.CanonicalLayoutFamily][]string)
	for i := range layouts {
		fam := EffectiveCanonicalType(&layouts[i]).Family()
		if fam == types.LayoutFamilyOther {
			continue
		}
		coverage[fam] = append(coverage[fam], layouts[i].Name)
	}
	return coverage
}

// DerivableLayout describes whether a higher-level layout can be derived from a
// template's base layouts, and what is missing when it cannot.
type DerivableLayout struct {
	Name    string   // derivable layout name (e.g. "two-content")
	Ready   bool     // true when the template can produce this layout
	Missing []string // specific prerequisites that are absent (empty when Ready)
}

// layoutCapabilities is a one-pass summary of the base capabilities a template's
// layouts provide, used to evaluate derivable layouts.
type layoutCapabilities struct {
	hasOneContent  bool
	hasTwoContent  bool
	hasBlankTitle  bool
	hasBlank       bool
	hasTitleHolder bool // any layout with a title placeholder
	hasBodyHolder  bool // any layout with a usable body/content placeholder
	hasLargeImage  bool // any layout with a large image placeholder
}

// scanCapabilities summarises the base capabilities of a layout set in one pass.
func scanCapabilities(layouts []types.LayoutMetadata) layoutCapabilities {
	var c layoutCapabilities
	for i := range layouts {
		l := &layouts[i]
		switch EffectiveCanonicalType(l) {
		case types.CanonicalLayoutOneContent:
			c.hasOneContent = true
		case types.CanonicalLayoutTwoContent:
			c.hasTwoContent = true
		case types.CanonicalLayoutBlankTitle:
			c.hasBlankTitle = true
		case types.CanonicalLayoutBlank:
			c.hasBlank = true
		}
		for _, ph := range l.Placeholders {
			switch ph.Type {
			case types.PlaceholderTitle:
				c.hasTitleHolder = true
			case types.PlaceholderBody, types.PlaceholderContent:
				if ph.MaxChars == 0 || ph.MaxChars >= minUsableBodyChars {
					c.hasBodyHolder = true
				}
			case types.PlaceholderImage:
				if ph.Bounds.Width > emuLargeImageWidth && ph.Bounds.Height > emuLargeImageHeight {
					c.hasLargeImage = true
				}
			}
		}
	}
	return c
}

// DerivableLayouts evaluates which higher-level layouts the template can produce
// from its base layouts, returning a deterministically-ordered list. A layout is
// "ready" when the engine can either select a native layout or synthesise/overlay
// one from existing placeholders; otherwise Missing names the absent prerequisite.
func DerivableLayouts(layouts []types.LayoutMetadata) []DerivableLayout {
	c := scanCapabilities(layouts)

	// A grid base (used by SVG/shape-grid patterns) needs a title-or-content
	// canvas to overlay; resolveVirtualLayout accepts a blank-title, a title
	// placeholder, or any body placeholder.
	gridBaseReady := c.hasBlankTitle || c.hasTitleHolder || c.hasBodyHolder
	gridBaseMissing := func() []string {
		if gridBaseReady {
			return nil
		}
		return []string{"no blank-title, title, or body placeholder to overlay a grid on"}
	}

	results := []DerivableLayout{
		derivableTwoContent("two-content", c),
		derivableTwoContent("comparison", c),
		derivableFullImage(c),
		derivableBlankTitle(c),
		gridPattern("stat-grid", gridBaseReady, gridBaseMissing()),
		gridPattern("timeline", gridBaseReady, gridBaseMissing()),
		gridPattern("journey", gridBaseReady, gridBaseMissing()),
		gridPattern("panel-layout", gridBaseReady, gridBaseMissing()),
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

// derivableTwoContent evaluates two-content / comparison: a native Two Content
// layout, or a One Content layout the synthesiser can split.
func derivableTwoContent(name string, c layoutCapabilities) DerivableLayout {
	if c.hasTwoContent || c.hasOneContent {
		return DerivableLayout{Name: name, Ready: true}
	}
	return DerivableLayout{
		Name:    name,
		Missing: []string{"no Two Content layout and no One Content layout to split into two columns"},
	}
}

// derivableFullImage evaluates full-image: a large image placeholder, or a blank
// canvas onto which a full-bleed picture can be placed.
func derivableFullImage(c layoutCapabilities) DerivableLayout {
	if c.hasLargeImage || c.hasBlank || c.hasBlankTitle {
		return DerivableLayout{Name: "full-image", Ready: true}
	}
	return DerivableLayout{
		Name:    "full-image",
		Missing: []string{"no large image placeholder and no blank canvas for a full-bleed image"},
	}
}

// derivableBlankTitle evaluates blank-title: a native blank-title layout or any
// layout carrying a title placeholder that can be cleared to a title-only canvas.
func derivableBlankTitle(c layoutCapabilities) DerivableLayout {
	if c.hasBlankTitle || c.hasTitleHolder {
		return DerivableLayout{Name: "blank-title", Ready: true}
	}
	return DerivableLayout{
		Name:    "blank-title",
		Missing: []string{"no blank-title layout and no layout with a title placeholder"},
	}
}

// gridPattern builds a DerivableLayout for an SVG/shape-grid pattern that shares
// the grid-base prerequisite.
func gridPattern(name string, ready bool, missing []string) DerivableLayout {
	return DerivableLayout{Name: name, Ready: ready, Missing: missing}
}

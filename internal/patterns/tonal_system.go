package patterns

import (
	"github.com/sebahrens/json2pptx/svggen"
)

// The tonal system (go-slide-creator-x5m8f).
//
// Every default fill a pattern draws plays one of four roles, and the role —
// not the pattern — picks the tone:
//
//	panel     a backdrop that GROUPS content (a tier band, a pillar shaft, a
//	          lane, a tile): the lightest neutral surface, tonalPanel.
//	content   a shape that IS the content (a step, a tier, a ring segment, a
//	          header, a tab): the accent's "Lighter N%" swatch, tonalContent
//	          and tonalRung, with the ink measured on it.
//	emphasis  the single item the slide is about: the solid accent,
//	          tonalEmphasis. At most one per exemplar.
//	anchor    a small dark mark that ties a label to a shape (a numbered
//	          badge): the neutral dark, tonalBadge.
//
// The mid grey (dk1 at 16%, NeutralTint16) that used to be the structural
// fill is no longer a default: a row of equal mid-grey rectangles groups
// nothing and leads nowhere. It survives in three deliberate places — as the
// fallback of tonalContent on a template whose accent tint cannot be told
// from its paper or from its own solid (a yellow accent), in the data ladders
// of the heat maps (neutral = "low", a value, not a structure) and in the
// explicit legacy styles.

const (
	// TonalLighterContent is the "Lighter N%" swatch of a content shape.
	TonalLighterContent = 80
	// TonalLighterDeep is the deeper swatch of a content shape that heads
	// others (a cap, a tab, a second rung).
	TonalLighterDeep = 60
	// TonalLighterPale is the swatch of a content shape that recedes behind
	// its siblings (an outer ring, a stage not yet reached).
	TonalLighterPale = 90

	// tonalPanelLighterBy is how much lighter (relative luminance) than the
	// accent's content swatch a template's declared subtle surface must be to
	// be kept as a panel: a panel darker than the shapes it groups would lead.
	tonalPanelLighterBy = 0.05
	// tonalPaperMin is the contrast a content swatch needs against the page
	// to read as a shape at all.
	tonalPaperMin = 1.12
	// tonalEmphasisMin is the contrast the solid accent needs against the
	// content swatch for the emphasised item to stand out from its siblings.
	tonalEmphasisMin = 1.6
)

// tonalLighter is color at PowerPoint's "Lighter pct%" swatch:
// L' = (1-pct)·L + pct in HSL, hue and theme link kept. A hex colour, whose
// modifiers the fill resolver ignores, is returned as the lightened hex.
func tonalLighter(color string, pct int) fillTone {
	lumMod := (100 - pct) * 1000
	if isHexColor(color) {
		if c, err := svggen.ParseColor(color); err == nil {
			return fillTone{Color: applyLumModOff(c, lumMod, 100000-lumMod).Hex()}
		}
		return neutralTone(NeutralTint8)
	}
	return fillTone{Color: color, LumMod: lumMod, LumOff: 100000 - lumMod}
}

// tonalAccentCollides reports whether the accent's content swatch fails as a
// structural fill on this template: too close to the paper to be seen, or too
// close to the solid accent for an emphasised sibling to stand out. Without a
// theme nothing can be measured and the swatch is kept.
func tonalAccentCollides(ctx ExpandContext, accent string) bool {
	swatch := tonalLighter(accent, TonalLighterContent)
	paper, okPaper := fillContrast(ctx, swatch, fillTone{Color: "lt1"})
	solid, okSolid := fillContrast(ctx, swatch, fillTone{Color: accent})
	if !okPaper || !okSolid {
		return false
	}
	return paper < tonalPaperMin || solid < tonalEmphasisMin
}

// tonalContent is the fill of a content shape: the accent's Lighter 80%
// swatch, or the mid neutral where that swatch collides (tonalAccentCollides).
func tonalContent(ctx ExpandContext, accent string) fillTone {
	return tonalRung(ctx, accent, TonalLighterContent)
}

// tonalRung is one rung of the accent ladder, as a "Lighter pct%" swatch.
// Where the accent's tint collides on this template the ladder is built from
// the neutral instead, at the ink coverage that matches the rung's depth
// (Lighter 90 / 80 / 60 -> dk1 at 8 / 16 / 24%), so the rungs keep their
// order.
func tonalRung(ctx ExpandContext, accent string, pct int) fillTone {
	if tonalAccentCollides(ctx, accent) {
		return neutralTone(tonalNeutralStep(pct))
	}
	return tonalLighter(accent, pct)
}

// tonalNeutralStep is the neutral ink coverage standing in for a "Lighter
// pct%" rung on a template whose accent tint collides.
func tonalNeutralStep(pct int) int {
	switch {
	case pct >= TonalLighterPale:
		return NeutralTint8
	case pct >= TonalLighterContent:
		return NeutralTint16
	default:
		return NeutralTint16 + (TonalLighterContent-pct)*2/5
	}
}

// tonalInk is the ink of text on a tonal fill, chosen by measurement.
func tonalInk(ctx ExpandContext, tone fillTone) string {
	return readableTextOn(ctx, tone, "dk1")
}

// tonalPanel is the fill of a panel or backdrop: the lightest neutral step,
// or the template's own subtle surface where it declares one that stays
// clearly lighter than the accent's content swatch.
func tonalPanel(ctx ExpandContext, accent string) fillTone {
	tone := neutralTone(NeutralTint4)
	v, ok := declaredSurface(ctx, "subtle")
	if !ok || isPageColor(v) {
		return tone
	}
	declared := fillTone{Color: v}
	panel, okP := effectiveFillColor(ctx, declared)
	swatch, okS := effectiveFillColor(ctx, tonalLighter(accent, TonalLighterContent))
	if okP && okS && panel.Luminance() < swatch.Luminance()+tonalPanelLighterBy {
		return tone
	}
	return declared
}

// tonalEmphasis is the fill and ink of the one emphasised item: the solid
// accent, deepened where white ink would not read on it.
func tonalEmphasis(ctx ExpandContext, accent string) (fillTone, string) {
	return accentFillAndInk(ctx, fillTone{Color: accent}, svggen.WCAGAANormal)
}

// tonalBadgeInkPct is the neutral dark of a badge when dk2 is black.
const tonalBadgeInkPct = 80

// tonalBadge is the fill and ink of a numbered badge: the neutral dark with
// the page colour as ink (the brand's own dark, or dk1 at 80% when dk2 is
// black). A badge is an anchor, not an emphasis, so it never takes the accent.
func tonalBadge(ctx ExpandContext) (fillTone, string) {
	tone := fillTone{Color: "dk2"}
	if isNearBlack(ctx, "dk2") {
		tone = neutralTone(tonalBadgeInkPct)
	}
	return tone, readableTextOn(ctx, tone, "lt1")
}

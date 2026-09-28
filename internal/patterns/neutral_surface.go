package patterns

import (
	"encoding/json"
)

// Neutral surfaces and rules (go-slide-creator-pgdkp, go-slide-creator-8xsj3).
//
// Pattern surfaces are neutral tints of the template's dk1 ink, not outlined
// white boxes and not 40–70% accent mid-tints. Filled shapes carry no outline:
// neighbours are separated by white gutters over a neutral field, or by two
// neutral steps (4% / 8%). The only strokes a pattern draws are thin row
// dividers (dk1 15%), a header underline, or a 2–3pt accent bar.
//
// The percentages are ink coverage: "dk1 4%" keeps 4% of dk1's lightness
// distance from white (lumMod 4000 + lumOff 96000 in OOXML's HSL terms), so
// on a black dk1 it is #F5F5F5 and on a navy dk1 a faint navy-grey — always
// the template's own ink, never a hard-coded cool or warm grey.
const (
	// NeutralTint4 is the card surface on white paper.
	NeutralTint4 = 4
	// NeutralTint8 is the second step of an alternating pair.
	NeutralTint8 = 8
	// NeutralTint16 is a structural box (value-chain step, stack tier).
	NeutralTint16 = 16
	// NeutralTint60 replaces a dk2 structural fill when dk2 is black.
	NeutralTint60 = 60
)

// neutralTone returns dk1 at pct% ink coverage.
func neutralTone(pct int) fillTone {
	return fillTone{Color: "dk1", LumMod: pct * 1000, LumOff: 100000 - pct*1000}
}

// neutralFillJSON renders neutralTone as a shape fill.
func neutralFillJSON(pct int) json.RawMessage {
	return neutralTone(pct).fillJSON()
}

// noLine is the explicit "no outline" line value for filled shapes.
var noLine = json.RawMessage(`"none"`)

// nearBlackLuminance is the relative luminance under which a theme colour
// reads as black. A dk2 that dark adds nothing a neutral does not, and a
// solid block of it is the heavy "structure" fill the review rejected.
const nearBlackLuminance = 0.02

// isNearBlack reports whether the named theme colour resolves to (near) black.
func isNearBlack(ctx ExpandContext, name string) bool {
	c, ok := resolveThemeColor(ctx, name)
	if !ok {
		return false
	}
	// A deep navy or forest dk2 (midnight-blue's #1B2A4A sits at 0.024) is a
	// brand colour, not black: require it to be achromatic as well as dark.
	hi := max(c.R, c.G, c.B)
	lo := min(c.R, c.G, c.B)
	return c.Luminance() < nearBlackLuminance && hi-lo <= nearBlackChroma
}

// nearBlackChroma is the largest RGB channel spread a "black" dk2 may have.
const nearBlackChroma = 24

// structuralDarkTone returns the dark structural fill: dk2 when it carries
// the template's brand (navy, forest), or dk1 at 60% when dk2 is black.
func structuralDarkTone(ctx ExpandContext) fillTone {
	if isNearBlack(ctx, "dk2") {
		return neutralTone(NeutralTint60)
	}
	return fillTone{Color: "dk2"}
}

// declaredSurface returns the template's declared value for a surface role
// and whether one was declared (a valid scheme name or hex).
func declaredSurface(ctx ExpandContext, role string) (string, bool) {
	const unset = "\x00"
	v := ctx.ResolveSurface(role, unset)
	return v, v != unset
}

// isPageColor reports whether a fill is the page colour (lt1 / bg1): a card
// in it is invisible on white paper.
func isPageColor(fill string) bool {
	return fill == "lt1" || fill == "bg1"
}

// surfaceFillJSON resolves one surface role as a card fill. A role the
// template declares keeps its colour — the template author chose it — unless
// it is the page colour, since a white card on white paper is not allowed; an
// undeclared role (whose generic lt2 is warm on some templates and cool on
// others) becomes the dk1 neutral step (go-slide-creator-pgdkp).
func surfaceFillJSON(ctx ExpandContext, role string, step int) json.RawMessage {
	if v, ok := declaredSurface(ctx, role); ok && !isPageColor(v) {
		data, _ := json.Marshal(v)
		return data
	}
	return neutralFillJSON(step)
}

// surfacePairJSON resolves the alternating subtle / paper pair used by
// zebra-striped rows and cards. Each side follows surfaceFillJSON; when only
// one side keeps a declared template colour, the other takes the 4% step, so
// the pair stays two distinct tints and never outlined-white beside filled.
func surfacePairJSON(ctx ExpandContext) (a, b json.RawMessage) {
	va, okA := declaredSurface(ctx, "subtle")
	vb, okB := declaredSurface(ctx, "paper")
	keepA := okA && !isPageColor(va)
	keepB := okB && !isPageColor(vb)
	switch {
	case keepA && keepB:
		a, _ = json.Marshal(va)
		b, _ = json.Marshal(vb)
	case keepA:
		a, _ = json.Marshal(va)
		b = neutralFillJSON(NeutralTint4)
	case keepB:
		a = neutralFillJSON(NeutralTint4)
		b, _ = json.Marshal(vb)
	default:
		a, b = neutralFillJSON(NeutralTint4), neutralFillJSON(NeutralTint8)
	}
	return a, b
}

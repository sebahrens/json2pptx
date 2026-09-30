package textfit

import (
	"strings"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// AtomicTokenWidthPct is the share of an available width a token that must
// never break — a KPI value such as "$4.2M", a single label word such as
// "PRODUCTION" — may occupy when its measured face is a stand-in for the
// template font. A substitute's metrics are a guess: modern-yellow's "Segoe
// UI" measures as Arial here, while LibreOffice draws it with a face ~20%
// wider, so a token measured to fit edge-to-edge renders as "$4.2" / "M" or
// "PRODUCTI" / "ON" (go-slide-creator-b7qqg.14 / .15).
const AtomicTokenWidthPct = 80

// FontSubstituted reports whether the measurer lacks the named face and would
// measure it with a substitute. An unnamed face and Arial measure with the
// embedded, metric-compatible Liberation Sans and are not substitutes.
func FontSubstituted(name string) bool {
	if name == "" || strings.EqualFold(name, "Arial") {
		return false
	}
	_, _, substituted := fontcache.Resolve(name, "")
	return substituted
}

// AtomicTokenWidthPt returns the width (points) a must-not-break token may be
// measured against: widthPt itself for an available face, AtomicTokenWidthPct
// of it for a substituted one.
func AtomicTokenWidthPt(fontName string, widthPt float64) float64 {
	if FontSubstituted(fontName) {
		return widthPt * AtomicTokenWidthPct / 100
	}
	return widthPt
}

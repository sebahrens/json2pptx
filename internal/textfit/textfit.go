// Package textfit calculates font scaling to fit text within PPTX placeholders.
//
// It uses tdewolff/canvas for font metrics to determine whether text overflows
// a placeholder's bounds, and returns OOXML-compatible fontScale and
// lnSpcReduction values for <a:normAutofit>.
package textfit

import (
	"image/color"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/tdewolff/canvas"
)

// FitResult contains the OOXML autofit parameters.
type FitResult struct {
	// FontScale is the font scale percentage in thousandths (e.g., 85000 = 85%).
	// 0 means no scaling needed (content fits at original size).
	FontScale int
	// LnSpcReduction is the line spacing reduction in thousandths (e.g., 20000 = 20%).
	// 0 means no reduction needed.
	LnSpcReduction int
	// Overflow is true if text still doesn't fit even at minimum settings.
	Overflow bool
}

// NeedsAutofit returns true if any scaling is needed.
func (r FitResult) NeedsAutofit() bool {
	return r.FontScale > 0 || r.LnSpcReduction > 0
}

// Params for Calculate.
type Params struct {
	// WidthEMU is the placeholder width in EMUs.
	WidthEMU int64
	// HeightEMU is the placeholder height in EMUs.
	HeightEMU int64
	// FontSizeHPt is the font size in hundredths of a point (e.g., 1800 = 18pt).
	// If 0, defaults to 2000 (20pt, matching typical slide master body level 1).
	FontSizeHPt int
	// FontName is the font family name (e.g., "Arial"). Falls back to "Arial".
	FontName string
	// Paragraphs is the list of text paragraphs to fit.
	Paragraphs []string
	// LineSpacing is the line spacing multiplier (e.g., 1.2). Defaults to 1.2.
	LineSpacing float64
	// ExtraSpacingPt is additional spacing per paragraph in points (e.g., spcBef + spcAft
	// from the slide master bodyStyle). Defaults to 0. Typical values: 8-12pt.
	// Used as uniform spacing when ExtraSpacingsPt is not set.
	ExtraSpacingPt float64
	// ExtraSpacingsPt is per-paragraph extra spacing in points. When set, overrides
	// ExtraSpacingPt for each corresponding paragraph. Paragraphs beyond the slice
	// length fall back to ExtraSpacingPt. This accounts for explicit spcBef on
	// bullet group headers and trailing body paragraphs.
	ExtraSpacingsPt []float64
	// LeftMarginsPt is per-paragraph additional left margin in points. Bullet
	// paragraphs inherit marL from the slide master bodyStyle (e.g., 30pt for
	// level 1 bullets) which reduces the available width for text wrapping.
	// Paragraphs beyond the slice length use 0 (no extra margin).
	LeftMarginsPt []float64
	// MinFontScalePct overrides the minimum font scale percentage floor.
	// Default (0) uses the package constant (60%). Dense content like 10+ bullet
	// lists can set this lower (e.g., 45) to allow more text to fit before
	// overflow trimming kicks in.
	MinFontScalePct int
}

const (
	// emuPerPoint converts points to EMU.
	emuPerPoint = int64(types.EMUPerPoint)
	// minFontScalePct is the minimum font scale percentage (OOXML floor).
	// At 60%, a 24pt capped body font floors at ~14pt, keeping dense
	// bullet-group content readable on projected slides. Content that
	// still overflows at this floor is trimmed with "..." by
	// trimOverflowParagraphs rather than shrinking to illegible sizes.
	minFontScalePct = 60
	// maxLnSpcReductionPct is the maximum line spacing reduction percentage.
	maxLnSpcReductionPct = 20
	// fontScaleStep is the decrement step for font scale (percentage points).
	fontScaleStep = 5
	// ptToMM converts points to millimeters.
	ptToMM = 0.3528
)

// Calculate determines the font scale and line spacing reduction needed to fit
// the given paragraphs within placeholder bounds.
func Calculate(p Params) FitResult {
	if len(p.Paragraphs) == 0 || p.WidthEMU <= 0 || p.HeightEMU <= 0 {
		return FitResult{}
	}

	// Apply defaults
	if p.FontSizeHPt <= 0 {
		p.FontSizeHPt = 2000 // 20pt, matching typical slide master body level 1
	}
	if p.FontName == "" {
		p.FontName = "Arial"
	}
	if p.LineSpacing <= 0 {
		p.LineSpacing = 1.2
	}

	// Convert placeholder bounds from EMU to points
	widthPt := float64(p.WidthEMU) / float64(emuPerPoint)
	heightPt := float64(p.HeightEMU) / float64(emuPerPoint)

	// Account for OOXML default margins (91440 EMU = 0.1 inch = 7.2pt on each side)
	const marginPt = 7.2
	usableWidthPt := widthPt - 2*marginPt
	usableHeightPt := heightPt - 2*marginPt
	if usableWidthPt <= 0 || usableHeightPt <= 0 {
		return FitResult{}
	}

	fontSizePt := float64(p.FontSizeHPt) / 100.0

	ff := fontcache.Get(p.FontName, "")
	if ff == nil {
		// Can't measure — let PowerPoint handle it with default autofit
		return FitResult{}
	}

	// Determine minimum font scale floor
	minScale := minFontScalePct
	if p.MinFontScalePct > 0 {
		minScale = p.MinFontScalePct
	}

	// Try fitting at 100% scale, then reduce
	for scalePct := 100; scalePct >= minScale; scalePct -= fontScaleStep {
		scale := float64(scalePct) / 100.0
		scaledFontPt := fontSizePt * scale

		totalHeight := estimateTextHeight(ff, p.Paragraphs, scaledFontPt, usableWidthPt, p.LineSpacing, p.ExtraSpacingPt, p.ExtraSpacingsPt, p.LeftMarginsPt)

		if totalHeight <= usableHeightPt {
			if scalePct == 100 {
				return FitResult{} // Fits at full size
			}
			return FitResult{
				FontScale: scalePct * 1000, // Convert to OOXML thousandths
			}
		}
	}

	// At minimum font scale, try reducing line spacing
	minScaleF := float64(minScale) / 100.0
	scaledFontPt := fontSizePt * minScaleF
	for lnReduction := 5; lnReduction <= maxLnSpcReductionPct; lnReduction += 5 {
		reducedSpacing := p.LineSpacing * (1.0 - float64(lnReduction)/100.0)
		totalHeight := estimateTextHeight(ff, p.Paragraphs, scaledFontPt, usableWidthPt, reducedSpacing, p.ExtraSpacingPt, p.ExtraSpacingsPt, p.LeftMarginsPt)

		if totalHeight <= usableHeightPt {
			return FitResult{
				FontScale:      minScale * 1000,
				LnSpcReduction: lnReduction * 1000,
			}
		}
	}

	// Still doesn't fit — return maximum reduction values and flag overflow
	return FitResult{
		FontScale:      minScale * 1000,
		LnSpcReduction: maxLnSpcReductionPct * 1000,
		Overflow:       true,
	}
}

// estimateTextHeight estimates the total height in points for the given paragraphs
// when rendered at the specified font size within the given width, with word wrapping.
// extraSpacingPt is the default per-paragraph spacing (e.g., spcBef + spcAft from slide master).
// perParaSpacings overrides extraSpacingPt for each corresponding paragraph index.
// leftMargins is per-paragraph left margin in points (from bullet marL); reduces available width.
func estimateTextHeight(ff *canvas.FontFamily, paragraphs []string, fontSizePt, widthPt, lineSpacing, extraSpacingPt float64, perParaSpacings, leftMargins []float64) float64 {
	face := ff.Face(fontSizePt*ptToMM, color.Black, canvas.FontRegular, canvas.FontNormal)

	lineHeightPt := fontSizePt * lineSpacing
	var totalHeight float64

	for i, para := range paragraphs {
		spacing := extraSpacingPt
		if i < len(perParaSpacings) {
			spacing = perParaSpacings[i]
		}
		if para == "" {
			// Empty paragraph still takes one line
			totalHeight += lineHeightPt + spacing
			continue
		}
		// Use per-paragraph width if bullet margins are specified
		effectiveWidth := widthPt
		if i < len(leftMargins) && leftMargins[i] > 0 {
			effectiveWidth = widthPt - leftMargins[i]
			if effectiveWidth < fontSizePt {
				effectiveWidth = fontSizePt // at least one character width
			}
		}
		lines := wrapText(face, para, effectiveWidth)
		totalHeight += float64(lines)*lineHeightPt + spacing
	}

	return totalHeight
}

// MaxFontForWidth returns the maximum font size in hundredths of a point (hPt)
// that allows the longest word in text to fit within widthEMU without character-level
// wrapping. This prevents titles like "Performance Overview" from breaking into
// "Perform / ance / Overvie / w" when the lstStyle font is excessively large
// (e.g., 350pt) relative to the placeholder width.
//
// The calculation uses binary search over font sizes, measuring each word with
// actual font metrics. Returns 0 if measurement fails (caller should skip capping).
func MaxFontForWidth(text string, widthEMU int64, fontName string) int {
	if text == "" || widthEMU <= 0 {
		return 0
	}

	ff := fontcache.Get(fontName, "")
	if ff == nil {
		ff = fontcache.Get("Arial", "")
	}
	if ff == nil {
		return 0
	}

	// Usable width after default OOXML margins (7.2pt each side)
	widthPt := float64(widthEMU) / float64(emuPerPoint)
	usableWidthPt := widthPt - 2*7.2
	if usableWidthPt <= 0 {
		return 0
	}

	// Find the longest word
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	// Binary search for the largest font size where every word fits on one line.
	// Search range: 12pt (144 hPt) to 500pt (50000 hPt).
	lo, hi := 144, 50000
	for lo < hi {
		mid := (lo + hi + 1) / 2 // round up to converge on largest valid size
		fontPt := float64(mid) / 100.0
		face := ff.Face(fontPt*ptToMM, color.Black, canvas.FontRegular, canvas.FontNormal)

		fits := true
		for _, w := range words {
			wl := canvas.NewTextLine(face, w, canvas.Left)
			if wl.Bounds().W() > usableWidthPt*ptToMM {
				fits = false
				break
			}
		}

		if fits {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	return lo
}

// wrapText estimates how many lines a paragraph will need when word-wrapped
// to fit within widthPt points.
func wrapText(face *canvas.FontFace, text string, widthPt float64) int {
	widthMM := widthPt * ptToMM
	words := strings.Fields(text)
	if len(words) == 0 {
		return 1
	}

	// Measure space width once for this font face, not per word
	spaceLine := canvas.NewTextLine(face, " ", canvas.Left)
	spaceWidth := spaceLine.Bounds().W()

	lines := 1
	var currentWidth float64

	for i, word := range words {
		wordLine := canvas.NewTextLine(face, word, canvas.Left)
		wordWidth := wordLine.Bounds().W()

		if i > 0 {
			if currentWidth+spaceWidth+wordWidth > widthMM {
				lines++
				currentWidth = wordWidth
			} else {
				currentWidth += spaceWidth + wordWidth
			}
		} else {
			currentWidth = wordWidth
			// Single word wider than available width — it wraps by character
			if wordWidth > widthMM && widthMM > 0 {
				lines = int(math.Ceil(wordWidth / widthMM))
				currentWidth = wordWidth - float64(lines-1)*widthMM
			}
		}
	}

	return lines
}

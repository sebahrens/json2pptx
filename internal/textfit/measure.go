package textfit

import (
	"image/color"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// RunMeasurement holds the result of measuring a single text run at a given
// font size within a width budget.
type RunMeasurement struct {
	// Lines is the number of wrapped lines at the given width.
	Lines int
	// RequiredEMU is the height in EMU needed to render those lines at the
	// given font size with default line spacing (1.2×).
	RequiredEMU int64
	// OverflowChars is the approximate number of characters that do not fit
	// within (widthEMU × maxLines). 0 if the text fits.
	OverflowChars int
	// Fits is true when OverflowChars == 0.
	Fits bool
}

// MeasureRun measures text rendered at fontPt within widthEMU using resolved
// font metrics. Pure function — no I/O, no mutation, no logging. Safe to call
// from any goroutine.
//
// fontName selects the font family (falls back to "Arial").
// maxLines caps how many lines are allowed; characters beyond that count as
// overflow. Pass 0 or negative to allow unlimited lines.
func MeasureRun(text string, fontName string, fontPt float64, widthEMU int64, maxLines int) RunMeasurement {
	if text == "" {
		return RunMeasurement{Lines: 0, Fits: true}
	}
	if widthEMU <= 0 || fontPt <= 0 {
		return RunMeasurement{Lines: 1, Fits: false, OverflowChars: len([]rune(text))}
	}

	ff := fontcache.Get(fontName, "")
	if ff == nil {
		ff = fontcache.Get("Arial", "")
	}
	if ff == nil {
		// Cannot measure — report single line, fits (let PowerPoint handle it).
		return RunMeasurement{Lines: 1, Fits: true}
	}

	// Convert EMU to points, subtract default OOXML margins (7.2pt each side).
	widthPt := float64(widthEMU)/float64(emuPerPoint) - 2*7.2
	if widthPt <= 0 {
		return RunMeasurement{Lines: 1, Fits: false, OverflowChars: len([]rune(text))}
	}

	face := ff.Face(fontPt*ptToMM, color.Black, canvas.FontRegular, canvas.FontNormal)
	lines := wrapText(face, text, widthPt)

	// Default line spacing 1.2×
	const defaultLineSpacing = 1.2
	lineHeightPt := fontPt * defaultLineSpacing
	totalHeightPt := float64(lines) * lineHeightPt
	requiredEMU := int64(math.Ceil(totalHeightPt * float64(emuPerPoint)))

	overflowChars := 0
	if maxLines > 0 && lines > maxLines {
		overflowChars = estimateOverflowChars(face, text, widthPt, maxLines)
	}

	return RunMeasurement{
		Lines:         lines,
		RequiredEMU:   requiredEMU,
		OverflowChars: overflowChars,
		Fits:          overflowChars == 0,
	}
}

// estimateOverflowChars approximates how many characters don't fit within
// maxLines of the given width. It walks words forward, consuming the first
// maxLines worth of space, then counts remaining runes.
func estimateOverflowChars(face *canvas.FontFace, text string, widthPt float64, maxLines int) int {
	widthMM := widthPt * ptToMM
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	spaceLine := canvas.NewTextLine(face, " ", canvas.Left)
	spaceWidth := spaceLine.Bounds().W()

	line := 1
	var currentWidth float64

	for i, word := range words {
		wordLine := canvas.NewTextLine(face, word, canvas.Left)
		wordWidth := wordLine.Bounds().W()
		wordRunes := len([]rune(word))

		if i == 0 {
			if wordWidth > widthMM && widthMM > 0 {
				// Single word wraps by character
				charsPerLine := int(math.Floor(float64(wordRunes) * widthMM / wordWidth))
				if charsPerLine < 1 {
					charsPerLine = 1
				}
				fittingChars := charsPerLine * maxLines
				if fittingChars < wordRunes {
					// Count remaining runes in this word + all subsequent words
					remaining := wordRunes - fittingChars
					for _, w := range words[1:] {
						remaining += len([]rune(w)) + 1 // +1 for space
					}
					return remaining
				}
				charLines := int(math.Ceil(float64(wordRunes) / float64(charsPerLine)))
				line = charLines
				currentWidth = wordWidth - float64(charLines-1)*widthMM
			} else {
				currentWidth = wordWidth
			}
			continue
		}

		if currentWidth+spaceWidth+wordWidth > widthMM {
			line++
			if line > maxLines {
				// Everything from this word onward is overflow
				remaining := wordRunes
				for _, w := range words[i+1:] {
					remaining += len([]rune(w)) + 1 // +1 for space
				}
				return remaining
			}
			currentWidth = wordWidth
		} else {
			currentWidth += spaceWidth + wordWidth
		}
	}

	return 0
}

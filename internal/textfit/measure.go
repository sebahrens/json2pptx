package textfit

import (
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
	Fits            bool
	FontFamily      string
	FontSubstituted bool
}

type StyledRun struct {
	Text string
	Bold bool
}

type StyledMeasureParams struct {
	Runs        []StyledRun
	FontName    string
	FontPt      float64
	WidthEMU    int64
	MaxLines    int
	InsetsPt    [4]float64 // left, top, right, bottom
	LineSpacing float64
}

// MeasureStyledRuns measures regular/bold mixed text using the same insets and
// line spacing that emission applies. Explicit newlines always start a line;
// missing fonts are reported through FontSubstituted.
func MeasureStyledRuns(p StyledMeasureParams) (RunMeasurement, error) {
	if p.FontPt <= 0 || p.WidthEMU <= 0 {
		return RunMeasurement{Lines: 1, Fits: false}, nil
	}
	ff, resolvedFont, substituted := fontcache.Resolve(p.FontName, "Arial")
	if ff == nil {
		return RunMeasurement{}, ErrNoFontCache
	}
	widthMM := (float64(p.WidthEMU)/float64(emuPerPoint) - p.InsetsPt[0] - p.InsetsPt[2]) * ptToMM
	if widthMM <= 0 {
		return RunMeasurement{Lines: 1, Fits: false}, nil
	}
	spacing := p.LineSpacing
	if spacing <= 0 {
		spacing = 1.2
	}
	lines := 1
	current := 0.0
	for _, run := range p.Runs {
		style := canvas.FontRegular
		if run.Bold {
			style = canvas.FontBold
		}
		face := newFace(ff, p.FontPt, style)
		parts := strings.Split(run.Text, "\n")
		for partIndex, part := range parts {
			if partIndex > 0 {
				lines++
				current = 0
			}
			for _, word := range strings.Fields(part) {
				wordWidth := canvas.NewTextLine(face, word, canvas.Left).Bounds().W()
				spaceWidth := canvas.NewTextLine(face, " ", canvas.Left).Bounds().W()
				needed := wordWidth
				if current > 0 {
					needed += spaceWidth
				}
				if current > 0 && current+needed > widthMM {
					lines++
					current = 0
					needed = wordWidth
				}
				if wordWidth > widthMM {
					extra := int(math.Ceil(wordWidth/widthMM)) - 1
					lines += extra
					current = wordWidth - float64(extra)*widthMM
				} else {
					current += needed
				}
			}
		}
	}
	if len(p.Runs) == 0 {
		lines = 0
	}
	requiredPt := float64(lines)*p.FontPt*spacing + p.InsetsPt[1] + p.InsetsPt[3]
	overflow := 0
	if p.MaxLines > 0 && lines > p.MaxLines {
		for _, run := range p.Runs {
			overflow += len([]rune(run.Text))
		}
	}
	return RunMeasurement{Lines: lines, RequiredEMU: int64(math.Ceil(requiredPt * float64(emuPerPoint))), OverflowChars: overflow, Fits: overflow == 0, FontFamily: resolvedFont, FontSubstituted: substituted}, nil
}

// MeasureRun measures text rendered at fontPt within widthEMU using resolved
// font metrics. Pure function — no I/O, no mutation, no logging. Safe to call
// from any goroutine.
//
// fontName selects the font family (falls back to "Arial").
// maxLines caps how many lines are allowed; characters beyond that count as
// overflow. Pass 0 or negative to allow unlimited lines.
//
// Returns ErrNoFontCache if neither the requested font nor "Arial" can be
// resolved from the font cache.
func MeasureRun(text string, fontName string, fontPt float64, widthEMU int64, maxLines int) (RunMeasurement, error) {
	if text == "" {
		return RunMeasurement{Lines: 0, Fits: true}, nil
	}
	if widthEMU <= 0 || fontPt <= 0 {
		return RunMeasurement{Lines: 1, Fits: false, OverflowChars: len([]rune(text))}, nil
	}

	ff, resolvedFont, substituted := fontcache.Resolve(fontName, "Arial")
	if ff == nil {
		return RunMeasurement{}, ErrNoFontCache
	}

	// Convert EMU to points, subtract default OOXML margins (7.2pt each side).
	widthPt := float64(widthEMU)/float64(emuPerPoint) - 2*7.2
	if widthPt <= 0 {
		return RunMeasurement{Lines: 1, Fits: false, OverflowChars: len([]rune(text))}, nil
	}

	face := newFace(ff, fontPt, canvas.FontRegular)
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
		Lines:           lines,
		RequiredEMU:     requiredEMU,
		OverflowChars:   overflowChars,
		Fits:            overflowChars == 0,
		FontFamily:      resolvedFont,
		FontSubstituted: substituted,
	}, nil
}

// MeasureLineWidth returns the width of the longest explicit line at fontPt.
// It does not add text-box insets; callers can add their actual cell margins.
func MeasureLineWidth(text, fontName string, fontPt float64) (int64, error) {
	if text == "" || fontPt <= 0 {
		return 0, nil
	}
	ff, _, _ := fontcache.Resolve(fontName, "Arial")
	if ff == nil {
		return 0, ErrNoFontCache
	}
	face := newFace(ff, fontPt, canvas.FontRegular)
	var widest float64
	for _, line := range strings.Split(text, "\n") {
		if width := canvas.NewTextLine(face, line, canvas.Left).Bounds().W(); width > widest {
			widest = width
		}
	}
	return int64(math.Ceil(widest / ptToMM * float64(emuPerPoint))), nil
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

	// overflowFrom counts the runes of words[j:] plus their separating spaces,
	// less the first `fitted` runes of words[j] that still fit.
	overflowFrom := func(j, fitted int) int {
		remaining := len([]rune(words[j])) - fitted
		for _, w := range words[j+1:] {
			remaining += len([]rune(w)) + 1 // +1 for space
		}
		return remaining
	}

	for i, word := range words {
		wordLine := canvas.NewTextLine(face, word, canvas.Left)
		wordWidth := wordLine.Bounds().W()
		wordRunes := len([]rune(word))

		if i > 0 {
			if currentWidth+spaceWidth+wordWidth <= widthMM {
				currentWidth += spaceWidth + wordWidth
				continue
			}
			line++
			if line > maxLines {
				// Everything from this word onward is overflow
				return overflowFrom(i, 0)
			}
		}
		currentWidth = wordWidth
		if wordWidth > widthMM && widthMM > 0 {
			// A word wider than the line wraps by character, wherever it sits
			// in the paragraph (go-slide-creator-s1uvj.12).
			charsPerLine := int(math.Floor(float64(wordRunes) * widthMM / wordWidth))
			if charsPerLine < 1 {
				charsPerLine = 1
			}
			charLines := int(math.Ceil(float64(wordRunes) / float64(charsPerLine)))
			if line+charLines-1 > maxLines {
				return overflowFrom(i, charsPerLine*(maxLines-line+1))
			}
			line += charLines - 1
			currentWidth = wordWidth - float64(charLines-1)*widthMM
		}
	}

	return 0
}

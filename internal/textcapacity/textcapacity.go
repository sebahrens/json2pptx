// Package textcapacity provides a single source of truth for text budget
// computation in slide cells and placeholders.
//
// All budgets use the embedded Liberation Sans font (metric-compatible with
// Arial) for deterministic cross-platform results. This avoids platform-
// dependent jitter between macOS (with system Arial) and Linux CI (which may
// have different font metrics). Budget values are exact — no rounding — because
// the embedded font is identical on every platform.
//
// # Font precedence for cell budget computation
//
// When determining font size for a cell budget, the following precedence
// applies (first non-zero wins):
//
//  1. Per-paragraph font size in the cell's text JSON
//  2. Cell-level text.size in the shape specification
//  3. Pattern override (HeaderSize / BodySize on the resolved cell)
//  4. Default: 11pt
//
// For shape_grid cells, any explicitly authored size below the renderer's
// minimum floor (shapegrid.MinTextSizePt, 12pt) is raised to that floor so the
// budget matches what the renderer produces. The unspecified-size default above
// (11pt) is not floored.
//
// # Post-markdown rule
//
// Character counts (both budget and actual) use post-markdown-conversion
// length. Markdown emphasis markers like **bold** are stripped before counting,
// so '**bold**' counts as 4 characters, not 8. This uses
// pptx.ConvertMarkdownEmphasis internally.
package textcapacity

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// lineSpacing is the line height as a multiple of font size (matches
// textfit.MeasureRun's default), and defaultInsetPt is the OOXML default top /
// bottom text inset. Both mirror what the renderer emits, so a measured block
// height can be compared with the box a viewer lays it out in.
const (
	lineSpacing    = 1.2
	defaultInsetPt = 7.2
)

// budgetFontName is the font used for all budget computations.
// Liberation Sans is embedded in the binary and metric-compatible with Arial,
// ensuring deterministic results regardless of host platform.
const budgetFontName = "Liberation Sans"

// Status classifies a cell or placeholder's text density.
type Status string

const (
	StatusUnderfilled Status = "underfilled" // DensityPct < UnderfilledPct
	StatusOptimal     Status = "optimal"     // UnderfilledPct <= DensityPct <= OverflowPct
	StatusOverflow    Status = "overflow"    // DensityPct > OverflowPct
)

// UnderfilledPct and OverflowPct are the published density bands.
//
// The underfill floor was 60% of a CHARACTER budget. Read as a height ratio it
// is far too high: across the 42-pattern realistic corpus the median cell fills
// 48% of its box, so a 60% floor called the median well-authored slide
// "underfilled" — 23 of 42 patterns had a majority of their cells flagged, and
// the only pattern-level text signal an agent had was noise pushing it to pad
// full slides (go-slide-creator-yj77). Patterns leave whitespace on purpose; a
// cell under ~a third of its height is the one that actually reads as empty.
const (
	UnderfilledPct = 35
	OverflowPct    = 110
)

// Budget describes the text capacity of a cell or placeholder at a given font size.
type Budget struct {
	MaxChars  int     // body-paragraph ceiling at resolved font size
	MaxLines  int     // maximum number of lines that fit
	FontPt    float64 // font size used for computation (points)
	WidthEMU  int64   // usable width in EMU
	HeightEMU int64   // usable height in EMU
	// AvailableHeightPt is the text height the box actually offers (height minus
	// the OOXML insets), in points.
	AvailableHeightPt float64
}

// Density extends Budget with actual usage and density classification.
//
// For a shape_grid cell, DensityPct is a HEIGHT ratio: the wrapped block's
// height over the height the cell offers. It used to be a character ratio
// against a single font size — the largest of the cell's paragraphs — which made
// every mixed-size cell nonsense in both directions: a stat-hero cell with a
// 120pt number plus three small support lines reported 911% "overflow" while
// rendering with room to spare, and 33 of 45 patterns reported most of their
// cells "underfilled" (go-slide-creator-yj77, go-slide-creator-lmpu).
type Density struct {
	Budget
	ActualChars int    // post-markdown character count of actual content
	DensityPct  int    // round(required height / available height * 100); 0 if no content
	Status      Status // underfilled, optimal, or overflow
	// RequiredHeightPt is the measured height of the wrapped text block: every
	// paragraph laid out at ITS OWN font size and summed. Zero when the cell has
	// no text.
	RequiredHeightPt float64
	// Lines is the total wrapped line count across paragraphs.
	Lines int
	// AutofitScale is the uniform font scale the renderer's <a:normAutofit/>
	// applies to make the block fit (1.0 when it already fits). Cells rely on
	// it routinely, which is why a >100% height ratio is not by itself a
	// rendering defect.
	AutofitScale float64
	// Fits reports whether the text fits at or above the autofit floor. False is
	// the genuine overflow: even the smallest shrink the renderer will apply
	// leaves text outside the cell.
	Fits bool
}

// ForPlaceholder computes text density for a template placeholder with given text.
func ForPlaceholder(ph types.PlaceholderInfo, text string) Density {
	fontPt := 11.0
	if ph.FontSize > 0 {
		fontPt = float64(ph.FontSize) / 100.0 // hundredths of a point → points
	}

	cleanText := stripMarkdown(text)
	budget := computeBudget(ph.Bounds.Width, ph.Bounds.Height, fontPt)
	return buildDensity(budget, len([]rune(cleanText)))
}

// ForResolvedGrid computes text density for every cell in a resolved grid.
// Cells without text content (icons, images, diagrams, tables) get a zero
// Density with StatusUnderfilled.
func ForResolvedGrid(result *shapegrid.ResolveResult) []Density {
	if result == nil {
		return nil
	}
	densities := make([]Density, len(result.Cells))
	for i, cell := range result.Cells {
		if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil {
			densities[i] = Density{Status: StatusUnderfilled}
			continue
		}
		paras, authoredInsets := extractCellParagraphs(cell)
		// The renderer lays text out inside a box smaller than the cell: both the
		// icon-overlay insets (ResolvedCell.TextInsets) and the authored text
		// insets are subtracted before text is placed (see
		// shapegrid.GenerateShapeXML, which adds the overlay insets onto the
		// authored Insets from buildTextBody). Mirror that here so the budget
		// reflects the box PowerPoint actually receives, not the full cell.
		w, h := effectiveTextRect(cell.CellBounds, cell.TextInsets, authoredInsets)
		densities[i] = measuredDensity(paras, w, h)
	}
	return densities
}

// effectiveTextRect subtracts the icon-overlay insets and authored text insets
// (both [L,T,R,B] in EMU) from a cell's bounds and returns the usable text
// width and height. Results are clamped at zero so over-large insets cannot
// yield a negative rectangle.
func effectiveTextRect(bounds pptx.RectEmu, overlay, authored [4]int64) (int64, int64) {
	w := bounds.CX - overlay[0] - overlay[2] - authored[0] - authored[2]
	h := bounds.CY - overlay[1] - overlay[3] - authored[1] - authored[3]
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}

// computeBudget determines how many characters fit in the given EMU rectangle
// at the specified font size, using the embedded Liberation Sans font for
// deterministic cross-platform results.
func computeBudget(widthEMU, heightEMU int64, fontPt float64) Budget {
	if widthEMU <= 0 || heightEMU <= 0 || fontPt <= 0 {
		return Budget{FontPt: fontPt, WidthEMU: widthEMU, HeightEMU: heightEMU}
	}

	lineHeightPt := fontPt * lineSpacing
	usableHeightPt := availableTextHeightPt(heightEMU)
	if usableHeightPt <= 0 {
		return Budget{FontPt: fontPt, WidthEMU: widthEMU, HeightEMU: heightEMU}
	}
	maxLines := int(math.Floor(usableHeightPt / lineHeightPt))
	if maxLines < 1 {
		maxLines = 1
	}

	// Use MeasureRun to binary-search for max chars that fit one line,
	// then multiply by maxLines.
	charsPerLine := binarySearchCharsPerLine(widthEMU, fontPt)
	if charsPerLine < 1 {
		// A box narrower than one glyph still holds (and wraps) one character
		// per line; a zero budget would hide the overflow entirely.
		charsPerLine = 1
	}
	maxChars := charsPerLine * maxLines

	return Budget{
		MaxChars:  maxChars,
		MaxLines:  maxLines,
		FontPt:    fontPt,
		WidthEMU:  widthEMU,
		HeightEMU: heightEMU,
	}
}

// binarySearchCharsPerLine finds how many average-width characters fit on one
// line at the given font size and width, using the embedded font's metrics.
func binarySearchCharsPerLine(widthEMU int64, fontPt float64) int {
	// Start with a reasonable upper bound estimate.
	// At 11pt, roughly 10 chars per inch; each inch = 914400 EMU.
	upperBound := int(float64(widthEMU) / (fontPt * 600)) // rough initial guess
	if upperBound < 10 {
		upperBound = 10
	}
	if upperBound > 1000 {
		upperBound = 1000
	}

	// Generate test strings of varying lengths using "n" (close to average width).
	lo, hi := 0, upperBound*2
	for lo < hi {
		mid := (lo + hi + 1) / 2
		testStr := strings.Repeat("n", mid)
		m, err := textfit.MeasureRun(testStr, budgetFontName, fontPt, widthEMU, 0)
		if err != nil || m.Lines > 1 {
			hi = mid - 1
		} else {
			lo = mid
		}
	}
	return lo
}

// buildDensity creates a Density from a Budget and actual character count.
func buildDensity(b Budget, actualChars int) Density {
	d := Density{
		Budget:      b,
		ActualChars: actualChars,
	}
	if b.MaxChars > 0 && actualChars > 0 {
		d.DensityPct = int(math.Round(float64(actualChars) / float64(b.MaxChars) * 100))
	}
	d.Status = statusForDensity(d.DensityPct)
	return d
}

// defaultCellFontPt is the size an unsized shape_grid text cell is MEASURED at.
// It mirrors the renderer's own default (shapegrid.DefaultTextSizePt, 14pt).
//
// It used to sit at a legacy 11pt budget default, deliberately independent of
// the renderer. That is defensible for a character budget but wrong for a height
// measurement: every unsized cell was measured 27% smaller than it renders
// (go-slide-creator-lmpu).
const defaultCellFontPt = shapegrid.DefaultTextSizePt

// cellParagraph is one paragraph of a cell's text with the size it renders at.
type cellParagraph struct {
	text   string
	fontPt float64
}

// extractCellParagraphs parses a resolved cell's shape text into paragraphs,
// each carrying its OWN effective font size, plus any authored text insets
// ([L,T,R,B] in EMU).
//
// The previous extractCellText summed every paragraph's characters but kept only
// the LARGEST paragraph size, so a cell mixing one big number with small support
// lines was budgeted as if all of its text were set at the big size
// (go-slide-creator-yj77).
func extractCellParagraphs(cell shapegrid.ResolvedCell) ([]cellParagraph, [4]int64) {
	if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
		return nil, [4]int64{}
	}

	raw := cell.ShapeSpec.Text

	// String shorthand: one paragraph at the default size.
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return []cellParagraph{{text: stripMarkdown(str), fontPt: defaultCellFontPt}}, [4]int64{}
	}

	var obj struct {
		Content     string  `json:"content"`
		Size        float64 `json:"size,omitempty"`
		InsetLeft   float64 `json:"inset_left,omitempty"`
		InsetRight  float64 `json:"inset_right,omitempty"`
		InsetTop    float64 `json:"inset_top,omitempty"`
		InsetBottom float64 `json:"inset_bottom,omitempty"`
		Paragraphs  []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size,omitempty"`
		} `json:"paragraphs,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, [4]int64{}
	}

	insets := [4]int64{
		pointsToEMU(obj.InsetLeft),
		pointsToEMU(obj.InsetTop),
		pointsToEMU(obj.InsetRight),
		pointsToEMU(obj.InsetBottom),
	}

	// Cell-level authored size, floored to the renderer's minimum, is the
	// fallback for paragraphs that do not set their own.
	cellPt := defaultCellFontPt
	if obj.Size > 0 {
		cellPt = shapegrid.EffectiveTextSizePt(obj.Size)
	}

	if len(obj.Paragraphs) > 0 {
		paras := make([]cellParagraph, 0, len(obj.Paragraphs))
		for _, p := range obj.Paragraphs {
			pt := cellPt
			if p.Size > 0 {
				// Mirror the renderer's per-paragraph floor.
				pt = shapegrid.EffectiveTextSizePt(p.Size)
			}
			paras = append(paras, cellParagraph{text: stripMarkdown(p.Content), fontPt: pt})
		}
		return paras, insets
	}

	return []cellParagraph{{text: stripMarkdown(obj.Content), fontPt: cellPt}}, insets
}

// measuredDensity lays each paragraph out at its own size inside the cell's text
// rectangle and reports the block height against the height available — the way
// the renderer stacks paragraphs. This is the measurement fit_overflow and
// cell_underfilled are derived from (go-slide-creator-lmpu, go-slide-creator-yj77).
func measuredDensity(paras []cellParagraph, widthEMU, heightEMU int64) Density {
	availablePt := availableTextHeightPt(heightEMU)
	chars := 0
	for _, p := range paras {
		chars += len([]rune(p.text))
	}

	// The dominant size — the one carrying the most characters — is what the
	// derived char budget and the reported FontPt describe. A 120pt number above
	// a 14pt caption is a 14pt text box with a number in it, not a 120pt one.
	dominantPt := dominantFontPt(paras)
	d := Density{
		Budget: Budget{
			FontPt:            dominantPt,
			WidthEMU:          widthEMU,
			HeightEMU:         heightEMU,
			AvailableHeightPt: availablePt,
		},
		ActualChars: chars,
	}
	if widthEMU <= 0 || availablePt <= 0 {
		d.Status = StatusUnderfilled
		return d
	}

	// Char budget at the dominant size, kept as the hint agents use to size text
	// (reduce_cell_text's max_chars) — derived, never the density itself.
	perLine := binarySearchCharsPerLine(widthEMU, dominantPt)
	if perLine < 1 {
		perLine = 1
	}
	d.MaxLines = int(math.Floor(availablePt / (dominantPt * lineSpacing)))
	if d.MaxLines < 1 {
		d.MaxLines = 1
	}
	d.MaxChars = perLine * d.MaxLines

	for _, p := range paras {
		if strings.TrimSpace(p.text) == "" {
			continue
		}
		lines := 1
		if m, err := textfit.MeasureRun(p.text, budgetFontName, p.fontPt, widthEMU, 0); err == nil && m.Lines > 0 {
			lines = m.Lines
		}
		d.Lines += lines
		d.RequiredHeightPt += float64(lines) * p.fontPt * lineSpacing
	}

	if chars == 0 {
		d.Status = StatusUnderfilled
		d.AutofitScale, d.Fits = 1, true
		return d
	}
	d.DensityPct = int(math.Round(d.RequiredHeightPt / availablePt * 100))
	d.Status = statusForDensity(d.DensityPct)
	d.AutofitScale, d.Fits = AutofitScale(paras, widthEMU, heightEMU)
	return d
}

// AutofitFloorScale is the smallest uniform font scale the renderer's
// <a:normAutofit/> shrink is assumed to reach. Below it, text is clipped rather
// than shrunk.
const AutofitFloorScale = 0.2

// ParagraphSpec is one paragraph of cell text at its rendered size, for callers
// outside this package that need the autofit prediction.
type ParagraphSpec struct {
	Text   string
	FontPt float64
}

// AutofitScaleFor predicts the shrink the renderer applies to fit the given
// paragraphs into a text rectangle, and whether they fit at all. It is the single
// implementation of the prediction: the fit report and the readability check must
// agree on the size text actually renders at (go-slide-creator-lmpu).
func AutofitScaleFor(paras []ParagraphSpec, widthEMU, heightEMU int64) (float64, bool) {
	converted := make([]cellParagraph, 0, len(paras))
	for _, p := range paras {
		converted = append(converted, cellParagraph{text: p.Text, fontPt: p.FontPt})
	}
	return AutofitScale(converted, widthEMU, heightEMU)
}

// AutofitScale is the internal form of AutofitScaleFor: the largest scale, in 2%
// steps, whose measured wrapped height fits the rectangle. The second result is
// false when even AutofitFloorScale overflows.
func AutofitScale(paras []cellParagraph, widthEMU, heightEMU int64) (float64, bool) {
	availablePt := availableTextHeightPt(heightEMU)
	if availablePt <= 0 || widthEMU <= 0 {
		return 1, true
	}
	fits := func(scale float64) bool {
		total := 0.0
		for _, p := range paras {
			if strings.TrimSpace(p.text) == "" {
				continue
			}
			pt := p.fontPt * scale
			m, err := textfit.MeasureRun(p.text, budgetFontName, pt, widthEMU, 0)
			if err != nil {
				return true // cannot measure: do not invent an overflow
			}
			total += float64(m.Lines) * pt * lineSpacing
		}
		return total <= availablePt
	}
	for step := 0; ; step++ {
		scale := 1.0 - 0.02*float64(step)
		if scale < AutofitFloorScale {
			break
		}
		if fits(scale) {
			return scale, true
		}
	}
	return AutofitFloorScale, false
}

// dominantFontPt returns the font size carrying the most characters, falling back
// to the largest size when no paragraph has text.
func dominantFontPt(paras []cellParagraph) float64 {
	best, bestChars, largest := 0.0, -1, 0.0
	for _, p := range paras {
		if p.fontPt > largest {
			largest = p.fontPt
		}
		if n := len([]rune(strings.TrimSpace(p.text))); n > bestChars {
			best, bestChars = p.fontPt, n
		}
	}
	if bestChars > 0 {
		return best
	}
	if largest > 0 {
		return largest
	}
	return defaultCellFontPt
}

// availableTextHeightPt is the text height a box of heightEMU offers, after the
// OOXML default top and bottom insets.
func availableTextHeightPt(heightEMU int64) float64 {
	if heightEMU <= 0 {
		return 0
	}
	return float64(heightEMU)/float64(types.EMUPerPoint) - 2*defaultInsetPt
}

// statusForDensity applies the published density bands.
func statusForDensity(pct int) Status {
	switch {
	case pct > OverflowPct:
		return StatusOverflow
	case pct >= UnderfilledPct:
		return StatusOptimal
	default:
		return StatusUnderfilled
	}
}

// pointsToEMU converts an authored point inset to EMU, mirroring the renderer's
// shapegrid.buildTextBody conversion. Non-positive insets yield 0 so a missing
// or negative authored inset never expands the budgeted rectangle.
func pointsToEMU(pt float64) int64 {
	if pt <= 0 {
		return 0
	}
	return int64(types.FromPoints(pt))
}

// stripMarkdown removes markdown emphasis markers from text and returns the
// plain-text representation. Uses pptx.ConvertMarkdownEmphasis to strip
// markers, then removes the resulting XML tags.
func stripMarkdown(text string) string {
	if !strings.Contains(text, "*") {
		return text
	}
	// ConvertMarkdownEmphasis turns **bold** → <b>bold</b>, *italic* → <i>italic</i>.
	// We strip the resulting tags to get the plain-text length.
	converted := pptx.ConvertMarkdownEmphasis(text)
	// Remove <b>, </b>, <i>, </i> tags.
	r := strings.NewReplacer("<b>", "", "</b>", "", "<i>", "", "</i>", "")
	return r.Replace(converted)
}

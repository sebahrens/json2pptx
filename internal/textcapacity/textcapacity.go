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

// budgetFontName is the font used for all budget computations.
// Liberation Sans is embedded in the binary and metric-compatible with Arial,
// ensuring deterministic results regardless of host platform.
const budgetFontName = "Liberation Sans"

// Status classifies a cell or placeholder's text density.
type Status string

const (
	StatusUnderfilled Status = "underfilled" // DensityPct < 60
	StatusOptimal     Status = "optimal"     // 60 <= DensityPct <= 110
	StatusOverflow    Status = "overflow"    // DensityPct > 110
)

// Budget describes the text capacity of a cell or placeholder at a given font size.
type Budget struct {
	MaxChars  int     // body-paragraph ceiling at resolved font size
	MaxLines  int     // maximum number of lines that fit
	FontPt    float64 // font size used for computation (points)
	WidthEMU  int64   // usable width in EMU
	HeightEMU int64   // usable height in EMU
}

// Density extends Budget with actual usage and density classification.
type Density struct {
	Budget
	ActualChars int    // post-markdown character count of actual content
	DensityPct  int    // round(actual / max * 100); 0 if no content
	Status      Status // underfilled, optimal, or overflow
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
		fontPt, actualChars := extractCellText(cell)
		budget := computeBudget(cell.CellBounds.CX, cell.CellBounds.CY, fontPt)
		densities[i] = buildDensity(budget, actualChars)
	}
	return densities
}

// computeBudget determines how many characters fit in the given EMU rectangle
// at the specified font size, using the embedded Liberation Sans font for
// deterministic cross-platform results.
func computeBudget(widthEMU, heightEMU int64, fontPt float64) Budget {
	if widthEMU <= 0 || heightEMU <= 0 || fontPt <= 0 {
		return Budget{FontPt: fontPt, WidthEMU: widthEMU, HeightEMU: heightEMU}
	}

	// Line height: 1.2× font size (matches textfit.MeasureRun default).
	const lineSpacing = 1.2
	lineHeightPt := fontPt * lineSpacing
	emuPerPt := float64(types.EMUPerPoint)

	// Usable height: subtract OOXML default insets (top + bottom = 7.2pt each).
	const insetPt = 7.2
	usableHeightPt := float64(heightEMU)/emuPerPt - 2*insetPt
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
	switch {
	case d.DensityPct > 110:
		d.Status = StatusOverflow
	case d.DensityPct >= 60:
		d.Status = StatusOptimal
	default:
		d.Status = StatusUnderfilled
	}
	return d
}

// defaultCellFontPt is the budget default font size (points) for shape_grid
// text cells whose size is not explicitly authored. It intentionally stays at
// the long-standing 11pt budget default and is independent of the shape_grid
// renderer's larger visual default (shapegrid defaultTextSizeHPt); only the
// renderer's minimum floor is mirrored here for authored sizes.
const defaultCellFontPt = 11.0

// extractCellText parses a resolved cell's shape text to determine font size
// and post-markdown character count. Returns (fontPt, actualChars).
//
// Authored sizes below the shape_grid renderer's floor are raised via
// shapegrid.EffectiveTextSizePt so the budget reflects the size the renderer
// actually produces (e.g. an authored size of 10 is rendered, and budgeted, at
// 12pt). Unspecified sizes keep defaultCellFontPt and are not floored.
func extractCellText(cell shapegrid.ResolvedCell) (float64, int) {
	if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
		return defaultCellFontPt, 0
	}

	raw := cell.ShapeSpec.Text

	// Try string shorthand (no authored size → default).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return defaultCellFontPt, len([]rune(stripMarkdown(s)))
	}

	// Object form with possible paragraphs array.
	var obj struct {
		Content    string  `json:"content"`
		Size       float64 `json:"size,omitempty"`
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size,omitempty"`
		} `json:"paragraphs,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return defaultCellFontPt, 0
	}

	// Cell-level authored size, floored to the renderer's minimum.
	fontPt := defaultCellFontPt
	if obj.Size > 0 {
		fontPt = shapegrid.EffectiveTextSizePt(obj.Size)
	}

	// If paragraphs array is present, concatenate all content.
	if len(obj.Paragraphs) > 0 {
		total := 0
		for _, p := range obj.Paragraphs {
			total += len([]rune(stripMarkdown(p.Content)))
			// Mirror the renderer's per-paragraph floor, then keep the largest
			// effective paragraph font size for the budget.
			if eff := shapegrid.EffectiveTextSizePt(p.Size); eff > fontPt {
				fontPt = eff
			}
		}
		return fontPt, total
	}

	return fontPt, len([]rune(stripMarkdown(obj.Content)))
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

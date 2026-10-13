package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Tables inherit a defined template default (or an explicit style GUID).
// The engine draws the consulting fallback only when no formatting definition
// is available. Appearance overrides are additive; typography is independent
// of style selection so switching templates cannot inflate a dense table.

// Engine-default type scale (go-slide-creator-1iiej): 12pt rows, 11pt header.
// The shrink chain below never takes either under the 10pt table
// readability floor (minFontSizeForTable).
const (
	engineDefaultRowFontSize    = 1200
	engineDefaultHeaderFontSize = 1100
)

// Engine-default rule weights in EMU.
const (
	engineDefaultHeaderRuleW = 12700 // 1pt under the header (and over a total)
	engineDefaultRowRuleW    = 6350  // 0.5pt between data rows
)

// IsEngineDefaultTableStyle reports whether an (unresolved) authored table
// style is a candidate for the engine fallback before template resolution: no
// use_table_style and a style_id that is empty or the engine-default GUID.
// header_background does not leave the default (go-slide-creator-87eu0): it
// only adds a header fill on top of it.
func IsEngineDefaultTableStyle(st types.TableStyle) bool {
	return !st.UseTableStyle &&
		(st.StyleID == "" || st.StyleID == types.DefaultTableStyleID)
}

// TableBaseFontSize is the unscaled body font size (hundredths of a point)
// the renderer starts its shrink chain from for a table with this style when
// no placeholder size is configured: 12pt for every appearance. Preflight and
// fit-report predictors use it so their predictions match the render.
func TableBaseFontSize(_ types.TableStyle) int {
	return engineDefaultRowFontSize
}

// columnScaledFontSize applies the wide-table shrink: a standard slide fits
// four columns at 18pt, so wider tables scale down linearly, never below the
// 10pt readability floor. Legacy tables scale their size by 4/numCols; an
// engine-default table (already 12pt) is only capped at the 18pt-equivalent
// size, so it keeps 12pt up to six columns. Reports whether it shrank.
func columnScaledFontSize(size, numCols int, compactTypography bool) (int, bool) {
	if numCols <= 4 {
		return size, false
	}
	base := size
	if compactTypography {
		base = defaultFontSize
	}
	scaled := int(float64(base) * 4.0 / float64(numCols))
	if scaled < minFontSizeForTable {
		scaled = minFontSizeForTable
	}
	if compactTypography && scaled >= size {
		return size, false
	}
	return scaled, true
}

// TableColumnScaledFontSize is columnScaledFontSize for the fit-report
// walker, keyed by the authored table style.
func TableColumnScaledFontSize(_ types.TableStyle, size, numCols int) int {
	scaled, _ := columnScaledFontSize(size, numCols, size <= engineDefaultRowFontSize)
	return scaled
}

// applyDefaultTableStyling marks engine-default tables and infers numeric
// column types. Explicit author choices always win: use_table_style, a
// non-default style_id, column_types, or a per-column alignment.
func applyDefaultTableStyling(table *types.TableSpec, config *TableRenderConfig) {
	if !config.templateStyle && IsEngineDefaultTableStyle(config.Style) {
		config.engineDefault = true
	}
	if len(config.Style.ColumnTypes) == 0 {
		config.Style.ColumnTypes = inferColumnTypes(table, config.ColumnAlignments)
	}
}

// tableStriped reports whether data rows get zebra stripes. The engine
// default draws none unless the author explicitly asks (striped: true);
// otherwise banding stays on unless explicitly switched off.
func tableStriped(config TableRenderConfig) bool {
	if config.engineDefault {
		return config.Style.Striped != nil && *config.Style.Striped
	}
	return config.Style.Striped == nil || *config.Style.Striped
}

// headerFontSize is the header-row font size for a table rendered at the
// given body size: 11/12 of the body for compact typography (never below the
// readability floor, never above the body), 110% of the body otherwise.
func headerFontSize(config TableRenderConfig, bodySize int) int {
	if !config.compactTypography {
		return int(float64(bodySize) * 1.1)
	}
	h := bodySize * engineDefaultHeaderFontSize / engineDefaultRowFontSize
	if h < minFontSizeForTable {
		h = minFontSizeForTable
	}
	if h > bodySize {
		h = bodySize
	}
	return h
}

// engineDefaultRuleXML returns a horizontal rule on side ("T" or "B") in the
// text color, tinted to 15% for the hairline row rules.
func engineDefaultRuleXML(side string, w int, hairline bool) string {
	clr := `<a:schemeClr val="tx1"/>`
	if hairline {
		clr = `<a:schemeClr val="tx1"><a:lumMod val="15000"/><a:lumOff val="85000"/></a:schemeClr>`
	}
	return fmt.Sprintf(`<a:ln%s w="%d" cap="flat" cmpd="sng"><a:solidFill>%s</a:solidFill></a:ln%s>`, side, w, clr, side)
}

// engineDefaultBordersXML returns the cell borders of the engine-default
// look when the author set no borders. Vertical rules are always off. The
// header carries the 1pt rule under it; the first data row repeats it on its
// top edge; other data rows carry a hairline above and, unless last, below.
// A totals row gets the 1pt rule above it.
func engineDefaultBordersXML(isHeader bool, rowIdx int, lastRow bool, overrides *cellBorderOverrides) string {
	none := func(side string) string { return fmt.Sprintf(`<a:ln%s w="0"><a:noFill/></a:ln%s>`, side, side) }
	var top, bottom string
	switch {
	case isHeader:
		top = none("T")
		bottom = engineDefaultRuleXML("B", engineDefaultHeaderRuleW, false)
	case rowIdx == 0:
		top = engineDefaultRuleXML("T", engineDefaultHeaderRuleW, false)
	default:
		top = engineDefaultRuleXML("T", engineDefaultRowRuleW, true)
	}
	if !isHeader {
		if lastRow {
			bottom = none("B")
		} else {
			bottom = engineDefaultRuleXML("B", engineDefaultRowRuleW, true)
		}
	}
	if overrides != nil {
		switch {
		case overrides.suppressTop:
			top = none("T")
		case overrides.totalsTopBorder:
			top = engineDefaultRuleXML("T", engineDefaultHeaderRuleW, false)
		}
		if overrides.suppressBottom {
			bottom = none("B")
		}
	}
	return none("L") + none("R") + top + bottom
}

// engineDefaultTableLevelBorders switches every table-level rule off so the
// cell-level rules are the only lines drawn and the referenced table style's
// grid never shows through.
func engineDefaultTableLevelBorders() string {
	const no = `<a:ln w="0"><a:noFill/></a:ln>`
	var b strings.Builder
	b.WriteString(`<a:tblBorders>`)
	for _, tag := range []string{"a:top", "a:bottom", "a:left", "a:right", "a:insideH", "a:insideV"} {
		fmt.Fprintf(&b, `<%s>%s</%s>`, tag, no, tag)
	}
	b.WriteString(`</a:tblBorders>`)
	return b.String()
}

// inferColumnTypes marks columns whose non-empty data cells are all numeric
// (numbers, currency, percentages, multiples, deltas) as "number" so they are
// right-aligned. Columns with an explicit alignment keep it ("text"). Returns
// nil when no column is numeric.
func inferColumnTypes(table *types.TableSpec, alignments []string) []string {
	numCols := len(table.Headers)
	if numCols == 0 || len(table.Rows) == 0 {
		return nil
	}
	colTypes := make([]string, numCols)
	found := false
	for col := 0; col < numCols; col++ {
		colTypes[col] = "text"
		if col < len(alignments) && alignments[col] != "" {
			continue
		}
		numeric, seen := 0, 0
		for _, row := range table.Rows {
			if col >= len(row) || row[col].IsMerged {
				continue
			}
			v := strings.TrimSpace(row[col].Content)
			if v == "" || v == "-" || v == "—" || strings.EqualFold(v, "n/a") {
				continue
			}
			seen++
			if isNumericCell(v) {
				numeric++
			}
		}
		if seen > 0 && numeric == seen {
			colTypes[col] = "number"
			found = true
		}
	}
	if !found {
		return nil
	}
	return colTypes
}

// numericCellRegexp matches common numeric cell renderings: optional sign or
// parentheses, currency symbol, digits with separators/decimals, and an
// optional unit suffix (%, x, pp, bps, K/M/B/bn/mn, pts).
var numericCellRegexp = regexp.MustCompile(
	`^[+\-−±(]?\s*[$€£¥]?\s*[+\-−]?\d[\d,.\s]*(?:[.,]\d+)?\s*(?:%|x|×|pp|bps|pts?|k|m|b|bn|mn|mm|t)?\)?$`)

// isNumericCell reports whether a cell's text reads as a number.
func isNumericCell(s string) bool {
	return numericCellRegexp.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// totalRowLabelRegexp matches first-column labels of total/summary rows.
var totalRowLabelRegexp = regexp.MustCompile(`^(?:grand\s+)?(?:total|totals|sum|subtotal|sub-total)\b`)

// isTotalRow reports whether a data row is a total/summary row by its label.
func isTotalRow(row []types.TableCell) bool {
	if len(row) == 0 {
		return false
	}
	return totalRowLabelRegexp.MatchString(strings.ToLower(strings.TrimSpace(row[0].Content)))
}

// headerTextColorXML returns the run fill for header text. On a filled
// header the text is lt1 or tx1, whichever contrasts more with the fill
// (resolved through the theme when one is available); dark scheme fills
// (accents, dk*, tx*) fall back to lt1 without a theme. The unfilled
// engine-default header uses the text color (tx1); a legacy unfilled header
// emits nothing.
func headerTextColorXML(config TableRenderConfig) string {
	const light = `<a:solidFill><a:schemeClr val="lt1"/></a:solidFill>`
	const dark = `<a:solidFill><a:schemeClr val="tx1"/></a:solidFill>`
	if config.Style.UseTableStyle {
		return ""
	}
	bg := strings.ToLower(strings.TrimSpace(config.Style.HeaderBackground))
	if bg == "" || bg == "none" {
		if config.engineDefault {
			return dark
		}
		return ""
	}
	if fill, ok := headerFillColor(bg, config.Theme); ok {
		white := svggen.Color{R: 255, G: 255, B: 255, A: 1}
		black := svggen.Color{A: 1}
		if c, ok := headerFillColor("dk1", config.Theme); ok {
			black = c
		}
		if c, ok := headerFillColor("lt1", config.Theme); ok {
			white = c
		}
		if fill.ContrastWith(white) >= fill.ContrastWith(black) {
			return light
		}
		return dark
	}
	switch bg {
	case "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "dk1", "dk2", "tx1", "tx2":
		return light
	}
	if config.engineDefault {
		return dark
	}
	return ""
}

// headerFillColor resolves a header_background value (hex or scheme name)
// to a concrete color, using the theme for scheme names.
func headerFillColor(bg string, theme *types.ThemeInfo) (svggen.Color, bool) {
	hex := bg
	if !strings.HasPrefix(hex, "#") {
		if theme == nil {
			return svggen.Color{}, false
		}
		hex = resolveSchemeColorToHex(bg, theme.Colors)
		if hex == "" {
			return svggen.Color{}, false
		}
	}
	c, err := svggen.ParseColor(hex)
	if err != nil {
		return svggen.Color{}, false
	}
	return c, true
}

// contentRowHeight is the minimum row height for content-driven tables: a
// single line at the table font plus cell insets, never below the legacy
// defaultRowHeight floor used by the truncation math. Renderers grow rows
// that need more lines.
func contentRowHeight(fontSizeHPt int) int64 {
	h := compactRowHeight(fontSizeHPt)
	if h < defaultRowHeight {
		h = defaultRowHeight
	}
	return h
}

// compactRowHeight is the row pitch a table falls back to when its rows do not
// fit the frame at contentRowHeight's 0.4in floor: the text's own line height
// plus the cell insets — about 0.3in at 12pt, the pitch of a consulting price
// list or risk register. planTableRows tries it before truncating a row
// (go-slide-creator-plg7r); rows whose text wraps still grow past it.
func compactRowHeight(fontSizeHPt int) int64 {
	return int64(float64(fontSizeHPt)/100.0*1.2*12700) + cellMargin*2
}

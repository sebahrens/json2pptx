package generator

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Per-run contrast bars for brand-coloured text (go-slide-creator-tinsz).
//
// The shape-grid pass judges a text body against one background, and it used
// to take the bar from the body's SMALLEST run: an 11pt label beside a 40pt
// KPI value set 4.5:1 for both. That is right for a neutral ink (lt1 / dk1):
// one cell must not come out half white, half black (go-slide-creator-z668n).
// It is wrong for a brand accent written as a display colour: p-style's
// #FD5108 reads 3.3:1 on white, clears the WCAG large-text bar on every 40pt
// value and hero number, and was still swapped to black — the deck lost its
// accent exactly where consulting slides put it. So a non-neutral foreground
// is judged per run, at that run's own size and weight; the 11pt label is
// still fixed while the 40pt value keeps the accent.

// textPropsOpenRegexp matches the opening tag of a run / paragraph-default /
// end-of-paragraph property block, capturing the element name and attributes.
var textPropsOpenRegexp = regexp.MustCompile(`<a:(rPr|defRPr|endParaRPr)\b([^>]*)>`)

var (
	textPropsSizeRegexp = regexp.MustCompile(`\bsz="(\d+)"`)
	textPropsBoldRegexp = regexp.MustCompile(`\bb="([01])"`)
)

// textPropsBlock is one property block inside a text body: its byte span and
// the size / weight its opening tag declares.
type textPropsBlock struct {
	start, end int
	pt         float64 // 0 when the tag declares no size
	bold       bool
}

// textPropsBlocks returns the property blocks of body in order. Text colours
// live inside these blocks (a:solidFill under a:rPr / a:defRPr /
// a:endParaRPr), so they partition every colour the contrast pass rewrites.
func textPropsBlocks(body string) []textPropsBlock {
	var out []textPropsBlock
	for _, loc := range textPropsOpenRegexp.FindAllStringSubmatchIndex(body, -1) {
		if len(out) > 0 && loc[0] < out[len(out)-1].end {
			continue // inside the previous block (not expected in OOXML)
		}
		name := body[loc[2]:loc[3]]
		attrs := body[loc[4]:loc[5]]
		blk := textPropsBlock{start: loc[0], end: loc[1]}
		if !strings.HasSuffix(attrs, "/") {
			closing := "</a:" + name + ">"
			idx := strings.Index(body[loc[1]:], closing)
			if idx < 0 {
				continue
			}
			blk.end = loc[1] + idx + len(closing)
		}
		if m := textPropsSizeRegexp.FindStringSubmatch(attrs); m != nil {
			if h, err := strconv.Atoi(m[1]); err == nil && h > 0 {
				blk.pt = float64(h) / 100
			}
		}
		if m := textPropsBoldRegexp.FindStringSubmatch(attrs); m != nil {
			blk.bold = m[1] == "1"
		}
		out = append(out, blk)
	}
	return out
}

// blockColorIsNeutral reports whether the text colour in a property block is
// a pure neutral (white / black / lt1 / dk1 ...). A block without a colour
// counts as neutral: it inherits, and the body-wide bar applies.
func blockColorIsNeutral(block string) bool {
	m := textColorRunRegexp.FindStringSubmatch(block)
	if m == nil {
		return true
	}
	if m[1] == "srgbClr" {
		return isNeutralExtremeHex(m[2])
	}
	return isNeutralExtremeScheme(m[2])
}

// blockThreshold is the WCAG bar a property block's colour must clear: the
// body's smallest-text bar for neutral inks, the block's own size bar for a
// brand colour. A block that declares no size inherits, so it keeps the body
// bar too.
func blockThreshold(b textPropsBlock, block string, bodyThreshold float64) float64 {
	if b.pt <= 0 || blockColorIsNeutral(block) {
		return bodyThreshold
	}
	return contrastThresholdFor(b.pt, b.bold)
}

// mapTextPropsBlocks rewrites each property block of body with fn, passing the
// block's contrast bar. Text outside the blocks is left untouched.
func mapTextPropsBlocks(body string, bodyThreshold float64, fn func(block string, threshold float64) string) string {
	blocks := textPropsBlocks(body)
	if len(blocks) == 0 {
		return body
	}
	var sb strings.Builder
	last := 0
	for _, b := range blocks {
		sb.WriteString(body[last:b.start])
		block := body[b.start:b.end]
		sb.WriteString(fn(block, blockThreshold(b, block, bodyThreshold)))
		last = b.end
	}
	sb.WriteString(body[last:])
	return sb.String()
}

// textColorThreshold is one text colour of a body (resolved hex) with the bar
// it is judged at.
type textColorThreshold struct {
	hex       string
	threshold float64
}

// textColorsWithThresholds returns the distinct (colour, bar) pairs of a text
// body, in first-seen order. A brand colour used at two sizes yields two pairs.
func textColorsWithThresholds(body string, bodyThreshold float64, themeColors []types.ThemeColor) []textColorThreshold {
	var out []textColorThreshold
	seen := map[textColorThreshold]bool{}
	mapTextPropsBlocks(body, bodyThreshold, func(block string, threshold float64) string {
		for _, hex := range textColorsIn(block, themeColors) {
			k := textColorThreshold{hex, threshold}
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
		return block
	})
	return out
}

// replaceTextColorAt rewrites fromHex to toHex only in the property blocks
// judged at threshold.
func replaceTextColorAt(body, fromHex, toHex string, threshold, bodyThreshold float64, themeColors []types.ThemeColor) string {
	return mapTextPropsBlocks(body, bodyThreshold, func(block string, t float64) string {
		if t != threshold {
			return block
		}
		return replaceTextColor(block, fromHex, toHex, themeColors)
	})
}

// replaceFailingTextColor rewrites fromHex to toHex in the property blocks
// where fromHex misses the block's own bar against bg. Neutral inks carry the
// body bar in every block, so they are rewritten wherever the body failed.
func replaceFailingTextColor(body, fromHex, toHex string, bg svggen.Color, bodyThreshold float64, themeColors []types.ThemeColor) string {
	from, err := svggen.ParseColor(fromHex)
	if err != nil {
		return replaceTextColor(body, fromHex, toHex, themeColors)
	}
	ratio := from.ContrastWith(bg)
	return mapTextPropsBlocks(body, bodyThreshold, func(block string, t float64) string {
		if ratio >= t && t < bodyThreshold {
			return block // a large brand-coloured run that clears its own bar
		}
		return replaceTextColor(block, fromHex, toHex, themeColors)
	})
}

// bodyContrastThreshold is the body-wide bar: the smallest run's.
func bodyContrastThreshold(body string) float64 {
	return contrastThresholdFor(smallestTextPt(body))
}

// isLightBackground reports whether bg is light enough that a failing brand
// colour is fixed by darkening it.
func isLightBackground(bg svggen.Color) bool {
	return bg.ContrastWith(svggen.MustParseColor("#000000")) > bg.ContrastWith(svggen.MustParseColor("#FFFFFF"))
}

package svggen

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// mmToPxFactor converts mm to CSS pixels at 96 DPI.
const mmToPxFactor = 96.0 / 25.4 // ≈ 3.7795275590551

// replaceViewBox replaces viewBox="0 0 W H" with pixel values.
var viewBoxRe = regexp.MustCompile(`viewBox="0 0 [0-9.]+ [0-9.]+"`)

func replaceViewBox(s string, widthPx, heightPx float64) string {
	return viewBoxRe.ReplaceAllString(s, fmt.Sprintf(`viewBox="0 0 %.0f %.0f"`, widthPx, heightPx))
}

// scalePathDataInSVG finds all d="..." attributes and scales their coordinates.
var pathDataRe = regexp.MustCompile(`\bd="([^"]*)"`)

func scalePathDataInSVG(s string) string {
	return pathDataRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the path data between d=" and "
		inner := match[3 : len(match)-1] // strip d=" and trailing "
		scaled := scalePathData(inner)
		return `d="` + scaled + `"`
	})
}

// scalePathData scales all coordinate values in SVG path data by mmToPxFactor.
// Handles M, L, H, V, C, S, Q, T, A, and Z commands.
// For arc (A) commands, rotation and flags are NOT scaled.
func scalePathData(d string) string {
	var result strings.Builder
	result.Grow(len(d) * 2) // pre-allocate for scaled numbers

	i := 0
	var cmd byte
	paramIdx := 0     // parameter index within current command
	paramsPerCmd := 0  // total params per command instance

	for i < len(d) {
		ch := d[i]

		// Skip whitespace and commas
		if ch == ' ' || ch == ',' || ch == '\n' || ch == '\r' || ch == '\t' {
			result.WriteByte(ch)
			i++
			continue
		}

		// Command letter
		if isPathCommand(ch) {
			result.WriteByte(ch)
			cmd = ch
			paramIdx = 0
			paramsPerCmd = pathParamCount(cmd)
			i++
			continue
		}

		// Number (coordinate or flag)
		if ch == '-' || ch == '+' || ch == '.' || (ch >= '0' && ch <= '9') {
			// SVG arc flags (large-arc and sweep at indices 3,4) are single binary
			// digits (0 or 1) that can be concatenated with the next coordinate
			// without a separator: "A10 10 0 0120 30" = A rx=10 ry=10 rot=0
			// large-arc=0 sweep=1 x=20 y=30. We must consume only 1 char for flags.
			if isArcFlag(cmd, paramIdx, paramsPerCmd) {
				result.WriteByte(ch)
				i++
				paramIdx++
				continue
			}

			numStr, end := readNumber(d, i)
			i = end

			shouldScale := shouldScaleParam(cmd, paramIdx, paramsPerCmd)
			if shouldScale {
				val, err := strconv.ParseFloat(numStr, 64)
				if err == nil {
					scaled := val * mmToPxFactor
					result.WriteString(formatScaledNum(scaled))
				} else {
					result.WriteString(numStr) // fallback: keep original
				}
			} else {
				result.WriteString(numStr) // flags/rotation: keep original
			}

			paramIdx++
			if paramsPerCmd > 0 && paramIdx >= paramsPerCmd {
				paramIdx = 0 // reset for implicit repeated command
			}
			continue
		}

		// Unknown character — keep as-is
		result.WriteByte(ch)
		i++
	}

	return result.String()
}

func isPathCommand(ch byte) bool {
	switch ch {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v',
		'C', 'c', 'S', 's', 'Q', 'q', 'T', 't',
		'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

// pathParamCount returns the number of parameters per command instance.
func pathParamCount(cmd byte) int {
	switch cmd {
	case 'M', 'm', 'L', 'l', 'T', 't':
		return 2
	case 'H', 'h', 'V', 'v':
		return 1
	case 'C', 'c':
		return 6
	case 'S', 's', 'Q', 'q':
		return 4
	case 'A', 'a':
		return 7
	case 'Z', 'z':
		return 0
	}
	return 0
}

// isArcFlag returns true if the current parameter position is an arc flag
// (large-arc-flag at index 3 or sweep-flag at index 4). Arc flags are single
// binary digits (0 or 1) that may be concatenated with subsequent numbers
// without any separator, so they need special single-character parsing.
func isArcFlag(cmd byte, paramIdx, paramsPerCmd int) bool {
	if paramsPerCmd == 0 {
		return false
	}
	if cmd != 'A' && cmd != 'a' {
		return false
	}
	idx := paramIdx % paramsPerCmd
	return idx == 3 || idx == 4
}

// shouldScaleParam returns whether a parameter at the given index should be scaled.
// For arc commands, rotation (index 2), large-arc-flag (index 3), and sweep-flag (index 4) are not scaled.
func shouldScaleParam(cmd byte, paramIdx, paramsPerCmd int) bool {
	if paramsPerCmd == 0 {
		return false
	}
	idx := paramIdx % paramsPerCmd

	switch cmd {
	case 'A', 'a':
		// A: rx(0) ry(1) rotation(2) large-arc(3) sweep(4) x(5) y(6)
		// Scale rx, ry, x, y. Don't scale rotation, large-arc, sweep.
		return idx == 0 || idx == 1 || idx == 5 || idx == 6
	default:
		return true // scale all coordinate params
	}
}

// readNumber reads a number starting at position i and returns the string and end position.
func readNumber(d string, i int) (string, int) {
	start := i
	// Optional sign
	if i < len(d) && (d[i] == '-' || d[i] == '+') {
		i++
	}
	// Integer part
	for i < len(d) && d[i] >= '0' && d[i] <= '9' {
		i++
	}
	// Decimal part
	if i < len(d) && d[i] == '.' {
		i++
		for i < len(d) && d[i] >= '0' && d[i] <= '9' {
			i++
		}
	}
	// Exponent part (e.g., 1e-5)
	if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
		i++
		if i < len(d) && (d[i] == '-' || d[i] == '+') {
			i++
		}
		for i < len(d) && d[i] >= '0' && d[i] <= '9' {
			i++
		}
	}
	if i == start {
		// No number found, advance one character to avoid infinite loop
		return string(d[i]), i + 1
	}
	return d[start:i], i
}

func formatScaledNum(v float64) string {
	// Use enough precision to avoid visible rounding, but trim trailing zeros
	s := strconv.FormatFloat(v, 'f', 4, 64)
	// Trim trailing zeros after decimal point
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// scaleCoordAttributes scales x, y, cx, cy, r, x1, y1, x2, y2, fx, fy, fr attributes.
var coordAttrRe = regexp.MustCompile(`\b(x|y|cx|cy|r|rx|ry|x1|y1|x2|y2|fx|fy|fr)="([^"]*)"`)

func scaleCoordAttributes(s string) string {
	return coordAttrRe.ReplaceAllStringFunc(s, func(match string) string {
		eqIdx := strings.Index(match, `="`)
		if eqIdx < 0 {
			return match
		}
		attrName := match[:eqIdx]
		val := match[eqIdx+2 : len(match)-1]

		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return match
		}
		scaled := f * mmToPxFactor
		return fmt.Sprintf(`%s="%s"`, attrName, formatScaledNum(scaled))
	})
}

// scaleFontSizes scales font-size values in CSS font shorthand: "font: [italic] [weight] Npx family"
var fontSizePxInStyleRe = regexp.MustCompile(`(\d+\.?\d*)px(\s)`)

func scaleFontSizes(s string) string {
	return fontSizePxInStyleRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the number before "px"
		numStr := match[:len(match)-3] // strip "px" + trailing space/char
		trailing := match[len(match)-1:]
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return match
		}
		scaled := f * mmToPxFactor
		return formatScaledNum(scaled) + "px" + trailing
	})
}

// scaleStrokeWidths scales stroke-width values in style attributes.
var strokeWidthRe = regexp.MustCompile(`stroke-width:([0-9.]+)`)

func scaleStrokeWidths(s string) string {
	return strokeWidthRe.ReplaceAllStringFunc(s, func(match string) string {
		numStr := match[len("stroke-width:"):]
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return match
		}
		scaled := f * mmToPxFactor
		return "stroke-width:" + formatScaledNum(scaled)
	})
}

// scaleTranslateTransforms scales translate(x,y) values in transform attributes.
var translateRe = regexp.MustCompile(`translate\(([^)]+)\)`)

func scaleTranslateTransforms(s string) string {
	return translateRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[len("translate(") : len(match)-1]
		parts := strings.Split(inner, ",")
		var scaled []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			f, err := strconv.ParseFloat(p, 64)
			if err != nil {
				scaled = append(scaled, p)
				continue
			}
			scaled = append(scaled, formatScaledNum(f*mmToPxFactor))
		}
		return "translate(" + strings.Join(scaled, ",") + ")"
	})
}

// scaleMatrixTransforms scales all values in matrix(a,b,c,d,e,f) transforms.
// The canvas library outputs image transforms as matrix() where all values
// are in mm coordinates. Since the SVG viewBox uses CSS pixels, all six
// components must be scaled: a,b,c,d convert source pixels to output units,
// and e,f are translation offsets — all need mm→px conversion.
var matrixRe = regexp.MustCompile(`matrix\(([^)]+)\)`)

func scaleMatrixTransforms(s string) string {
	return matrixRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[len("matrix(") : len(match)-1]
		parts := strings.Split(inner, ",")
		var scaled []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			f, err := strconv.ParseFloat(p, 64)
			if err != nil {
				scaled = append(scaled, p)
				continue
			}
			scaled = append(scaled, formatScaledNum(f*mmToPxFactor))
		}
		return "matrix(" + strings.Join(scaled, ",") + ")"
	})
}

// scaleGradientCoords handles gradient coordinate attributes that are inline
// rather than in separate XML attributes. For gradients using gradientUnits="userSpaceOnUse",
// their coordinate attributes (x1, y1, x2, y2, cx, cy, r, fx, fy, fr) are
// already handled by scaleCoordAttributes. This function handles any additional
// coordinate values in gradient transforms.
func scaleGradientCoords(s string) string {
	// Gradient coordinates like x1, y1, x2, y2, cx, cy, r, fx, fy, fr
	// are already handled by scaleCoordAttributes since they appear as
	// standard XML attributes. No additional handling needed.
	return s
}

// scaleSVGStopOffset is intentionally NOT implemented because stop offsets (0-1) are ratios.
// They should NOT be scaled.

// --- Helper for detecting the SVG <style> section to avoid scaling font data ---

// splitSVGContentAndStyle separates the SVG content from the trailing <style> and </svg>.
// This prevents the base64 font data in <style> from being accidentally modified.
func splitSVGContentAndStyle(s string) (content string, suffix string) {
	// The canvas library outputs <style>...</style></svg> at the end.
	// Also there may be <defs>...</defs> before <style>.
	styleIdx := strings.LastIndex(s, "<style>")
	defsIdx := strings.LastIndex(s, "<defs>")

	splitAt := len(s)
	if styleIdx >= 0 && styleIdx < splitAt {
		splitAt = styleIdx
	}
	if defsIdx >= 0 && defsIdx < splitAt {
		splitAt = defsIdx
	}

	return s[:splitAt], s[splitAt:]
}

// scaleSVGToPixelCoordsSafe is the production entry point that avoids modifying
// <style> (base64 font data) and <defs> (gradient definitions) incorrectly.
func scaleSVGToPixelCoordsSafe(svgContent []byte, widthMM, heightMM float64) []byte {
	widthPx := math.Round(widthMM * mmToPxFactor)
	heightPx := math.Round(heightMM * mmToPxFactor)

	s := string(svgContent)

	// Split into content (paths, text, etc.) and suffix (defs, style, </svg>)
	content, suffix := splitSVGContentAndStyle(s)

	// 1. Replace SVG tag viewport and viewBox in content
	content = svgViewportMMRe.ReplaceAllString(content, fmt.Sprintf(`width="%.0f" height="%.0f"`, widthPx, heightPx))
	content = replaceViewBox(content, widthPx, heightPx)

	// 2. Scale path data
	content = scalePathDataInSVG(content)

	// 3. Scale coordinate attributes
	content = scaleCoordAttributes(content)

	// 4. Scale font sizes
	content = scaleFontSizes(content)

	// 5. Scale stroke widths
	content = scaleStrokeWidths(content)

	// 6. Scale transform translate values
	content = scaleTranslateTransforms(content)

	// 7. Scale matrix() transform values (used by canvas DrawImage)
	content = scaleMatrixTransforms(content)

	// Handle defs/style suffix: scale gradient coordinate attributes but NOT font data
	if strings.Contains(suffix, "<defs>") {
		defsEnd := strings.Index(suffix, "</defs>")
		if defsEnd > 0 {
			defsEnd += len("</defs>")
			defsPart := suffix[:defsEnd]
			rest := suffix[defsEnd:]
			// Scale gradient coordinates in defs
			defsPart = scaleCoordAttributes(defsPart)
			defsPart = scaleTranslateTransforms(defsPart)
			defsPart = scaleMatrixTransforms(defsPart)
			suffix = defsPart + rest
		}
	}

	var buf bytes.Buffer
	buf.Grow(len(content) + len(suffix))
	buf.WriteString(content)
	buf.WriteString(suffix)
	return buf.Bytes()
}

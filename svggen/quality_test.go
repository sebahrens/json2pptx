package svggen

import (
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// SVG Quality Assertion Helper
// =============================================================================
//
// AssertSVGQuality parses rendered SVG content and checks for structural
// quality problems that indicate visual defects. These assertions are
// independent of exact SVG output and focus on measurable quality outcomes.
//
// Quality checks:
//   - No font-size declarations below DefaultMinFontSize (9px)
//   - No NaN or Inf values in coordinate attributes
//   - No empty path data (d="")
//   - All <text> elements have non-zero font-size
//   - No viewBox overflow (elements outside declared bounds)
//   - Basic overlap detection for <rect> elements in the same parent group
// =============================================================================

// svgQualityIssue describes a single quality problem found in SVG content.
type svgQualityIssue struct {
	Category string // e.g., "font-size", "NaN", "overlap"
	Message  string
}

// AssertSVGQuality runs all quality checks on the given SVG content and
// reports failures through the testing.T. It does not fail on the first
// issue; it collects all issues and reports them together.
func AssertSVGQuality(t *testing.T, svgContent string) {
	t.Helper()

	issues := collectSVGQualityIssues(svgContent)
	for _, issue := range issues {
		t.Errorf("[SVG Quality: %s] %s", issue.Category, issue.Message)
	}
}

// collectSVGQualityIssues performs all quality checks and returns a list of issues.
func collectSVGQualityIssues(svgContent string) []svgQualityIssue {
	var issues []svgQualityIssue

	issues = append(issues, checkMinFontSize(svgContent)...)
	issues = append(issues, checkNaNInf(svgContent)...)
	issues = append(issues, checkEmptyPaths(svgContent)...)
	issues = append(issues, checkTextFontSize(svgContent)...)
	issues = append(issues, checkViewBoxOverflow(svgContent)...)
	issues = append(issues, checkRectOverlap(svgContent)...)

	return issues
}

// --- Individual quality checks ---

// checkMinFontSize finds font-size declarations below DefaultMinFontSize (9px).
// This catches illegibly small text that degrades visual quality.
// Note: the SVG uses CSS pixel units; 9px ≈ 6.7pt which is a safety net below
// the 9pt minimum enforced in DrawText (≈12px).
// It checks both explicit font-size properties and CSS font shorthand.
func checkMinFontSize(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	// Match font-size in style attributes: font-size:Npx or font-size:N
	// Also match font-size="N" XML attributes
	// Also match CSS font shorthand: font: [weight] Npx family
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`font-size\s*:\s*([\d.]+)\s*(?:px)?`),
		regexp.MustCompile(`font-size\s*=\s*"([\d.]+)(?:px)?"`),
		regexp.MustCompile(`\bfont\s*:\s*(?:[^;]*?\s)?([\d.]+)\s*px`),
	}

	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(svg, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			size, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				continue
			}
			// A font-size of 0 is sometimes used for hidden elements; skip those.
			// We care about sizes that are positive but below the minimum.
			if size > 0 && size < DefaultMinFontSize {
				issues = append(issues, svgQualityIssue{
					Category: "font-size",
					Message:  fmt.Sprintf("font-size %.1fpx is below minimum %.1fpx: %s", size, DefaultMinFontSize, match[0]),
				})
			}
		}
	}

	return issues
}

// checkNaNInf detects NaN or Inf in coordinate attributes, which indicate
// broken math in the rendering pipeline (division by zero, etc.).
func checkNaNInf(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	// Strip base64 data and text content to avoid false positives.
	cleaned := stripBase64FromSVG(svg)
	cleaned = stripTextContentFromSVG(cleaned)

	// Check for NaN in attribute values
	nanRe := regexp.MustCompile(`(?:x|y|cx|cy|r|width|height|x1|y1|x2|y2|rx|ry)\s*=\s*"[^"]*\bNaN\b[^"]*"`)
	nanMatches := nanRe.FindAllString(cleaned, -1)
	for _, m := range nanMatches {
		issues = append(issues, svgQualityIssue{
			Category: "NaN",
			Message:  fmt.Sprintf("NaN found in coordinate attribute: %s", m),
		})
	}

	// Also check for NaN in path data
	pathNanRe := regexp.MustCompile(`d\s*=\s*"[^"]*\bNaN\b[^"]*"`)
	for _, m := range pathNanRe.FindAllString(cleaned, -1) {
		issues = append(issues, svgQualityIssue{
			Category: "NaN",
			Message:  fmt.Sprintf("NaN found in path data: %.80s...", m),
		})
	}

	// Check for Inf
	infRe := regexp.MustCompile(`(?:x|y|cx|cy|r|width|height|x1|y1|x2|y2|rx|ry)\s*=\s*"[^"]*[+-]?Inf\b[^"]*"`)
	infMatches := infRe.FindAllString(cleaned, -1)
	for _, m := range infMatches {
		issues = append(issues, svgQualityIssue{
			Category: "Inf",
			Message:  fmt.Sprintf("Inf found in coordinate attribute: %s", m),
		})
	}

	// Broader NaN/Inf check in style attributes
	styleNanRe := regexp.MustCompile(`style\s*=\s*"[^"]*\bNaN\b[^"]*"`)
	for _, m := range styleNanRe.FindAllString(cleaned, -1) {
		issues = append(issues, svgQualityIssue{
			Category: "NaN",
			Message:  fmt.Sprintf("NaN found in style attribute: %.80s...", m),
		})
	}

	return issues
}

// checkEmptyPaths finds empty d="" path data which indicates degenerate rendering
// where a shape was supposed to be drawn but had no geometry.
func checkEmptyPaths(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	emptyPathRe := regexp.MustCompile(`<path[^>]*\bd\s*=\s*""[^>]*>`)
	matches := emptyPathRe.FindAllString(svg, -1)
	for _, m := range matches {
		issues = append(issues, svgQualityIssue{
			Category: "empty-path",
			Message:  fmt.Sprintf("empty path data (d=\"\"): %.80s", m),
		})
	}

	// Also check for paths with only whitespace in d
	wsPathRe := regexp.MustCompile(`<path[^>]*\bd\s*=\s*"\s+"[^>]*>`)
	for _, m := range wsPathRe.FindAllString(svg, -1) {
		issues = append(issues, svgQualityIssue{
			Category: "empty-path",
			Message:  fmt.Sprintf("path data contains only whitespace: %.80s", m),
		})
	}

	return issues
}

// checkTextFontSize verifies that all <text> elements have a resolvable,
// non-zero font-size. Text without a font-size declaration relies on
// browser defaults which can vary.
//
// This check recognizes font sizes declared via:
//   - font-size: Npx (CSS property)
//   - font-size="N" (XML attribute)
//   - font: Npx ... (CSS font shorthand, e.g., "font: 700 13.3333px Arial")
//   - font: Npx/N ... (CSS font shorthand with line-height)
func checkTextFontSize(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	// Find all <text elements and check for font-size
	textRe := regexp.MustCompile(`<text\b[^>]*>`)
	textMatches := textRe.FindAllString(svg, -1)

	// Match explicit font-size property
	fontSizeRe := regexp.MustCompile(`font-size\s*[:=]\s*"?([\d.]+)`)

	// Match CSS font shorthand: "font: [style] [variant] [weight] SIZE[/line-height] family"
	// The size is the first token that looks like a number followed by "px" or is just a number.
	// Examples: "font: 13.3333px Arial", "font: 700 16px Arial", "font: italic bold 12px/1.5 serif"
	fontShorthandRe := regexp.MustCompile(`\bfont\s*:\s*(?:[^;]*?\s)?([\d.]+)\s*px`)

	for _, textTag := range textMatches {
		// Try explicit font-size first
		fsMatch := fontSizeRe.FindStringSubmatch(textTag)
		if fsMatch != nil {
			size, err := strconv.ParseFloat(fsMatch[1], 64)
			if err == nil && size == 0 {
				issues = append(issues, svgQualityIssue{
					Category: "text-font-size",
					Message:  fmt.Sprintf("<text> has font-size=0: %.80s", textTag),
				})
			}
			continue
		}

		// Try CSS font shorthand
		shMatch := fontShorthandRe.FindStringSubmatch(textTag)
		if shMatch != nil {
			size, err := strconv.ParseFloat(shMatch[1], 64)
			if err == nil && size == 0 {
				issues = append(issues, svgQualityIssue{
					Category: "text-font-size",
					Message:  fmt.Sprintf("<text> has font-size=0 via font shorthand: %.80s", textTag),
				})
			}
			continue
		}

		// No font size found at all. Only flag if the element has a style
		// attribute (indicating it was styled but font size was omitted).
		if strings.Contains(textTag, "style=") {
			issues = append(issues, svgQualityIssue{
				Category: "text-font-size",
				Message:  fmt.Sprintf("<text> has style but no font-size: %.80s", textTag),
			})
		}
	}

	return issues
}

// checkViewBoxOverflow detects elements with coordinates outside the SVG viewBox.
// Elements that overflow the viewBox will be clipped or invisible.
func checkViewBoxOverflow(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	// Extract viewBox
	vbRe := regexp.MustCompile(`viewBox\s*=\s*"([\d.\-]+)\s+([\d.\-]+)\s+([\d.\-]+)\s+([\d.\-]+)"`)
	vbMatch := vbRe.FindStringSubmatch(svg)
	if vbMatch == nil {
		return issues // No viewBox to check against
	}

	vbMinX, _ := strconv.ParseFloat(vbMatch[1], 64)
	vbMinY, _ := strconv.ParseFloat(vbMatch[2], 64)
	vbWidth, _ := strconv.ParseFloat(vbMatch[3], 64)
	vbHeight, _ := strconv.ParseFloat(vbMatch[4], 64)

	if vbWidth <= 0 || vbHeight <= 0 {
		issues = append(issues, svgQualityIssue{
			Category: "viewBox",
			Message:  fmt.Sprintf("viewBox has non-positive dimensions: %s", vbMatch[0]),
		})
		return issues
	}

	vbMaxX := vbMinX + vbWidth
	vbMaxY := vbMinY + vbHeight

	// Generous margin for overflow detection (10% of viewBox dimensions).
	// Small overflows are normal for text elements that extend slightly
	// beyond their anchor point.
	marginX := vbWidth * 0.10
	marginY := vbHeight * 0.10

	// Check <rect> elements for overflow
	rectRe := regexp.MustCompile(`<rect\b[^>]*>`)
	for _, rect := range rectRe.FindAllString(svg, -1) {
		x := extractAttrFloat(rect, "x")
		y := extractAttrFloat(rect, "y")
		w := extractAttrFloat(rect, "width")
		h := extractAttrFloat(rect, "height")

		if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(w) || math.IsNaN(h) {
			continue // NaN is caught by checkNaNInf
		}

		if x+w > vbMaxX+marginX || y+h > vbMaxY+marginY {
			issues = append(issues, svgQualityIssue{
				Category: "viewBox-overflow",
				Message: fmt.Sprintf(
					"rect extends beyond viewBox: rect(x=%.1f,y=%.1f,w=%.1f,h=%.1f) vs viewBox(%.0fx%.0f)",
					x, y, w, h, vbWidth, vbHeight),
			})
		}
		if x < vbMinX-marginX || y < vbMinY-marginY {
			issues = append(issues, svgQualityIssue{
				Category: "viewBox-overflow",
				Message: fmt.Sprintf(
					"rect starts before viewBox: rect(x=%.1f,y=%.1f) vs viewBox min(%.0f,%.0f)",
					x, y, vbMinX, vbMinY),
			})
		}
	}

	// Check <circle> elements
	circleRe := regexp.MustCompile(`<circle\b[^>]*>`)
	for _, circle := range circleRe.FindAllString(svg, -1) {
		cx := extractAttrFloat(circle, "cx")
		cy := extractAttrFloat(circle, "cy")
		r := extractAttrFloat(circle, "r")

		if math.IsNaN(cx) || math.IsNaN(cy) || math.IsNaN(r) {
			continue
		}

		if cx+r > vbMaxX+marginX || cy+r > vbMaxY+marginY ||
			cx-r < vbMinX-marginX || cy-r < vbMinY-marginY {
			issues = append(issues, svgQualityIssue{
				Category: "viewBox-overflow",
				Message: fmt.Sprintf(
					"circle extends beyond viewBox: circle(cx=%.1f,cy=%.1f,r=%.1f) vs viewBox(%.0fx%.0f)",
					cx, cy, r, vbWidth, vbHeight),
			})
		}
	}

	return issues
}

// checkRectOverlap does basic overlap detection for <rect> elements within
// the same parent <g> group. Significant overlap between sibling rectangles
// indicates layout problems (e.g., bars rendering on top of each other).
func checkRectOverlap(svg string) []svgQualityIssue {
	var issues []svgQualityIssue

	// Parse groups and their child rects
	type rectBounds struct {
		x, y, w, h float64
		raw         string
	}

	// Simple regex-based group extraction. Find <g ...>...</g> blocks and
	// collect their direct <rect> children.
	groupRe := regexp.MustCompile(`<g\b[^>]*>([\s\S]*?)</g>`)
	rectRe := regexp.MustCompile(`<rect\b[^>]*>`)

	for _, gMatch := range groupRe.FindAllStringSubmatch(svg, -1) {
		groupContent := gMatch[1]
		rectMatches := rectRe.FindAllString(groupContent, -1)

		if len(rectMatches) < 2 {
			continue // Need at least 2 rects to check overlap
		}

		var rects []rectBounds
		for _, rect := range rectMatches {
			x := extractAttrFloat(rect, "x")
			y := extractAttrFloat(rect, "y")
			w := extractAttrFloat(rect, "width")
			h := extractAttrFloat(rect, "height")

			if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(w) || math.IsNaN(h) {
				continue
			}
			if w <= 0 || h <= 0 {
				continue // Skip zero-size rects
			}

			rects = append(rects, rectBounds{x: x, y: y, w: w, h: h, raw: rect})
		}

		// Check all pairs for significant overlap
		for i := 0; i < len(rects); i++ {
			for j := i + 1; j < len(rects); j++ {
				a, b := rects[i], rects[j]
				overlapArea := rectOverlapArea(a.x, a.y, a.w, a.h, b.x, b.y, b.w, b.h)
				minArea := math.Min(a.w*a.h, b.w*b.h)

				if minArea <= 0 {
					continue
				}

				// Flag if overlap exceeds 50% of the smaller rect's area.
				// This threshold avoids false positives from intentional layering
				// (e.g., background rects, borders) while catching layout bugs
				// where data bars render on top of each other.
				overlapRatio := overlapArea / minArea
				if overlapRatio > 0.50 {
					issues = append(issues, svgQualityIssue{
						Category: "rect-overlap",
						Message: fmt.Sprintf(
							"sibling rects overlap %.0f%% of smaller area: "+
								"rect1(x=%.1f,y=%.1f,w=%.1f,h=%.1f) rect2(x=%.1f,y=%.1f,w=%.1f,h=%.1f)",
							overlapRatio*100,
							a.x, a.y, a.w, a.h,
							b.x, b.y, b.w, b.h),
					})
				}
			}
		}
	}

	return issues
}

// --- Helper functions ---

// extractAttrFloat extracts a numeric attribute value from an SVG element string.
func extractAttrFloat(element, attr string) float64 {
	re := regexp.MustCompile(attr + `\s*=\s*"([^"]*)"`)
	match := re.FindStringSubmatch(element)
	if match == nil {
		return math.NaN()
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	if err != nil {
		return math.NaN()
	}
	return val
}

// rectOverlapArea calculates the overlapping area between two rectangles.
func rectOverlapArea(x1, y1, w1, h1, x2, y2, w2, h2 float64) float64 {
	overlapX := math.Max(0, math.Min(x1+w1, x2+w2)-math.Max(x1, x2))
	overlapY := math.Max(0, math.Min(y1+h1, y2+h2)-math.Max(y1, y2))
	return overlapX * overlapY
}

// stripBase64FromSVG removes base64-encoded data to prevent false positives.
func stripBase64FromSVG(svg string) string {
	re := regexp.MustCompile(`base64,[A-Za-z0-9+/=]+`)
	return re.ReplaceAllString(svg, "base64,STRIPPED")
}

// stripTextContentFromSVG removes text content between tags to prevent false positives.
func stripTextContentFromSVG(svg string) string {
	re := regexp.MustCompile(`<tspan[^>]*>[^<]*</tspan>`)
	svg = re.ReplaceAllString(svg, "<tspan/>")
	re2 := regexp.MustCompile(`<text[^>]*>[^<]*</text>`)
	return re2.ReplaceAllString(svg, "<text/>")
}

// =============================================================================
// Tests for the quality helper itself
// =============================================================================

func TestAssertSVGQuality_CleanSVG(t *testing.T) {
	// A minimal valid SVG should produce no quality issues.
	cleanSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600">
		<rect x="10" y="10" width="100" height="50" fill="#336699"/>
		<text x="50" y="30" style="font-size:12px">Hello</text>
		<path d="M 10 10 L 100 100"/>
	</svg>`

	issues := collectSVGQualityIssues(cleanSVG)
	if len(issues) > 0 {
		for _, issue := range issues {
			t.Errorf("unexpected issue on clean SVG: [%s] %s", issue.Category, issue.Message)
		}
	}
}

func TestAssertSVGQuality_DetectsSmallFontSize(t *testing.T) {
	svg := `<svg viewBox="0 0 800 600">
		<text style="font-size:5px">Too small</text>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "font-size" {
			found = true
		}
	}
	if !found {
		t.Error("expected font-size issue for 5px text, got none")
	}
}

func TestAssertSVGQuality_DetectsNaN(t *testing.T) {
	svg := `<svg viewBox="0 0 800 600">
		<rect x="NaN" y="10" width="100" height="50"/>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "NaN" {
			found = true
		}
	}
	if !found {
		t.Error("expected NaN issue, got none")
	}
}

func TestAssertSVGQuality_DetectsEmptyPath(t *testing.T) {
	svg := `<svg viewBox="0 0 800 600">
		<path d=""/>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "empty-path" {
			found = true
		}
	}
	if !found {
		t.Error("expected empty-path issue, got none")
	}
}

func TestAssertSVGQuality_DetectsViewBoxOverflow(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<rect x="0" y="0" width="200" height="200"/>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "viewBox-overflow" {
			found = true
		}
	}
	if !found {
		t.Error("expected viewBox-overflow issue for rect extending to 200x200 in 100x100 viewBox, got none")
	}
}

func TestAssertSVGQuality_DetectsRectOverlap(t *testing.T) {
	// Two rects at the same position inside a group = 100% overlap
	svg := `<svg viewBox="0 0 800 600">
		<g>
			<rect x="10" y="10" width="100" height="50"/>
			<rect x="10" y="10" width="100" height="50"/>
		</g>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "rect-overlap" {
			found = true
		}
	}
	if !found {
		t.Error("expected rect-overlap issue for identical overlapping rects, got none")
	}
}

func TestAssertSVGQuality_AllowsSmallOverlap(t *testing.T) {
	// Two rects with only 10% overlap should not trigger
	svg := `<svg viewBox="0 0 800 600">
		<g>
			<rect x="0" y="0" width="100" height="50"/>
			<rect x="90" y="0" width="100" height="50"/>
		</g>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	for _, issue := range issues {
		if issue.Category == "rect-overlap" {
			t.Errorf("unexpected rect-overlap issue for rects with small overlap: %s", issue.Message)
		}
	}
}

func TestAssertSVGQuality_FontSizeAttr(t *testing.T) {
	// Test font-size as an XML attribute (not CSS style)
	svg := `<svg viewBox="0 0 800 600">
		<text font-size="4px" x="10" y="10">Tiny</text>
	</svg>`

	issues := collectSVGQualityIssues(svg)
	found := false
	for _, issue := range issues {
		if issue.Category == "font-size" {
			found = true
		}
	}
	if !found {
		t.Error("expected font-size issue for font-size=\"4px\" attribute, got none")
	}
}

func TestExtractAttrFloat(t *testing.T) {
	tests := []struct {
		element string
		attr    string
		want    float64
		wantNaN bool
	}{
		{`<rect x="10.5" y="20"/>`, "x", 10.5, false},
		{`<rect x="10.5" y="20"/>`, "y", 20, false},
		{`<rect x="10.5" y="20"/>`, "width", math.NaN(), true},
		{`<circle cx="100" cy="200" r="50"/>`, "r", 50, false},
	}

	for _, tt := range tests {
		got := extractAttrFloat(tt.element, tt.attr)
		if tt.wantNaN {
			if !math.IsNaN(got) {
				t.Errorf("extractAttrFloat(%q, %q) = %v, want NaN", tt.element, tt.attr, got)
			}
		} else {
			if got != tt.want {
				t.Errorf("extractAttrFloat(%q, %q) = %v, want %v", tt.element, tt.attr, got, tt.want)
			}
		}
	}
}

func TestRectOverlapArea(t *testing.T) {
	tests := []struct {
		name string
		x1, y1, w1, h1 float64
		x2, y2, w2, h2 float64
		want            float64
	}{
		{"no overlap", 0, 0, 10, 10, 20, 20, 10, 10, 0},
		{"full overlap", 0, 0, 10, 10, 0, 0, 10, 10, 100},
		{"partial overlap", 0, 0, 10, 10, 5, 5, 10, 10, 25},
		{"adjacent", 0, 0, 10, 10, 10, 0, 10, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rectOverlapArea(tt.x1, tt.y1, tt.w1, tt.h1, tt.x2, tt.y2, tt.w2, tt.h2)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("rectOverlapArea() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Quality gate: run AssertSVGQuality on every golden test render
// =============================================================================

// TestQualityGate_GoldenTests renders every golden test case and runs
// quality assertions on the output. This ensures that golden tests enforce
// minimum quality standards in addition to regression detection.
func TestQualityGate_GoldenTests(t *testing.T) {
	// Re-use the same test cases from TestGolden_AllDiagramTypes.
	// We render each one and check quality, but do NOT compare to golden files.
	testCases := goldenTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Render(tc.req)
			if err != nil {
				t.Fatalf("failed to render %s: %v", tc.name, err)
			}
			AssertSVGQuality(t, string(doc.Content))
		})
	}
}

// TestQualityGate_MultiColumnLayouts renders every multi-column layout golden
// test case and runs quality assertions.
func TestQualityGate_MultiColumnLayouts(t *testing.T) {
	testCases := multiColumnGoldenTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Render(tc.req)
			if err != nil {
				t.Fatalf("failed to render %s: %v", tc.name, err)
			}
			AssertSVGQuality(t, string(doc.Content))
		})
	}
}

// =============================================================================
// Test case extraction helpers (to share golden test data without duplication)
// =============================================================================

// goldenTestCase holds a named test case for reuse across golden and quality tests.
type goldenTestCase struct {
	name string
	req  *RequestEnvelope
}

// goldenTestCases returns the same test cases used in TestGolden_AllDiagramTypes.
func goldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "waterfall_basic",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Revenue Bridge",
					"points": []any{
						map[string]any{"label": "Start", "value": 100, "type": "increase"},
						map[string]any{"label": "Growth", "value": 30, "type": "increase"},
						map[string]any{"label": "Churn", "value": -15, "type": "decrease"},
						map[string]any{"label": "End", "value": 115, "type": "total"},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowValues: true, ShowGrid: true},
			},
		},
		{
			name: "pie_chart_basic",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{35.0, 25.0, 20.0, 15.0, 5.0},
					"categories": []any{"Product A", "Product B", "Product C", "Product D", "Other"},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "bar_chart_basic",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Sales by Quarter",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{100.0, 150.0, 120.0, 180.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "line_chart_basic",
			req: &RequestEnvelope{
				Type:  "line_chart",
				Title: "Monthly Trend",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May"},
					"series": []any{
						map[string]any{"name": "Sales", "values": []any{100.0, 120.0, 115.0, 135.0, 150.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowGrid: true},
			},
		},
		{
			name: "scatter_chart_basic",
			req: &RequestEnvelope{
				Type:  "scatter_chart",
				Title: "Correlation",
				Data: map[string]any{
					"series": []any{
						map[string]any{
							"name":     "Data Points",
							"values":   []any{20.0, 35.0, 45.0, 60.0, 75.0},
							"x_values": []any{10.0, 25.0, 30.0, 50.0, 65.0},
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowGrid: true},
			},
		},
		{
			name: "org_chart_basic",
			req: &RequestEnvelope{
				Type:  "org_chart",
				Title: "Company Structure",
				Data: map[string]any{
					"root": map[string]any{
						"name": "CEO", "title": "Chief Executive",
						"children": []any{
							map[string]any{"name": "CTO", "title": "Technology"},
							map[string]any{"name": "CFO", "title": "Finance"},
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "fishbone_basic",
			req: &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Low Productivity",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Skill gaps", "Low morale"}},
						map[string]any{"name": "Process", "causes": []any{"Bottlenecks", "Rework"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated tools"}},
					},
				},
				Output: OutputSpec{Width: 900, Height: 500},
			},
		},
	}
}

// multiColumnGoldenTestCases returns a representative subset of multi-column
// layout golden tests for quality checking.
func multiColumnGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "waterfall_halfwidth",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Q4 Budget",
					"points": []any{
						map[string]any{"label": "Start", "value": 500, "type": "increase"},
						map[string]any{"label": "Sales", "value": 150, "type": "increase"},
						map[string]any{"label": "Costs", "value": -80, "type": "decrease"},
						map[string]any{"label": "End", "value": 570, "type": "total"},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowValues: true},
			},
		},
		{
			name: "pie_chart_thirdwidth",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{60.0, 25.0, 15.0},
					"categories": []any{"Primary", "Secondary", "Other"},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "fishbone_halfwidth",
			req: &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Low Productivity",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Skill gaps", "Low morale"}},
						map[string]any{"name": "Process", "causes": []any{"Bottlenecks", "Rework"}},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
			},
		},
	}
}

// Ensure xml import is used (prevents "imported and not used" error).
var _ = xml.Header

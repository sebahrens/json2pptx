package svggen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// textElemX returns the x coordinate and the full element string of the first
// <text> element whose content contains the given substring. The bool reports
// whether such an element was found.
func textElemX(svg, substr string) (x float64, elem string, found bool) {
	re := regexp.MustCompile(`<text[^>]*>.*?</text>`)
	xRe := regexp.MustCompile(`<text x="([\d.]+)"`)
	for _, m := range re.FindAllString(svg, -1) {
		if !strings.Contains(m, substr) {
			continue
		}
		xm := xRe.FindStringSubmatch(m)
		if xm == nil {
			continue
		}
		v, err := strconv.ParseFloat(xm[1], 64)
		if err != nil {
			continue
		}
		return v, m, true
	}
	return 0, "", false
}

// TestGanttChart_RowLabelGap6pt verifies that left-column row labels are flush
// right and sit 6pt (Spacing.SM) to the left of the y-axis (the right edge of
// the label column), not the previous 8pt (Spacing.MD).
func TestGanttChart_RowLabelGap6pt(t *testing.T) {
	b := NewSVGBuilder(900, 500)
	gc := NewGanttChart(b, DefaultGanttConfig(900, 500))
	style := b.StyleGuide()

	const labelX, labelWidth = 0.0, 200.0
	gc.drawRowLabel("ROWLBL", 100, labelX, labelWidth)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	scale := float64(doc.Width) / 900.0

	x, elem, found := textElemX(svg, "ROWLBL")
	if !found {
		t.Fatal("row label not found in SVG")
	}
	// Flush right.
	if !regexp.MustCompile(`text-anchor="end"`).MatchString(elem) {
		t.Errorf("row label must be right-aligned (text-anchor=\"end\"); got %s", elem)
	}
	// 6pt gap: right edge = labelX + labelWidth - SM.
	wantPt := labelX + labelWidth - style.Spacing.SM
	wantPx := wantPt * scale
	if math.Abs(x-wantPx) > 0.5 {
		t.Errorf("row label right edge x = %.3f px, want %.3f px (labelWidth-%.0fpt gap)", x, wantPx, style.Spacing.SM)
	}
}

// TestGanttChart_BarLabelInsideLeftAligned6pt verifies that a label that fits
// inside its bar is left-aligned 6pt (Spacing.SM) from the bar's left edge,
// rather than centered.
func TestGanttChart_BarLabelInsideLeftAligned6pt(t *testing.T) {
	b := NewSVGBuilder(900, 500)
	gc := NewGanttChart(b, DefaultGanttConfig(900, 500))
	style := b.StyleGuide()

	const barX, barW, barH = 300.0, 240.0, 24.0
	gc.drawBarLabel("INBAR", barX, 100, barW, barH, MustParseColor("#4E79A7"), 800)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	scale := float64(doc.Width) / 900.0

	x, elem, found := textElemX(svg, "INBAR")
	if !found {
		t.Fatal("in-bar label not found in SVG")
	}
	// Left-aligned: must NOT be centered.
	if regexp.MustCompile(`text-anchor="middle"`).MatchString(elem) {
		t.Errorf("in-bar label must be left-aligned, not centered; got %s", elem)
	}
	// 6pt from the bar's left edge.
	wantPx := (barX + style.Spacing.SM) * scale
	if math.Abs(x-wantPx) > 0.5 {
		t.Errorf("in-bar label x = %.3f px, want %.3f px (barX + %.0fpt)", x, wantPx, style.Spacing.SM)
	}
}

// TestGanttChart_BarLabelExternalLeftAligned6pt verifies that when a label does
// not fit inside a narrow bar it is drawn to the right of the bar, left-aligned,
// 6pt (Spacing.SM) from the bar's right edge.
func TestGanttChart_BarLabelExternalLeftAligned6pt(t *testing.T) {
	b := NewSVGBuilder(900, 500)
	gc := NewGanttChart(b, DefaultGanttConfig(900, 500))
	style := b.StyleGuide()

	const barX, barW, barH = 300.0, 20.0, 24.0
	gc.drawBarLabel("ExternalLabelTextThatIsWide", barX, 100, barW, barH, MustParseColor("#4E79A7"), 800)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)
	scale := float64(doc.Width) / 900.0

	x, elem, found := textElemX(svg, "ExternalLabelTextThatIsWide")
	if !found {
		t.Fatal("external bar label not found in SVG")
	}
	if regexp.MustCompile(`text-anchor="(middle|end)"`).MatchString(elem) {
		t.Errorf("external bar label must be left-aligned; got %s", elem)
	}
	wantPx := (barX + barW + style.Spacing.SM) * scale
	if math.Abs(x-wantPx) > 0.5 {
		t.Errorf("external bar label x = %.3f px, want %.3f px (barX + barW + %.0fpt)", x, wantPx, style.Spacing.SM)
	}
}

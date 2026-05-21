package examine

import (
	"fmt"
	"strconv"
	"strings"
)

// svgCanvasWidthPx is the rendered width of the annotated layout SVG. The
// height is derived from the slide aspect ratio; a ribbon strip is appended
// below for the z-order legend.
const svgCanvasWidthPx = 1000.0

// RenderLayoutSVG produces a browser-openable annotated overlay for a single
// layout. It draws the slide frame, the derived content zone as a dashed inset,
// every placeholder rectangle (in z-order) labelled with its id, role,
// font-aware budget, exact bounds in inches, and z-index, a section-number
// badge on decorative number frames, and a z-order ribbon legend.
//
// Every placeholder group carries the same numeric facts as report.json on
// data-* attributes (data-font-pt, data-max-chars, data-z, data-x-in, …) so a
// machine can cross-check the overlay against the JSON, and the same numbers
// are drawn as visible text for a human reading the SVG in a browser.
func RenderLayoutSVG(l LayoutReport, slide SlideDimensions, theme ThemeReport) string {
	slideW := slide.WidthEMU
	slideH := slide.HeightEMU
	if slideW <= 0 || slideH <= 0 {
		slideW, slideH = defaultSlideWidthEMU, defaultSlideHeightEMU
	}
	scale := svgCanvasWidthPx / float64(slideW)
	canvasH := float64(slideH) * scale
	ribbonH := 24.0 + float64(len(l.Placeholders))*16.0
	totalH := canvasH + ribbonH

	px := func(emu int64) float64 { return float64(emu) * scale }

	bg := theme.Colors["lt1"]
	if bg == "" {
		bg = "#FFFFFF"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="Helvetica,Arial,sans-serif">`,
		svgCanvasWidthPx, totalH, svgCanvasWidthPx, totalH)
	b.WriteString("\n")

	// Slide frame.
	fmt.Fprintf(&b, `  <rect x="0" y="0" width="%.0f" height="%.1f" fill="%s" stroke="#333333" stroke-width="1.5"/>`+"\n",
		svgCanvasWidthPx, canvasH, svgEsc(bg))

	// Header: layout name + canonical type.
	fmt.Fprintf(&b, `  <text x="8" y="16" font-size="13" font-weight="bold" fill="#111111">%s</text>`+"\n",
		svgEsc(l.Name))
	canonical := l.CanonicalType
	if canonical == "" {
		canonical = "unknown"
	}
	fmt.Fprintf(&b, `  <text x="8" y="32" font-size="11" fill="#555555">%s · %s · conf %.2f</text>`+"\n",
		svgEsc(canonical), svgEsc(l.CanonicalFamily), l.CanonicalConfidence)

	// Content zone as a dashed inset.
	z := l.ContentZone
	fmt.Fprintf(&b, `  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="#1f77b4" stroke-width="1" stroke-dasharray="6 4"/>`+"\n",
		px(z.LeftEMU), px(z.TopEMU), px(z.RightEMU-z.LeftEMU), px(z.BottomEMU-z.TopEMU))
	fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" font-size="9" fill="#1f77b4">content zone</text>`+"\n",
		px(z.LeftEMU)+3, px(z.TopEMU)+11)

	// Placeholders in z-order (document order; later = on top).
	for i := range l.Placeholders {
		writePlaceholder(&b, &l.Placeholders[i], px)
	}

	// Z-order ribbon legend below the slide frame.
	writeRibbon(&b, l.Placeholders, canvasH)

	b.WriteString("</svg>\n")
	return b.String()
}

// writePlaceholder draws one annotated placeholder group.
func writePlaceholder(b *strings.Builder, ph *PlaceholderReport, px func(int64) float64) {
	x := px(ph.Bounds.XEMU)
	y := px(ph.Bounds.YEMU)
	w := px(ph.Bounds.WEMU)
	h := px(ph.Bounds.HEMU)
	color := roleColor(ph.Role)

	fmt.Fprintf(b, `  <g class="placeholder" data-ph-id="%s" data-role="%s" data-type="%s" data-font-pt="%s" data-max-chars="%d" data-z="%d" data-x-in="%s" data-y-in="%s" data-w-in="%s" data-h-in="%s">`+"\n",
		svgEsc(ph.ID), svgEsc(ph.Role), svgEsc(ph.Type),
		fnum(ph.FontPt), ph.MaxChars, ph.ZIndex,
		fnum(ph.Bounds.XIn), fnum(ph.Bounds.YIn), fnum(ph.Bounds.WIn), fnum(ph.Bounds.HIn))

	fmt.Fprintf(b, `    <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" fill-opacity="0.12" stroke="%s" stroke-width="2"/>`+"\n",
		x, y, w, h, color, color)

	// Labels, clamped to start inside the slide frame.
	tx := x + 4
	ty := y + 14
	fmt.Fprintf(b, `    <text x="%.1f" y="%.1f" font-size="11" font-weight="bold" fill="%s">%s</text>`+"\n",
		tx, ty, color, svgEsc(ph.ID))
	fmt.Fprintf(b, `    <text x="%.1f" y="%.1f" font-size="9.5" fill="#222222">%s · %spt · ≤%d ch</text>`+"\n",
		tx, ty+13, svgEsc(ph.Role), fnum(ph.FontPt), ph.MaxChars)
	fmt.Fprintf(b, `    <text x="%.1f" y="%.1f" font-size="9.5" fill="#222222">%s×%s in @ (%s, %s)</text>`+"\n",
		tx, ty+26, fnum(ph.Bounds.WIn), fnum(ph.Bounds.HIn), fnum(ph.Bounds.XIn), fnum(ph.Bounds.YIn))
	fmt.Fprintf(b, `    <text x="%.1f" y="%.1f" font-size="9.5" fill="#888888">z=%d</text>`+"\n",
		tx, ty+39, ph.ZIndex)

	if ph.Role == "section_number" {
		bx := x + w - 16
		by := y + 4
		fmt.Fprintf(b, `    <rect x="%.1f" y="%.1f" width="13" height="13" rx="2" fill="#d62728"/>`+"\n", bx, by)
		fmt.Fprintf(b, `    <text x="%.1f" y="%.1f" font-size="10" font-weight="bold" fill="#ffffff" text-anchor="middle">#</text>`+"\n",
			bx+6.5, by+10)
	}

	b.WriteString("  </g>\n")
}

// writeRibbon draws a compact z-order legend below the slide frame.
func writeRibbon(b *strings.Builder, phs []PlaceholderReport, top float64) {
	fmt.Fprintf(b, `  <text x="8" y="%.1f" font-size="11" font-weight="bold" fill="#111111">z-order (bottom → top)</text>`+"\n", top+16)
	y := top + 30
	for i := range phs {
		ph := &phs[i]
		color := roleColor(ph.Role)
		fmt.Fprintf(b, `  <rect x="8" y="%.1f" width="11" height="11" fill="%s"/>`+"\n", y-9, color)
		fmt.Fprintf(b, `  <text x="24" y="%.1f" font-size="10" fill="#333333">z=%d  %s (%s)</text>`+"\n",
			y, ph.ZIndex, svgEsc(ph.ID), svgEsc(ph.Role))
		y += 16
	}
}

// roleColor maps a placeholder role to a stable stroke color.
func roleColor(role string) string {
	switch role {
	case "title":
		return "#1f77b4"
	case "subtitle":
		return "#17becf"
	case "eyebrow":
		return "#9467bd"
	case "section_number":
		return "#d62728"
	case "body":
		return "#2ca02c"
	case "image":
		return "#ff7f0e"
	case "chart":
		return "#bcbd22"
	case "footer", "page_number", "date":
		return "#7f7f7f"
	default:
		return "#555555"
	}
}

// fnum formats a float with the same minimal representation Go's json encoder
// uses, so the SVG data-* attributes are byte-comparable with report.json
// numbers after parsing.
func fnum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// svgEsc escapes a string for inclusion in SVG text/attributes.
func svgEsc(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

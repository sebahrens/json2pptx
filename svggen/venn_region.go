package svggen

import (
	"fmt"
	"math"
	"strings"
)

// Region-aware Venn captions (go-slide-creator-b7qqg.27).
//
// Intersection captions used to be wrapped against a rectangular slot
// (a fixed fraction of the radius) that was wider than the lens it sat in, and
// the fitter floor could sit below the placement floor that DrawText enforces,
// so a caption was wrapped for one size and drawn at a larger one. A short
// caption such as "Focused value proposition" then crossed both circle
// outlines. The fitter below measures the region itself: every line of a
// caption must lie inside all the circles of its region and outside the
// others, over the full height of the line, at a font no smaller than the
// builder's floor. When no size and line break fits, the two-circle diagram
// widens its overlap (unless the author fixed overlap_ratio), and a caption
// that still cannot fit is drawn best-effort and reported as
// diagram.region_overflow with a concrete alternative.

// vennRegionShape is a Venn region: the points inside every circle of in and
// outside every circle of out, shrunk by pad so text clears the outlines.
type vennRegionShape struct {
	in, out []circleLayout
	pad     float64
}

func (s vennRegionShape) contains(x, y float64) bool {
	for _, c := range s.in {
		if math.Hypot(x-c.cx, y-c.cy) > c.radius-s.pad {
			return false
		}
	}
	for _, c := range s.out {
		if math.Hypot(x-c.cx, y-c.cy) < c.radius+s.pad {
			return false
		}
	}
	return true
}

// bandFits reports whether the horizontal band [ax-h, ax+h] × [y0, y1] lies in
// the region, sampled along its edges and centre line.
func (s vennRegionShape) bandFits(ax, y0, y1, h float64) bool {
	const steps = 4
	for _, y := range [3]float64{y0, (y0 + y1) / 2, y1} {
		for k := -steps; k <= steps; k++ {
			if !s.contains(ax+h*float64(k)/steps, y) {
				return false
			}
		}
	}
	return true
}

// halfWidth returns the largest half-width of a band centred on ax that stays
// inside the region between y0 and y1 (0 when the centre itself is outside).
func (s vennRegionShape) halfWidth(ax, y0, y1 float64) float64 {
	if !s.bandFits(ax, y0, y1, 0) {
		return 0
	}
	hi := 0.0
	for _, c := range s.in {
		hi = math.Max(hi, 2*c.radius)
	}
	if hi == 0 {
		return 0
	}
	lo := 0.0
	for i := 0; i < 24; i++ {
		mid := (lo + hi) / 2
		if s.bandFits(ax, y0, y1, mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// segmentFits reports whether the band [xl, xr] × [y0, y1] lies in the region.
func (s vennRegionShape) segmentFits(xl, xr, y0, y1 float64) bool {
	const steps = 8
	for _, y := range [3]float64{y0, (y0 + y1) / 2, y1} {
		for k := 0; k <= steps; k++ {
			if !s.contains(xl+(xr-xl)*float64(k)/steps, y) {
				return false
			}
		}
	}
	return true
}

// chord returns the horizontal extent [xl, xr] of the region between y0 and
// y1 that contains ax. The region need not be symmetric about ax: a pairwise
// region of a three-circle diagram lies on a diagonal axis, so its widest
// horizontal chord is off-centre. ok is false when ax itself is outside.
func (s vennRegionShape) chord(ax, y0, y1 float64) (xl, xr float64, ok bool) {
	if !s.segmentFits(ax, ax, y0, y1) {
		return ax, ax, false
	}
	span := 0.0
	for _, c := range s.in {
		span = math.Max(span, 2*c.radius)
	}
	search := func(fits func(h float64) bool) float64 {
		lo, hi := 0.0, span
		for i := 0; i < 24; i++ {
			mid := (lo + hi) / 2
			if fits(mid) {
				lo = mid
			} else {
				hi = mid
			}
		}
		return lo
	}
	left := search(func(h float64) bool { return s.segmentFits(ax-h, ax, y0, y1) })
	right := search(func(h float64) bool { return s.segmentFits(ax-left, ax+h, y0, y1) })
	return ax - left, ax + right, true
}

// vennCaptionFit is a caption laid out for a region.
type vennCaptionFit struct {
	fontSize   float64
	lineHeight float64
	lines      []string
	// top is the y of the first line's band; the block spans
	// top .. top+len(lines)*lineHeight.
	top float64
	// xs are the line centres: each line is centred in its own chord of the
	// region. Nil means every line is centred on the anchor.
	xs []float64
	// ok is true when every line lies inside the region.
	ok bool
}

func (f vennCaptionFit) height() float64 { return float64(len(f.lines)) * f.lineHeight }

// vennCaptionMaxLines caps how many lines a caption may be broken into.
const vennCaptionMaxLines = 4

// vennCaptionStep is the font-size decrement of the region fitter.
const vennCaptionStep = 0.5

// fitCaptionInRegion finds the largest font size in [minSize, preferred] and
// the fewest lines at which label, centred on (ax, ay), lies inside region.
// weight is the font weight the caption is drawn with. When nothing fits, the
// result is the minSize layout with ok=false.
func (vc *VennChart) fitCaptionInRegion(label string, region vennRegionShape, ax, ay, preferred, minSize float64, weight int) vennCaptionFit {
	words := strings.Fields(label)
	if len(words) == 0 {
		return vennCaptionFit{ok: true}
	}
	if minSize > preferred {
		preferred = minSize
	}
	b := vc.builder
	lineFactor := 1.2
	if st := b.StyleGuide(); st != nil && st.Typography != nil && st.Typography.LineHeight > 0 {
		lineFactor = st.Typography.LineHeight
	}
	b.Push()
	defer b.Pop()
	b.SetFontWeight(weight)

	for fs := preferred; fs >= minSize-1e-9; fs -= vennCaptionStep {
		b.SetFontSize(fs)
		lineH := fs * lineFactor
		for n := 1; n <= vennCaptionMaxLines && n <= len(words); n++ {
			if lines, xs, top, ok := vc.breakIntoRegion(words, n, region, ax, ay, lineH); ok {
				return vennCaptionFit{fontSize: fs, lineHeight: lineH, lines: lines, xs: xs, top: top, ok: true}
			}
		}
	}

	// Nothing fits: lay out at the floor, balancing lines against the widest
	// band so the overflow is as small as it can be.
	b.SetFontSize(minSize)
	lineH := minSize * lineFactor
	n := len(words)
	if n > vennCaptionMaxLines {
		n = vennCaptionMaxLines
	}
	lines := greedyLines(words, n, func(s string) float64 { w, _ := b.MeasureText(s); return w })
	return vennCaptionFit{
		fontSize:   minSize,
		lineHeight: lineH,
		lines:      lines,
		top:        ay - float64(len(lines))*lineH/2,
		ok:         false,
	}
}

// breakIntoRegion greedily fills n line bands, stacked and centred
// vertically on ay, with words: each line no wider than its band's chord
// through the region, and centred in that chord.
func (vc *VennChart) breakIntoRegion(words []string, n int, region vennRegionShape, ax, ay, lineH float64) ([]string, []float64, float64, bool) {
	b := vc.builder
	top := ay - float64(n)*lineH/2
	lines := make([]string, 0, n)
	xs := make([]float64, 0, n)
	wi := 0
	for li := 0; li < n && wi < len(words); li++ {
		y0 := top + float64(li)*lineH
		xl, xr, inside := region.chord(ax, y0, y0+lineH)
		if !inside {
			return nil, nil, 0, false
		}
		line := ""
		for wi < len(words) {
			candidate := words[wi]
			if line != "" {
				candidate = line + " " + words[wi]
			}
			if w, _ := b.MeasureText(candidate); w > xr-xl {
				break
			}
			line = candidate
			wi++
		}
		if line == "" {
			return nil, nil, 0, false // a single word is wider than its band
		}
		lines = append(lines, line)
		xs = append(xs, (xl+xr)/2)
	}
	if wi < len(words) {
		return nil, nil, 0, false
	}
	if m := len(lines); m < n {
		// Fewer lines than planned: re-centre the shorter block when it still
		// fits, else keep the planned (checked) bands.
		mTop := ay - float64(m)*lineH/2
		mxs := make([]float64, 0, m)
		for li, l := range lines {
			y0 := mTop + float64(li)*lineH
			xl, xr, inside := region.chord(ax, y0, y0+lineH)
			if w, _ := b.MeasureText(l); !inside || w > xr-xl {
				return lines, xs, top, true
			}
			mxs = append(mxs, (xl+xr)/2)
		}
		return lines, mxs, mTop, true
	}
	return lines, xs, top, true
}

// greedyLines splits words into at most n lines of roughly equal width.
func greedyLines(words []string, n int, measure func(string) float64) []string {
	if n <= 1 || len(words) <= 1 {
		return []string{strings.Join(words, " ")}
	}
	target := measure(strings.Join(words, " ")) / float64(n)
	var lines []string
	line := ""
	for i, w := range words {
		candidate := w
		if line != "" {
			candidate = line + " " + w
		}
		remainingWords := len(words) - i
		remainingLines := n - len(lines)
		if line != "" && measure(candidate) > target && remainingLines > 1 && remainingWords >= 1 {
			lines = append(lines, line)
			line = w
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// drawCaption draws a fitted caption: each line at its chord centre, or
// centred on ax when the fit carries no per-line centres.
func (vc *VennChart) drawCaption(fit vennCaptionFit, ax float64, weight int, color Color) {
	if len(fit.lines) == 0 {
		return
	}
	b := vc.builder
	b.Push()
	b.SetFontSize(fit.fontSize)
	b.SetFontWeight(weight)
	b.SetTextColor(color)
	for i, line := range fit.lines {
		cy := fit.top + (float64(i)+0.5)*fit.lineHeight
		x := ax
		if i < len(fit.xs) {
			x = fit.xs[i]
		}
		b.DrawText(line, x, cy, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()
}

// vennCaptionFloor is the smallest size a region caption may use: the
// renderer's legibility floor for the canvas, never below the builder's floor
// (which DrawText enforces, so fitting below it would wrap for one size and
// draw at another).
func (vc *VennChart) vennCaptionFloor(r float64) float64 {
	return math.Max(math.Max(6, math.Min(11, r*0.15)), vc.builder.MinFontSize())
}

// vennCaptionPad is the clearance kept between caption text and an outline.
func (vc *VennChart) vennCaptionPad(fontSize float64) float64 {
	return vc.config.StrokeWidth/2 + fontSize*0.15
}

// vennRegionNames maps a region key to a readable description.
func vennRegionNames(key string, data VennData) string {
	var names []string
	for _, ch := range key {
		i := int(ch - 'a')
		if i >= 0 && i < len(data.Circles) && data.Circles[i].Label != "" {
			names = append(names, fmt.Sprintf("%q", data.Circles[i].Label))
		}
	}
	return strings.Join(names, " and ")
}

// reportRegionOverflow emits diagram.region_overflow for a caption that could
// not be fitted inside its region at a readable size.
func (vc *VennChart) reportRegionOverflow(key, label string, fit vennCaptionFit, data VennData, suggestedOverlap float64) {
	params := map[string]any{
		"diagram_type": "venn",
		"region":       key,
		"label":        label,
		"font_size":    math.Round(fit.fontSize*10) / 10,
		"alternatives": []string{"shorten the caption", "move the shared items to a callout beside the diagram", "matrix_2x2", "comparison-2col"},
	}
	remedy := "shorten it, move the shared detail to a callout beside the diagram, or use a matrix_2x2 / comparison-2col"
	if suggestedOverlap > 0 {
		params["overlap_ratio"] = suggestedOverlap
		remedy = fmt.Sprintf("shorten it, set data.overlap_ratio to %.2f to widen the overlap, or move the shared detail to a callout beside the diagram (matrix_2x2 / comparison-2col are alternatives)", suggestedOverlap)
	}
	where := vennRegionNames(key, data)
	if where == "" {
		where = "its circles"
	}
	vc.builder.AddFinding(Finding{
		Field: "intersections." + key,
		Code:  FindingDiagramRegionOverflow,
		Message: fmt.Sprintf(
			"venn: intersection caption %q (%s) does not fit inside the overlap of %s at a readable size and crosses the circle outlines — %s",
			label, key, where, remedy),
		Severity: "warning",
		Fix: &FixSuggestion{
			Kind:   FixKindShortenLabels,
			Params: params,
		},
	})
}

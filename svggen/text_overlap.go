package svggen

import (
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
	"github.com/tdewolff/canvas"
)

type textRotation struct {
	angle, cx, cy float64
	skip          bool
}

type drawnTextBox struct {
	text string
	rect Rect
}

// recordDrawnText uses the same face and baseline as DrawText. Diagonal labels
// need oriented-box intersection rather than an axis-aligned approximation, so
// they are omitted until that geometry is available. Quarter-turn axis titles
// can be measured exactly by rotating their four corners.
func (b *SVGBuilder) recordDrawnText(text string, x, y float64, align TextAlign, baseline TextBaseline, face *canvas.FontFace) {
	if strings.TrimSpace(text) == "" || b.textRotation.skip {
		return
	}
	angle := math.Mod(b.textRotation.angle, 360)
	if math.Abs(math.Remainder(angle, 90)) > 0.001 {
		return
	}
	w, _ := b.MeasureText(text)
	metrics := face.Metrics()
	ascent, descent := metrics.Ascent*mmToPt, metrics.Descent*mmToPt
	if w <= 0 || ascent+descent <= 0 {
		return
	}
	switch align {
	case TextAlignCenter:
		x -= w / 2
	case TextAlignRight:
		x -= w
	}
	switch baseline {
	case TextBaselineTop:
		// y already points at the top of the font box.
	case TextBaselineMiddle:
		y -= (ascent + descent) / 2
	case TextBaselineBottom:
		y -= ascent + descent
	default:
		y -= ascent
	}
	r := Rect{X: x, Y: y, W: w, H: ascent + descent}
	if math.Abs(angle) > 0.001 {
		r = rotatedTextRect(r, b.textRotation)
	}
	b.textBoxes = append(b.textBoxes, drawnTextBox{text: text, rect: r})
}

func rotatedTextRect(r Rect, rotation textRotation) Rect {
	rad := rotation.angle * math.Pi / 180
	sin, cos := math.Sin(rad), math.Cos(rad)
	points := [4]Point{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X, r.Y + r.H}, {r.X + r.W, r.Y + r.H}}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range points {
		dx, dy := p.X-rotation.cx, p.Y-rotation.cy
		x := rotation.cx + dx*cos - dy*sin
		y := rotation.cy + dx*sin + dy*cos
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	return Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// textOverlapFindings reports substantial intersections only. Small touches
// caused by font ascenders and conservative metrics are below this threshold.
func (b *SVGBuilder) textOverlapFindings() []core.Finding {
	var findings []core.Finding
	for i, a := range b.textBoxes {
		for _, c := range b.textBoxes[i+1:] {
			w := math.Min(a.rect.X+a.rect.W, c.rect.X+c.rect.W) - math.Max(a.rect.X, c.rect.X)
			h := math.Min(a.rect.Y+a.rect.H, c.rect.Y+c.rect.H) - math.Max(a.rect.Y, c.rect.Y)
			if w < 2 || h < 2 {
				continue
			}
			intersection := w * h
			smaller := math.Min(a.rect.W*a.rect.H, c.rect.W*c.rect.H)
			if smaller <= 0 || intersection/smaller < 0.3 {
				continue
			}
			findings = append(findings, core.Finding{
				Code:     core.FindingDiagramTextOverlap,
				Message:  fmt.Sprintf("drawn text %q overlaps %q by %.0f%% of the smaller text box; shorten a label or increase the diagram space", a.text, c.text, 100*intersection/smaller),
				Severity: "warning",
				Fix:      &core.FixSuggestion{Kind: core.FixKindShortenLabels},
			})
		}
	}
	return findings
}

package pptx

import (
	"bytes"
	"fmt"
)

// Effect is an interface for DrawingML effect elements that can be
// composed inside an <a:effectLst>.
type Effect interface {
	// WriteEffectXML writes this effect's DrawingML XML into buf.
	WriteEffectXML(buf *bytes.Buffer)
}

// Shadow represents an outer shadow effect (<a:outerShdw>).
type Shadow struct {
	// BlurRadius in EMU.
	BlurRadius int64
	// Distance in EMU.
	Distance int64
	// Direction in 60,000ths of a degree (e.g. 2700000 = 45°).
	Direction int64
	// Color is the shadow color fill (solid or scheme).
	Color Fill
	// Alignment is the shadow alignment: "tl", "t", "tr", "l", "ctr", "r", "bl", "b", "br".
	Alignment string
}

// OuterShadow creates a Shadow with convenient point-based dimensions.
// blurPt and distPt are in points, dirDeg is in degrees, color is a hex string,
// and alpha is in thousandths of a percent (e.g. 50000 = 50%).
func OuterShadow(blurPt, distPt float64, dirDeg int, color string, alpha int) Shadow {
	s := Shadow{
		BlurRadius: int64(blurPt * float64(EMUPerPoint)),
		Distance:   int64(distPt * float64(EMUPerPoint)),
		Direction:  int64(dirDeg) * 60000,
		Alignment:  "bl",
	}
	if alpha > 0 {
		s.Color = SolidFill(color)
		s.Color.mods = append(s.Color.mods, Alpha(alpha))
	} else {
		s.Color = SolidFill(color)
	}
	return s
}

// WriteEffectXML writes the <a:outerShdw> element.
func (s Shadow) WriteEffectXML(buf *bytes.Buffer) {
	buf.WriteString(`<a:outerShdw`)
	if s.BlurRadius > 0 {
		fmt.Fprintf(buf, ` blurRad="%d"`, s.BlurRadius)
	}
	if s.Distance > 0 {
		fmt.Fprintf(buf, ` dist="%d"`, s.Distance)
	}
	if s.Direction != 0 {
		fmt.Fprintf(buf, ` dir="%d"`, s.Direction)
	}
	if s.Alignment != "" {
		buf.WriteString(fmt.Sprintf(` algn="%s"`, s.Alignment))
	}
	buf.WriteString(`>`)
	if !s.Color.IsZero() {
		s.Color.WriteColorTo(buf)
	}
	buf.WriteString(`</a:outerShdw>`)
}

// Glow represents a glow effect (<a:glow>).
type Glow struct {
	// Radius in EMU.
	Radius int64
	// Color is the glow color fill (solid or scheme).
	Color Fill
}

// WriteEffectXML writes the <a:glow> element.
func (g Glow) WriteEffectXML(buf *bytes.Buffer) {
	buf.WriteString(fmt.Sprintf(`<a:glow rad="%d">`, g.Radius))
	if !g.Color.IsZero() {
		g.Color.WriteColorTo(buf)
	}
	buf.WriteString(`</a:glow>`)
}

// SoftEdge represents a soft edge effect (<a:softEdge>).
type SoftEdge struct {
	// Radius in EMU.
	Radius int64
}

// WriteEffectXML writes the <a:softEdge> element.
func (se SoftEdge) WriteEffectXML(buf *bytes.Buffer) {
	buf.WriteString(fmt.Sprintf(`<a:softEdge rad="%d"/>`, se.Radius))
}

// WriteEffectList writes an <a:effectLst> element containing the given effects.
// If effects is empty, nothing is written.
func WriteEffectList(buf *bytes.Buffer, effects []Effect) {
	if len(effects) == 0 {
		return
	}
	buf.WriteString(`<a:effectLst>`)
	for _, eff := range effects {
		eff.WriteEffectXML(buf)
	}
	buf.WriteString(`</a:effectLst>`)
}

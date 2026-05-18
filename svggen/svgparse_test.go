package svggen

// Helpers for parsing emitted SVG and reasoning about <text> geometry from
// tests. These exist so axis / label tests can stop pattern-matching regexes
// and instead assert on text-anchor, dominant-baseline, x/y, and the
// post-transform bounding box of every label.
//
// Test-only file (suffix `_test.go`) — not compiled into the production
// binary. All callers must live in `package svggen` tests.

import (
	"encoding/xml"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// SVGTextElement is a parsed <text>...<tspan/>...</text> pair with the
// attributes that matter for axis-label geometry assertions.
//
// X / Y are the SVG attribute values on the <text> element (in user-space px;
// see ViewBoxPxToPt to convert).
//
// TspanX / TspanY are the local offset on the <tspan>. For unrotated labels
// this is the on-screen offset relative to (X, Y). For rotated labels (when
// Transform is non-zero) X / Y are absent and the pivot is encoded in
// TranslateX / TranslateY; TspanX / TspanY then describe the glyph offset
// inside the rotated frame.
//
// Content is the inner chardata of the tspan, trimmed.
//
// Anchor is the text-anchor attribute ("start" if absent, but stored as ""
// when omitted so callers can distinguish "no attribute" from "start").
//
// Baseline is the dominant-baseline attribute (same omitted-vs-present
// semantics as Anchor).
type SVGTextElement struct {
	Content    string
	Anchor     string
	Baseline   string
	X          float64
	Y          float64
	HasX       bool
	HasY       bool
	TspanX     float64
	TspanY     float64
	HasTspanX  bool
	HasTspanY  bool
	TranslateX float64
	TranslateY float64
	Rotation   float64
	HasRotate  bool
}

// HasTransform reports whether the <text> carried a transform="translate(...)
// rotate(...)" attribute. When false, X / Y are the on-screen pivot.
func (t SVGTextElement) HasTransform() bool { return t.HasRotate }

// PivotX returns the on-screen x of the rotation pivot. For an untransformed
// element this is X; for a transformed element it is TranslateX.
func (t SVGTextElement) PivotX() float64 {
	if t.HasTransform() {
		return t.TranslateX
	}
	return t.X
}

// PivotY returns the on-screen y of the rotation pivot (analogous to PivotX).
func (t SVGTextElement) PivotY() float64 {
	if t.HasTransform() {
		return t.TranslateY
	}
	return t.Y
}

// TransformPoint maps a local (lx, ly) coordinate inside the text element's
// rotated frame to on-screen (sx, sy). For an unrotated element it returns
// (lx + X, ly + Y); for a rotated element it applies translate-then-rotate.
//
// SVG semantics: transform="translate(tx,ty) rotate(angle)" first rotates
// around the local origin, then translates. So a point (lx, ly) in the
// local frame maps to:
//
//	sx = tx + cos(theta)*lx - sin(theta)*ly
//	sy = ty + sin(theta)*lx + cos(theta)*ly
func (t SVGTextElement) TransformPoint(lx, ly float64) (sx, sy float64) {
	if !t.HasRotate {
		// Pure translation: tspan offsets accumulate from (X, Y).
		return t.X + lx, t.Y + ly
	}
	theta := t.Rotation * math.Pi / 180
	cos, sin := math.Cos(theta), math.Sin(theta)
	sx = t.TranslateX + cos*lx - sin*ly
	sy = t.TranslateY + sin*lx + cos*ly
	return sx, sy
}

// BBox holds an axis-aligned bounding box in screen coordinates.
type BBox struct {
	MinX, MinY, MaxX, MaxY float64
}

// PostTransformBBox returns the screen-space axis-aligned bounding box of the
// glyph block assuming a glyph width of `width` and an ascent of `ascent`
// (both in the same units as Translate / X / Y, i.e. SVG user-space px).
//
// The local glyph bbox is taken as [tspanX, tspanX+width] × [-ascent, 0]
// (i.e. the glyph extends UP from the baseline by `ascent`). Width is
// caller-supplied (tests typically pass a conservative estimate from
// fontSize × character count) because the canvas library does not emit
// glyph metrics into the SVG.
func (t SVGTextElement) PostTransformBBox(width, ascent float64) BBox {
	lx := t.TspanX
	corners := [4][2]float64{
		{lx, -ascent},
		{lx + width, -ascent},
		{lx, 0},
		{lx + width, 0},
	}
	bb := BBox{
		MinX: math.Inf(1), MinY: math.Inf(1),
		MaxX: math.Inf(-1), MaxY: math.Inf(-1),
	}
	for _, c := range corners {
		sx, sy := t.TransformPoint(c[0], c[1])
		if sx < bb.MinX {
			bb.MinX = sx
		}
		if sx > bb.MaxX {
			bb.MaxX = sx
		}
		if sy < bb.MinY {
			bb.MinY = sy
		}
		if sy > bb.MaxY {
			bb.MaxY = sy
		}
	}
	return bb
}

// ParseSVGTexts walks `svgStr` and returns every <text> element that contains
// a non-empty <tspan>. The element's text-anchor, dominant-baseline, x, y,
// tspan x, tspan y, and transform="translate(tx,ty) rotate(angle)" are
// extracted. Elements without a <tspan> (or with an empty tspan) are skipped.
//
// The parser is lenient: malformed elements are silently dropped. Callers
// should assert on the labels they care about (`FindByContent`) rather than
// indexing into the result slice.
func ParseSVGTexts(t *testing.T, svgStr string) []SVGTextElement {
	t.Helper()
	type tspanXML struct {
		Content string `xml:",chardata"`
		X       string `xml:"x,attr"`
		Y       string `xml:"y,attr"`
	}
	type textXML struct {
		Anchor    string   `xml:"text-anchor,attr"`
		Baseline  string   `xml:"dominant-baseline,attr"`
		X         string   `xml:"x,attr"`
		Y         string   `xml:"y,attr"`
		Transform string   `xml:"transform,attr"`
		Tspan     tspanXML `xml:"tspan"`
	}
	transformRe := regexp.MustCompile(
		`translate\(\s*([0-9.\-]+)\s*,\s*([0-9.\-]+)\s*\)(?:\s*rotate\(\s*([0-9.\-]+)\s*\))?`,
	)

	var out []SVGTextElement
	dec := xml.NewDecoder(strings.NewReader(svgStr))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "text" {
			continue
		}
		var raw textXML
		if err := dec.DecodeElement(&raw, &se); err != nil {
			continue
		}
		content := strings.TrimSpace(raw.Tspan.Content)
		if content == "" {
			continue
		}
		el := SVGTextElement{
			Content:  content,
			Anchor:   raw.Anchor,
			Baseline: raw.Baseline,
		}
		if raw.X != "" {
			if v, err := strconv.ParseFloat(raw.X, 64); err == nil {
				el.X = v
				el.HasX = true
			}
		}
		if raw.Y != "" {
			if v, err := strconv.ParseFloat(raw.Y, 64); err == nil {
				el.Y = v
				el.HasY = true
			}
		}
		if raw.Tspan.X != "" {
			if v, err := strconv.ParseFloat(raw.Tspan.X, 64); err == nil {
				el.TspanX = v
				el.HasTspanX = true
			}
		}
		if raw.Tspan.Y != "" {
			if v, err := strconv.ParseFloat(raw.Tspan.Y, 64); err == nil {
				el.TspanY = v
				el.HasTspanY = true
			}
		}
		if raw.Transform != "" {
			if m := transformRe.FindStringSubmatch(raw.Transform); m != nil {
				tx, _ := strconv.ParseFloat(m[1], 64)
				ty, _ := strconv.ParseFloat(m[2], 64)
				el.TranslateX = tx
				el.TranslateY = ty
				if m[3] != "" {
					if rot, err := strconv.ParseFloat(m[3], 64); err == nil {
						el.Rotation = rot
						el.HasRotate = true
					}
				}
			}
		}
		out = append(out, el)
	}
	return out
}

// FindByContent returns the first SVGTextElement whose Content equals
// `want`, or nil if none matches.
func FindByContent(texts []SVGTextElement, want string) *SVGTextElement {
	for i := range texts {
		if texts[i].Content == want {
			return &texts[i]
		}
	}
	return nil
}

// ViewBoxPxToPt parses the SVG `viewBox="0 0 W H"` attribute and returns the
// px-to-pt scale factor given the builder's pt width. svggen emits SVG units
// in px while the builder's coordinate system is pt, so all on-screen
// comparisons against axisX / axisY (pt) must scale through this factor.
//
// Returns 1.0 if the viewBox cannot be parsed (callers should check the
// `ok` return value and skip the test).
func ViewBoxPxToPt(t *testing.T, svgStr string, builderWidthPt float64) (pxToPt float64, ok bool) {
	t.Helper()
	re := regexp.MustCompile(`viewBox="0 0 ([0-9.]+) ([0-9.]+)"`)
	m := re.FindStringSubmatch(svgStr)
	if m == nil {
		return 1.0, false
	}
	w, err := strconv.ParseFloat(m[1], 64)
	if err != nil || w == 0 {
		return 1.0, false
	}
	return builderWidthPt / w, true
}

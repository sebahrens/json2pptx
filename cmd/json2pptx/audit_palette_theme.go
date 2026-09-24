package main

import (
	"encoding/xml"
	"fmt"
	"image"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

const defaultThemeDeltaE = 15.0

type auditThemeColor struct {
	Scheme string
	Tint   int // OOXML tint modifier, in 100000ths; zero is the base accent.
	Color  svggen.Color
}

type auditChroma struct {
	R, G, B uint8
	Pixels  int
}

type auditChromaBucket struct {
	r, g, b uint64
	n       int
}

type auditThemeMatch struct {
	Slide              int         `json:"slide"`
	Pic                auditRegion `json:"pic"`
	Hex                string      `json:"hex"`
	PixelCount         int         `json:"pixel_count"`
	NearestSchemeColor string      `json:"nearest_scheme_color"`
	NearestTint        int         `json:"nearest_tint"`
	NearestHex         string      `json:"nearest_hex"`
	DeltaE             float64     `json:"delta_e"`
	Pass               bool        `json:"pass"`
}

func validAuditMode(mode string) bool {
	return mode == "theme" || mode == "pair" || mode == "both"
}

// readAuditThemeColors deliberately has no fallback palette: a missing or
// malformed theme cannot give a meaningful on-brand verdict.
func readAuditThemeColors(pkg *pptx.Package) ([]auditThemeColor, error) {
	data, err := pkg.ReadEntry("ppt/theme/theme1.xml")
	if err != nil {
		return nil, fmt.Errorf("read ppt/theme/theme1.xml: %w", err)
	}
	return parseAuditThemeColors(data)
}

func parseAuditThemeColors(data []byte) ([]auditThemeColor, error) {
	var theme pptx.ThemeXML
	if err := xml.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("parse ppt/theme/theme1.xml: %w", err)
	}
	cs := theme.ThemeElements.ColorScheme
	defs := []struct {
		name string
		def  pptx.ColorDefXML
	}{
		{"accent1", cs.Accent1}, {"accent2", cs.Accent2}, {"accent3", cs.Accent3},
		{"accent4", cs.Accent4}, {"accent5", cs.Accent5}, {"accent6", cs.Accent6},
	}
	colors := make([]auditThemeColor, 0, len(defs)*5)
	for _, d := range defs {
		hex := d.def.SRGBColor.Val
		if hex == "" {
			hex = d.def.SystemColor.LastClr
		}
		base, err := svggen.ParseColor("#" + strings.TrimPrefix(hex, "#"))
		if err != nil {
			return nil, fmt.Errorf("theme %s has invalid color %q: %w", d.name, hex, err)
		}
		colors = append(colors, auditThemeColor{Scheme: d.name, Color: base})
		for _, tint := range []int{20000, 40000, 60000, 80000} {
			colors = append(colors, auditThemeColor{
				Scheme: d.name,
				Tint:   tint,
				Color:  patterns.EffectiveColorMods(base, patterns.ColorMods{Tint: tint}, svggen.Color{R: 255, G: 255, B: 255, A: 1}),
			})
		}
	}
	return colors, nil
}

func nearestAuditThemeColor(slide int, pic auditRegion, chroma auditChroma, colors []auditThemeColor, threshold float64) auditThemeMatch {
	match := auditThemeMatch{
		Slide: slide, Pic: pic, Hex: fmt.Sprintf("%02x%02x%02x", chroma.R, chroma.G, chroma.B),
		PixelCount: chroma.Pixels, DeltaE: math.Inf(1),
	}
	for _, c := range colors {
		delta := deltaE76Hex(chroma.R, chroma.G, chroma.B, c.Color.R, c.Color.G, c.Color.B)
		if delta < match.DeltaE {
			match.DeltaE = delta
			match.NearestSchemeColor = c.Scheme
			match.NearestTint = c.Tint
			match.NearestHex = strings.TrimPrefix(strings.ToLower(c.Color.Hex()), "#")
		}
	}
	match.Pass = match.DeltaE <= threshold
	return match
}

// auditAccentMod captures a native shape fill brightened in HSL. In a
// saturated template this can drift far from the accent's RGB tints.
type auditAccentMod struct {
	Name   string
	Scheme string
	Mod    int
	Off    int
}

func extractAuditAccentMods(slideXML []byte) ([]auditAccentMod, error) {
	var slide auditSlideXML
	if err := xml.Unmarshal(slideXML, &slide); err != nil {
		return nil, err
	}
	var out []auditAccentMod
	var walk func(auditSpTree)
	walk = func(tree auditSpTree) {
		for _, shape := range tree.Shapes {
			fill := shape.SpPr.SolidFill
			if fill == nil || fill.Scheme == nil || !strings.HasPrefix(fill.Scheme.Val, "accent") || fill.Scheme.LumOff == nil || fill.Scheme.LumOff.Val <= 0 {
				continue
			}
			mod := 100000
			if fill.Scheme.LumMod != nil {
				mod = fill.Scheme.LumMod.Val
			}
			out = append(out, auditAccentMod{Name: shape.NvSpPr.CNvPr.Name, Scheme: fill.Scheme.Val, Mod: mod, Off: fill.Scheme.LumOff.Val})
		}
		for _, group := range tree.Groups {
			walk(group)
		}
	}
	walk(slide.CSld.SpTree)
	return out, nil
}

func scoreAuditAccentMods(slide *auditSlide, index int, mods []auditAccentMod, colors []auditThemeColor, threshold float64) int {
	violations := 0
	for _, m := range mods {
		var base svggen.Color
		found := false
		var schemeColors []auditThemeColor
		for _, c := range colors {
			if c.Scheme == m.Scheme {
				schemeColors = append(schemeColors, c)
				if c.Tint == 0 {
					base, found = c.Color, true
				}
			}
		}
		if !found {
			continue
		}
		actual := patterns.EffectiveColorMods(base, patterns.ColorMods{LumMod: m.Mod, LumOff: m.Off}, svggen.Color{R: 255, G: 255, B: 255, A: 1})
		shape := auditRegion{Kind: "shape", Name: m.Name, DeclaredHex: m.Scheme}
		match := nearestAuditThemeColor(index, shape, auditChroma{R: actual.R, G: actual.G, B: actual.B}, schemeColors, threshold)
		slide.ThemeMatches = append(slide.ThemeMatches, match)
		if !match.Pass {
			violations++
		}
		if match.DeltaE > slide.MaxThemeDeltaE {
			slide.MaxThemeDeltaE = match.DeltaE
		}
	}
	slide.ThemeMatchCount = len(slide.ThemeMatches)
	return violations
}

// dominantChromasPx returns distinct, material color clusters rather than an
// average of every colored pixel. The latter blends two legitimate series into
// a fictitious third color and can hide a minority off-brand series.
func dominantChromasPx(img image.Image, rect [4]int, chromaMin uint8) []auditChroma {
	bins, total := chromaHistogramPx(img, rect, chromaMin)
	if total == 0 {
		return nil
	}
	// A material series must cover at least 1% of chromatic pixels. The floor
	// also rejects antialiased edge shades without hiding small chart series.
	minimum := max(4, (total+99)/100)
	colors := make([]auditChroma, 0, len(bins))
	for _, v := range bins {
		if v.n >= minimum {
			colors = append(colors, auditChroma{R: uint8(v.r / uint64(v.n)), G: uint8(v.g / uint64(v.n)), B: uint8(v.b / uint64(v.n)), Pixels: v.n})
		}
	}
	sort.Slice(colors, func(i, j int) bool {
		if colors[i].Pixels != colors[j].Pixels {
			return colors[i].Pixels > colors[j].Pixels
		}
		left := uint32(colors[i].R)<<16 | uint32(colors[i].G)<<8 | uint32(colors[i].B)
		right := uint32(colors[j].R)<<16 | uint32(colors[j].G)<<8 | uint32(colors[j].B)
		return left < right
	})
	// Adjacent quantization buckets are usually the same antialiased fill.
	result := make([]auditChroma, 0, min(len(colors), 16))
	for _, c := range colors {
		duplicate := false
		for _, kept := range result {
			if deltaE76Hex(c.R, c.G, c.B, kept.R, kept.G, kept.B) < 6 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, c)
		}
		if len(result) == 16 {
			break
		}
	}
	return result
}

func chromaHistogramPx(img image.Image, rect [4]int, chromaMin uint8) (map[uint16]*auditChromaBucket, int) {
	b := img.Bounds()
	x0, y0 := max(rect[0], b.Min.X), max(rect[1], b.Min.Y)
	x1, y1 := min(rect[2], b.Max.X), min(rect[3], b.Max.Y)
	if x1 <= x0 || y1 <= y0 {
		return nil, 0
	}
	bins := make(map[uint16]*auditChromaBucket)
	total := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r16, g16, b16, _ := img.At(x, y).RGBA()
			r, g, blue := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)
			maxc, minc := r, r
			if g > maxc {
				maxc = g
			}
			if g < minc {
				minc = g
			}
			if blue > maxc {
				maxc = blue
			}
			if blue < minc {
				minc = blue
			}
			if maxc-minc < chromaMin {
				continue
			}
			key := uint16(r>>4)<<8 | uint16(g>>4)<<4 | uint16(blue>>4)
			v := bins[key]
			if v == nil {
				v = &auditChromaBucket{}
				bins[key] = v
			}
			v.r += uint64(r)
			v.g += uint64(g)
			v.b += uint64(blue)
			v.n++
			total++
		}
	}
	return bins, total
}

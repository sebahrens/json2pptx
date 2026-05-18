// Package heuristic provides rule-based visual QA checks that operate on
// rendered slide images without requiring a vision API.
//
// These checks are an offline fallback for inspect_slide_images when
// ANTHROPIC_API_KEY is unset. They are intentionally conservative — each
// check has a clear, simple signal (image entropy, edge-band content,
// aspect ratio) and produces findings tagged with Source="heuristic" and
// SeverityP3 (advisory). Vision-backed findings remain distinguishable
// by their Source="vision" tag and the Report.Mode field.
//
// What this package does NOT do:
//   - It does not call any external service.
//   - It does not attempt the rich layout reasoning the vision model performs.
//   - It does not produce repair_slide auto-fix mappings beyond what
//     visualqa.SuggestedFixesForCategory provides for the canonical categories.
//
// Practical use: agents that hit INSPECT_DISABLED in earlier versions now
// receive a degraded-but-useful report. They can still iterate via
// repair_slide for findings whose categories map to fix kinds, and surface
// the rest for human review.
package heuristic

import (
	"bytes"
	"fmt"
	"image"
	// Register PNG and JPEG decoders for image.Decode.
	_ "image/jpeg"
	_ "image/png"

	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// SourceTag is the value written to Finding.Source for heuristic findings.
const SourceTag = "heuristic"

// edgeBandFraction is the relative width of the border band scanned for
// content touching the slide edge. 0.01 = 1% of the smaller dimension.
const edgeBandFraction = 0.01

// edgeContentRatioThreshold is the minimum fraction of "content" pixels
// (non-background) within an edge band required to flag a potential overflow.
// Tuned conservatively to favour low false positives over recall.
const edgeContentRatioThreshold = 0.05

// blankContentRatioThreshold is the maximum non-background pixel fraction
// below which we flag the slide as effectively blank.
const blankContentRatioThreshold = 0.005

// aspectTolerance defines how close to 16:9 or 4:3 the image must be to be
// considered "standard". Larger deviations are flagged.
const aspectTolerance = 0.02

// Inspect runs all heuristic checks on a single decoded slide image and
// returns a visualqa.SlideResult. Findings are tagged with
// Source="heuristic" and SeverityP3 (advisory).
//
// If the image cannot be decoded, the returned SlideResult has Error set
// and no findings.
func Inspect(data []byte, info visualqa.SlideInfo) visualqa.SlideResult {
	res := visualqa.SlideResult{
		SlideIndex: info.Index,
		SlideType:  info.Type,
		RawOutput:  "heuristic mode (no vision API)",
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		res.Error = fmt.Sprintf("decode image: %v", err)
		return res
	}

	bg, contentRatio := analyzePixels(img)

	res.Findings = append(res.Findings, checkBlank(info, contentRatio)...)
	res.Findings = append(res.Findings, checkEdgeOverflow(img, bg, info)...)
	res.Findings = append(res.Findings, checkAspectRatio(img, info)...)

	return res
}

// InspectAll runs heuristic checks on every slide and returns a Report
// with Mode="heuristic".
func InspectAll(slides []visualqa.SlideImage) *visualqa.Report {
	report := &visualqa.Report{
		SlideCount: len(slides),
		Results:    make([]visualqa.SlideResult, len(slides)),
		Mode:       SourceTag,
	}
	for i, s := range slides {
		report.Results[i] = Inspect(s.Data, s.Info)
	}
	report.Summarize()
	return report
}

// analyzePixels samples the image to find the dominant background colour
// and the fraction of non-background pixels.
//
// The background is estimated as the most common quantized color across a
// coarse pixel grid (every ~strideStep pixels), and the content ratio is
// the fraction of sampled pixels whose color differs from the background
// by more than colorDistanceThreshold in 8-bit-per-channel Euclidean space.
func analyzePixels(img image.Image) (bg color8, contentRatio float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// Coarse-grid sampling to keep this O(image area / stride^2).
	stride := w / 200
	if stride < 1 {
		stride = 1
	}
	if hs := h / 200; hs > stride {
		stride = hs
	}

	counts := make(map[color8]int)
	var sampled int
	for y := b.Min.Y; y < b.Max.Y; y += stride {
		for x := b.Min.X; x < b.Max.X; x += stride {
			c := quantizeAt(img, x, y)
			counts[c]++
			sampled++
		}
	}
	if sampled == 0 {
		return color8{255, 255, 255}, 0
	}

	// Pick the highest-count quantized colour as the background.
	var maxCount int
	for c, n := range counts {
		if n > maxCount {
			bg = c
			maxCount = n
		}
	}

	// Second pass to compute the content ratio (pixels meaningfully
	// different from bg). We reuse the same grid.
	var nonBG int
	for y := b.Min.Y; y < b.Max.Y; y += stride {
		for x := b.Min.X; x < b.Max.X; x += stride {
			if !nearBackground(img, x, y, bg) {
				nonBG++
			}
		}
	}
	contentRatio = float64(nonBG) / float64(sampled)
	return bg, contentRatio
}

// checkBlank flags a slide whose non-background content is below
// blankContentRatioThreshold. Likely missed content / blank template.
func checkBlank(info visualqa.SlideInfo, contentRatio float64) []visualqa.Finding {
	if contentRatio >= blankContentRatioThreshold {
		return nil
	}
	return []visualqa.Finding{newHeuristic(
		info,
		"missing_content",
		fmt.Sprintf("Slide appears nearly blank (content ratio %.3f%%). Heuristic detected very little non-background content; verify body shapes were populated.", contentRatio*100),
		"whole slide",
	)}
}

// checkEdgeOverflow scans 1%-wide bands along each edge for non-background
// pixels. Significant edge content suggests text or shapes flush against
// the slide boundary, a common overflow signal.
func checkEdgeOverflow(img image.Image, bg color8, info visualqa.SlideInfo) []visualqa.Finding {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	band := int(float64(min(w, h)) * edgeBandFraction)
	if band < 1 {
		band = 1
	}

	type edge struct {
		name           string
		x0, y0, x1, y1 int
	}
	edges := []edge{
		{"top edge", b.Min.X, b.Min.Y, b.Max.X, b.Min.Y + band},
		{"bottom edge", b.Min.X, b.Max.Y - band, b.Max.X, b.Max.Y},
		{"left edge", b.Min.X, b.Min.Y, b.Min.X + band, b.Max.Y},
		{"right edge", b.Max.X - band, b.Min.Y, b.Max.X, b.Max.Y},
	}

	var findings []visualqa.Finding
	for _, e := range edges {
		ratio := nonBackgroundRatio(img, bg, e.x0, e.y0, e.x1, e.y1)
		if ratio >= edgeContentRatioThreshold {
			findings = append(findings, newHeuristic(
				info,
				"text_overflow",
				fmt.Sprintf("%.1f%% of pixels in the %s contain non-background content; possible overflow or content touching the slide edge.", ratio*100, e.name),
				e.name,
			))
		}
	}
	return findings
}

// checkAspectRatio flags images whose width/height ratio differs noticeably
// from 16:9 or 4:3. These are review-only — the aspect ratio is usually a
// template-level choice, but unexpected ratios are worth surfacing.
func checkAspectRatio(img image.Image, info visualqa.SlideInfo) []visualqa.Finding {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	r := float64(w) / float64(h)
	const r16x9 = 16.0 / 9.0
	const r4x3 = 4.0 / 3.0
	if absDelta(r, r16x9) <= aspectTolerance || absDelta(r, r4x3) <= aspectTolerance {
		return nil
	}
	return []visualqa.Finding{newHeuristic(
		info,
		"aspect_ratio",
		fmt.Sprintf("Image aspect ratio %.3f does not match 16:9 (%.3f) or 4:3 (%.3f). Verify the render pipeline used the intended slide dimensions.", r, r16x9, r4x3),
		"whole slide",
	)}
}

// newHeuristic builds a Finding pre-populated with the heuristic Source tag,
// SeverityP3, and any auto-fix mappings registered for the category.
func newHeuristic(info visualqa.SlideInfo, category, description, location string) visualqa.Finding {
	return visualqa.Finding{
		SlideIndex:     info.Index,
		SlideType:      info.Type,
		Severity:       visualqa.SeverityP3,
		Category:       category,
		Description:    description,
		Location:       location,
		Source:         SourceTag,
		SuggestedFixes: visualqa.SuggestedFixesForCategory(category),
	}
}

// color8 is an 8-bit-per-channel RGB color used for quantized background
// detection.
type color8 struct {
	r, g, b uint8
}

// quantizeAt returns the colour of the pixel at (x, y) quantized to a
// 32-step palette per channel. Quantization absorbs minor compression and
// dithering noise so the background detection is stable across re-renders.
func quantizeAt(img image.Image, x, y int) color8 {
	r, g, b, _ := img.At(x, y).RGBA()
	return color8{
		r: uint8(r>>8) & 0xF8, // keep top 5 bits → 32 levels
		g: uint8(g>>8) & 0xF8,
		b: uint8(b>>8) & 0xF8,
	}
}

// nearBackground reports whether the pixel at (x, y) is within
// colorDistanceThreshold of the dominant background colour.
func nearBackground(img image.Image, x, y int, bg color8) bool {
	c := quantizeAt(img, x, y)
	const threshold = 24 // squared-distance threshold ~= per-channel ±13 units
	dr := int(c.r) - int(bg.r)
	dg := int(c.g) - int(bg.g)
	db := int(c.b) - int(bg.b)
	return absInt(dr)+absInt(dg)+absInt(db) <= threshold
}

// nonBackgroundRatio returns the fraction of pixels in the rectangle
// [x0,y0)–[x1,y1) whose colour is meaningfully different from bg.
func nonBackgroundRatio(img image.Image, bg color8, x0, y0, x1, y1 int) float64 {
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	w := x1 - x0
	h := y1 - y0
	stride := 1
	// Cap per-edge sampling at ~10k pixels to keep this fast on large images.
	if total := w * h; total > 10000 {
		// Stride the larger dimension so we sample ~10k cells.
		stride = (total / 10000)
		if stride < 1 {
			stride = 1
		}
	}
	var seen, nonBG int
	for y := y0; y < y1; y += stride {
		for x := x0; x < x1; x += stride {
			seen++
			if !nearBackground(img, x, y, bg) {
				nonBG++
			}
		}
	}
	if seen == 0 {
		return 0
	}
	return float64(nonBG) / float64(seen)
}

func absDelta(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

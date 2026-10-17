package generator

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// svgImageDimensions returns intrinsic dimensions for placement. Explicit root
// dimensions describe the viewport; viewBox supplies its aspect when dimensions
// are omitted or relative.
func svgImageDimensions(data []byte) (float64, float64, error) {
	var root struct {
		XMLName xml.Name
		Width   string `xml:"width,attr"`
		Height  string `xml:"height,attr"`
		ViewBox string `xml:"viewBox,attr"`
	}
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		return 0, 0, err
	}
	if root.XMLName.Local != "svg" {
		return 0, 0, fmt.Errorf("image root is not svg")
	}
	w, h := svgLengthPixels(root.Width), svgLengthPixels(root.Height)
	if w <= 0 || h <= 0 {
		fields := strings.Fields(strings.ReplaceAll(root.ViewBox, ",", " "))
		if len(fields) == 4 {
			values := [4]float64{}
			valid := true
			for i, field := range fields {
				v, err := strconv.ParseFloat(field, 64)
				if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
					valid = false
				}
				values[i] = v
			}
			vw, vh := values[2], values[3]
			if valid && vw > 0 && vh > 0 {
				switch {
				case w > 0:
					h = w * vh / vw
				case h > 0:
					w = h * vw / vh
				default:
					w, h = vw, vh
				}
			}
		}
	}
	if w <= 0 || h <= 0 || math.IsNaN(w) || math.IsNaN(h) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return 0, 0, fmt.Errorf("SVG has no positive intrinsic dimensions or viewBox")
	}
	return w, h, nil
}

func svgLengthPixels(value string) float64 {
	value = strings.TrimSpace(value)
	factor := 1.0
	for _, unit := range []struct {
		suffix string
		factor float64
	}{
		{"px", 1}, {"in", 96}, {"cm", 96 / 2.54}, {"mm", 96 / 25.4},
		{"pt", 96.0 / 72}, {"pc", 16}, {"Q", 96 / 101.6},
	} {
		if strings.HasSuffix(value, unit.suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			factor = unit.factor
			break
		}
	}
	n, _ := strconv.ParseFloat(value, 64)
	return n * factor
}

// registerGridSVGImage uses the same placement transform as raster cells and
// overlay anchors. The native and fallback images share both frame and crop.
func (ctx *singlePassContext) registerGridSVGImage(slideNum, imgIdx int, img ImageInsert) error {
	data, err := os.ReadFile(img.Path)
	if err != nil {
		return fmt.Errorf("slide %d shape_grid SVG image: %w", slideNum, err)
	}
	placed, crop := gridImagePlacement(img)
	c := ctx.svgConverter
	// Render the complete source at the size needed after cropping. A wide
	// SVG cover-filled into a square needs more source pixels than its frame.
	rasterW, rasterH := placed.Width, placed.Height
	if crop != nil {
		rasterW = int64(math.Ceil(float64(rasterW) * 100000 / float64(100000-crop.L-crop.R)))
		rasterH = int64(math.Ceil(float64(rasterH) * 100000 / float64(100000-crop.T-crop.B)))
	}
	pngData, err := c.rasterizeWithinMaxWidth(ctx.ctx, data, c.gridDiagramRasterSizePx(rasterW, rasterH))
	if err != nil {
		return fmt.Errorf("slide %d shape_grid SVG image fallback PNG: %w", slideNum, err)
	}
	ctx.recordPictureFrame(slideNum, placed)
	sourceID := fmt.Sprintf("gridsvg-s%d-i%d", slideNum, imgIdx)
	if c.GetStrategy() != SVGStrategyNative {
		ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
			mediaFileName: ctx.allocPNG(sourceID), data: pngData, description: img.Alt,
			offsetX: placed.X, offsetY: placed.Y, extentCX: placed.Width, extentCY: placed.Height,
			placeholderIdx: -1, crop: crop, geometry: img.Geometry, behindRawShapes: img.BehindShapes,
		})
		return nil
	}
	svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(sourceID)
	ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
		svgData: data, pngData: pngData, svgMediaFile: svgMediaFile, pngMediaFile: pngMediaFile,
		description: img.Alt, offsetX: placed.X, offsetY: placed.Y, extentCX: placed.Width, extentCY: placed.Height,
		placeholderIdx: -1, crop: crop, geometry: img.Geometry, behindText: img.BehindShapes,
	})
	return nil
}

package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

const chromeCollisionMinOverlapEMU int64 = 228600 // 0.25 inch

// collectChromeCollisionFindings reports template artwork that intrudes into
// a populated body placeholder. The renderer reserves tall side artwork, so
// this is a review signal that the authored content area was narrowed rather
// than a false claim that the final slide still overlaps.
func collectChromeCollisionFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth int64) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)
		if layout == nil {
			continue
		}
		for _, content := range slide.Content {
			ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
			if ph == nil || (ph.Type != types.PlaceholderBody && ph.Type != types.PlaceholderContent && ph.Type != types.PlaceholderImage && ph.Type != types.PlaceholderChart && ph.Type != types.PlaceholderTable) {
				continue
			}
			for _, decor := range layout.DecorRegions {
				if !sideArtworkOverlaps(ph.Bounds, decor, slideWidth) {
					continue
				}
				findings = append(findings, patterns.FitFinding{
					ValidationError: patterns.ValidationError{
						Pattern: "template", Path: slidepath.Content(si, content.PlaceholderID), Code: patterns.ErrCodeChromeCollision,
						Message: fmt.Sprintf("layout %q %s placeholder intersects %s artwork %q; review the template geometry (tall side artwork is reserved at render)", layout.Name, content.PlaceholderID, decor.Source, decor.Name),
					},
					Action: "review",
				})
				break // one finding per authored content item
			}
		}
	}
	return findings
}

func sideArtworkOverlaps(bounds types.BoundingBox, decor types.DecorRegion, slideWidth int64) bool {
	if bounds.Width <= 0 || bounds.Height <= 0 || decor.Width <= 0 || decor.Height <= 0 || slideWidth <= 0 || decor.Width*5 >= slideWidth*2 {
		return false
	}
	xEnd := bounds.X + bounds.Width
	if artEnd := decor.X + decor.Width; artEnd < xEnd {
		xEnd = artEnd
	}
	xStart := bounds.X
	if decor.X > xStart {
		xStart = decor.X
	}
	yEnd := bounds.Y + bounds.Height
	if artEnd := decor.Y + decor.Height; artEnd < yEnd {
		yEnd = artEnd
	}
	yStart := bounds.Y
	if decor.Y > yStart {
		yStart = decor.Y
	}
	xOverlap, yOverlap := xEnd-xStart, yEnd-yStart
	return xOverlap >= chromeCollisionMinOverlapEMU && yOverlap >= chromeCollisionMinOverlapEMU &&
		((decor.X <= bounds.X && decor.X+decor.Width > bounds.X) || (decor.X < bounds.X+bounds.Width && decor.X+decor.Width >= bounds.X+bounds.Width))
}

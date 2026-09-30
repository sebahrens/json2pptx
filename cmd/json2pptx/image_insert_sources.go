package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// authoredImageSources maps each image file path a slide authors — pattern
// image values and shape_grid image cells at any nesting depth — to the JSON
// pointer of the field that supplied it. It must run BEFORE pattern / compose
// expansion replaces slide.ShapeGrid with the generated grid, so the
// generator can address an IMAGE_ASSET_UNAVAILABLE finding to the field the
// author wrote (/slides/0/pattern/values/image) rather than to an expanded
// grid cell the author never saw (go-slide-creator-b7qqg.2).
func authoredImageSources(slide *SlideInput, slideIdx int) map[string]string {
	sources := map[string]string{}
	if slide.ShapeGrid != nil {
		collectGridImageSources(slide.ShapeGrid, slidepath.ShapeGrid(slideIdx), sources)
	}
	refs, _ := patternImageRefs(slide, slideIdx)
	for _, r := range refs {
		if r.img.Path != "" {
			sources[r.img.Path] = r.path
		}
	}
	return sources
}

// collectGridImageSources records the image cells of grid (and of every
// nested cell grid) under prefix. The first authored occurrence of a path
// wins.
func collectGridImageSources(grid *ShapeGridInput, prefix string, sources map[string]string) {
	for r := range grid.Rows {
		for c, cell := range grid.Rows[r].Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", prefix, r, c)
			if cell.Image != nil && cell.Image.Path != "" {
				if _, seen := sources[cell.Image.Path]; !seen {
					sources[cell.Image.Path] = cellPath + "/image"
				}
			}
			if cell.Grid != nil {
				collectGridImageSources(cell.Grid, cellPath+"/grid", sources)
			}
		}
	}
}

// annotateImageInsertSources stamps SourcePath on each insert whose file
// path an authored field supplied.
func annotateImageInsertSources(inserts []generator.ImageInsert, sources map[string]string) {
	for k := range inserts {
		if inserts[k].SourcePath != "" {
			continue
		}
		if src, ok := sources[inserts[k].Path]; ok {
			inserts[k].SourcePath = src
		}
	}
}

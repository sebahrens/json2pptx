package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// gridImageCoverCrop returns the a:srcRect that cover-fills a shape_grid
// image cell: the picture keeps its aspect ratio and is centre-cropped to the
// cell's frame instead of being stretched (which distorted every photo whose
// aspect differed from the cell). Nil when the image already matches the
// frame or its size cannot be read (the plain stretch is then kept).
func gridImageCoverCrop(img ImageInsert) *pptx.SrcRect {
	c, ok := utils.CoverCropForFile(img.Path, img.ExtentCX, img.ExtentCY)
	if !ok || c.IsZero() {
		return nil
	}
	return &pptx.SrcRect{L: c.L, T: c.T, R: c.R, B: c.B}
}

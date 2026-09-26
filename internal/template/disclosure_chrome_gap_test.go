package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureChromeSourceTakeawayAndContentStayOutsideLegalBand(t *testing.T) {
	for _, y := range []int64{1000000, 5000000} {
		layout := types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
			{ID: "legal_disclosure", Type: types.PlaceholderBody, Role: types.PlaceholderRoleDisclosure, Bounds: types.BoundingBox{Y: y, Width: 6000000, Height: 500000}},
		}}
		frame := ResolveChromeFrame(&layout, nil, 12192000, 6858000, true, true)
		for _, rect := range []ChromeRect{frame.Content, frame.Source, frame.Takeaway} {
			if rect.CY <= 0 {
				continue
			}
			if y < 6858000/2 && rect.Y < y+500000 {
				t.Errorf("chrome content enters upper legal text: %+v", rect)
			}
			if y >= 6858000/2 && rect.Y+rect.CY > y {
				t.Errorf("chrome content enters lower legal text: %+v", rect)
			}
		}
	}
}

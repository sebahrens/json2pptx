package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureGridBoundsDoNotEnterProtectedLegalBand(t *testing.T) {
	for _, y := range []int64{1000000, 5000000} {
		layout := types.LayoutMetadata{ID: "closing", Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{Y: 100000, Width: 6000000, Height: 500000}},
			{ID: "legal_disclosure", Type: types.PlaceholderBody, Role: types.PlaceholderRoleDisclosure, Bounds: types.BoundingBox{Y: y, Width: 6000000, Height: 500000}},
			{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{Y: 2000000, Width: 6000000, Height: 3000000}},
		}}
		body, ok := firstBodyOrContentBounds(&layout)
		if !ok || body.Y != 2000000 {
			t.Fatal("grid fallback selected legal text as main content")
		}
		for _, zone := range []*shapegrid.ContentZone{contentZoneFromLayout(&layout, 12192000, 6858000), titleOnlyContentZone(&layout, 12192000, 6858000)} {
			if zone == nil {
				t.Fatal("missing protected content zone")
			}
			if y < 6858000/2 && zone.TitleBottom < y+500000 {
				t.Errorf("grid content starts inside upper legal text: %+v", zone)
			}
			if y >= 6858000/2 && zone.FooterTop > y {
				t.Errorf("grid content extends into lower legal text: %+v", zone)
			}
		}
	}
}

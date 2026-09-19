package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestGridFooterTopUsesTheResolvedFooter is the render half of
// go-slide-creator-p41d6: the grid's bottom boundary comes from the layout's
// RESOLVED footer regions (layout else master), not from whichever utility
// placeholder happens to come first in document order, and not from a fixed
// bottom margin.
func TestGridFooterTopUsesTheResolvedFooter(t *testing.T) {
	const slideH int64 = 6858000
	const resolved int64 = 6356350

	t.Run("resolved regions win over placeholder order", func(t *testing.T) {
		layout := &types.LayoutMetadata{
			ID: "slideLayout7",
			// A lower utility placeholder appears first: the old scan took it and
			// let the grid run into the higher chrome rect.
			Placeholders: []types.PlaceholderInfo{
				{ID: "sldNum", Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 1, Y: 6600000, Width: 100, Height: 100}},
			},
			FooterRegions: []types.ChromeRegion{
				{Type: "ftr", X: 1, Y: resolved, Width: 100, Height: 365125},
				{Type: "sldNum", X: 2, Y: 6600000, Width: 100, Height: 100},
			},
		}
		got, ok := gridFooterTop(layout, slideH)
		if !ok || got != resolved {
			t.Errorf("gridFooterTop = %d (ok=%v), want the highest resolved chrome rect %d", got, ok, resolved)
		}
	})

	t.Run("falls back to a placeholder when there are no regions", func(t *testing.T) {
		layout := &types.LayoutMetadata{
			ID: "x",
			Placeholders: []types.PlaceholderInfo{
				{ID: "ftr", Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 1, Y: 6500000, Width: 100, Height: 100}},
			},
		}
		if got, ok := gridFooterTop(layout, slideH); !ok || got != 6500000 {
			t.Errorf("gridFooterTop = %d (ok=%v), want the placeholder's 6500000", got, ok)
		}
	})

	t.Run("no footer at all keeps the bottom margin", func(t *testing.T) {
		got, ok := gridFooterTop(&types.LayoutMetadata{ID: "x"}, slideH)
		if ok {
			t.Error("a layout with no footer should report ok=false")
		}
		if want := slideH - 365760; got != want {
			t.Errorf("gridFooterTop = %d, want the %d bottom margin", got, want)
		}
	})
}

// TestGridBoundsClearTheFooter pins the outcome on a real bundled template: the
// grid area a blank layout hands a pattern must end ABOVE the footer line that
// examine_template reports, with clearance rather than flush against it.
func TestGridBoundsClearTheFooter(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/midnight-blue.pptx")
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse layouts: %v", err)
	}
	w, h := template.ParseSlideDimensions(reader)

	var checked int
	for i := range layouts {
		layout := &layouts[i]
		footerTop, ok := template.FooterTop(layout, h)
		if !ok {
			continue
		}
		res := pickBlankLayout(layout, nil, w, h)
		if res == nil {
			continue
		}
		checked++
		if bottom := res.Bounds.Y + res.Bounds.CY; bottom > footerTop {
			t.Errorf("%s: grid bottom %d runs past the footer line %d", layout.ID, bottom, footerTop)
		}
		if res.Zone != nil && res.Zone.FooterTop != footerTop {
			t.Errorf("%s: zone footer_top %d != the resolved %d", layout.ID, res.Zone.FooterTop, footerTop)
		}
	}
	if checked == 0 {
		t.Fatal("no layout with a footer was exercised")
	}
}

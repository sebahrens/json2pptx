package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Native text fitting needs separate role regions; checking a short title's
// glyphs alone hides the old overlap when a longer title fills the same frame.
func TestModernSectionNumberTitleClearance(t *testing.T) {
	r, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	layouts, err := ParseLayouts(r)
	if err != nil {
		t.Fatal(err)
	}
	var title, number *types.PlaceholderInfo
	for _, layout := range layouts {
		if layout.ID != "slideLayout2" {
			continue
		}
		for _, ph := range layout.Placeholders {
			ph := ph
			if ph.Type == types.PlaceholderTitle {
				title = &ph
			}
			if ph.ID == "Section Number" {
				number = &ph
			}
		}
	}
	if title == nil || number == nil {
		t.Fatal("native section title/number missing")
	}
	const halfCentimeter = 180000
	if gap := title.Bounds.Y - (number.Bounds.Y + number.Bounds.Height); gap < halfCentimeter {
		t.Fatalf("section number/title frame gap=%d EMU, need >=%d", gap, halfCentimeter)
	}
	if title.FontSize != 6500 || number.FontSize != 9600 {
		t.Fatalf("template typography changed: title=%d number=%d", title.FontSize, number.FontSize)
	}
	if title.Bounds.Height <= 0 || number.Bounds.Height < int64(number.FontSize)*127 {
		t.Fatal("role regions cannot hold their native text size")
	}
}

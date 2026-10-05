package generator

import "testing"

// go-slide-creator-tvy9q: a template that parks its date placeholder outside
// the slide (PowerPoint's way of hiding it) while keeping a wide visible
// footer placeholder had its left footer text anchored on the hidden date's
// x: a 3in box starting mid-slide, with the nine-inch footer slot left empty.

// hiddenDateFooterPositions is that geometry: dt below the 6858000 EMU slide
// edge, ftr spanning 1066800..9881616, a narrow sldNum in the right corner.
func hiddenDateFooterPositions() map[string]*transformXML {
	return map[string]*transformXML{
		"type:dt":     {Offset: offsetXML{X: 5265850, Y: 7056786}, Extent: extentXML{CX: 2743200, CY: 161583}},
		"type:ftr":    {Offset: offsetXML{X: 1066800, Y: 6497441}, Extent: extentXML{CX: 8814816, CY: 161583}},
		"type:sldNum": {Offset: offsetXML{X: 11503818, Y: 6501384}, Extent: extentXML{CX: 298209, CY: 161583}},
	}
}

func TestResolveFooterPositions_OffSlideDateAnchorsOnFooter(t *testing.T) {
	const slideW, slideH int64 = 12192000, 6858000
	positions := resolveFooterPositions(hiddenDateFooterPositions(), slideH)

	box := leftFooterBox(positions)
	if box == nil {
		t.Fatal("no left footer box")
	}
	if box.Offset.X != 1066800 || box.Extent.CX != 8814816 {
		t.Errorf("left footer box = x %d w %d, want the visible ftr slot x 1066800 w 8814816", box.Offset.X, box.Extent.CX)
	}
	// The band is the visible placeholders' band, inside the slide.
	if box.Offset.Y != 6501384 || box.Offset.Y+box.Extent.CY > slideH {
		t.Errorf("left footer box y = %d h %d, want the visible band at 6501384", box.Offset.Y, box.Extent.CY)
	}
	sn := positions["type:sldNum"]
	if sn == nil {
		t.Fatal("slide number slot lost")
	}
	if right := box.Offset.X + box.Extent.CX; right > sn.Offset.X-footerBoxGap {
		t.Errorf("left footer box right %d runs under the slide number at %d", right, sn.Offset.X)
	}
	// The page number may still grow leftward, but not into the text's
	// minimum width measured from the new anchor.
	if got, want := pageNumberLeftLimit(positions), 1066800+minLeftFooterWidth+footerBoxGap; got != want {
		t.Errorf("page-number left limit = %d, want %d (measured from the ftr anchor)", got, want)
	}
	if ftr := positions["type:ftr"]; ftr == positions["type:dt"] {
		t.Error("dt and ftr must not share one transform: later per-slot edits would move both")
	} else if ftr.Offset.X != 1066800 || ftr.Extent.CX != 8814816 {
		t.Errorf("ftr slot changed: %+v", ftr)
	}
}

// Off-slide is either axis: a date parked beyond the left or right edge is as
// hidden as one below the bottom edge. The right edge needs the slide width.
func TestResolveFooterPositions_DateOffSlideHorizontally(t *testing.T) {
	const slideW, slideH int64 = 12192000, 6858000
	for name, x := range map[string]int64{"right of the slide": slideW, "left of the slide": -2743200} {
		pos := hiddenDateFooterPositions()
		pos["type:dt"].Offset = offsetXML{X: x, Y: 6497441}
		positions := resolveFooterPositionsOnSlide(pos, slideW, slideH)
		if box := leftFooterBox(positions); box == nil || box.Offset.X != 1066800 || box.Extent.CX != 8814816 {
			t.Errorf("dt %s: left footer box = %+v, want the ftr slot", name, box)
		}
	}
	// One EMU of the date on the slide makes it a visible slot again.
	pos := hiddenDateFooterPositions()
	pos["type:dt"].Offset = offsetXML{X: slideW - 1, Y: 6497441}
	if x := resolveFooterPositionsOnSlide(pos, slideW, slideH)["type:dt"].Offset.X; x != slideW-1 {
		t.Errorf("dt x = %d, want its own %d", x, slideW-1)
	}
}

// Only a date that is wholly outside the slide is hidden. One that merely
// overhangs the bottom edge is a visible slot that gets clamped, as before.
func TestResolveFooterPositions_OverhangingDateKeepsItsAnchor(t *testing.T) {
	pos := hiddenDateFooterPositions()
	pos["type:dt"].Offset.Y = 6858000 - 100000 // top edge still on the slide
	positions := resolveFooterPositions(pos, 6858000)
	if x := positions["type:dt"].Offset.X; x != 5265850 {
		t.Errorf("dt x = %d, want its own 5265850", x)
	}
}

// With no visible footer placeholder to fall back on, the hidden date is
// clamped into view as before, so the left text is never dropped.
func TestResolveFooterPositions_OffSlideDateWithoutVisibleFooter(t *testing.T) {
	noFtr := hiddenDateFooterPositions()
	delete(noFtr, "type:ftr")
	positions := resolveFooterPositions(noFtr, 6858000)
	if dt := positions["type:dt"]; dt.Offset.X != 5265850 || dt.Offset.Y+dt.Extent.CY > 6858000 {
		t.Errorf("dt = %+v, want its own x, clamped into the slide", dt)
	}

	bothHidden := hiddenDateFooterPositions()
	bothHidden["type:ftr"].Offset.Y = 7056786
	positions = resolveFooterPositions(bothHidden, 6858000)
	if dt := positions["type:dt"]; dt.Offset.X != 5265850 {
		t.Errorf("dt x = %d, want its own 5265850 when ftr is hidden too", dt.Offset.X)
	}
}

// A visible date keeps today's geometry: text starts at dt and runs across ftr.
func TestResolveFooterPositions_VisibleDateUnchanged(t *testing.T) {
	positions := resolveFooterPositions(masterFooterPositions(), 6858000)
	box := leftFooterBox(positions)
	if box == nil || box.Offset.X != 838200 || box.Extent.CX != 4038600+4114800-838200 {
		t.Errorf("left footer box = %+v, want x 838200 through the ftr right edge", box)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// makeMatrix2x2Cells returns a synthetic set of ResolvedCells representing a
// 2x2 matrix grid spanning 9144000 x 5143500 EMU (16:9 default minus chrome).
// This avoids invoking the full grid resolver; the overlay code only reads
// (RowIdx, ColIdx, Bounds, CellBounds) which is sufficient.
func makeMatrix2x2Cells() []shapegrid.ResolvedCell {
	const (
		x0   = int64(457200)  // 0.5"
		y0   = int64(457200)
		colW = int64(4114800) // 4.5"
		rowH = int64(2286000) // 2.5"
		gap  = int64(91440)   // 0.1"
	)
	cells := make([]shapegrid.ResolvedCell, 0, 4)
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			b := pptx.RectEmu{
				X:  x0 + int64(c)*(colW+gap),
				Y:  y0 + int64(r)*(rowH+gap),
				CX: colW,
				CY: rowH,
			}
			cells = append(cells, shapegrid.ResolvedCell{
				Kind:       shapegrid.CellKindShape,
				Bounds:     b,
				CellBounds: b,
				ID:         uint32(100 + r*2 + c),
				RowIdx:     r,
				ColIdx:     c,
			})
		}
	}
	return cells
}

// TestResolveOverlays_MatrixDiagonalArrows is the acceptance-criteria golden
// test: a 2x2 matrix renders with two diagonal arrow overlays. We assert the
// connector geometry (bounds + flip flags) corresponds to the cell centers.
func TestResolveOverlays_MatrixDiagonalArrows(t *testing.T) {
	cells := makeMatrix2x2Cells()

	overlays := []*OverlayShapeInput{
		// Top-left → Bottom-right diagonal arrow
		{
			Kind:  "arrow",
			Color: "accent2",
			Width: 2.0,
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 0, Col: 0, At: "center"}},
			To:    &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 1, Col: 1, At: "center"}},
		},
		// Top-right → Bottom-left diagonal arrow
		{
			Kind:  "arrow",
			Color: "accent3",
			Width: 2.0,
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 0, Col: 1, At: "center"}},
			To:    &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 1, Col: 0, At: "center"}},
		},
	}

	alloc := newAllocFrom(400)
	frags, err := resolveOverlays(overlays, cells, alloc, 0, 0, nil)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	if len(frags) != 2 {
		t.Fatalf("expected 2 overlay shapes, got %d", len(frags))
	}

	// Expected centers:
	//   c00: (457200 + 4114800/2, 457200 + 2286000/2)        = (2514600, 1600200)
	//   c01: (457200 + 4114800 + 91440 + 4114800/2, 1600200) = (6720840, 1600200)
	//   c10: (2514600, 457200 + 2286000 + 91440 + 2286000/2) = (2514600, 3977640)
	//   c11: (6720840, 3977640)

	// Arrow 1: (c00 → c11). startX<endX and startY<endY, so no flip.
	// Bounds: X=2514600, Y=1600200, CX=6720840-2514600=4206240, CY=3977640-1600200=2377440
	want1 := pptx.RectEmu{X: 2514600, Y: 1600200, CX: 4206240, CY: 2377440}
	assertConnectorBounds(t, frags[0], want1, false, false, true /*arrowhead*/)

	// Arrow 2: (c01 → c10). startX>endX and startY<endY → flipH=true, flipV=false.
	want2 := pptx.RectEmu{X: 2514600, Y: 1600200, CX: 4206240, CY: 2377440}
	assertConnectorBounds(t, frags[1], want2, true, false, true /*arrowhead*/)

	// Both fragments must be p:cxnSp connectors (arrows), not p:sp shapes.
	for i, frag := range frags {
		if !bytes.Contains(frag, []byte("<p:cxnSp>")) {
			t.Errorf("frag %d: expected <p:cxnSp> connector, got: %s", i, string(frag[:min(80, len(frag))]))
		}
		if !bytes.Contains(frag, []byte("straightConnector1")) {
			t.Errorf("frag %d: expected straightConnector1 geometry", i)
		}
		if !bytes.Contains(frag, []byte("<a:tailEnd")) {
			t.Errorf("frag %d: expected tailEnd arrowhead", i)
		}
	}
}

// makeMatrix2x2CellsWithFillsAndText returns a 2x2 set of resolved cells
// whose shape specs carry text labels and theme-resolved fills. Used by the
// overlay-arrow regression tests for center-anchor routing and contrast.
func makeMatrix2x2CellsWithFillsAndText() []shapegrid.ResolvedCell {
	cells := makeMatrix2x2Cells()
	fills := []string{`"accent1"`, `"accent2"`, `"accent3"`, `"accent4"`}
	labels := []string{`"Low cost\nLow risk"`, `"High cost\nLow risk"`, `"Low cost\nHigh risk"`, `"High cost\nHigh risk"`}
	for i := range cells {
		cells[i].ShapeSpec = &shapegrid.ShapeSpec{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fills[i]),
			Text:     json.RawMessage(labels[i]),
		}
	}
	return cells
}

// TestResolveOverlays_CenterArrowsAvoidLabelCenters asserts that arrow
// overlays with center-anchored endpoints get routed to the cell corners
// facing the opposite endpoint when both cells carry text labels. This
// prevents the arrowhead from sitting on top of the quadrant label.
func TestResolveOverlays_CenterArrowsAvoidLabelCenters(t *testing.T) {
	cells := makeMatrix2x2CellsWithFillsAndText()

	overlays := []*OverlayShapeInput{
		// Top-left → Bottom-right diagonal arrow
		{
			Kind:  "arrow",
			Color: "FF0000", // hex avoids contrast flip path
			Width: 2.0,
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 0, Col: 0, At: "center"}},
			To:    &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 1, Col: 1, At: "center"}},
		},
	}

	alloc := newAllocFrom(400)
	frags, err := resolveOverlays(overlays, cells, alloc, 0, 0, nil)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("expected 1 overlay shape, got %d", len(frags))
	}

	// Without routing, the arrow would have spanned c00-center→c11-center =
	// (2514600,1600200)→(6720840,3977640) with cx=4206240, cy=2377440.
	// With routing snapping each endpoint toward the opposite corner with a
	// 10% inset, both off.x/off.y and ext.cx/ext.cy must shrink noticeably.
	rawCX, _ := parseInt(extractAttr(string(frags[0]), "<a:ext ", "cx"))
	rawCY, _ := parseInt(extractAttr(string(frags[0]), "<a:ext ", "cy"))
	if rawCX >= 4206240 {
		t.Errorf("expected routed arrow cx < 4206240, got %d (arrow still spans full diagonal)", rawCX)
	}
	if rawCY >= 2377440 {
		t.Errorf("expected routed arrow cy < 2377440, got %d (arrow still spans full diagonal)", rawCY)
	}
	rawX, _ := parseInt(extractAttr(string(frags[0]), "<a:off ", "x"))
	rawY, _ := parseInt(extractAttr(string(frags[0]), "<a:off ", "y"))
	if rawX <= 2514600 {
		t.Errorf("expected routed arrow off.x > 2514600 (start moved toward bottom-right corner of c00), got %d", rawX)
	}
	if rawY <= 1600200 {
		t.Errorf("expected routed arrow off.y > 1600200 (start moved toward bottom-right corner of c00), got %d", rawY)
	}
}

// TestResolveOverlays_StrokeColorFlipsForContrast asserts that a dark arrow
// stroke on a dark-fill quadrant gets swapped to a light contrast color.
func TestResolveOverlays_StrokeColorFlipsForContrast(t *testing.T) {
	cells := makeMatrix2x2Cells()
	// Give c00 an explicit dark fill via hex (no theme needed). Both
	// endpoints reference c00 / c11 with similar dark fills.
	for i := range cells {
		cells[i].ShapeSpec = &shapegrid.ShapeSpec{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"#1F2A44"`), // dark navy
			Text:     json.RawMessage(`"label"`),
		}
	}

	overlays := []*OverlayShapeInput{
		{
			Kind:  "arrow",
			Color: "000000", // dark stroke — would be invisible on dark fill
			Width: 3.0,
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 0, Col: 0, At: "center"}},
			To:    &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 1, Col: 1, At: "center"}},
		},
	}

	alloc := newAllocFrom(400)
	frags, err := resolveOverlays(overlays, cells, alloc, 0, 0, nil)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("expected 1 overlay shape, got %d", len(frags))
	}
	s := string(frags[0])
	// The stroke is rendered inside <a:ln>...<a:solidFill>...<a:srgbClr val="..."/>.
	// After the contrast flip, the requested 000000 must be replaced with a
	// light color (FFFFFF) so it is visible on the dark fill.
	if strings.Contains(s, `<a:srgbClr val="000000"`) {
		t.Errorf("expected stroke color 000000 to be flipped for contrast against dark fills, got fragment: %s", s)
	}
	if !strings.Contains(s, `<a:srgbClr val="FFFFFF"`) {
		t.Errorf("expected stroke flipped to FFFFFF for visibility on dark fills, got: %s", s)
	}
}

// TestResolveOverlays_StrokeColorFlipUsesTheme verifies the theme palette is
// consulted when an endpoint cell fill references a scheme color.
func TestResolveOverlays_StrokeColorFlipUsesTheme(t *testing.T) {
	cells := makeMatrix2x2Cells()
	for i := range cells {
		cells[i].ShapeSpec = &shapegrid.ShapeSpec{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"accent1"`), // resolved via theme below
			Text:     json.RawMessage(`"label"`),
		}
	}
	theme := []types.ThemeColor{
		{Name: "accent1", RGB: "#0B1F3A"}, // dark navy
	}
	overlays := []*OverlayShapeInput{
		{
			Kind:  "arrow",
			Color: "dk1", // unresolvable without theme → falls back to user color
			Width: 2.0,
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 0, Col: 0, At: "center"}},
			To:    &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 1, Col: 1, At: "center"}},
		},
	}
	theme = append(theme, types.ThemeColor{Name: "dk1", RGB: "#000000"})

	alloc := newAllocFrom(400)
	frags, err := resolveOverlays(overlays, cells, alloc, 0, 0, theme)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	s := string(frags[0])
	if !strings.Contains(s, `<a:srgbClr val="FFFFFF"`) {
		t.Errorf("expected dk1 stroke on dark accent1 fill to flip to FFFFFF, got: %s", s)
	}
}

// parseInt is a strconv-free helper that parses a non-negative base-10 string.
func parseInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n, true
}

// TestResolveOverlays_LinePercent verifies percent-of-slide positioning for a
// plain line (no arrowhead) when no anchor_cell is used.
func TestResolveOverlays_LinePercent(t *testing.T) {
	overlays := []*OverlayShapeInput{
		{
			Kind:  "line",
			Color: "FF0000",
			Width: 1.0,
			From:  &OverlayPointInput{X: 10, Y: 20},
			To:    &OverlayPointInput{X: 90, Y: 80},
		},
	}
	alloc := newAllocFrom(400)
	// Use 16:9 default: 9144000 x 5143500 EMU.
	frags, err := resolveOverlays(overlays, nil, alloc, 9144000, 5143500, nil)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(frags))
	}
	want := pptx.RectEmu{
		X:  int64(float64(9144000) * 0.10),                              // 914400
		Y:  int64(float64(5143500) * 0.20),                              // 1028700
		CX: int64(float64(9144000)*0.90) - int64(float64(9144000)*0.10), // 7315200
		CY: int64(float64(5143500)*0.80) - int64(float64(5143500)*0.20), // 3086100
	}
	assertConnectorBounds(t, frags[0], want, false, false, false /*no arrowhead*/)

	if bytes.Contains(frags[0], []byte("<a:tailEnd")) {
		t.Errorf("plain line should not have arrowhead")
	}
}

// TestResolveOverlays_Badge verifies a badge renders as a roundRect with text.
func TestResolveOverlays_Badge(t *testing.T) {
	overlays := []*OverlayShapeInput{
		{
			Kind:   "badge",
			Color:  "accent1",
			Text:   "EXEC SUMMARY",
			Width:  20.0, // 20% of slide width
			Height: 7.0,  // 7% of slide height
			From:   &OverlayPointInput{X: 40, Y: 2},
		},
	}
	alloc := newAllocFrom(400)
	frags, err := resolveOverlays(overlays, nil, alloc, 9144000, 5143500, nil)
	if err != nil {
		t.Fatalf("resolveOverlays: %v", err)
	}
	if len(frags) != 1 {
		t.Fatalf("expected 1 overlay, got %d", len(frags))
	}
	frag := frags[0]
	if !bytes.Contains(frag, []byte("<p:sp>")) {
		t.Fatalf("expected <p:sp> for badge, got: %s", string(frag[:min(80, len(frag))]))
	}
	if !bytes.Contains(frag, []byte("roundRect")) {
		t.Errorf("expected roundRect geometry")
	}
	if !bytes.Contains(frag, []byte("EXEC SUMMARY")) {
		t.Errorf("expected badge text in fragment")
	}
}

// TestResolveOverlays_AnchorOutOfRangeError verifies a clear error when an
// anchor_cell references a cell that does not exist.
func TestResolveOverlays_AnchorOutOfRangeError(t *testing.T) {
	overlays := []*OverlayShapeInput{
		{
			Kind:  "arrow",
			From:  &OverlayPointInput{AnchorCell: &OverlayAnchorCellInput{Row: 5, Col: 5}},
			To:    &OverlayPointInput{X: 50, Y: 50},
		},
	}
	alloc := newAllocFrom(400)
	_, err := resolveOverlays(overlays, makeMatrix2x2Cells(), alloc, 9144000, 5143500, nil)
	if err == nil {
		t.Fatal("expected error for missing anchor cell")
	}
	if !strings.Contains(err.Error(), "anchor_cell") {
		t.Errorf("expected anchor_cell in error, got: %v", err)
	}
}

// TestResolveOverlays_MissingFieldsErrors verifies clear errors when required
// fields are missing.
func TestResolveOverlays_MissingFieldsErrors(t *testing.T) {
	cases := []struct {
		name      string
		ov        *OverlayShapeInput
		wantMatch string
	}{
		{"empty kind", &OverlayShapeInput{From: &OverlayPointInput{X: 0, Y: 0}}, "kind is required"},
		{"unknown kind", &OverlayShapeInput{Kind: "ribbon"}, "unsupported kind"},
		{"arrow no from", &OverlayShapeInput{Kind: "arrow", To: &OverlayPointInput{X: 50, Y: 50}}, "requires 'from'"},
		{"arrow no to", &OverlayShapeInput{Kind: "arrow", From: &OverlayPointInput{X: 0, Y: 0}}, "requires 'to'"},
		{"badge no from", &OverlayShapeInput{Kind: "badge"}, "requires 'from'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := newAllocFrom(400)
			_, err := resolveOverlays([]*OverlayShapeInput{tc.ov}, nil, alloc, 9144000, 5143500, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Errorf("expected error to contain %q, got: %v", tc.wantMatch, err)
			}
		})
	}
}

// TestPointOnRect_AnchorNames verifies each anchor name maps to the correct
// corner / edge midpoint of a known rectangle.
func TestPointOnRect_AnchorNames(t *testing.T) {
	r := pptx.RectEmu{X: 100, Y: 200, CX: 400, CY: 300}
	// Center: (300, 350), TL: (100,200), TR: (500,200), BL: (100,500), BR: (500,500)
	cases := []struct {
		at     string
		wantX  int64
		wantY  int64
	}{
		{"center", 300, 350},
		{"", 300, 350},
		{"top-left", 100, 200},
		{"top", 300, 200},
		{"top-right", 500, 200},
		{"right", 500, 350},
		{"bottom-right", 500, 500},
		{"bottom", 300, 500},
		{"bottom-left", 100, 500},
		{"left", 100, 350},
		{"BR", 500, 500}, // case-insensitive
		{"unknown-fallback", 300, 350},
	}
	for _, tc := range cases {
		t.Run(tc.at, func(t *testing.T) {
			x, y := pointOnRect(r, tc.at)
			if x != tc.wantX || y != tc.wantY {
				t.Errorf("at=%q: got (%d,%d), want (%d,%d)", tc.at, x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

// assertConnectorBounds parses connector XML and asserts the off/ext geometry
// and flip flags match expectations.
func assertConnectorBounds(t *testing.T, frag []byte, want pptx.RectEmu, wantFlipH, wantFlipV, wantArrowhead bool) {
	t.Helper()
	s := string(frag)

	// Look for <a:off x="X" y="Y"/> and <a:ext cx="CX" cy="CY"/>.
	if got := extractAttr(s, "<a:off ", "x"); got != toStr(want.X) {
		t.Errorf("off.x: got %s, want %d", got, want.X)
	}
	if got := extractAttr(s, "<a:off ", "y"); got != toStr(want.Y) {
		t.Errorf("off.y: got %s, want %d", got, want.Y)
	}
	if got := extractAttr(s, "<a:ext ", "cx"); got != toStr(want.CX) {
		t.Errorf("ext.cx: got %s, want %d", got, want.CX)
	}
	if got := extractAttr(s, "<a:ext ", "cy"); got != toStr(want.CY) {
		t.Errorf("ext.cy: got %s, want %d", got, want.CY)
	}

	hasFlipH := strings.Contains(s, `flipH="1"`)
	if hasFlipH != wantFlipH {
		t.Errorf("flipH: got %v, want %v", hasFlipH, wantFlipH)
	}
	hasFlipV := strings.Contains(s, `flipV="1"`)
	if hasFlipV != wantFlipV {
		t.Errorf("flipV: got %v, want %v", hasFlipV, wantFlipV)
	}
	hasArrowhead := strings.Contains(s, `<a:tailEnd`)
	if hasArrowhead != wantArrowhead {
		t.Errorf("arrowhead: got %v, want %v", hasArrowhead, wantArrowhead)
	}
}

// extractAttr returns the value of attribute `attr` inside the first element
// starting with `prefix` in `s`. Returns "" if not found.
func extractAttr(s, prefix, attr string) string {
	i := strings.Index(s, prefix)
	if i < 0 {
		return ""
	}
	rest := s[i+len(prefix):]
	end := strings.Index(rest, "/>")
	if end < 0 {
		end = strings.Index(rest, ">")
	}
	if end < 0 {
		return ""
	}
	tag := rest[:end]
	needle := attr + `="`
	j := strings.Index(tag, needle)
	if j < 0 {
		return ""
	}
	val := tag[j+len(needle):]
	k := strings.Index(val, `"`)
	if k < 0 {
		return ""
	}
	return val[:k]
}

func toStr(n int64) string {
	// strconv-free formatter to keep imports small.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

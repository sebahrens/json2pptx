package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-kkc5t: overlay endpoints can target a point on an image
// cell's source picture, resolved through the transform that places it.

const screenshotPath = "../../examples/images/ops-console.png" // 1600x900

// imageCell returns a resolved image cell [0,0] with the given picture frame.
func imageCell(path, fit string, frame pptx.RectEmu) shapegrid.ResolvedCell {
	return shapegrid.ResolvedCell{
		Kind:       shapegrid.CellKindImage,
		Bounds:     frame,
		CellBounds: frame,
		ImageSpec:  &shapegrid.ImageSpec{Path: path, Fit: fit},
	}
}

// connectorEnd returns the drawn end point (the `to` endpoint) of a connector
// fragment, honouring flipH / flipV.
func connectorEnd(t *testing.T, frag []byte) (int64, int64) {
	t.Helper()
	s := string(frag)
	x, _ := strconv.ParseInt(extractAttr(s, "<a:off ", "x"), 10, 64)
	y, _ := strconv.ParseInt(extractAttr(s, "<a:off ", "y"), 10, 64)
	cx, _ := strconv.ParseInt(extractAttr(s, "<a:ext ", "cx"), 10, 64)
	cy, _ := strconv.ParseInt(extractAttr(s, "<a:ext ", "cy"), 10, 64)
	if !strings.Contains(s, `flipH="1"`) {
		x += cx
	}
	if !strings.Contains(s, `flipV="1"`) {
		y += cy
	}
	return x, y
}

func near(a, b, tol int64) bool { return a-b <= tol && b-a <= tol }

// TestAnchorImage_FollowsFrameAndCrop: the same source point resolves through
// the picture's own placement in a square cover frame, a wide cover frame and
// a contain frame, in fraction and pixel units alike.
func TestAnchorImage_FollowsFrameAndCrop(t *testing.T) {
	frames := map[string]struct {
		fit   string
		frame pptx.RectEmu
	}{
		"square-cover": {"", pptx.RectEmu{X: 500000, Y: 1000000, CX: 4000000, CY: 4000000}},
		"wide-cover":   {"cover", pptx.RectEmu{X: 500000, Y: 1000000, CX: 7000000, CY: 3000000}},
		"contain":      {"contain", pptx.RectEmu{X: 500000, Y: 1000000, CX: 4000000, CY: 4000000}},
	}
	for name, tc := range frames {
		t.Run(name, func(t *testing.T) {
			cells := []shapegrid.ResolvedCell{imageCell(screenshotPath, tc.fit, tc.frame)}
			place := generator.GridImagePlacement(screenshotPath, types.BoundingBox{X: tc.frame.X, Y: tc.frame.Y, Width: tc.frame.CX, Height: tc.frame.CY}, tc.fit)
			wantX, wantY, ok := place.SourceToSlide(0.35, 0.5378)
			if !ok {
				t.Fatal("target should be visible")
			}
			for _, to := range []*OverlayAnchorImageInput{
				{X: 0.35, Y: 0.5378},
				{X: 560, Y: 484.02, Units: "px"},
			} {
				frags, findings, err := resolveOverlays([]*OverlayShapeInput{{
					Kind: "arrow",
					From: &OverlayPointInput{X: 90, Y: 10},
					To:   &OverlayPointInput{AnchorImage: to},
				}}, cells, newAllocFrom(400), 0, 0, overlayEnv{})
				if err != nil || len(findings) != 0 || len(frags) != 1 {
					t.Fatalf("frags=%d findings=%v err=%v", len(frags), findings, err)
				}
				if x, y := connectorEnd(t, frags[0]); !near(x, wantX, 2) || !near(y, wantY, 2) {
					t.Errorf("units %q: arrow ends at (%d,%d), want (%d,%d)", to.Units, x, y, wantX, wantY)
				}
			}
		})
	}
}

// TestAnchorImage_OffCropCalloutKeepsLabel: a target the square cover crop
// trims away reports OVERLAY_TARGET_CROPPED, keeps the label wording, and
// draws no leader; an arrow to the same target is omitted.
func TestAnchorImage_OffCropCalloutKeepsLabel(t *testing.T) {
	cells := []shapegrid.ResolvedCell{imageCell(screenshotPath, "", pptx.RectEmu{X: 500000, Y: 1000000, CX: 4000000, CY: 4000000})}
	target := &OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 0.08, Y: 0.2}} // left nav: cropped away
	frags, findings, err := resolveOverlays([]*OverlayShapeInput{
		{Kind: "callout", Text: "3  Queues menu", From: &OverlayPointInput{X: 60, Y: 20}, To: target},
		{Kind: "arrow", From: &OverlayPointInput{X: 60, Y: 40}, To: target},
	}, cells, newAllocFrom(400), 0, 0, overlayEnv{SlideIdx: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || !strings.Contains(string(frags[0]), "3  Queues menu") || strings.Contains(string(frags[0]), "<p:cxnSp>") {
		t.Fatalf("want only the callout label (wording kept), got %d fragments", len(frags))
	}
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %+v", findings)
	}
	f := findings[0]
	if f.Code != patterns.ErrCodeOverlayTargetCropped || f.Path != "/slides/4/overlays/0/to/anchor_image" || f.Action != "review" {
		t.Errorf("finding = %+v", f)
	}
	if !strings.Contains(f.Message, "0.219") || !strings.Contains(f.Message, "leader was omitted") {
		t.Errorf("message should give the visible interval and consequence: %s", f.Message)
	}
	if findings[1].Path != "/slides/4/overlays/1/to/anchor_image" {
		t.Errorf("arrow finding path = %s", findings[1].Path)
	}
}

// TestCallout_LeaderStartsOnLabelEdge: the leader leaves the label at its
// boundary (never through its text) and ends on the target dot.
func TestCallout_LeaderStartsOnLabelEdge(t *testing.T) {
	cells := []shapegrid.ResolvedCell{imageCell(screenshotPath, "cover", pptx.RectEmu{X: 500000, Y: 1000000, CX: 7000000, CY: 3937500})}
	frags, _, err := resolveOverlays([]*OverlayShapeInput{{
		Kind: "callout", Text: "1  Delayed queue",
		From: &OverlayPointInput{X: 75, Y: 20},
		To:   &OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 560, Y: 484, Units: "px"}},
	}}, cells, newAllocFrom(400), 12192000, 6858000, overlayEnv{})
	if err != nil || len(frags) != 2 {
		t.Fatalf("frags=%d err=%v", len(frags), err)
	}
	label := string(frags[0])
	lx, _ := strconv.ParseInt(extractAttr(label, "<a:off ", "x"), 10, 64)
	ly, _ := strconv.ParseInt(extractAttr(label, "<a:off ", "y"), 10, 64)
	lcx, _ := strconv.ParseInt(extractAttr(label, "<a:ext ", "cx"), 10, 64)
	lcy, _ := strconv.ParseInt(extractAttr(label, "<a:ext ", "cy"), 10, 64)
	if !strings.Contains(label, `sz="1200"`) || !strings.Contains(label, "1  Delayed queue") {
		t.Errorf("label should be 12pt native text with the authored wording")
	}
	leader := string(frags[1])
	if !strings.Contains(leader, `<a:tailEnd type="oval"`) {
		t.Errorf("leader should end in a target dot: %s", leader)
	}
	// The start is the end opposite connectorEnd.
	sx, _ := strconv.ParseInt(extractAttr(leader, "<a:off ", "x"), 10, 64)
	sy, _ := strconv.ParseInt(extractAttr(leader, "<a:off ", "y"), 10, 64)
	cx, _ := strconv.ParseInt(extractAttr(leader, "<a:ext ", "cx"), 10, 64)
	cy, _ := strconv.ParseInt(extractAttr(leader, "<a:ext ", "cy"), 10, 64)
	if strings.Contains(leader, `flipH="1"`) {
		sx += cx
	}
	if strings.Contains(leader, `flipV="1"`) {
		sy += cy
	}
	onEdge := (near(sx, lx, 1) || near(sx, lx+lcx, 1)) && sy >= ly-1 && sy <= ly+lcy+1 ||
		(near(sy, ly, 1) || near(sy, ly+lcy, 1)) && sx >= lx-1 && sx <= lx+lcx+1
	if !onEdge {
		t.Errorf("leader starts at (%d,%d), not on the label edge %d,%d %dx%d", sx, sy, lx, ly, lcx, lcy)
	}
}

// fragRect reads a shape fragment's bounds.
func fragRect(frag []byte) pptx.RectEmu {
	s := string(frag)
	x, _ := strconv.ParseInt(extractAttr(s, "<a:off ", "x"), 10, 64)
	y, _ := strconv.ParseInt(extractAttr(s, "<a:off ", "y"), 10, 64)
	cx, _ := strconv.ParseInt(extractAttr(s, "<a:ext ", "cx"), 10, 64)
	cy, _ := strconv.ParseInt(extractAttr(s, "<a:ext ", "cy"), 10, 64)
	return pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy}
}

// TestCallout_AutoPlacedLabel (go-slide-creator-n3j96): a callout with no
// `from` and an anchor_image target places its own label — inside the
// picture's frame, clear of every callout target and of the other labels —
// and still draws the leader to the target. Without an anchor_image target
// `from` stays required.
func TestCallout_AutoPlacedLabel(t *testing.T) {
	frame := pptx.RectEmu{X: 500000, Y: 1000000, CX: 5000000, CY: 2812500}
	cells := []shapegrid.ResolvedCell{imageCell(screenshotPath, "contain", frame)}
	points := [][2]float64{{0.35, 0.54}, {0.62, 0.54}, {0.97, 0.05}}
	overlays := make([]*OverlayShapeInput, 0, len(points))
	for i, p := range points {
		overlays = append(overlays, &OverlayShapeInput{Kind: "callout", Text: fmt.Sprintf("%d  Label", i+1),
			To: &OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: p[0], Y: p[1]}}})
	}
	frags, findings, err := resolveOverlays(overlays, cells, newAllocFrom(400), 12192000, 6858000, overlayEnv{})
	if err != nil || len(findings) != 0 || len(frags) != 2*len(points) {
		t.Fatalf("frags=%d findings=%v err=%v", len(frags), findings, err)
	}
	place := generator.GridImagePlacement(screenshotPath, types.BoundingBox{X: frame.X, Y: frame.Y, Width: frame.CX, Height: frame.CY}, "contain")
	var labels []pptx.RectEmu
	for i, p := range points {
		label := fragRect(frags[2*i])
		if label.X < frame.X || label.Y < frame.Y || label.X+label.CX > frame.X+frame.CX || label.Y+label.CY > frame.Y+frame.CY {
			t.Errorf("label %d %+v leaves the picture frame %+v", i, label, frame)
		}
		for j, q := range points {
			if tx, ty, _ := place.SourceToSlide(q[0], q[1]); pointInRect(tx, ty, label) {
				t.Errorf("label %d covers the target of callout %d", i, j)
			}
		}
		for j, other := range labels {
			if label.X < other.X+other.CX && other.X < label.X+label.CX && label.Y < other.Y+other.CY && other.Y < label.Y+label.CY {
				t.Errorf("label %d overlaps label %d", i, j)
			}
		}
		labels = append(labels, label)
		wantX, wantY, _ := place.SourceToSlide(p[0], p[1])
		if x, y := connectorEnd(t, frags[2*i+1]); !near(x, wantX, 2) || !near(y, wantY, 2) {
			t.Errorf("leader %d ends at (%d,%d), want (%d,%d)", i, x, y, wantX, wantY)
		}
	}

	_, _, err = resolveOverlays([]*OverlayShapeInput{{Kind: "callout", Text: "x", To: &OverlayPointInput{X: 50, Y: 50}}},
		cells, newAllocFrom(400), 0, 0, overlayEnv{})
	if err == nil || !strings.Contains(err.Error(), "requires 'from'") {
		t.Errorf("a callout to a slide point still needs from: %v", err)
	}
}

// TestAnchorImage_PictureInsideNestedGrid: a pattern that stacks a caption
// under its picture nests the picture one grid down. anchor_image addressing
// the hosting cell resolves to the one picture inside it; a host with two
// pictures stays an error.
func TestAnchorImage_PictureInsideNestedGrid(t *testing.T) {
	host := shapegrid.ResolvedCell{Kind: shapegrid.CellKindSubGrid, Bounds: pptx.RectEmu{X: 400000, Y: 900000, CX: 4200000, CY: 3600000}}
	picture := imageCell(screenshotPath, "contain", pptx.RectEmu{X: 500000, Y: 1000000, CX: 4000000, CY: 2250000})
	caption := shapegrid.ResolvedCell{Kind: shapegrid.CellKindShape, RowIdx: 1, Bounds: pptx.RectEmu{X: 500000, Y: 3400000, CX: 4000000, CY: 400000}}
	overlay := []*OverlayShapeInput{{Kind: "arrow", From: &OverlayPointInput{X: 90, Y: 10},
		To: &OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 0.5, Y: 0.5}}}}

	frags, _, err := resolveOverlays(overlay, []shapegrid.ResolvedCell{host, picture, caption}, newAllocFrom(400), 0, 0, overlayEnv{})
	if err != nil || len(frags) != 1 {
		t.Fatalf("frags=%d err=%v", len(frags), err)
	}
	if x, y := connectorEnd(t, frags[0]); !near(x, 2500000, 2) || !near(y, 2125000, 2) {
		t.Errorf("arrow ends at (%d,%d), want the nested picture's centre (2500000,2125000)", x, y)
	}

	second := imageCell(screenshotPath, "contain", pptx.RectEmu{X: 500000, Y: 3400000, CX: 4000000, CY: 1000000})
	if _, _, err := resolveOverlays(overlay, []shapegrid.ResolvedCell{host, picture, second}, newAllocFrom(400), 0, 0, overlayEnv{}); err == nil || !strings.Contains(err.Error(), "not an image cell") {
		t.Errorf("a host with two pictures is ambiguous: err = %v", err)
	}
}

// TestAnchorImage_Errors covers authoring mistakes that cannot render.
func TestAnchorImage_Errors(t *testing.T) {
	frame := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 1000000}
	shapeCell := shapegrid.ResolvedCell{Kind: shapegrid.CellKindShape, Bounds: frame, RowIdx: 0, ColIdx: 1}
	svgCell := imageCell("../../examples/images/missing.svg", "", frame)
	svgCell.ColIdx = 2
	cells := []shapegrid.ResolvedCell{imageCell(screenshotPath, "", frame), shapeCell, svgCell}
	cases := map[string]struct {
		pt   OverlayPointInput
		want string
	}{
		"not an image":  {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{Col: 1, X: 0.5, Y: 0.5}}, "not an image cell"},
		"missing cell":  {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{Row: 3, X: 0.5, Y: 0.5}}, "not found"},
		"fraction > 1":  {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 1.5, Y: 0.5}}, "outside the image"},
		"px too large":  {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 1700, Y: 10, Units: "px"}}, "outside the 1600x900 image"},
		"bad units":     {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 1, Y: 1, Units: "pt"}}, "units"},
		"both anchors":  {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 0.5, Y: 0.5}, AnchorCell: &OverlayAnchorCellInput{}}, "not both"},
		"px unreadable": {OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{Col: 2, X: 10, Y: 10, Units: "px"}}, "pixel size"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pt := tc.pt
			_, _, err := resolveOverlays([]*OverlayShapeInput{{Kind: "line", From: &OverlayPointInput{X: 1, Y: 1}, To: &pt}},
				cells, newAllocFrom(400), 0, 0, overlayEnv{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
	if _, _, err := resolveOverlays([]*OverlayShapeInput{{Kind: "callout", From: &OverlayPointInput{}, To: &OverlayPointInput{X: 50, Y: 50}}},
		nil, newAllocFrom(400), 0, 0, overlayEnv{}); err == nil || !strings.Contains(err.Error(), "requires 'text'") {
		t.Errorf("callout without text: err = %v", err)
	}
}

var (
	picChunkRE  = regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`)
	cxnChunkRE  = regexp.MustCompile(`(?s)<p:cxnSp>.*?</p:cxnSp>`)
	srcRectAttr = regexp.MustCompile(`<a:srcRect([^/]*)/>`)
)

// TestExhibitCalloutsDeck_LeadersLandOnPictureTargets renders the shipped
// example on tracked templates (and the local p-style when present) and checks
// each callout leader ends where its source pixel lands in the picture XML as
// written — offset, extent and a:srcRect — independently of the resolver.
func TestExhibitCalloutsDeck_LeadersLandOnPictureTargets(t *testing.T) {
	templates := []string{"midnight-blue", "warm-coral"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	targets := [][2]float64{{560.0 / 1600, 484.0 / 900}, {1182.0 / 1600, 484.0 / 900}}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			if err := runJSONMode("../../examples/exhibit-callouts.json", filepath.Join(dir, "result.json"), "../../templates", dir,
				"", false, false, tpl, "off", false, "off", "", false); err != nil {
				t.Fatalf("generate: %v", err)
			}
			parts := pptxParts(t, filepath.Join(dir, "exhibit-callouts.pptx"))
			for n := 1; n <= 3; n++ {
				xml := string(parts[fmt.Sprintf("ppt/slides/slide%d.xml", n)])
				var pic string
				for _, c := range picChunkRE.FindAllString(xml, -1) {
					if strings.Contains(c, `descr="OpsConsole`) {
						pic = c
					}
				}
				if pic == "" {
					t.Fatalf("slide %d: screenshot picture missing", n)
				}
				px, _ := strconv.ParseFloat(extractAttr(pic, "<a:off ", "x"), 64)
				py, _ := strconv.ParseFloat(extractAttr(pic, "<a:off ", "y"), 64)
				pcx, _ := strconv.ParseFloat(extractAttr(pic, "<a:ext ", "cx"), 64)
				pcy, _ := strconv.ParseFloat(extractAttr(pic, "<a:ext ", "cy"), 64)
				crop := map[string]float64{}
				if m := srcRectAttr.FindStringSubmatch(pic); m != nil {
					for _, side := range []string{"l", "t", "r", "b"} {
						if v := extractAttr("<x "+m[1]+"/>", "<x ", side); v != "" {
							f, _ := strconv.ParseFloat(v, 64)
							crop[side] = f / 100000
						}
					}
				}
				if n == 2 && crop["l"] < 0.2 {
					t.Errorf("slide 2 is the square cover frame; want a heavy left/right crop, got %v", crop)
				}
				var leaders []string
				for _, c := range cxnChunkRE.FindAllString(xml, -1) {
					if strings.Contains(c, "Overlay callout leader") {
						leaders = append(leaders, c)
					}
				}
				if len(leaders) != 2 {
					t.Fatalf("slide %d: want 2 callout leaders, got %d", n, len(leaders))
				}
				for i, tg := range targets {
					wantX := px + (tg[0]-crop["l"])/(1-crop["l"]-crop["r"])*pcx
					wantY := py + (tg[1]-crop["t"])/(1-crop["t"]-crop["b"])*pcy
					x, y := connectorEnd(t, []byte(leaders[i]))
					if !near(x, int64(wantX), 200) || !near(y, int64(wantY), 200) {
						t.Errorf("slide %d leader %d ends at (%d,%d); its pixel renders at (%.0f,%.0f)", n, i+1, x, y, wantX, wantY)
					}
				}
				for _, label := range []string{"1  Delayed queue", "2  Status badge: DELAYED"} {
					if !strings.Contains(xml, label) {
						t.Errorf("slide %d: callout label %q missing", n, label)
					}
				}
			}
		})
	}
}

// writeAnchorTestPNG writes a blank w×h PNG.
func writeAnchorTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shot.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAnchorImage_PixelUnitsUseIntrinsicSize: px units divide by the file's
// own pixel size, not the frame's.
func TestAnchorImage_PixelUnitsUseIntrinsicSize(t *testing.T) {
	path := writeAnchorTestPNG(t, 400, 200)
	frame := pptx.RectEmu{X: 0, Y: 0, CX: 4000000, CY: 2000000}
	frags, _, err := resolveOverlays([]*OverlayShapeInput{{
		Kind: "line", From: &OverlayPointInput{X: 90, Y: 90},
		To: &OverlayPointInput{AnchorImage: &OverlayAnchorImageInput{X: 100, Y: 50, Units: "px"}},
	}}, []shapegrid.ResolvedCell{imageCell(path, "cover", frame)}, newAllocFrom(400), 0, 0, overlayEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if x, y := connectorEnd(t, frags[0]); x != 1000000 || y != 500000 {
		t.Errorf("(100,50)px of 400x200 in a matching frame -> (%d,%d), want (1000000,500000)", x, y)
	}
}

package template

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// A bundled template's body placeholders stay clear of its diagonal
// decorative lines (go-slide-creator-bvv3v).
//
// The content zone of a shape grid, a chart, a table and the native body text
// all follow the layout's body placeholder. abstract's One Content layout
// draws a diagonal line from the top right corner down to x = 814.6pt at the
// bottom edge; when its body was widened to the master's 66–894pt column the
// right-hand 66pt of every wide pattern ran under that line. A placeholder is
// the template's statement of where content goes, so the fix is in the
// placeholder — the body ends at 816pt, 10pt left of the line at the foot of
// the content area — and this test holds every bundled layout to it.
//
// Axis-aligned rules are separators that sit on a placeholder's edge by
// design and are not checked; filled artwork is covered by the chrome frame's
// own side-decoration rule (excludeSideDecor).

// decorLineClearancePt is the least distance between a diagonal decorative
// line and a body placeholder.
const decorLineClearancePt = 8.0

const emuPerPoint = 12700.0

type clearanceXfrm struct {
	FlipH bool `xml:"flipH,attr"`
	FlipV bool `xml:"flipV,attr"`
	Rot   int  `xml:"rot,attr"`
	Off   *struct {
		X float64 `xml:"x,attr"`
		Y float64 `xml:"y,attr"`
	} `xml:"off"`
	Ext *struct {
		CX float64 `xml:"cx,attr"`
		CY float64 `xml:"cy,attr"`
	} `xml:"ext"`
	ChOff *struct {
		X float64 `xml:"x,attr"`
		Y float64 `xml:"y,attr"`
	} `xml:"chOff"`
	ChExt *struct {
		CX float64 `xml:"cx,attr"`
		CY float64 `xml:"cy,attr"`
	} `xml:"chExt"`
}

type clearanceShape struct {
	Name struct {
		Name string `xml:"name,attr"`
	} `xml:"nvSpPr>cNvPr"`
	CxnName struct {
		Name string `xml:"name,attr"`
	} `xml:"nvCxnSpPr>cNvPr"`
	PH *struct {
		Type string `xml:"type,attr"`
	} `xml:"nvSpPr>nvPr>ph"`
	Xfrm *clearanceXfrm `xml:"spPr>xfrm"`
	Geom *struct {
		Prst string `xml:"prst,attr"`
	} `xml:"spPr>prstGeom"`
}

type clearanceGroup struct {
	Xfrm       *clearanceXfrm   `xml:"grpSpPr>xfrm"`
	Shapes     []clearanceShape `xml:"sp"`
	Connectors []clearanceShape `xml:"cxnSp"`
}

type clearanceLayout struct {
	CSld struct {
		Name       string           `xml:"name,attr"`
		Shapes     []clearanceShape `xml:"spTree>sp"`
		Connectors []clearanceShape `xml:"spTree>cxnSp"`
		Groups     []clearanceGroup `xml:"spTree>grpSp"`
	} `xml:"cSld"`
}

// clearanceSegment is a straight line in slide points.
type clearanceSegment struct {
	name           string
	x0, y0, x1, y1 float64
}

// segment returns the line a straight connector (or a "line" shape) draws, in
// the coordinates its xfrm is given in. A half turn maps a segment onto
// itself; any other rotation is not a case the bundled templates have.
func (s clearanceShape) segment() (clearanceSegment, bool) {
	if s.PH != nil || s.Xfrm == nil || s.Xfrm.Off == nil || s.Xfrm.Ext == nil || s.Geom == nil {
		return clearanceSegment{}, false
	}
	if p := s.Geom.Prst; p != "line" && p != "straightConnector1" {
		return clearanceSegment{}, false
	}
	if r := s.Xfrm.Rot % 10800000; r != 0 {
		return clearanceSegment{}, false
	}
	x0, y0 := s.Xfrm.Off.X, s.Xfrm.Off.Y
	x1, y1 := x0+s.Xfrm.Ext.CX, y0+s.Xfrm.Ext.CY
	if s.Xfrm.FlipH != s.Xfrm.FlipV {
		x0, x1 = x1, x0
	}
	name := s.Name.Name
	if name == "" {
		name = s.CxnName.Name
	}
	return clearanceSegment{name: name, x0: x0, y0: y0, x1: x1, y1: y1}, true
}

func (l clearanceLayout) diagonals() []clearanceSegment {
	var out []clearanceSegment
	add := func(shapes []clearanceShape, toSlide func(x, y float64) (float64, float64)) {
		for _, s := range shapes {
			seg, ok := s.segment()
			if !ok {
				continue
			}
			seg.x0, seg.y0 = toSlide(seg.x0, seg.y0)
			seg.x1, seg.y1 = toSlide(seg.x1, seg.y1)
			seg.x0, seg.y0, seg.x1, seg.y1 = seg.x0/emuPerPoint, seg.y0/emuPerPoint, seg.x1/emuPerPoint, seg.y1/emuPerPoint
			if abs(seg.x1-seg.x0) < 1 || abs(seg.y1-seg.y0) < 1 {
				continue // a rule along an edge, not a diagonal
			}
			out = append(out, seg)
		}
	}
	identity := func(x, y float64) (float64, float64) { return x, y }
	add(l.CSld.Shapes, identity)
	add(l.CSld.Connectors, identity)
	for _, g := range l.CSld.Groups {
		x := g.Xfrm
		if x == nil || x.Off == nil || x.Ext == nil || x.ChOff == nil || x.ChExt == nil || x.ChExt.CX == 0 || x.ChExt.CY == 0 || x.FlipH || x.FlipV || x.Rot != 0 {
			continue
		}
		toSlide := func(cx, cy float64) (float64, float64) {
			return x.Off.X + (cx-x.ChOff.X)*x.Ext.CX/x.ChExt.CX, x.Off.Y + (cy-x.ChOff.Y)*x.Ext.CY/x.ChExt.CY
		}
		add(g.Shapes, toSlide)
		add(g.Connectors, toSlide)
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// crosses reports whether the segment enters the rectangle (Liang–Barsky).
func (s clearanceSegment) crosses(left, top, right, bottom float64) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := s.x1-s.x0, s.y1-s.y0
	for _, c := range [4][2]float64{{-dx, s.x0 - left}, {dx, right - s.x0}, {-dy, s.y0 - top}, {dy, bottom - s.y0}} {
		p, q := c[0], c[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		t := q / p
		if p < 0 {
			if t > t1 {
				return false
			}
			if t > t0 {
				t0 = t
			}
		} else {
			if t < t0 {
				return false
			}
			if t < t1 {
				t1 = t
			}
		}
	}
	return t0 < t1
}

func readClearanceLayouts(t *testing.T, path string) []clearanceLayout {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	var out []clearanceLayout
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slideLayouts/slideLayout") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		var l clearanceLayout
		if err := xml.Unmarshal(data, &l); err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		out = append(out, l)
	}
	return out
}

func TestBodyPlaceholdersClearDiagonalDecorLines(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(testutil.TemplatesDir(), "*.pptx"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no bundled templates: %v", err)
	}
	diagonals := 0
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".pptx")
		for _, l := range readClearanceLayouts(t, path) {
			lines := l.diagonals()
			diagonals += len(lines)
			for _, s := range l.CSld.Shapes {
				if s.PH == nil || s.Xfrm == nil || s.Xfrm.Off == nil || s.Xfrm.Ext == nil {
					continue
				}
				// A body or generic content placeholder: where text, charts,
				// tables and shape grids are written.
				if typ := s.PH.Type; typ != "body" && typ != "" && typ != "obj" && typ != "tbl" && typ != "chart" {
					continue
				}
				left, top := s.Xfrm.Off.X/emuPerPoint, s.Xfrm.Off.Y/emuPerPoint
				right, bottom := left+s.Xfrm.Ext.CX/emuPerPoint, top+s.Xfrm.Ext.CY/emuPerPoint
				for _, line := range lines {
					if line.crosses(left-decorLineClearancePt, top-decorLineClearancePt, right+decorLineClearancePt, bottom+decorLineClearancePt) {
						t.Errorf("%s layout %q: body placeholder %q (x %.0f–%.0f, y %.0f–%.0fpt) is within %.0fpt of the diagonal decorative line %q (%.0f,%.0f → %.0f,%.0fpt): content written to the placeholder's edge crosses the line — end the placeholder short of it",
							name, l.CSld.Name, s.Name.Name, left, right, top, bottom, decorLineClearancePt, line.name, line.x0, line.y0, line.x1, line.y1)
					}
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join(testutil.TemplatesDir(), "abstract.pptx")); err == nil && diagonals == 0 {
		t.Error("no diagonal decorative line was read from any template; abstract's One Content layout has two")
	}
}

// The clip that the check rests on.
func TestClearanceSegmentCrosses(t *testing.T) {
	diag := clearanceSegment{x0: 960, y0: -2, x1: 814.6, y1: 540}
	for _, tc := range []struct {
		right float64
		want  bool
	}{{894, true}, {829.5, true}, {816, false}, {791, false}} {
		if got := diag.crosses(66, 143.8, tc.right, 486.4); got != tc.want {
			t.Errorf("body ending at %.1fpt: crosses = %v, want %v", tc.right, got, tc.want)
		}
	}
	if (clearanceSegment{x0: 0, y0: 100, x1: 50, y1: 100}).crosses(66, 90, 800, 400) {
		t.Error("a segment left of the rectangle is reported as crossing it")
	}
}

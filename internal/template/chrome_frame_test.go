package template

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestChromeFrameGoldenGeometryBundledTemplates is the golden geometry check
// for go-slide-creator-7m9v: on every bundled mktemplate-family template and
// modern-template, the takeaway band starts at the body placeholder's x (±1
// EMU), spans the body column, and never overlaps the footer placeholders.
func TestChromeFrameGoldenGeometryBundledTemplates(t *testing.T) {
	for _, name := range []string{"midnight-blue", "warm-coral", "forest-green", "modern-template"} {
		t.Run(name, func(t *testing.T) {
			r, err := OpenTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
			if err != nil {
				t.Fatalf("OpenTemplate: %v", err)
			}
			defer func() { _ = r.Close() }()
			p, err := BuildProfile(r)
			if err != nil {
				t.Fatalf("BuildProfile: %v", err)
			}
			if len(p.Geometry) != len(p.Layouts) {
				t.Fatalf("geometry entries = %d, want one per layout (%d)", len(p.Geometry), len(p.Layouts))
			}
			ref := p.ReferenceLayout()
			if ref == nil {
				t.Fatal("no One Content reference layout")
			}
			body := findBody(ref)
			if body == nil {
				t.Fatalf("reference layout %s has no body placeholder", ref.ID)
			}
			frame := p.ChromeFrame(ref.ID, true, true)
			if !frame.Fits || frame.Basis != ChromeBasisLayout {
				t.Fatalf("frame fits=%v basis=%q", frame.Fits, frame.Basis)
			}
			if d := frame.Takeaway.X - body.Bounds.X; d < -1 || d > 1 {
				t.Errorf("takeaway x = %d, want body x %d (±1)", frame.Takeaway.X, body.Bounds.X)
			}
			if d := frame.Takeaway.X + frame.Takeaway.CX - (body.Bounds.X + body.Bounds.Width); d < -1 || d > 1 {
				t.Errorf("takeaway right = %d, want body right %d", frame.Takeaway.X+frame.Takeaway.CX, body.Bounds.X+body.Bounds.Width)
			}
			if len(ref.FooterRegions) == 0 {
				t.Fatal("expected resolved footer regions from the master")
			}
			for _, band := range []ChromeRect{frame.Takeaway, frame.Source} {
				for _, fr := range ref.FooterRegions {
					if rectsOverlap(band, ChromeRect{X: fr.X, Y: fr.Y, CX: fr.Width, CY: fr.Height}) {
						t.Errorf("band %+v overlaps %s footer %+v", band, fr.Type, fr)
					}
				}
			}
			if frame.Takeaway.Bottom() > frame.Source.Y {
				t.Errorf("takeaway %+v overlaps source %+v", frame.Takeaway, frame.Source)
			}
			if frame.Content.Bottom() > frame.Takeaway.Y {
				t.Errorf("content %+v overlaps takeaway %+v", frame.Content, frame.Takeaway)
			}
		})
	}
}

func TestModernYellowMasterDiscExcludedFromContent(t *testing.T) {
	r, err := OpenTemplate(filepath.Join("..", "..", "templates", "modern-yellow.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	p, err := BuildProfile(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []types.CanonicalLayoutType{types.CanonicalLayoutOneContent, types.CanonicalLayoutTwoContent} {
		id := p.RoleBindings[role]
		layout := p.Layout(id)
		if layout == nil {
			t.Fatalf("missing layout for %s", role)
		}
		var disc *types.DecorRegion
		for i := range layout.DecorRegions {
			if layout.DecorRegions[i].Source == "master" && layout.DecorRegions[i].Name == "Freeform 3" {
				disc = &layout.DecorRegions[i]
				break
			}
		}
		if disc == nil {
			t.Fatalf("%s missing master disc", role)
		}
		for _, bands := range []bool{false, true} {
			frame := p.ChromeFrame(id, bands, bands)
			if frame.Content.X < disc.X+disc.Width {
				t.Errorf("%s bands=%v: content starts %d inside disc ending %d", role, bands, frame.Content.X, disc.X+disc.Width)
			}
			if bands && frame.Takeaway.X < disc.X+disc.Width {
				t.Errorf("%s takeaway starts inside disc", role)
			}
		}
	}
}

func TestResolveChromeFrameAcrossAspectRatios(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int64
	}{
		{"four-three", 9144000, 6858000},
		{"wide", 12192000, 6858000},
		{"ultrawide", 16000000, 6000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ResolveChromeFrame(nil, nil, tc.w, tc.h, true, true)
			for name, r := range map[string]ChromeRect{"content": f.Content, "takeaway": f.Takeaway, "source": f.Source} {
				if r.X < 0 || r.Y < 0 || r.CX < 0 || r.CY < 0 || r.X+r.CX > tc.w || r.Y+r.CY > tc.h {
					t.Errorf("%s outside %dx%d: %+v", name, tc.w, tc.h, r)
				}
			}
			if f.Content.Bottom() > f.Takeaway.Y || f.Takeaway.Bottom() > f.Source.Y {
				t.Fatalf("chrome regions overlap: %+v", f)
			}
			if f.Basis != ChromeBasisSlideFallback {
				t.Errorf("basis = %q, want slide_fallback", f.Basis)
			}
		})
	}
}

func TestResolveChromeFrameReservationMonotonic(t *testing.T) {
	base := ResolveChromeFrame(nil, nil, 12192000, 6858000, false, false)
	source := ResolveChromeFrame(nil, nil, 12192000, 6858000, false, true)
	both := ResolveChromeFrame(nil, nil, 12192000, 6858000, true, true)
	if base.Content.CY <= source.Content.CY || source.Content.CY <= both.Content.CY {
		t.Fatalf("reservations do not reduce content monotonically: %d %d %d", base.Content.CY, source.Content.CY, both.Content.CY)
	}
}

func TestResolveChromeFrameNoFitAndReference(t *testing.T) {
	// A layout whose footer sits just below a tall body leaves no room for
	// the band stack: the frame must report Fits=false instead of overlapping.
	cramped := &types.LayoutMetadata{
		ID: "cramped",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 500000, Y: 200000, Width: 11000000, Height: 3600000}},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 500000, Y: 3900000, Width: 11000000, Height: 1500000}},
		},
		FooterRegions: []types.ChromeRegion{{Type: "ftr", X: 500000, Y: 5600000, Width: 4000000, Height: 300000}},
	}
	if f := ResolveChromeFrame(cramped, nil, 12192000, 6858000, true, true); f.Fits {
		t.Errorf("expected Fits=false for cramped layout, got %+v", f)
	}
	if f := ResolveChromeFrame(cramped, nil, 12192000, 6858000, false, false); !f.Fits {
		t.Error("no bands requested must always fit")
	}

	// A title-only layout borrows the reference layout's body column.
	ref := &types.LayoutMetadata{ID: "ref", Placeholders: []types.PlaceholderInfo{
		{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 700000, Y: 1500000, Width: 10000000, Height: 4000000}},
	}}
	titleOnly := &types.LayoutMetadata{ID: "t", Placeholders: []types.PlaceholderInfo{
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 300000, Y: 300000, Width: 11000000, Height: 1000000}},
	}}
	f := ResolveChromeFrame(titleOnly, ref, 12192000, 6858000, true, false)
	if f.Basis != ChromeBasisReferenceLayout || f.Takeaway.X != 700000 || f.Takeaway.CX != 10000000 {
		t.Errorf("reference geometry not used: %+v", f)
	}
	if f.HasFooter {
		t.Error("a known layout without footer regions must not borrow the reference footers")
	}
}

// TestBuildProfileCacheInvalidatesOnTemplateBytes verifies the profile cache is
// keyed by content hash: rewriting the template at the same path with different
// bytes (here a different slide size) yields a fresh profile, not a stale hit.
func TestBuildProfileCacheInvalidatesOnTemplateBytes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tpl.pptx")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	first := buildProfileAt(t, path)

	// Same path, new bytes: switch the canvas to 4:3.
	if err := os.WriteFile(path, rewriteZipEntry(t, src, "ppt/presentation.xml", func(s string) string {
		return strings.Replace(s, `<p:sldSz cx="12192000" cy="6858000"/>`, `<p:sldSz cx="9144000" cy="6858000"/>`, 1)
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	second := buildProfileAt(t, path)

	if first.TemplateHash == second.TemplateHash {
		t.Fatal("template hash did not change with template bytes")
	}
	if second.SlideWidth != 9144000 || second.AspectRatio != "4:3" {
		t.Fatalf("stale profile after template change: %dx%d %s", second.SlideWidth, second.SlideHeight, second.AspectRatio)
	}
	if first.Geometry[0].Frame.Canvas.CX == second.Geometry[0].Frame.Canvas.CX {
		t.Error("geometry not rebuilt for the new canvas")
	}
}

func buildProfileAt(t *testing.T, path string) *TemplateProfile {
	t.Helper()
	r, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer func() { _ = r.Close() }()
	p, err := BuildProfile(r)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	return p
}

func rewriteZipEntry(t *testing.T, data []byte, name string, edit func(string) string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == name {
			content = []byte(edit(string(content)))
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func findBody(l *types.LayoutMetadata) *types.PlaceholderInfo {
	for i := range l.Placeholders {
		if l.Placeholders[i].Type == types.PlaceholderBody {
			return &l.Placeholders[i]
		}
	}
	return nil
}

func rectsOverlap(a, b ChromeRect) bool {
	return a.X < b.X+b.CX && b.X < a.X+a.CX && a.Y < b.Y+b.CY && b.Y < a.Y+a.CY
}

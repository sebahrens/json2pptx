package template

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
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
			// midnight-blue's full-height accent bar and stripe sit inside the
			// left margin: the column clears them by the right-hand margin
			// (stripe end 396000 + 838200), go-slide-creator-oa0ru.
			wantLeft := body.Bounds.X
			if name == "midnight-blue" {
				wantLeft = 1234200
			}
			if d := frame.Takeaway.X - wantLeft; d < -1 || d > 1 {
				t.Errorf("takeaway x = %d, want %d (±1)", frame.Takeaway.X, wantLeft)
			}
			if gap := frame.Source.Y - frame.Takeaway.Bottom(); gap < 12*12700 {
				t.Errorf("takeaway-to-source gap = %d EMU, want at least 12pt", gap)
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
	// Read the artwork independently of the profile's opaque-obstacle filter.
	// The repaired disc is translucent, but its geometry must still clear text.
	master, err := r.ReadFile("ppt/slideMasters/slideMaster1.xml")
	if err != nil {
		t.Fatal(err)
	}
	var doc decorDocument
	if err := xml.Unmarshal(master, &doc); err != nil {
		t.Fatal(err)
	}
	var disc *types.DecorRegion
	for _, shape := range doc.Shapes {
		if shape.NV.CNV.Name != "Freeform 3" {
			continue
		}
		if disc != nil || shape.Properties.Xfrm == nil || shape.Properties.Xfrm.Off == nil || shape.Properties.Xfrm.Ext == nil {
			t.Fatal("missing or duplicate native disc geometry")
		}
		x := shape.Properties.Xfrm
		disc = &types.DecorRegion{X: x.Off.X, Y: x.Off.Y, Width: x.Ext.CX, Height: x.Ext.CY}
	}
	if disc == nil || disc.Width <= 0 || disc.Height <= 0 || disc.X+disc.Width != 300000 {
		t.Fatalf("native gutter disc geometry drifted: %+v", disc)
	}
	for _, role := range []types.CanonicalLayoutType{types.CanonicalLayoutOneContent, types.CanonicalLayoutTwoContent} {
		id := p.RoleBindings[role]
		layout := p.Layout(id)
		if layout == nil {
			t.Fatalf("missing layout for %s", role)
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

func TestResolveChromeFrameExcludesOpaqueSideArtwork(t *testing.T) {
	layout := &types.LayoutMetadata{
		Placeholders: []types.PlaceholderInfo{{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 792164, Y: 2051050, Width: 10961222, Height: 3692525}}},
		DecorRegions: []types.DecorRegion{{Source: "master", Name: "synthetic opaque disc", X: 0, Y: 1898247, Width: 2079706, Height: 4140000}},
	}
	for _, bands := range []bool{false, true} {
		frame := ResolveChromeFrame(layout, nil, 12192000, 6858000, bands, bands)
		if frame.Content.X <= 2079706 || (bands && frame.Takeaway.X <= 2079706) {
			t.Fatalf("opaque side artwork overlaps content or takeaway: %+v", frame)
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
			// A bare hasTakeaway reserves the one-line band: the two-line
			// band (9.65% of the slide height, at least the 52pt two lines
			// need) less one 17pt line.
			if want := max(tc.h*takeawayBandHeightPer10k/10000, TakeawayTextHeightEMU(2)) - 17*12700; f.Takeaway.CY != want {
				t.Errorf("one-line takeaway band = %.1fpt, want %.1fpt", float64(f.Takeaway.CY)/12700, float64(want)/12700)
			}
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

	// A title-only layout's bands share its title column, the span its
	// shape_grid content takes, not the reference body column
	// (go-slide-creator-svrpx).
	ref := &types.LayoutMetadata{ID: "ref", Placeholders: []types.PlaceholderInfo{
		{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 700000, Y: 1500000, Width: 10000000, Height: 4000000}},
	}}
	titleOnly := &types.LayoutMetadata{ID: "t", Placeholders: []types.PlaceholderInfo{
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 300000, Y: 300000, Width: 11000000, Height: 1000000}},
	}}
	f := ResolveChromeFrame(titleOnly, ref, 12192000, 6858000, true, false)
	if f.Basis != ChromeBasisLayoutTitle || f.Takeaway.X != 300000 || f.Takeaway.CX != 12192000-2*300000 {
		t.Errorf("title column not used: %+v", f)
	}
	// Without a title the reference body column still applies.
	noTitle := &types.LayoutMetadata{ID: "b"}
	if nf := ResolveChromeFrame(noTitle, ref, 12192000, 6858000, true, false); nf.Basis != ChromeBasisReferenceLayout || nf.Takeaway.X != 700000 {
		t.Errorf("reference geometry not used: %+v", nf)
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

// TestChromeFrameReservesTheTakeawayItsTextNeeds pins go-slide-creator-me53q:
// the frame used to reserve a 40.5pt (two-line) band under every takeaway, so
// a one-line takeaway left 17pt of empty band and every pattern under it lost
// 17pt of content height. The band is now reserved by the takeaway's measured
// line count at the layout's real band width, on every bundled template.
func TestChromeFrameReservesTheTakeawayItsTextNeeds(t *testing.T) {
	const (
		oneLine = "Margin recovers in the second half"
		twoLine = "Margin recovers in the second half as the pricing reset lands in the three largest regions, the freight contract reprices and the two loss-making plants close on schedule in the autumn"
	)
	matches, err := filepath.Glob(filepath.Join("..", "..", "templates", "*.pptx"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no templates: %v", err)
	}
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".pptx")
		t.Run(name, func(t *testing.T) {
			r, err := OpenTemplate(path)
			if err != nil {
				t.Fatalf("OpenTemplate: %v", err)
			}
			defer func() { _ = r.Close() }()
			p, err := BuildProfile(r)
			if err != nil {
				t.Fatalf("BuildProfile: %v", err)
			}
			ref := p.ReferenceLayout()
			if ref == nil {
				t.Skip("no One Content layout")
			}
			if ref.BodyFont == "" {
				t.Errorf("parsed layout carries no body font; the takeaway would be measured in the Arial fallback")
			}
			none := p.ChromeFrameForTakeaway(ref.ID, "", true)
			one := p.ChromeFrameForTakeaway(ref.ID, oneLine, true)
			two := p.ChromeFrameForTakeaway(ref.ID, twoLine, true)
			if !none.Takeaway.IsZero() {
				t.Errorf("no takeaway reserved a band: %+v", none.Takeaway)
			}
			// Two lines take the band's share of the slide height, and never
			// less than the two lines need.
			if want := max(p.SlideHeight*takeawayBandHeightPer10k/10000, TakeawayTextHeightEMU(2)); two.Takeaway.CY != want {
				t.Errorf("two-line takeaway band = %.1fpt, want %.1fpt", float64(two.Takeaway.CY)/12700, float64(want)/12700)
			}
			if one.Takeaway.CY < TakeawayTextHeightEMU(1) {
				t.Errorf("one-line takeaway band = %.1fpt, under the 35pt its text needs", float64(one.Takeaway.CY)/12700)
			}
			if gain := float64(one.Content.CY-two.Content.CY) / 12700; gain != 17 {
				t.Errorf("a one-line takeaway leaves the content %.1fpt more than a two-line one, want 17pt", gain)
			}
			for label, f := range map[string]ChromeFrame{"one-line": one, "two-line": two} {
				if gap := f.Source.Y - f.Takeaway.Bottom(); gap < 12*12700 {
					t.Errorf("%s: takeaway sits %.1fpt above the source line, want at least 12pt", label, float64(gap)/12700)
				}
				if gap := f.Takeaway.Y - f.Content.Bottom(); gap < 16*12700 {
					t.Errorf("%s: content ends %.1fpt above the takeaway, want at least 16pt", label, float64(gap)/12700)
				}
			}
			// The bare frame and the text frame agree for a one-line takeaway.
			if bare := p.ChromeFrame(ref.ID, true, true); bare != one {
				t.Errorf("ChromeFrame(true) = %+v, ChromeFrameForTakeaway(one line) = %+v", bare, one)
			}
			// A third line never grows the band past two lines: the takeaway
			// fit finding refuses it instead.
			if three := p.ChromeFrameForTakeaway(ref.ID, twoLine+" "+twoLine, true); three.Takeaway != two.Takeaway {
				t.Errorf("three-line takeaway band %+v, want the two-line band %+v", three.Takeaway, two.Takeaway)
			}
		})
	}
}

// TestTakeawayLinesReservesTheSecondLineNearTheEdge: a takeaway within 4% of
// the line end reserves two lines, so a slightly wider renderer font cannot
// wrap it into a band that was reserved for one.
func TestTakeawayLinesReservesTheSecondLineNearTheEdge(t *testing.T) {
	const band = int64(600 * 12700)
	text := "Margin"
	for TakeawayLines(text+" recovers", "Arial", band, 1) == 1 {
		text += " recovers"
	}
	// text is the longest run of words that still fits one line as drawn.
	if got := TakeawayLines(text, "Arial", band, 1); got != 1 {
		t.Fatalf("setup: %q measures %d lines", text, got)
	}
	if got := TakeawayLines(text, "Arial", int64(float64(band)*0.5), takeawayWrapSlack); got < 2 {
		t.Errorf("half-width band: %d lines, want a wrap", got)
	}
	const h = int64(6858000)
	if one, two, five := TakeawayBandHeightEMU(1, h), TakeawayBandHeightEMU(2, h), TakeawayBandHeightEMU(5, h); one != 445897 || two != 661797 || five != two {
		t.Errorf("band heights on a 7.5in slide = %d / %d / %d EMU, want 35.1pt / 52.1pt / 52.1pt", one, two, five)
	}
	if TakeawayTextHeightEMU(1) != 35*12700 || TakeawayTextHeightEMU(2) != 52*12700 {
		t.Errorf("text heights = %d / %d EMU, want 35pt / 52pt", TakeawayTextHeightEMU(1), TakeawayTextHeightEMU(2))
	}
	// A short slide still reserves what two lines need.
	if short := TakeawayBandHeightEMU(2, 5143500); short != TakeawayTextHeightEMU(2) {
		t.Errorf("two-line band on a 5.6in slide = %.1fpt, want the 52pt the text needs", float64(short)/12700)
	}
}

//go:build probe

package main

import (
	"archive/zip"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"regexp"

	"math"
	"os"
	"os/exec"
	"path/filepath"

	"strconv"
	"strings"
	"testing"
)

// Run with JSON2PPTX_CYCLE_FIGURE_EIGHT_RENDER=1 (needs soffice and pdftoppm
// on PATH): renders the figure through LibreOffice and samples the crossing.
// The band must run through it in one piece: along both diagonals, from the
// crossing point to each lobe's end and on into the lobe's end segment, no
// pixel is the page colour (no gap where an arm meets its lobe, none at the
// crossing point). The two upper arms — the ones that enter a lobe — carry a
// dark arrowhead on the band; the two lower arms carry nothing.
func TestCycleFigureEightCrossingRendersFilled(t *testing.T) {
	if os.Getenv("JSON2PPTX_CYCLE_FIGURE_EIGHT_RENDER") == "" {
		t.Skip("set JSON2PPTX_CYCLE_FIGURE_EIGHT_RENDER=1")
	}
	soffice, err1 := exec.LookPath("soffice")
	pdftoppm, err2 := exec.LookPath("pdftoppm")
	if err1 != nil || err2 != nil {
		t.Skip("soffice and pdftoppm are required")
	}
	for _, tpl := range []string{"abstract", "midnight-blue"} {
		for _, n := range []int{4, 8} {
			t.Run(fmt.Sprintf("%s/%d", tpl, n), func(t *testing.T) {
				values := cycleFigureEightBudgetValues(n, 0)
				values.LeftLabel, values.RightLabel = "", ""
				path := generateForSweep(t, tpl, cycleFigureEightDeck(t, values, nil))
				dir := filepath.Dir(path)
				profile := "-env:UserInstallation=file://" + filepath.Join(dir, "lo-profile")
				if out, err := exec.Command(soffice, profile, "--headless", "--convert-to", "pdf", "--outdir", dir, path).CombinedOutput(); err != nil { //nolint:gosec // test-controlled arguments
					t.Fatalf("soffice: %v\n%s", err, out)
				}
				pdf := strings.TrimSuffix(path, ".pptx") + ".pdf"
				if out, err := exec.Command(pdftoppm, "-r", "100", "-png", "-singlefile", pdf, filepath.Join(dir, "slide")).CombinedOutput(); err != nil { //nolint:gosec // test-controlled arguments
					t.Fatalf("pdftoppm: %v\n%s", err, out)
				}
				f, err := os.Open(filepath.Join(dir, "slide.png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				img, _, err := image.Decode(f)
				if err != nil {
					t.Fatal(err)
				}

				// The lobes' squares, from the blockArc frames in the slide.
				m := cfeSldSz.FindStringSubmatch(cfeZipText(t, path, "ppt/presentation.xml"))
				if m == nil {
					t.Fatal("no slide size")
				}
				slideW, _ := strconv.ParseFloat(m[1], 64)
				slideH, _ := strconv.ParseFloat(m[2], 64)
				var minX, maxX, top, side, bandFrac float64
				minX = math.Inf(1)
				for _, sp := range cfeSpRe.FindAllString(cfeZipText(t, path, "ppt/slides/slide1.xml"), -1) {
					if !strings.Contains(sp, `prst="blockArc"`) {
						continue
					}
					o := cfeOffRe.FindStringSubmatch(sp)
					g := cfeAdj3.FindStringSubmatch(sp)
					if o == nil || g == nil {
						continue
					}
					x, _ := strconv.ParseFloat(o[1], 64)
					y, _ := strconv.ParseFloat(o[2], 64)
					cx, _ := strconv.ParseFloat(o[3], 64)
					adj3, _ := strconv.ParseFloat(g[1], 64)
					minX, maxX, top, side, bandFrac = math.Min(minX, x), math.Max(maxX, x), y, cx, adj3/100000
				}
				// The rings are a little smaller than their cells, which share
				// an edge: the centres are one cell side apart.
				cell := maxX - minX
				if side == 0 || side > cell*1.001 || side < cell*0.94 {
					t.Fatalf("lobes at x=%.0f and x=%.0f, %.0f EMU across, are not two rings of 94-100%% of adjoining squares", minX, maxX, side)
				}
				b := img.Bounds()
				px := func(x, y float64) color.Color {
					return img.At(b.Min.X+int(x/slideW*float64(b.Dx())), b.Min.Y+int(y/slideH*float64(b.Dy())))
				}
				same := func(a, c color.Color) bool {
					ar, ag, ab, _ := a.RGBA()
					cr, cg, cb, _ := c.RGBA()
					d := func(p, q uint32) float64 { return math.Abs(float64(p) - float64(q)) }
					return d(ar, cr)+d(ag, cg)+d(ab, cb) < 3*0x0600
				}
				touchX, touchY := minX+side/2+cell/2, top+side/2
				// The page: the hole of the left lobe (no lobe title on this deck).
				page := px(minX+side/2, touchY)
				fill := px(touchX, touchY)
				if same(fill, page) {
					t.Fatalf("the crossing point is the page colour: the crossing is not filled")
				}
				// A lobe's end, in cell sides from the crossing point: its
				// centreline radius is R, the tangent from the crossing point
				// (half a side from the centre) meets it at cos δ = 2R.
				radius := (side - bandFrac*side) / 2 / cell
				endX, endY := 0.5-2*radius*radius, radius*math.Sqrt(1-4*radius*radius)
				for _, dir := range [][2]float64{{-endX, -endY}, {-endX, endY}, {endX, -endY}, {endX, endY}} {
					dark := 0
					// Past 100% the samples are on the lobe's end segment (the
					// tangent leaves the band slowly).
					for step := 0.0; step <= 1.12; step += 0.02 {
						x, y := touchX+dir[0]*cell*step, touchY+dir[1]*cell*step
						got := px(x, y)
						switch {
						case same(got, fill):
						case same(got, page):
							t.Errorf("arm towards (%.2f, %.2f): unfilled at %.0f%% of its length", dir[0], dir[1], step*100)
						default:
							dark++
						}
					}
					switch upper := dir[1] < 0; {
					case upper && dark < 3:
						t.Errorf("arm towards (%.2f, %.2f) enters its lobe: %d samples on an arrowhead, want at least 3", dir[0], dir[1], dark)
					case !upper && dark > 0:
						t.Errorf("arm towards (%.2f, %.2f) leaves its lobe: %d samples are neither band nor page, want a plain band", dir[0], dir[1], dark)
					}
				}
			})
		}
	}
}

var (
	cfeSpRe  = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	cfeOffRe = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)
	cfeSldSz = regexp.MustCompile(`<p:sldSz cx="(\d+)" cy="(\d+)"`)
	cfeAdj3  = regexp.MustCompile(`<a:gd name="adj3" fmla="val (\d+)"/>`)
)

func cfeZipText(t *testing.T, pptxPath, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

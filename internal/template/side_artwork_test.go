package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Test-only temporary templates exercise real layout/master parsing without
// restoring side artwork to blue-corporate or its logo-free portability copy.
// These are collision-geometry controls, not visual approval of a new design.
func TestSyntheticSideArtworkProfileAcrossCanvases(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "templates", "modern-template.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	for _, canvas := range []struct {
		name string
		w, h int64
	}{
		{"wide", 12192000, 6858000},
		{"four-three", 9144000, 6858000},
		{"portrait", 6858000, 9144000},
		{"ultrawide", 16000000, 6000000},
	} {
		for _, origin := range []string{"master-shape", "layout-picture"} {
			for _, side := range []string{"left", "right"} {
				t.Run(canvas.name+"/"+origin+"/"+side, func(t *testing.T) {
					t.Parallel()
					data := rewriteZipEntry(t, source, "ppt/presentation.xml", func(xml string) string {
						old := `<p:sldSz cx="12192000" cy="6858000"/>`
						if strings.Count(xml, old) != 1 {
							t.Fatal("native canvas declaration missing or duplicated")
						}
						return strings.Replace(xml, old, fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/>`, canvas.w, canvas.h), 1)
					})
					path := filepath.Join(t.TempDir(), "synthetic-test-only.pptx")
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					baseline := buildProfileAt(t, path)
					base := baseline.ChromeFrame("slideLayout5", false, false)
					width := canvas.w / 5
					x := base.Content.X
					if side == "right" {
						x = base.Content.X + base.Content.CX - width
					}
					name := "SYNTHETIC TEST ONLY side artwork"
					transform := fmt.Sprintf(`<a:xfrm><a:off x="%d" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, x, width, canvas.h)
					if origin == "master-shape" {
						data = rewriteZipEntry(t, data, "ppt/slideMasters/slideMaster1.xml", func(xml string) string {
							if strings.Count(xml, "</p:spTree>") != 1 {
								t.Fatal("native master shape tree missing or duplicated")
							}
							fixture := `<p:sp><p:nvSpPr><p:cNvPr id="9001" name="` + name + `"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>` + transform + `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr></p:sp>`
							return strings.Replace(xml, "</p:spTree>", fixture+"</p:spTree>", 1)
						})
					} else {
						data = rewriteZipEntry(t, data, "ppt/slideLayouts/slideLayout5.xml", func(xml string) string {
							pictures := regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`)
							if len(pictures.FindAllString(xml, -1)) != 1 {
								t.Fatal("expected one existing native Closing image")
							}
							return pictures.ReplaceAllStringFunc(xml, func(pic string) string {
								if strings.Count(pic, `name="Picture 7"`) != 1 {
									t.Fatal("existing native picture identity changed")
								}
								pic = strings.Replace(pic, `name="Picture 7"`, `name="`+name+`"`, 1)
								xfrm := regexp.MustCompile(`(?s)<a:xfrm[^>]*>.*?</a:xfrm>`)
								if len(xfrm.FindAllString(pic, -1)) != 1 {
									t.Fatal("native picture transform missing or duplicated")
								}
								return xfrm.ReplaceAllString(pic, transform)
							})
						})
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					profile := buildProfileAt(t, path)
					if profile.TemplateHash == baseline.TemplateHash {
						t.Fatal("temporary artwork mutation did not invalidate profile cache")
					}
					layout := profile.Layout("slideLayout5")
					if layout == nil {
						t.Fatal("native Closing layout missing")
					}
					found := 0
					for _, region := range layout.DecorRegions {
						if region.Name == name {
							found++
							wantOrigin := "master"
							if origin == "layout-picture" {
								wantOrigin = "layout"
							}
							if region.Source != wantOrigin || region.X != x || region.Y != 0 || region.Width != width || region.Height != canvas.h {
								t.Fatalf("native artwork parsing lost source/geometry: %+v", region)
							}
						}
					}
					if found != 1 {
						t.Fatalf("side artwork region count=%d, want1", found)
					}
					padding := min(canvas.w, canvas.h) * 26 / 1000
					for _, bands := range []bool{false, true} {
						frame := profile.ChromeFrame("slideLayout5", bands, bands)
						wantLeft, wantRight := base.Content.X, base.Content.X+base.Content.CX
						if side == "left" {
							wantLeft = x + width + padding
						} else {
							wantRight = x - padding
						}
						if !frame.Fits || frame.Content.X != wantLeft || frame.Content.CX != wantRight-wantLeft {
							t.Fatalf("bands=%v artwork reservation incorrect: %+v; want horizontal span%d..%d", bands, frame, wantLeft, wantRight)
						}
						if bands && (frame.Takeaway.X != wantLeft || frame.Takeaway.CX != wantRight-wantLeft || frame.Source.X != wantLeft || frame.Source.CX != wantRight-wantLeft) {
							t.Fatal("source/takeaway bands do not share artwork-safe span")
						}
					}
				})
			}
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/png"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Exercise authored full-slide bounds through the shared CLI/MCP runner and
// inspect the saved package, rather than trusting intermediate grid bounds.
// Keep this in the short suite: CI must catch SVG returning to a square frame.
func TestFullSlideImages(t *testing.T) {
	for _, format := range []string{"png", "svg"} {
		t.Run(format, func(t *testing.T) {
			if format == "svg" && !generator.NewSVGConverter().IsPNGAvailable() {
				t.Skip("native SVG fallback needs rsvg-convert or resvg")
			}
			dir := t.TempDir()
			cases := []struct {
				name, fit, cellFit string
				width, height      int
			}{
				{"landscape-default", "", "", 1600, 900},
				{"landscape-contain", "contain", "", 1600, 900},
				{"portrait-contain", "contain", "", 600, 900},
				{"portrait-cover", "cover", "", 600, 900},
				{"square-contain", "contain", "", 900, 900},
				{"explicit-square", "cover", "contain", 1600, 900},
			}
			in := &PresentationInput{Template: "midnight-blue", OutputFilename: "full-slide.pptx"}
			for _, tc := range cases {
				file := filepath.Join(dir, tc.name+"."+format)
				if format == "png" {
					writeTestPNG(t, file, tc.width, tc.height)
				} else {
					// viewBox-only sources are common exported diagrams. The two
					// solid halves also identify real rendering in the PNG fallback.
					svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><rect width="%d" height="%d" fill="#ff0000"/><rect x="%d" width="%d" height="%d" fill="#0000ff"/></svg>`, tc.width, tc.height, tc.width, tc.height, tc.width/2, tc.width/2, tc.height)
					if err := os.WriteFile(file, []byte(svg), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				in.Slides = append(in.Slides, SlideInput{
					LayoutID: "blank",
					ShapeGrid: &ShapeGridInput{
						Bounds: &GridBoundsInput{Width: 100, Height: 100}, Columns: json.RawMessage(`1`),
						Rows: []GridRowInput{{Cells: []*GridCellInput{{Fit: tc.cellFit, Image: &GridImageInput{Path: file, Fit: tc.fit, Alt: tc.name}}}}},
					},
				})
			}
			applyDefaults(in)
			result, cleanup, err := RunPresentation(t.Context(), in, RenderOptions{
				OutputDir: dir, TemplatesDir: testutil.TemplatesDir(), StrictFit: "off",
				OutputValidation: "strict", AllowedImagePaths: []string{dir}, SVGStrategy: "native",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			presentation := readZipEntry(t, result.OutputPath, "ppt/presentation.xml")
			w := fullSlideXMLInt(t, presentation, "<p:sldSz ", "cx")
			h := fullSlideXMLInt(t, presentation, "<p:sldSz ", "cy")
			if w <= h || h <= 0 {
				t.Fatalf("fixture must have a landscape slide, got %dx%d", w, h)
			}
			for i, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					slide := readDeckSlide(t, result.OutputPath, i+1)
					pics := regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`).FindAll(slide, -1)
					if len(pics) != 1 {
						t.Fatalf("want one image, got %d", len(pics))
					}
					pic := string(pics[0])
					xfrm := regexp.MustCompile(`(?s)<a:xfrm[^>]*>.*?</a:xfrm>`).FindString(pic)
					x, y, cx, cy := int64(0), int64(0), w, h
					if tc.cellFit == "contain" {
						cx = h
						x = (w - h) / 2
					}
					if tc.fit == "contain" {
						scale := math.Min(float64(cx)/float64(tc.width), float64(cy)/float64(tc.height))
						fitW, fitH := int64(float64(tc.width)*scale), int64(float64(tc.height)*scale)
						x, y = x+(cx-fitW)/2, y+(cy-fitH)/2
						cx, cy = fitW, fitH
					}
					for _, attr := range []struct {
						tag, name string
						want      int64
					}{{"<a:off ", "x", x}, {"<a:off ", "y", y}, {"<a:ext ", "cx", cx}, {"<a:ext ", "cy", cy}} {
						if got := fullSlideXMLInt(t, xfrm, attr.tag, attr.name); !near(got, attr.want, 2) {
							t.Errorf("%s %s = %d, want %d (slide %dx%d)", attr.tag, attr.name, got, attr.want, w, h)
						}
					}
					vertical, horizontal := slideSrcRect(pics[0])
					if (tc.fit == "contain" || tc.name == "landscape-default") && (vertical != 0 || horizontal != 0) {
						t.Errorf("image that should show the entire source was cropped: vertical=%d horizontal=%d", vertical, horizontal)
					}
					if tc.name == "portrait-cover" && (vertical <= 0 || horizontal != 0) {
						t.Errorf("portrait cover must crop vertically: vertical=%d horizontal=%d", vertical, horizontal)
					}
					if tc.name == "explicit-square" && (horizontal <= 0 || vertical != 0) {
						t.Errorf("square cover must crop landscape source horizontally: vertical=%d horizontal=%d", vertical, horizontal)
					}
					if format == "svg" {
						verifyFullSlideSVGMedia(t, result.OutputPath, i+1, pic, tc.width, tc.height)
					}
				})
			}
		})
	}
}

func fullSlideXMLInt(t *testing.T, src, tag, attr string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(extractAttr(src, tag, attr), 10, 64)
	if err != nil {
		t.Fatalf("missing/invalid %s %s: %v", tag, attr, err)
	}
	return value
}

func verifyFullSlideSVGMedia(t *testing.T, deck string, slide int, pic string, width, height int) {
	t.Helper()
	svgID := extractAttr(pic, "<asvg:svgBlip ", "r:embed")
	pngID := extractAttr(pic, "<a:blip ", "r:embed")
	if svgID == "" || pngID == "" || svgID == pngID {
		t.Fatalf("native SVG must have separate SVG and PNG references: %s", pic)
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal([]byte(readZipEntry(t, deck, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slide))), &rels); err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, rel := range rels.Items {
		parts[rel.ID] = path.Clean(path.Join("ppt/slides", rel.Target))
	}
	svg := readZipEntry(t, deck, parts[svgID])
	if !strings.Contains(svg, fmt.Sprintf(`viewBox="0 0 %d %d"`, width, height)) {
		t.Errorf("embedded SVG lost its original viewBox: %s", svg)
	}
	fallback, err := png.Decode(bytes.NewReader([]byte(readZipEntry(t, deck, parts[pngID]))))
	if err != nil {
		t.Fatalf("PNG fallback: %v", err)
	}
	b := fallback.Bounds()
	if b.Dx() <= 1 || b.Dy() <= 1 || math.Abs(float64(b.Dx())/float64(b.Dy())-float64(width)/float64(height)) > 0.01 {
		t.Fatalf("fallback dimensions %v do not preserve source %dx%d", b, width, height)
	}
	r, g, blue, a := fallback.At(b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/2).RGBA()
	if r < 60000 || g > 1000 || blue > 1000 || a < 60000 {
		t.Errorf("fallback must render the red left half, got RGBA %d %d %d %d", r, g, blue, a)
	}
	r, g, blue, a = fallback.At(b.Min.X+3*b.Dx()/4, b.Min.Y+b.Dy()/2).RGBA()
	if r > 1000 || g > 1000 || blue < 60000 || a < 60000 {
		t.Errorf("fallback must render the blue right half, got RGBA %d %d %d %d", r, g, blue, a)
	}
}

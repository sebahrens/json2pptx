package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func writeGridTestSVG(t *testing.T, attrs string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.svg")
	data := `<svg xmlns="http://www.w3.org/2000/svg" ` + attrs + `><rect width="100%" height="100%" fill="#134E96"/></svg>`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGridSVGImagePlacement(t *testing.T) {
	for _, tc := range []struct {
		name, attrs string
		wide        bool
	}{
		{"wide dimensions", `width="1600" height="900"`, true},
		{"tall dimensions", `width="900" height="1600"`, false},
		{"viewBox only", `viewBox="10 20 1600 900"`, true},
		{"relative dimensions", `width="100%" height="100%" viewBox="0,0,1600,900"`, true},
		{"physical units", `width="16in" height="648pt"`, true},
		{"fractional dimensions", `width="1.6" height="0.9"`, true},
		{"viewport overrides viewBox", `width="1600" height="900" viewBox="0 0 10 10"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGridTestSVG(t, tc.attrs)
			frame := types.BoundingBox{X: 100, Y: 200, Width: 1600000, Height: 1600000}
			p := GridImagePlacement(path, frame, "cover")
			if p.Bounds != frame {
				t.Fatalf("cover bounds = %+v, want %+v", p.Bounds, frame)
			}
			if tc.wide {
				if p.Crop.L != 21875 || p.Crop.R != 21875 || p.Crop.T != 0 || p.Crop.B != 0 {
					t.Fatalf("wide crop = %+v", p.Crop)
				}
				if _, _, visible := p.SourceToSlide(0.1, 0.5); visible {
					t.Error("cropped SVG anchor must be invisible")
				}
			} else if p.Crop.T != 21875 || p.Crop.B != 21875 || p.Crop.L != 0 || p.Crop.R != 0 {
				t.Fatalf("tall crop = %+v", p.Crop)
			}
			p = GridImagePlacement(path, frame, "contain")
			want := types.BoundingBox{X: 100, Y: 350200, Width: 1600000, Height: 900000}
			if !tc.wide {
				want = types.BoundingBox{X: 350100, Y: 200, Width: 900000, Height: 1600000}
			}
			if p.Bounds != want || !p.Crop.IsZero() {
				t.Fatalf("contain = %+v, want %+v with no crop", p, want)
			}
		})
	}
}

func TestGridSVGImageDeckMedia(t *testing.T) {
	if !NewSVGConverter().IsPNGAvailable() {
		t.Skip("rsvg-convert or resvg not installed")
	}
	path := writeGridTestSVG(t, `viewBox="0 0 1600 900"`)
	for _, strategy := range []string{"native", "png", "emf"} {
		for _, fit := range []string{"cover", "contain"} {
			t.Run(strategy+"/"+fit, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "grid-svg.pptx")
				_, err := Generate(context.Background(), GenerationRequest{
					TemplatePath: "../../templates/modern.pptx", OutputPath: output,
					AllowedImagePaths: []string{filepath.Dir(path)}, ExcludeTemplateSlides: true, SVGStrategy: strategy,
					Slides: []SlideSpec{{LayoutID: "slideLayout2", ImageInserts: []ImageInsert{{Path: path, Alt: "SVG frame test", Fit: fit, Geometry: "ellipse", OffsetX: 100, OffsetY: 200, ExtentCX: 6096000, ExtentCY: 6096000}}}},
				})
				if err != nil {
					t.Fatal(err)
				}
				z, err := zip.OpenReader(output)
				if err != nil {
					t.Fatal(err)
				}
				defer z.Close()
				slide := string(zipSlideXML(t, &z.Reader, 1))
				if got := strings.Contains(slide, "asvg:svgBlip"); got != (strategy == "native") {
					t.Errorf("native SVG = %v for strategy %s", got, strategy)
				}
				if !strings.Contains(slide, `prst="ellipse"`) {
					t.Error("SVG picture lost ellipse geometry")
				}
				if fit == "cover" {
					if !strings.Contains(slide, `<a:srcRect l="21875" t="0" r="21875" b="0"`) {
						t.Errorf("missing SVG cover crop: %s", slide)
					}
				} else {
					if strings.Contains(slide, "a:srcRect") {
						t.Error("contain must not crop")
					}
					if !strings.Contains(slide, `cx="6096000" cy="3429000"`) {
						t.Error("contain lost SVG aspect ratio")
					}
				}
				// Follow the picture's PNG relationship so template media cannot satisfy
				// this check accidentally. The generated SVG frame has its unique alt text.
				var pic struct {
					Tree struct {
						Pics []struct {
							NV struct {
								Props struct {
									Descr string `xml:"descr,attr"`
								} `xml:"cNvPr"`
							} `xml:"nvPicPr"`
							Fill struct {
								Blip struct {
									Embed string `xml:"embed,attr"`
								} `xml:"blip"`
							} `xml:"blipFill"`
						} `xml:"pic"`
					} `xml:"cSld>spTree"`
				}
				// Namespaces are deliberately ignored by encoding/xml's local-name match.
				if err := xml.Unmarshal([]byte(slide), &pic); err != nil {
					t.Fatal(err)
				}
				var relID string
				for _, p := range pic.Tree.Pics {
					if p.NV.Props.Descr == "SVG frame test" {
						relID = p.Fill.Blip.Embed
					}
				}
				if relID == "" {
					t.Fatal("image relationship missing")
				}
				relData := readGridZipEntry(t, &z.Reader, "ppt/slides/_rels/slide1.xml.rels")
				var rels struct {
					Items []struct {
						ID     string `xml:"Id,attr"`
						Target string `xml:"Target,attr"`
					} `xml:"Relationship"`
				}
				if err := xml.Unmarshal(relData, &rels); err != nil {
					t.Fatal(err)
				}
				var target string
				for _, rel := range rels.Items {
					if rel.ID == relID {
						target = filepath.ToSlash(filepath.Clean(filepath.Join("ppt/slides", rel.Target)))
					}
				}
				data := readGridZipEntry(t, &z.Reader, target)
				image, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				b := image.Bounds()
				if b.Dx() < 640 || b.Dy() < 360 || math.Abs(float64(b.Dx()*9-b.Dy()*16)) > 16 {
					t.Fatalf("fallback resolution/aspect = %v", b)
				}
				_, _, _, alpha := image.At(b.Dx()/2, b.Dy()/2).RGBA()
				if alpha == 0 {
					t.Fatal("SVG fallback is transparent")
				}
			})
		}
	}
}

func readGridZipEntry(t *testing.T, z *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	t.Fatalf("missing ZIP entry %s", name)
	return nil
}

func TestSVGImageDimensions(t *testing.T) {
	for _, tc := range []struct {
		name, attrs string
		w, h        float64
		wantErr     bool
	}{
		{"pixels", `width="1600" height="900"`, 1600, 900, false},
		{"width and viewBox", `width="800" viewBox="0 0 1600 900"`, 800, 450, false},
		{"height and viewBox", `height="450" viewBox="0 0 1600 900"`, 800, 450, false},
		{"missing dimensions", ``, 0, 0, true},
		{"invalid viewBox origin", `viewBox="bad 0 1600 900"`, 0, 0, true},
		{"infinite viewBox", `viewBox="0 0 Inf 900"`, 0, 0, true},
		{"nan dimensions", `width="NaN" height="900"`, 0, 0, true},
		{"negative viewBox", `viewBox="0 0 -1600 900"`, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, h, err := svgImageDimensions([]byte(`<svg ` + tc.attrs + `/>`))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && (w != tc.w || h != tc.h) {
				t.Fatalf("dimensions = %gx%g, want %gx%g", w, h, tc.w, tc.h)
			}
		})
	}
}

func TestGridSVGImageRequiresVisibleFallback(t *testing.T) {
	path := writeGridTestSVG(t, `width="1600" height="900"`)
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverter()
	markToolsFound(ctx.svgConverter)
	err := ctx.registerGridSVGImage(1, 0, ImageInsert{Path: path, ExtentCX: 1600000, ExtentCY: 900000})
	if err == nil || !strings.Contains(err.Error(), "shape_grid SVG image fallback PNG") {
		t.Fatalf("missing converter error = %v", err)
	}
	if len(ctx.nativeSVGInserts[1]) != 0 {
		t.Fatal("failed fallback must not leave an invisible SVG insert")
	}
}

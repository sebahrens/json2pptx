package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// The radial-hub test wall (go-slide-creator-rtb0y): every count on every
// template including the local p-style, the split layouts, the copy budgets
// the skill documents, the budget warning in the fit report and one rendered
// spoke.

var radialHubLabels = []string{"Marketing", "Sales", "Service", "Finance", "Risk and compliance", "Product", "Partners", "Data science"}

// radialHubValues is n spokes with real labels; descPt > 0 adds a description
// of that many characters to each.
func radialHubValues(n, labelChars, descChars int) *patterns.RadialHubValues {
	v := &patterns.RadialHubValues{Center: patterns.RadialHubCenter{Label: "Customer platform", Sublabel: "One governed source"}}
	for i := 0; i < n; i++ {
		s := patterns.RadialHubSpoke{Label: radialHubLabels[i]}
		if labelChars > 0 {
			s.Label = budgetProbeCopy(labelChars)
		}
		if descChars > 0 {
			s.Description = budgetProbeCopy(descChars)
		}
		v.Spokes = append(v.Spokes, s)
	}
	return v
}

// radialHubTemplates is every template (p-style included when installed), cut
// to the short matrix under -short.
func radialHubTemplates(t *testing.T) []string {
	t.Helper()
	if testing.Short() {
		return schemaMaximaRunTemplateNames(t)
	}
	return testutil.AllTestTemplateNames()
}

func radialHubSlideXML(t *testing.T, template string, slide SlideInput) string {
	t.Helper()
	slide.SlideType = "content"
	title := "Functions around one customer platform"
	slide.Content = []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}
	result, cleanup, err := RunPresentation(context.Background(), &PresentationInput{Template: template, Slides: []SlideInput{slide}}, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return readSlideXML(t, result.OutputPath, "ppt/slides/slide1.xml")
}

var radialHubRunRE = regexp.MustCompile(`<a:rPr\b[^>]*\bsz="(\d+)"`)

type radialHubShape struct {
	xml          string
	x, y, cx, cy int64
}

func radialHubShapes(t *testing.T, slideXML, geometry string) []radialHubShape {
	t.Helper()
	var out []radialHubShape
	for _, shape := range matrixRenderedShapeRE.FindAllString(slideXML, -1) {
		if !strings.Contains(shape, `<a:prstGeom prst="`+geometry+`">`) {
			continue
		}
		off, ext := matrixShapeOffsetRE.FindStringSubmatch(shape), matrixShapeExtentRE.FindStringSubmatch(shape)
		if len(off) != 3 || len(ext) != 3 {
			t.Fatalf("%s shape without a frame: %s", geometry, shape)
		}
		s := radialHubShape{xml: shape}
		for i, dst := range []*int64{&s.x, &s.y, &s.cx, &s.cy} {
			src := off
			if i >= 2 {
				src = ext
			}
			v, err := strconv.ParseInt(src[1+i%2], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			*dst = v
		}
		out = append(out, s)
	}
	return out
}

// assertRadialHubDrawing checks the written slide: circles are circles, every
// run is written at 12pt or above with no stored shrink below it, and there is
// one hub, wantSatellites satellites and wantSpokes rotated spokes.
func assertRadialHubDrawing(t *testing.T, slideXML string, wantSatellites, wantSpokes int) (hub radialHubShape, satellites []radialHubShape) {
	t.Helper()
	circles := radialHubShapes(t, slideXML, "ellipse")
	if len(circles) != wantSatellites+1 {
		t.Fatalf("%d circles, want a hub and %d satellites", len(circles), wantSatellites)
	}
	for i, c := range circles {
		if d := c.cx - c.cy; d < -1 || d > 1 || c.cx <= 0 {
			t.Errorf("circle %d is %d x %d EMU: not round", i, c.cx, c.cy)
		}
		if hub.cx < c.cx {
			hub = c
		}
	}
	for _, c := range circles {
		if c != hub {
			satellites = append(satellites, c)
		}
	}
	if !strings.Contains(hub.xml, "Customer") {
		t.Errorf("the largest circle is not the hub: %s", hub.xml)
	}
	if spokes := radialHubSpokes(t, slideXML, hub, satellites); len(spokes) != wantSpokes {
		t.Errorf("%d spokes, want %d", len(spokes), wantSpokes)
	}
	for _, shape := range matrixRenderedShapeRE.FindAllString(slideXML, -1) {
		if !strings.Contains(shape, "<a:t>") || strings.Contains(shape, `type="title"`) || strings.Contains(shape, `<p:ph`) {
			continue
		}
		scale := 100000
		if m := matrixFontScaleRE.FindStringSubmatch(shape); len(m) == 2 {
			scale, _ = strconv.Atoi(m[1])
		}
		for _, run := range radialHubRunRE.FindAllStringSubmatch(shape, -1) {
			size, _ := strconv.Atoi(run[1])
			if effective := size * scale / 100000; effective < 1200 {
				t.Errorf("text written at %.1fpt (declared %.1fpt, stored scale %d): %s", float64(effective)/100, float64(size)/100, scale, shape)
			}
		}
	}
	return hub, satellites
}

// radialHubSpokes are the thin text-less rectangles whose middle lies between
// the hub and the satellites' circle. A spoke's frame is its unrotated length
// x stroke box, so it is thin whatever its angle.
func radialHubSpokes(t *testing.T, slideXML string, hub radialHubShape, satellites []radialHubShape) []radialHubShape {
	t.Helper()
	if len(satellites) == 0 {
		return nil
	}
	hx, hy := float64(hub.x)+float64(hub.cx)/2, float64(hub.y)+float64(hub.cy)/2
	ring := math.Hypot(float64(satellites[0].x)+float64(satellites[0].cx)/2-hx, float64(satellites[0].y)+float64(satellites[0].cy)/2-hy)
	var out []radialHubShape
	for _, r := range radialHubShapes(t, slideXML, "rect") {
		if r.cy > 4*12700 || strings.Contains(r.xml, "<a:t>") {
			continue
		}
		if d := math.Hypot(float64(r.x)+float64(r.cx)/2-hx, float64(r.y)+float64(r.cy)/2-hy); d > float64(hub.cx)/2 && d < ring {
			out = append(out, r)
		}
	}
	return out
}

// Every count renders on every template with round circles, readable text and
// one spoke per satellite.
func TestRadialHubCountSweepAcrossTemplates(t *testing.T) {
	for _, tpl := range radialHubTemplates(t) {
		for n := 4; n <= 8; n++ {
			t.Run(fmt.Sprintf("%s/n=%d", tpl, n), func(t *testing.T) {
				t.Parallel()
				values, _ := json.Marshal(radialHubValues(n, 0, 24))
				xml := radialHubSlideXML(t, tpl, SlideInput{Pattern: &PatternInput{Name: "radial-hub", Values: values}})
				hub, sats := assertRadialHubDrawing(t, xml, n, n)
				// The satellites sit on one circle about the hub's centre.
				hx, hy := float64(hub.x)+float64(hub.cx)/2, float64(hub.y)+float64(hub.cy)/2
				var radius float64
				for i, s := range sats {
					r := math.Hypot(float64(s.x)+float64(s.cx)/2-hx, float64(s.y)+float64(s.cy)/2-hy)
					if i == 0 {
						radius = r
					} else if math.Abs(r-radius) > 12700 {
						t.Errorf("satellite %d is %.1fpt from the hub centre, the first %.1fpt", i, r/12700, radius/12700)
					}
				}
				for _, label := range radialHubLabels[:n] {
					if !strings.Contains(xml, "<a:t>"+label+"</a:t>") {
						t.Errorf("label %q missing from the slide", label)
					}
				}
			})
		}
	}
}

// The documented copy budgets hold on every template: no readability finding
// and no run written under its role's floor.
func TestRadialHubCopyBudgetsHoldAcrossTemplates(t *testing.T) {
	budgets := []struct{ n, label, description int }{
		{4, 26, 60}, {5, 26, 60}, {6, 26, 60}, {7, 22, 40}, {8, 22, 40},
	}
	for _, tpl := range radialHubTemplates(t) {
		geom := loadSchemaMaximaGeometry(t, tpl)
		for _, b := range budgets {
			t.Run(fmt.Sprintf("%s/n=%d", tpl, b.n), func(t *testing.T) {
				values, _ := json.Marshal(radialHubValues(b.n, b.label, b.description))
				input := &PresentationInput{Template: tpl, Slides: []SlideInput{{
					SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "radial-hub", Values: values},
				}}}
				if findings := collectReadabilityFindings(input, geom.layouts, geom.width, geom.height); len(findings) > 0 {
					t.Errorf("label %d / description %d characters with %d spokes: %+v", b.label, b.description, b.n, findings)
				}
				if n := writtenRoleViolations(t, input, geom); n > 0 {
					t.Errorf("label %d / description %d characters with %d spokes: %d runs written under their floor", b.label, b.description, b.n, n)
				}
			})
		}
	}
}

// Run with JSON2PPTX_RADIAL_HUB_BUDGET_PROBE=1 to measure the readable
// description budget per count (every label at its documented maximum).
func TestRadialHubBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_RADIAL_HUB_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_RADIAL_HUB_BUDGET_PROBE=1")
	}
	for n := 4; n <= 8; n++ {
		label := 26
		if n > 6 {
			label = 22
		}
		budget := probeReadableBudget(t, "radial-hub", 60, func(length int) any { return radialHubValues(n, label, length) })
		t.Logf("spokes=%d label=%d description budget=%d", n, label, budget)
	}
}

func TestRadialHubBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := radialHubValues(6, 0, 0)
	values.Spokes[4].Label = "Risk"
	values.Spokes[1].Label = "Infrastructure" // 14 characters in one word: allowed by the schema, too wide for a satellite
	assertBudgetFindingAcrossTemplates(t, "radial-hub", values, "spokes[1].label", "does not fit its", map[string]string{"labels": "inside"})
}

// A horizontal compose segment of 50% and 60%, a vertical segment and a
// nested grid cell keep the hub round and the text readable; a segment too
// narrow for two label columns falls back to the keyed legend.
func TestRadialHubSplitLayoutsAcrossTemplates(t *testing.T) {
	short := radialHubValues(6, 0, 0)
	short.Center = patterns.RadialHubCenter{Label: "Core platform"}
	short.Spokes[4].Label = "Risk"
	values, _ := json.Marshal(short)
	rows := json.RawMessage(`{"rows":[{"label":"WHY","body":"Each function keeps its own copy today."},{"label":"WHAT","body":"One governed platform for all."},{"label":"HOW","body":"Connect risk first, then sales."}]}`)
	hubSegment := func(pct float64, overrides string) SegmentInput {
		p := PatternInput{Name: "radial-hub", Values: values}
		if overrides != "" {
			p.Overrides = json.RawMessage(overrides)
		}
		return SegmentInput{SizePct: pct, Pattern: p}
	}
	compose := func(direction string, pct float64, overrides string) SlideInput {
		return SlideInput{Compose: &ComposeInput{Direction: direction, Segments: []SegmentInput{
			hubSegment(pct, overrides),
			{SizePct: 100 - pct, Pattern: PatternInput{Name: "labeled-rows", Values: rows}},
		}}}
	}
	keyed := func(sats []radialHubShape) int {
		n := 0
		for _, s := range sats {
			for _, key := range []string{"A", "B", "C", "D", "E", "F"} {
				if strings.Contains(s.xml, "<a:t>"+key+"</a:t>") {
					n++
				}
			}
		}
		return n
	}
	cases := []struct {
		name       string
		slide      SlideInput
		wantLegend int // -1 = either layout
	}{
		{"horizontal-50", compose("horizontal", 50, ""), -1},
		{"horizontal-60", compose("horizontal", 60, ""), 0},
		{"horizontal-35-falls-back-to-legend", compose("horizontal", 35, ""), 6},
		{"horizontal-50-legend", compose("horizontal", 50, `{"labels":"legend"}`), 6},
		{"horizontal-50-inside", compose("horizontal", 50, `{"labels":"inside"}`), 0},
		{"vertical-60", compose("vertical", 60, ""), 0},
	}
	for _, tpl := range radialHubTemplates(t) {
		for _, tc := range cases {
			t.Run(tpl+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				xml := radialHubSlideXML(t, tpl, tc.slide)
				hub, sats := assertRadialHubDrawing(t, strings.ReplaceAll(xml, "Core platform", "Customer"), 6, 6)
				if got := keyed(sats); tc.wantLegend >= 0 && got != tc.wantLegend {
					t.Errorf("%d keyed satellites, want %d", got, tc.wantLegend)
				}
				if hub.cx < 45*12700 {
					t.Errorf("hub is %.0fpt across: too small to read", float64(hub.cx)/12700)
				}
			})
		}
		t.Run(tpl+"/nested-cell", func(t *testing.T) {
			t.Parallel()
			grid := fmt.Sprintf(`{"columns":[55,45],"rows":[{"cells":[{"pattern":{"name":"radial-hub","values":%s}},{"shape":{"geometry":"rect","fill":"none","text":{"content":"One governed platform for every function"}}}]}]}`, values)
			slide := SlideInput{}
			if err := json.Unmarshal([]byte(`{"shape_grid":`+grid+`}`), &slide); err != nil {
				t.Fatal(err)
			}
			xml := radialHubSlideXML(t, tpl, slide)
			assertRadialHubDrawing(t, strings.ReplaceAll(xml, "Core platform", "Customer"), 6, 6)
		})
	}
}

// One rendered slide: midway along a spoke the page shows the accent, not the
// paper. Needs LibreOffice and pdftoppm; skipped without them and under
// -short.
func TestRadialHubSpokeRenderCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("renders through LibreOffice")
	}
	soffice, err := exec.LookPath("soffice")
	if err != nil {
		t.Skip("soffice not installed")
	}
	pdftoppm, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm not installed")
	}
	dir := t.TempDir()
	values, _ := json.Marshal(radialHubValues(8, 0, 0))
	title := "Eight functions around one platform"
	result, cleanup, err := RunPresentation(context.Background(), &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{{
		SlideType: "content", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		Pattern: &PatternInput{Name: "radial-hub", Values: values},
	}}}, RenderOptions{OutputDir: dir, TemplatesDir: testutil.TemplatesDir(), StrictFit: "off"})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	xml := readSlideXML(t, result.OutputPath, "ppt/slides/slide1.xml")
	presXML := readSlideXML(t, result.OutputPath, "ppt/presentation.xml")
	sld := regexp.MustCompile(`<p:sldSz\b[^>]*\bcx="(\d+)"[^>]*\bcy="(\d+)"`).FindStringSubmatch(presXML)
	if len(sld) != 3 {
		t.Fatal("no slide size")
	}
	slideW, _ := strconv.ParseFloat(sld[1], 64)
	slideH, _ := strconv.ParseFloat(sld[2], 64)

	profile := "-env:UserInstallation=file://" + filepath.Join(dir, "lo-profile")
	convert := exec.Command(soffice, profile, "--headless", "--convert-to", "pdf", "--outdir", dir, result.OutputPath) //nolint:gosec // a LookPath binary on a file this test wrote
	if out, err := convert.CombinedOutput(); err != nil {
		t.Skipf("soffice could not convert: %v: %s", err, out)
	}
	pdf := strings.TrimSuffix(result.OutputPath, ".pptx") + ".pdf"
	raster := exec.Command(pdftoppm, "-r", "150", "-png", "-singlefile", pdf, filepath.Join(dir, "slide")) //nolint:gosec // a LookPath binary on a file this test wrote
	if out, err := raster.CombinedOutput(); err != nil {
		t.Skipf("pdftoppm failed: %v: %s", err, out)
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
	b := img.Bounds()
	hub, sats := assertRadialHubDrawing(t, xml, 8, 8)
	checked := 0
	for _, r := range radialHubSpokes(t, xml, hub, sats) {
		// The middle of the frame is the middle of the spoke whatever its rotation.
		px := b.Min.X + int((float64(r.x)+float64(r.cx)/2)/slideW*float64(b.Dx()))
		py := b.Min.Y + int((float64(r.y)+float64(r.cy)/2)/slideH*float64(b.Dy()))
		coloured := false
		for dy := -2; dy <= 2 && !coloured; dy++ {
			for dx := -2; dx <= 2 && !coloured; dx++ {
				cr, cg, cb, _ := img.At(px+dx, py+dy).RGBA()
				lo, hi := cr, cr
				for _, c := range []uint32{cg, cb} {
					if c < lo {
						lo = c
					}
					if c > hi {
						hi = c
					}
				}
				coloured = (hi-lo)>>8 > 40 // the accent, not white paper or a grey disc
			}
		}
		if !coloured {
			t.Errorf("no accent pixel midway along the spoke at (%d, %d) of %dx%d", px, py, b.Dx(), b.Dy())
		}
		checked++
	}
	if checked != 8 {
		t.Errorf("checked %d spokes, want 8", checked)
	}
}

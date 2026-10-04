package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Render truth (go-slide-creator-bhbtk).
//
// Every fit decision in the engine is a prediction: text is measured with the
// engine's own metrics and written into a box the measure says holds it.
// LibreOffice — the renderer behind every preview and visual-QA pass — fits
// each normAutofit shape again with its own layout, and when it disagrees it
// shrinks the text and says so: converting the deck back to .pptx writes the
// fontScale / lnSpcReduction it applied onto the shape. Reading that back is
// the one check that does not share the engine's assumptions.
//
// TestRenderTruthExemplars round-trips the exemplar deck of every named
// pattern through LibreOffice and fails when a shape comes back font-shrunk.
// A shape that only had its line spacing tightened (lnSpcReduction, no
// fontScale) is logged: the text is drawn at the written size.
//
// It needs LibreOffice and the template's faces, takes about a minute per
// template, and so runs only where it is asked for:
//
//	RENDER_TRUTH_TEMPLATES="midnight-blue forest-green" go test ./cmd/json2pptx -run TestRenderTruthExemplars -count=1 -v
//
// CI's render-integration job runs it on the Calibri templates, which
// LibreOffice draws in the metric-compatible Carlito (fonts-crosextra-carlito).
// A template whose face the host substitutes with a wider one reports shrinks
// that are the host's, not the engine's: pick templates the host has fonts for.
// RENDER_TRUTH_KEEP=<dir> keeps the generated and round-tripped decks.

// rendererRefit is one shape LibreOffice re-fitted.
type rendererRefit struct {
	slide          int // 1-based
	fontScale      int // thousandths of a percent; 0 when the font was kept
	lnSpcReduction int
	text           string
}

var (
	refitShapeRe    = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	refitAutofitRe  = regexp.MustCompile(`<a:normAutofit([^>]*)/>`)
	refitScaleRe    = regexp.MustCompile(`fontScale="(\d+)"`)
	refitSpacingRe  = regexp.MustCompile(`lnSpcReduction="(\d+)"`)
	refitSlideRe    = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	refitFullScale  = 100000
	refitRoundTrips = 5 * time.Minute
)

// rendererRefits lists the shapes of a deck that carry a normAutofit shrink:
// in a deck LibreOffice wrote, the shapes it re-fitted.
func rendererRefits(pptxPath string) ([]rendererRefit, error) {
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var out []rendererRefit
	for _, f := range zr.File {
		m := refitSlideRe.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		slide, _ := strconv.Atoi(m[1])
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		for _, sp := range refitShapeRe.FindAllString(string(data), -1) {
			af := refitAutofitRe.FindStringSubmatch(sp)
			if af == nil {
				continue
			}
			r := rendererRefit{slide: slide}
			if s := refitScaleRe.FindStringSubmatch(af[1]); s != nil {
				if v, _ := strconv.Atoi(s[1]); v < refitFullScale {
					r.fontScale = v
				}
			}
			if s := refitSpacingRe.FindStringSubmatch(af[1]); s != nil {
				r.lnSpcReduction, _ = strconv.Atoi(s[1])
			}
			if r.fontScale == 0 && r.lnSpcReduction == 0 {
				continue
			}
			var text []string
			for _, tm := range shapeRunTextRe.FindAllStringSubmatch(sp, -1) {
				text = append(text, tm[1])
			}
			r.text = strings.Join(text, " ")
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].slide < out[j].slide })
	return out, nil
}

// The reader finds what LibreOffice writes, and only that: a bare
// normAutofit and a 100% scale are not shrinks. Runs everywhere.
func TestRendererRefitsReadsAutofitAttributes(t *testing.T) {
	shape := func(autofit, text string) string {
		return `<p:sp><p:txBody><a:bodyPr>` + autofit + `</a:bodyPr><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	slide := `<p:sld>` +
		shape(`<a:normAutofit/>`, "fits") +
		shape(`<a:normAutofit fontScale="100000"/>`, "full") +
		shape(`<a:normAutofit fontScale="92500" lnSpcReduction="10000"/>`, "shrunk") +
		shape(`<a:normAutofit lnSpcReduction="9999"/>`, "tight") +
		`</p:sld>`
	path := filepath.Join(t.TempDir(), "deck.pptx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{"ppt/slides/slide2.xml": slide, "ppt/slideLayouts/slideLayout1.xml": slide} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := rendererRefits(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []rendererRefit{
		{slide: 2, fontScale: 92500, lnSpcReduction: 10000, text: "shrunk"},
		{slide: 2, lnSpcReduction: 9999, text: "tight"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rendererRefits = %v, want %v", got, want)
	}
}

func TestRenderTruthExemplars(t *testing.T) {
	templates := strings.Fields(strings.ReplaceAll(os.Getenv("RENDER_TRUTH_TEMPLATES"), ",", " "))
	if len(templates) == 0 {
		t.Skip("RENDER_TRUTH_TEMPLATES unset: the LibreOffice round trip runs in CI's render-integration job")
	}
	office, err := render.OfficeCommand()
	if err != nil {
		t.Fatalf("RENDER_TRUTH_TEMPLATES is set but %v", err)
	}
	templatesDir := testutil.TemplatesDir()
	keep := os.Getenv("RENDER_TRUTH_KEEP")
	// One private profile for the run: conversions are sequential, and a
	// shared default profile is what hangs a headless LibreOffice.
	profile := t.TempDir()
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(templatesDir, tpl+".pptx")); err != nil {
				t.Fatalf("template %s: %v", tpl, err)
			}
			dir := t.TempDir()
			if keep != "" {
				dir = filepath.Join(keep, tpl)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			deck := exemplarDeck(t)
			deck.Template = tpl
			deck.OutputFilename = "exemplars.pptx"
			raw, err := json.Marshal(deck)
			if err != nil {
				t.Fatal(err)
			}
			inputPath := filepath.Join(dir, "input.json")
			if err := os.WriteFile(inputPath, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(dir, "result.json")
			if err := runJSONMode(inputPath, resultPath, templatesDir, dir, "", false, false, tpl, "off", false, "off", "free", false); err != nil {
				t.Fatalf("generate: %v", err)
			}
			resBytes, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			var out JSONOutput
			if err := json.Unmarshal(resBytes, &out); err != nil {
				t.Fatal(err)
			}
			if !out.Success {
				t.Fatalf("generation failed: %s", out.Error)
			}
			if stored, err := rendererRefits(out.OutputPath); err != nil {
				t.Fatal(err)
			} else if len(stored) > 0 {
				// A stored shrink would be read back as LibreOffice's own.
				t.Fatalf("the generated deck already stores %d autofit shrink(s), first on slide %d (%q)", len(stored), stored[0].slide, stored[0].text)
			}

			rtDir := filepath.Join(dir, "roundtrip")
			if err := os.MkdirAll(rtDir, 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), refitRoundTrips)
			defer cancel()
			// office is render.OfficeCommand's fixed name; the paths are the test's own.
			cmd := exec.CommandContext(ctx, office, "-env:UserInstallation=file://"+filepath.ToSlash(profile), //nolint:gosec // G204: fixed binary, test-owned paths
				"--headless", "--convert-to", "pptx", "--outdir", rtDir, out.OutputPath)
			if log, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("LibreOffice round trip: %v\n%s", err, log)
			}
			rtPath := filepath.Join(rtDir, filepath.Base(out.OutputPath))
			refits, err := rendererRefits(rtPath)
			if err != nil {
				t.Fatalf("read round-tripped deck: %v", err)
			}
			pattern := func(slide int) string {
				if slide >= 1 && slide <= len(deck.Slides) && deck.Slides[slide-1].Pattern != nil {
					return deck.Slides[slide-1].Pattern.Name
				}
				return "?"
			}
			for _, r := range refits {
				if r.fontScale > 0 {
					t.Errorf("%s slide %d (%s): LibreOffice shrinks %q to %.1f%% of its written size (line spacing -%.0f%%): the box does not hold its text at the size written",
						tpl, r.slide, pattern(r.slide), r.text, float64(r.fontScale)/1000, float64(r.lnSpcReduction)/1000)
					continue
				}
				t.Logf("%s slide %d (%s): LibreOffice tightens the line spacing of %q by %.0f%% (font kept)", tpl, r.slide, pattern(r.slide), r.text, float64(r.lnSpcReduction)/1000)
			}
		})
	}
}

package qualitybench

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReferenceDeck_AllCategoriesProduceValidDecks(t *testing.T) {
	for _, cat := range []string{"KPI", "comparison", "process", "narrative", "chart", "unknown"} {
		data, err := ReferenceDeck(Brief{ID: "b-" + cat, Category: cat, Prompt: "Create a deck about something important, with details: more"}, Template{Name: "midnight-blue"})
		if err != nil {
			t.Fatalf("%s: %v", cat, err)
		}
		var deck struct {
			Template string `json:"template"`
			Slides   []struct {
				LayoutID string `json:"layout_id"`
				Pattern  *struct {
					Name string `json:"name"`
				} `json:"pattern"`
			} `json:"slides"`
		}
		if err := json.Unmarshal(data, &deck); err != nil {
			t.Fatalf("%s: invalid JSON: %v", cat, err)
		}
		if deck.Template != "midnight-blue" || len(deck.Slides) != 3 || deck.Slides[1].Pattern == nil {
			t.Errorf("%s: unexpected deck %s", cat, data)
		}
	}
}

func TestReferenceDeckUsesExplicitTemplatePath(t *testing.T) {
	data, err := ReferenceDeck(Brief{ID: "fixture", Category: "KPI", Prompt: "Fixture"}, Template{Name: "held", Path: "/tmp/held.pptx"})
	if err != nil {
		t.Fatal(err)
	}
	var deck map[string]any
	if err := json.Unmarshal(data, &deck); err != nil {
		t.Fatal(err)
	}
	if deck["template_path"] != "/tmp/held.pptx" || deck["template"] != nil {
		t.Fatalf("deck template fields = %+v", deck)
	}
}

func TestBriefTitleTruncatesAtWordBoundary(t *testing.T) {
	got := briefTitle(Brief{Prompt: "Create an executive quarterly KPI snapshot with growth, margin, retention and cash metrics."})
	if !strings.HasSuffix(got, "…") || strings.Contains(got, "m…") || len([]rune(got)) > 60 {
		t.Errorf("title = %q", got)
	}
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.Black)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestContactSheetTilesSlides(t *testing.T) {
	dir := t.TempDir()
	var imgs []string
	for i := 0; i < 4; i++ {
		p := filepath.Join(dir, "s-"+string(rune('1'+i))+".png")
		writePNG(t, p, 320, 180)
		imgs = append(imgs, p)
	}
	out := filepath.Join(dir, "sheet.png")
	if err := ContactSheet(imgs, out, 3, 160); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(out)
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	// 3 columns x 2 rows of 160x90 thumbs + 12px gutters.
	if cfg.Width != 3*160+4*12 || cfg.Height != 2*90+3*12 {
		t.Errorf("sheet size = %dx%d", cfg.Width, cfg.Height)
	}
	if err := ContactSheet(nil, out, 3, 160); err == nil {
		t.Error("empty image list must error")
	}
}

func TestRatingsTemplateHasBlindIDsOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.csv")
	if err := writeRatingsTemplate(path, map[string]string{"bbb": "run-2", "aaa": "run-1"}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(path)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][0] != "aaa" || rows[2][0] != "bbb" || rows[0][2] != "reviewer_type" || rows[1][2] != "human" {
		t.Fatalf("rows = %v", rows)
	}
	for _, r := range rows {
		if strings.Contains(strings.Join(r, ","), "run-") {
			t.Errorf("ratings template leaks run IDs: %v", r)
		}
	}
}

func TestRendererResolve(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	fake := func(avail ...string) func(string) (string, error) {
		return func(name string) (string, error) {
			for _, a := range avail {
				if a == name {
					return name, nil
				}
			}
			return "", errors.New("missing")
		}
	}
	lookPath = fake("soffice", "pdftoppm")
	r := &Renderer{}
	if err := r.Resolve(); err != nil || r.office != "soffice" || !r.rasterIsPoppler {
		t.Errorf("soffice+pdftoppm: %v %+v", err, r)
	}
	lookPath = fake("pdftoppm")
	if err := (&Renderer{}).Resolve(); err == nil {
		t.Error("missing LibreOffice must error")
	}
	lookPath = fake("libreoffice")
	if err := (&Renderer{}).Resolve(); err == nil {
		t.Error("missing rasterizer must error")
	}
}

func TestDryRunRequiresBriefsAndTemplates(t *testing.T) {
	if _, _, err := (DryRun{Renderer: &Renderer{}}).Run(context.Background()); err == nil {
		t.Error("expected brief-count error")
	}
}

// TestDryRunEndToEnd runs the real pipeline when LibreOffice, a rasterizer
// and the Go toolchain are available (skipped in -short and on CI hosts
// without an office suite).
func TestDryRunEndToEnd(t *testing.T) {
	if testing.Short() || runtime.GOOS == "windows" {
		t.Skip("renders with LibreOffice")
	}
	r := &Renderer{}
	if err := r.Resolve(); err != nil {
		t.Skipf("render toolchain unavailable: %v", err)
	}
	root, _ := filepath.Abs("../..")
	dir := t.TempDir()
	bin := filepath.Join(dir, "json2pptx")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/json2pptx") // #nosec G204 -- fixed go toolchain command; bin is a fresh temp path
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build json2pptx: %v %s", err, out)
	}
	r.JSON2PPTX, r.TemplatesDir = bin, filepath.Join(root, "templates")
	briefs := make([]Brief, 12)
	for i := range briefs {
		briefs[i] = Brief{ID: "b" + string(rune('a'+i)), Category: []string{"KPI", "process", "chart"}[i%3], Prompt: "Brief"}
	}
	d := DryRun{Renderer: r, Briefs: briefs, Templates: []Template{{Name: "midnight-blue", Family: "corporate"}, {Name: "warm-coral", Family: "editorial"}, {Name: "business-template", Family: "seven-layout", HeldOut: true}}, OutputDir: filepath.Join(dir, "out"), ResultsDir: filepath.Join(dir, "results")}
	r.Density = 20 // thumbnails only; keeps the 36 renders fast
	report, path, err := d.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("results JSON missing: %v", err)
	}
	for _, ev := range report.Evidence {
		if ev.Error != "" || ev.ContactSheet == "" || ev.BlindID == "" {
			t.Errorf("run %s: err=%q sheet=%q", ev.Request.RunID, ev.Error, ev.ContactSheet)
		}
	}
}

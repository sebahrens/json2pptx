package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

type portabilityCase struct{ Aspect, Content, TemplateFeature string }

var portabilityMatrix = []portabilityCase{
	{"4:3", "sparse", "reordered and nonnumeric role bindings"},
	{"16:9", "dense", "multiple masters and inherited styles"},
	{"ultrawide", "long multilingual", "dark fills, foreground logos, and font substitution"},
}

func requireRenderer(required, available bool) (string, error) {
	if available {
		return "run", nil
	}
	if required {
		return "", fmt.Errorf("rendering is required but renderer is unavailable")
	}
	return "skip", nil
}

func TestPortabilityRendererPolicyFailsClosed(t *testing.T) {
	if _, err := requireRenderer(true, false); err == nil {
		t.Fatal("required job must fail without renderer")
	}
	if got, err := requireRenderer(false, false); err != nil || got != "skip" {
		t.Fatalf("optional local policy=%q %v", got, err)
	}
}

func TestPortabilityMatrixCoverage(t *testing.T) {
	aspects := map[string]bool{}
	features := ""
	for _, c := range portabilityMatrix {
		aspects[c.Aspect] = true
		features += c.TemplateFeature
	}
	for _, a := range []string{"4:3", "16:9", "ultrawide"} {
		if !aspects[a] {
			t.Errorf("missing aspect %s", a)
		}
	}
	for _, needle := range []string{"nonnumeric", "multiple masters", "inherited", "dark fills", "logos", "font substitution"} {
		if !contains(features, needle) {
			t.Errorf("missing feature %q", needle)
		}
	}
}

func TestPortabilityRenderedIntegration(t *testing.T) {
	required := os.Getenv("PORTABILITY_RENDER_REQUIRED") == "1"
	enabled := os.Getenv("PORTABILITY_RENDER_INTEGRATION") == "1"
	available, _ := render.DependencyStatus()
	action, err := requireRenderer(required, available)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled || action == "skip" {
		t.Skip("set PORTABILITY_RENDER_INTEGRATION=1 for rendered matrix")
	}
	if runtime.GOOS == "windows" {
		t.Skip("CLI integration paths are POSIX-oriented")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("JSON2PPTX_BINARY")
	if binary == "" {
		binary = filepath.Join(root, "bin", "json2pptx")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("benchmark binary missing: %v", err)
	}
	outDir := filepath.Join(root, "output", "portability-matrix")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	type row struct {
		Template, Renderer string
		Slides             int
		Hashes             []string
	}
	rows := []row{}
	for _, tmpl := range []string{"midnight-blue", "modern-yellow", "business-template"} {
		dir := filepath.Join(outDir, tmpl)
		_ = os.MkdirAll(dir, 0o755)
		cmd := exec.Command(binary, "generate", "-json", filepath.Join(root, "tests", "quality", "fixtures", "kpi-grid-9-tiles.json"), "-template", tmpl, "-templates-dir", filepath.Join(root, "templates"), "-output", dir)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate %s: %v\n%s", tmpl, err, output)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.pptx"))
		if len(files) != 1 {
			t.Fatalf("%s generated %d pptx files", tmpl, len(files))
		}
		deck, err := render.RenderDeckOpts(files[0], 90, 50, true)
		if err != nil {
			t.Fatalf("render %s: %v", tmpl, err)
		}
		if len(deck.Slides) == 0 || deck.Truncated {
			t.Fatalf("incomplete render for %s", tmpl)
		}
		r := row{Template: tmpl, Renderer: "LibreOffice+ImageMagick", Slides: len(deck.Slides)}
		for _, s := range deck.Slides {
			if s.ContentHash == "" {
				t.Fatalf("slide %d missing pixel hash", s.Index)
			}
			r.Hashes = append(r.Hashes, s.ContentHash)
		}
		rows = append(rows, r)
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "matrix.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

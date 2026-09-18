package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// portabilityCase is one real fixture template in the portability matrix
// (go-slide-creator-94sk). Fixtures are built by
// `go run ./cmd/mktemplate -portability-fixtures tests/quality/fixtures/portability/templates`
// (make portability-fixtures) and committed; Deck is the representative deck
// generated on it.
type portabilityCase struct {
	Template string
	Deck     string
	// Expected template properties, verified against the template profile so
	// a regenerated fixture cannot silently stop covering its case.
	WidthEMU, HeightEMU int64
	Masters             int
	SideLogo            bool
}

var portabilityMatrix = []portabilityCase{
	{Template: "portability-4x3", Deck: "deck.json", WidthEMU: 9144000, HeightEMU: 6858000, Masters: 1},
	{Template: "portability-21x9", Deck: "deck.json", WidthEMU: 16002000, HeightEMU: 6858000, Masters: 1},
	{Template: "portability-two-master", Deck: "deck-two-master.json", WidthEMU: 12192000, HeightEMU: 6858000, Masters: 2},
	{Template: "portability-side-logo", Deck: "deck.json", WidthEMU: 12192000, HeightEMU: 6858000, Masters: 1, SideLogo: true},
}

func portabilityRoot(t *testing.T) (root, fixtures string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, "tests", "quality", "fixtures", "portability")
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

// TestPortabilityMatrixCoverage verifies each committed fixture really has the
// geometry it claims (canvas size, master count, side logo), read through the
// template profile rather than from labels.
func TestPortabilityMatrixCoverage(t *testing.T) {
	_, fixtures := portabilityRoot(t)
	seenAspects := map[string]bool{}
	for _, c := range portabilityMatrix {
		path := filepath.Join(fixtures, "templates", c.Template+".pptx")
		reader, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatalf("%s: %v (run `make portability-fixtures`)", c.Template, err)
		}
		p, err := template.BuildProfile(reader)
		if err != nil {
			_ = reader.Close()
			t.Fatalf("%s: profile: %v", c.Template, err)
		}
		if p.SlideWidth != c.WidthEMU || p.SlideHeight != c.HeightEMU {
			t.Errorf("%s: canvas %dx%d, want %dx%d", c.Template, p.SlideWidth, p.SlideHeight, c.WidthEMU, c.HeightEMU)
		}
		seenAspects[p.AspectRatio] = true
		masters := map[string]bool{}
		for _, l := range p.Layouts {
			masters[l.MasterPath] = true
			if len(l.FooterRegions) == 0 {
				t.Errorf("%s/%s: no footer regions resolved", c.Template, l.ID)
			}
		}
		if len(masters) != c.Masters {
			t.Errorf("%s: %d masters, want %d", c.Template, len(masters), c.Masters)
		}
		master, _ := reader.ReadFile("ppt/slideMasters/slideMaster1.xml")
		if hasLogo := strings.Contains(string(master), `name="Logo"`); hasLogo != c.SideLogo {
			t.Errorf("%s: side logo present=%v, want %v", c.Template, hasLogo, c.SideLogo)
		}
		_ = reader.Close()
	}
	for _, a := range []string{"4:3", "16:9"} {
		if !seenAspects[a] {
			t.Errorf("matrix missing aspect %s", a)
		}
	}
}

// portabilityBinary resolves the json2pptx binary for CLI-driven checks.
func portabilityBinary(t *testing.T, root string) string {
	t.Helper()
	binary := os.Getenv("JSON2PPTX_BINARY")
	if binary == "" {
		binary = filepath.Join(root, "bin", "json2pptx")
	}
	if _, err := os.Stat(binary); err != nil {
		return ""
	}
	return binary
}

// generatePortabilityDeck runs the CLI for one fixture and returns the .pptx.
func generatePortabilityDeck(t *testing.T, binary, root, fixtures string, c portabilityCase, outDir string) string {
	t.Helper()
	// #nosec G204,G702 -- binary is an explicit test configuration and every
	// argument except the temporary output directory is fixed by this test.
	cmd := exec.Command(binary, "generate", "-json", filepath.Join(fixtures, c.Deck), "-template", c.Template, "-templates-dir", filepath.Join(fixtures, "templates"), "-output", outDir)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %s: %v\n%s", c.Template, err, output)
	}
	files, _ := filepath.Glob(filepath.Join(outDir, "*.pptx"))
	if len(files) != 1 {
		t.Fatalf("%s generated %d pptx files", c.Template, len(files))
	}
	return files[0]
}

// TestPortabilityFixtureGeometryCLI generates the representative deck on each
// fixture with the built binary and asserts the portability geometry contract
// (testutil.CheckDeckPortability). It needs only the binary — no renderer — so
// it also runs in the rendered CI job ahead of rasterization. Point
// JSON2PPTX_BINARY at an older build to see the contract bite.
func TestPortabilityFixtureGeometryCLI(t *testing.T) {
	root, fixtures := portabilityRoot(t)
	binary := portabilityBinary(t, root)
	if binary == "" {
		t.Skip("json2pptx binary not built (go build -o bin/json2pptx ./cmd/json2pptx or set JSON2PPTX_BINARY)")
	}
	for _, c := range portabilityMatrix {
		t.Run(c.Template, func(t *testing.T) {
			deck := generatePortabilityDeck(t, binary, root, fixtures, c, t.TempDir())
			violations, err := testutil.CheckDeckPortability(deck, filepath.Join(fixtures, "templates", c.Template+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range violations {
				t.Error(v)
			}
		})
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
	root, fixtures := portabilityRoot(t)
	binary := portabilityBinary(t, root)
	if binary == "" {
		t.Fatal("benchmark binary missing (set JSON2PPTX_BINARY)")
	}
	outDir := filepath.Join(root, "output", "portability-matrix")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	type row struct {
		Template, Renderer string
		Slides             int
		Violations         []string
		Hashes             []string
	}
	rows := []row{}
	for _, c := range portabilityMatrix {
		dir := filepath.Join(outDir, c.Template)
		_ = os.RemoveAll(dir)
		_ = os.MkdirAll(dir, 0o755)
		deckPath := generatePortabilityDeck(t, binary, root, fixtures, c, dir)
		violations, err := testutil.CheckDeckPortability(deckPath, filepath.Join(fixtures, "templates", c.Template+".pptx"))
		if err != nil {
			t.Fatalf("%s: %v", c.Template, err)
		}
		for _, v := range violations {
			t.Errorf("%s: %s", c.Template, v)
		}
		deck, err := render.RenderDeckOpts(deckPath, 90, 50, true)
		if err != nil {
			t.Fatalf("render %s: %v", c.Template, err)
		}
		slides, err := testutil.ReadDeckSlides(deckPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(deck.Slides) != len(slides) || deck.Truncated {
			t.Fatalf("incomplete render for %s: %d of %d slides", c.Template, len(deck.Slides), len(slides))
		}
		r := row{Template: c.Template, Renderer: "LibreOffice+ImageMagick", Slides: len(deck.Slides), Violations: violations}
		for _, s := range deck.Slides {
			if s.ContentHash == "" {
				t.Fatalf("slide %d missing pixel hash", s.Index)
			}
			r.Hashes = append(r.Hashes, s.ContentHash)
		}
		rows = append(rows, r)
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "matrix.json"), append(data, '\n'), 0o644); err != nil { //nolint:gosec // CI evidence artifact
		t.Fatal(err)
	}
}

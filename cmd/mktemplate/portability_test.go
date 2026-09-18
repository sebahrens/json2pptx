package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestPortabilityFixturesGenerate builds every portability fixture and checks
// it parses with the geometry it claims; it also verifies the committed
// fixtures are up to date with the generator.
func TestPortabilityFixturesGenerate(t *testing.T) {
	dir := t.TempDir()
	if err := generatePortabilityFixtures(dir); err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		w, h    int64
		masters int
		layouts int
	}{
		"4x3":        {9144000, 6858000, 1, 7},
		"21x9":       {16002000, 6858000, 1, 7},
		"two-master": {12192000, 6858000, 2, 14},
		"side-logo":  {12192000, 6858000, 1, 7},
	}
	for _, v := range portabilityVariants {
		path := filepath.Join(dir, "portability-"+v+".pptx")
		r, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		p, err := template.BuildProfile(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		masters := map[string]bool{}
		for _, l := range p.Layouts {
			masters[l.MasterPath] = true
		}
		w := want[v]
		if p.SlideWidth != w.w || p.SlideHeight != w.h || len(masters) != w.masters || len(p.Layouts) != w.layouts {
			t.Errorf("%s: %dx%d masters=%d layouts=%d, want %dx%d masters=%d layouts=%d",
				v, p.SlideWidth, p.SlideHeight, len(masters), len(p.Layouts), w.w, w.h, w.masters, w.layouts)
		}

		committed, err := os.ReadFile(filepath.Join("..", "..", "tests", "quality", "fixtures", "portability", "templates", "portability-"+v+".pptx"))
		if err != nil {
			t.Fatalf("committed fixture missing: %v (run make portability-fixtures)", err)
		}
		fresh, err := os.ReadFile(path) //nolint:gosec // test temp path
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(committed, fresh) {
			t.Errorf("%s: committed fixture is stale; run make portability-fixtures", v)
		}
	}
	if err := generateVariant(templates[0], "bogus", filepath.Join(dir, "x.pptx")); err == nil {
		t.Error("unknown variant must fail")
	}
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hardcodedRenderBinaryRe matches a shell-out to a render binary by name.
// internal/render resolves libreoffice OR soffice and rasterises with
// ImageMagick; a tool that names one of them itself is dead on any machine that
// installs it under the other name — audit_palette demanded a binary literally
// called "libreoffice" plus pdftoppm and so failed on every Homebrew macOS box
// while every other render tool worked (go-slide-creator-rdql, and
// go-slide-creator-71kj before it for pptx2jpg).
var hardcodedRenderBinaryRe = regexp.MustCompile(`exec\.(Command|CommandContext|LookPath)\([^)]*?"(libreoffice|soffice|pdftoppm|magick|convert)"`)

// renderBinaryOwners are the packages allowed to name a render binary: the
// render package itself resolves them, and pptx2jpg is the standalone
// conversion CLI that predates it.
var renderBinaryOwners = []string{
	filepath.Join("internal", "render"),
	filepath.Join("cmd", "pptx2jpg"),
}

func TestRenderBinariesResolveThroughOneHelper(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "testdata", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, owner := range renderBinaryOwners {
			if strings.Contains(path, owner) {
				return nil
			}
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // repo-relative walk
		if readErr != nil {
			return readErr
		}
		for _, m := range hardcodedRenderBinaryRe.FindAllStringSubmatch(string(data), -1) {
			offenders = append(offenders, path+": "+m[2])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("these files shell out to a render binary by name instead of going through internal/render "+
			"(render.DeckPNGs / render.DependencyStatus), so they break on a machine that installs it under "+
			"another name:\n  %s", strings.Join(offenders, "\n  "))
	}
}

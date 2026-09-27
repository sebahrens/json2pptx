package templatepreview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderingSourceHashTracksEmbeddedFontBytes(t *testing.T) {
	for _, extension := range []string{".ttf", ".otf"} {
		t.Run(extension, func(t *testing.T) {
			root := t.TempDir()
			for path, data := range map[string]string{
				"internal/templatepreview/recipe.go": "package recipe\n",
				"svggen/fonts/embed.go":              "package fonts\n",
				"cmd/templatepreviews/main.go":       "package main\n",
				"go.mod":                             "module fixture\n",
				"go.sum":                             "dependency\n",
				"svggen/fonts/Original" + extension:  "original embedded font bytes",
			} {
				full := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := RenderingSourceHash(root)
			if err != nil {
				t.Fatal(err)
			}
			font := filepath.Join(root, "svggen", "fonts", "Original"+extension)
			if err := os.WriteFile(font, []byte("changed embedded font bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			after, err := RenderingSourceHash(root)
			if err != nil {
				t.Fatal(err)
			}
			if after == before {
				t.Fatal("asset-only font change did not invalidate rendering-source provenance")
			}
		})
	}
}

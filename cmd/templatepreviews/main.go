// templatepreviews renders the committed per-layout template thumbnails
// (templates/previews/<template>/<layoutID>.png) that recommend_visual points
// agents at. Requires LibreOffice + ImageMagick.
//
// Usage:
//
//	go run ./cmd/templatepreviews -templates-dir templates -output templates/previews
//	make template-previews
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/sebahrens/json2pptx/internal/templatepreview"
)

func main() {
	templatesDir := flag.String("templates-dir", "templates", "Directory containing the template .pptx files")
	output := flag.String("output", "templates/previews", "Output directory for <template>/<layoutID>.png thumbnails")
	width := flag.Int("width", templatepreview.DefaultWidth, "Thumbnail width in pixels")
	dpi := flag.Int("dpi", 60, "Render density before downscaling")
	sourceRoot := flag.String("source-root", ".", "Repository root for rendering-source provenance")
	cacheDir := flag.String("cache-dir", "", "Optional retained full-resolution preview cache")
	flag.Parse()

	profile, err := os.MkdirTemp("", "templatepreviews-lo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(profile) }()

	counts, err := templatepreview.GenerateAll(*templatesDir, *output, templatepreview.Options{
		Width: *width, DPI: *dpi, LibreOfficeProfileDir: profile, SourceRoot: *sourceRoot, CacheDir: *cacheDir,
	})
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("%-20s %d layout previews\n", n, counts[n])
	}
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// runRenderSlide implements the "render-slide" CLI subcommand.
// It outputs the same response as the render_slide_image MCP tool.
func runRenderSlide() error {
	fs := flag.NewFlagSet("render-slide", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	pptxPath := fs.String("pptx", "", "Path to the PPTX file to render (required)")
	slideIndex := fs.Int("slide-index", 0, "0-based slide index to render")
	slideID := fs.String("slide-id", "", "Stable slide id to render instead of --slide-index (a DeckSpec slide's id; needs the <deck>.pptx.authoring.json sidecar 'semantic render' writes)")
	density := fs.Int("density", 100, "DPI for rendering (50-300)")
	force := fs.Bool("force", false, "Bypass render cache")
	outPath := fs.String("out", "", "Write the PNG to this file and print a manifest (path, sha256, bytes). Without it the manifest points at the render cache")
	base64Out := fs.Bool("base64", false, "Print the legacy JSON envelope with the image as png_base64 instead of a file manifest")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-slide <file.pptx> [--slide-index N | --slide-id ID] [--out slide.png] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a single slide from a PPTX to a PNG file and print a small manifest\n")
		fmt.Fprintf(os.Stderr, "(path, sha256, bytes, width, height) on stdout.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide output/deck.pptx --out slide-0.png\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide output/deck.pptx --slide-index 3 --density 200 --out slide-3.png\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if *pptxPath == "" {
		fs.Usage()
		return fmt.Errorf("--pptx is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"pptx_path": *pptxPath,
		"density":   float64(*density),
		"force":     *force,
	}
	if *slideID != "" {
		args["slide_id"] = *slideID
		// An explicit --slide-index beside it is the ambiguity the tool refuses.
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "slide-index" {
				args["slide_index"] = float64(*slideIndex)
			}
		})
	} else {
		args["slide_index"] = float64(*slideIndex)
	}
	if *base64Out {
		args[argIncludeBase64JSON] = true
	}

	result, err := mc.handleRenderSlideImage(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("render-slide: %w", err)
	}
	if *base64Out {
		return printMCPResultJSON(result)
	}
	return cliPrintRenderManifest(result, "", singleSlideDest(*outPath), false)
}

// singleSlideDest returns the destination mapper for a one-slide render: the
// file the caller named, or nil (leave it in the render cache) when none.
func singleSlideDest(outPath string) func(int) string {
	if outPath == "" {
		return nil
	}
	return func(int) string { return outPath }
}

// runRenderSlideFromJSON implements the "render-slide-from-json" CLI subcommand.
// It outputs the same response as the render_slide_image_from_json MCP tool.
func runRenderSlideFromJSON() error {
	fs := flag.NewFlagSet("render-slide-from-json", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")
	slidePath := fs.String("slide", "", "Path to a JSON file containing a single slide object (required, or use - for stdin)")
	density := fs.Int("density", 100, "DPI for rendering (50-300)")
	force := fs.Bool("force", false, "Bypass render cache")
	overlay := fs.Bool("overlay", false, "Composite shape_grid cell bounds + fit-finding badges onto the rendered PNG")
	outPath := fs.String("out", "", "Write the PNG to this file and print a manifest (path, sha256, bytes). Without it the manifest points at the render cache")
	base64Out := fs.Bool("base64", false, "Print the legacy JSON envelope with the image as png_base64 instead of a file manifest")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-slide-from-json <slide.json> --template <name> [--out slide.png] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a single slide directly from JSON to a PNG file, without generating the full deck,\n")
		fmt.Fprintf(os.Stderr, "and print a small manifest (path, sha256, bytes) on stdout.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide-from-json slide.json --template midnight-blue --out slide.png\n")
		fmt.Fprintf(os.Stderr, "  cat slide.json | json2pptx render-slide-from-json --template midnight-blue --slide -\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" {
		fs.Usage()
		return fmt.Errorf("--template is required")
	}
	if *slidePath == "" {
		fs.Usage()
		return fmt.Errorf("--slide is required")
	}

	var slideBytes []byte
	var err error
	if *slidePath == "-" {
		slideBytes, err = io.ReadAll(os.Stdin)
	} else {
		slideBytes, err = os.ReadFile(*slidePath)
	}
	if err != nil {
		return fmt.Errorf("read slide: %w", err)
	}

	var slideObj any
	if err := json.Unmarshal(slideBytes, &slideObj); err != nil {
		return fmt.Errorf("parse slide JSON: %w", err)
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"slide":    slideObj,
		"template": *templateName,
		"density":  float64(*density),
		"force":    *force,
		"overlay":  *overlay,
	}
	if *base64Out {
		args[argIncludeBase64JSON] = true
	}

	result, err := mc.handleRenderSlideImageFromJSON(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("render-slide-from-json: %w", err)
	}
	if *base64Out {
		return printMCPResultJSON(result)
	}
	return cliPrintRenderManifest(result, "", singleSlideDest(*outPath), false)
}

// runRenderThumbnails implements the "render-thumbnails" CLI subcommand: the
// render_deck_thumbnails MCP tool, delivered as PNG files plus a manifest.
func runRenderThumbnails() error {
	fs := flag.NewFlagSet("render-thumbnails", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	pptxPath := fs.String("pptx", "", "Path to the PPTX file to render (required)")
	density := fs.Int("density", 50, "DPI for thumbnails (25-150)")
	maxSlides := fs.Int("max-slides", 50, "Maximum number of slides to render, counting from the first")
	slides := fs.String("slides", "", "Render only these slides, comma-separated: 0-based indices or slide ids (e.g. 1,3 or costs,s4). Mutually exclusive with --max-slides")
	force := fs.Bool("force", false, "Bypass render cache")
	knownHashes := fs.String("known-hashes", "", "Comma-separated content_hash values from an earlier manifest: a slide whose pixels still hash to one is listed as unchanged and its file is not rewritten")
	outDir := fs.String("out-dir", "", "Write slide-<index>.png files (0-based, the names 'inspect --images' reads) into this directory and print a manifest (paths, sha256, sizes). Without it the manifest points at the render cache")
	base64Out := fs.Bool("base64", false, "Print the legacy JSON envelope with every image as png_base64 instead of a file manifest")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-thumbnails <file.pptx> --out-dir <dir> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render the slides of a PPTX as PNG files and print a small manifest\n")
		fmt.Fprintf(os.Stderr, "(path, sha256, bytes, width, height per slide) on stdout.\n")
		fmt.Fprintf(os.Stderr, "Files are named slide-0.png, slide-1.png, ... - what 'json2pptx inspect <dir>' reads.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails output/deck.pptx --out-dir slides/\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails output/deck.pptx --out-dir slides/ --density 100 --max-slides 10\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails output/deck.pptx --out-dir slides/ --slides 1,3\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails output/deck.pptx --out-dir slides/ --known-hashes <content_hash>,<content_hash>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if *pptxPath == "" {
		fs.Usage()
		return fmt.Errorf("--pptx is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"pptx_path": *pptxPath,
		"density":   float64(*density),
		"force":     *force,
	}
	if *base64Out {
		args[argIncludeBase64JSON] = true
	}
	if *knownHashes != "" {
		hashes := []any{}
		for _, h := range strings.Split(*knownHashes, ",") {
			hashes = append(hashes, h)
		}
		args[argKnownHashes] = hashes
	}
	// --slides names slides; --max-slides caps a prefix. The MCP tool refuses
	// both at once, so send whichever the caller asked for.
	if *slides != "" {
		indices, err := parseSlideList(*slides)
		if err != nil {
			return fmt.Errorf("render-thumbnails: --slides: %w", err)
		}
		args["slide_indices"] = indices
	} else {
		args["max_slides"] = float64(*maxSlides)
	}

	result, err := mc.handleRenderDeckThumbnails(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("render-thumbnails: %w", err)
	}
	if *base64Out {
		return printMCPResultJSON(result)
	}
	if *outDir == "" {
		return cliPrintRenderManifest(result, "", nil, false)
	}
	dir := *outDir
	// A subset render (--slides) adds to the directory; a deck render owns it,
	// so slide files a longer earlier deck left behind are removed.
	return cliPrintRenderManifest(result, dir, func(index int) string {
		return filepath.Join(dir, fmt.Sprintf("slide-%d.png", index))
	}, *slides == "")
}

// parseSlideList parses a comma-separated list of slides, the CLI spelling of
// the render_deck_thumbnails slide_indices argument: a number is a 0-based
// index, anything else a slide id (go-slide-creator-1w3uo).
func parseSlideList(v string) ([]any, error) {
	out := make([]any, 0, 4)
	for _, field := range strings.Split(v, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if n, err := strconv.Atoi(field); err == nil {
			out = append(out, float64(n))
			continue
		}
		out = append(out, field)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no slides given")
	}
	return out, nil
}

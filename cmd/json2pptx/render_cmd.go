package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	density := fs.Int("density", 100, "DPI for rendering (50-300)")
	force := fs.Bool("force", false, "Bypass render cache")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-slide --pptx <file.pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a single slide from a PPTX to a PNG image.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide --pptx output/deck.pptx\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide --pptx output/deck.pptx --slide-index 3 --density 200\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *pptxPath == "" {
		fs.Usage()
		return fmt.Errorf("--pptx is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"pptx_path":   *pptxPath,
		"slide_index": float64(*slideIndex),
		"density":     float64(*density),
		"force":       *force,
		// The CLI prints JSON to stdout; keep the base64/path envelope.
		argIncludeBase64JSON: true,
	}

	result, err := mc.handleRenderSlideImage(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("render-slide: %w", err)
	}

	return printMCPResultJSON(result)
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

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-slide-from-json --template <name> --slide <file.json> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a single slide directly from JSON to a PNG, without generating the full deck.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-slide-from-json --template midnight-blue --slide slide.json\n")
		fmt.Fprintf(os.Stderr, "  cat slide.json | json2pptx render-slide-from-json --template midnight-blue --slide -\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
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
		// The CLI prints JSON to stdout; keep the base64/path envelope.
		argIncludeBase64JSON: true,
	}

	result, err := mc.handleRenderSlideImageFromJSON(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("render-slide-from-json: %w", err)
	}

	return printMCPResultJSON(result)
}

// runRenderThumbnails implements the "render-thumbnails" CLI subcommand.
// It outputs the same response as the render_deck_thumbnails MCP tool.
func runRenderThumbnails() error {
	fs := flag.NewFlagSet("render-thumbnails", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	pptxPath := fs.String("pptx", "", "Path to the PPTX file to render (required)")
	density := fs.Int("density", 50, "DPI for thumbnails (25-150)")
	maxSlides := fs.Int("max-slides", 50, "Maximum number of slides to render, counting from the first")
	slides := fs.String("slides", "", "Render only these 0-based slides, comma-separated (e.g. 1,3). Mutually exclusive with --max-slides")
	force := fs.Bool("force", false, "Bypass render cache")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx render-thumbnails --pptx <file.pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render all slides in a PPTX as low-resolution PNG thumbnails.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails --pptx output/deck.pptx\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails --pptx output/deck.pptx --density 100 --max-slides 10\n")
		fmt.Fprintf(os.Stderr, "  json2pptx render-thumbnails --pptx output/deck.pptx --slides 1,3\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
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
		// The CLI prints JSON to stdout; keep the base64/path envelope.
		argIncludeBase64JSON: true,
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

	return printMCPResultJSON(result)
}

// parseSlideList parses a comma-separated list of 0-based slide numbers, the CLI
// spelling of the render_deck_thumbnails slide_indices argument.
func parseSlideList(v string) ([]any, error) {
	out := make([]any, 0, 4)
	for _, field := range strings.Split(v, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("%q is not a slide number", field)
		}
		out = append(out, float64(n))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no slide numbers given")
	}
	return out, nil
}

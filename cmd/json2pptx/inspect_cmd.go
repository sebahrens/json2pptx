package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// runInspect implements the "inspect" CLI subcommand — runs visual QA on
// rendered slide images via the same handler as the inspect_slide_images MCP
// tool. Outputs the JSON Report on stdout.
func runInspect() error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)

	imagesDir := fs.String("images", "", "Directory containing slide PNG/JPG images (slide-0.png, slide-1.png, ...)")
	templateName := fs.String("template", "", "Template name to echo back on the report (optional)")
	model := fs.String("model", "", "Claude model override (default: claude-haiku-4-5-20251001)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx inspect --images <dir> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Run vision-based visual QA on rendered slide images.\n\n")
		fmt.Fprintf(os.Stderr, "Requires ANTHROPIC_API_KEY in the environment.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx inspect --images /tmp/slides/\n")
		fmt.Fprintf(os.Stderr, "  json2pptx inspect --images /tmp/slides/ --template midnight-blue\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *imagesDir == "" {
		fs.Usage()
		return fmt.Errorf("--images is required")
	}

	images, err := collectInspectImages(*imagesDir)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return fmt.Errorf("no slide images found in %s (looked for .png/.jpg/.jpeg)", *imagesDir)
	}

	args := map[string]any{
		"slide_images": images,
	}
	if *templateName != "" {
		args["deck_metadata"] = map[string]any{"template": *templateName}
	}
	if *model != "" {
		args["model"] = *model
	}

	mc := cliMCPConfig("./templates", "")
	result, err := mc.handleInspectSlideImages(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	return printMCPResultJSON(result)
}

// collectInspectImages lists image files in dir, sorted by name, and returns
// the slide_images input entries (absolute path form) for the inspect handler.
func collectInspectImages(dir string) ([]map[string]any, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve images dir: %w", err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read images dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") {
			files = append(files, filepath.Join(abs, e.Name()))
		}
	}
	sort.Strings(files)
	out := make([]map[string]any, len(files))
	for i, p := range files {
		out[i] = map[string]any{
			"index": i,
			"path":  p,
		}
	}
	return out, nil
}

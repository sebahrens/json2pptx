package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// runPreviewWireframe implements the "preview-wireframe" CLI subcommand.
// It mirrors the preview_slide_wireframe MCP tool: takes a presentation
// JSON + a slide index and emits an annotated SVG (and/or PNG) wireframe
// showing placeholders, grid cells, occupancy, and fit findings.
//
// Output modes:
//   --format=svg  → writes raw SVG to stdout (or to --out file).
//   --format=png  → writes raw PNG to stdout (or to --out file).
//   --format=both → writes the JSON envelope (svg + base64 PNG) to stdout.
func runPreviewWireframe() error {
	fs := flag.NewFlagSet("preview-wireframe", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonPath := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	slideIdx := fs.Int("slide", 0, "0-based slide index to render")
	format := fs.String("format", "svg", "Output format: svg, png, or both")
	widthPx := fs.Int("width-px", 960, "Canvas width in pixels (clamped 320..2400)")
	outPath := fs.String("out", "", "Output path; when set with --format=svg|png, writes the raw bytes to this file. Ignored for --format=both.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preview-wireframe --json <file.json> --slide <n> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render an annotated wireframe of one slide's resolved plan as SVG / PNG.\n")
		fmt.Fprintf(os.Stderr, "No LibreOffice / ImageMagick required — rendered in-process via svggen.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview-wireframe --json slides.json --slide 0 > slide0.svg\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview-wireframe --json slides.json --slide 2 --format png --out slide2.png\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview-wireframe --json slides.json --slide 1 --format both\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *jsonPath == "" {
		fs.Usage()
		return fmt.Errorf("--json is required")
	}
	switch *format {
	case "svg", "png", "both":
	default:
		return fmt.Errorf("--format must be one of svg, png, both (got %q)", *format)
	}

	presentation, err := readJSONObject(*jsonPath)
	if err != nil {
		return err
	}

	mc := cliMCPConfig(*templatesDir, "")
	args := map[string]any{
		"presentation": presentation,
		"slide_index":  float64(*slideIdx),
		"format":       *format,
		"width_px":     float64(*widthPx),
	}
	result, err := mc.handlePreviewSlideWireframe(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("preview-wireframe: %w", err)
	}

	body := extractMCPTextContent(result)
	if body == "" {
		return fmt.Errorf("preview-wireframe: empty response")
	}
	if result.IsError {
		// Forward structured error JSON to stdout for shell scripts.
		_, _ = os.Stdout.WriteString(body)
		return fmt.Errorf("preview-wireframe: handler reported an error")
	}

	// For svg / png output modes, extract raw bytes from the envelope so
	// callers can pipe them directly into a viewer / file.
	if *format == "svg" || *format == "png" {
		var env struct {
			SVG   string `json:"svg"`
			PNG64 string `json:"png_base64"`
		}
		if err := json.Unmarshal([]byte(body), &env); err != nil {
			return fmt.Errorf("preview-wireframe: parse response: %w", err)
		}
		var payload []byte
		if *format == "svg" {
			payload = []byte(env.SVG)
		} else {
			decoded, decErr := base64.StdEncoding.DecodeString(env.PNG64)
			if decErr != nil {
				return fmt.Errorf("preview-wireframe: decode png: %w", decErr)
			}
			payload = decoded
		}
		if *outPath == "" {
			_, err := os.Stdout.Write(payload)
			return err
		}
		return os.WriteFile(*outPath, payload, 0o644) //nolint:gosec // user-specified output path
	}

	// format=both: emit the full JSON envelope.
	return printMCPResultJSON(result)
}

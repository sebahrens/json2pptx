// Package main provides a CLI tool for rendering SVG diagrams.
//
// Usage:
//
//	svggen render -i input.json -o output.svg
//	svggen types
//	svggen validate -i input.json
//	svggen batch -i inputs/ -o outputs/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	// Import root package to auto-register all diagram types via init().
	"github.com/ahrens/svggen"
)

// Exit codes.
const (
	exitOK         = 0
	exitRender     = 1
	exitValidation = 2
	exitIO         = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitRender
	}

	switch args[0] {
	case "render":
		return cmdRender(args[1:], stdin, stdout, stderr)
	case "types":
		return cmdTypes(args[1:], stdout, stderr)
	case "validate":
		return cmdValidate(args[1:], stdin, stderr)
	case "batch":
		return cmdBatch(args[1:], stderr)
	case "help", "--help", "-h":
		printUsage(stdout)
		return exitOK
	case "version", "--version":
		fmt.Fprintln(stdout, "svggen 0.1.0")
		return exitOK
	default:
		fmt.Fprintf(stderr, "svggen: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return exitRender
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: svggen <command> [flags]

Commands:
  render     Render a diagram to SVG, PNG, or PDF
  types      List all registered diagram types
  validate   Validate input without rendering
  batch      Batch render multiple diagrams
  version    Print version information
  help       Show this help

Run "svggen <command> --help" for details on each command.`)
}

// --- render ---

type renderFlags struct {
	input  string
	output string
	format string
	typ    string
	data   string
	width  int
	height int
	style  string
	theme  string
}

func parseRenderFlags(args []string) (*renderFlags, error) {
	f := &renderFlags{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for %s", args[i-1])
			}
			f.input = args[i]
		case "-o", "--output":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for %s", args[i-1])
			}
			f.output = args[i]
		case "--format":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --format")
			}
			f.format = args[i]
		case "--type":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --type")
			}
			f.typ = args[i]
		case "--data":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --data")
			}
			f.data = args[i]
		case "--width":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --width")
			}
			if _, err := fmt.Sscanf(args[i], "%d", &f.width); err != nil {
				return nil, fmt.Errorf("invalid --width: %s", args[i])
			}
		case "--height":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --height")
			}
			if _, err := fmt.Sscanf(args[i], "%d", &f.height); err != nil {
				return nil, fmt.Errorf("invalid --height: %s", args[i])
			}
		case "--style":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --style")
			}
			f.style = args[i]
		case "--theme":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing value for --theme")
			}
			f.theme = args[i]
		case "-h", "--help":
			return nil, nil // signal to print help
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return f, nil
}

func cmdRender(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, err := parseRenderFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "svggen render: %v\n", err)
		return exitRender
	}
	if f == nil {
		printRenderHelp(stdout)
		return exitOK
	}

	// Read input
	var inputData []byte
	if f.data != "" {
		// Build envelope from --type and --data flags
		if f.typ == "" {
			fmt.Fprintln(stderr, "svggen render: --type is required when using --data")
			return exitValidation
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(f.data), &data); err != nil {
			fmt.Fprintf(stderr, "svggen render: invalid --data JSON: %v\n", err)
			return exitValidation
		}
		envelope := map[string]any{
			"type": f.typ,
			"data": data,
		}
		inputData, _ = json.Marshal(envelope)
	} else if f.input != "" {
		inputData, err = os.ReadFile(f.input)
		if err != nil {
			fmt.Fprintf(stderr, "svggen render: %v\n", err)
			return exitIO
		}
	} else {
		// Read from stdin
		inputData, err = io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "svggen render: reading stdin: %v\n", err)
			return exitIO
		}
	}

	if len(inputData) == 0 {
		fmt.Fprintln(stderr, "svggen render: no input provided")
		return exitIO
	}

	// Decode
	decoder := svggen.NewDecoder(svggen.DefaultDecodeOptions())
	req, err := decoder.Decode(inputData)
	if err != nil {
		fmt.Fprintf(stderr, "svggen render: %v\n", err)
		if svggen.IsValidationError(err) {
			return exitValidation
		}
		return exitRender
	}

	// Apply flag overrides
	if f.typ != "" && f.data == "" {
		req.Type = f.typ
	}
	if f.width > 0 {
		req.Output.Width = f.width
	}
	if f.height > 0 {
		req.Output.Height = f.height
	}
	if f.style != "" {
		req.Style.Palette = svggen.PaletteSpec{Name: f.style}
	}
	if f.theme != "" {
		var palette svggen.PaletteSpec
		if err := json.Unmarshal([]byte(f.theme), &palette); err != nil {
			// Try as array of colors
			var colors []string
			if err2 := json.Unmarshal([]byte(f.theme), &colors); err2 == nil {
				palette.Colors = colors
			} else {
				// Try as palette name string
				palette.Name = f.theme
			}
		}
		req.Style.Palette = palette
	}

	// Determine output format
	format := f.format
	if format == "" && f.output != "" {
		ext := strings.TrimPrefix(filepath.Ext(f.output), ".")
		switch ext {
		case "svg", "png", "pdf":
			format = ext
		}
	}
	if format == "" {
		format = "svg"
	}

	// Render
	result, err := svggen.RenderMultiFormat(req, format)
	if err != nil {
		fmt.Fprintf(stderr, "svggen render: %v\n", err)
		if svggen.IsValidationError(err) {
			return exitValidation
		}
		return exitRender
	}

	// Get output bytes
	var outputBytes []byte
	switch format {
	case "svg":
		outputBytes = []byte(result.SVG.Content)
	case "png":
		outputBytes = result.PNG
	case "pdf":
		outputBytes = result.PDF
	default:
		fmt.Fprintf(stderr, "svggen render: unsupported format %q\n", format)
		return exitRender
	}

	if outputBytes == nil {
		fmt.Fprintf(stderr, "svggen render: no %s output generated\n", format)
		return exitRender
	}

	// Write output
	if f.output != "" {
		if err := os.WriteFile(f.output, outputBytes, 0644); err != nil {
			fmt.Fprintf(stderr, "svggen render: %v\n", err)
			return exitIO
		}
	} else {
		if _, err := stdout.Write(outputBytes); err != nil {
			fmt.Fprintf(stderr, "svggen render: writing stdout: %v\n", err)
			return exitIO
		}
	}

	return exitOK
}

func printRenderHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: svggen render [flags]

Render a diagram to SVG, PNG, or PDF.

Flags:
  -i, --input FILE    Input file (JSON or YAML)
  -o, --output FILE   Output file (format inferred from extension)
  --format FORMAT     Output format: svg (default), png, pdf
  --type TYPE         Diagram type (overrides input file)
  --data JSON         Inline data JSON (requires --type)
  --width N           Output width in pixels
  --height N          Output height in pixels
  --style STYLE       Style preset name
  --theme JSON        Theme/palette override (name or color array)

If -i is not provided, reads from stdin.
If -o is not provided, writes to stdout.`)
}

// --- types ---

func cmdTypes(args []string, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "-h", "--help":
			fmt.Fprintln(stdout, `Usage: svggen types [--json]

List all registered diagram types.

Flags:
  --json    Output as JSON array`)
			return exitOK
		default:
			fmt.Fprintf(stderr, "svggen types: unknown flag: %s\n", a)
			return exitRender
		}
	}

	types := svggen.Types()
	sort.Strings(types)

	if jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(types); err != nil {
			fmt.Fprintf(stderr, "svggen types: %v\n", err)
			return exitRender
		}
	} else {
		for _, t := range types {
			fmt.Fprintln(stdout, t)
		}
	}

	return exitOK
}

// --- validate ---

func cmdValidate(args []string, stdin io.Reader, stderr io.Writer) int {
	var input string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "svggen validate: missing value for %s\n", args[i-1])
				return exitValidation
			}
			input = args[i]
		case "-h", "--help":
			fmt.Fprintln(stderr, `Usage: svggen validate [flags]

Validate input without rendering.

Flags:
  -i, --input FILE    Input file (JSON or YAML)

If -i is not provided, reads from stdin.
Exit codes: 0=valid, 2=invalid`)
			return exitOK
		default:
			fmt.Fprintf(stderr, "svggen validate: unknown flag: %s\n", args[i])
			return exitValidation
		}
	}

	var data []byte
	var err error
	if input != "" {
		data, err = os.ReadFile(input)
		if err != nil {
			fmt.Fprintf(stderr, "svggen validate: %v\n", err)
			return exitIO
		}
	} else {
		data, err = io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "svggen validate: reading stdin: %v\n", err)
			return exitIO
		}
	}

	if len(data) == 0 {
		fmt.Fprintln(stderr, "svggen validate: no input provided")
		return exitIO
	}

	decoder := svggen.NewDecoder(svggen.DecodeOptions{
		Strict:   true,
		Format:   "auto",
		Defaults: true,
	})
	req, err := decoder.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "svggen validate: %v\n", err)
		return exitValidation
	}

	// Also validate against the diagram's own validator
	d := svggen.DefaultRegistry().Get(req.Type)
	if d == nil {
		fmt.Fprintf(stderr, "svggen validate: unknown diagram type %q\n", req.Type)
		return exitValidation
	}
	if err := d.Validate(req); err != nil {
		fmt.Fprintf(stderr, "svggen validate: %v\n", err)
		return exitValidation
	}

	fmt.Fprintln(stderr, "valid")
	return exitOK
}

// --- batch ---

func cmdBatch(args []string, stderr io.Writer) int {
	var inputPath, outputPath, format string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "svggen batch: missing value for %s\n", args[i-1])
				return exitRender
			}
			inputPath = args[i]
		case "-o", "--output":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "svggen batch: missing value for %s\n", args[i-1])
				return exitRender
			}
			outputPath = args[i]
		case "--format":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "svggen batch: missing value for --format\n")
				return exitRender
			}
			format = args[i]
		case "-h", "--help":
			fmt.Fprintln(stderr, `Usage: svggen batch [flags]

Batch render multiple diagrams from a directory or JSONL file.

Flags:
  -i, --input PATH    Input directory or .jsonl file (required)
  -o, --output DIR    Output directory (required)
  --format FORMAT     Output format: svg (default), png, pdf`)
			return exitOK
		default:
			fmt.Fprintf(stderr, "svggen batch: unknown flag: %s\n", args[i])
			return exitRender
		}
	}

	if inputPath == "" || outputPath == "" {
		fmt.Fprintln(stderr, "svggen batch: -i and -o are required")
		return exitRender
	}
	if format == "" {
		format = "svg"
	}

	// Create output directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		fmt.Fprintf(stderr, "svggen batch: %v\n", err)
		return exitIO
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		fmt.Fprintf(stderr, "svggen batch: %v\n", err)
		return exitIO
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(inputPath)
		if err != nil {
			fmt.Fprintf(stderr, "svggen batch: %v\n", err)
			return exitIO
		}
		for _, e := range entries {
			ext := filepath.Ext(e.Name())
			if ext == ".json" || ext == ".yaml" || ext == ".yml" {
				files = append(files, filepath.Join(inputPath, e.Name()))
			}
		}
	} else if filepath.Ext(inputPath) == ".jsonl" {
		// JSONL: each line is a separate request
		data, err := os.ReadFile(inputPath)
		if err != nil {
			fmt.Fprintf(stderr, "svggen batch: %v\n", err)
			return exitIO
		}
		return batchRenderJSONL(data, outputPath, format, stderr)
	} else {
		fmt.Fprintln(stderr, "svggen batch: input must be a directory or .jsonl file")
		return exitRender
	}

	if len(files) == 0 {
		fmt.Fprintln(stderr, "svggen batch: no input files found")
		return exitRender
	}

	decoder := svggen.NewDecoder(svggen.DefaultDecodeOptions())
	errCount := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(stderr, "  ERROR %s: %v\n", filepath.Base(file), err)
			errCount++
			continue
		}

		req, err := decoder.Decode(data)
		if err != nil {
			fmt.Fprintf(stderr, "  ERROR %s: %v\n", filepath.Base(file), err)
			errCount++
			continue
		}

		result, err := svggen.RenderMultiFormat(req, format)
		if err != nil {
			fmt.Fprintf(stderr, "  ERROR %s: %v\n", filepath.Base(file), err)
			errCount++
			continue
		}

		baseName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		outFile := filepath.Join(outputPath, baseName+"."+format)
		if err := writeResult(outFile, format, result); err != nil {
			fmt.Fprintf(stderr, "  ERROR %s: %v\n", filepath.Base(file), err)
			errCount++
			continue
		}
		fmt.Fprintf(stderr, "  OK %s -> %s\n", filepath.Base(file), filepath.Base(outFile))
	}

	if errCount > 0 {
		fmt.Fprintf(stderr, "svggen batch: %d/%d failed\n", errCount, len(files))
		return exitRender
	}
	fmt.Fprintf(stderr, "svggen batch: %d files rendered\n", len(files))
	return exitOK
}

func batchRenderJSONL(data []byte, outputPath, format string, stderr io.Writer) int {
	decoder := svggen.NewDecoder(svggen.DefaultDecodeOptions())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	errCount := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		req, err := decoder.Decode([]byte(line))
		if err != nil {
			fmt.Fprintf(stderr, "  ERROR line %d: %v\n", i+1, err)
			errCount++
			continue
		}

		result, err := svggen.RenderMultiFormat(req, format)
		if err != nil {
			fmt.Fprintf(stderr, "  ERROR line %d (%s): %v\n", i+1, req.Type, err)
			errCount++
			continue
		}

		outFile := filepath.Join(outputPath, fmt.Sprintf("%03d_%s.%s", i, req.Type, format))
		if err := writeResult(outFile, format, result); err != nil {
			fmt.Fprintf(stderr, "  ERROR line %d: %v\n", i+1, err)
			errCount++
			continue
		}
		fmt.Fprintf(stderr, "  OK line %d -> %s\n", i+1, filepath.Base(outFile))
	}

	total := len(lines)
	if errCount > 0 {
		fmt.Fprintf(stderr, "svggen batch: %d/%d failed\n", errCount, total)
		return exitRender
	}
	fmt.Fprintf(stderr, "svggen batch: %d diagrams rendered\n", total)
	return exitOK
}

func writeResult(path, format string, result *svggen.RenderResult) error {
	var data []byte
	switch format {
	case "svg":
		data = []byte(result.SVG.Content)
	case "png":
		data = result.PNG
	case "pdf":
		data = result.PDF
	}
	if data == nil {
		return fmt.Errorf("no %s output generated", format)
	}
	return os.WriteFile(path, data, 0644)
}

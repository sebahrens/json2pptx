// preview_icon_cmd.go provides the CLI counterpart of the preview_icon MCP
// tool. It accepts a single IconInput JSON blob (file or "-" for stdin), or
// individual flags, and writes the resolved SVG + base64 PNG to stdout as
// JSON or to disk via --out-svg / --out-png.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func runPreviewIcon() error {
	fs := flag.NewFlagSet("preview-icon", flag.ExitOnError)
	iconFile := fs.String("icon", "", "Path to a JSON file containing an IconInput object (or '-' for stdin). Mutually exclusive with --name/--path/--url/--svg-data.")
	name := fs.String("name", "", "Bundled icon name (e.g. 'filled:chart-pie').")
	path := fs.String("path", "", "Filesystem path to a .svg icon (relative paths resolve against --base-dir).")
	url := fs.String("url", "", "HTTP/HTTPS URL to a remote .svg icon.")
	svgData := fs.String("svg-data", "", "Inline SVG markup.")
	fill := fs.String("fill", "", "Optional hex fill (e.g. '#FF0000'). Ignored for inline svg-data.")
	alt := fs.String("alt", "", "Optional alt text.")
	sizePx := fs.Int("size-px", previewIconDefaultSize, "Target pixel size for the PNG preview (clamped to [16, 1024]).")
	baseDir := fs.String("base-dir", "", "Absolute directory used to resolve a relative icon path.")
	outSVG := fs.String("out-svg", "", "Optional path to write the resolved SVG bytes (default: include svg_data in JSON output).")
	outPNG := fs.String("out-png", "", "Optional path to write the PNG preview (default: include png_base64 in JSON output).")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preview-icon [--icon <file>|-] [--name|--path|--url|--svg-data ...] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a single icon spec to SVG + PNG without building a deck.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview-icon --name filled:chart-pie --fill '#FF0000'\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview-icon --path ./logo.svg --base-dir /tmp\n")
		fmt.Fprintf(os.Stderr, "  echo '{\"name\":\"abacus\"}' | json2pptx preview-icon --icon -\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	iconArg, err := buildPreviewIconArg(*iconFile, *name, *path, *url, *svgData, *fill, *alt)
	if err != nil {
		return err
	}

	args := map[string]any{
		"icon":    iconArg,
		"size_px": float64(*sizePx),
	}
	if *baseDir != "" {
		args["base_dir"] = *baseDir
	}

	mc := &mcpConfig{}
	req := mcpRequestWithArgs(args)
	result, callErr := mc.handlePreviewIcon(context.Background(), req)
	if callErr != nil {
		return fmt.Errorf("preview-icon: %w", callErr)
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}

	resp, err := decodePreviewIconResult(result)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if *outSVG != "" {
		if err := os.WriteFile(*outSVG, []byte(resp.SVGData), 0o644); err != nil { //nolint:gosec
			return fmt.Errorf("write svg: %w", err)
		}
		resp.SVGData = ""
	}
	if *outPNG != "" && resp.PNGBase64 != "" {
		pngBytes, decodeErr := base64.StdEncoding.DecodeString(resp.PNGBase64)
		if decodeErr != nil {
			return fmt.Errorf("decode png_base64: %w", decodeErr)
		}
		if err := os.WriteFile(*outPNG, pngBytes, 0o644); err != nil { //nolint:gosec
			return fmt.Errorf("write png: %w", err)
		}
		resp.PNGBase64 = ""
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

// buildPreviewIconArg assembles the icon argument from either a JSON file
// (or stdin) or the explicit flag set. Returns a structured error when the
// caller mixes the two input modes or supplies multiple source flags.
func buildPreviewIconArg(iconFile, name, path, url, svgData, fill, alt string) (map[string]any, error) {
	usingFile := iconFile != ""
	flagsCount := 0
	for _, s := range []string{name, path, url, svgData} {
		if s != "" {
			flagsCount++
		}
	}
	if usingFile && flagsCount > 0 {
		return nil, fmt.Errorf("--icon is mutually exclusive with --name/--path/--url/--svg-data")
	}
	if usingFile {
		data, err := readPreviewIconBlob(iconFile)
		if err != nil {
			return nil, fmt.Errorf("--icon: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("--icon: invalid JSON: %w", err)
		}
		return obj, nil
	}
	if flagsCount == 0 {
		return nil, fmt.Errorf("supply --icon, or one of --name/--path/--url/--svg-data")
	}
	out := map[string]any{}
	if name != "" {
		out["name"] = name
	}
	if path != "" {
		out["path"] = path
	}
	if url != "" {
		out["url"] = url
	}
	if svgData != "" {
		out["svg_data"] = svgData
	}
	if fill != "" {
		out["fill"] = fill
	}
	if alt != "" {
		out["alt"] = alt
	}
	return out, nil
}

func readPreviewIconBlob(spec string) ([]byte, error) {
	if spec == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(spec) //nolint:gosec
}

// decodePreviewIconResult extracts the typed response from an MCP success
// result so the CLI can post-process it (--out-svg / --out-png).
func decodePreviewIconResult(result *mcpgo.CallToolResult) (*previewIconResponse, error) {
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	text, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		return nil, fmt.Errorf("unexpected content kind %T", result.Content[0])
	}
	var direct previewIconResponse
	if err := json.Unmarshal([]byte(text.Text), &direct); err != nil {
		return nil, err
	}
	return &direct, nil
}

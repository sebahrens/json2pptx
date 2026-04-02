package svggen

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ahrens/go-slide-creator/icons"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
)

// iconMaxBytes is the maximum size for fetched icon data (512KB).
const iconMaxBytes = 512 * 1024

// iconHTTPTimeout is the timeout for HTTP icon fetches.
const iconHTTPTimeout = 5 * time.Second

// IconKind classifies the type of icon string.
type IconKind int

const (
	IconKindEmpty IconKind = iota
	IconKindURL
	IconKindDataURI
	IconKindInlineSVG
	IconKindFilePath
	IconKindName // bundled icon by name, e.g. "chart-pie" or "filled:chart-pie"
)

// ClassifyIcon determines the kind of icon string.
func ClassifyIcon(icon string) IconKind {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return IconKindEmpty
	}
	if strings.HasPrefix(icon, "data:") {
		return IconKindDataURI
	}
	if strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") {
		return IconKindURL
	}
	if strings.Contains(icon, "<svg") {
		return IconKindInlineSVG
	}
	// Recognize file paths: contain "/" or "\" and end with an image extension.
	if looksLikeFilePath(icon) {
		return IconKindFilePath
	}
	// Check bundled icon names (e.g. "chart-pie", "filled:chart-pie").
	if icons.Exists(icon) {
		return IconKindName
	}
	return IconKindEmpty
}

// looksLikeFilePath returns true if the string appears to be a file path
// to an image file (SVG, PNG, JPG, etc.).
func looksLikeFilePath(s string) bool {
	lower := strings.ToLower(s)
	hasPathSep := strings.ContainsAny(s, "/\\")
	hasExt := strings.HasSuffix(lower, ".svg") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg")
	return hasPathSep && hasExt
}

// LoadIcon loads an icon string into an image.Image at the given target pixel size.
// Returns nil on any failure (caller should use fallback).
func LoadIcon(icon string, targetSizePx int) image.Image {
	kind := ClassifyIcon(icon)
	if kind == IconKindEmpty {
		return nil
	}

	data, err := fetchIconBytes(icon, kind)
	if err != nil {
		slog.Warn("icon_loader: fetch failed", "error", err, "kind", kind)
		return nil
	}

	img, err := rasterizeIconData(data, targetSizePx)
	if err != nil {
		slog.Warn("icon_loader: rasterize failed", "error", err)
		return nil
	}

	return img
}

// fetchIconBytes retrieves raw icon bytes based on the icon kind.
func fetchIconBytes(icon string, kind IconKind) ([]byte, error) {
	switch kind {
	case IconKindURL:
		return fetchURL(icon)
	case IconKindDataURI:
		return decodeDataURI(icon)
	case IconKindInlineSVG:
		return []byte(strings.TrimSpace(icon)), nil
	case IconKindFilePath:
		return readFilePath(icon)
	case IconKindName:
		return icons.Lookup(icon)
	default:
		return nil, fmt.Errorf("unsupported icon kind: %d", kind)
	}
}

// fetchURL fetches icon bytes from a URL with timeout and size limit.
func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: iconHTTPTimeout}
	resp, err := client.Get(url) //nolint:gosec // URL comes from user-provided JSON data
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	limited := io.LimitReader(resp.Body, iconMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}
	if len(data) > iconMaxBytes {
		return nil, fmt.Errorf("icon from %s exceeds %d bytes", url, iconMaxBytes)
	}

	return data, nil
}

// decodeDataURI decodes a data: URI into raw bytes.
// Supports: data:image/svg+xml;base64,... and data:image/png;base64,...
func decodeDataURI(uri string) ([]byte, error) {
	// Find the comma separating metadata from data
	commaIdx := strings.Index(uri, ",")
	if commaIdx < 0 {
		return nil, fmt.Errorf("invalid data URI: no comma found")
	}

	meta := uri[:commaIdx]
	encoded := uri[commaIdx+1:]

	if strings.Contains(meta, ";base64") {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
		return data, nil
	}

	// Raw data (URL-encoded or plain)
	return []byte(encoded), nil
}

// IsEmojiIcon returns true if the icon string is likely an emoji or very short
// text (1-2 runes) that should be rendered as text rather than loaded as an image.
func IsEmojiIcon(icon string) bool {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return false
	}
	// If ClassifyIcon recognizes it as a loadable type, it's not emoji.
	if kind := ClassifyIcon(icon); kind != IconKindEmpty {
		return false
	}
	// Short strings (≤4 runes) that aren't loadable are treated as emoji/text icons.
	runes := []rune(icon)
	return len(runes) <= 4
}

// readFilePath reads icon bytes from a local file path.
func readFilePath(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Path comes from user-provided JSON data
	if err != nil {
		return nil, fmt.Errorf("reading icon file %s: %w", path, err)
	}
	if len(data) > iconMaxBytes {
		return nil, fmt.Errorf("icon file %s exceeds %d bytes", path, iconMaxBytes)
	}
	return data, nil
}

// rasterizeIconData attempts to parse icon data as SVG and rasterize it,
// falling back to PNG/JPEG decode if SVG parsing fails.
func rasterizeIconData(data []byte, targetSizePx int) (image.Image, error) {
	if targetSizePx <= 0 {
		targetSizePx = 128
	}

	// Try SVG parse first (most icons will be SVG)
	svgCanvas, err := canvas.ParseSVG(bytes.NewReader(data))
	if err == nil && svgCanvas != nil {
		return rasterizeSVGCanvas(svgCanvas, targetSizePx), nil
	}

	// Try as raster image (PNG, JPEG, etc.)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}

	// Try PNG specifically (image.Decode needs format registration)
	pngImg, err := png.Decode(bytes.NewReader(data))
	if err == nil {
		return pngImg, nil
	}

	return nil, fmt.Errorf("could not parse icon as SVG or raster image")
}

// rasterizeSVGCanvas renders a parsed SVG canvas to an image.Image at the target size.
func rasterizeSVGCanvas(c *canvas.Canvas, targetSizePx int) image.Image {
	// Calculate DPI to achieve target size.
	// Canvas dimensions are in mm; we want targetSizePx pixels on the longer side.
	w := c.W
	h := c.H
	if w <= 0 || h <= 0 {
		w, h = 100, 100 // fallback
	}

	longerMM := w
	if h > w {
		longerMM = h
	}

	// DPI = pixels / inches = pixels / (mm / 25.4)
	dpi := float64(targetSizePx) / (longerMM / 25.4)

	return rasterizer.Draw(c, canvas.DPI(dpi), nil)
}

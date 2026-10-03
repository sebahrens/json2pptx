package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// The render commands write PNG files (go-slide-creator-92gox).
//
// render-thumbnails and render-slide used to answer with every image as base64
// inside the stdout JSON — 169 KB for seven thumbnails — and had no output
// flag, so "give me PNGs of the slides" meant writing a decode script and
// inventing file names, and `inspect --images <dir>` asked for slide-0.png
// files no command produced. They now print a small manifest of files instead:
// with --out-dir / --out the PNGs are written where the caller asked, named
// slide-<index>.png (0-based, the names inspect reads); without it the manifest
// points at the render cache. --base64 restores the old envelope.

// cliSlideFile is one rendered slide in the manifest.
type cliSlideFile struct {
	// Index is the 0-based slide index, the N in slide-N.png.
	Index int `json:"index"`
	// Path is the PNG on disk.
	Path string `json:"path"`
	// SHA256 is the hash of the bytes at Path.
	SHA256 string `json:"sha256"`
	// Bytes is the size of the file at Path.
	Bytes  int64 `json:"bytes"`
	Width  int   `json:"width,omitempty"`
	Height int   `json:"height,omitempty"`
}

// cliRenderManifest is what the render commands print on stdout.
type cliRenderManifest struct {
	// OutDir is the directory the slide files were written to; absent when
	// the paths point into the render cache.
	OutDir string `json:"out_dir,omitempty"`
	// SlideCount is the deck's slide count, whatever was rendered.
	SlideCount int            `json:"slide_count,omitempty"`
	Slides     []cliSlideFile `json:"slides"`
	// Truncated is true when --max-slides cut the deck short.
	Truncated bool `json:"truncated,omitempty"`
	// SourceHash identifies the PPTX revision the images were rendered from.
	SourceHash string `json:"source_hash,omitempty"`
	// Note says how long cache paths live, when the paths are cache paths.
	Note string `json:"note,omitempty"`
}

// staleSlideFileRE matches the files a previous render-thumbnails run wrote.
var staleSlideFileRE = regexp.MustCompile(`^slide-[0-9]+\.png$`)

// cliRenderedSlides reads the per-slide metadata of a render tool result
// (image_content mode: content[0] is the JSON metadata, whose slides carry the
// full-resolution PNG path).
func cliRenderedSlides(result *mcpgo.CallToolResult) (renderedDeckThumbnailsResponse, error) {
	text := cliResultText(result)
	var deck renderedDeckThumbnailsResponse
	if err := json.Unmarshal([]byte(text), &deck); err != nil {
		return deck, fmt.Errorf("parse render result: %w", err)
	}
	if len(deck.Slides) > 0 {
		return deck, nil
	}
	// A single-slide render answers with the slide's own fields at top level.
	var single renderedSlideImageResponse
	if err := json.Unmarshal([]byte(text), &single); err != nil {
		return deck, fmt.Errorf("parse render result: %w", err)
	}
	if single.Path == "" {
		return deck, fmt.Errorf("render result names no image file")
	}
	deck.SourceHash = single.SourceHash
	deck.Slides = []renderedSlideMeta{single.renderedSlideMeta}
	return deck, nil
}

// cliPrintRenderManifest turns a render tool result into files plus a
// manifest. dest maps a slide index to the file it should be written to; a nil
// dest leaves the images in the render cache and reports those paths. outDir
// is recorded in the manifest and, when sweep is set, cleared of slide files a
// previous run left that this run did not rewrite (a shorter deck must not
// leave the longer one's tail behind for inspect to read).
func cliPrintRenderManifest(result *mcpgo.CallToolResult, outDir string, dest func(index int) string, sweep bool) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}
	deck, err := cliRenderedSlides(result)
	if err != nil {
		return err
	}

	manifest := cliRenderManifest{
		OutDir:     outDir,
		SlideCount: deck.SlideCount,
		Slides:     make([]cliSlideFile, 0, len(deck.Slides)),
		Truncated:  deck.Truncated,
		SourceHash: deck.SourceHash,
	}
	written := map[string]bool{}
	for _, s := range deck.Slides {
		path := s.Path
		if dest != nil {
			path = dest(s.Index)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
		}
		sum, size, err := copyFileHashed(s.Path, path)
		if err != nil {
			return fmt.Errorf("slide %d: %w", s.Index, err)
		}
		written[filepath.Base(path)] = true
		manifest.Slides = append(manifest.Slides, cliSlideFile{
			Index: s.Index, Path: path, SHA256: sum, Bytes: size, Width: s.Width, Height: s.Height,
		})
	}
	if dest == nil {
		manifest.Note = "paths are in the render cache (swept after 24h); pass --out-dir <dir> to write slide-<index>.png files you keep"
	} else if sweep && outDir != "" {
		if entries, err := os.ReadDir(outDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && staleSlideFileRE.MatchString(e.Name()) && !written[e.Name()] {
					_ = os.Remove(filepath.Join(outDir, e.Name()))
				}
			}
		}
	}
	return printJSONIndent(manifest)
}

// copyFileHashed copies src to dst (skipped when they are the same path) and
// returns the SHA-256 and size of the bytes at dst.
func copyFileHashed(src, dst string) (string, int64, error) {
	in, err := os.Open(src) // #nosec G304 -- render-cache artifact path returned by this process's renderer
	if err != nil {
		return "", 0, fmt.Errorf("open rendered image: %w", err)
	}
	defer func() { _ = in.Close() }()

	h := sha256.New()
	if src == dst {
		n, err := io.Copy(h, in)
		if err != nil {
			return "", 0, fmt.Errorf("read rendered image: %w", err)
		}
		return hex.EncodeToString(h.Sum(nil)), n, nil
	}

	// Write beside the destination and rename, so a reader never sees a
	// half-written PNG.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".slide-*.png.tmp")
	if err != nil {
		return "", 0, fmt.Errorf("write %s: %w", dst, err)
	}
	tmpName := tmp.Name()
	n, err := io.Copy(io.MultiWriter(tmp, h), in)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmpName, 0o644) // #nosec G302 -- a slide image the caller asked for; not a secret
	}
	if err == nil {
		err = os.Rename(tmpName, dst)
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return "", 0, fmt.Errorf("write %s: %w", dst, err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Package render converts PPTX files to PNG images using LibreOffice and ImageMagick.
package render

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// maxInlineBytes is the base64-decoded size cap per slide image.
// If a rendered PNG exceeds this, the tool returns a path reference instead.
const maxInlineBytes = 200 * 1024 // 200 KB

// mu serializes LibreOffice invocations (single-threaded per process).
var mu sync.Mutex

// SlideImage holds the rendered output for a single slide.
type SlideImage struct {
	Index   int    `json:"index"`
	PNG64   string `json:"png_base64,omitempty"`
	Path    string `json:"path,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	SizeErr string `json:"size_error,omitempty"`
}

// DeckResult holds the result of rendering an entire deck.
type DeckResult struct {
	Slides    []SlideImage `json:"slides"`
	Truncated bool         `json:"truncated"`
}

// checkDep verifies that a command-line tool is available on PATH.
func checkDep(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: install it to use render tools", name)
	}
	return nil
}

// CheckDependencies verifies LibreOffice and ImageMagick are available.
func CheckDependencies() error {
	if err := checkDep("libreoffice"); err != nil {
		return err
	}
	return checkDep("magick")
}

// pptxToPDF converts a PPTX to PDF via LibreOffice headless.
// Returns the path to the generated PDF.
func pptxToPDF(pptxPath, tmpDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	cmd := exec.Command("libreoffice",
		"--headless",
		"--convert-to", "pdf",
		"--outdir", tmpDir,
		pptxPath,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("libreoffice conversion failed: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(pptxPath), filepath.Ext(pptxPath))
	pdfPath := filepath.Join(tmpDir, base+".pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return "", fmt.Errorf("PDF not created at %s", pdfPath)
	}
	return pdfPath, nil
}

// pdfToPNGs converts a multi-page PDF to individual PNG files using ImageMagick.
// Returns sorted list of generated PNG paths.
func pdfToPNGs(pdfPath, outDir string, density int) ([]string, error) {
	pattern := filepath.Join(outDir, "slide-%d.png")
	cmd := exec.Command("magick", //nolint:gosec // density is a clamped int; paths are internal temp dirs
		"-density", fmt.Sprintf("%d", density),
		pdfPath,
		"-quality", "95",
		pattern,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("imagemagick conversion failed: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(outDir, "slide-*.png"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// readAsBase64 reads a file and returns its base64-encoded content.
func readAsBase64(path string) (string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(data), len(data), nil
}

// RenderSlide renders a single slide from a PPTX file to a PNG.
// slideIndex is 0-based.
func RenderSlide(pptxPath string, slideIndex, density int) (*SlideImage, error) {
	if err := CheckDependencies(); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "render-slide-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath, err := pptxToPDF(pptxPath, tmpDir)
	if err != nil {
		return nil, err
	}

	pngs, err := pdfToPNGs(pdfPath, tmpDir, density)
	if err != nil {
		return nil, err
	}

	if slideIndex < 0 || slideIndex >= len(pngs) {
		return nil, fmt.Errorf("slide_index %d out of range (deck has %d slides)", slideIndex, len(pngs))
	}

	img := &SlideImage{Index: slideIndex}
	b64, size, err := readAsBase64(pngs[slideIndex])
	if err != nil {
		return nil, fmt.Errorf("read rendered slide: %w", err)
	}

	if size > maxInlineBytes {
		// Too large for inline — copy to a stable path and return reference.
		stablePath := filepath.Join(os.TempDir(), fmt.Sprintf("json2pptx-slide-%d.png", slideIndex))
		data, _ := os.ReadFile(pngs[slideIndex])
		if writeErr := os.WriteFile(stablePath, data, 0644); writeErr != nil {
			return nil, fmt.Errorf("write stable file: %w", writeErr)
		}
		img.Path = stablePath
	} else {
		img.PNG64 = b64
	}

	return img, nil
}

// RenderDeck renders all slides in a PPTX to PNG thumbnails.
func RenderDeck(pptxPath string, density, maxSlides int) (*DeckResult, error) {
	if err := CheckDependencies(); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "render-deck-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath, err := pptxToPDF(pptxPath, tmpDir)
	if err != nil {
		return nil, err
	}

	pngs, err := pdfToPNGs(pdfPath, tmpDir, density)
	if err != nil {
		return nil, err
	}

	result := &DeckResult{}
	limit := len(pngs)
	if maxSlides > 0 && maxSlides < limit {
		limit = maxSlides
		result.Truncated = true
	}

	for i := 0; i < limit; i++ {
		img := SlideImage{Index: i}
		b64, size, err := readAsBase64(pngs[i])
		if err != nil {
			img.SizeErr = err.Error()
		} else if size > maxInlineBytes {
			stablePath := filepath.Join(os.TempDir(), fmt.Sprintf("json2pptx-thumb-%d.png", i))
			data, _ := os.ReadFile(pngs[i])
			if writeErr := os.WriteFile(stablePath, data, 0644); writeErr == nil {
				img.Path = stablePath
			} else {
				img.SizeErr = fmt.Sprintf("file too large for inline (%d bytes) and copy failed", size)
			}
		} else {
			img.PNG64 = b64
		}
		result.Slides = append(result.Slides, img)
	}

	return result, nil
}

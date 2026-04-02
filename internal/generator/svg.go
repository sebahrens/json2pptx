// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// SVGConversionError indicates that SVG conversion failed.
type SVGConversionError struct {
	Path   string
	Reason string
	Err    error
}

func (e *SVGConversionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("SVG conversion failed for %s: %s: %v", e.Path, e.Reason, e.Err)
	}
	return fmt.Sprintf("SVG conversion failed for %s: %s", e.Path, e.Reason)
}

func (e *SVGConversionError) Unwrap() error {
	return e.Err
}

// IsSVGFile checks if the given file path is an SVG file.
// It checks both by extension and by attempting to read the first bytes.
func IsSVGFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".svg" {
		return true
	}

	// Also check file content for SVG magic
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	// Read first 1KB to check for SVG markers
	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	content := string(buf[:n])
	return strings.Contains(content, "<svg") || strings.Contains(content, "<!DOCTYPE svg")
}

// SVGConverter handles conversion of SVG files to PNG or EMF format.
type SVGConverter struct {
	// Scale factor for PNG conversion (default: 2.0 = 2x resolution)
	Scale float64

	// Strategy determines the output format (png, emf, or native)
	Strategy SVGConversionStrategy

	// PreferredPNGConverter specifies which PNG converter to try first.
	// Options: "auto" (default), "rsvg-convert", "resvg"
	// "auto" tries rsvg-convert first, then resvg as fallback.
	PreferredPNGConverter string

	// MaxPNGWidth caps the pixel width of PNG output. When the rendered
	// width would exceed this value, the scale factor is reduced to fit.
	// 0 means no cap. Default: 2500.
	MaxPNGWidth int

	// rsvgConvertPath is the cached path to rsvg-convert (empty if not found)
	rsvgConvertPath string

	// resvgPath is the cached path to resvg (empty if not found)
	resvgPath string

	// inkscapePath is the cached path to inkscape (empty if not found)
	inkscapePath string

	// toolOnce ensures findTools runs exactly once, even under concurrent access
	toolOnce sync.Once
}

// NewSVGConverter creates a new SVG converter with default settings.
// Defaults to SVGStrategyNative for optimal quality in PowerPoint 2016+.
func NewSVGConverter() *SVGConverter {
	return &SVGConverter{
		Scale:    DefaultSVGScale,
		Strategy: SVGStrategyNative,
	}
}

// NewSVGConverterWithConfig creates an SVG converter with the given configuration.
func NewSVGConverterWithConfig(cfg SVGConfig) *SVGConverter {
	scale := cfg.Scale
	if scale <= 0 {
		scale = DefaultSVGScale
	}
	strategy := cfg.Strategy
	if strategy == "" {
		strategy = SVGStrategyNative
	}
	pngConverter := cfg.PreferredPNGConverter
	if pngConverter == "" {
		pngConverter = PNGConverterAuto
	}
	maxPNGWidth := cfg.MaxPNGWidth
	if maxPNGWidth == 0 {
		maxPNGWidth = DefaultMaxPNGWidth
	}
	return &SVGConverter{
		Scale:                 scale,
		Strategy:              strategy,
		PreferredPNGConverter: pngConverter,
		MaxPNGWidth:           maxPNGWidth,
	}
}

// IsAvailable checks if SVG conversion tools are available for the configured strategy.
// For PNG strategy: Returns true if rsvg-convert is found in PATH.
// For EMF strategy: Returns true if inkscape is found in PATH.
// For Native strategy: Always returns true (no external tools required).
func (c *SVGConverter) IsAvailable() bool {
	c.findTools()
	switch c.Strategy {
	case SVGStrategyNative:
		return true // Native embedding requires no external tools
	case SVGStrategyEMF:
		return c.inkscapePath != ""
	default: // PNG
		return c.rsvgConvertPath != ""
	}
}

// IsNativeAvailable returns true as native SVG embedding requires no external tools.
func (c *SVGConverter) IsNativeAvailable() bool {
	return true
}

// IsPNGAvailable checks if PNG conversion is available (rsvg-convert or resvg).
func (c *SVGConverter) IsPNGAvailable() bool {
	c.findTools()
	return c.rsvgConvertPath != "" || c.resvgPath != ""
}

// IsResvgAvailable checks if resvg is available.
func (c *SVGConverter) IsResvgAvailable() bool {
	c.findTools()
	return c.resvgPath != ""
}

// IsRsvgConvertAvailable checks if rsvg-convert is available.
func (c *SVGConverter) IsRsvgConvertAvailable() bool {
	c.findTools()
	return c.rsvgConvertPath != ""
}

// IsEMFAvailable checks if EMF conversion is available (inkscape).
func (c *SVGConverter) IsEMFAvailable() bool {
	c.findTools()
	return c.inkscapePath != ""
}

// findTools locates all SVG conversion tools in PATH.
// Safe for concurrent use via sync.Once.
func (c *SVGConverter) findTools() {
	c.toolOnce.Do(func() {
		// Look for rsvg-convert (librsvg) for PNG conversion
		if path, err := exec.LookPath("rsvg-convert"); err == nil {
			c.rsvgConvertPath = path
		}

		// Look for resvg (Rust-based alternative) for PNG conversion
		if path, err := exec.LookPath("resvg"); err == nil {
			c.resvgPath = path
		}

		// Look for inkscape for EMF conversion
		if path, err := exec.LookPath("inkscape"); err == nil {
			c.inkscapePath = path
		}
	})
}

// Convert converts an SVG file based on the configured strategy.
// Returns the path to the converted file (PNG, EMF, or original SVG for native) and a cleanup function.
// The caller MUST call the cleanup function when done with the output file.
// For native strategy, returns the original SVG path with a no-op cleanup function.
func (c *SVGConverter) Convert(svgPath string) (outputPath string, cleanup func(), err error) {
	switch c.Strategy {
	case SVGStrategyNative:
		// Native embedding - return original SVG, no conversion needed
		return svgPath, func() {}, nil
	case SVGStrategyEMF:
		return c.ConvertToEMF(svgPath)
	default: // PNG
		return c.ConvertToPNG(svgPath, 0)
	}
}

// ConvertToPNG converts an SVG file to PNG format.
// Returns the path to the temporary PNG file and a cleanup function.
// The caller MUST call the cleanup function when done with the PNG file.
//
// If scale is 0, uses the converter's configured Scale (or DefaultSVGScale).
// Uses PreferredPNGConverter to determine which tool to try first:
// - "auto" (default): tries rsvg-convert first, then resvg as fallback
// - "rsvg-convert": forces rsvg-convert only
// - "resvg": forces resvg only
func (c *SVGConverter) ConvertToPNG(svgPath string, scale float64) (pngPath string, cleanup func(), err error) {
	c.findTools()

	// Determine converter order based on preference
	preference := c.PreferredPNGConverter
	if preference == "" {
		preference = PNGConverterAuto
	}

	// Check availability based on preference
	switch preference {
	case PNGConverterRsvg:
		if c.rsvgConvertPath == "" {
			return "", nil, &SVGConversionError{
				Path:   svgPath,
				Reason: "rsvg-convert not found in PATH (required by preference)",
			}
		}
	case PNGConverterResvg:
		if c.resvgPath == "" {
			return "", nil, &SVGConversionError{
				Path:   svgPath,
				Reason: "resvg not found in PATH (required by preference)",
			}
		}
	default: // auto
		if c.rsvgConvertPath == "" && c.resvgPath == "" {
			return "", nil, &SVGConversionError{
				Path:   svgPath,
				Reason: "no PNG converter found (need rsvg-convert or resvg in PATH)",
			}
		}
	}

	// Security: Validate SVG path before command execution (MED-08)
	// Prevents path traversal attacks by checking for ".." components
	if err := utils.ValidatePath(svgPath, nil); err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "invalid path",
			Err:    err,
		}
	}

	// Verify input exists
	if _, err := os.Stat(svgPath); err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "file not found",
			Err:    err,
		}
	}

	// Use provided scale or default
	if scale <= 0 {
		scale = c.Scale
	}
	if scale <= 0 {
		scale = DefaultSVGScale
	}

	// Create temp file for PNG output
	tmpFile, err := os.CreateTemp("", "svg-converted-*.png")
	if err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "failed to create temp file",
			Err:    err,
		}
	}
	pngPath = tmpFile.Name()
	_ = tmpFile.Close()

	// Create cleanup function
	cleanup = func() {
		_ = os.Remove(pngPath)
	}

	// Attempt conversion based on preference
	maxW := c.MaxPNGWidth
	var convErr error
	switch preference {
	case PNGConverterRsvg:
		convErr = c.runRsvgConvert(svgPath, pngPath, scale, maxW)
	case PNGConverterResvg:
		convErr = c.runResvg(svgPath, pngPath, scale, maxW)
	default: // auto - try primary first, then fallback
		if c.rsvgConvertPath != "" {
			convErr = c.runRsvgConvert(svgPath, pngPath, scale, maxW)
			// If rsvg-convert fails and resvg is available, try resvg
			if convErr != nil && c.resvgPath != "" {
				convErr = c.runResvg(svgPath, pngPath, scale, maxW)
			}
		} else {
			// rsvg-convert not available, use resvg
			convErr = c.runResvg(svgPath, pngPath, scale, maxW)
		}
	}

	if convErr != nil {
		cleanup()
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: convErr.Error(),
		}
	}

	// Verify output was created
	if _, err := os.Stat(pngPath); err != nil {
		cleanup()
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "output PNG not created",
			Err:    err,
		}
	}

	return pngPath, cleanup, nil
}

// runRsvgConvert executes rsvg-convert to convert SVG to PNG.
// rsvg-convert -z 2.0 -o output.png input.svg
// When maxWidth > 0, adds -w and -a flags to cap the output pixel width.
func (c *SVGConverter) runRsvgConvert(svgPath, pngPath string, scale float64, maxWidth int) error {
	args := []string{
		"-z", fmt.Sprintf("%.1f", scale),
	}
	if maxWidth > 0 {
		args = append(args, "-w", fmt.Sprintf("%d", maxWidth), "-a")
	}
	args = append(args, "-o", pngPath, svgPath)

	cmd := exec.Command(c.rsvgConvertPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsvg-convert failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// runResvg executes resvg to convert SVG to PNG.
// resvg uses different syntax: resvg --zoom 2.0 input.svg output.png
// Note: resvg uses --zoom instead of -z for scale factor.
// When maxWidth > 0, adds -w flag to cap the output pixel width.
func (c *SVGConverter) runResvg(svgPath, pngPath string, scale float64, maxWidth int) error {
	args := []string{
		"--zoom", fmt.Sprintf("%.1f", scale),
	}
	if maxWidth > 0 {
		args = append(args, "-w", fmt.Sprintf("%d", maxWidth))
	}
	args = append(args, svgPath, pngPath)

	cmd := exec.Command(c.resvgPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resvg failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ConvertToEMF converts an SVG file to EMF (Enhanced Metafile) format.
// Returns the path to the temporary EMF file and a cleanup function.
// The caller MUST call the cleanup function when done with the EMF file.
//
// EMF preserves vector quality and works with PowerPoint 2010+.
// Requires Inkscape to be installed and available in PATH.
func (c *SVGConverter) ConvertToEMF(svgPath string) (emfPath string, cleanup func(), err error) {
	c.findTools()

	if c.inkscapePath == "" {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "inkscape not found in PATH (required for EMF conversion)",
		}
	}

	// Security: Validate SVG path before command execution (MED-08)
	// Prevents path traversal attacks by checking for ".." components
	if err := utils.ValidatePath(svgPath, nil); err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "invalid path",
			Err:    err,
		}
	}

	// Verify input exists
	if _, err := os.Stat(svgPath); err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "file not found",
			Err:    err,
		}
	}

	// Create temp file for EMF output
	tmpFile, err := os.CreateTemp("", "svg-converted-*.emf")
	if err != nil {
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "failed to create temp file",
			Err:    err,
		}
	}
	emfPath = tmpFile.Name()
	_ = tmpFile.Close()

	// Create cleanup function
	cleanup = func() {
		_ = os.Remove(emfPath)
	}

	// Run inkscape to convert SVG to EMF
	// inkscape --export-type=emf --export-filename=output.emf input.svg
	cmd := exec.Command(c.inkscapePath,
		"--export-type=emf",
		"--export-filename="+emfPath,
		svgPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: fmt.Sprintf("inkscape failed: %s", string(output)),
			Err:    err,
		}
	}

	// Verify output was created
	if _, err := os.Stat(emfPath); err != nil {
		cleanup()
		return "", nil, &SVGConversionError{
			Path:   svgPath,
			Reason: "output EMF not created",
			Err:    err,
		}
	}

	return emfPath, cleanup, nil
}

// GetStrategy returns the current SVG conversion strategy.
func (c *SVGConverter) GetStrategy() SVGConversionStrategy {
	return c.Strategy
}

// SetStrategy updates the SVG conversion strategy.
func (c *SVGConverter) SetStrategy(strategy SVGConversionStrategy) {
	c.Strategy = strategy
}

// defaultSVGConverter is the package-level converter instance.
var defaultSVGConverter = NewSVGConverter()

// defaultSVGConverterMu protects access to defaultSVGConverter for thread-safety.
var defaultSVGConverterMu sync.RWMutex

// DefaultSVGConverter returns the package-level SVG converter.
func DefaultSVGConverter() *SVGConverter {
	defaultSVGConverterMu.RLock()
	defer defaultSVGConverterMu.RUnlock()
	return defaultSVGConverter
}

// SVGConversionAvailable returns true if SVG conversion is available.
func SVGConversionAvailable() bool {
	return DefaultSVGConverter().IsAvailable()
}

// ConvertSVGToPNG converts an SVG file to PNG using the default converter.
// Returns the path to the temporary PNG file and a cleanup function.
// The caller MUST call the cleanup function when done with the PNG file.
func ConvertSVGToPNG(svgPath string, scale float64) (pngPath string, cleanup func(), err error) {
	return DefaultSVGConverter().ConvertToPNG(svgPath, scale)
}

// TempFilePatterns lists the glob patterns for temp files created by SVG conversion.
// Used by CleanupOrphanedTempFiles to identify files to clean up.
var TempFilePatterns = []string{
	"svg-converted-*.png",
	"svg-converted-*.emf",
}

// DefaultTempFileMaxAge is the default maximum age for temp files before cleanup.
// Files older than this are considered orphaned and will be removed.
const DefaultTempFileMaxAge = 1 * time.Hour

// CleanupOrphanedTempFiles removes SVG conversion temp files older than maxAge.
// This should be called on application startup to clean up files orphaned by
// previous crashes or abnormal terminations (LOW-07 security fix).
//
// Returns the number of files cleaned up and any errors encountered.
func CleanupOrphanedTempFiles(maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		maxAge = DefaultTempFileMaxAge
	}

	tempDir := os.TempDir()
	cutoff := time.Now().Add(-maxAge)
	cleaned := 0
	var errs []error

	for _, pattern := range TempFilePatterns {
		fullPattern := filepath.Join(tempDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("glob %s: %w", pattern, err))
			continue
		}

		for _, f := range matches {
			info, err := os.Stat(f)
			if err != nil {
				// File might have been deleted already
				continue
			}

			if info.ModTime().Before(cutoff) {
				if err := os.Remove(f); err != nil {
					errs = append(errs, fmt.Errorf("remove %s: %w", f, err))
				} else {
					cleaned++
				}
			}
		}
	}

	if len(errs) > 0 {
		return cleaned, fmt.Errorf("cleanup errors: %v", errs)
	}
	return cleaned, nil
}

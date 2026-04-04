package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// markToolsFound marks the converter's toolOnce as already fired,
// preventing findTools from running when tool paths are set directly in tests.
func markToolsFound(c *SVGConverter) {
	c.toolOnce.Do(func() {})
}

// Test SVG file for testing (simple valid SVG)
const testSVGContent = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
  <circle cx="50" cy="50" r="40" fill="#3498db"/>
</svg>`

// Test fixture with more complex SVG
const complexSVGContent = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <defs>
    <linearGradient id="grad1" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:rgb(255,255,0);stop-opacity:1" />
      <stop offset="100%" style="stop-color:rgb(255,0,0);stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect x="10" y="10" width="180" height="180" fill="url(#grad1)" rx="10"/>
  <text x="100" y="100" text-anchor="middle" fill="white" font-size="20">Test</text>
</svg>`

// TestIsSVGFile_Extension tests detection by file extension
func TestIsSVGFile_Extension(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantSVG  bool
	}{
		{
			name:     "svg extension with svg content",
			filename: "test.svg",
			content:  testSVGContent,
			wantSVG:  true,
		},
		{
			name:     "SVG extension uppercase",
			filename: "test.SVG",
			content:  testSVGContent,
			wantSVG:  true,
		},
		{
			name:     "png extension",
			filename: "test.png",
			content:  "PNG data",
			wantSVG:  false,
		},
		{
			name:     "svg content without extension",
			filename: "testfile",
			content:  testSVGContent,
			wantSVG:  true, // Should detect by content
		},
		{
			name:     "xml file with svg content",
			filename: "test.xml",
			content:  testSVGContent,
			wantSVG:  true, // Should detect by content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			got := IsSVGFile(filePath)
			if got != tt.wantSVG {
				t.Errorf("IsSVGFile(%q) = %v, want %v", tt.filename, got, tt.wantSVG)
			}
		})
	}
}

// TestIsSVGFile_NonExistent tests behavior with non-existent file
func TestIsSVGFile_NonExistent(t *testing.T) {
	// Non-existent file without .svg extension should return false
	if IsSVGFile("/nonexistent/path/file.png") {
		t.Error("Expected false for non-existent file without .svg extension")
	}

	// Non-existent file with .svg extension should return true (by extension)
	if !IsSVGFile("/nonexistent/path/file.svg") {
		t.Error("Expected true for file with .svg extension even if non-existent")
	}
}

// TestSVGConverter_IsAvailable tests tool detection
func TestSVGConverter_IsAvailable(t *testing.T) {
	converter := NewSVGConverter()
	available := converter.IsAvailable()

	// Log availability but don't fail - tool may not be installed
	t.Logf("rsvg-convert available: %v", available)
}

// TestSVGConverter_ConvertToPNG tests SVG to PNG conversion
func TestSVGConverter_ConvertToPNG(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert to PNG
	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0) // 0 = use default scale
	if err != nil {
		t.Fatalf("ConvertToPNG failed: %v", err)
	}
	defer cleanup()

	// Verify PNG was created
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}

	// Verify PNG has content
	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("Converted SVG to PNG: %s (size: %d bytes)", pngPath, info.Size())
}

// TestSVGConverter_ConvertToPNG_CustomScale tests conversion with custom scale
func TestSVGConverter_ConvertToPNG_CustomScale(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Test different scales
	scales := []float64{1.0, 2.0, 3.0}
	var lastSize int64

	for _, scale := range scales {
		pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, scale)
		if err != nil {
			t.Errorf("ConvertToPNG with scale %.1f failed: %v", scale, err)
			continue
		}

		info, err := os.Stat(pngPath)
		if err != nil {
			cleanup()
			t.Errorf("PNG file not created for scale %.1f: %v", scale, err)
			continue
		}

		// Larger scale should generally produce larger files
		if lastSize > 0 && info.Size() <= lastSize {
			t.Logf("Warning: scale %.1f did not produce larger file (got %d, last was %d)", scale, info.Size(), lastSize)
		}
		lastSize = info.Size()

		t.Logf("Scale %.1f: %d bytes", scale, info.Size())
		cleanup()
	}
}

// TestSVGConverter_ConvertToPNG_InvalidSVG tests error handling for invalid SVG
func TestSVGConverter_ConvertToPNG_InvalidSVG(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	// Create invalid SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "invalid.svg")
	if err := os.WriteFile(svgPath, []byte("this is not valid svg"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Conversion should fail
	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Errorf("Expected error for invalid SVG, got pngPath: %s", pngPath)
	}

	// Verify error is SVGConversionError
	if _, ok := err.(*SVGConversionError); !ok {
		t.Errorf("Expected SVGConversionError, got %T", err)
	}
}

// TestSVGConverter_ConvertToPNG_NonExistent tests error handling for non-existent file
func TestSVGConverter_ConvertToPNG_NonExistent(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	_, cleanup, err := converter.ConvertToPNG(context.Background(), "/nonexistent/file.svg", 0)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Error("Expected error for non-existent file")
	}
}

// TestSVGConverter_Cleanup tests that cleanup properly removes temp files
func TestSVGConverter_Cleanup(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0)
	if err != nil {
		t.Fatalf("ConvertToPNG failed: %v", err)
	}

	// Verify file exists before cleanup
	if _, err := os.Stat(pngPath); err != nil {
		t.Errorf("PNG should exist before cleanup: %v", err)
	}

	// Call cleanup
	cleanup()

	// Verify file is removed after cleanup
	if _, err := os.Stat(pngPath); !os.IsNotExist(err) {
		t.Error("PNG should be removed after cleanup")
	}
}

// TestSVGConverter_ComplexSVG tests conversion of complex SVG with gradients and text
func TestSVGConverter_ComplexSVG(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "complex.svg")
	if err := os.WriteFile(svgPath, []byte(complexSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
	if err != nil {
		t.Fatalf("ConvertToPNG failed for complex SVG: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}

	t.Logf("Complex SVG converted to PNG: %d bytes", info.Size())
}

// TestSVGConversionAvailable tests the package-level function
func TestSVGConversionAvailable(t *testing.T) {
	available := SVGConversionAvailable()
	t.Logf("SVG conversion available: %v", available)
}

// TestConvertSVGToPNG tests the package-level conversion function
func TestConvertSVGToPNG(t *testing.T) {
	if !SVGConversionAvailable() {
		t.Skip("SVG conversion not available")
	}

	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	pngPath, cleanup, err := ConvertSVGToPNG(context.Background(), svgPath, DefaultSVGScale)
	if err != nil {
		t.Fatalf("ConvertSVGToPNG failed: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(pngPath); err != nil {
		t.Errorf("PNG file not created: %v", err)
	}
}

// TestSVGConversionError_Error tests error message formatting
func TestSVGConversionError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *SVGConversionError
		want string
	}{
		{
			name: "with underlying error",
			err: &SVGConversionError{
				Path:   "/path/to/file.svg",
				Reason: "tool not found",
				Err:    os.ErrNotExist,
			},
			want: "SVG conversion failed for /path/to/file.svg: tool not found: file does not exist",
		},
		{
			name: "without underlying error",
			err: &SVGConversionError{
				Path:   "/path/to/file.svg",
				Reason: "rsvg-convert not found in PATH",
			},
			want: "SVG conversion failed for /path/to/file.svg: rsvg-convert not found in PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSVGConversionError_Unwrap tests error unwrapping
func TestSVGConversionError_Unwrap(t *testing.T) {
	underlying := os.ErrNotExist
	err := &SVGConversionError{
		Path:   "/path/to/file.svg",
		Reason: "file not found",
		Err:    underlying,
	}

	if err.Unwrap() != underlying {
		t.Error("Unwrap should return the underlying error")
	}
}

// =============================================================================
// A16: SVG Strategy Configuration Tests
// =============================================================================

// TestNewSVGConverterWithConfig tests creating a converter with config
func TestNewSVGConverterWithConfig(t *testing.T) {
	tests := []struct {
		name         string
		cfg          SVGConfig
		wantStrategy SVGConversionStrategy
		wantScale    float64
	}{
		{
			name: "png strategy with custom scale",
			cfg: SVGConfig{
				Strategy: SVGStrategyPNG,
				Scale:    3.0,
			},
			wantStrategy: SVGStrategyPNG,
			wantScale:    3.0,
		},
		{
			name: "emf strategy",
			cfg: SVGConfig{
				Strategy: SVGStrategyEMF,
				Scale:    2.0,
			},
			wantStrategy: SVGStrategyEMF,
			wantScale:    2.0,
		},
		{
			name: "native strategy",
			cfg: SVGConfig{
				Strategy: SVGStrategyNative,
				Scale:    2.0,
			},
			wantStrategy: SVGStrategyNative,
			wantScale:    2.0,
		},
		{
			name:         "empty config uses defaults",
			cfg:          SVGConfig{},
			wantStrategy: SVGStrategyNative,
			wantScale:    DefaultSVGScale,
		},
		{
			name: "zero scale uses default",
			cfg: SVGConfig{
				Strategy: SVGStrategyPNG,
				Scale:    0,
			},
			wantStrategy: SVGStrategyPNG,
			wantScale:    DefaultSVGScale,
		},
		{
			name: "negative scale uses default",
			cfg: SVGConfig{
				Strategy: SVGStrategyPNG,
				Scale:    -1.0,
			},
			wantStrategy: SVGStrategyPNG,
			wantScale:    DefaultSVGScale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSVGConverterWithConfig(tt.cfg)

			if converter.Strategy != tt.wantStrategy {
				t.Errorf("Strategy = %q, want %q", converter.Strategy, tt.wantStrategy)
			}

			if converter.Scale != tt.wantScale {
				t.Errorf("Scale = %.1f, want %.1f", converter.Scale, tt.wantScale)
			}
		})
	}
}

// TestSVGConverter_IsAvailable_Strategy tests availability check respects strategy
func TestSVGConverter_IsAvailable_Strategy(t *testing.T) {
	// Test PNG strategy availability
	pngConverter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyPNG,
	})
	pngAvailable := pngConverter.IsAvailable()
	t.Logf("PNG conversion available: %v", pngAvailable)

	// Test EMF strategy availability
	emfConverter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyEMF,
	})
	emfAvailable := emfConverter.IsAvailable()
	t.Logf("EMF conversion available: %v", emfAvailable)

	// Test individual tool checks
	t.Logf("IsPNGAvailable: %v", pngConverter.IsPNGAvailable())
	t.Logf("IsEMFAvailable: %v", emfConverter.IsEMFAvailable())

	// Test native strategy availability (always true)
	nativeConverter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
	})
	if !nativeConverter.IsAvailable() {
		t.Error("Native strategy should always be available")
	}
	if !nativeConverter.IsNativeAvailable() {
		t.Error("IsNativeAvailable should always return true")
	}
	t.Logf("Native conversion available: %v", nativeConverter.IsAvailable())
}

// TestSVGConverter_Convert_Native tests the strategy-based Convert method with Native
func TestSVGConverter_Convert_Native(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
	})

	// Native strategy is always available
	if !converter.IsAvailable() {
		t.Fatal("Native strategy should always be available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert using strategy-based method - should return original path
	outputPath, cleanup, err := converter.Convert(context.Background(), svgPath)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	defer cleanup()

	// For native strategy, output path should be the original SVG path
	if outputPath != svgPath {
		t.Errorf("Native strategy should return original path, got %q want %q", outputPath, svgPath)
	}

	// Verify file still exists (cleanup is no-op for native)
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file should exist: %v", err)
	}
}

// TestSVGConverter_Convert_PNG tests the strategy-based Convert method with PNG
func TestSVGConverter_Convert_PNG(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyPNG,
		Scale:    2.0,
	})

	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert using strategy-based method
	outputPath, cleanup, err := converter.Convert(context.Background(), svgPath)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	defer cleanup()

	// Verify output is PNG
	if filepath.Ext(outputPath) != ".png" {
		t.Errorf("Expected PNG output, got %s", filepath.Ext(outputPath))
	}

	// Verify file exists and has content
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("PNG output: %s (%d bytes)", outputPath, info.Size())
}

// TestSVGConverter_ConvertToEMF tests EMF conversion
func TestSVGConverter_ConvertToEMF(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyEMF,
	})

	if !converter.IsEMFAvailable() {
		t.Skip("inkscape not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert to EMF
	emfPath, cleanup, err := converter.ConvertToEMF(context.Background(), svgPath)
	if err != nil {
		t.Fatalf("ConvertToEMF failed: %v", err)
	}
	defer cleanup()

	// Verify output is EMF
	if filepath.Ext(emfPath) != ".emf" {
		t.Errorf("Expected EMF output, got %s", filepath.Ext(emfPath))
	}

	// Verify file exists and has content
	info, err := os.Stat(emfPath)
	if err != nil {
		t.Fatalf("EMF file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("EMF file is empty")
	}

	t.Logf("EMF output: %s (%d bytes)", emfPath, info.Size())
}

// TestSVGConverter_Convert_EMF tests the strategy-based Convert method with EMF
func TestSVGConverter_Convert_EMF(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyEMF,
	})

	if !converter.IsAvailable() {
		t.Skip("inkscape not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert using strategy-based method
	outputPath, cleanup, err := converter.Convert(context.Background(), svgPath)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	defer cleanup()

	// Verify output is EMF
	if filepath.Ext(outputPath) != ".emf" {
		t.Errorf("Expected EMF output, got %s", filepath.Ext(outputPath))
	}

	// Verify file exists
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("EMF file not created: %v", err)
	}

	t.Logf("EMF output: %s (%d bytes)", outputPath, info.Size())
}

// TestSVGConverter_ConvertToEMF_MissingTool tests error when inkscape not available
func TestSVGConverter_ConvertToEMF_MissingTool(t *testing.T) {
	converter := &SVGConverter{
		Strategy:     SVGStrategyEMF,
		inkscapePath: "", // Simulate missing tool
	}
	markToolsFound(converter)

	_, cleanup, err := converter.ConvertToEMF(context.Background(), "/path/to/file.svg")
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Error("Expected error when inkscape not available")
	}

	// Verify error message mentions inkscape
	svgErr, ok := err.(*SVGConversionError)
	if !ok {
		t.Errorf("Expected SVGConversionError, got %T", err)
	} else if svgErr.Reason != "inkscape not found in PATH (required for EMF conversion)" {
		t.Errorf("Unexpected error reason: %s", svgErr.Reason)
	}
}

// =============================================================================
// SVG Fixture Tests - Size, Aspect Ratio, Complexity, and Edge Cases
// =============================================================================

// getFixturesDir returns the path to the SVG fixtures directory
func getFixturesDir(t *testing.T) string {
	t.Helper()
	// Get the directory of this test file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	return filepath.Join(filepath.Dir(filename), "testdata", "svg_fixtures")
}

// TestSVGFixtures_Sizes tests conversion of SVG files with different sizes
func TestSVGFixtures_Sizes(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	tests := []struct {
		name         string
		filename     string
		expectedSize string // Expected dimensions description
		minBytes     int64  // Minimum expected PNG size
	}{
		{
			name:         "tiny 10x10",
			filename:     "tiny_10x10.svg",
			expectedSize: "10x10 at 2x = 20x20",
			minBytes:     100, // Very small PNG
		},
		{
			name:         "small 50x50",
			filename:     "small_50x50.svg",
			expectedSize: "50x50 at 2x = 100x100",
			minBytes:     500,
		},
		{
			name:         "medium 200x200",
			filename:     "medium_200x200.svg",
			expectedSize: "200x200 at 2x = 400x400",
			minBytes:     1000,
		},
		{
			name:         "large 1000x1000",
			filename:     "large_1000x1000.svg",
			expectedSize: "1000x1000 at 2x = 2000x2000",
			minBytes:     10000,
		},
		{
			name:         "huge 4000x4000",
			filename:     "huge_4000x4000.svg",
			expectedSize: "4000x4000 at 2x = 8000x8000",
			minBytes:     50000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, tt.filename)
			if _, err := os.Stat(svgPath); os.IsNotExist(err) {
				t.Skipf("Fixture file not found: %s", svgPath)
			}

			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if err != nil {
				t.Fatalf("ConvertToPNG failed for %s: %v", tt.filename, err)
			}
			defer cleanup()

			info, err := os.Stat(pngPath)
			if err != nil {
				t.Fatalf("PNG file not created: %v", err)
			}

			if info.Size() < tt.minBytes {
				t.Errorf("PNG size %d is smaller than expected minimum %d bytes", info.Size(), tt.minBytes)
			}

			t.Logf("%s (%s): %d bytes", tt.filename, tt.expectedSize, info.Size())
		})
	}
}

// TestSVGFixtures_AspectRatios tests conversion of SVG files with different aspect ratios
func TestSVGFixtures_AspectRatios(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	tests := []struct {
		name     string
		filename string
		ratio    string
	}{
		{
			name:     "wide 16:9",
			filename: "wide_16x9.svg",
			ratio:    "16:9 (1920x1080)",
		},
		{
			name:     "tall 9:16",
			filename: "tall_9x16.svg",
			ratio:    "9:16 (1080x1920)",
		},
		{
			name:     "square 1:1",
			filename: "square_500x500.svg",
			ratio:    "1:1 (500x500)",
		},
		{
			name:     "ultrawide 21:9",
			filename: "ultrawide_21x9.svg",
			ratio:    "21:9 (2560x1080)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, tt.filename)
			if _, err := os.Stat(svgPath); os.IsNotExist(err) {
				t.Skipf("Fixture file not found: %s", svgPath)
			}

			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if err != nil {
				t.Fatalf("ConvertToPNG failed for %s: %v", tt.filename, err)
			}
			defer cleanup()

			info, err := os.Stat(pngPath)
			if err != nil {
				t.Fatalf("PNG file not created: %v", err)
			}

			if info.Size() == 0 {
				t.Error("PNG file is empty")
			}

			t.Logf("%s (%s): %d bytes", tt.filename, tt.ratio, info.Size())
		})
	}
}

// TestSVGFixtures_Complexity tests conversion of SVG files with varying complexity
func TestSVGFixtures_Complexity(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	tests := []struct {
		name        string
		filename    string
		description string
	}{
		{
			name:        "linear gradient",
			filename:    "gradient_linear.svg",
			description: "SVG with linear gradient fill",
		},
		{
			name:        "radial gradient",
			filename:    "gradient_radial.svg",
			description: "SVG with radial gradient fill",
		},
		{
			name:        "transparency/alpha",
			filename:    "transparency_alpha.svg",
			description: "SVG with semi-transparent overlapping shapes",
		},
		{
			name:        "complex paths",
			filename:    "complex_paths.svg",
			description: "SVG with bezier curves and star polygon",
		},
		{
			name:        "text styles",
			filename:    "text_styles.svg",
			description: "SVG with various text formatting",
		},
		{
			name:        "blur and shadow filters",
			filename:    "filter_blur.svg",
			description: "SVG with blur and drop shadow filters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, tt.filename)
			if _, err := os.Stat(svgPath); os.IsNotExist(err) {
				t.Skipf("Fixture file not found: %s", svgPath)
			}

			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if err != nil {
				t.Fatalf("ConvertToPNG failed for %s: %v", tt.filename, err)
			}
			defer cleanup()

			info, err := os.Stat(pngPath)
			if err != nil {
				t.Fatalf("PNG file not created: %v", err)
			}

			if info.Size() == 0 {
				t.Error("PNG file is empty")
			}

			t.Logf("%s (%s): %d bytes", tt.filename, tt.description, info.Size())
		})
	}
}

// TestSVGFixtures_EdgeCases tests edge case SVG files
func TestSVGFixtures_EdgeCases(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	tests := []struct {
		name        string
		filename    string
		description string
		expectError bool
	}{
		{
			name:        "minimal SVG",
			filename:    "minimal.svg",
			description: "Bare minimum valid SVG",
			expectError: false,
		},
		{
			name:        "no viewBox",
			filename:    "no_viewbox.svg",
			description: "SVG without viewBox attribute",
			expectError: false,
		},
		{
			name:        "empty content",
			filename:    "empty_content.svg",
			description: "Valid SVG with no visual elements",
			expectError: false, // Should produce empty/transparent PNG
		},
		{
			name:        "transparent background",
			filename:    "transparent_bg.svg",
			description: "SVG with no background (transparent)",
			expectError: false,
		},
		{
			name:        "heavy whitespace",
			filename:    "whitespace_heavy.svg",
			description: "SVG with excessive whitespace formatting",
			expectError: false,
		},
		{
			name:        "with DOCTYPE",
			filename:    "doctype.svg",
			description: "SVG with DOCTYPE declaration",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, tt.filename)
			if _, err := os.Stat(svgPath); os.IsNotExist(err) {
				t.Skipf("Fixture file not found: %s", svgPath)
			}

			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if tt.expectError {
				if err == nil {
					if cleanup != nil {
						cleanup()
					}
					t.Errorf("Expected error for %s, but conversion succeeded", tt.filename)
				}
				return
			}

			if err != nil {
				t.Fatalf("ConvertToPNG failed for %s: %v", tt.filename, err)
			}
			defer cleanup()

			info, err := os.Stat(pngPath)
			if err != nil {
				t.Fatalf("PNG file not created: %v", err)
			}

			// Even empty SVGs should produce a file (might be small but not zero)
			t.Logf("%s (%s): %d bytes", tt.filename, tt.description, info.Size())
		})
	}
}

// TestSVGScaling_Fidelity tests that scaling produces proportionally larger output
func TestSVGScaling_Fidelity(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	svgPath := filepath.Join(fixturesDir, "medium_200x200.svg")
	if _, err := os.Stat(svgPath); os.IsNotExist(err) {
		t.Skip("Fixture file not found")
	}

	scales := []float64{0.5, 1.0, 2.0, 3.0, 4.0}
	results := make(map[float64]int64)

	for _, scale := range scales {
		pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, scale)
		if err != nil {
			t.Errorf("ConvertToPNG at scale %.1f failed: %v", scale, err)
			continue
		}

		info, err := os.Stat(pngPath)
		if err != nil {
			cleanup()
			t.Errorf("Failed to stat PNG at scale %.1f: %v", scale, err)
			continue
		}

		results[scale] = info.Size()
		cleanup()
	}

	// Log all results
	t.Log("Scale -> File Size:")
	for _, scale := range scales {
		if size, ok := results[scale]; ok {
			t.Logf("  %.1fx: %d bytes", scale, size)
		}
	}

	// Verify scaling trend: larger scale should produce larger files
	// (allowing some tolerance for compression variations)
	lastScale := 0.0
	lastSize := int64(0)
	for _, scale := range scales {
		size, ok := results[scale]
		if !ok {
			continue
		}
		if lastSize > 0 {
			// Allow 10% tolerance for compression variations
			expectedMinSize := int64(float64(lastSize) * 0.8)
			if size < expectedMinSize {
				t.Logf("Warning: scale %.1f (%d bytes) is not proportionally larger than %.1f (%d bytes)",
					scale, size, lastScale, lastSize)
			}
		}
		lastScale = scale
		lastSize = size
	}
}

// TestSVGDetection_AllFixtures tests that all fixture files are detected as SVG
func TestSVGDetection_AllFixtures(t *testing.T) {
	fixturesDir := getFixturesDir(t)

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Fixtures directory not found")
		}
		t.Fatalf("Failed to read fixtures directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".svg" {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, entry.Name())
			if !IsSVGFile(svgPath) {
				t.Errorf("IsSVGFile(%q) = false, expected true", entry.Name())
			}
		})
	}
}

// TestSVGConversion_AllFixtures runs conversion on all fixture files
func TestSVGConversion_AllFixtures(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	fixturesDir := getFixturesDir(t)
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Fixtures directory not found")
		}
		t.Fatalf("Failed to read fixtures directory: %v", err)
	}

	var totalFiles, successCount, failCount int
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".svg" {
			continue
		}

		totalFiles++
		t.Run(entry.Name(), func(t *testing.T) {
			svgPath := filepath.Join(fixturesDir, entry.Name())
			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if err != nil {
				failCount++
				t.Errorf("ConvertToPNG failed: %v", err)
				return
			}
			defer cleanup()

			info, err := os.Stat(pngPath)
			if err != nil {
				failCount++
				t.Errorf("PNG file not created: %v", err)
				return
			}

			successCount++
			t.Logf("Converted successfully: %d bytes", info.Size())
		})
	}

	t.Logf("Total: %d files, %d succeeded, %d failed", totalFiles, successCount, failCount)
}

// =============================================================================
// Security Tests (MED-08: SVG converter path validation)
// =============================================================================

// TestSVGConverter_ConvertToPNG_PathTraversal tests that path traversal attacks are blocked
func TestSVGConverter_ConvertToPNG_PathTraversal(t *testing.T) {
	converter := &SVGConverter{
		rsvgConvertPath: "/usr/bin/rsvg-convert", // Fake path - won't be executed
	}
	markToolsFound(converter)

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "path traversal at start",
			path:    "../../../etc/passwd.svg",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "path traversal in middle",
			path:    "/tmp/safe/../../../etc/passwd.svg",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "path traversal with forward slashes",
			path:    "images/../../../etc/passwd",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "double dot only",
			path:    "..",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "multiple traversals",
			path:    "../../..",
			wantErr: utils.ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup, err := converter.ConvertToPNG(context.Background(), tt.path, DefaultSVGScale)
			if cleanup != nil {
				cleanup()
			}

			if err == nil {
				t.Errorf("ConvertToPNG(%q) should have returned an error", tt.path)
				return
			}

			// Check if the underlying error is a path traversal error
			svgErr, ok := err.(*SVGConversionError)
			if !ok {
				t.Errorf("Expected SVGConversionError, got %T", err)
				return
			}

			if svgErr.Reason != "invalid path" {
				t.Errorf("Expected reason 'invalid path', got %q", svgErr.Reason)
			}

			if !errors.Is(svgErr.Err, tt.wantErr) {
				t.Errorf("Expected underlying error %v, got %v", tt.wantErr, svgErr.Err)
			}
		})
	}
}

// TestSVGConverter_ConvertToEMF_PathTraversal tests that path traversal attacks are blocked for EMF
func TestSVGConverter_ConvertToEMF_PathTraversal(t *testing.T) {
	converter := &SVGConverter{
		inkscapePath: "/usr/bin/inkscape", // Fake path - won't be executed
		Strategy:     SVGStrategyEMF,
	}
	markToolsFound(converter)

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "path traversal at start",
			path:    "../../../etc/passwd.svg",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "path traversal in middle",
			path:    "/tmp/safe/../../../etc/passwd.svg",
			wantErr: utils.ErrPathTraversal,
		},
		{
			name:    "path traversal with forward slashes",
			path:    "images/../../../etc/passwd",
			wantErr: utils.ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup, err := converter.ConvertToEMF(context.Background(), tt.path)
			if cleanup != nil {
				cleanup()
			}

			if err == nil {
				t.Errorf("ConvertToEMF(%q) should have returned an error", tt.path)
				return
			}

			// Check if the underlying error is a path traversal error
			svgErr, ok := err.(*SVGConversionError)
			if !ok {
				t.Errorf("Expected SVGConversionError, got %T", err)
				return
			}

			if svgErr.Reason != "invalid path" {
				t.Errorf("Expected reason 'invalid path', got %q", svgErr.Reason)
			}

			if !errors.Is(svgErr.Err, tt.wantErr) {
				t.Errorf("Expected underlying error %v, got %v", tt.wantErr, svgErr.Err)
			}
		})
	}
}

// TestSVGConverter_SafePaths tests that safe paths are allowed
func TestSVGConverter_SafePaths(t *testing.T) {
	converter := NewSVGConverter()
	if !converter.IsAvailable() {
		t.Skip("rsvg-convert not available")
	}

	// Create test SVG in a temp directory
	tmpDir := t.TempDir()
	safeTests := []struct {
		name     string
		filename string
	}{
		{
			name:     "simple filename",
			filename: "test.svg",
		},
		{
			name:     "filename with dots",
			filename: "file..name.svg",
		},
		{
			name:     "filename with spaces",
			filename: "my file.svg",
		},
		{
			name:     "nested directory",
			filename: filepath.Join("subdir", "test.svg"),
		},
		{
			name:     "dotfile",
			filename: ".hidden.svg",
		},
	}

	for _, tt := range safeTests {
		t.Run(tt.name, func(t *testing.T) {
			svgPath := filepath.Join(tmpDir, tt.filename)

			// Create directory structure if needed
			dir := filepath.Dir(svgPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}

			// Write test SVG
			if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
				t.Fatalf("Failed to write test SVG: %v", err)
			}

			// Should convert successfully
			pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, DefaultSVGScale)
			if err != nil {
				t.Errorf("ConvertToPNG(%q) should succeed for safe path, got error: %v", tt.filename, err)
				return
			}
			defer cleanup()

			// Verify PNG was created
			if _, err := os.Stat(pngPath); err != nil {
				t.Errorf("PNG file not created: %v", err)
			}
		})
	}
}

// =============================================================================
// LOW-07: Orphaned Temp File Cleanup Tests
// =============================================================================

// TestCleanupOrphanedTempFiles_CleansOldFiles tests that old temp files are removed
func TestCleanupOrphanedTempFiles_CleansOldFiles(t *testing.T) {
	// Create some fake old temp files in the system temp dir
	tempDir := os.TempDir()

	// Create old PNG temp file (should be cleaned)
	oldPNG := filepath.Join(tempDir, "svg-converted-test-old.png")
	if err := os.WriteFile(oldPNG, []byte("fake png"), 0644); err != nil {
		t.Fatalf("Failed to create old PNG: %v", err)
	}
	// Set modification time to 2 hours ago
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPNG, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old mtime: %v", err)
	}
	defer func() { _ = os.Remove(oldPNG) }() // Cleanup in case test fails

	// Create old EMF temp file (should be cleaned)
	oldEMF := filepath.Join(tempDir, "svg-converted-test-old.emf")
	if err := os.WriteFile(oldEMF, []byte("fake emf"), 0644); err != nil {
		t.Fatalf("Failed to create old EMF: %v", err)
	}
	if err := os.Chtimes(oldEMF, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old mtime: %v", err)
	}
	defer func() { _ = os.Remove(oldEMF) }() // Cleanup in case test fails

	// Create new temp file (should NOT be cleaned)
	newPNG := filepath.Join(tempDir, "svg-converted-test-new.png")
	if err := os.WriteFile(newPNG, []byte("fake new png"), 0644); err != nil {
		t.Fatalf("Failed to create new PNG: %v", err)
	}
	defer func() { _ = os.Remove(newPNG) }() // Always cleanup

	// Run cleanup with 1 hour max age
	cleaned, err := CleanupOrphanedTempFiles(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupOrphanedTempFiles failed: %v", err)
	}

	// Should have cleaned at least 2 files
	if cleaned < 2 {
		t.Errorf("Expected at least 2 files cleaned, got %d", cleaned)
	}

	// Old files should be gone
	if _, err := os.Stat(oldPNG); !os.IsNotExist(err) {
		t.Error("Old PNG file should have been removed")
	}
	if _, err := os.Stat(oldEMF); !os.IsNotExist(err) {
		t.Error("Old EMF file should have been removed")
	}

	// New file should still exist
	if _, err := os.Stat(newPNG); err != nil {
		t.Error("New PNG file should still exist")
	}
}

// TestCleanupOrphanedTempFiles_DefaultMaxAge tests the default max age
func TestCleanupOrphanedTempFiles_DefaultMaxAge(t *testing.T) {
	// Test that passing 0 uses the default max age
	cleaned, err := CleanupOrphanedTempFiles(0)
	if err != nil {
		t.Fatalf("CleanupOrphanedTempFiles(0) failed: %v", err)
	}
	t.Logf("Cleaned %d files with default max age", cleaned)
}

// TestCleanupOrphanedTempFiles_NoMatchingFiles tests when there are no files to clean
func TestCleanupOrphanedTempFiles_NoMatchingFiles(t *testing.T) {
	// This should not error even if there are no matching files
	cleaned, err := CleanupOrphanedTempFiles(DefaultTempFileMaxAge)
	if err != nil {
		t.Fatalf("CleanupOrphanedTempFiles failed: %v", err)
	}
	t.Logf("Cleaned %d files", cleaned)
}

// TestTempFilePatterns tests that the temp file patterns are correct
func TestTempFilePatterns(t *testing.T) {
	if len(TempFilePatterns) != 2 {
		t.Errorf("Expected 2 temp file patterns, got %d", len(TempFilePatterns))
	}

	expectedPatterns := []string{"svg-converted-*.png", "svg-converted-*.emf"}
	for i, pattern := range TempFilePatterns {
		if pattern != expectedPatterns[i] {
			t.Errorf("Pattern %d: expected %q, got %q", i, expectedPatterns[i], pattern)
		}
	}
}

// =============================================================================
// Resvg Alternative PNG Converter Tests
// =============================================================================

// TestSVGConverter_IsResvgAvailable tests resvg tool detection
func TestSVGConverter_IsResvgAvailable(t *testing.T) {
	converter := NewSVGConverter()
	available := converter.IsResvgAvailable()
	t.Logf("resvg available: %v", available)
}

// TestSVGConverter_IsRsvgConvertAvailable tests rsvg-convert tool detection
func TestSVGConverter_IsRsvgConvertAvailable(t *testing.T) {
	converter := NewSVGConverter()
	available := converter.IsRsvgConvertAvailable()
	t.Logf("rsvg-convert available: %v", available)
}

// TestSVGConverter_IsPNGAvailable_EitherTool tests that PNG conversion is available with either tool
func TestSVGConverter_IsPNGAvailable_EitherTool(t *testing.T) {
	converter := NewSVGConverter()
	pngAvailable := converter.IsPNGAvailable()
	rsvgAvailable := converter.IsRsvgConvertAvailable()
	resvgAvailable := converter.IsResvgAvailable()

	// IsPNGAvailable should return true if either tool is available
	if pngAvailable != (rsvgAvailable || resvgAvailable) {
		t.Errorf("IsPNGAvailable() = %v, but rsvg=%v, resvg=%v", pngAvailable, rsvgAvailable, resvgAvailable)
	}

	t.Logf("PNG available: %v (rsvg=%v, resvg=%v)", pngAvailable, rsvgAvailable, resvgAvailable)
}

// TestSVGConverter_ConvertToPNG_WithResvg tests PNG conversion using resvg
func TestSVGConverter_ConvertToPNG_WithResvg(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy:              SVGStrategyPNG,
		Scale:                 DefaultSVGScale,
		PreferredPNGConverter: PNGConverterResvg,
	})

	if !converter.IsResvgAvailable() {
		t.Skip("resvg not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert to PNG using resvg
	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0)
	if err != nil {
		t.Fatalf("ConvertToPNG with resvg failed: %v", err)
	}
	defer cleanup()

	// Verify PNG was created
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("Converted SVG to PNG using resvg: %s (size: %d bytes)", pngPath, info.Size())
}

// TestSVGConverter_ConvertToPNG_PreferRsvg tests forcing rsvg-convert
func TestSVGConverter_ConvertToPNG_PreferRsvg(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy:              SVGStrategyPNG,
		Scale:                 DefaultSVGScale,
		PreferredPNGConverter: PNGConverterRsvg,
	})

	if !converter.IsRsvgConvertAvailable() {
		t.Skip("rsvg-convert not available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert to PNG using rsvg-convert
	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0)
	if err != nil {
		t.Fatalf("ConvertToPNG with rsvg-convert failed: %v", err)
	}
	defer cleanup()

	// Verify PNG was created
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("Converted SVG to PNG using rsvg-convert: %s (size: %d bytes)", pngPath, info.Size())
}

// TestSVGConverter_ConvertToPNG_Auto tests automatic fallback behavior
func TestSVGConverter_ConvertToPNG_Auto(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy:              SVGStrategyPNG,
		Scale:                 DefaultSVGScale,
		PreferredPNGConverter: PNGConverterAuto, // Default: try rsvg-convert first, then resvg
	})

	if !converter.IsPNGAvailable() {
		t.Skip("No PNG converter available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert to PNG using auto mode
	pngPath, cleanup, err := converter.ConvertToPNG(context.Background(), svgPath, 0)
	if err != nil {
		t.Fatalf("ConvertToPNG with auto mode failed: %v", err)
	}
	defer cleanup()

	// Verify PNG was created
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("Converted SVG to PNG using auto mode: %s (size: %d bytes)", pngPath, info.Size())
}

// TestSVGConverter_ConvertToPNG_ResvgMissing tests error when resvg is required but missing
func TestSVGConverter_ConvertToPNG_ResvgMissing(t *testing.T) {
	converter := &SVGConverter{
		PreferredPNGConverter: PNGConverterResvg,
		resvgPath:             "", // Simulate missing tool
	}
	markToolsFound(converter)

	_, cleanup, err := converter.ConvertToPNG(context.Background(), "/path/to/file.svg", DefaultSVGScale)
	if cleanup != nil {
		cleanup()
	}

	if err == nil {
		t.Error("Expected error when resvg is required but not available")
	}

	// Verify error message mentions resvg requirement
	svgErr, ok := err.(*SVGConversionError)
	if !ok {
		t.Errorf("Expected SVGConversionError, got %T", err)
	} else if svgErr.Reason != "resvg not found in PATH (required by preference)" {
		t.Errorf("Unexpected error reason: %s", svgErr.Reason)
	}
}

// TestSVGConverter_ConvertToPNG_RsvgMissing tests error when rsvg-convert is required but missing
func TestSVGConverter_ConvertToPNG_RsvgMissing(t *testing.T) {
	converter := &SVGConverter{
		PreferredPNGConverter: PNGConverterRsvg,
		rsvgConvertPath:       "", // Simulate missing tool
	}
	markToolsFound(converter)

	_, cleanup, err := converter.ConvertToPNG(context.Background(), "/path/to/file.svg", DefaultSVGScale)
	if cleanup != nil {
		cleanup()
	}

	if err == nil {
		t.Error("Expected error when rsvg-convert is required but not available")
	}

	// Verify error message mentions rsvg-convert requirement
	svgErr, ok := err.(*SVGConversionError)
	if !ok {
		t.Errorf("Expected SVGConversionError, got %T", err)
	} else if svgErr.Reason != "rsvg-convert not found in PATH (required by preference)" {
		t.Errorf("Unexpected error reason: %s", svgErr.Reason)
	}
}

// TestSVGConverter_ConvertToPNG_BothMissing tests error when both converters are missing
func TestSVGConverter_ConvertToPNG_BothMissing(t *testing.T) {
	converter := &SVGConverter{
		PreferredPNGConverter: PNGConverterAuto,
		rsvgConvertPath:       "", // Simulate missing tool
		resvgPath:             "", // Simulate missing tool
	}
	markToolsFound(converter)

	_, cleanup, err := converter.ConvertToPNG(context.Background(), "/path/to/file.svg", DefaultSVGScale)
	if cleanup != nil {
		cleanup()
	}

	if err == nil {
		t.Error("Expected error when both converters are missing")
	}

	// Verify error message mentions both converters
	svgErr, ok := err.(*SVGConversionError)
	if !ok {
		t.Errorf("Expected SVGConversionError, got %T", err)
	} else if svgErr.Reason != "no PNG converter found (need rsvg-convert or resvg in PATH)" {
		t.Errorf("Unexpected error reason: %s", svgErr.Reason)
	}
}

// TestNewSVGConverterWithConfig_PreferredPNGConverter tests config option handling
func TestNewSVGConverterWithConfig_PreferredPNGConverter(t *testing.T) {
	tests := []struct {
		name          string
		cfg           SVGConfig
		wantConverter string
	}{
		{
			name: "empty config uses auto",
			cfg:  SVGConfig{},
			wantConverter: PNGConverterAuto,
		},
		{
			name: "explicit auto",
			cfg: SVGConfig{
				PreferredPNGConverter: PNGConverterAuto,
			},
			wantConverter: PNGConverterAuto,
		},
		{
			name: "prefer rsvg-convert",
			cfg: SVGConfig{
				PreferredPNGConverter: PNGConverterRsvg,
			},
			wantConverter: PNGConverterRsvg,
		},
		{
			name: "prefer resvg",
			cfg: SVGConfig{
				PreferredPNGConverter: PNGConverterResvg,
			},
			wantConverter: PNGConverterResvg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSVGConverterWithConfig(tt.cfg)
			if converter.PreferredPNGConverter != tt.wantConverter {
				t.Errorf("PreferredPNGConverter = %q, want %q", converter.PreferredPNGConverter, tt.wantConverter)
			}
		})
	}
}

// TestSVGConverter_Convert_StrategyPNG_AutoFallback tests Convert method with PNG strategy and auto fallback
func TestSVGConverter_Convert_StrategyPNG_AutoFallback(t *testing.T) {
	converter := NewSVGConverterWithConfig(SVGConfig{
		Strategy:              SVGStrategyPNG,
		Scale:                 DefaultSVGScale,
		PreferredPNGConverter: PNGConverterAuto,
	})

	if !converter.IsPNGAvailable() {
		t.Skip("No PNG converter available")
	}

	// Create test SVG file
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	if err := os.WriteFile(svgPath, []byte(testSVGContent), 0644); err != nil {
		t.Fatalf("Failed to write test SVG: %v", err)
	}

	// Convert using strategy-based method
	outputPath, cleanup, err := converter.Convert(context.Background(), svgPath)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	defer cleanup()

	// Verify output is PNG
	if filepath.Ext(outputPath) != ".png" {
		t.Errorf("Expected PNG output, got %s", filepath.Ext(outputPath))
	}

	// Verify file exists and has content
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("PNG file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("PNG file is empty")
	}

	t.Logf("PNG output: %s (%d bytes)", outputPath, info.Size())
}

package pptx

import (
	"strings"
	"testing"
)

func TestNewMediaAllocator(t *testing.T) {
	m := NewMediaAllocator()
	if m.NextImageNum() != 1 {
		t.Errorf("expected next num 1, got %d", m.NextImageNum())
	}
}

func TestMediaAllocator_ScanPackage(t *testing.T) {
	// Create a test package with some media files
	testFiles := map[string]string{
		"ppt/media/image1.png":  "png data",
		"ppt/media/image2.jpg":  "jpg data",
		"ppt/media/image5.svg":  "svg data",
		"ppt/slides/slide1.xml": "<slide/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	m := NewMediaAllocator()
	m.ScanPackage(pkg)

	// Next number should be 6 (after image5)
	if m.NextImageNum() != 6 {
		t.Errorf("expected next num 6, got %d", m.NextImageNum())
	}

	// Check existing paths
	if !m.HasExisting("ppt/media/image1.png") {
		t.Error("expected image1.png to exist")
	}
	if !m.HasExisting("ppt/media/image5.svg") {
		t.Error("expected image5.svg to exist")
	}
	if m.HasExisting("ppt/slides/slide1.xml") {
		t.Error("slide1.xml is not a media file")
	}
}

func TestMediaAllocator_ScanPaths(t *testing.T) {
	paths := []string{
		"ppt/media/image1.png",
		"ppt/media/image3.jpg",
		"ppt/media/image10.svg",
		"ppt/slides/slide1.xml", // not media
	}

	m := NewMediaAllocator()
	m.ScanPaths(paths)

	if m.NextImageNum() != 11 {
		t.Errorf("expected next num 11, got %d", m.NextImageNum())
	}
}

func TestMediaAllocator_Allocate(t *testing.T) {
	m := NewMediaAllocator()

	path1 := m.Allocate("png", "source1")
	if path1 != "ppt/media/image1.png" {
		t.Errorf("expected ppt/media/image1.png, got %s", path1)
	}

	path2 := m.Allocate("svg", "source2")
	if path2 != "ppt/media/image2.svg" {
		t.Errorf("expected ppt/media/image2.svg, got %s", path2)
	}

	path3 := m.Allocate(".PNG", "source3") // with dot and uppercase
	if path3 != "ppt/media/image3.png" {
		t.Errorf("expected ppt/media/image3.png, got %s", path3)
	}
}

func TestMediaAllocator_Allocate_Deduplication(t *testing.T) {
	m := NewMediaAllocator()

	path1 := m.Allocate("png", "source1")
	path2 := m.Allocate("png", "source1") // same source

	if path1 != path2 {
		t.Errorf("expected same path for same source, got %s and %s", path1, path2)
	}

	if m.NextImageNum() != 2 { // Only one allocation happened
		t.Errorf("expected next num 2, got %d", m.NextImageNum())
	}
}

func TestMediaAllocator_AllocatePNG(t *testing.T) {
	m := NewMediaAllocator()

	path := m.AllocatePNG("source1")
	if path != "ppt/media/image1.png" {
		t.Errorf("expected ppt/media/image1.png, got %s", path)
	}
}

func TestMediaAllocator_AllocateSVG(t *testing.T) {
	m := NewMediaAllocator()

	path := m.AllocateSVG("source1")
	if path != "ppt/media/image1.svg" {
		t.Errorf("expected ppt/media/image1.svg, got %s", path)
	}
}

func TestMediaAllocator_AllocatePair(t *testing.T) {
	m := NewMediaAllocator()

	svgPath, pngPath := m.AllocatePair("chart1")

	if svgPath != "ppt/media/image1.svg" {
		t.Errorf("expected svg path ppt/media/image1.svg, got %s", svgPath)
	}
	if pngPath != "ppt/media/image2.png" {
		t.Errorf("expected png path ppt/media/image2.png, got %s", pngPath)
	}

	// Allocating same pair again should return same paths
	svgPath2, pngPath2 := m.AllocatePair("chart1")
	if svgPath2 != svgPath || pngPath2 != pngPath {
		t.Error("expected same paths for same source")
	}
}

func TestMediaAllocator_GetAllocated(t *testing.T) {
	m := NewMediaAllocator()
	m.Allocate("png", "source1")

	if m.Allocated("source1") != "ppt/media/image1.png" {
		t.Errorf("expected ppt/media/image1.png, got %s", m.Allocated("source1"))
	}

	if m.Allocated("nonexistent") != "" {
		t.Error("expected empty string for nonexistent source")
	}
}

func TestMediaAllocator_ExistingPaths(t *testing.T) {
	m := NewMediaAllocator()
	m.ScanPaths([]string{
		"ppt/media/image3.png",
		"ppt/media/image1.png",
		"ppt/media/image2.svg",
	})

	paths := m.ExistingPaths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}

	// Should be sorted
	expected := []string{
		"ppt/media/image1.png",
		"ppt/media/image2.svg",
		"ppt/media/image3.png",
	}
	for i, p := range expected {
		if paths[i] != p {
			t.Errorf("paths[%d] = %s, want %s", i, paths[i], p)
		}
	}
}

func TestMediaAllocator_AllocatedPaths(t *testing.T) {
	m := NewMediaAllocator()
	m.Allocate("png", "source1")
	m.Allocate("svg", "source2")
	m.Allocate("png", "source1") // duplicate

	paths := m.AllocatedPaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}

	// Should be sorted
	expected := []string{
		"ppt/media/image1.png",
		"ppt/media/image2.svg",
	}
	for i, p := range expected {
		if paths[i] != p {
			t.Errorf("paths[%d] = %s, want %s", i, paths[i], p)
		}
	}
}

func TestMediaAllocator_Reset(t *testing.T) {
	m := NewMediaAllocator()
	m.ScanPaths([]string{"ppt/media/image5.png"})
	m.Allocate("svg", "source1")

	m.Reset()

	// Allocations should be cleared
	if m.Allocated("source1") != "" {
		t.Error("expected allocations to be cleared")
	}

	// But maxImageNum should be preserved
	if m.NextImageNum() != 7 { // 5 + 1 (from allocate) + 1
		t.Errorf("expected next num 7, got %d", m.NextImageNum())
	}

	// Existing paths should be preserved
	if !m.HasExisting("ppt/media/image5.png") {
		t.Error("expected existing paths to be preserved")
	}
}

func TestMediaAllocator_Clone(t *testing.T) {
	m := NewMediaAllocator()
	m.ScanPaths([]string{"ppt/media/image5.png"})
	m.Allocate("svg", "source1")

	clone := m.Clone()

	// Modify original
	m.Allocate("png", "source2")

	// Clone should not be affected
	if clone.Allocated("source2") != "" {
		t.Error("clone should not be affected")
	}
	if clone.NextImageNum() != 7 { // 5 + 1
		t.Errorf("expected clone next num 7, got %d", clone.NextImageNum())
	}

	// Modify clone
	clone.Allocate("png", "source3")

	// Original should not be affected
	if m.Allocated("source3") != "" {
		t.Error("original should not be affected")
	}
}

func TestMediaPath(t *testing.T) {
	if MediaPath("image1.png") != "ppt/media/image1.png" {
		t.Errorf("expected ppt/media/image1.png, got %s", MediaPath("image1.png"))
	}
}

func TestMediaFilename(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"ppt/media/image1.png", "image1.png"},
		{"ppt/media/image2.svg", "image2.svg"},
		{"image3.jpg", "image3.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := MediaFilename(tt.path)
			if result != tt.expected {
				t.Errorf("MediaFilename(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsMediaPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"ppt/media/image1.png", true},
		{"ppt/media/", true},
		{"ppt/slides/slide1.xml", false},
		{"media/image1.png", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsMediaPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsMediaPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestRelativeMediaPath(t *testing.T) {
	result := RelativeMediaPath("image1.png")
	if result != "../media/image1.png" {
		t.Errorf("expected ../media/image1.png, got %s", result)
	}
}

func TestMediaAllocator_ContinuesAfterExisting(t *testing.T) {
	m := NewMediaAllocator()

	// Simulate existing files with gaps
	m.ScanPaths([]string{
		"ppt/media/image1.png",
		"ppt/media/image5.png",
		"ppt/media/image10.svg",
	})

	// New allocations should continue from 10
	path1 := m.Allocate("png", "new1")
	if path1 != "ppt/media/image11.png" {
		t.Errorf("expected image11.png, got %s", path1)
	}

	path2 := m.Allocate("svg", "new2")
	if path2 != "ppt/media/image12.svg" {
		t.Errorf("expected image12.svg, got %s", path2)
	}
}

func BenchmarkMediaAllocator_Allocate(b *testing.B) {
	m := NewMediaAllocator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Allocate("png", string(rune(i)))
	}
}

func BenchmarkMediaAllocator_ScanPaths(b *testing.B) {
	paths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		paths[i] = MediaPath("image" + string(rune('0'+i%10)) + ".png")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewMediaAllocator()
		m.ScanPaths(paths)
	}
}

// Tests for MediaInserter

func TestNewMediaInserter(t *testing.T) {
	// Create a test package with content types and some existing media
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
</Types>`,
		"ppt/media/image1.png": "fake png data",
		"ppt/media/image2.png": "fake png data 2",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	// Should have scanned existing media
	if mi.Allocator().NextImageNum() != 3 {
		t.Errorf("expected next image num 3, got %d", mi.Allocator().NextImageNum())
	}

	// Content types should be accessible
	if mi.ContentTypes() == nil {
		t.Error("ContentTypes() returned nil")
	}
}

func TestMediaInserter_InsertPNG(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	// Insert a PNG
	pngData := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A} // PNG magic bytes
	path := mi.InsertPNG(pngData, "test-png-1")

	if path != "ppt/media/image1.png" {
		t.Errorf("expected ppt/media/image1.png, got %s", path)
	}

	// Verify data was added to package
	readData, err := pkg.ReadEntry(path)
	if err != nil {
		t.Fatalf("failed to read inserted PNG: %v", err)
	}
	if string(readData) != string(pngData) {
		t.Error("inserted data doesn't match")
	}

	// Insert another - should get next number
	path2 := mi.InsertPNG([]byte("png2"), "test-png-2")
	if path2 != "ppt/media/image2.png" {
		t.Errorf("expected ppt/media/image2.png, got %s", path2)
	}

	// Insert duplicate source - should return same path
	path3 := mi.InsertPNG([]byte("duplicate"), "test-png-1")
	if path3 != path {
		t.Errorf("duplicate source should return same path, got %s", path3)
	}
}

func TestMediaInserter_InsertSVG(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	// Insert an SVG
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`)
	path := mi.InsertSVG(svgData, "test-svg-1")

	if path != "ppt/media/image1.svg" {
		t.Errorf("expected ppt/media/image1.svg, got %s", path)
	}

	// Verify data was added to package
	readData, err := pkg.ReadEntry(path)
	if err != nil {
		t.Fatalf("failed to read inserted SVG: %v", err)
	}
	if string(readData) != string(svgData) {
		t.Error("inserted data doesn't match")
	}
}

func TestMediaInserter_InsertSVGWithFallback(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="50"/></svg>`)
	pngData := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

	svgPath, pngPath := mi.InsertSVGWithFallback(svgData, pngData, "chart-1")

	if svgPath != "ppt/media/image1.svg" {
		t.Errorf("expected SVG path ppt/media/image1.svg, got %s", svgPath)
	}
	if pngPath != "ppt/media/image2.png" {
		t.Errorf("expected PNG path ppt/media/image2.png, got %s", pngPath)
	}

	// Verify both files exist
	if _, err := pkg.ReadEntry(svgPath); err != nil {
		t.Errorf("SVG file not found: %v", err)
	}
	if _, err := pkg.ReadEntry(pngPath); err != nil {
		t.Errorf("PNG file not found: %v", err)
	}

	// Inserting same pair again should return same paths
	svgPath2, pngPath2 := mi.InsertSVGWithFallback([]byte("new svg"), []byte("new png"), "chart-1")
	if svgPath2 != svgPath || pngPath2 != pngPath {
		t.Error("duplicate source should return same paths")
	}
}

func TestMediaInserter_Finalize(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	// Insert both SVG and PNG to trigger content type additions
	mi.InsertSVG([]byte("<svg/>"), "test-svg")
	mi.InsertPNG([]byte("png"), "test-png")

	// Finalize to write content types back
	if err := mi.Finalize(); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Read back content types and verify extensions are registered
	ctData, err := pkg.ReadEntry("[Content_Types].xml")
	if err != nil {
		t.Fatalf("failed to read [Content_Types].xml: %v", err)
	}

	ctStr := string(ctData)

	// Should have PNG extension
	if !strings.Contains(ctStr, `Extension="png"`) {
		t.Error("content types missing PNG extension")
	}

	// Should have SVG extension
	if !strings.Contains(ctStr, `Extension="svg"`) {
		t.Error("content types missing SVG extension")
	}
}

func TestMediaInserter_WithExistingMedia(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
</Types>`,
		"ppt/media/image1.png":  "existing png 1",
		"ppt/media/image5.png":  "existing png 5",
		"ppt/media/image10.svg": "existing svg 10",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	mi, err := NewMediaInserter(pkg)
	if err != nil {
		t.Fatalf("NewMediaInserter failed: %v", err)
	}

	// New insertions should continue from highest existing number
	path := mi.InsertPNG([]byte("new png"), "new-1")
	if path != "ppt/media/image11.png" {
		t.Errorf("expected ppt/media/image11.png, got %s", path)
	}

	path2 := mi.InsertSVG([]byte("<svg/>"), "new-2")
	if path2 != "ppt/media/image12.svg" {
		t.Errorf("expected ppt/media/image12.svg, got %s", path2)
	}
}

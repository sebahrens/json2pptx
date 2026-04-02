package pptx

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestRectFromInches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		x, y, width, height  float64
		wantX, wantY, wantCX int64
		wantCY               int64
	}{
		{
			name: "one inch square at origin",
			x:    0, y: 0, width: 1, height: 1,
			wantX: 0, wantY: 0, wantCX: 914400, wantCY: 914400,
		},
		{
			name: "standard chart size",
			x:    1, y: 2, width: 5, height: 3,
			wantX: 914400, wantY: 1828800, wantCX: 4572000, wantCY: 2743200,
		},
		{
			name: "half inch precision",
			x:    0.5, y: 0.5, width: 2.5, height: 1.5,
			wantX: 457200, wantY: 457200, wantCX: 2286000, wantCY: 1371600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RectFromInches(tt.x, tt.y, tt.width, tt.height)
			if r.X != tt.wantX {
				t.Errorf("X = %d, want %d", r.X, tt.wantX)
			}
			if r.Y != tt.wantY {
				t.Errorf("Y = %d, want %d", r.Y, tt.wantY)
			}
			if r.CX != tt.wantCX {
				t.Errorf("CX = %d, want %d", r.CX, tt.wantCX)
			}
			if r.CY != tt.wantCY {
				t.Errorf("CY = %d, want %d", r.CY, tt.wantCY)
			}
		})
	}
}

func TestRectFromCM(t *testing.T) {
	t.Parallel()

	r := RectFromCM(2.54, 2.54, 12.7, 7.62)
	// 2.54cm = 1 inch = 914400 EMU
	if r.X != 914400 {
		t.Errorf("X = %d, want 914400", r.X)
	}
	if r.Y != 914400 {
		t.Errorf("Y = %d, want 914400", r.Y)
	}
	// 12.7cm = 5 inches
	if r.CX != 4572000 {
		t.Errorf("CX = %d, want 4572000", r.CX)
	}
	// 7.62cm = 3 inches
	if r.CY != 2743200 {
		t.Errorf("CY = %d, want 2743200", r.CY)
	}
}

func TestRectFromPixels(t *testing.T) {
	t.Parallel()

	r := RectFromPixels(96, 96, 480, 288)
	// 96px at 96 DPI = 1 inch = 914400 EMU
	if r.X != 914400 {
		t.Errorf("X = %d, want 914400", r.X)
	}
	if r.Y != 914400 {
		t.Errorf("Y = %d, want 914400", r.Y)
	}
	// 480px at 96 DPI = 5 inches
	if r.CX != 4572000 {
		t.Errorf("CX = %d, want 4572000", r.CX)
	}
	// 288px at 96 DPI = 3 inches
	if r.CY != 2743200 {
		t.Errorf("CY = %d, want 2743200", r.CY)
	}
}

func TestRectToInches(t *testing.T) {
	t.Parallel()

	r := RectEmu{X: 914400, Y: 1828800, CX: 4572000, CY: 2743200}
	x, y, w, h := r.ToInches()

	if x != 1.0 {
		t.Errorf("x = %f, want 1.0", x)
	}
	if y != 2.0 {
		t.Errorf("y = %f, want 2.0", y)
	}
	if w != 5.0 {
		t.Errorf("width = %f, want 5.0", w)
	}
	if h != 3.0 {
		t.Errorf("height = %f, want 3.0", h)
	}
}

func TestRectToCM(t *testing.T) {
	t.Parallel()

	r := RectEmu{X: 360000, Y: 720000, CX: 1800000, CY: 1080000}
	x, y, w, h := r.ToCM()

	if x != 1.0 {
		t.Errorf("x = %f, want 1.0", x)
	}
	if y != 2.0 {
		t.Errorf("y = %f, want 2.0", y)
	}
	if w != 5.0 {
		t.Errorf("width = %f, want 5.0", w)
	}
	if h != 3.0 {
		t.Errorf("height = %f, want 3.0", h)
	}
}

func TestRectToPixels(t *testing.T) {
	t.Parallel()

	r := RectEmu{X: 9525, Y: 19050, CX: 47625, CY: 28575}
	x, y, w, h := r.ToPixels()

	if x != 1 {
		t.Errorf("x = %d, want 1", x)
	}
	if y != 2 {
		t.Errorf("y = %d, want 2", y)
	}
	if w != 5 {
		t.Errorf("width = %d, want 5", w)
	}
	if h != 3 {
		t.Errorf("height = %d, want 3", h)
	}
}

func TestRectIsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rect RectEmu
		want bool
	}{
		{"zero rect", RectEmu{}, true},
		{"zero width", RectEmu{X: 100, Y: 100, CX: 0, CY: 100}, true},
		{"zero height", RectEmu{X: 100, Y: 100, CX: 100, CY: 0}, true},
		{"non-zero", RectEmu{X: 0, Y: 0, CX: 100, CY: 100}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rect.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectContains(t *testing.T) {
	t.Parallel()

	r := RectEmu{X: 100, Y: 100, CX: 200, CY: 200}

	tests := []struct {
		name   string
		px, py int64
		want   bool
	}{
		{"top-left corner", 100, 100, true},
		{"inside", 150, 150, true},
		{"bottom-right edge (exclusive)", 300, 300, false},
		{"outside left", 50, 150, false},
		{"outside right", 350, 150, false},
		{"outside top", 150, 50, false},
		{"outside bottom", 150, 350, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Contains(tt.px, tt.py); got != tt.want {
				t.Errorf("Contains(%d, %d) = %v, want %v", tt.px, tt.py, got, tt.want)
			}
		})
	}
}

func TestRectIntersects(t *testing.T) {
	t.Parallel()

	r := RectEmu{X: 100, Y: 100, CX: 200, CY: 200}

	tests := []struct {
		name  string
		other RectEmu
		want  bool
	}{
		{"same rect", RectEmu{X: 100, Y: 100, CX: 200, CY: 200}, true},
		{"overlapping", RectEmu{X: 150, Y: 150, CX: 200, CY: 200}, true},
		{"touching right (no overlap)", RectEmu{X: 300, Y: 100, CX: 100, CY: 100}, false},
		{"touching bottom (no overlap)", RectEmu{X: 100, Y: 300, CX: 100, CY: 100}, false},
		{"inside", RectEmu{X: 150, Y: 150, CX: 50, CY: 50}, true},
		{"containing", RectEmu{X: 0, Y: 0, CX: 500, CY: 500}, true},
		{"left of", RectEmu{X: 0, Y: 100, CX: 50, CY: 100}, false},
		{"above", RectEmu{X: 100, Y: 0, CX: 100, CY: 50}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Intersects(tt.other); got != tt.want {
				t.Errorf("Intersects(%+v) = %v, want %v", tt.other, got, tt.want)
			}
		})
	}
}

func TestInsertOptions_Validation(t *testing.T) {
	t.Parallel()

	// Test that InsertOptions can hold all expected data
	opts := InsertOptions{
		SlideIndex: 0,
		Bounds: RectEmu{
			X:  914400,
			Y:  914400,
			CX: 4572000,
			CY: 2743200,
		},
		SVGData: []byte("<svg></svg>"),
		PNGData: []byte("\x89PNG\r\n\x1a\n"),
		Name:    "Test Image",
		AltText: "A test image description",
		ZOrder:  intPtr(5),
	}

	if opts.SlideIndex != 0 {
		t.Errorf("SlideIndex = %d, want 0", opts.SlideIndex)
	}
	if opts.Bounds.CX != 4572000 {
		t.Errorf("Bounds.CX = %d, want 4572000", opts.Bounds.CX)
	}
	if len(opts.SVGData) == 0 {
		t.Error("SVGData should not be empty")
	}
	if len(opts.PNGData) == 0 {
		t.Error("PNGData should not be empty")
	}
	if opts.Name != "Test Image" {
		t.Errorf("Name = %q, want %q", opts.Name, "Test Image")
	}
	if opts.AltText != "A test image description" {
		t.Errorf("AltText = %q, want %q", opts.AltText, "A test image description")
	}
	if opts.ZOrder == nil || *opts.ZOrder != 5 {
		t.Errorf("ZOrder = %v, want 5", opts.ZOrder)
	}
}

func intPtr(v int) *int {
	return &v
}

func TestOpenDocumentFromBytes(t *testing.T) {
	t.Parallel()

	// Create a minimal valid ZIP (which is the basis for PPTX)
	// This test verifies the Document wrapper works correctly
	zipData := createMinimalPPTX()

	doc, err := OpenDocumentFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	if doc.Package() == nil {
		t.Error("Package() returned nil")
	}
}

func TestDocumentSaveToBytes(t *testing.T) {
	t.Parallel()

	zipData := createMinimalPPTX()

	doc, err := OpenDocumentFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	saved, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	if len(saved) == 0 {
		t.Error("SaveToBytes returned empty data")
	}
}

func TestDocumentSave(t *testing.T) {
	t.Parallel()

	zipData := createMinimalPPTX()

	doc, err := OpenDocumentFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	var buf bytes.Buffer
	err = doc.Save(&buf)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Save wrote no data")
	}
}

// createMinimalPPTX creates a minimal valid PPTX for testing.
// Uses archive/zip directly to create a proper ZIP structure.
func createMinimalPPTX() []byte {
	return createTestZIPHelper(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
</Types>`,
	})
}

// createTestZIPHelper creates a test ZIP archive from a map of files.
func createTestZIPHelper(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			panic("failed to create zip entry: " + err.Error())
		}
		if _, err := w.Write([]byte(content)); err != nil {
			panic("failed to write zip entry: " + err.Error())
		}
	}

	if err := zw.Close(); err != nil {
		panic("failed to close zip: " + err.Error())
	}

	return buf.Bytes()
}

func TestEMUConstants(t *testing.T) {
	t.Parallel()

	// Verify EMU constants match the OOXML specification
	if EMUPerInch != 914400 {
		t.Errorf("EMUPerInch = %d, want 914400", EMUPerInch)
	}
	if EMUPerPoint != 12700 {
		t.Errorf("EMUPerPoint = %d, want 12700", EMUPerPoint)
	}
	if EMUPerCM != 360000 {
		t.Errorf("EMUPerCM = %d, want 360000", EMUPerCM)
	}
	if EMUPerPixel != 9525 {
		t.Errorf("EMUPerPixel = %d, want 9525", EMUPerPixel)
	}

	// Verify mathematical relationships
	// 72 points = 1 inch
	pointsPerInch := EMUPerInch / EMUPerPoint
	if pointsPerInch != 72 {
		t.Errorf("points per inch = %d, want 72", pointsPerInch)
	}

	// 96 pixels = 1 inch at 96 DPI
	pixelsPerInch := EMUPerInch / EMUPerPixel
	if pixelsPerInch != 96 {
		t.Errorf("pixels per inch = %d, want 96", pixelsPerInch)
	}
}

func TestRectRoundTrip(t *testing.T) {
	t.Parallel()

	// Test that conversion round-trips correctly
	original := RectFromInches(1.5, 2.5, 5.0, 3.0)
	x, y, w, h := original.ToInches()

	if x != 1.5 {
		t.Errorf("round-trip x = %f, want 1.5", x)
	}
	if y != 2.5 {
		t.Errorf("round-trip y = %f, want 2.5", y)
	}
	if w != 5.0 {
		t.Errorf("round-trip width = %f, want 5.0", w)
	}
	if h != 3.0 {
		t.Errorf("round-trip height = %f, want 3.0", h)
	}
}

// createTestPPTXWithSlide creates a minimal PPTX with one slide for testing.
func createTestPPTXWithSlide() []byte {
	return createTestZIPHelper(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr>
        <a:xfrm>
          <a:off x="0" y="0"/>
          <a:ext cx="0" cy="0"/>
          <a:chOff x="0" y="0"/>
          <a:chExt cx="0" cy="0"/>
        </a:xfrm>
      </p:grpSpPr>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	})
}

func TestInsertSVG_Success(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	// Test SVG and PNG data
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG magic header

	opts := InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 5, 3),
		SVGData:    svgData,
		PNGData:    pngData,
		Name:       "Test Chart",
		AltText:    "A test chart",
	}

	err = doc.InsertSVG(opts)
	if err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	// Verify the document can be saved
	saved, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// Verify the saved document can be opened
	doc2, err := OpenDocumentFromBytes(saved)
	if err != nil {
		t.Fatalf("Failed to open saved document: %v", err)
	}

	// Verify media files were added
	pkg := doc2.Package()
	entries := pkg.Entries()

	hasSVG := false
	hasPNG := false
	for _, entry := range entries {
		if entry == "ppt/media/image1.svg" {
			hasSVG = true
		}
		if entry == "ppt/media/image2.png" {
			hasPNG = true
		}
	}

	if !hasSVG {
		t.Error("SVG media file not found in package")
	}
	if !hasPNG {
		t.Error("PNG media file not found in package")
	}

	// Verify slide was modified to include p:pic
	slideXML, err := pkg.ReadEntry("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("Failed to read slide: %v", err)
	}

	if !bytes.Contains(slideXML, []byte("<p:pic")) {
		t.Error("p:pic element not found in slide")
	}
	if !bytes.Contains(slideXML, []byte(`name="Test Chart"`)) {
		t.Error("Shape name not found in slide")
	}
	if !bytes.Contains(slideXML, []byte(`descr="A test chart"`)) {
		t.Error("Alt text not found in slide")
	}

	// Verify slide relationships were created
	slideRelsXML, err := pkg.ReadEntry("ppt/slides/_rels/slide1.xml.rels")
	if err != nil {
		t.Fatalf("Failed to read slide rels: %v", err)
	}

	if !bytes.Contains(slideRelsXML, []byte("image")) {
		t.Error("Image relationship not found in slide rels")
	}
}

func TestInsertSVG_EmptySVGData(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	opts := InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 5, 3),
		SVGData:    nil, // Empty!
		PNGData:    []byte{0x89, 0x50, 0x4E, 0x47},
	}

	err = doc.InsertSVG(opts)
	if err == nil {
		t.Error("Expected error for empty SVGData, got nil")
	}
}

func TestInsertSVG_EmptyPNGData(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	opts := InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 5, 3),
		SVGData:    []byte("<svg></svg>"),
		PNGData:    nil, // Empty!
	}

	err = doc.InsertSVG(opts)
	if err == nil {
		t.Error("Expected error for empty PNGData, got nil")
	}
}

func TestInsertSVG_ZeroBounds(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	opts := InsertOptions{
		SlideIndex: 0,
		Bounds:     RectEmu{}, // Zero bounds!
		SVGData:    []byte("<svg></svg>"),
		PNGData:    []byte{0x89, 0x50, 0x4E, 0x47},
	}

	err = doc.InsertSVG(opts)
	if err == nil {
		t.Error("Expected error for zero bounds, got nil")
	}
}

func TestInsertSVG_InvalidSlideIndex(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	opts := InsertOptions{
		SlideIndex: 99, // Out of range!
		Bounds:     RectFromInches(1, 1, 5, 3),
		SVGData:    []byte("<svg></svg>"),
		PNGData:    []byte{0x89, 0x50, 0x4E, 0x47},
	}

	err = doc.InsertSVG(opts)
	if err == nil {
		t.Error("Expected error for invalid slide index, got nil")
	}
}

func TestSlideCount(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	count, err := doc.SlideCount()
	if err != nil {
		t.Fatalf("SlideCount failed: %v", err)
	}

	if count != 1 {
		t.Errorf("SlideCount = %d, want 1", count)
	}
}

func TestRelativePathFromSlide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mediaPath string
		want      string
	}{
		{"ppt/media/image1.png", "../media/image1.png"},
		{"ppt/media/image2.svg", "../media/image2.svg"},
		{"ppt/media/image100.jpeg", "../media/image100.jpeg"},
	}

	for _, tt := range tests {
		got := relativePathFromSlide(tt.mediaPath)
		if got != tt.want {
			t.Errorf("relativePathFromSlide(%q) = %q, want %q", tt.mediaPath, got, tt.want)
		}
	}
}

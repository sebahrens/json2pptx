// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"fmt"
	"io"
	"path"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Document represents an open PPTX document for editing.
//
// Document wraps the low-level Package type and provides a high-level API
// for common PPTX manipulation operations like inserting images.
//
// Create a Document using Open or OpenFile. After making modifications,
// call Save to write the changes. The caller is responsible for closing
// any underlying file handles.
//
// Example:
//
//	doc, closer, err := OpenDocument("template.pptx")
//	if err != nil {
//	    return err
//	}
//	defer closer.Close()
//
//	err = doc.InsertSVG(InsertOptions{
//	    SlideIndex: 0,
//	    Bounds:     RectEmu{X: 914400, Y: 914400, CX: 4572000, CY: 2743200},
//	    SVGData:    svgBytes,
//	    PNGData:    pngBytes,
//	})
//	if err != nil {
//	    return err
//	}
//
//	f, _ := os.Create("output.pptx")
//	defer f.Close()
//	return doc.Save(f)
type Document struct {
	pkg *Package
}

// RectEmu defines a rectangle in English Metric Units (EMUs).
//
// EMU is the standard unit used in OOXML documents. There are 914400 EMUs
// per inch, allowing for precise sub-pixel positioning.
//
// The rectangle is defined by an origin (X, Y) and extent (CX, CY):
//   - X:  Horizontal offset from the left edge of the slide
//   - Y:  Vertical offset from the top edge of the slide
//   - CX: Width (extent in the X direction)
//   - CY: Height (extent in the Y direction)
//
// Common conversions:
//   - 914400 EMU = 1 inch
//   - 12700 EMU = 1 point
//   - 360000 EMU = 1 cm
//   - 9525 EMU = 1 pixel (at 96 DPI)
//
// Example for a 5" x 3" rectangle starting 1" from top-left:
//
//	RectEmu{
//	    X:  914400,   // 1 inch from left
//	    Y:  914400,   // 1 inch from top
//	    CX: 4572000,  // 5 inches wide
//	    CY: 2743200,  // 3 inches tall
//	}
type RectEmu struct {
	X  int64 // Horizontal offset from slide left edge (EMU)
	Y  int64 // Vertical offset from slide top edge (EMU)
	CX int64 // Width (EMU)
	CY int64 // Height (EMU)
}

// InsertOptions configures the insertion of an SVG image into a slide.
//
// InsertOptions requires both SVG source data and a PNG fallback image.
// The PNG fallback is mandatory because:
//   - PowerPoint versions before 2016 cannot render SVG
//   - LibreOffice Impress uses the PNG fallback
//   - Older mobile viewers require raster images
//
// The PNG data should be a pre-rendered version of the SVG at appropriate
// resolution (typically 2x scale or 300 DPI for print quality).
//
// Example:
//
//	opts := InsertOptions{
//	    SlideIndex: 0,
//	    Bounds: RectEmu{
//	        X:  914400,   // 1 inch from left
//	        Y:  1828800,  // 2 inches from top
//	        CX: 3657600,  // 4 inches wide
//	        CY: 2743200,  // 3 inches tall
//	    },
//	    SVGData: svgBytes,
//	    PNGData: pngFallbackBytes,
//	    Name:    "Sales Chart Q4",
//	    AltText: "Bar chart showing Q4 sales by region",
//	}
type InsertOptions struct {
	// SlideIndex is the zero-based index of the target slide.
	// Slide indices start at 0 for the first slide.
	SlideIndex int

	// Bounds defines the position and size of the image on the slide.
	// All values are in EMUs (English Metric Units).
	Bounds RectEmu

	// SVGData is the raw SVG file content.
	// This is embedded as the primary image for PowerPoint 2016+.
	SVGData []byte

	// PNGData is the PNG fallback image content.
	// This is mandatory and used by older PowerPoint versions and other viewers.
	// The PNG should be pre-rendered at appropriate resolution.
	PNGData []byte

	// Name is the shape name visible in PowerPoint's selection pane.
	// If empty, defaults to "Picture N" where N is the shape ID.
	// Optional.
	Name string

	// AltText is the alternative text description for accessibility.
	// This is read by screen readers and shown when the image cannot load.
	// Optional but recommended for accessibility compliance.
	AltText string

	// ZOrder controls the stacking position of the image.
	// If nil, the image is added at the default position (typically on top).
	// Lower values are further back; higher values are on top.
	// Optional.
	ZOrder *int
}

// OpenDocumentFile opens a PPTX document from a file path.
//
// Returns the Document, a Closer for the underlying file, and any error.
// The caller must close the Closer when done with the Document.
//
// Example:
//
//	doc, closer, err := OpenDocumentFile("template.pptx")
//	if err != nil {
//	    return err
//	}
//	defer closer.Close()
//
//	// ... modify document ...
//
//	return doc.Save(outputFile)
func OpenDocumentFile(path string) (*Document, io.Closer, error) {
	pkg, closer, err := OpenFile(path)
	if err != nil {
		return nil, nil, err
	}
	return &Document{pkg: pkg}, closer, nil
}

// OpenDocumentFromBytes opens a PPTX document from a byte slice.
//
// This is a convenience function for in-memory operations.
// The byte slice is not copied, so it should not be modified
// while the Document is in use.
func OpenDocumentFromBytes(data []byte) (*Document, error) {
	pkg, err := OpenFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &Document{pkg: pkg}, nil
}

// Save writes the document to the given writer.
//
// The output is deterministic: entries are sorted alphabetically and
// timestamps are fixed to ensure identical input produces identical output.
//
// Example:
//
//	f, err := os.Create("output.pptx")
//	if err != nil {
//	    return err
//	}
//	defer f.Close()
//
//	return doc.Save(f)
func (d *Document) Save(w io.Writer) error {
	return d.pkg.Save(w)
}

// SaveToBytes saves the document to a byte slice.
//
// This is a convenience function for in-memory operations.
func (d *Document) SaveToBytes() ([]byte, error) {
	return d.pkg.SaveToBytes()
}

// Package returns the underlying Package for advanced operations.
//
// This provides access to low-level PPTX manipulation when the
// high-level Document API is insufficient.
func (d *Document) Package() *Package {
	return d.pkg
}

// Re-export EMU constants from types package for backward compatibility.
// New code should import types.EMUPerInch, etc. directly.
const (
	EMUPerInch  = int64(types.EMUPerInch)
	EMUPerPoint = int64(types.EMUPerPoint)
	EMUPerCM    = int64(types.EMUPerCM)
	EMUPerPixel = int64(types.EMUPerPixel)
)

// RectFromInches creates a RectEmu from inch values.
//
// This is a convenience function for specifying bounds in inches.
//
// Example:
//
//	bounds := RectFromInches(1.0, 1.5, 5.0, 3.0) // x=1", y=1.5", 5" wide, 3" tall
func RectFromInches(x, y, width, height float64) RectEmu {
	return RectEmu{
		X:  types.FromInches(x).Int64(),
		Y:  types.FromInches(y).Int64(),
		CX: types.FromInches(width).Int64(),
		CY: types.FromInches(height).Int64(),
	}
}

// RectFromCM creates a RectEmu from centimeter values.
//
// This is a convenience function for specifying bounds in centimeters.
func RectFromCM(x, y, width, height float64) RectEmu {
	return RectEmu{
		X:  types.FromCM(x).Int64(),
		Y:  types.FromCM(y).Int64(),
		CX: types.FromCM(width).Int64(),
		CY: types.FromCM(height).Int64(),
	}
}

// RectFromPixels creates a RectEmu from pixel values at 96 DPI.
//
// This is a convenience function for specifying bounds in pixels.
func RectFromPixels(x, y, width, height int) RectEmu {
	return RectEmu{
		X:  types.FromPixels(x).Int64(),
		Y:  types.FromPixels(y).Int64(),
		CX: types.FromPixels(width).Int64(),
		CY: types.FromPixels(height).Int64(),
	}
}

// ToInches converts the rectangle to inch values.
func (r RectEmu) ToInches() (x, y, width, height float64) {
	return types.EMU(r.X).Inches(),
		types.EMU(r.Y).Inches(),
		types.EMU(r.CX).Inches(),
		types.EMU(r.CY).Inches()
}

// ToCM converts the rectangle to centimeter values.
func (r RectEmu) ToCM() (x, y, width, height float64) {
	return types.EMU(r.X).CM(),
		types.EMU(r.Y).CM(),
		types.EMU(r.CX).CM(),
		types.EMU(r.CY).CM()
}

// ToPixels converts the rectangle to pixel values at 96 DPI.
func (r RectEmu) ToPixels() (x, y, width, height int) {
	return types.EMU(r.X).Pixels(),
		types.EMU(r.Y).Pixels(),
		types.EMU(r.CX).Pixels(),
		types.EMU(r.CY).Pixels()
}

// IsZero returns true if the rectangle has zero area.
func (r RectEmu) IsZero() bool {
	return r.CX == 0 || r.CY == 0
}

// Contains returns true if point (px, py) is inside the rectangle.
func (r RectEmu) Contains(px, py int64) bool {
	return px >= r.X && px < r.X+r.CX && py >= r.Y && py < r.Y+r.CY
}

// Intersects returns true if this rectangle overlaps with another.
func (r RectEmu) Intersects(other RectEmu) bool {
	return r.X < other.X+other.CX &&
		r.X+r.CX > other.X &&
		r.Y < other.Y+other.CY &&
		r.Y+r.CY > other.Y
}

// InsertSVG inserts an SVG image with PNG fallback into a slide.
//
// This is the primary method for adding vector graphics to a PPTX document.
// The SVG is embedded using the Office 2016+ asvg:svgBlip extension, with
// the PNG as a fallback for older viewers.
//
// The method performs the following operations:
//  1. Validates the input options
//  2. Locates the target slide by index
//  3. Adds the SVG and PNG as media files
//  4. Registers their content types
//  5. Creates relationships from the slide to the media
//  6. Generates a p:pic element with proper positioning
//  7. Inserts the p:pic into the slide's shape tree
//
// Returns an error if:
//   - SVGData or PNGData is empty
//   - SlideIndex is out of range
//   - The slide structure is malformed
//
// Example:
//
//	err := doc.InsertSVG(InsertOptions{
//	    SlideIndex: 0,
//	    Bounds:     RectFromInches(1, 1, 5, 3),
//	    SVGData:    svgBytes,
//	    PNGData:    pngBytes,
//	    Name:       "Chart",
//	    AltText:    "Quarterly sales chart",
//	})
func (d *Document) InsertSVG(opts InsertOptions) error {
	// Validate required fields
	if len(opts.SVGData) == 0 {
		return fmt.Errorf("InsertSVG: SVGData is required")
	}
	if len(opts.PNGData) == 0 {
		return fmt.Errorf("InsertSVG: PNGData (fallback) is required")
	}
	if opts.Bounds.IsZero() {
		return fmt.Errorf("InsertSVG: Bounds must have non-zero area")
	}

	// 1. Enumerate slides and find the target
	slideEnum, err := NewSlideEnumerator(d.pkg)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to enumerate slides: %w", err)
	}

	slideInfo := slideEnum.ByIndex(opts.SlideIndex)
	if slideInfo == nil {
		return fmt.Errorf("InsertSVG: slide index %d out of range (have %d slides)",
			opts.SlideIndex, slideEnum.Count())
	}

	// 2. Create media inserter for adding images
	mediaInserter, err := NewMediaInserter(d.pkg)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to create media inserter: %w", err)
	}

	// 3. Insert both media files with unique source IDs
	sourceID := fmt.Sprintf("svg-insert-%d", opts.SlideIndex)
	svgPath, pngPath := mediaInserter.InsertSVGWithFallback(opts.SVGData, opts.PNGData, sourceID)

	// 4. Finalize content types (must be done before saving)
	if err := mediaInserter.Finalize(); err != nil {
		return fmt.Errorf("InsertSVG: failed to finalize content types: %w", err)
	}

	// 5. Load or create slide relationships
	slideRelsPath := GetRelsPath(slideInfo.PartPath)
	slideRels, err := d.loadOrCreateRelationships(slideRelsPath)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to load slide relationships: %w", err)
	}

	// 6. Add relationships for both media files
	// Targets are relative paths from slide to media
	pngRelTarget := relativePathFromSlide(pngPath)
	svgRelTarget := relativePathFromSlide(svgPath)

	pngRelID := slideRels.Add(RelTypeImage, pngRelTarget)
	svgRelID := slideRels.Add(RelTypeImage, svgRelTarget)

	// 7. Save updated relationships
	relsData, err := slideRels.Marshal()
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to marshal slide relationships: %w", err)
	}
	d.pkg.SetEntry(slideRelsPath, relsData)

	// 8. Read slide XML and allocate shape ID
	slideXML, err := d.pkg.ReadEntry(slideInfo.PartPath)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to read slide: %w", err)
	}

	shapeAlloc := NewShapeIDAllocator(slideXML)
	shapeID := shapeAlloc.Alloc()

	// 9. Generate p:pic element
	picOpts := PicOptions{
		ID:          shapeID,
		Name:        opts.Name,
		Description: opts.AltText,
		PNGRelID:    pngRelID,
		SVGRelID:    svgRelID,
		OffsetX:     opts.Bounds.X,
		OffsetY:     opts.Bounds.Y,
		ExtentCX:    opts.Bounds.CX,
		ExtentCY:    opts.Bounds.CY,
	}

	picXML, err := GeneratePic(picOpts)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to generate p:pic: %w", err)
	}

	// 10. Determine insertion position
	position := InsertAtEnd // Default: on top
	if opts.ZOrder != nil {
		position = InsertPosition(*opts.ZOrder)
	}

	// 11. Insert into spTree
	newSlideXML, err := InsertIntoSpTree(slideXML, picXML, position)
	if err != nil {
		return fmt.Errorf("InsertSVG: failed to insert into spTree: %w", err)
	}

	// 12. Save modified slide
	d.pkg.SetEntry(slideInfo.PartPath, newSlideXML)

	return nil
}

// loadOrCreateRelationships loads relationships from a path, or creates empty ones if not found.
func (d *Document) loadOrCreateRelationships(relsPath string) (*Relationships, error) {
	if !d.pkg.HasEntry(relsPath) {
		return NewRelationships(), nil
	}

	data, err := d.pkg.ReadEntry(relsPath)
	if err != nil {
		return nil, err
	}

	return ParseRelationships(data)
}

// relativePathFromSlide converts a media path to a relative path from slide.
// For example: "ppt/media/image1.png" -> "../media/image1.png"
func relativePathFromSlide(mediaPath string) string {
	// Slides are in ppt/slides/, media is in ppt/media/
	// So relative path is ../media/filename
	filename := path.Base(mediaPath)
	return "../media/" + filename
}

// InsertShapeOptions configures the insertion of a shape into a slide.
type InsertShapeOptions struct {
	// SlideIndex is the zero-based index of the target slide.
	SlideIndex int

	// Shape defines the shape to insert.
	Shape ShapeOptions

	// ZOrder controls the stacking position of the shape.
	// If nil, the shape is added at the default position (on top).
	ZOrder *int
}

// InsertShape inserts a preset geometry shape into a slide.
func (d *Document) InsertShape(opts InsertShapeOptions) error {
	if opts.Shape.Geometry == "" {
		return fmt.Errorf("InsertShape: Shape.Geometry is required")
	}
	if opts.Shape.Bounds.IsZero() {
		return fmt.Errorf("InsertShape: Shape.Bounds must have non-zero area")
	}

	// 1. Find the target slide
	slideEnum, err := NewSlideEnumerator(d.pkg)
	if err != nil {
		return fmt.Errorf("InsertShape: failed to enumerate slides: %w", err)
	}

	slideInfo := slideEnum.ByIndex(opts.SlideIndex)
	if slideInfo == nil {
		return fmt.Errorf("InsertShape: slide index %d out of range (have %d slides)",
			opts.SlideIndex, slideEnum.Count())
	}

	// 2. Read slide XML and allocate shape ID
	slideXML, err := d.pkg.ReadEntry(slideInfo.PartPath)
	if err != nil {
		return fmt.Errorf("InsertShape: failed to read slide: %w", err)
	}

	shapeAlloc := NewShapeIDAllocator(slideXML)
	opts.Shape.ID = shapeAlloc.Alloc()

	// 3. Generate p:sp element
	spXML, err := GenerateShape(opts.Shape)
	if err != nil {
		return fmt.Errorf("InsertShape: failed to generate p:sp: %w", err)
	}

	// 4. Determine insertion position
	position := InsertAtEnd
	if opts.ZOrder != nil {
		position = InsertPosition(*opts.ZOrder)
	}

	// 5. Insert into spTree
	newSlideXML, err := InsertIntoSpTree(slideXML, spXML, position)
	if err != nil {
		return fmt.Errorf("InsertShape: failed to insert into spTree: %w", err)
	}

	// 6. Save modified slide
	d.pkg.SetEntry(slideInfo.PartPath, newSlideXML)

	return nil
}

// InsertGroupOptions configures the insertion of a group shape into a slide.
type InsertGroupOptions struct {
	// SlideIndex is the zero-based index of the target slide.
	SlideIndex int

	// Group defines the group to insert.
	Group GroupOptions

	// ChildBounds defines the child coordinate space. If zero, defaults to
	// identity transform (same as Group.Bounds).
	ChildBounds RectEmu

	// ZOrder controls the stacking position of the group.
	// If nil, the group is added at the default position (on top).
	ZOrder *int
}

// InsertGroup inserts a group shape into a slide.
func (d *Document) InsertGroup(opts InsertGroupOptions) error {
	if opts.Group.Bounds.IsZero() {
		return fmt.Errorf("InsertGroup: Group.Bounds must have non-zero area")
	}

	// 1. Find the target slide
	slideEnum, err := NewSlideEnumerator(d.pkg)
	if err != nil {
		return fmt.Errorf("InsertGroup: failed to enumerate slides: %w", err)
	}

	slideInfo := slideEnum.ByIndex(opts.SlideIndex)
	if slideInfo == nil {
		return fmt.Errorf("InsertGroup: slide index %d out of range (have %d slides)",
			opts.SlideIndex, slideEnum.Count())
	}

	// 2. Read slide XML and allocate shape ID
	slideXML, err := d.pkg.ReadEntry(slideInfo.PartPath)
	if err != nil {
		return fmt.Errorf("InsertGroup: failed to read slide: %w", err)
	}

	shapeAlloc := NewShapeIDAllocator(slideXML)
	opts.Group.ID = shapeAlloc.Alloc()

	// 3. Generate p:grpSp element
	var grpXML []byte
	if opts.ChildBounds.IsZero() {
		grpXML, err = GenerateGroup(opts.Group)
	} else {
		grpXML, err = GenerateGroupWithChildSpace(opts.Group, opts.ChildBounds)
	}
	if err != nil {
		return fmt.Errorf("InsertGroup: failed to generate p:grpSp: %w", err)
	}

	// 4. Determine insertion position
	position := InsertAtEnd
	if opts.ZOrder != nil {
		position = InsertPosition(*opts.ZOrder)
	}

	// 5. Insert into spTree
	newSlideXML, err := InsertIntoSpTree(slideXML, grpXML, position)
	if err != nil {
		return fmt.Errorf("InsertGroup: failed to insert into spTree: %w", err)
	}

	// 6. Save modified slide
	d.pkg.SetEntry(slideInfo.PartPath, newSlideXML)

	return nil
}

// SlideCount returns the number of slides in the document.
func (d *Document) SlideCount() (int, error) {
	enum, err := NewSlideEnumerator(d.pkg)
	if err != nil {
		return 0, err
	}
	return enum.Count(), nil
}

// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// MediaDir is the standard path for media files in a PPTX package.
const MediaDir = "ppt/media/"

// mediaNamePattern matches media filenames like "image1.png", "image2.svg"
var mediaNamePattern = regexp.MustCompile(`^image(\d+)\.(\w+)$`)

// MediaAllocator manages allocation of media file names.
// It tracks existing media files and allocates new names deterministically.
type MediaAllocator struct {
	maxImageNum int               // highest imageN number seen
	allocated   map[string]string // source identifier -> allocated path
	existing    map[string]bool   // set of existing media paths
}

// NewMediaAllocator creates a new media allocator.
// It should be initialized with existing media paths from the package.
func NewMediaAllocator() *MediaAllocator {
	return &MediaAllocator{
		maxImageNum: 0,
		allocated:   make(map[string]string),
		existing:    make(map[string]bool),
	}
}

// ScanPackage scans a package to find existing media files
// and initialize the allocator state.
func (m *MediaAllocator) ScanPackage(pkg *Package) {
	for _, entry := range pkg.Entries() {
		if strings.HasPrefix(entry, MediaDir) {
			m.existing[entry] = true
			m.updateMaxFromPath(entry)
		}
	}
}

// ScanPaths scans a list of paths to find existing media files.
// This is useful when working with paths directly without a Package.
func (m *MediaAllocator) ScanPaths(paths []string) {
	for _, p := range paths {
		if strings.HasPrefix(p, MediaDir) {
			m.existing[p] = true
			m.updateMaxFromPath(p)
		}
	}
}

// updateMaxFromPath extracts the image number from a path and updates maxImageNum.
func (m *MediaAllocator) updateMaxFromPath(mediaPath string) {
	// Extract filename
	filename := path.Base(mediaPath)
	matches := mediaNamePattern.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return
	}

	if num > m.maxImageNum {
		m.maxImageNum = num
	}
}

// Allocate allocates a new media path for the given extension.
// The sourceID is used for deduplication - if the same sourceID is allocated twice,
// the same path is returned.
//
// Extension should be lowercase without a leading dot (e.g., "png", "svg").
func (m *MediaAllocator) Allocate(extension string, sourceID string) string {
	// Check if already allocated
	if existing, ok := m.allocated[sourceID]; ok {
		return existing
	}

	// Normalize extension
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))

	// Allocate next number
	m.maxImageNum++
	filename := fmt.Sprintf("image%d.%s", m.maxImageNum, ext)
	fullPath := MediaDir + filename

	// Track allocation
	m.allocated[sourceID] = fullPath
	m.existing[fullPath] = true

	return fullPath
}

// AllocatePNG allocates a new PNG media path.
func (m *MediaAllocator) AllocatePNG(sourceID string) string {
	return m.Allocate("png", sourceID)
}

// AllocateSVG allocates a new SVG media path.
func (m *MediaAllocator) AllocateSVG(sourceID string) string {
	return m.Allocate("svg", sourceID)
}

// AllocatePair allocates both SVG and PNG paths for fallback embedding.
// Returns (svgPath, pngPath).
// The sourceID should uniquely identify the content pair.
func (m *MediaAllocator) AllocatePair(sourceID string) (svgPath string, pngPath string) {
	svgPath = m.Allocate("svg", sourceID+":svg")
	pngPath = m.Allocate("png", sourceID+":png")
	return svgPath, pngPath
}

// Allocated returns the allocated path for a sourceID, or empty string if not found.
func (m *MediaAllocator) Allocated(sourceID string) string {
	return m.allocated[sourceID]
}

// HasExisting checks if a media path already exists.
func (m *MediaAllocator) HasExisting(mediaPath string) bool {
	return m.existing[mediaPath]
}

// ExistingPaths returns all existing media paths (sorted).
func (m *MediaAllocator) ExistingPaths() []string {
	paths := make([]string, 0, len(m.existing))
	for p := range m.existing {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}

// AllocatedPaths returns all paths that have been allocated in this session (sorted).
func (m *MediaAllocator) AllocatedPaths() []string {
	pathSet := make(map[string]bool)
	for _, p := range m.allocated {
		pathSet[p] = true
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}

// NextImageNum returns the next image number that would be allocated.
// Useful for preview/planning purposes.
func (m *MediaAllocator) NextImageNum() int {
	return m.maxImageNum + 1
}

// Reset clears all allocations but keeps existing paths.
// Useful for restarting allocation from the current state.
func (m *MediaAllocator) Reset() {
	m.allocated = make(map[string]string)
}

// Clone creates an independent copy of the allocator.
func (m *MediaAllocator) Clone() *MediaAllocator {
	clone := &MediaAllocator{
		maxImageNum: m.maxImageNum,
		allocated:   make(map[string]string),
		existing:    make(map[string]bool),
	}

	for k, v := range m.allocated {
		clone.allocated[k] = v
	}
	for k := range m.existing {
		clone.existing[k] = true
	}

	return clone
}

// MediaPath constructs a media path from a filename.
func MediaPath(filename string) string {
	return MediaDir + filename
}

// MediaFilename extracts just the filename from a media path.
func MediaFilename(mediaPath string) string {
	return path.Base(mediaPath)
}

// IsMediaPath checks if a path is in the media directory.
func IsMediaPath(p string) bool {
	return strings.HasPrefix(p, MediaDir)
}

// RelativeMediaPath returns the relative path from slide to media.
// This is used in relationship targets.
// For example: "../media/image1.png"
func RelativeMediaPath(filename string) string {
	return "../media/" + filename
}

// MediaInserter provides helpers for inserting media into a PPTX package.
// It manages both the media files and their content type registrations.
type MediaInserter struct {
	pkg          *Package
	allocator    *MediaAllocator
	contentTypes *ContentTypes
}

// NewMediaInserter creates a new media inserter for a package.
// It scans the package for existing media files and parses [Content_Types].xml.
func NewMediaInserter(pkg *Package) (*MediaInserter, error) {
	// Parse content types
	ctData, err := pkg.ReadEntry(ContentTypesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read [Content_Types].xml: %w", err)
	}

	ct, err := ParseContentTypes(ctData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml: %w", err)
	}

	// Create and initialize allocator
	allocator := NewMediaAllocator()
	allocator.ScanPackage(pkg)

	return &MediaInserter{
		pkg:          pkg,
		allocator:    allocator,
		contentTypes: ct,
	}, nil
}

// InsertPNG inserts a PNG image into the package.
// Returns the allocated media path (e.g., "ppt/media/image5.png").
// The sourceID is used for deduplication - same sourceID returns same path.
func (mi *MediaInserter) InsertPNG(data []byte, sourceID string) string {
	// Allocate path
	mediaPath := mi.allocator.AllocatePNG(sourceID)

	// Add to package
	mi.pkg.SetEntry(mediaPath, data)

	// Ensure content type is registered
	mi.contentTypes.EnsurePNG()

	return mediaPath
}

// InsertSVG inserts an SVG image into the package.
// Returns the allocated media path (e.g., "ppt/media/image5.svg").
// The sourceID is used for deduplication - same sourceID returns same path.
func (mi *MediaInserter) InsertSVG(data []byte, sourceID string) string {
	// Allocate path
	mediaPath := mi.allocator.AllocateSVG(sourceID)

	// Add to package
	mi.pkg.SetEntry(mediaPath, data)

	// Ensure content type is registered
	mi.contentTypes.EnsureSVG()

	return mediaPath
}

// InsertSVGWithFallback inserts both an SVG and its PNG fallback.
// Returns (svgPath, pngPath) - both media paths.
// The sourceID should uniquely identify the content pair.
func (mi *MediaInserter) InsertSVGWithFallback(svgData, pngData []byte, sourceID string) (svgPath, pngPath string) {
	svgPath = mi.InsertSVG(svgData, sourceID+":svg")
	pngPath = mi.InsertPNG(pngData, sourceID+":png")
	return svgPath, pngPath
}

// Finalize writes the updated [Content_Types].xml back to the package.
// This must be called before saving the package if any media was inserted.
func (mi *MediaInserter) Finalize() error {
	ctData, err := mi.contentTypes.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal [Content_Types].xml: %w", err)
	}

	mi.pkg.SetEntry(ContentTypesPath, ctData)
	return nil
}

// Allocator returns the underlying MediaAllocator.
// Useful for querying allocated paths without insertion.
func (mi *MediaInserter) Allocator() *MediaAllocator {
	return mi.allocator
}

// ContentTypes returns the underlying ContentTypes manager.
// Useful for additional content type registrations.
func (mi *MediaInserter) ContentTypes() *ContentTypes {
	return mi.contentTypes
}

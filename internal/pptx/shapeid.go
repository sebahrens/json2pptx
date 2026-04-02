// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"regexp"
	"strconv"
)

// cNvPrIDPattern matches cNvPr id="N" attributes in slide XML.
// This works for both p:cNvPr and other namespaced cNvPr elements.
var cNvPrIDPattern = regexp.MustCompile(`cNvPr\s+id="(\d+)"`)

// ShapeIDAllocator manages allocation of unique shape IDs within a slide.
// In PPTX, each shape/picture/etc. has a cNvPr element with a unique id attribute.
// The id must be unique within the slide, and new shapes should use max(existing)+1.
type ShapeIDAllocator struct {
	maxID uint32
}

// NewShapeIDAllocator creates a new allocator by scanning slide XML for existing IDs.
// Pass the raw slide XML content to extract all cNvPr/@id values.
func NewShapeIDAllocator(slideXML []byte) *ShapeIDAllocator {
	alloc := &ShapeIDAllocator{maxID: 0}
	alloc.ScanXML(slideXML)
	return alloc
}

// ScanXML scans XML content for cNvPr id attributes and updates maxID.
// This can be called multiple times if needed (e.g., after loading additional parts).
func (a *ShapeIDAllocator) ScanXML(xmlData []byte) {
	matches := cNvPrIDPattern.FindAllSubmatch(xmlData, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			if id, err := strconv.ParseUint(string(match[1]), 10, 32); err == nil {
				if uint32(id) > a.maxID {
					a.maxID = uint32(id)
				}
			}
		}
	}
}

// Alloc allocates and returns the next available shape ID.
// The returned ID is guaranteed to be greater than any previously seen ID.
func (a *ShapeIDAllocator) Alloc() uint32 {
	a.maxID++
	return a.maxID
}

// AllocN allocates N consecutive shape IDs.
// Returns the starting ID. IDs will be start, start+1, ..., start+n-1.
func (a *ShapeIDAllocator) AllocN(n int) uint32 {
	if n <= 0 {
		return a.maxID + 1
	}
	start := a.maxID + 1
	a.maxID += uint32(n)
	return start
}

// NextID returns what the next allocated ID would be without allocating it.
// Useful for preview/planning purposes.
func (a *ShapeIDAllocator) NextID() uint32 {
	return a.maxID + 1
}

// MaxID returns the highest ID seen so far.
func (a *ShapeIDAllocator) MaxID() uint32 {
	return a.maxID
}

// SetMinID ensures the allocator will return at least minID on the next allocation.
// This is useful when you need to reserve a range of IDs.
func (a *ShapeIDAllocator) SetMinID(minID uint32) {
	if minID > a.maxID+1 {
		a.maxID = minID - 1
	}
}

// Clone creates an independent copy of the allocator.
func (a *ShapeIDAllocator) Clone() *ShapeIDAllocator {
	return &ShapeIDAllocator{maxID: a.maxID}
}

// NewShapeIDAllocatorForSlide creates an allocator for a specific slide in a package.
// It reads the slide XML and scans it for existing shape IDs.
func NewShapeIDAllocatorForSlide(pkg *Package, slidePath string) (*ShapeIDAllocator, error) {
	slideXML, err := pkg.ReadEntry(slidePath)
	if err != nil {
		return nil, err
	}
	return NewShapeIDAllocator(slideXML), nil
}

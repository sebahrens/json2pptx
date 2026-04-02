// Package pptx provides PPTX file manipulation primitives.
//
// This file implements the OPC (Open Packaging Convention) layer for PPTX files.
// PPTX files are ZIP archives with specific structure and relationships.
package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/ahrens/go-slide-creator/internal/utils"
)

// Package represents an open PPTX package (OPC archive).
// It provides read access to parts and supports modifications.
type Package struct {
	reader  *zip.Reader
	entries map[string]*zip.File // path -> entry for O(1) lookup

	// Modifications tracked for write
	modified map[string][]byte // path -> new content
	added    map[string][]byte // path -> content (new files)
	deleted  map[string]bool   // paths to exclude from output
}

// Open opens a PPTX package from an io.ReaderAt with known size.
// This allows reading from any source (file, memory, network) without
// requiring a file path.
func Open(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP: %w", err)
	}

	pkg := &Package{
		reader:   zr,
		entries:  make(map[string]*zip.File),
		modified: make(map[string][]byte),
		added:    make(map[string][]byte),
		deleted:  make(map[string]bool),
	}

	for _, f := range zr.File {
		pkg.entries[f.Name] = f
	}

	return pkg, nil
}

// OpenFile opens a PPTX package from a file path.
// Convenience wrapper around Open for file-based access.
func OpenFile(path string) (*Package, io.Closer, error) {
	f, err := newFileReaderAt(path)
	if err != nil {
		return nil, nil, err
	}

	pkg, err := Open(f, f.size)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	return pkg, f, nil
}

// fileReaderAt wraps os.File to implement io.ReaderAt with size.
type fileReaderAt struct {
	file readerAtFile
	size int64
}

// readerAtFile is an interface matching os.File methods we need.
type readerAtFile interface {
	io.ReaderAt
	io.Closer
	Stat() (fileInfo, error)
}

// fileInfo matches os.FileInfo.Size() method.
type fileInfo interface {
	Size() int64
}

// newFileReaderAt opens a file and returns a fileReaderAt.
func newFileReaderAt(path string) (*fileReaderAt, error) {
	return newFileReaderAtWithOpener(path, defaultFileOpener)
}

// fileOpener is a function type for opening files (allows testing).
type fileOpener func(path string) (readerAtFile, error)

// defaultFileOpener uses os.Open.
var defaultFileOpener fileOpener = func(path string) (readerAtFile, error) {
	return openOSFile(path)
}

// newFileReaderAtWithOpener opens a file using the provided opener.
func newFileReaderAtWithOpener(path string, opener fileOpener) (*fileReaderAt, error) {
	f, err := opener(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &fileReaderAt{file: f, size: info.Size()}, nil
}

func (f *fileReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return f.file.ReadAt(p, off)
}

func (f *fileReaderAt) Close() error {
	return f.file.Close()
}

// Entries returns all part paths in the package.
func (p *Package) Entries() []string {
	paths := make([]string, 0, len(p.entries))
	for path := range p.entries {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// HasEntry checks if a part exists in the package.
func (p *Package) HasEntry(path string) bool {
	_, ok := p.entries[path]
	return ok
}

// ReadEntry reads the content of a part by path.
// Returns the modified content if the part has been modified.
func (p *Package) ReadEntry(path string) ([]byte, error) {
	// Check for modified content first
	if content, ok := p.modified[path]; ok {
		return content, nil
	}

	// Check for added content
	if content, ok := p.added[path]; ok {
		return content, nil
	}

	// Read from original ZIP
	entry, ok := p.entries[path]
	if !ok {
		return nil, fmt.Errorf("part not found: %s", path)
	}

	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open part %s: %w", path, err)
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(io.LimitReader(rc, utils.MaxZipEntrySize))
}

// SetEntry sets or replaces content for a part.
// For existing parts, this marks them as modified.
// For new parts, this adds them to the package.
func (p *Package) SetEntry(path string, content []byte) {
	if _, exists := p.entries[path]; exists {
		p.modified[path] = content
		delete(p.deleted, path)
	} else {
		p.added[path] = content
		delete(p.deleted, path)
	}
}

// DeleteEntry marks a part for deletion.
// The part will not be included in the output when Save is called.
func (p *Package) DeleteEntry(path string) {
	p.deleted[path] = true
	delete(p.modified, path)
	delete(p.added, path)
}

// Save writes the package to the given writer.
// Output is deterministic: entries are sorted alphabetically and
// all timestamps are set to a fixed epoch for reproducibility.
func (p *Package) Save(w io.Writer) error {
	return p.SaveWithTimestamp(w, utils.DeterministicTimestamp)
}

// SaveWithTimestamp writes the package with a custom timestamp.
// Use Save() for deterministic output with fixed timestamp.
func (p *Package) SaveWithTimestamp(w io.Writer, modTime time.Time) error {
	zw := zip.NewWriter(w)

	// Collect all output paths
	outputPaths := make(map[string]bool)

	// Original entries (excluding deleted)
	for path := range p.entries {
		if !p.deleted[path] {
			outputPaths[path] = true
		}
	}

	// Added entries
	for path := range p.added {
		if !p.deleted[path] {
			outputPaths[path] = true
		}
	}

	// Sort paths for deterministic output
	sortedPaths := make([]string, 0, len(outputPaths))
	for path := range outputPaths {
		sortedPaths = append(sortedPaths, path)
	}
	slices.Sort(sortedPaths)

	// Write entries in sorted order
	for _, path := range sortedPaths {
		var content []byte
		var err error

		// Get content (priority: modified > added > original)
		if modified, ok := p.modified[path]; ok {
			content = modified
		} else if added, ok := p.added[path]; ok {
			content = added
		} else {
			content, err = p.readOriginalEntry(path)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}
		}

		// Create entry with fixed timestamp for determinism
		header := &zip.FileHeader{
			Name:     path,
			Method:   zip.Deflate,
			Modified: modTime,
		}

		fw, err := zw.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create entry %s: %w", path, err)
		}

		if _, err := fw.Write(content); err != nil {
			return fmt.Errorf("failed to write entry %s: %w", path, err)
		}
	}

	return zw.Close()
}

// readOriginalEntry reads content from the original ZIP archive.
func (p *Package) readOriginalEntry(path string) ([]byte, error) {
	entry, ok := p.entries[path]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", path)
	}

	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(io.LimitReader(rc, utils.MaxZipEntrySize))
}

// Clone creates an independent copy of the package state.
// The clone shares the underlying reader but has separate modification state.
func (p *Package) Clone() *Package {
	clone := &Package{
		reader:   p.reader,
		entries:  p.entries, // shared, read-only
		modified: make(map[string][]byte),
		added:    make(map[string][]byte),
		deleted:  make(map[string]bool),
	}

	for k, v := range p.modified {
		clone.modified[k] = v
	}
	for k, v := range p.added {
		clone.added[k] = v
	}
	for k := range p.deleted {
		clone.deleted[k] = true
	}

	return clone
}

// OpenFromBytes opens a PPTX package from a byte slice.
// Convenience wrapper for in-memory operations.
func OpenFromBytes(data []byte) (*Package, error) {
	r := bytes.NewReader(data)
	return Open(r, int64(len(data)))
}

// SaveToBytes saves the package to a byte slice.
// Convenience wrapper for in-memory operations.
func (p *Package) SaveToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.Save(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

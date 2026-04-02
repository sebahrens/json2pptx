package template

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// Reader provides access to PPTX template files.
type Reader struct {
	path   string
	hash   string
	zip    *zip.ReadCloser
	closed bool
}

// OpenTemplate opens a PPTX template file and validates its structure.
// Returns an error if the file doesn't exist, is not a valid ZIP, or lacks required PPTX structure.
func OpenTemplate(path string) (*Reader, error) {
	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot access template file: %w", err)
	}

	// Check it's a regular file
	if info.IsDir() {
		return nil, fmt.Errorf("template path is a directory, not a file: %s", path)
	}

	// Calculate file hash for cache invalidation
	hash, err := calculateFileHash(path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate template hash: %w", err)
	}

	// Open as ZIP archive
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("invalid PPTX format (not a ZIP archive): %w", err)
	}

	reader := &Reader{
		path: path,
		hash: hash,
		zip:  zipReader,
	}

	// Validate PPTX structure
	if err := reader.validateStructure(); err != nil {
		_ = zipReader.Close()
		return nil, err
	}

	return reader, nil
}

// validateStructure checks that the ZIP contains required PPTX files.
func (r *Reader) validateStructure() error {
	// Check for presentation.xml - the core PPTX file
	if !r.hasFile("ppt/presentation.xml") {
		return fmt.Errorf("corrupted template: missing ppt/presentation.xml")
	}

	// Check for at least one slide layout
	hasLayout := false
	for _, f := range r.zip.File {
		if matched, _ := filepath.Match("ppt/slideLayouts/slideLayout*.xml", f.Name); matched {
			hasLayout = true
			break
		}
	}
	if !hasLayout {
		return fmt.Errorf("templates must have layouts: no slideLayout files found")
	}

	return nil
}

// hasFile checks if a file exists in the ZIP archive.
func (r *Reader) hasFile(name string) bool {
	for _, f := range r.zip.File {
		if f.Name == name {
			return true
		}
	}
	return false
}

// ReadFile reads a file from the ZIP archive.
func (r *Reader) ReadFile(name string) ([]byte, error) {
	for _, f := range r.zip.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open %s: %w", name, err)
			}
			defer func() { _ = rc.Close() }()

			data, err := io.ReadAll(io.LimitReader(rc, utils.MaxZipEntrySize))
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("file not found in template: %s", name)
}

// ListFiles returns all file names in the ZIP archive matching the pattern.
// Pattern uses filepath.Match syntax (e.g., "ppt/slideLayouts/*.xml").
func (r *Reader) ListFiles(pattern string) ([]string, error) {
	var matches []string
	for _, f := range r.zip.File {
		matched, err := filepath.Match(pattern, f.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if matched {
			matches = append(matches, f.Name)
		}
	}
	return matches, nil
}

// Path returns the template file path.
func (r *Reader) Path() string {
	return r.path
}

// Hash returns the SHA256 hash of the template file.
func (r *Reader) Hash() string {
	return r.hash
}

// Close closes the ZIP reader and releases resources.
// Multiple calls to Close are safe and will not return an error.
func (r *Reader) Close() error {
	if r.closed || r.zip == nil {
		return nil
	}
	r.closed = true
	return r.zip.Close()
}

// calculateFileHash computes the SHA256 hash of a file.
func calculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

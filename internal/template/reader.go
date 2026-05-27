package template

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// Reader provides access to PPTX template files.
type Reader struct {
	path      string
	hash      string
	zip       *zip.ReadCloser
	closed    bool
	tblStyles *tableStyleIndex // lazy; created on first ResolveTableStyleID call
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
		name, ok := safeZipName(f.Name)
		if !ok {
			continue
		}
		if matched, _ := filepath.Match("ppt/slideLayouts/slideLayout*.xml", name); matched {
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
	safe, ok := safeZipName(name)
	if !ok {
		return false
	}
	for _, f := range r.zip.File {
		entryName, entryOK := safeZipName(f.Name)
		if !entryOK {
			continue
		}
		if entryName == safe {
			return true
		}
	}
	return false
}

// ReadFile reads a file from the ZIP archive.
func (r *Reader) ReadFile(name string) ([]byte, error) {
	safe, ok := safeZipName(name)
	if !ok {
		return nil, fmt.Errorf("unsafe file name: %q", name)
	}
	for _, f := range r.zip.File {
		entryName, entryOK := safeZipName(f.Name)
		if !entryOK {
			continue
		}
		if entryName == safe {
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

// safeZipName returns the entry name only if it is safe (no absolute path or
// ".." traversal). ZIP entries always use forward slashes per the PKZIP spec.
func safeZipName(name string) (string, bool) {
	clean := path.Clean(name)
	if path.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", false
	}
	return clean, true
}

// ListFiles returns all file names in the ZIP archive matching the pattern.
// Pattern uses filepath.Match syntax (e.g., "ppt/slideLayouts/*.xml").
// Entries whose names contain path-traversal sequences are silently skipped.
func (r *Reader) ListFiles(pattern string) ([]string, error) {
	var matches []string
	for _, f := range r.zip.File {
		name, ok := safeZipName(f.Name)
		if !ok {
			continue
		}
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if matched {
			matches = append(matches, name)
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

// TableStyles returns all table style entries declared in the template's
// ppt/tableStyles.xml.  Returns an empty (non-nil) slice when the template
// has no styles or the XML is missing/malformed.
func (r *Reader) TableStyles() []TableStyleEntry {
	if r.tblStyles == nil {
		r.tblStyles = newTableStyleIndex(r)
	}
	return r.tblStyles.all()
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

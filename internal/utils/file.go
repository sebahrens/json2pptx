// Package utils provides shared utility functions used across the application.
package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"time"
)

// MaxZipEntrySize is the maximum decompressed size allowed for a single ZIP entry.
// 100 MB is generous for any PPTX XML part (typical slide XML is <50 KB).
const MaxZipEntrySize = 100 << 20 // 100 MB

// DeterministicTimestamp is the fixed modification time used for all ZIP entries
// to ensure byte-identical output across runs. Matches internal/pptx convention.
var DeterministicTimestamp = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// ZipCreateDeterministic creates a new entry in the ZIP writer with a fixed timestamp.
// This replaces zip.Writer.Create() which uses time.Now() and breaks determinism.
func ZipCreateDeterministic(w *zip.Writer, name string) (io.Writer, error) {
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: DeterministicTimestamp,
	}
	return w.CreateHeader(header)
}

// readLimited reads up to MaxZipEntrySize bytes from r and returns an error if the
// entry exceeds the limit. This prevents zip bomb attacks.
func readLimited(r io.Reader, name string) ([]byte, error) {
	lr := io.LimitReader(r, MaxZipEntrySize+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxZipEntrySize {
		return nil, fmt.Errorf("ZIP entry %s exceeds %d byte limit", name, MaxZipEntrySize)
	}
	return data, nil
}

// CopyZipFile copies a file from one ZIP to another.
// It preserves the file name and uses a deterministic timestamp.
func CopyZipFile(w *zip.Writer, f *zip.File) error {
	fw, err := ZipCreateDeterministic(w, f.Name)
	if err != nil {
		return err
	}

	fr, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = fr.Close() }()

	_, err = io.Copy(fw, io.LimitReader(fr, MaxZipEntrySize))
	return err
}

// ZipIndex provides O(1) filename lookups over a zip.Reader.
// Build once with BuildZipIndex, then use ReadFileFromZipIndex for repeated lookups.
type ZipIndex map[string]*zip.File

// BuildZipIndex builds a filename-to-File map for O(1) lookups.
func BuildZipIndex(r *zip.Reader) ZipIndex {
	idx := make(ZipIndex, len(r.File))
	for _, f := range r.File {
		idx[f.Name] = f
	}
	return idx
}

// ReadFileFromZipIndex reads a file by name using a pre-built index.
func ReadFileFromZipIndex(idx ZipIndex, name string) ([]byte, error) {
	f, ok := idx[name]
	if !ok {
		return nil, fmt.Errorf("file not found in ZIP: %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return readLimited(rc, name)
}

// ReadFileFromZip reads a file by name from a ZIP reader.
// Returns the file contents as a byte slice, or an error if the file is not found.
// For repeated lookups on the same zip.Reader, prefer BuildZipIndex + ReadFileFromZipIndex.
func ReadFileFromZip(r *zip.Reader, name string) ([]byte, error) {
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return readLimited(rc, name)
		}
	}
	return nil, fmt.Errorf("file not found in ZIP: %s", name)
}

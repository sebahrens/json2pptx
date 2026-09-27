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

// MaxZipTotalSize caps the summed declared uncompressed size of every entry
// in a user-supplied PPTX/template archive (zip-bomb guard).
const MaxZipTotalSize = 256 << 20 // 256 MB

// maxZipEntryRatio is the largest uncompressed:compressed ratio accepted for
// an entry larger than zipRatioMinSize. Real OOXML parts compress ~5-30x;
// DEFLATE tops out near 1030x, which is what a bomb uses.
const (
	maxZipEntryRatio = 200
	zipRatioMinSize  = 10 << 20 // 10 MB
)

// CheckZipLimits rejects a ZIP archive whose entries would decompress past
// MaxZipEntrySize each or MaxZipTotalSize in total, or whose large entries
// have a bomb-like compression ratio. archive/zip refuses to read past an
// entry's declared UncompressedSize64, so the declared sizes are binding.
func CheckZipLimits(files []*zip.File) error {
	var total uint64
	for _, f := range files {
		size := f.UncompressedSize64
		if size > MaxZipEntrySize {
			return fmt.Errorf("ZIP entry %s: uncompressed size %d exceeds %d byte limit", f.Name, size, MaxZipEntrySize)
		}
		if size > zipRatioMinSize && size > f.CompressedSize64*maxZipEntryRatio {
			return fmt.Errorf("ZIP entry %s: compression ratio exceeds %d:1 (possible zip bomb)", f.Name, maxZipEntryRatio)
		}
		total += size
		if total > MaxZipTotalSize {
			return fmt.Errorf("ZIP archive: total uncompressed size exceeds %d byte limit", MaxZipTotalSize)
		}
	}
	return nil
}

// ReadZipEntryLimited reads up to MaxZipEntrySize bytes from r and returns an
// error, rather than silently truncating, when the entry is larger.
func ReadZipEntryLimited(r io.Reader, name string) ([]byte, error) {
	return readLimited(r, name)
}

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

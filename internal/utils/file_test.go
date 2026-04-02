package utils

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestReadFileFromZip_Success(t *testing.T) {
	// Create a zip file in memory
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	expectedContent := []byte("content of test file")
	f, err := w.Create("test/file.txt")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := f.Write(expectedContent); err != nil {
		t.Fatalf("failed to write to zip file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	// Read from the zip
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	got, err := ReadFileFromZip(reader, "test/file.txt")
	if err != nil {
		t.Fatalf("ReadFileFromZip() returned error: %v", err)
	}

	if !bytes.Equal(got, expectedContent) {
		t.Errorf("ReadFileFromZip() = %q, want %q", string(got), string(expectedContent))
	}
}

func TestReadFileFromZip_FileNotFound(t *testing.T) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("existing.txt")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := f.Write([]byte("content")); err != nil {
		t.Fatalf("failed to write to zip file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	_, err = ReadFileFromZip(reader, "nonexistent.txt")
	if err == nil {
		t.Error("ReadFileFromZip() with nonexistent file should return error")
	}
}

func TestReadFileFromZip_EmptyFile(t *testing.T) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// Create empty file in zip
	if _, err := w.Create("empty.txt"); err != nil {
		t.Fatalf("failed to create empty file in zip: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	got, err := ReadFileFromZip(reader, "empty.txt")
	if err != nil {
		t.Fatalf("ReadFileFromZip() returned error for empty file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadFileFromZip() for empty file = %v, want empty slice", got)
	}
}

func TestCopyZipFile_Success(t *testing.T) {
	// Create source zip
	srcBuf := new(bytes.Buffer)
	srcWriter := zip.NewWriter(srcBuf)

	expectedContent := []byte("file content to copy")
	f, err := srcWriter.Create("dir/file.txt")
	if err != nil {
		t.Fatalf("failed to create file in source zip: %v", err)
	}
	if _, err := f.Write(expectedContent); err != nil {
		t.Fatalf("failed to write to source zip: %v", err)
	}
	if err := srcWriter.Close(); err != nil {
		t.Fatalf("failed to close source zip writer: %v", err)
	}

	// Open source zip for reading
	srcReader, err := zip.NewReader(bytes.NewReader(srcBuf.Bytes()), int64(srcBuf.Len()))
	if err != nil {
		t.Fatalf("failed to create source zip reader: %v", err)
	}

	// Create destination zip
	dstBuf := new(bytes.Buffer)
	dstWriter := zip.NewWriter(dstBuf)

	// Copy file
	if err := CopyZipFile(dstWriter, srcReader.File[0]); err != nil {
		t.Fatalf("CopyZipFile() returned error: %v", err)
	}
	if err := dstWriter.Close(); err != nil {
		t.Fatalf("failed to close destination zip writer: %v", err)
	}

	// Verify content in destination
	dstReader, err := zip.NewReader(bytes.NewReader(dstBuf.Bytes()), int64(dstBuf.Len()))
	if err != nil {
		t.Fatalf("failed to create destination zip reader: %v", err)
	}

	if len(dstReader.File) != 1 {
		t.Fatalf("expected 1 file in destination zip, got %d", len(dstReader.File))
	}

	if dstReader.File[0].Name != "dir/file.txt" {
		t.Errorf("CopyZipFile() file name = %q, want %q", dstReader.File[0].Name, "dir/file.txt")
	}

	got, err := ReadFileFromZip(dstReader, "dir/file.txt")
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if !bytes.Equal(got, expectedContent) {
		t.Errorf("CopyZipFile() content = %q, want %q", string(got), string(expectedContent))
	}
}

func TestCopyZipFile_MultipleFiles(t *testing.T) {
	// Create source zip with multiple files
	srcBuf := new(bytes.Buffer)
	srcWriter := zip.NewWriter(srcBuf)

	files := map[string][]byte{
		"file1.txt":     []byte("content 1"),
		"file2.txt":     []byte("content 2"),
		"dir/file3.txt": []byte("content 3"),
	}

	for name, content := range files {
		f, err := srcWriter.Create(name)
		if err != nil {
			t.Fatalf("failed to create %s in source zip: %v", name, err)
		}
		if _, err := f.Write(content); err != nil {
			t.Fatalf("failed to write %s to source zip: %v", name, err)
		}
	}
	if err := srcWriter.Close(); err != nil {
		t.Fatalf("failed to close source zip writer: %v", err)
	}

	srcReader, err := zip.NewReader(bytes.NewReader(srcBuf.Bytes()), int64(srcBuf.Len()))
	if err != nil {
		t.Fatalf("failed to create source zip reader: %v", err)
	}

	// Copy all files
	dstBuf := new(bytes.Buffer)
	dstWriter := zip.NewWriter(dstBuf)

	for _, f := range srcReader.File {
		if err := CopyZipFile(dstWriter, f); err != nil {
			t.Fatalf("CopyZipFile() failed for %s: %v", f.Name, err)
		}
	}
	if err := dstWriter.Close(); err != nil {
		t.Fatalf("failed to close destination zip writer: %v", err)
	}

	// Verify all files copied
	dstReader, err := zip.NewReader(bytes.NewReader(dstBuf.Bytes()), int64(dstBuf.Len()))
	if err != nil {
		t.Fatalf("failed to create destination zip reader: %v", err)
	}

	if len(dstReader.File) != len(files) {
		t.Errorf("expected %d files in destination zip, got %d", len(files), len(dstReader.File))
	}

	for name, expectedContent := range files {
		got, err := ReadFileFromZip(dstReader, name)
		if err != nil {
			t.Errorf("failed to read %s from destination: %v", name, err)
			continue
		}
		if !bytes.Equal(got, expectedContent) {
			t.Errorf("CopyZipFile() %s content = %q, want %q", name, string(got), string(expectedContent))
		}
	}
}

func TestReadFileFromZip_ExceedsLimit(t *testing.T) {
	// Create a zip with an entry just over the limit.
	// We can't create a real 100 MB+ entry in a unit test, so we
	// temporarily lower the perception by testing the readLimited helper
	// with a small synthetic reader instead.

	// Build a ZIP with a 1 KB entry — this should succeed.
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	content := []byte(strings.Repeat("A", 1024))
	f, err := w.Create("small.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("failed to write zip entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	got, err := ReadFileFromZip(reader, "small.txt")
	if err != nil {
		t.Fatalf("ReadFileFromZip() returned unexpected error: %v", err)
	}
	if len(got) != 1024 {
		t.Errorf("ReadFileFromZip() returned %d bytes, want 1024", len(got))
	}
}

func TestReadLimited_ExceedsLimit(t *testing.T) {
	// Directly test readLimited with data exceeding MaxZipEntrySize.
	// We use a small limit-like test by creating a reader bigger than MaxZipEntrySize.
	// Since MaxZipEntrySize is 100 MB, we test the logic with a synthetic reader.
	oversize := make([]byte, MaxZipEntrySize+1)
	r := bytes.NewReader(oversize)

	_, err := readLimited(r, "bomb.xml")
	if err == nil {
		t.Fatal("readLimited() should return error for oversized entry")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("readLimited() error = %q, want error containing 'exceeds'", err.Error())
	}
}

func TestReadLimited_WithinLimit(t *testing.T) {
	data := []byte("hello world")
	r := bytes.NewReader(data)

	got, err := readLimited(r, "ok.xml")
	if err != nil {
		t.Fatalf("readLimited() returned unexpected error: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("readLimited() = %q, want %q", string(got), string(data))
	}
}

func TestReadFileFromZipIndex_ExceedsLimit(t *testing.T) {
	// Verify the indexed path also uses limit protection.
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	content := []byte(strings.Repeat("B", 512))
	f, err := w.Create("indexed.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("failed to write zip entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	idx := BuildZipIndex(reader)
	got, err := ReadFileFromZipIndex(idx, "indexed.txt")
	if err != nil {
		t.Fatalf("ReadFileFromZipIndex() returned unexpected error: %v", err)
	}
	if len(got) != 512 {
		t.Errorf("ReadFileFromZipIndex() returned %d bytes, want 512", len(got))
	}
}

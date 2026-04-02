package pptx

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// createTestZIP creates a minimal test ZIP archive in memory.
func createTestZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	return buf.Bytes()
}

func TestOpenFromBytes(t *testing.T) {
	testFiles := map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types/>`,
		"ppt/slides/slide1.xml": "<p:sld/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Verify entries are accessible
	entries := pkg.Entries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Verify content can be read
	content, err := pkg.ReadEntry("[Content_Types].xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != testFiles["[Content_Types].xml"] {
		t.Errorf("content mismatch: got %q", string(content))
	}
}

func TestOpen_InvalidZIP(t *testing.T) {
	invalidData := []byte("not a zip file")
	r := bytes.NewReader(invalidData)

	_, err := Open(r, int64(len(invalidData)))
	if err == nil {
		t.Fatal("expected error for invalid ZIP")
	}
}

func TestPackage_Entries(t *testing.T) {
	testFiles := map[string]string{
		"b.xml": "<b/>",
		"a.xml": "<a/>",
		"c.xml": "<c/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	entries := pkg.Entries()

	// Entries should be sorted alphabetically
	expected := []string{"a.xml", "b.xml", "c.xml"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}

	for i, name := range expected {
		if entries[i] != name {
			t.Errorf("entry %d: expected %q, got %q", i, name, entries[i])
		}
	}
}

func TestPackage_HasEntry(t *testing.T) {
	testFiles := map[string]string{
		"exists.xml": "<data/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	if !pkg.HasEntry("exists.xml") {
		t.Error("expected HasEntry to return true for existing entry")
	}

	if pkg.HasEntry("notexists.xml") {
		t.Error("expected HasEntry to return false for non-existing entry")
	}
}

func TestPackage_ReadEntry_NotFound(t *testing.T) {
	testFiles := map[string]string{
		"exists.xml": "<data/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	_, err = pkg.ReadEntry("notexists.xml")
	if err == nil {
		t.Fatal("expected error for non-existing entry")
	}
}

func TestPackage_SetEntry_Modify(t *testing.T) {
	testFiles := map[string]string{
		"data.xml": "<original/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Modify existing entry
	pkg.SetEntry("data.xml", []byte("<modified/>"))

	// Read should return modified content
	content, err := pkg.ReadEntry("data.xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != "<modified/>" {
		t.Errorf("expected modified content, got %q", string(content))
	}
}

func TestPackage_SetEntry_Add(t *testing.T) {
	testFiles := map[string]string{
		"existing.xml": "<existing/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Add new entry
	pkg.SetEntry("new.xml", []byte("<new/>"))

	// Should be readable
	content, err := pkg.ReadEntry("new.xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != "<new/>" {
		t.Errorf("expected new content, got %q", string(content))
	}
}

func TestPackage_DeleteEntry(t *testing.T) {
	testFiles := map[string]string{
		"keep.xml":   "<keep/>",
		"delete.xml": "<delete/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	pkg.DeleteEntry("delete.xml")

	// Save and reopen
	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	if pkg2.HasEntry("delete.xml") {
		t.Error("deleted entry should not be present")
	}

	if !pkg2.HasEntry("keep.xml") {
		t.Error("kept entry should be present")
	}
}

func TestPackage_Save_Deterministic(t *testing.T) {
	// Create files in non-alphabetical order to test sorting
	testFiles := map[string]string{
		"z.xml": "<z/>",
		"a.xml": "<a/>",
		"m.xml": "<m/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Save multiple times
	output1, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes 1 failed: %v", err)
	}

	output2, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes 2 failed: %v", err)
	}

	// Outputs should be identical (deterministic)
	hash1 := sha256.Sum256(output1)
	hash2 := sha256.Sum256(output2)

	if hash1 != hash2 {
		t.Error("save output is not deterministic")
	}
}

func TestPackage_Save_SortedEntries(t *testing.T) {
	testFiles := map[string]string{
		"z.xml": "<z/>",
		"a.xml": "<a/>",
		"m.xml": "<m/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// Open output and verify order
	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Read entries from the underlying zip reader to check actual order
	var actualOrder []string
	for _, f := range pkg2.reader.File {
		actualOrder = append(actualOrder, f.Name)
	}

	expected := []string{"a.xml", "m.xml", "z.xml"}
	if len(actualOrder) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(actualOrder))
	}

	for i, name := range expected {
		if actualOrder[i] != name {
			t.Errorf("entry %d: expected %q, got %q", i, name, actualOrder[i])
		}
	}
}

func TestPackage_Save_FixedTimestamp(t *testing.T) {
	testFiles := map[string]string{
		"test.xml": "<test/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// Open output and check timestamp
	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	for _, f := range pkg2.reader.File {
		// Timestamp should be close to utils.DeterministicTimestamp
		// (zip format has limited precision, so check within 2 seconds)
		diff := f.Modified.Sub(utils.DeterministicTimestamp)
		if diff < -2*time.Second || diff > 2*time.Second {
			t.Errorf("entry %s has unexpected timestamp: %v (expected ~%v)",
				f.Name, f.Modified, utils.DeterministicTimestamp)
		}
	}
}

func TestPackage_SaveWithTimestamp(t *testing.T) {
	testFiles := map[string]string{
		"test.xml": "<test/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	customTime := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)

	var buf bytes.Buffer
	if err := pkg.SaveWithTimestamp(&buf, customTime); err != nil {
		t.Fatalf("SaveWithTimestamp failed: %v", err)
	}

	// Open output and check timestamp
	pkg2, err := OpenFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	for _, f := range pkg2.reader.File {
		diff := f.Modified.Sub(customTime)
		if diff < -2*time.Second || diff > 2*time.Second {
			t.Errorf("entry %s has unexpected timestamp: %v (expected ~%v)",
				f.Name, f.Modified, customTime)
		}
	}
}

func TestPackage_Clone(t *testing.T) {
	testFiles := map[string]string{
		"data.xml": "<original/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Modify original
	pkg.SetEntry("data.xml", []byte("<modified/>"))

	// Clone
	clone := pkg.Clone()

	// Modify clone differently
	clone.SetEntry("data.xml", []byte("<clone/>"))

	// Original should still have its modification
	origContent, _ := pkg.ReadEntry("data.xml")
	if string(origContent) != "<modified/>" {
		t.Errorf("original was affected by clone: %q", string(origContent))
	}

	// Clone should have its own modification
	cloneContent, _ := clone.ReadEntry("data.xml")
	if string(cloneContent) != "<clone/>" {
		t.Errorf("clone content wrong: %q", string(cloneContent))
	}
}

func TestPackage_ModifyThenDelete(t *testing.T) {
	testFiles := map[string]string{
		"data.xml": "<original/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Modify then delete
	pkg.SetEntry("data.xml", []byte("<modified/>"))
	pkg.DeleteEntry("data.xml")

	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	if pkg2.HasEntry("data.xml") {
		t.Error("deleted entry should not exist")
	}
}

func TestPackage_DeleteThenSet(t *testing.T) {
	testFiles := map[string]string{
		"data.xml": "<original/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Delete then set (re-add)
	pkg.DeleteEntry("data.xml")
	pkg.SetEntry("data.xml", []byte("<restored/>"))

	content, err := pkg.ReadEntry("data.xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != "<restored/>" {
		t.Errorf("expected restored content, got %q", string(content))
	}
}

func TestOpenFile(t *testing.T) {
	// Create a temp file with test ZIP
	testFiles := map[string]string{
		"test.xml": "<test/>",
	}
	zipData := createTestZIP(t, testFiles)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(tmpFile, zipData, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	pkg, closer, err := OpenFile(tmpFile)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer func() { _ = closer.Close() }()

	if !pkg.HasEntry("test.xml") {
		t.Error("expected entry test.xml")
	}

	content, err := pkg.ReadEntry("test.xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != "<test/>" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestOpenFile_NotFound(t *testing.T) {
	_, _, err := OpenFile("/nonexistent/path/file.pptx")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestPackage_SaveToFile(t *testing.T) {
	testFiles := map[string]string{
		"a.xml": "<a/>",
		"b.xml": "<b/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	// Modify content
	pkg.SetEntry("a.xml", []byte("<modified-a/>"))

	// Save to file
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	f, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}

	if err := pkg.Save(f); err != nil {
		_ = f.Close()
		t.Fatalf("Save failed: %v", err)
	}
	_ = f.Close()

	// Verify output
	pkg2, closer, err := OpenFile(outputPath)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer func() { _ = closer.Close() }()

	content, err := pkg2.ReadEntry("a.xml")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}
	if string(content) != "<modified-a/>" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestPackage_LargeContent(t *testing.T) {
	// Test with larger content to ensure streaming works
	largeContent := bytes.Repeat([]byte("x"), 1024*1024) // 1MB

	testFiles := map[string]string{
		"small.xml": "<small/>",
	}
	zipData := createTestZIP(t, testFiles)

	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	pkg.SetEntry("large.bin", largeContent)

	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	readContent, err := pkg2.ReadEntry("large.bin")
	if err != nil {
		t.Fatalf("ReadEntry failed: %v", err)
	}

	if !bytes.Equal(readContent, largeContent) {
		t.Errorf("large content mismatch: got %d bytes", len(readContent))
	}
}

func TestPackage_EmptyZIP(t *testing.T) {
	// Create empty ZIP
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	pkg, err := OpenFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	entries := pkg.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	// Add entry to empty package
	pkg.SetEntry("new.xml", []byte("<new/>"))

	output, err := pkg.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	pkg2, err := OpenFromBytes(output)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	if !pkg2.HasEntry("new.xml") {
		t.Error("expected entry new.xml")
	}
}

// BenchmarkSave benchmarks the deterministic save operation
func BenchmarkSave(b *testing.B) {
	// Create a package with multiple entries
	testFiles := make(map[string]string)
	for i := 0; i < 50; i++ {
		testFiles[fmt.Sprintf("file%02d.xml", i)] = "<data/>"
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range testFiles {
		w, err := zw.Create(name)
		if err != nil {
			b.Fatalf("failed to create entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			b.Fatalf("failed to write entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		b.Fatalf("failed to close zip: %v", err)
	}

	zipData := buf.Bytes()
	pkg, err := OpenFromBytes(zipData)
	if err != nil {
		b.Fatalf("failed to open package: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		if err := pkg.Save(&out); err != nil {
			b.Fatalf("failed to save: %v", err)
		}
	}
}

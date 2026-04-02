package template

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

func TestOpenTemplate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string // Returns path to test file
		wantErr string
	}{
		{
			name: "valid template with layouts",
			setup: func(t *testing.T) string {
				return createTestPPTX(t, []string{
					"ppt/presentation.xml",
					"ppt/slideLayouts/slideLayout1.xml",
					"ppt/slideLayouts/slideLayout2.xml",
				})
			},
			wantErr: "",
		},
		{
			name: "file not found",
			setup: func(t *testing.T) string {
				return "/nonexistent/template.pptx"
			},
			wantErr: "template file not found",
		},
		{
			name: "directory instead of file",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return dir
			},
			wantErr: "template path is a directory",
		},
		{
			name: "not a ZIP file",
			setup: func(t *testing.T) string {
				f, err := os.CreateTemp("", "notzip-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = f.Close() }()
				if _, err := f.WriteString("This is not a ZIP file"); err != nil {
					t.Fatal(err)
				}
				return f.Name()
			},
			wantErr: "invalid PPTX format",
		},
		{
			name: "missing presentation.xml",
			setup: func(t *testing.T) string {
				return createTestPPTX(t, []string{
					"ppt/slideLayouts/slideLayout1.xml",
				})
			},
			wantErr: "corrupted template: missing ppt/presentation.xml",
		},
		{
			name: "no slide layouts",
			setup: func(t *testing.T) string {
				return createTestPPTX(t, []string{
					"ppt/presentation.xml",
				})
			},
			wantErr: "templates must have layouts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			reader, err := OpenTemplate(path)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("OpenTemplate() error = nil, wantErr %q", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("OpenTemplate() error = %q, wantErr substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("OpenTemplate() unexpected error = %v", err)
				return
			}

			defer func() { _ = reader.Close() }()

			// Verify reader properties
			if reader.Path() != path {
				t.Errorf("Path() = %q, want %q", reader.Path(), path)
			}

			if reader.Hash() == "" {
				t.Error("Hash() returned empty string")
			}

			if len(reader.Hash()) != 64 { // SHA256 hex is 64 chars
				t.Errorf("Hash() length = %d, want 64", len(reader.Hash()))
			}
		})
	}
}

func TestReader_ReadFile(t *testing.T) {
	// Create test PPTX with known content
	content := []byte("<presentation>test</presentation>")
	path := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":              content,
		"ppt/slideLayouts/slideLayout1.xml": []byte("<layout1/>"),
		"ppt/slideLayouts/slideLayout2.xml": []byte("<layout2/>"),
		"ppt/slideMasters/slideMaster1.xml": []byte("<master/>"),
		"ppt/theme/theme1.xml":              []byte("<theme/>"),
	})

	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  string
	}{
		{
			name:     "read existing file",
			filename: "ppt/presentation.xml",
			want:     string(content),
			wantErr:  "",
		},
		{
			name:     "read layout file",
			filename: "ppt/slideLayouts/slideLayout1.xml",
			want:     "<layout1/>",
			wantErr:  "",
		},
		{
			name:     "file not found",
			filename: "ppt/nonexistent.xml",
			want:     "",
			wantErr:  "file not found in template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := reader.ReadFile(tt.filename)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("ReadFile() error = nil, wantErr %q", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ReadFile() error = %q, wantErr substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ReadFile() unexpected error = %v", err)
				return
			}

			if string(data) != tt.want {
				t.Errorf("ReadFile() = %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestReader_ListFiles(t *testing.T) {
	path := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":              []byte("<presentation/>"),
		"ppt/slideLayouts/slideLayout1.xml": []byte("<layout1/>"),
		"ppt/slideLayouts/slideLayout2.xml": []byte("<layout2/>"),
		"ppt/slideLayouts/slideLayout3.xml": []byte("<layout3/>"),
		"ppt/slideMasters/slideMaster1.xml": []byte("<master/>"),
		"ppt/theme/theme1.xml":              []byte("<theme/>"),
	})

	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	tests := []struct {
		name      string
		pattern   string
		wantCount int
		wantErr   string
	}{
		{
			name:      "list all layouts",
			pattern:   "ppt/slideLayouts/*.xml",
			wantCount: 3,
			wantErr:   "",
		},
		{
			name:      "list specific layout",
			pattern:   "ppt/slideLayouts/slideLayout1.xml",
			wantCount: 1,
			wantErr:   "",
		},
		{
			name:      "no matches",
			pattern:   "ppt/slides/*.xml",
			wantCount: 0,
			wantErr:   "",
		},
		{
			name:      "invalid pattern",
			pattern:   "ppt/slideLayouts/[invalid",
			wantCount: 0,
			wantErr:   "invalid pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := reader.ListFiles(tt.pattern)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("ListFiles() error = nil, wantErr %q", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ListFiles() error = %q, wantErr substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ListFiles() unexpected error = %v", err)
				return
			}

			if len(files) != tt.wantCount {
				t.Errorf("ListFiles() returned %d files, want %d", len(files), tt.wantCount)
			}
		})
	}
}

func TestReader_Hash(t *testing.T) {
	// Create two files with different content
	path1 := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":              []byte("<presentation>file1</presentation>"),
		"ppt/slideLayouts/slideLayout1.xml": []byte("<layout1/>"),
	})

	path2 := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":              []byte("<presentation>file2</presentation>"),
		"ppt/slideLayouts/slideLayout1.xml": []byte("<layout1/>"),
	})

	reader1, err := OpenTemplate(path1)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader1.Close() }()

	reader2, err := OpenTemplate(path2)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader2.Close() }()

	// Different files should have different hashes
	if reader1.Hash() == reader2.Hash() {
		t.Error("Expected different hashes for different files")
	}

	// Same file should have same hash
	reader3, err := OpenTemplate(path1)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader3.Close() }()

	if reader1.Hash() != reader3.Hash() {
		t.Errorf("Same file has different hashes: %s vs %s", reader1.Hash(), reader3.Hash())
	}
}

func TestReader_Close(t *testing.T) {
	path := createTestPPTX(t, []string{
		"ppt/presentation.xml",
		"ppt/slideLayouts/slideLayout1.xml",
	})

	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}

	// Should not error on close
	if err := reader.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Multiple closes should not panic
	if err := reader.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

// Helper functions

// createTestPPTX creates a minimal valid PPTX file with the specified files.
func createTestPPTX(t *testing.T, files []string) string {
	t.Helper()
	content := make(map[string][]byte)
	for _, f := range files {
		content[f] = []byte("<xml/>")
	}
	return createTestPPTXWithContent(t, content)
}

// createTestPPTXWithContent creates a PPTX file with specific content for each file.
func createTestPPTXWithContent(t *testing.T, content map[string][]byte) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "test-*.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tmpfile.Close() }()

	zipWriter := zip.NewWriter(tmpfile)
	if zipWriter == nil {
		t.Fatal("failed to create zip writer")
	}

	for name, data := range content {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.Remove(tmpfile.Name())
	})

	return tmpfile.Name()
}

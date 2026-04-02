package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockCommandRunner tracks command invocations and allows configuring responses.
type MockCommandRunner struct {
	// Calls records all command invocations as "name arg1 arg2 ..."
	Calls []string
	// RunFunc allows custom behavior for the Run method.
	// If nil, returns nil (success).
	RunFunc func(name string, args ...string) error
}

// Run records the command invocation and executes RunFunc if set.
func (m *MockCommandRunner) Run(name string, args ...string) error {
	call := name + " " + strings.Join(args, " ")
	m.Calls = append(m.Calls, call)
	if m.RunFunc != nil {
		return m.RunFunc(name, args...)
	}
	return nil
}

// --- Tests for CommandRunner interface ---

func TestRealCommandRunner_Run(t *testing.T) {
	runner := &RealCommandRunner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Test a simple command that should succeed
	err := runner.Run("echo", "hello")
	if err != nil {
		t.Errorf("expected echo to succeed, got: %v", err)
	}

	// Test a command that should fail
	err = runner.Run("nonexistent-command-12345")
	if err == nil {
		t.Error("expected nonexistent command to fail")
	}
}

// --- Tests for input validation ---

func TestConvertPPTXToJPG_InputFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &MockCommandRunner{}

	err := convertPPTXToJPGWithRunner("/nonexistent/path/test.pptx", tmpDir, 150, mock)
	if err == nil {
		t.Fatal("expected error for nonexistent input file")
	}
	if !strings.Contains(err.Error(), "input file does not exist") {
		t.Errorf("expected 'input file does not exist' error, got: %v", err)
	}

	// No commands should have been run
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 command calls, got: %v", mock.Calls)
	}
}

func TestConvertPPTXToJPG_OutputDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use nested output directory that doesn't exist
	outputDir := filepath.Join(tmpDir, "nested", "output", "dir")

	// Mock that creates PDF and JPG files when commands succeed
	mock := &MockCommandRunner{
		RunFunc: func(name string, args ...string) error {
			if name == "libreoffice" {
				// Simulate LibreOffice creating the PDF
				pdfPath := filepath.Join(outputDir, "test.pdf")
				if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
					t.Fatalf("mock failed to create PDF: %v", err)
				}
			} else if name == "convert" {
				// Simulate ImageMagick creating JPG files
				for i := 0; i < 3; i++ {
					jpgPath := filepath.Join(outputDir, "test-slide-"+string(rune('0'+i))+".jpg")
					if err := os.WriteFile(jpgPath, []byte("dummy jpg"), 0644); err != nil {
						t.Fatalf("mock failed to create JPG: %v", err)
					}
				}
			}
			return nil
		},
	}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("expected output directory to be created")
	}
}

// --- Tests for LibreOffice step ---

func TestConvertPPTXToJPG_LibreOfficeFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	mock := &MockCommandRunner{
		RunFunc: func(name string, args ...string) error {
			if name == "libreoffice" {
				return errors.New("libreoffice not found")
			}
			return nil
		},
	}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
	if err == nil {
		t.Fatal("expected error when LibreOffice fails")
	}
	if !strings.Contains(err.Error(), "LibreOffice conversion failed") {
		t.Errorf("expected 'LibreOffice conversion failed' error, got: %v", err)
	}

	// Only LibreOffice should have been called
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 command call, got: %v", mock.Calls)
	}
	if !strings.HasPrefix(mock.Calls[0], "libreoffice") {
		t.Errorf("expected libreoffice call, got: %s", mock.Calls[0])
	}
}

func TestConvertPPTXToJPG_PDFNotCreated(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	// Mock that doesn't create the PDF file
	mock := &MockCommandRunner{}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
	if err == nil {
		t.Fatal("expected error when PDF is not created")
	}
	if !strings.Contains(err.Error(), "PDF was not created") {
		t.Errorf("expected 'PDF was not created' error, got: %v", err)
	}
}

// --- Tests for ImageMagick step ---

func TestConvertPPTXToJPG_ImageMagickFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	mock := &MockCommandRunner{
		RunFunc: func(name string, args ...string) error {
			if name == "libreoffice" {
				// Simulate LibreOffice creating the PDF
				pdfPath := filepath.Join(outputDir, "test.pdf")
				if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
					t.Fatalf("mock failed to create PDF: %v", err)
				}
				return nil
			}
			if name == "convert" {
				return errors.New("imagemagick not found")
			}
			return nil
		},
	}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
	if err == nil {
		t.Fatal("expected error when ImageMagick fails")
	}
	if !strings.Contains(err.Error(), "ImageMagick conversion failed") {
		t.Errorf("expected 'ImageMagick conversion failed' error, got: %v", err)
	}

	// Both LibreOffice and convert should have been called
	if len(mock.Calls) != 2 {
		t.Errorf("expected 2 command calls, got: %v", mock.Calls)
	}
}

func TestConvertPPTXToJPG_NoJPGsGenerated(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	mock := &MockCommandRunner{
		RunFunc: func(name string, args ...string) error {
			if name == "libreoffice" {
				// Simulate LibreOffice creating the PDF
				pdfPath := filepath.Join(outputDir, "test.pdf")
				if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
					t.Fatalf("mock failed to create PDF: %v", err)
				}
			}
			// ImageMagick succeeds but doesn't create any files
			return nil
		},
	}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
	if err == nil {
		t.Fatal("expected error when no JPGs are generated")
	}
	if !strings.Contains(err.Error(), "no JPG files were generated") {
		t.Errorf("expected 'no JPG files were generated' error, got: %v", err)
	}
}

// --- Tests for successful conversion ---

func TestConvertPPTXToJPG_SuccessfulConversion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	pptxPath := filepath.Join(tmpDir, "presentation.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	density := 200

	mock := &MockCommandRunner{
		RunFunc: func(name string, args ...string) error {
			if name == "libreoffice" {
				// Simulate LibreOffice creating the PDF
				pdfPath := filepath.Join(outputDir, "presentation.pdf")
				if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
					t.Fatalf("mock failed to create PDF: %v", err)
				}
			} else if name == "convert" {
				// Simulate ImageMagick creating 5 JPG files
				for i := 0; i < 5; i++ {
					jpgPath := filepath.Join(outputDir, "presentation-slide-"+string(rune('0'+i))+".jpg")
					if err := os.WriteFile(jpgPath, []byte("dummy jpg"), 0644); err != nil {
						t.Fatalf("mock failed to create JPG: %v", err)
					}
				}
			}
			return nil
		},
	}

	err := convertPPTXToJPGWithRunner(pptxPath, outputDir, density, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both commands were called
	if len(mock.Calls) != 2 {
		t.Errorf("expected 2 command calls, got: %d", len(mock.Calls))
	}

	// Verify LibreOffice command
	if !strings.HasPrefix(mock.Calls[0], "libreoffice") {
		t.Errorf("expected libreoffice call first, got: %s", mock.Calls[0])
	}
	if !strings.Contains(mock.Calls[0], "--headless") {
		t.Error("expected --headless flag in libreoffice call")
	}
	if !strings.Contains(mock.Calls[0], "--convert-to pdf") {
		t.Error("expected --convert-to pdf in libreoffice call")
	}
	if !strings.Contains(mock.Calls[0], pptxPath) {
		t.Error("expected input path in libreoffice call")
	}

	// Verify ImageMagick command
	if !strings.HasPrefix(mock.Calls[1], "convert") {
		t.Errorf("expected convert call second, got: %s", mock.Calls[1])
	}
	if !strings.Contains(mock.Calls[1], "-density 200") {
		t.Error("expected -density 200 in convert call")
	}
	if !strings.Contains(mock.Calls[1], "-quality 90") {
		t.Error("expected -quality 90 in convert call")
	}
}

// --- Tests for various density values ---

func TestConvertPPTXToJPG_DensityValues(t *testing.T) {
	tests := []struct {
		name     string
		density  int
		expected string
	}{
		{"default 150", 150, "-density 150"},
		{"high 300", 300, "-density 300"},
		{"low 72", 72, "-density 72"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create input file
			pptxPath := filepath.Join(tmpDir, "test.pptx")
			if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			outputDir := filepath.Join(tmpDir, "output")

			mock := &MockCommandRunner{
				RunFunc: func(name string, args ...string) error {
					if name == "libreoffice" {
						pdfPath := filepath.Join(outputDir, "test.pdf")
						if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
							t.Fatalf("mock failed to create PDF: %v", err)
						}
					} else if name == "convert" {
						jpgPath := filepath.Join(outputDir, "test-slide-0.jpg")
						if err := os.WriteFile(jpgPath, []byte("dummy jpg"), 0644); err != nil {
							t.Fatalf("mock failed to create JPG: %v", err)
						}
					}
					return nil
				},
			}

			err := convertPPTXToJPGWithRunner(pptxPath, outputDir, tt.density, mock)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify density was passed correctly
			found := false
			for _, call := range mock.Calls {
				if strings.Contains(call, tt.expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s in command calls, got: %v", tt.expected, mock.Calls)
			}
		})
	}
}

// --- Tests for file path handling ---

func TestConvertPPTXToJPG_BaseName(t *testing.T) {
	tests := []struct {
		name        string
		inputFile   string
		expectedPDF string
		expectedJPG string
	}{
		{
			name:        "simple name",
			inputFile:   "slides.pptx",
			expectedPDF: "slides.pdf",
			expectedJPG: "slides-slide-",
		},
		{
			name:        "name with spaces",
			inputFile:   "my slides.pptx",
			expectedPDF: "my slides.pdf",
			expectedJPG: "my slides-slide-",
		},
		{
			name:        "name with dots",
			inputFile:   "v1.2.slides.pptx",
			expectedPDF: "v1.2.slides.pdf",
			expectedJPG: "v1.2.slides-slide-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create input file
			pptxPath := filepath.Join(tmpDir, tt.inputFile)
			if err := os.WriteFile(pptxPath, []byte("dummy pptx"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			outputDir := filepath.Join(tmpDir, "output")

			mock := &MockCommandRunner{
				RunFunc: func(name string, args ...string) error {
					if name == "libreoffice" {
						pdfPath := filepath.Join(outputDir, tt.expectedPDF)
						if err := os.WriteFile(pdfPath, []byte("dummy pdf"), 0644); err != nil {
							t.Fatalf("mock failed to create PDF: %v", err)
						}
					} else if name == "convert" {
						jpgPath := filepath.Join(outputDir, tt.expectedJPG+"0.jpg")
						if err := os.WriteFile(jpgPath, []byte("dummy jpg"), 0644); err != nil {
							t.Fatalf("mock failed to create JPG: %v", err)
						}
					}
					return nil
				},
			}

			err := convertPPTXToJPGWithRunner(pptxPath, outputDir, 150, mock)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the PDF path was checked
			pdfPath := filepath.Join(outputDir, tt.expectedPDF)
			if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
				t.Errorf("expected PDF at %s", pdfPath)
			}
		})
	}
}

// --- Test for defaultRunner initialization ---

func TestDefaultRunnerIsInitialized(t *testing.T) {
	if defaultRunner == nil {
		t.Error("defaultRunner should be initialized")
	}
}

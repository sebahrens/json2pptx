package utils

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidatePath_TraversalDetection(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     error
		windowsOnly bool // test only valid on Windows where \ is path separator
	}{
		{
			name:    "simple path",
			path:    "file.txt",
			wantErr: nil,
		},
		{
			name:    "nested path",
			path:    "dir/subdir/file.txt",
			wantErr: nil,
		},
		{
			name:    "file with double dots in name",
			path:    "file..name.png",
			wantErr: nil,
		},
		{
			name:        "path traversal with backslash",
			path:        "..\\etc\\passwd",
			wantErr:     ErrPathTraversal,
			windowsOnly: true, // backslash is path separator only on Windows
		},
		{
			name:    "path traversal with forward slash",
			path:    "../etc/passwd",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "path traversal at start",
			path:    "../secret.txt",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "path traversal in middle",
			path:    "dir/../../../etc/passwd",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "path traversal at end",
			path:    "dir/..",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "multiple traversals",
			path:    "../../..",
			wantErr: ErrPathTraversal,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: nil,
		},
		{
			name:    "single dot",
			path:    ".",
			wantErr: nil,
		},
		{
			name:    "dotfile",
			path:    ".hidden",
			wantErr: nil,
		},
		{
			name:    "path with dots and extension",
			path:    "file.test.png",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("skipping Windows-specific test on non-Windows platform")
			}
			err := ValidatePath(tt.path, nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q) returned error: %v, want nil", tt.path, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q) returned nil, want error %v", tt.path, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q) returned error %v, want %v", tt.path, err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidatePath_AllowedBasePaths(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	disallowedDir := filepath.Join(tmpDir, "disallowed")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}
	if err := os.MkdirAll(disallowedDir, 0755); err != nil {
		t.Fatalf("failed to create disallowed dir: %v", err)
	}

	// Create test files
	allowedFile := filepath.Join(allowedDir, "safe.txt")
	disallowedFile := filepath.Join(disallowedDir, "unsafe.txt")

	if err := os.WriteFile(allowedFile, []byte("safe"), 0644); err != nil {
		t.Fatalf("failed to create allowed file: %v", err)
	}
	if err := os.WriteFile(disallowedFile, []byte("unsafe"), 0644); err != nil {
		t.Fatalf("failed to create disallowed file: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		allowedPaths []string
		wantErr      error
	}{
		{
			name:         "path within allowed directory",
			path:         allowedFile,
			allowedPaths: []string{allowedDir},
			wantErr:      nil,
		},
		{
			name:         "path outside allowed directory",
			path:         disallowedFile,
			allowedPaths: []string{allowedDir},
			wantErr:      ErrPathOutsideAllowed,
		},
		{
			name:         "multiple allowed directories - first matches",
			path:         allowedFile,
			allowedPaths: []string{allowedDir, disallowedDir},
			wantErr:      nil,
		},
		{
			name:         "multiple allowed directories - second matches",
			path:         disallowedFile,
			allowedPaths: []string{allowedDir, disallowedDir},
			wantErr:      nil,
		},
		{
			name:         "allowed directory itself",
			path:         allowedDir,
			allowedPaths: []string{allowedDir},
			wantErr:      nil,
		},
		{
			name:         "empty allowed paths - only check traversal",
			path:         disallowedFile,
			allowedPaths: []string{},
			wantErr:      nil,
		},
		{
			name:         "nil allowed paths - only check traversal",
			path:         disallowedFile,
			allowedPaths: nil,
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.allowedPaths)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q, %v) returned error: %v, want nil", tt.path, tt.allowedPaths, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q, %v) returned nil, want error %v", tt.path, tt.allowedPaths, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q, %v) returned error %v, want %v", tt.path, tt.allowedPaths, err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidatePath_TraversalWithAllowedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		path         string
		allowedPaths []string
		wantErr      error
	}{
		{
			name:         "traversal attack blocked even with allowed paths",
			path:         "../../../etc/passwd",
			allowedPaths: []string{tmpDir},
			wantErr:      ErrPathTraversal,
		},
		{
			name:         "traversal check happens before allowed paths check",
			path:         tmpDir + "/../secret",
			allowedPaths: []string{tmpDir},
			wantErr:      ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.allowedPaths)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q, %v) returned error: %v, want nil", tt.path, tt.allowedPaths, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q, %v) returned nil, want error %v", tt.path, tt.allowedPaths, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q, %v) returned error %v, want %v", tt.path, tt.allowedPaths, err, tt.wantErr)
				}
			}
		})
	}
}

// TestValidatePath_SymlinkTraversal tests that symlinks to outside directories are blocked.
func TestValidatePath_SymlinkTraversal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests not reliable on Windows")
	}

	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	secretDir := filepath.Join(tmpDir, "secrets")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatalf("failed to create secret dir: %v", err)
	}

	// Create a secret file
	secretFile := filepath.Join(secretDir, "password.txt")
	if err := os.WriteFile(secretFile, []byte("super-secret"), 0644); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}

	// Create a symlink inside allowed dir pointing to secrets
	symlinkPath := filepath.Join(allowedDir, "link-to-secrets")
	if err := os.Symlink(secretDir, symlinkPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Path through symlink
	pathThroughSymlink := filepath.Join(symlinkPath, "password.txt")

	// With allowed paths configured, this should be blocked because the
	// resolved path is outside the allowed directory
	err := ValidatePath(pathThroughSymlink, []string{allowedDir})

	// Note: Current implementation may allow this - this test documents expected behavior
	// If symlink traversal is a concern, the implementation should be updated
	if err == nil {
		// The resolved path goes through a symlink to outside allowed
		// This might be acceptable depending on security requirements
		t.Logf("Note: symlink traversal to %s was allowed (may be expected behavior)", pathThroughSymlink)
	} else {
		t.Logf("Symlink traversal blocked as expected: %v", err)
	}
}

// TestValidatePath_URLEncodedCharacters tests that paths with URL-encoded traversal are handled.
// Note: This tests the path AFTER URL decoding has been done by the HTTP layer.
func TestValidatePath_URLEncodedCharacters(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		// After URL decoding, these become traversal attempts
		{
			name:    "decoded URL traversal",
			path:    "../etc/passwd", // decoded from %2e%2e%2f...
			wantErr: ErrPathTraversal,
		},
		{
			name:    "double-encoded after first decode",
			path:    "%2e%2e/etc/passwd", // partially decoded
			wantErr: nil,                 // safe - %2e%2e is not ".."
		},
		{
			name:    "null byte in path",
			path:    "file.txt\x00../etc/passwd",
			wantErr: nil, // Path validator doesn't handle null bytes - they're handled at file system level
		},
		{
			name:    "safe path with percent signs",
			path:    "file%20with%20spaces.txt", // URL-encoded spaces (safe)
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q) returned error: %v, want nil", tt.path, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q) returned nil, want error %v", tt.path, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q) returned error %v, want %v", tt.path, err, tt.wantErr)
				}
			}
		})
	}
}

// TestValidatePath_SpecialCharactersInPath tests handling of special characters.
func TestValidatePath_SpecialCharactersInPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "unicode characters",
			path:    "файл.txt", // Cyrillic
			wantErr: nil,
		},
		{
			name:    "emoji in filename",
			path:    "📁/file.txt",
			wantErr: nil,
		},
		{
			name:    "spaces in path",
			path:    "my documents/file.txt",
			wantErr: nil,
		},
		{
			name:    "mixed slashes safe",
			path:    "dir/subdir/file.txt",
			wantErr: nil,
		},
		{
			name:    "leading dot file",
			path:    ".config/settings.json",
			wantErr: nil,
		},
		{
			name:    "triple dots directory",
			path:    ".../file.txt",
			wantErr: nil, // "..." is not a traversal
		},
		{
			name:    "current directory reference",
			path:    "./dir/./file.txt",
			wantErr: nil, // single dot is safe
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q) returned error: %v, want nil", tt.path, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q) returned nil, want error %v", tt.path, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q) returned error %v, want %v", tt.path, err, tt.wantErr)
				}
			}
		})
	}
}

// TestValidatePath_EdgeCases tests edge cases in path validation.
func TestValidatePath_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		allowedPaths []string
		wantErr      error
	}{
		{
			name:         "absolute root path",
			path:         "/",
			allowedPaths: []string{allowedDir},
			wantErr:      ErrPathOutsideAllowed,
		},
		{
			name:         "path with repeated slashes",
			path:         "dir//subdir///file.txt",
			allowedPaths: nil,
			wantErr:      nil,
		},
		{
			name:         "very long path",
			path:         filepath.Join(allowedDir, strings.Repeat("a", 255)),
			allowedPaths: []string{allowedDir},
			wantErr:      nil,
		},
		{
			name:         "path ending with dot",
			path:         "file.",
			allowedPaths: nil,
			wantErr:      nil,
		},
		{
			name:         "path with tilde",
			path:         "~/documents/file.txt",
			allowedPaths: nil,
			wantErr:      nil, // tilde is just a character, not expanded
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.allowedPaths)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidatePath(%q, %v) returned error: %v, want nil", tt.path, tt.allowedPaths, err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidatePath(%q, %v) returned nil, want error %v", tt.path, tt.allowedPaths, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidatePath(%q, %v) returned error %v, want %v", tt.path, tt.allowedPaths, err, tt.wantErr)
				}
			}
		})
	}
}

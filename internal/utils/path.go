// Package utils provides shared utility functions used across the application.
package utils

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Path validation errors
var (
	// ErrPathTraversal indicates an attempted path traversal attack.
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrPathOutsideAllowed indicates a path is outside allowed directories.
	ErrPathOutsideAllowed = errors.New("path outside allowed directories")
)

// ValidatePath validates that a path is safe to use.
// It prevents path traversal attacks by checking for ".." components.
//
// If allowedBasePaths is non-empty, it also ensures the resolved absolute path
// is within one of the allowed directories.
//
// Returns nil if the path is safe, or an error describing the security issue.
func ValidatePath(path string, allowedBasePaths []string) error {
	// Check for path traversal patterns in path components
	// We need to check individual path components, not just for ".." anywhere in the string
	// because "file..name.png" is safe but "../etc" is not
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if component == ".." {
			return fmt.Errorf("%w: path contains '..' component", ErrPathTraversal)
		}
	}

	// Also check for forward slash on all platforms (common in URLs/markdown)
	for _, component := range strings.Split(path, "/") {
		if component == ".." {
			return fmt.Errorf("%w: path contains '..' component", ErrPathTraversal)
		}
	}

	// If no allowed base paths are configured, only check for traversal
	if len(allowedBasePaths) == 0 {
		return nil
	}

	// Resolve to absolute path for comparison
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if the path is within any allowed directory
	for _, basePath := range allowedBasePaths {
		absBasePath, err := filepath.Abs(basePath)
		if err != nil {
			continue // Skip invalid base paths
		}

		// Ensure base path ends with separator for proper prefix matching
		if !strings.HasSuffix(absBasePath, string(filepath.Separator)) {
			absBasePath += string(filepath.Separator)
		}

		// Check if the path starts with the allowed base path
		if strings.HasPrefix(absPath, absBasePath) || absPath == strings.TrimSuffix(absBasePath, string(filepath.Separator)) {
			return nil
		}
	}

	return fmt.Errorf("%w: path must be within allowed directories", ErrPathOutsideAllowed)
}

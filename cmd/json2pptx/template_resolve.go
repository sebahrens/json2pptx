package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/templates"
)

const (
	// envTemplatesDir is the environment variable for setting the templates directory.
	envTemplatesDir = "JSON2PPTX_TEMPLATES_DIR"

	// envTemplatesDirLegacy is the old environment variable name, checked as a fallback.
	envTemplatesDirLegacy = "MD2PPTX_TEMPLATES_DIR"

	// userTemplatesDir is the subdirectory under the user's home directory.
	userTemplatesDir = ".json2pptx/templates"

	// userTemplatesDirLegacy is the old subdirectory name, checked as a fallback.
	userTemplatesDirLegacy = ".md2pptx/templates"
)

// resolveTemplatesDir finds the best templates directory based on priority.
// Search order:
//  1. flagValue (explicit --templates-dir flag, if non-empty and not the default)
//  2. $JSON2PPTX_TEMPLATES_DIR environment variable (falls back to $MD2PPTX_TEMPLATES_DIR)
//  3. ~/.json2pptx/templates/ (falls back to ~/.md2pptx/templates/)
//  4. ./templates/ (current working directory)
//  5. Embedded templates (lowest priority, always available)
//
// Returns the resolved path and whether it's using embedded templates.
// When using embedded templates, the returned path is empty.
func resolveTemplatesDir(flagValue string) (string, bool) {
	// 1. Explicit flag value (only if user actually set it, not the default)
	if flagValue != "" && flagValue != "./templates" {
		if dirExists(flagValue) {
			slog.Debug("templates dir: using --templates-dir flag", "path", flagValue)
			return flagValue, false
		}
		// If explicitly specified but doesn't exist, still return it
		// so the caller can produce a clear error message.
		slog.Warn("templates dir from flag does not exist", "path", flagValue)
		return flagValue, false
	}

	// 2. Environment variable (new name first, then legacy fallback)
	for _, envName := range []string{envTemplatesDir, envTemplatesDirLegacy} {
		if envDir := os.Getenv(envName); envDir != "" {
			if dirExists(envDir) {
				slog.Debug("templates dir: using env var", "var", envName, "path", envDir)
				return envDir, false
			}
			slog.Warn("templates dir from env var does not exist", "var", envName, "path", envDir)
		}
	}

	// 3. User home directory (new name first, then legacy fallback)
	if home, err := os.UserHomeDir(); err == nil {
		for _, subdir := range []string{userTemplatesDir, userTemplatesDirLegacy} {
			homeDir := filepath.Join(home, subdir)
			if dirExists(homeDir) {
				slog.Debug("templates dir: using home directory", "path", homeDir)
				return homeDir, false
			}
		}
	}

	// 4. Current working directory
	if dirExists("./templates") {
		abs, err := filepath.Abs("./templates")
		if err == nil {
			slog.Debug("templates dir: using ./templates", "path", abs)
			return abs, false
		}
		return "./templates", false
	}

	// 5. Embedded templates (always available)
	slog.Debug("templates dir: using embedded templates")
	return "", true
}

// resolveTemplatePath finds a specific template file across all search locations.
// It searches directories in priority order and falls back to embedded templates.
// Returns the full path to the .pptx file and a cleanup function. The cleanup
// function removes any temporary file created for embedded templates; callers
// must call it when done with the template path. For non-embedded templates the
// cleanup function is a no-op.
//
// The flagTemplatesDir is the value from --templates-dir (may be empty or default).
func resolveTemplatePath(templateName, flagTemplatesDir string) (string, func(), error) {
	// Strip .pptx extension if user included it (e.g., "my-template.pptx" -> "my-template")
	templateName = strings.TrimSuffix(templateName, ".pptx")
	filename := templateName + ".pptx"

	// Build candidate directories in priority order.
	var candidates []string

	// 1. Explicit flag
	if flagTemplatesDir != "" && flagTemplatesDir != "./templates" {
		candidates = append(candidates, flagTemplatesDir)
	}

	// 2. Environment variables (new name first, then legacy)
	for _, envName := range []string{envTemplatesDir, envTemplatesDirLegacy} {
		if envDir := os.Getenv(envName); envDir != "" {
			candidates = append(candidates, envDir)
		}
	}

	// 3. User home directory (new name first, then legacy)
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, userTemplatesDir))
		candidates = append(candidates, filepath.Join(home, userTemplatesDirLegacy))
	}

	// 4. Current directory
	candidates = append(candidates, "./templates")

	noop := func() {}

	// Search each candidate
	for _, dir := range candidates {
		path := filepath.Join(dir, filename)
		if fileExists(path) {
			abs, err := filepath.Abs(path)
			if err != nil {
				return path, noop, nil
			}
			return abs, noop, nil
		}
	}

	// 5. Embedded templates
	data, err := fs.ReadFile(templates.Embedded, filename)
	if err != nil {
		return "", noop, fmt.Errorf("template %q not found in any search location or embedded templates", templateName)
	}

	// Extract to temp file (PPTX libraries need a real file path)
	tmpFile, err := os.CreateTemp("", "json2pptx-template-*.pptx")
	if err != nil {
		return "", noop, fmt.Errorf("failed to create temp file for embedded template: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", noop, fmt.Errorf("failed to write embedded template to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", noop, fmt.Errorf("failed to close temp file for embedded template: %w", err)
	}

	slog.Debug("extracted embedded template to temp file",
		"template", templateName,
		"path", tmpFile.Name(),
	)

	cleanup := func() {
		if err := os.Remove(tmpFile.Name()); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to remove temp template file", "path", tmpFile.Name(), "err", err)
		}
	}

	return tmpFile.Name(), cleanup, nil
}

// listAvailableTemplates returns the names of all available templates
// (without .pptx extension), combining both disk and embedded templates.
func listAvailableTemplates(flagTemplatesDir string) []string {
	seen := make(map[string]bool)
	var names []string

	addFromDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pptx") {
				name := strings.TrimSuffix(e.Name(), ".pptx")
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}

	// Search disk directories in priority order
	dir, embedded := resolveTemplatesDir(flagTemplatesDir)
	if !embedded {
		addFromDir(dir)
	}

	// Always add embedded templates
	entries, err := fs.ReadDir(templates.Embedded, ".")
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pptx") {
				name := strings.TrimSuffix(e.Name(), ".pptx")
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}

	return names
}

// templateNotFoundError builds a descriptive error message when a template is not found.
// It lists available template names so the caller can correct typos.
func templateNotFoundError(templateName, flagTemplatesDir string) string {
	msg := fmt.Sprintf("template not found: %s", templateName)
	available := listAvailableTemplates(flagTemplatesDir)
	if len(available) > 0 {
		msg += fmt.Sprintf("; available templates: [%s]", strings.Join(available, ", "))
	}
	return msg
}

// dirExists checks if a directory exists and is actually a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists checks if a file exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

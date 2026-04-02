// Package data provides variable interpolation for slide presentations.
// Variables are defined in frontmatter (inline values or file references)
// and resolved before layout selection using {{ variable.path }} syntax.
package data

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/safeyaml"
)

// Context holds resolved variable data for interpolation.
type Context struct {
	// Vars holds the flattened variable tree. Keys are dot-separated paths.
	// Example: {"company": "Acme", "revenue.q4": 1200000}
	Vars map[string]any
}

// BuildContext constructs a data context from frontmatter data and CLI overrides.
// The baseDir is used to resolve relative file paths in data references.
// CLI overrides take precedence over frontmatter values.
func BuildContext(frontmatterData map[string]any, cliOverrides map[string]string, baseDir string) (*Context, []string, error) {
	ctx := &Context{Vars: make(map[string]any)}
	var warnings []string

	// Process frontmatter data values
	for key, val := range frontmatterData {
		switch v := val.(type) {
		case string:
			if isFileReference(v) {
				loaded, err := loadFile(v, baseDir)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("data: failed to load %q for key %q: %v", v, key, err))
					continue
				}
				ctx.Vars[key] = loaded
			} else {
				ctx.Vars[key] = v
			}
		default:
			ctx.Vars[key] = v
		}
	}

	// Apply CLI overrides (simple key=value, no file loading)
	for key, val := range cliOverrides {
		ctx.Vars[key] = val
	}

	return ctx, warnings, nil
}

// isFileReference returns true if the value looks like a file path
// (ends with .json, .yaml, .yml, or .csv).
func isFileReference(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".csv")
}

// loadFile loads data from a JSON, YAML, or CSV file.
func loadFile(path, baseDir string) (any, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	// Validate that the resolved path is within baseDir to prevent path traversal.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("invalid base dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return nil, fmt.Errorf("path %q is outside base directory", path)
	}

	// Limit file size to 1MB to prevent memory issues.
	// Check with os.Stat before reading to avoid allocating memory for huge files.
	const maxSize = 1024 * 1024
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file exceeds 1MB limit (%d bytes)", info.Size())
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return loadJSON(data)
	case strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"):
		return loadYAML(data)
	case strings.HasSuffix(lower, ".csv"):
		return loadCSV(data)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(path))
	}
}

func loadJSON(data []byte) (any, error) {
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return result, nil
}

func loadYAML(data []byte) (any, error) {
	var result any
	if err := safeyaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return result, nil
}


func loadCSV(data []byte) (any, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}

	if len(records) < 2 {
		return records, nil
	}

	// Convert to list of maps using first row as headers.
	headers := records[0]
	var rows []map[string]string
	for _, record := range records[1:] {
		row := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(record) {
				row[header] = record[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

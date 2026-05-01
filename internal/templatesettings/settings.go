// Package templatesettings provides persistent per-template settings
// stored as YAML sidecar files next to template .pptx files.
package templatesettings

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/safeyaml"
	"gopkg.in/yaml.v3"
)

// Kind identifies the type of setting being stored.
type Kind string

const (
	KindTableStyle Kind = "table_styles"
	KindCellStyle  Kind = "cell_styles"
)

// TableStyleDef is a named table style definition.
type TableStyleDef struct {
	StyleID       string `yaml:"style_id,omitempty" json:"style_id,omitempty"`
	UseTableStyle bool   `yaml:"use_table_style,omitempty" json:"use_table_style,omitempty"`
	HeaderRow     *bool  `yaml:"header_row,omitempty" json:"header_row,omitempty"`
	BandedRows    *bool  `yaml:"banded_rows,omitempty" json:"banded_rows,omitempty"`
}

// CellStyleFill defines fill for a cell style.
type CellStyleFill struct {
	Color string  `yaml:"color" json:"color"`
	Alpha float64 `yaml:"alpha,omitempty" json:"alpha,omitempty"`
}

// CellStyleBorder defines border for a cell style.
type CellStyleBorder struct {
	Color  string  `yaml:"color" json:"color"`
	Weight float64 `yaml:"weight,omitempty" json:"weight,omitempty"`
}

// CellStyleDef is a named cell style definition.
type CellStyleDef struct {
	Fill      *CellStyleFill   `yaml:"fill,omitempty" json:"fill,omitempty"`
	Border    *CellStyleBorder `yaml:"border,omitempty" json:"border,omitempty"`
	TextAlign string           `yaml:"text_align,omitempty" json:"text_align,omitempty"`
}

// File is the on-disk representation of a template settings file.
type File struct {
	Template    string                   `yaml:"template"`
	TableStyles map[string]TableStyleDef `yaml:"table_styles,omitempty"`
	CellStyles  map[string]CellStyleDef  `yaml:"cell_styles,omitempty"`
}

// settingsFilename returns the .settings.yaml path for a template name.
func settingsFilename(templatesDir, templateName string) string {
	return filepath.Join(templatesDir, templateName+".settings.yaml")
}

// validName matches a simple identifier: alphanumeric, hyphens, underscores.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidateName checks that a setting name is a valid identifier.
func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid setting name %q: must match [a-zA-Z0-9][a-zA-Z0-9_-]{0,63}", name)
	}
	return nil
}

// ValidateKind checks that a kind string is recognized.
func ValidateKind(k string) (Kind, error) {
	switch Kind(k) {
	case KindTableStyle:
		return KindTableStyle, nil
	case KindCellStyle:
		return KindCellStyle, nil
	default:
		return "", fmt.Errorf("unknown setting kind %q: must be %q or %q", k, KindTableStyle, KindCellStyle)
	}
}

// Load reads and parses the settings file for a template.
// Returns an empty File (with Template set) if the file does not exist.
func Load(templatesDir, templateName string) (*File, error) {
	path := settingsFilename(templatesDir, templateName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{
				Template:    templateName,
				TableStyles: make(map[string]TableStyleDef),
				CellStyles:  make(map[string]CellStyleDef),
			}, nil
		}
		return nil, fmt.Errorf("read settings file: %w", err)
	}

	var f File
	if err := safeyaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse settings file: %w", err)
	}
	if f.TableStyles == nil {
		f.TableStyles = make(map[string]TableStyleDef)
	}
	if f.CellStyles == nil {
		f.CellStyles = make(map[string]CellStyleDef)
	}
	f.Template = templateName
	return &f, nil
}

// Save atomically writes the settings file using temp-file + rename.
func Save(templatesDir string, f *File) (string, error) {
	path := settingsFilename(templatesDir, f.Template)

	data, err := yaml.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}

	// Atomic write: temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(templatesDir, ".settings-*.yaml.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp settings file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp settings file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp settings file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename temp settings file: %w", err)
	}

	return path, nil
}

// Delete removes a single named setting from the file on disk.
// Returns true if the setting existed and was removed.
func Delete(templatesDir, templateName string, kind Kind, name string) (bool, error) {
	f, err := Load(templatesDir, templateName)
	if err != nil {
		return false, err
	}

	var existed bool
	switch kind {
	case KindTableStyle:
		if _, ok := f.TableStyles[name]; ok {
			delete(f.TableStyles, name)
			existed = true
		}
	case KindCellStyle:
		if _, ok := f.CellStyles[name]; ok {
			delete(f.CellStyles, name)
			existed = true
		}
	}

	if !existed {
		return false, nil
	}

	// If file is now empty (no styles at all), remove it entirely.
	if len(f.TableStyles) == 0 && len(f.CellStyles) == 0 {
		path := settingsFilename(templatesDir, templateName)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return true, fmt.Errorf("remove empty settings file: %w", err)
		}
		return true, nil
	}

	if _, err := Save(templatesDir, f); err != nil {
		return true, err
	}
	return true, nil
}

// ValidateTemplateName checks that the template name is safe (no path traversal).
func ValidateTemplateName(name string) error {
	if name == "" {
		return fmt.Errorf("template name is required")
	}
	// Reject path separators and traversal.
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("template name %q contains path separator or traversal", name)
	}
	return nil
}

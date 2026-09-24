//go:build ignore

// Run with: go run scripts/backfill_template_metadata.go
// This embeds palette intent in the five shipped designer templates that
// predate the generator's metadata support. It never touches local templates.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const metadataPath = "ppt/go-slide-creator-metadata.json"

type palette struct {
	Version         string            `json:"version"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	SemanticAccents map[string]string `json:"semantic_accents"`
	SurfaceTints    map[string]string `json:"surface_tints"`
	DataPalette     []string          `json:"data_palette"`
}

var designerPalettes = map[string]palette{
	"abstract": {
		Name: "Abstract", Description: "Warm neutral abstract layout with orange and green highlights",
		SemanticAccents: map[string]string{"positive": "accent6", "negative": "accent2", "neutral": "accent3"},
		DataPalette:     []string{"accent1", "accent5", "accent2", "accent6", "accent4", "accent3"},
	},
	"blue-corporate": {
		Name: "Blue Corporate", Description: "Bright corporate layout with blue, green, and yellow accents",
		SemanticAccents: map[string]string{"positive": "accent1", "negative": "accent4", "neutral": "accent6"},
		DataPalette:     []string{"accent6", "accent1", "accent3", "accent4", "accent5", "accent2"},
	},
	"business-template": {
		Name: "Business Template", Description: "Muted purple and teal business layout",
		SemanticAccents: map[string]string{"positive": "accent5", "negative": "accent1", "neutral": "accent6"},
		DataPalette:     []string{"accent1", "accent3", "accent5", "accent2", "accent4", "accent6"},
	},
	"modern-yellow": {
		Name: "Modern Yellow", Description: "Blue and yellow modern layout",
		SemanticAccents: map[string]string{"positive": "accent6", "negative": "accent2", "neutral": "accent3"},
		DataPalette:     []string{"accent1", "accent4", "accent5", "accent2", "accent6", "accent3"},
	},
	"modern": {
		Name: "Modern", Description: "Modern layout with red, amber, and navy accents",
		SemanticAccents: map[string]string{"positive": "accent2", "negative": "accent1", "neutral": "accent6"},
		DataPalette:     []string{"accent1", "accent2", "accent4", "accent6", "accent3", "accent5"},
	},
}

func main() {
	for _, name := range []string{"abstract", "blue-corporate", "business-template", "modern-yellow", "modern"} {
		p := designerPalettes[name]
		p.Version = "1.0"
		p.SurfaceTints = map[string]string{"subtle": "lt2", "paper": "lt1", "elevated": "lt2", "inverse": "dk2"}
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			panic(err)
		}
		if err := embed(filepath.Join("templates", name+".pptx"), append(data, '\n')); err != nil {
			panic(fmt.Errorf("%s: %w", name, err))
		}
	}
}

func embed(path string, metadata []byte) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	var metadataCurrent, contentTypeCurrent bool
	for _, f := range reader.File {
		if f.Name != metadataPath && f.Name != "[Content_Types].xml" {
			continue
		}
		content, err := readZipFile(f)
		if err != nil {
			return err
		}
		if f.Name == metadataPath {
			metadataCurrent = bytes.Equal(content, metadata)
		} else {
			contentTypeCurrent = bytes.Contains(content, []byte(`Extension="json"`))
		}
	}
	if metadataCurrent && contentTypeCurrent {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".metadata-*.pptx")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	writer := zip.NewWriter(tmp)
	var sawTypes bool
	for _, f := range reader.File {
		if f.Name == metadataPath {
			continue
		}
		if f.Name != "[Content_Types].xml" {
			if err := writer.Copy(f); err != nil {
				return err
			}
			continue
		}
		sawTypes = true
		content, err := readZipFile(f)
		if err != nil {
			return err
		}
		if !bytes.Contains(content, []byte(`Extension="json"`)) {
			closing := []byte("</Types>")
			if !bytes.Contains(content, closing) {
				return fmt.Errorf("%s: missing content-types closing tag", path)
			}
			content = bytes.Replace(content, closing, []byte(`<Default Extension="json" ContentType="application/json"/>`+string(closing)), 1)
		}
		entry, err := writer.CreateHeader(&f.FileHeader)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
			return err
		}
	}
	if !sawTypes {
		return fmt.Errorf("%s: missing [Content_Types].xml", path)
	}
	entry, err := writer.Create(metadataPath)
	if err != nil {
		return err
	}
	if _, err := entry.Write(metadata); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode()); err != nil {
		return err
	}
	if err := reader.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

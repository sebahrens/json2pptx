//go:build ignore

// Run with: go run scripts/center_closing_layouts.go
// Makes each Closing layout's title/subtitle geometry match its own Title Slide.
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	for _, name := range []string{"midnight-blue", "forest-green"} {
		if err := centerClosing(filepath.Join("templates", name+".pptx")); err != nil {
			panic(fmt.Errorf("%s: %w", name, err))
		}
	}
}

func centerClosing(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	parts := map[string][]byte{}
	for _, file := range reader.File {
		if file.Name != "ppt/slideLayouts/slideLayout1.xml" && file.Name != "ppt/slideLayouts/slideLayout5.xml" {
			continue
		}
		entry, err := file.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(entry)
		closeErr := entry.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		parts[file.Name] = data
	}
	const titlePart = "ppt/slideLayouts/slideLayout1.xml"
	const closingPart = "ppt/slideLayouts/slideLayout5.xml"
	if len(parts) != 2 {
		return fmt.Errorf("expected title and closing layout parts, got %d", len(parts))
	}
	closing := parts[closingPart]
	for _, name := range []string{"title", "subtitle"} {
		bounds, err := shapeTransform(parts[titlePart], name)
		if err != nil {
			return fmt.Errorf("title slide %s: %w", name, err)
		}
		old, err := shapeTransform(closing, name)
		if err != nil {
			return fmt.Errorf("closing %s: %w", name, err)
		}
		if bytes.Equal(old, bounds) {
			continue
		}
		if bytes.Count(closing, old) != 1 {
			return fmt.Errorf("closing %s transform is not unique", name)
		}
		closing = bytes.Replace(closing, old, bounds, 1)
	}
	if bytes.Equal(closing, parts[closingPart]) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".closing-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	writer := zip.NewWriter(tmp)
	for _, file := range reader.File {
		if file.Name != closingPart {
			if err := writer.Copy(file); err != nil {
				return err
			}
			continue
		}
		entry, err := writer.CreateHeader(&file.FileHeader)
		if err != nil {
			return err
		}
		if _, err := entry.Write(closing); err != nil {
			return err
		}
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

func shapeTransform(layout []byte, name string) ([]byte, error) {
	marker := []byte(`name="` + name + `"`)
	pos := bytes.Index(layout, marker)
	if pos < 0 || bytes.Index(layout[pos+len(marker):], marker) >= 0 {
		return nil, fmt.Errorf("shape %q missing or duplicated", name)
	}
	start := bytes.LastIndex(layout[:pos], []byte("<p:sp>"))
	end := bytes.Index(layout[pos:], []byte("</p:sp>"))
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("shape %q has no p:sp", name)
	}
	shape := layout[start : pos+end]
	xfrmStart := bytes.Index(shape, []byte("<a:xfrm>"))
	xfrmEnd := bytes.Index(shape, []byte("</a:xfrm>"))
	if xfrmStart < 0 || xfrmEnd < 0 || xfrmStart >= xfrmEnd {
		return nil, fmt.Errorf("shape %q has no transform", name)
	}
	return shape[xfrmStart : xfrmEnd+len("</a:xfrm>")], nil
}

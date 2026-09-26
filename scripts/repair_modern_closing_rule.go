//go:build ignore

// Run with: go run scripts/repair_modern_closing_rule.go
// Repairs only the Closing layout's decorative rule, retaining all other ZIP
// entries byte-for-byte. The preimage is retained under output/ for comparison.
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
	if err := repairClosingRule("templates/modern-template.pptx"); err != nil {
		panic(err)
	}
}

func repairClosingRule(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	const part = "ppt/slideLayouts/slideLayout5.xml"
	const oldGeometry = `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="1700000"/>`
	const newGeometry = `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="320000"/>`
	var body []byte
	for _, entry := range reader.File {
		if entry.Name != part {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		body, err = io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
	}
	if bytes.Count(body, []byte(newGeometry)) == 1 && !bytes.Contains(body, []byte(oldGeometry)) {
		return nil // Already repaired, without rewriting the archive.
	}
	if bytes.Count(body, []byte(oldGeometry)) != 1 ||
		!bytes.Contains(body, []byte(`name="Straight Connector 10"`)) ||
		!bytes.Contains(body, []byte(`<a:off x="4830857" y="3800000"/>`)) {
		return fmt.Errorf("Closing geometry differs from reviewed source; refusing to patch")
	}
	body = bytes.Replace(body, []byte(oldGeometry), []byte(newGeometry), 1)
	backupDir := "output/template-repair-20260926"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	backupPath := filepath.Join(backupDir, "modern-template-before.pptx")
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("preserve template preimage: %w", err)
	}
	_, writeErr := backup.Write(source)
	closeErr := backup.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".modern-closing-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	w := zip.NewWriter(tmp)
	for _, entry := range reader.File {
		if entry.Name != part {
			if err := w.Copy(entry); err != nil {
				return err
			}
			continue
		}
		dst, err := w.CreateHeader(&entry.FileHeader)
		if err != nil {
			return err
		}
		if _, err := dst.Write(body); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
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

//go:build ignore

// Run with: go run scripts/repair_reviewed_templates.go
// Applies reviewed, narrowly scoped layout repairs. Unchanged ZIP entries and
// source preimages are preserved; unexpected sources fail before any mutation.
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
	for _, repair := range []func() error{
		func() error { return repairClosingRule("templates/modern-template.pptx") },
		func() error { return repairBusinessSubtitle("templates/business-template.pptx") },
		func() error { return repairModernSubtitle("templates/modern.pptx") },
	} {
		if err := repair(); err != nil {
			panic(err)
		}
	}
}

type reviewedPartRepair struct {
	old, replacement string
	guards           []string
}

func repairClosingRule(path string) error {
	return repairReviewedParts(path, "modern-template-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout5.xml": {
			old:         `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="1700000"/>`,
			replacement: `<a:off x="4985657" y="3300000"/><a:ext cx="0" cy="320000"/>`,
			guards:      []string{`name="Straight Connector 10"`, `<a:off x="4830857" y="3800000"/>`},
		},
	})
}

func repairBusinessSubtitle(path string) error {
	return repairReviewedParts(path, "business-template-before.pptx", map[string]reviewedPartRepair{
		"ppt/slideLayouts/slideLayout3.xml": {
			old: `<a:schemeClr val="accent1"/>`, replacement: `<a:schemeClr val="dk2"/>`,
			guards: []string{`name="subtitle"`, `<p:ph type="subTitle" idx="1"/>`, `<a:defRPr sz="1600" b="0" i="0">`},
		},
	})
}

func repairModernSubtitle(path string) error {
	parts := map[string]reviewedPartRepair{}
	for _, id := range []string{"2", "4"} {
		parts["ppt/slideLayouts/slideLayout"+id+".xml"] = reviewedPartRepair{
			old:         `<a:lumMod val="97000"/><a:lumOff val="3000"/>`,
			replacement: `<a:lumMod val="50000"/><a:lumOff val="0"/>`,
			guards:      []string{`name="subtitle"`, `<a:schemeClr val="accent2">`, `<a:gradFill`},
		}
	}
	return repairReviewedParts(path, "modern-before.pptx", parts)
}

func repairReviewedParts(path, backupName string, repairs map[string]reviewedPartRepair) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	updates := map[string][]byte{}
	seen := map[string]bool{}
	for _, entry := range reader.File {
		repair, needed := repairs[entry.Name]
		if !needed {
			continue
		}
		if seen[entry.Name] {
			return fmt.Errorf("duplicate reviewed part %s", entry.Name)
		}
		seen[entry.Name] = true
		r, err := entry.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return err
		}
		if bytes.Count(body, []byte(repair.replacement)) == 1 && !bytes.Contains(body, []byte(repair.old)) {
			continue // Already repaired, without rewriting this part.
		}
		if bytes.Count(body, []byte(repair.old)) != 1 {
			return fmt.Errorf("%s differs from reviewed source; refusing to patch", entry.Name)
		}
		for _, guard := range repair.guards {
			if !bytes.Contains(body, []byte(guard)) {
				return fmt.Errorf("%s missing reviewed marker %q", entry.Name, guard)
			}
		}
		updates[entry.Name] = bytes.Replace(body, []byte(repair.old), []byte(repair.replacement), 1)
	}
	for part := range repairs {
		if !seen[part] {
			return fmt.Errorf("reviewed part %s missing", part)
		}
	}
	if len(updates) == 0 {
		return nil
	}
	backupDir := "output/template-repair-20260926"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	backupPath := filepath.Join(backupDir, backupName)
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reviewed-template-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	w := zip.NewWriter(tmp)
	for _, entry := range reader.File {
		body, changed := updates[entry.Name]
		if !changed {
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

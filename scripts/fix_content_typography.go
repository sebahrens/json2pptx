//go:build ignore

// Run with: go run scripts/fix_content_typography.go
// Reapplies the reviewed content-layout typography fixes to the two designer
// templates. Unlisted PPTX files, including local p-style, are untouched.
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type replacement struct {
	old, new, alternateOld string
	count                  int
}

func replace(old, new string, count int) replacement {
	return replacement{old: old, new: new, count: count}
}

func replaceEither(old, alternateOld, new string, count int) replacement {
	return replacement{old: old, alternateOld: alternateOld, new: new, count: count}
}

type shapePatch struct {
	name    string
	changes []replacement
}

var typographyPatches = map[string]map[string][]shapePatch{
	"templates/blue-corporate.pptx": {
		"ppt/slideLayouts/slideLayout2.xml": {
			{"title", []replacement{
				replace(`<a:ext cx="10479024" cy="557784"/>`, `<a:ext cx="10479024" cy="914400"/>`, 1),
				replace(`<a:defRPr sz="2000" cap="all" spc="300" baseline="0"/>`, `<a:defRPr sz="2800" cap="all" spc="300" baseline="0"/>`, 1),
			}},
			{"body", []replacement{
				replace(`<a:off x="841248" y="1536827"/>`, `<a:off x="841248" y="1850000"/>`, 1),
				replace(`<a:ext cx="10479024" cy="4479925"/>`, `<a:ext cx="10479024" cy="4166752"/>`, 1),
				replace(`<a:defRPr sz="2000"/>`, `<a:defRPr sz="1400"/>`, 1),
				replace(`<a:defRPr sz="2400"/>`, `<a:defRPr sz="1600"/>`, 2),
				replace(`<a:defRPr sz="2800"/>`, `<a:defRPr sz="2000"/>`, 2),
			}},
		},
		"ppt/slideLayouts/slideLayout5.xml": {
			{"title", []replacement{
				replace(`<a:ext cx="10479024" cy="557784"/>`, `<a:ext cx="10479024" cy="914400"/>`, 1),
				replace(`<a:defRPr sz="2000" cap="all" spc="300" baseline="0"/>`, `<a:defRPr sz="2800" cap="all" spc="300" baseline="0"/>`, 1),
			}},
			{"body", []replacement{
				replace(`<a:off x="841248" y="1536827"/>`, `<a:off x="841248" y="1850000"/>`, 1),
				replace(`<a:ext cx="5057775" cy="4479925"/>`, `<a:ext cx="5057775" cy="4166752"/>`, 1),
			}},
			{"body_2", []replacement{
				replace(`<a:off x="6262497" y="1536827"/>`, `<a:off x="6262497" y="1850000"/>`, 1),
				replace(`<a:ext cx="5057775" cy="4479925"/>`, `<a:ext cx="5057775" cy="4166752"/>`, 1),
			}},
		},
	},
	"templates/business-template.pptx": {
		"ppt/slideLayouts/slideLayout3.xml": {
			{"body", []replacement{
				replaceEither(`<a:lstStyle/>`, `<a:lstStyle><a:lvl1pPr><a:defRPr sz="2200"/></a:lvl1pPr></a:lstStyle>`, `<a:lstStyle><a:lvl1pPr><a:defRPr sz="2200"/></a:lvl1pPr><a:lvl2pPr><a:defRPr sz="2000"/></a:lvl2pPr></a:lstStyle>`, 1),
			}},
		},
	},
}

func main() {
	for _, path := range []string{"templates/blue-corporate.pptx", "templates/business-template.pptx"} {
		if err := rewriteArchive(path, typographyPatches[path]); err != nil {
			panic(fmt.Errorf("%s: %w", path, err))
		}
	}
}

func rewriteArchive(path string, patches map[string][]shapePatch) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	modified := make(map[string][]byte, len(patches))
	var changed bool
	for _, f := range reader.File {
		shapes, ok := patches[f.Name]
		if !ok {
			continue
		}
		original, err := readEntry(f)
		if err != nil {
			return err
		}
		content := original
		for _, shape := range shapes {
			content, err = rewriteShape(content, shape)
			if err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
		}
		if !bytes.Equal(content, original) {
			changed = true
		}
		modified[f.Name] = content
	}
	if len(modified) != len(patches) {
		return fmt.Errorf("found %d/%d requested layout parts", len(modified), len(patches))
	}
	if !changed {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".typography-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	writer := zip.NewWriter(tmp)
	for _, f := range reader.File {
		content, patched := modified[f.Name]
		if !patched {
			if err := writer.Copy(f); err != nil {
				return err
			}
			continue
		}
		entry, err := writer.CreateHeader(&f.FileHeader)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
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

func rewriteShape(content []byte, patch shapePatch) ([]byte, error) {
	marker := []byte(`name="` + patch.name + `"`)
	pos := bytes.Index(content, marker)
	if pos < 0 || bytes.Index(content[pos+len(marker):], marker) >= 0 {
		return nil, fmt.Errorf("shape %q missing or duplicated", patch.name)
	}
	start := bytes.LastIndex(content[:pos], []byte("<p:sp>"))
	endRel := bytes.Index(content[pos:], []byte("</p:sp>"))
	if start < 0 || endRel < 0 {
		return nil, fmt.Errorf("shape %q has no enclosing p:sp", patch.name)
	}
	end := pos + endRel + len("</p:sp>")
	shape := string(content[start:end])
	for _, change := range patch.changes {
		if n := strings.Count(shape, change.old); n == change.count {
			shape = strings.ReplaceAll(shape, change.old, change.new)
		} else if change.alternateOld != "" && strings.Count(shape, change.alternateOld) == change.count {
			shape = strings.ReplaceAll(shape, change.alternateOld, change.new)
		} else if strings.Count(shape, change.new) != change.count {
			return nil, fmt.Errorf("shape %q: expected %d old values, found %d", patch.name, change.count, n)
		}
	}
	return bytes.Join([][]byte{content[:start], []byte(shape), content[end:]}, nil), nil
}

func readEntry(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

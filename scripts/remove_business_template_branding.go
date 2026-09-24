//go:build ignore

// Run with: go run scripts/remove_business_template_branding.go
// Removes sample company branding from the shipped business template.
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

const templatePath = "templates/business-template.pptx"

func main() {
	if err := removeBranding(templatePath); err != nil {
		panic(err)
	}
}

func removeBranding(filePath string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	const layout1 = "ppt/slideLayouts/slideLayout1.xml"
	const layout3 = "ppt/slideLayouts/slideLayout3.xml"
	const layout1Rels = "ppt/slideLayouts/_rels/slideLayout1.xml.rels"
	const media = "ppt/media/image1.emf"
	const contentTypes = "[Content_Types].xml"
	needed := map[string]bool{layout1: false, layout3: false, layout1Rels: false, media: false, contentTypes: false}
	parts := make(map[string][]byte)
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, ".rels") && file.Name != layout1Rels {
			data, err := readEntry(file)
			if err != nil {
				return err
			}
			if bytes.Contains(data, []byte("image1.emf")) {
				return fmt.Errorf("%s also references image1.emf; refusing to remove shared logo", file.Name)
			}
		}
		if _, ok := needed[file.Name]; !ok {
			continue
		}
		needed[file.Name] = true
		if file.Name != media {
			parts[file.Name], err = readEntry(file)
			if err != nil {
				return err
			}
		}
	}
	for name, found := range needed {
		if !found {
			return fmt.Errorf("expected archive entry %s missing", name)
		}
	}
	if !bytes.Contains(parts[layout1], []byte("My Consulting Company")) ||
		!bytes.Contains(parts[layout3], []byte("My Consulting Company")) {
		return fmt.Errorf("sample branding text missing; template may already be repaired")
	}
	if parts[layout1], err = removeShape(parts[layout1], "p:sp", "Company Branding", "My Consulting Company"); err != nil {
		return err
	}
	if parts[layout1], err = removeShape(parts[layout1], "p:pic", "Picture 11", `r:embed="rId2"`); err != nil {
		return err
	}
	if parts[layout3], err = removeShape(parts[layout3], "p:sp", "TextBox 23", "My Consulting Company"); err != nil {
		return err
	}
	const imageRel = `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.emf"/>`
	if bytes.Count(parts[layout1Rels], []byte(imageRel)) != 1 {
		return fmt.Errorf("expected unique image relationship in %s", layout1Rels)
	}
	parts[layout1Rels] = bytes.Replace(parts[layout1Rels], []byte(imageRel), nil, 1)
	const emfType = `<Default Extension="emf" ContentType="image/x-emf"/>`
	if bytes.Count(parts[contentTypes], []byte(emfType)) != 1 {
		return fmt.Errorf("expected unique EMF content type")
	}
	for _, file := range reader.File {
		if file.Name != media && strings.HasSuffix(strings.ToLower(file.Name), ".emf") {
			return fmt.Errorf("another EMF asset %s needs the content type", file.Name)
		}
	}
	parts[contentTypes] = bytes.Replace(parts[contentTypes], []byte(emfType), nil, 1)

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".business-template-*.pptx")
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	writer := zip.NewWriter(tmp)
	for _, file := range reader.File {
		if file.Name == media {
			continue
		}
		data, changed := parts[file.Name]
		if !changed {
			if err := writer.Copy(file); err != nil {
				return err
			}
			continue
		}
		entry, err := writer.CreateHeader(&file.FileHeader)
		if err != nil {
			return err
		}
		if _, err := entry.Write(data); err != nil {
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
	return os.Rename(tmp.Name(), filePath)
}

func readEntry(file *zip.File) ([]byte, error) {
	entry, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer entry.Close()
	return io.ReadAll(entry)
}

func removeShape(layout []byte, tag, name, required string) ([]byte, error) {
	marker := []byte(`name="` + name + `"`)
	if bytes.Count(layout, marker) != 1 {
		return nil, fmt.Errorf("expected unique %q shape", name)
	}
	position := bytes.Index(layout, marker)
	start := bytes.LastIndex(layout[:position], []byte("<"+tag+">"))
	end := bytes.Index(layout[position:], []byte("</"+tag+">"))
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("cannot delimit %q shape", name)
	}
	end += position + len("</"+tag+">")
	if !bytes.Contains(layout[start:end], []byte(required)) {
		return nil, fmt.Errorf("%q shape does not contain %q", name, required)
	}
	return append(append([]byte(nil), layout[:start]...), layout[end:]...), nil
}

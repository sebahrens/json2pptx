package template

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

const presentationNamespace = "http://schemas.openxmlformats.org/presentationml/2006/main"

// checkLayoutStaticText warns about ordinary layout shapes whose text would be
// copied into every slide using that layout. Placeholder sample text is fine.
func checkLayoutStaticText(reader *Reader) ([]ConformanceCheck, error) {
	files, err := reader.ListFiles("ppt/slideLayouts/slideLayout*.xml")
	if err != nil {
		return nil, err
	}
	var checks []ConformanceCheck
	for _, file := range files {
		data, err := reader.ReadFile(file)
		if err != nil {
			return nil, err
		}
		shapes, err := staticTextShapes(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		for _, shape := range shapes {
			checks = append(checks, ConformanceCheck{
				Category: "content",
				Check:    "No fixed text in layout shapes",
				Status:   ConformanceStatusWarn,
				Detail:   fmt.Sprintf("%s shape %q contains non-placeholder text %q", path.Base(file), shape.name, shape.text),
			})
		}
	}
	if len(checks) == 0 {
		checks = append(checks, ConformanceCheck{
			Category: "content", Check: "No fixed text in layout shapes", Status: ConformanceStatusPass,
			Detail: "No non-placeholder layout shapes contain text",
		})
	}
	return checks, nil
}

type staticTextShape struct {
	name string
	text string
}

func staticTextShapes(data []byte) ([]staticTextShape, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var result []staticTextShape
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Space != presentationNamespace || start.Name.Local != "sp" {
			continue
		}
		var shape struct {
			NvSpPr struct {
				CNvPr struct {
					Name string `xml:"name,attr"`
				} `xml:"cNvPr"`
				NvPr struct {
					Placeholder *struct{} `xml:"ph"`
				} `xml:"nvPr"`
			} `xml:"nvSpPr"`
			TextBody struct {
				Paragraphs []struct {
					Runs []struct {
						Text string `xml:"t"`
					} `xml:"r"`
					Fields []struct {
						Text string `xml:"t"`
					} `xml:"fld"`
				} `xml:"p"`
			} `xml:"txBody"`
		}
		if err := decoder.DecodeElement(&shape, &start); err != nil {
			return nil, err
		}
		if shape.NvSpPr.NvPr.Placeholder != nil {
			continue
		}
		var parts []string
		for _, paragraph := range shape.TextBody.Paragraphs {
			for _, run := range paragraph.Runs {
				if text := strings.TrimSpace(run.Text); text != "" {
					parts = append(parts, text)
				}
			}
			for _, field := range paragraph.Fields {
				if text := strings.TrimSpace(field.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			result = append(result, staticTextShape{name: shape.NvSpPr.CNvPr.Name, text: strings.Join(parts, " ")})
		}
	}
}

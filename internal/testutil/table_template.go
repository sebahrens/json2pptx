package testutil

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const BrandTableStyleID = "{B56CA0EE-1E45-4AA6-A82B-0F5650326712}"

// TableStyleTemplate builds a portable fixture with a default that differs
// from the engine's built-in style. No private template is needed in CI.
func TableStyleTemplate(t testing.TB, defined bool) string {
	t.Helper()
	source, err := zip.OpenReader(filepath.Join(TemplatesDir(), "modern-template.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	output := filepath.Join(t.TempDir(), "brand-table.pptx")
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for _, entry := range source.File {
		if entry.Name != "ppt/tableStyles.xml" {
			if err := writer.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		target, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "ppt/tableStyles.xml" {
			style := `<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="` + BrandTableStyleID + `"><a:tblStyle styleId="` + BrandTableStyleID + `" styleName="Brand Table 3">`
			if defined {
				style += `<a:wholeTbl><a:tcTxStyle><a:fontRef idx="minor"><a:schemeClr val="tx1"/></a:fontRef><a:schemeClr val="tx1"/></a:tcTxStyle><a:tcStyle><a:fill><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:fill></a:tcStyle></a:wholeTbl><a:band1H><a:tcStyle><a:fill><a:solidFill><a:schemeClr val="accent2"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr></a:solidFill></a:fill></a:tcStyle></a:band1H><a:firstRow><a:tcTxStyle b="on"><a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef><a:schemeClr val="lt1"/></a:tcTxStyle><a:tcStyle><a:fill><a:solidFill><a:schemeClr val="accent2"/></a:solidFill></a:fill></a:tcStyle></a:firstRow>`
			}
			if _, err := io.WriteString(target, style+`</a:tblStyle></a:tblStyleLst>`); err != nil {
				t.Fatal(err)
			}
			continue
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output
}

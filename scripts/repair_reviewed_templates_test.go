//go:build ignore

// Run with: go test scripts/repair_reviewed_templates.go scripts/repair_reviewed_templates_test.go
package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairModernSection(t *testing.T) {
	const original = `<p:cSld><p:sp name="title"><a:off x="1450428" y="990601"/><a:ext cx="9145991" cy="3630384"/><a:bodyPr anchor="b"/><a:defRPr sz="6500"/></p:sp><p:sp name="Section Number"><a:off x="7886700" y="572408"/><a:ext cx="3182938" cy="3417887"/><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr marL="11113" indent="-11113"><a:defRPr sz="9600"/></a:lvl1pPr></a:lstStyle></p:txBody></p:sp></p:cSld>`
	// Match the reviewed opening tags, not simplified self-closing font tags.
	source := strings.ReplaceAll(strings.ReplaceAll(original, `sz="6500"/>`, `sz="6500"></a:defRPr>`), `sz="9600"/>`, `sz="9600"></a:defRPr>`)
	source = strings.Replace(source, `<a:bodyPr anchor="b"/>`, `<a:bodyPr anchor="b"><a:noAutofit/></a:bodyPr>`, 1)
	for _, tc := range []struct {
		name, body                  string
		duplicate, missing, wantErr bool
	}{
		{name: "original", body: source},
		{name: "partial-repair", body: strings.Replace(source, `cy="3417887"`, `cy="1600000"`, 1), wantErr: true},
		{name: "unexpected-font", body: strings.Replace(source, `sz="9600"`, `sz="9000"`, 1), wantErr: true},
		{name: "unexpected-title-position", body: strings.Replace(source, `y="990601"`, `y="990602"`, 1), wantErr: true},
		{name: "duplicate-layout", body: source, duplicate: true, wantErr: true},
		{name: "missing-layout", missing: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			parts := []struct{ name, body string }{{"ppt/media/artwork.svg", "original artwork"}, {"ppt/slideLayouts/slideLayout3.xml", "unrelated layout"}}
			if !tc.missing {
				parts = append(parts, struct{ name, body string }{"ppt/slideLayouts/slideLayout2.xml", tc.body})
			}
			if tc.duplicate {
				parts = append(parts, struct{ name, body string }{"ppt/slideLayouts/slideLayout2.xml", tc.body})
			}
			for _, part := range parts {
				f, err := w.Create(part.name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(f, part.body); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			before := append([]byte(nil), buf.Bytes()...)
			if err := os.WriteFile("source.pptx", before, 0600); err != nil {
				t.Fatal(err)
			}
			err := repairModernSection("source.pptx")
			if (err != nil) != tc.wantErr {
				t.Fatalf("repair=%v, wantErr=%v", err, tc.wantErr)
			}
			after, err := os.ReadFile("source.pptx")
			if err != nil {
				t.Fatal(err)
			}
			backup := filepath.Join("output", "template-repair-20260926", "modern-template-section-before.pptx")
			if tc.wantErr {
				if !bytes.Equal(before, after) {
					t.Fatal("failed repair modified source")
				}
				if _, err := os.Stat(backup); !os.IsNotExist(err) {
					t.Fatal("invalid source created backup")
				}
				return
			}
			preimage, err := os.ReadFile(backup)
			if err != nil || !bytes.Equal(before, preimage) {
				t.Fatal("original not preserved")
			}
			r, err := zip.OpenReader("source.pptx")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range r.File {
				f, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(f)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if entry.Name == "ppt/slideLayouts/slideLayout2.xml" {
					for _, required := range []string{`y="2352408"`, `cy="2268577"`, `cy="1600000"`, `anchor="t"`, `anchor="b"`, `sz="6500"`, `sz="9600"`} {
						if !bytes.Contains(body, []byte(required)) {
							t.Fatalf("missing repaired geometry/style %s", required)
						}
					}
				} else if entry.Name == "ppt/media/artwork.svg" && string(body) != "original artwork" {
					t.Fatal("artwork changed")
				} else if entry.Name == "ppt/slideLayouts/slideLayout3.xml" && string(body) != "unrelated layout" {
					t.Fatal("unrelated layout changed")
				}
			}
			_ = r.Close()
			if err := repairModernSection("source.pptx"); err != nil {
				t.Fatal("repeat repair:", err)
			}
			again, err := os.ReadFile("source.pptx")
			if err != nil || !bytes.Equal(after, again) {
				t.Fatal("idempotent repair rewrote source")
			}
		})
	}
}

func TestRepairClosingRule(t *testing.T) {
	const original = `<p:cSld><p:sp name="subtitle"><a:off x="4830857" y="3800000"/></p:sp><p:cxnSp name="Straight Connector 10"><a:off x="4985657" y="3300000"/><a:ext cx="0" cy="1700000"/></p:cxnSp></p:cSld>`
	for _, tc := range []struct {
		name         string
		body         string
		backupExists bool
		wantErr      bool
	}{
		{"reviewed-source", original, false, false},
		{"unexpected-rule", `<p:cSld/>`, false, true},
		{"preimage-already-exists", original, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			path := filepath.Join(dir, "modern.pptx")
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			for _, part := range []struct{ name, body string }{
				{"ppt/slideLayouts/slideLayout5.xml", tc.body},
				{"ppt/media/artwork.png", "unchanged original artwork"},
			} {
				e, err := w.Create(part.name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(e, part.body); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			before := append([]byte(nil), archive.Bytes()...)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			backup := filepath.Join(dir, "output", "template-repair-20260926", "modern-template-before.pptx")
			if tc.backupExists {
				if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(backup, []byte("retain previous preimage"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := repairClosingRule(path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("repair error=%v, wantErr=%v", err, tc.wantErr)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantErr {
				if !bytes.Equal(before, after) {
					t.Fatal("failed repair modified source")
				}
				return
			}
			preimage, err := os.ReadFile(backup)
			if err != nil || !bytes.Equal(before, preimage) {
				t.Fatal("original not preserved")
			}
			r, err := zip.OpenReader(path)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			for _, entry := range r.File {
				f, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(f)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if entry.Name == "ppt/media/artwork.png" && string(body) != "unchanged original artwork" {
					t.Fatal("artwork modified")
				}
				if entry.Name == "ppt/slideLayouts/slideLayout5.xml" && !bytes.Contains(body, []byte(`cy="320000"`)) {
					t.Fatal("rule not shortened")
				}
			}
			if err := repairClosingRule(path); err != nil {
				t.Fatal("idempotent repair failed:", err)
			}
			again, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, again) {
				t.Fatal("idempotent repair rewrote archive")
			}
		})
	}
}

func TestReviewedPartsFailBeforeAnyMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []string
		b       string
	}{
		{"later-part-mismatch", []string{"a.xml", "b.xml"}, "unexpected"},
		{"missing-part", []string{"a.xml"}, ""},
		{"duplicate-part", []string{"a.xml", "a.xml", "b.xml"}, "old marker"},
		{"missing-guard", []string{"a.xml", "b.xml"}, "old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			path := filepath.Join(dir, "source.pptx")
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			for _, name := range tc.entries {
				e, err := w.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				body := "old marker"
				if name == "b.xml" {
					body = tc.b
				}
				if _, err := io.WriteString(e, body); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			before := append([]byte(nil), archive.Bytes()...)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			repairs := map[string]reviewedPartRepair{
				"a.xml": {old: "old", replacement: "new", guards: []string{"marker"}},
				"b.xml": {old: "old", replacement: "new", guards: []string{"marker"}},
			}
			if err := repairReviewedParts(path, "preimage.pptx", repairs); err == nil {
				t.Fatal("unexpected source accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed multipart repair changed source")
			}
			if _, err := os.Stat(filepath.Join("output", "template-repair-20260926", "preimage.pptx")); !os.IsNotExist(err) {
				t.Fatal("failed validation created a preimage or partial repair")
			}
		})
	}
}

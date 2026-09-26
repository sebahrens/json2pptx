//go:build ignore

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRepairModernBulletIndentation(t *testing.T) {
	source, err := os.ReadFile("../templates/modern.pptx")
	if err != nil {
		t.Fatal(err)
	}
	readParts := func(data []byte) map[string][]byte {
		t.Helper()
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		parts := map[string][]byte{}
		for _, entry := range z.File {
			r, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			parts[entry.Name], err = io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		return parts
	}
	const first = "ppt/slideLayouts/slideLayout3.xml"
	const second = "ppt/slideLayouts/slideLayout7.xml"
	for _, kind := range []string{"original", "already-repaired", "one-repaired", "unexpected-style", "partial-levels", "duplicate-part", "missing-part"} {
		t.Run(kind, func(t *testing.T) {
			parts := readParts(source)
			for _, part := range []string{first, second} {
				body := string(parts[part])
				for level := 2; level <= 5; level++ {
					old := fmt.Sprintf(`<a:lvl%dpPr marL="228600" indent="-228600">`, level)
					updated := fmt.Sprintf(`<a:lvl%dpPr marL="%d" indent="-228600">`, level, level*228600)
					body = strings.ReplaceAll(body, updated, old)
					if kind == "already-repaired" || (kind == "one-repaired" && part == first) || (kind == "partial-levels" && level == 2 && part == first) {
						body = strings.ReplaceAll(body, old, updated)
					}
				}
				parts[part] = []byte(body)
			}
			if kind == "unexpected-style" {
				parts[second] = bytes.Replace(parts[second], []byte(`sz="1800"`), []byte(`sz="1801"`), 1)
			}
			if kind == "missing-part" {
				delete(parts, second)
			}
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			for name, data := range parts {
				f, err := w.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "duplicate-part" {
				f, err := w.Create(first)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write(parts[first]); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			t.Chdir(t.TempDir())
			before := buf.Bytes()
			if err := os.WriteFile("source.pptx", before, 0600); err != nil {
				t.Fatal(err)
			}
			err := repairModernBulletIndentation("source.pptx")
			wantErr := kind != "original" && kind != "already-repaired" && kind != "one-repaired"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
			after, err := os.ReadFile("source.pptx")
			if err != nil {
				t.Fatal(err)
			}
			if wantErr || kind == "already-repaired" {
				if !bytes.Equal(before, after) {
					t.Fatal("failed/idempotent repair changed template")
				}
				return
			}
			backup, err := os.ReadFile("output/template-repair-20260926/modern-bullet-indentation-before.pptx")
			if err != nil || !bytes.Equal(backup, before) {
				t.Fatal("preimage not preserved")
			}
			result := readParts(after)
			if len(result) != len(parts) {
				t.Fatal("ZIP inventory changed")
			}
			for name, data := range parts {
				if name != first && name != second && !bytes.Equal(data, result[name]) {
					t.Fatalf("unrelated part changed: %s", name)
				}
			}
			if err := repairModernBulletIndentation("source.pptx"); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile("source.pptx")
			if err != nil || !bytes.Equal(again, after) {
				t.Fatal("repeat repair changed bytes")
			}
		})
	}
}

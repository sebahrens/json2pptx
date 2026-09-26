//go:build ignore

package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRepairModernFooterAccent(t *testing.T) {
	const old = `<a:off x="5291586" y="6303963"/><a:ext cx="4287186" cy="554037"/>`
	const updated = `<a:off x="0" y="6781800"/><a:ext cx="12192000" cy="76200"/>`
	const style = `<p:cNvPr name="Rectangle 7"/><a:gradFill flip="none" rotWithShape="1"><a:schemeClr val="accent5"/><a:tileRect r="-100000" b="-100000"/></a:gradFill>`
	for _, kind := range []string{"original", "already-repaired", "wrong-gradient", "repaired-wrong-gradient", "unexpected-bounds", "duplicate-part", "missing-part"} {
		t.Run(kind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			body := style + old
			if kind == "already-repaired" || kind == "repaired-wrong-gradient" {
				body = style + updated
			}
			if kind == "wrong-gradient" || kind == "repaired-wrong-gradient" {
				body = strings.Replace(body, `val="accent5"`, `val="accent4"`, 1)
			}
			if kind == "unexpected-bounds" {
				body = strings.Replace(body, `cy="554037"`, `cy="554038"`, 1)
			}
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			parts := map[string]string{"ppt/media/art.png": "original media", "ppt/slideMasters/slideMaster1.xml": "unchanged master"}
			if kind != "missing-part" {
				parts["ppt/slideLayouts/slideLayout3.xml"] = body
			}
			for name, value := range parts {
				f, err := w.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(f, value); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "duplicate-part" {
				f, err := w.Create("ppt/slideLayouts/slideLayout3.xml")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(f, body); err != nil {
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
			err := repairModernFooterAccent("source.pptx")
			wantErr := kind != "original" && kind != "already-repaired"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
			after, err := os.ReadFile("source.pptx")
			if err != nil {
				t.Fatal(err)
			}
			if wantErr || kind == "already-repaired" {
				if !bytes.Equal(before, after) {
					t.Fatal("failed/idempotent repair changed source")
				}
				return
			}
			backup, err := os.ReadFile("output/template-repair-20260926/modern-footer-accent-before.pptx")
			if err != nil || !bytes.Equal(backup, before) {
				t.Fatal("source preimage not preserved")
			}
			r, err := zip.OpenReader("source.pptx")
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range r.File {
				f, err := part.Open()
				if err != nil {
					t.Fatal(err)
				}
				value, err := io.ReadAll(f)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
				want := parts[part.Name]
				if part.Name == "ppt/slideLayouts/slideLayout3.xml" {
					want = style + updated
				}
				if string(value) != want {
					t.Fatalf("unexpected package change: %s", part.Name)
				}
			}
			_ = r.Close()
			if err := repairModernFooterAccent("source.pptx"); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile("source.pptx")
			if err != nil || !bytes.Equal(again, after) {
				t.Fatal("repeat repair rewrote source")
			}
		})
	}
}

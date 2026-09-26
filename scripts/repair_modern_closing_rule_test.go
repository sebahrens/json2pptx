//go:build ignore

// Run with: go test scripts/repair_modern_closing_rule.go scripts/repair_modern_closing_rule_test.go
package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

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

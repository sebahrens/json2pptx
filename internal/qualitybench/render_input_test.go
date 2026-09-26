package qualitybench

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateFromFileKeepsOriginalPathAndOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-generator")
	input := filepath.Join(dir, "original.json")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s ' \"$@\"\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte(`{"template_path":"./original.pptx"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r := Renderer{JSON2PPTX: bin, TemplatesDir: "fixture-templates", TemplateName: "recorded-template"}
	err := r.GenerateFromFile(context.Background(), input, filepath.Join(dir, "new", "deck.pptx"))
	if err == nil || !strings.Contains(err.Error(), "-json "+input) || !strings.Contains(err.Error(), "-template recorded-template") {
		t.Fatalf("source context or recorded override lost: %v", err)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"template_path":"./original.pptx"}` {
		t.Fatal("frozen input was modified")
	}
}

func TestGenerateFromFileRoutesAuthoringFormats(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, tc := range []struct{ name, input, command string }{
		{"raw", `{"slides":[{"type":"title"}]}`, "generate -json"},
		{"semantic", `{"meta":{"template":"midnight-blue"},"slides":[{"kind":"title"}]}`, "semantic render -spec"},
		{"semanticWithoutMeta", `{"slides":[{"kind":"title"}]}`, "semantic render -spec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "generator")
			input := filepath.Join(dir, "authoring.json")
			if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s ' \"$@\"\nexit 1\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(input, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			r := Renderer{JSON2PPTX: bin}
			err := r.GenerateFromFile(context.Background(), input, filepath.Join(dir, "deck.pptx"))
			if err == nil || !strings.Contains(err.Error(), tc.command+" "+input) {
				t.Fatalf("wrong authoring route: %v", err)
			}
		})
	}
}

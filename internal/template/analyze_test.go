package template

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/types"
)

func analyzerFixture(t *testing.T, metadata string, brokenLayouts bool) string {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == MetadataFilePath {
			continue
		}
		data, readErr := func() ([]byte, error) {
			r, openErr := f.Open()
			if openErr != nil {
				return nil, openErr
			}
			defer r.Close()
			return io.ReadAll(r)
		}()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if brokenLayouts && strings.HasPrefix(f.Name, "ppt/slideLayouts/") && strings.HasSuffix(f.Name, ".xml") {
			data = []byte("<invalid")
		}
		w, createErr := zw.Create(f.Name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := w.Write(data); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	w, err := zw.Create(MetadataFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, metadata); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "template.pptx")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzeSharedMetadataAndCache(t *testing.T) {
	path := analyzerFixture(t, `{"version":"1.0","semantic_accents":{"positive":"accent4"},"surface_tints":{"callout":"accent3"}}`, false)
	cache := NewMemoryCache(time.Hour)
	a, err := Analyze(path, cache, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.SlideWidth <= 0 || a.SlideHeight <= 0 || a.AspectRatio != AspectRatio(a.SlideWidth, a.SlideHeight) || a.Hash == "" {
		t.Fatalf("missing measured template identity/geometry: %+v", a)
	}
	if a.Theme.SemanticAccents["positive"] != "accent4" || a.Theme.SurfaceTints["callout"] != "accent3" {
		t.Fatalf("metadata lost from theme: %+v", a.Theme)
	}
	b, err := Analyze(path, cache, false)
	if err != nil || a != b || !reflect.DeepEqual(AnalysisFindings(a), AnalysisFindings(b)) {
		t.Fatalf("cached analysis differs: err=%v", err)
	}
	// Applying a per-deck override cannot poison the next caller's cached theme.
	before := b.Theme.Colors[0]
	changed, _ := a.Theme.ApplyOverride(&types.ThemeOverride{Colors: map[string]string{before.Name: "123456"}})
	if changed.Colors[0].RGB != "123456" || b.Theme.Colors[0] != before {
		t.Fatal("per-deck override mutated cached theme")
	}
}

func TestAnalyzeInvalidatesReplacedTemplate(t *testing.T) {
	path := analyzerFixture(t, `{"version":"1.0"}`, false)
	cache := NewMemoryCache(time.Hour)
	before, err := Analyze(path, cache, false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := os.ReadFile(analyzerFixture(t, `{`, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	next := info.ModTime().Add(time.Second)
	if err := os.Chtimes(path, next, next); err != nil {
		t.Fatal(err)
	}
	after, err := Analyze(path, cache, false)
	if err != nil || after == before || after.Hash == before.Hash || len(after.Warnings) == 0 {
		t.Fatalf("replaced template reused stale metadata: err=%v", err)
	}
}

func TestAnalysisFindingsPreserveSynthesisAfterNormalization(t *testing.T) {
	path := analyzerFixture(t, `{"version":"1.0"}`, false)
	analysis, err := Analyze(path, NewMemoryCache(time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var base []types.LayoutMetadata
	for _, layout := range analysis.Layouts {
		if !containsCapability(layout.Tags, "two-column") && !containsCapability(layout.Tags, "blank-title") {
			base = append(base, layout)
		}
	}
	analysis.Layouts, analysis.Synthesis = base, nil
	want := SynthesizeIfNeeded(reader, analysis)
	if len(want) == 0 {
		t.Fatal("fixture did not require synthesis")
	}
	// Normalization contributes native replacement XML to the same manifest.
	analysis.Synthesis.SyntheticFiles["ppt/slideLayouts/native.xml"] = []byte("<native/>")
	if got := AnalysisFindings(analysis); !reflect.DeepEqual(got, want) {
		t.Fatalf("cached advice lost provenance or counted native replacements: %+v", got)
	}
}

func TestAnalyzeMetadataWarningsSurviveCache(t *testing.T) {
	path := analyzerFixture(t, `{"version":`, false)
	cache := NewMemoryCache(time.Hour)
	for range 2 {
		a, err := Analyze(path, cache, false)
		if err != nil {
			t.Fatal(err)
		}
		findings := AnalysisFindings(a)
		found := false
		for _, f := range findings {
			if f.Code == "TEMPLATE_METADATA_PARSE" && f.Path == "template.metadata" && strings.Contains(f.Message, "invalid JSON") {
				found = true
			}
		}
		if !found || a.Metadata != nil {
			t.Fatalf("parse warning lost: metadata=%v findings=%+v", a.Metadata, findings)
		}
	}
	if _, err := Analyze(path, cache, true); err == nil {
		t.Fatal("strict analysis accepted cached malformed metadata")
	}
}

func TestAnalyzeRejectsBrokenTemplate(t *testing.T) {
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.pptx"), analyzerFixture(t, `{"version":"1.0"}`, true)} {
		if a, err := Analyze(path, NewMemoryCache(time.Hour), false); err == nil || a != nil {
			t.Fatalf("broken template became an empty analysis: a=%+v err=%v", a, err)
		}
	}
}

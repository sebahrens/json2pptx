package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sha256OfBytes returns the hex sha256 of b, matching describeArtifact's hash.
func sha256OfBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestDescribeArtifact_HashesAndSizes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.svg")
	content := []byte("<svg>hello</svg>")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := describeArtifact(path, "svg")
	if err != nil {
		t.Fatalf("describeArtifact: %v", err)
	}
	if got.Path != path {
		t.Errorf("path = %q, want %q", got.Path, path)
	}
	if got.Kind != "svg" {
		t.Errorf("kind = %q, want svg", got.Kind)
	}
	if got.Bytes != int64(len(content)) {
		t.Errorf("bytes = %d, want %d", got.Bytes, len(content))
	}
	if got.SHA256 != sha256OfBytes(content) {
		t.Errorf("sha256 = %q, want %q", got.SHA256, sha256OfBytes(content))
	}
}

func TestDescribeArtifact_MissingFileErrors(t *testing.T) {
	_, err := describeArtifact(filepath.Join(t.TempDir(), "nope.png"), "png")
	if err == nil {
		t.Fatal("expected error for missing artifact, got nil")
	}
}

func TestBuildWriteManifest_MultipleArtifacts(t *testing.T) {
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "a.svg")
	pngPath := filepath.Join(dir, "a.png")
	svg := []byte("<svg/>")
	png := []byte("\x89PNG\r\n")
	if err := os.WriteFile(svgPath, svg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, png, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := buildWriteManifest("preview-icon", []artifactSpec{
		{path: svgPath, kind: "svg"},
		{path: pngPath, kind: "png"},
	}, []string{"a warning"})
	if err != nil {
		t.Fatalf("buildWriteManifest: %v", err)
	}
	if !m.Success {
		t.Error("expected success=true")
	}
	if m.Command != "preview-icon" {
		t.Errorf("command = %q, want preview-icon", m.Command)
	}
	if len(m.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(m.Artifacts))
	}
	if m.Artifacts[0].SHA256 != sha256OfBytes(svg) || m.Artifacts[1].SHA256 != sha256OfBytes(png) {
		t.Errorf("artifact hashes mismatch: %+v", m.Artifacts)
	}
	if len(m.Warnings) != 1 || m.Warnings[0] != "a warning" {
		t.Errorf("warnings = %v, want [a warning]", m.Warnings)
	}
}

func TestBuildWriteManifest_EmptyArtifactsEncodesAsArray(t *testing.T) {
	m, err := buildWriteManifest("preview-patterns", nil, nil)
	if err != nil {
		t.Fatalf("buildWriteManifest: %v", err)
	}
	var buf bytes.Buffer
	if err := printWriteManifest(&buf, m); err != nil {
		t.Fatalf("printWriteManifest: %v", err)
	}
	if !strings.Contains(buf.String(), `"artifacts": []`) {
		t.Errorf("expected empty artifacts to encode as [], got %s", buf.String())
	}
}

func TestBuildWriteManifest_PropagatesMissingFileError(t *testing.T) {
	_, err := buildWriteManifest("preview-wireframe", []artifactSpec{
		{path: filepath.Join(t.TempDir(), "missing.svg"), kind: "svg"},
	}, nil)
	if err == nil {
		t.Fatal("expected error when an artifact is missing, got nil")
	}
}

func TestPrintWriteManifest_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := buildWriteManifest("preview-wireframe", []artifactSpec{{path: path, kind: "png"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := printWriteManifest(&buf, m); err != nil {
		t.Fatalf("printWriteManifest: %v", err)
	}
	var parsed WriteManifest
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("manifest is not valid JSON: %v\n%s", err, buf.String())
	}
	if !parsed.Success || len(parsed.Artifacts) != 1 || parsed.Artifacts[0].Kind != "png" {
		t.Errorf("round-trip mismatch: %+v", parsed)
	}
}

// --- preview-wireframe emit ---

func TestEmitWireframeArtifact_ToStdoutWhenNoOut(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("<svg>raw</svg>")
	if err := emitWireframeArtifact(&buf, payload, "svg", "", false); err != nil {
		t.Fatalf("emitWireframeArtifact: %v", err)
	}
	if buf.String() != string(payload) {
		t.Errorf("stdout = %q, want raw payload", buf.String())
	}
}

func TestEmitWireframeArtifact_WritesFileSilentlyWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "slide0.png")
	payload := []byte("PNGBYTES")
	var buf bytes.Buffer
	if err := emitWireframeArtifact(&buf, payload, "png", out, false); err != nil {
		t.Fatalf("emitWireframeArtifact: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected silent stdout without --manifest, got %q", buf.String())
	}
	got, err := os.ReadFile(out)
	if err != nil || !bytes.Equal(got, payload) {
		t.Errorf("file content = %q (err %v), want %q", got, err, payload)
	}
}

func TestEmitWireframeArtifact_EmitsManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "slide0.svg")
	payload := []byte("<svg>wire</svg>")
	var buf bytes.Buffer
	if err := emitWireframeArtifact(&buf, payload, "svg", out, true); err != nil {
		t.Fatalf("emitWireframeArtifact: %v", err)
	}
	var m WriteManifest
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("manifest parse: %v\n%s", err, buf.String())
	}
	if m.Command != "preview-wireframe" || len(m.Artifacts) != 1 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	a := m.Artifacts[0]
	if a.Path != out || a.Kind != "svg" || a.SHA256 != sha256OfBytes(payload) || a.Bytes != int64(len(payload)) {
		t.Errorf("artifact mismatch: %+v", a)
	}
}

// --- preview-icon emit ---

func TestEmitPreviewIcon_ManifestListsWrittenFiles(t *testing.T) {
	dir := t.TempDir()
	outSVG := filepath.Join(dir, "icon.svg")
	outPNG := filepath.Join(dir, "icon.png")
	pngRaw := []byte("\x89PNG\r\nrender")
	resp := &previewIconResponse{
		SVGData:   "<svg>icon</svg>",
		PNGBase64: base64.StdEncoding.EncodeToString(pngRaw),
		Warnings:  []string{"fill ignored"},
	}

	var buf bytes.Buffer
	if err := emitPreviewIcon(&buf, resp, outSVG, outPNG, true); err != nil {
		t.Fatalf("emitPreviewIcon: %v", err)
	}

	// Both files written with the expected bytes.
	if got, _ := os.ReadFile(outSVG); string(got) != "<svg>icon</svg>" {
		t.Errorf("svg file = %q", got)
	}
	if got, _ := os.ReadFile(outPNG); !bytes.Equal(got, pngRaw) {
		t.Errorf("png file = %q, want %q", got, pngRaw)
	}

	var m WriteManifest
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("manifest parse: %v\n%s", err, buf.String())
	}
	if m.Command != "preview-icon" || len(m.Artifacts) != 2 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if m.Artifacts[0].Kind != "svg" || m.Artifacts[1].Kind != "png" {
		t.Errorf("artifact kinds = %s,%s", m.Artifacts[0].Kind, m.Artifacts[1].Kind)
	}
	if len(m.Warnings) != 1 || m.Warnings[0] != "fill ignored" {
		t.Errorf("warnings carried through = %v", m.Warnings)
	}
}

func TestEmitPreviewIcon_ManifestSkipsPNGWhenNotRasterized(t *testing.T) {
	dir := t.TempDir()
	outSVG := filepath.Join(dir, "icon.svg")
	outPNG := filepath.Join(dir, "icon.png")
	resp := &previewIconResponse{
		SVGData:   "<svg/>",
		PNGBase64: "", // rasterization produced nothing
	}

	var buf bytes.Buffer
	if err := emitPreviewIcon(&buf, resp, outSVG, outPNG, true); err != nil {
		t.Fatalf("emitPreviewIcon: %v", err)
	}
	if _, err := os.Stat(outPNG); !os.IsNotExist(err) {
		t.Errorf("expected no PNG file when png_base64 is empty, stat err = %v", err)
	}
	var m WriteManifest
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if len(m.Artifacts) != 1 || m.Artifacts[0].Kind != "svg" {
		t.Errorf("expected only the svg artifact, got %+v", m.Artifacts)
	}
}

func TestEmitPreviewIcon_NonManifestEmitsResponseWithClearedBytes(t *testing.T) {
	dir := t.TempDir()
	outSVG := filepath.Join(dir, "icon.svg")
	resp := &previewIconResponse{
		SVGData:    "<svg>icon</svg>",
		SourceKind: "bundled",
		Alt:        "an icon",
	}

	var buf bytes.Buffer
	if err := emitPreviewIcon(&buf, resp, outSVG, "", false); err != nil {
		t.Fatalf("emitPreviewIcon: %v", err)
	}
	var parsed previewIconResponse
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("response parse: %v\n%s", err, buf.String())
	}
	if parsed.SVGData != "" {
		t.Errorf("expected svg_data cleared after --out-svg, got %q", parsed.SVGData)
	}
	if parsed.SourceKind != "bundled" || parsed.Alt != "an icon" {
		t.Errorf("expected non-byte fields preserved, got %+v", parsed)
	}
	if got, _ := os.ReadFile(outSVG); string(got) != "<svg>icon</svg>" {
		t.Errorf("svg file = %q", got)
	}
}

// --- preview-patterns manifest assembly ---
//
// The full preview-patterns path needs LibreOffice + ImageMagick, so the
// manifest assembly is exercised directly over fake PNG files — the same
// (written, warnings) -> manifest reduction the command performs after its
// render loop.
func TestPreviewPatternsManifest_AssemblesWrittenAndWarnings(t *testing.T) {
	dir := t.TempDir()
	var written []artifactSpec
	for _, name := range []string{"card-grid", "kpi-3up"} {
		p := filepath.Join(dir, name+".png")
		if err := os.WriteFile(p, []byte("png:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
		written = append(written, artifactSpec{path: p, kind: "pattern-preview"})
	}
	warnings := []string{"midnight-blue/pyramid: expand failed"}

	m, err := buildWriteManifest("preview-patterns", written, warnings)
	if err != nil {
		t.Fatalf("buildWriteManifest: %v", err)
	}
	if len(m.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(m.Artifacts))
	}
	for _, a := range m.Artifacts {
		if a.Kind != "pattern-preview" {
			t.Errorf("kind = %q, want pattern-preview", a.Kind)
		}
	}
	if len(m.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %v", m.Warnings)
	}
}

// --- generate JSON manifest ---
//
// generate's machine-readable success manifest is the JSONOutput written via
// --json-output. This verifies the end-to-end manifest lists the written PPTX
// path and a content hash so agents need not scrape stderr logs.
func TestGenerate_JSONOutputManifest(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")
	examplePath := filepath.Join(projectRoot, "examples", "basic-deck.json")
	if _, err := os.Stat(examplePath); err != nil {
		t.Skipf("missing fixture %s: %v", examplePath, err)
	}

	caseDir := t.TempDir()
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("parse example: %v", err)
	}
	input["template"] = "midnight-blue"
	input["output_filename"] = "manifest-deck.pptx"
	patched, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal patched example: %v", err)
	}
	inputPath := filepath.Join(caseDir, "input.json")
	if err := os.WriteFile(inputPath, patched, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	resultPath := filepath.Join(caseDir, "result.json")
	if err := runJSONMode(inputPath, resultPath, templatesDir, caseDir, "", false, false, "midnight-blue", "off", false, "off", "free", false); err != nil {
		t.Fatalf("runJSONMode: %v", err)
	}

	raw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read result manifest: %v", err)
	}
	var out JSONOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse result manifest: %v\n%s", err, raw)
	}
	if !out.Success {
		t.Errorf("expected success=true, got error %q", out.Error)
	}
	if !strings.HasSuffix(out.OutputPath, "manifest-deck.pptx") {
		t.Errorf("output_path = %q, want suffix manifest-deck.pptx", out.OutputPath)
	}
	if _, err := os.Stat(out.OutputPath); err != nil {
		t.Errorf("manifest output_path does not exist on disk: %v", err)
	}
	if out.ContentHash == "" {
		t.Error("expected non-empty content_hash in success manifest")
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// TestHandleInspectSlideImages_MissingArg ensures the handler rejects a call
// with no slide_images parameter.
func TestHandleInspectSlideImages_MissingArg(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for missing slide_images")
	}
}

// TestHandleInspectSlideImages_EmptyArray asserts an empty slide_images array
// is rejected.
func TestHandleInspectSlideImages_EmptyArray(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for empty slide_images")
	}
}

// TestHandleInspectSlideImages_BothSourcesSet rejects an entry that supplies
// both path and png_base64.
func TestHandleInspectSlideImages_BothSourcesSet(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{
				"index":      float64(0),
				"path":       "/tmp/x.png",
				"png_base64": "aGVsbG8=",
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError when both path and png_base64 are set")
	}
}

// TestHandleInspectSlideImages_PathTraversal rejects relative or traversal paths.
func TestHandleInspectSlideImages_PathTraversal(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	for _, bad := range []string{
		"../../etc/passwd.png",
		"/tmp/../../../etc/secret.png",
		"slide-0.png", // relative
	} {
		t.Run(bad, func(t *testing.T) {
			res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
				"slide_images": []any{
					map[string]any{"index": float64(0), "path": bad},
				},
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res == nil || !res.IsError {
				t.Fatal("expected IsError for traversal/relative path")
			}
		})
	}
}

// TestHandleInspectSlideImages_BadExtension rejects an absolute path that
// doesn't end in .png/.jpg/.jpeg.
func TestHandleInspectSlideImages_BadExtension(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{"index": float64(0), "path": "/tmp/slide.txt"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for non-image extension")
	}
}

// TestHandleInspectSlideImages_InvalidBase64 rejects malformed base64.
func TestHandleInspectSlideImages_InvalidBase64(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{"index": float64(0), "png_base64": "not!!!base64"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for malformed base64")
	}
}

// TestHandleInspectSlideImages_NoAPIKey verifies that when ANTHROPIC_API_KEY is
// unset, the handler degrades to heuristic mode and returns a successful
// Report tagged with mode="heuristic" — not INSPECT_DISABLED.
func TestHandleInspectSlideImages_NoAPIKey(t *testing.T) {
	// Save and clear the env var for this test.
	saved := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if saved != "" {
			_ = os.Setenv("ANTHROPIC_API_KEY", saved)
		}
	}()

	// Build a valid 4x3 white PNG so the heuristic decoder can read it.
	pngBytes := makeSolidPNG(t, 16, 9, 0xFF, 0xFF, 0xFF)
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{
				"index":      float64(0),
				"png_base64": base64.StdEncoding.EncodeToString(pngBytes),
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.IsError {
		t.Fatalf("expected success result in heuristic fallback, got error: %s", textContent(res))
	}
	got := textContent(res)
	if !strings.Contains(got, `"mode":"heuristic"`) && !strings.Contains(got, `"mode": "heuristic"`) {
		t.Errorf("expected mode=heuristic in response, got: %s", got)
	}
	if strings.Contains(got, "INSPECT_DISABLED") {
		t.Errorf("did not expect INSPECT_DISABLED in heuristic fallback, got: %s", got)
	}
}

// TestInspectSlideImages_FindingsEnvelopeShape verifies that inspect_slide_images
// returns its per-slide findings projected into a FindingEnvelope under the
// "findings" key, while keeping the visualqa.Report rollups (mode, results,
// total_*) at the top level. The envelope is always present so an agent can
// branch on findings.ok deterministically.
func TestInspectSlideImages_FindingsEnvelopeShape(t *testing.T) {
	// Run in heuristic mode for a deterministic, key-free pass.
	saved := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if saved != "" {
			_ = os.Setenv("ANTHROPIC_API_KEY", saved)
		}
	}()

	pngBytes := makeSolidPNG(t, 16, 9, 0xFF, 0xFF, 0xFF)
	mc := cliMCPConfig("./templates", "./out")
	result, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{
				"index":      float64(0),
				"png_base64": base64.StdEncoding.EncodeToString(pngBytes),
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	// Typed view: the envelope is always present and stamped for this surface.
	var output inspectOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if output.Findings.SchemaVersion != diagnostics.SchemaVersion {
		t.Errorf("findings.schema_version = %q, want %q", output.Findings.SchemaVersion, diagnostics.SchemaVersion)
	}
	if output.Findings.Tool != diagnostics.DefaultTool {
		t.Errorf("findings.tool = %q, want %q", output.Findings.Tool, diagnostics.DefaultTool)
	}
	if output.Findings.Subcommand != "inspect_slide_images" {
		t.Errorf("findings.subcommand = %q, want inspect_slide_images", output.Findings.Subcommand)
	}
	if output.Findings.Findings == nil {
		t.Error("findings.findings must be non-nil (may be empty)")
	}
	// Every projected finding carries a namespaced (dotted) code in the
	// RENDER/FIT namespaces, an info-or-stronger severity, and the source label
	// in evidence.
	for _, f := range output.Findings.Findings {
		if f.Category != diagnostics.NamespaceRender && f.Category != diagnostics.NamespaceFit {
			t.Errorf("finding %q category = %q, want RENDER or FIT", f.Code, f.Category)
		}
		if !strings.HasPrefix(f.Code, f.Category+".") {
			t.Errorf("finding code %q is not namespaced under %q", f.Code, f.Category)
		}
		if f.Evidence["visual_severity"] == nil {
			t.Errorf("finding %q missing evidence.visual_severity", f.Code)
		}
	}

	// Wire view: the report rollups stay at the top level and "findings" is the
	// envelope object carrying its required fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(textContent(result)), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	for _, key := range []string{"mode", "results", "total_issues", "findings"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response missing top-level key %q", key)
		}
	}
	var env map[string]any
	if err := json.Unmarshal(raw["findings"], &env); err != nil {
		t.Fatalf("findings is not an object: %v", err)
	}
	for _, key := range []string{"schema_version", "tool", "subcommand", "ok", "summary", "findings"} {
		if _, ok := env[key]; !ok {
			t.Errorf("findings envelope missing required key %q", key)
		}
	}
}

// makeSolidPNG returns a single-color PNG of the given dimensions.
func makeSolidPNG(t *testing.T, w, h int, r, g, b uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestValidateInspectImagePath exercises the path validator directly.
func TestValidateInspectImagePath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty", "", true},
		{"relative", "slide.png", true},
		{"traversal", "/tmp/../etc/passwd.png", true},
		{"absolute png", "/tmp/slide.png", false},
		{"absolute jpg", "/tmp/slide.jpg", false},
		{"absolute jpeg", "/tmp/slide.JPEG", false},
		{"absolute wrong ext", "/tmp/slide.pdf", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInspectImagePath(tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.path, err)
			}
		})
	}
}

// TestLoadInspectImage_FromPath round-trips a real file through the loader.
func TestLoadInspectImage_FromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slide.png")
	if err := os.WriteFile(path, []byte("fake-png"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := loadInspectImage(inspectSlideImageInput{Index: 0, Path: path})
	if err != nil {
		t.Fatalf("loadInspectImage: %v", err)
	}
	if string(data) != "fake-png" {
		t.Errorf("got %q, want %q", string(data), "fake-png")
	}
}

// TestLoadInspectImage_DataURLPrefix tolerates a data: URL prefix on base64.
func TestLoadInspectImage_DataURLPrefix(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("hello"))
	entry := inspectSlideImageInput{
		Index:     0,
		PNGBase64: "data:image/png;base64," + payload,
	}
	data, err := loadInspectImage(entry)
	if err != nil {
		t.Fatalf("loadInspectImage: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

// Sanity: ensure the visualqa.Report shape we serialize matches the output
// schema's required keys at the field level.
func TestInspectOutputSchema_HasRequiredKeys(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal(outputSchemaInspectSlideImages, &parsed); err != nil {
		t.Fatalf("output schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]any)
	if !ok {
		t.Fatal("schema missing required[] array")
	}
	want := map[string]bool{"slide_count": false, "results": false, "total_issues": false}
	for _, r := range required {
		if s, ok := r.(string); ok {
			if _, expected := want[s]; expected {
				want[s] = true
			}
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("schema is missing required key %q", k)
		}
	}
}

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
// unset, the handler returns INSPECT_DISABLED rather than INTERNAL or a panic.
// The image source itself must be valid so we get past the parsing stage.
func TestHandleInspectSlideImages_NoAPIKey(t *testing.T) {
	// Save and clear the env var for this test.
	saved := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if saved != "" {
			_ = os.Setenv("ANTHROPIC_API_KEY", saved)
		}
	}()

	// Use base64 source with non-zero bytes (the handler doesn't decode
	// image content until it reaches the agent).
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleInspectSlideImages(context.Background(), makeRequest(map[string]any{
		"slide_images": []any{
			map[string]any{
				"index":      float64(0),
				"png_base64": base64.StdEncoding.EncodeToString([]byte("fake-png-bytes")),
			},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError when ANTHROPIC_API_KEY is unset")
	}
	if got := textContent(res); !strings.Contains(got, "INSPECT_DISABLED") {
		t.Errorf("expected INSPECT_DISABLED in error envelope, got: %s", got)
	}
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

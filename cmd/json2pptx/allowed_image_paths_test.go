package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/config"
)

// go-slide-creator-s1uvj.19: ALLOWED_IMAGE_PATHS was loaded into
// config.Images.AllowedBasePaths but never reached
// generator.GenerationRequest.AllowedImagePaths, so an operator's image-root
// restriction silently did nothing. An image outside the configured root must
// be refused; with no restriction configured the same image still embeds.
func TestGenerateHonorsAllowedImagePaths(t *testing.T) {
	outsideDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(outsideDir); err == nil {
		outsideDir = resolved
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "photo.png"), buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{map[string]any{
			"layout_id": "slideLayout2",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Photo"},
				map[string]any{"placeholder_id": "body", "type": "image", "image_value": map[string]any{"path": "photo.png", "alt": "photo"}},
			},
		}},
	}

	run := func(allowed []string) string {
		t.Helper()
		cfg := config.DefaultConfig()
		cfg.Images.AllowedBasePaths = allowed
		mc := testMCPConfig(t)
		mc.cfg = cfg
		result, err := mc.handleGenerate(t.Context(), byoRequest(map[string]any{
			"presentation":    deck,
			"base_dir":        outsideDir,
			"output_filename": "photo.pptx",
		}))
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		return textContent(result)
	}

	const refused = "image path validation failed"
	if out := run(nil); strings.Contains(out, refused) {
		t.Fatalf("unrestricted config refused the image: %s", out)
	}
	if out := run([]string{t.TempDir()}); !strings.Contains(out, refused) {
		t.Fatalf("image outside ALLOWED_IMAGE_PATHS root was not refused: %s", out)
	}
	if out := run([]string{outsideDir}); strings.Contains(out, refused) {
		t.Fatalf("image inside ALLOWED_IMAGE_PATHS root was refused: %s", out)
	}
}

func TestImageAllowList(t *testing.T) {
	if got := imageAllowList(nil, "/cache"); got != nil {
		t.Errorf("empty configuration must stay unrestricted, got %v", got)
	}
	got := imageAllowList([]string{"/imgs"}, "", "/cache")
	if len(got) != 2 || got[0] != "/imgs" || got[1] != "/cache" {
		t.Errorf("imageAllowList = %v, want [/imgs /cache]", got)
	}
}

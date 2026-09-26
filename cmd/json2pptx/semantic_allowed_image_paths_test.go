package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// go-slide-creator-s1uvj.44: `semantic render` built its config from
// config.DefaultConfig() without environment overrides, so ALLOWED_IMAGE_PATHS
// (config Images.AllowedBasePaths) never reached
// GenerationRequest.AllowedImagePaths on this path. Mirrors
// TestGenerateHonorsAllowedImagePaths: an image outside the configured root
// must be refused; unrestricted and in-root renders must still embed it.
func TestSemanticRenderHonorsAllowedImagePaths(t *testing.T) {
	specPath := writeSpecWithImage(t, "deck.yaml", fmt.Sprintf(rawImageSemanticSpec, "logo.png"), "logo.png")
	specDir := filepath.Dir(specPath)
	if resolved, err := filepath.EvalSymlinks(specDir); err == nil {
		specDir = resolved
	}

	run := func(allowed string) string {
		t.Helper()
		t.Setenv("ALLOWED_IMAGE_PATHS", allowed)
		out := filepath.Join(t.TempDir(), "raw.pptx")
		orig := os.Args
		defer func() { os.Args = orig }()
		os.Args = []string{"json2pptx", "render", "--spec", specPath, "--output", out, "--templates-dir", testTemplatesDir}
		var stdout string
		stderr := captureStderr(t, func() {
			stdout = captureStdout(t, func() { _ = runSemantic() })
		})
		return stdout + stderr
	}

	const refused = "image path validation failed"
	if out := run(""); strings.Contains(out, refused) {
		t.Fatalf("unrestricted config refused the image: %s", out)
	}
	if out := run(t.TempDir()); !strings.Contains(out, refused) {
		t.Fatalf("image outside ALLOWED_IMAGE_PATHS root was not refused: %s", out)
	}
	if out := run(specDir); strings.Contains(out, refused) {
		t.Fatalf("image inside ALLOWED_IMAGE_PATHS root was refused: %s", out)
	}
}

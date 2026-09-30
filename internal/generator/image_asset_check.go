package generator

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// imageSniffBytes is how much of an image file is read to recognise it.
const imageSniffBytes = 8192

// imageAssetProblem reports why an authored image file cannot be embedded, or
// "" when it can: the path must pass the image-root policy, exist, be a
// readable regular file and carry a recognised image signature (raster magic
// bytes, or an <svg root for .svg). A file that merely exists is not enough —
// a truncated download or a text file named .png embeds as a broken picture
// (go-slide-creator-b7qqg.2).
func imageAssetProblem(path string, allowedPaths []string) string {
	if err := ValidateImagePathWithConfig(path, allowedPaths); err != nil {
		return fmt.Sprintf("security: image path validation failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "file not found"
	}
	if info.IsDir() {
		return "path is a directory, not an image file"
	}
	f, err := os.Open(path) //nolint:gosec // path validated above
	if err != nil {
		return "file is not readable"
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, imageSniffBytes)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "file is not readable"
	}
	head = head[:n]
	if IsSVGFile(path) {
		if !strings.Contains(strings.ToLower(string(head)), "<svg") {
			return "file is not a valid SVG image (no <svg root)"
		}
		return ""
	}
	if !hasRasterMagic(head) {
		return "file content is not a recognised image format"
	}
	return ""
}

// reportUnavailableImage records an authored picture that could not be
// embedded as a refuse-class IMAGE_ASSET_UNAVAILABLE finding addressed to its
// source field, alongside the legacy free-text warning.
func (ctx *singlePassContext) reportUnavailableImage(kind, imagePath, sourcePath, reason string) {
	ctx.warnings = append(ctx.warnings, fmt.Sprintf("%s: %s: %s", kind, reason, imagePath))
	ctx.emitFitFinding(patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    sourcePath,
			Code:    patterns.ErrCodeImageAssetUnavailable,
			Message: fmt.Sprintf("%s %q could not be embedded (%s); the frame renders empty", kind, imagePath, reason),
			Fix: &patterns.FixSuggestion{Kind: "provide_value", Params: map[string]any{
				"path": sourcePath,
				"hint": "supply an existing image file (relative paths resolve against base_dir) or a reachable http(s) url, or remove the image field",
			}},
		},
		Action: "refuse",
	})
}

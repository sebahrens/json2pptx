package templatepreview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
)

const manifestName = "manifest.json"
const previewRecipe = "native-layout-thumbnail-v2"

type previewManifest struct {
	Schema       int                                `json:"schema_version"`
	Recipe       string                             `json:"recipe"`
	TemplateHash string                             `json:"template_sha256"`
	SourceHash   string                             `json:"rendering_source_sha256,omitempty"`
	CacheKey     string                             `json:"render_cache_identity"`
	Width        int                                `json:"width"`
	DPI          int                                `json:"dpi"`
	Images       map[string]string                  `json:"png_sha256"`
	Content      map[string][]generator.ContentItem `json:"source_content"`
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// RenderingSourceHash binds shipped previews to the repository's rendering
// implementation, without a circular dependency on generated thumbnails or a
// binary that embeds them. Test sources are not rendering inputs.
func RenderingSourceHash(root string) (string, error) {
	var paths []string
	for _, dir := range []string{"internal", "svggen", "cmd/templatepreviews"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && ((strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")) || entry.Name() == "go.mod" || entry.Name() == "go.sum") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("preview source inventory: %w", err)
		}
	}
	paths = append(paths, filepath.Join(root, "go.mod"), filepath.Join(root, "go.sum"))
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // scoped repository source inventory
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(data))
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validManifest(data, png []byte, templateHash, layoutID string) bool {
	var manifest previewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	return manifest.Schema == 1 && manifest.Recipe == previewRecipe && manifest.TemplateHash == templateHash && manifest.Images[layoutID] == hashBytes(png)
}

func diskPreviewIsCurrent(templatePath, path, layoutID string, templateHash string) bool {
	metadata, err := os.ReadFile(filepath.Join(filepath.Dir(path), manifestName)) //nolint:gosec // adjacent preview metadata
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path) //nolint:gosec // adjacent preview PNG
	if err != nil || !validManifest(metadata, data, templateHash, layoutID) {
		return false
	}
	return previewSourcesAreCurrent(templatePath, metadata)
}

func previewSourcesAreCurrent(templatePath string, metadata []byte) bool {
	// A local repository can additionally verify current rendering sources.
	// Installed distributions ship manifests verified by repository tests.
	root, err := filepath.Abs(filepath.Dir(templatePath))
	if err != nil {
		return false
	}
	for {
		_, sourceErr := os.Stat(filepath.Join(root, "internal", "templatepreview"))
		_, moduleErr := os.Stat(filepath.Join(root, "go.mod"))
		if sourceErr == nil && moduleErr == nil {
			var manifest previewManifest
			if json.Unmarshal(metadata, &manifest) != nil {
				return false
			}
			current, err := RenderingSourceHash(root)
			return err == nil && current == manifest.SourceHash
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return true
}

package templatepreview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// RenderingPackageDirs are the repository package directories compiled into
// cmd/templatepreviews — exactly its transitive in-repo import set
// (`go list -deps ./cmd/templatepreviews`; TestRenderingPackageDirsMatchImports
// keeps the two in sync). Only these can change preview pixels, so only these
// are hashed: an edit to internal/api or internal/resource no longer marks
// every shipped preview stale (go-slide-creator-csclk.1).
var RenderingPackageDirs = []string{
	"cmd/templatepreviews",
	"internal/bidisafe",
	"internal/deckinput",
	"internal/diagnostics",
	"internal/generator",
	"internal/jsonschema",
	"internal/layout",
	"internal/layoutpreview",
	"internal/patterns",
	"internal/pixelhash",
	"internal/placeholderrole",
	"internal/pptx",
	"internal/render",
	"internal/shapegrid",
	"internal/slidepath",
	"internal/template",
	"internal/templatepreview",
	"internal/textcapacity",
	"internal/textfit",
	"internal/tokens",
	"internal/types",
	"internal/utils",
	"svggen",
	"svggen/core",
	"svggen/fontcache",
	"svggen/fonts",
	"svggen/icons",
	"svggen/raster",
	"svggen/safeyaml",
	"templates",
}

// RenderingSourceHash binds shipped previews to the repository's rendering
// implementation, without a circular dependency on generated thumbnails or a
// binary that embeds them. It covers the non-test Go sources, go.mod/go.sum
// and font assets of RenderingPackageDirs (each directory only, not its
// subdirectories — sub-packages are listed separately), plus the root module
// files. A listed directory absent from root is skipped.
func RenderingSourceHash(root string) (string, error) {
	var paths []string
	for _, dir := range RenderingPackageDirs {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("preview source inventory: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			fontAsset := strings.EqualFold(filepath.Ext(name), ".ttf") || strings.EqualFold(filepath.Ext(name), ".otf")
			if !entry.IsDir() && ((strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")) || name == "go.mod" || name == "go.sum" || fontAsset) {
				paths = append(paths, filepath.Join(root, filepath.FromSlash(dir), name))
			}
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

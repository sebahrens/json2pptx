package layoutpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Bump when the preview recipe or raster protocol changes independently of the
// executable. Executable bytes also isolate dirty/rebuilt generator binaries.
const previewCacheSchema = "layout-preview-v2"

var previewEngineIdentity = sync.OnceValues(func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return fileHash(path)
})

// Include the tool version as well as entrypoint bytes: bundled entrypoints can
// be wrapper scripts whose target changes without changing the wrapper itself.
func previewToolIdentity(binary string) (string, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", err
	}
	bytesHash, err := fileHash(path)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, err := previewRendererVersion(ctx, path)
	if err != nil {
		return "", fmt.Errorf("identify renderer %s: %w", path, err)
	}
	sum := sha256.Sum256(append([]byte(path+"\x00"+bytesHash+"\x00"), version...))
	return hex.EncodeToString(sum[:]), nil
}

func previewRendererVersion(ctx context.Context, path string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, "--version") //nolint:gosec // resolved rendering binary
	// A wrapper's descendants must not keep stdout open indefinitely after the
	// version probe deadline kills the wrapper.
	command.WaitDelay = time.Second
	version, err := command.Output()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(version)) == "" {
		return nil, fmt.Errorf("renderer reported an empty version")
	}
	return version, nil
}

func previewCacheIdentity(templateHash, engineHash, officeHash, rasterHash string, analysis *types.TemplateAnalysis, dpi int) (string, error) {
	// The effective sample recipe matters too: callers can pass a different
	// layout inventory/classification for the same template bytes.
	slides := make([]generator.SlideSpec, len(analysis.Layouts))
	for i, layout := range analysis.Layouts {
		slides[i] = generator.SlideSpec{LayoutID: layout.ID, Content: SampleContent(layout)}
	}
	data, err := json.Marshal(struct {
		Schema, Template, Engine, Office, Raster string
		DPI                                      int
		Slides                                   []generator.SlideSpec
	}{previewCacheSchema, templateHash, engineHash, officeHash, rasterHash, dpi, slides})
	if err != nil {
		return "", fmt.Errorf("encode preview recipe: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func currentPreviewCacheIdentity(templatePath string, analysis *types.TemplateAnalysis, opts *Options) (string, error) {
	templateHash, err := fileHash(templatePath)
	if err != nil {
		return "", fmt.Errorf("hash template: %w", err)
	}
	engineHash, err := previewEngineIdentity()
	if err != nil {
		return "", fmt.Errorf("hash preview executable: %w", err)
	}
	officeHash, err := previewToolIdentity(libreOfficeBin())
	if err != nil {
		return "", err
	}
	rasterHash, err := previewToolIdentity(imageMagickBin())
	if err != nil {
		return "", err
	}
	return previewCacheIdentity(templateHash, engineHash, officeHash, rasterHash, analysis, opts.dpi())
}

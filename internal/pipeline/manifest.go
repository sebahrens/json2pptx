package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

const AuthoringManifestVersion = "1"

type LogicalSlide struct {
	ID              string `json:"id"`
	SourceIndex     int    `json:"source_index"`
	GeneratedSlides []int  `json:"generated_slides"`
}

type VisualEvidence struct {
	ArtifactSHA256  string   `json:"artifact_sha256"`
	Renderer        string   `json:"renderer,omitempty"`
	RendererVersion string   `json:"renderer_version,omitempty"`
	Fonts           []string `json:"fonts,omitempty"`
	ReviewedSlides  []string `json:"reviewed_slides,omitempty"`
	Verdict         string   `json:"verdict,omitempty"`
	Reviewer        string   `json:"reviewer,omitempty"` // vision, provider, host, or manual
}

type AuthoringManifest struct {
	Version        string                          `json:"version"`
	Revision       string                          `json:"revision"`
	CreatedAt      time.Time                       `json:"created_at"`
	SourceFormat   string                          `json:"source_format"`
	Source         string                          `json:"source"`
	SourceSHA256   string                          `json:"source_sha256"`
	TemplatePath   string                          `json:"template_path"`
	TemplateSHA256 string                          `json:"template_sha256"`
	CompiledJSON   json.RawMessage                 `json:"compiled_json"`
	CompiledSHA256 string                          `json:"compiled_sha256"`
	PPTXPath       string                          `json:"pptx_path"`
	PPTXSHA256     string                          `json:"pptx_sha256"`
	Slides         []LogicalSlide                  `json:"slides"`
	SourceMap      map[string]semantic.SourceEntry `json:"source_map"`
	Diagnostics    json.RawMessage                 `json:"diagnostics,omitempty"`
	VisualEvidence *VisualEvidence                 `json:"visual_evidence,omitempty"`
}

func NewAuthoringManifest(source []byte, sourceFormat, templatePath, templateHash string, compiled json.RawMessage, pptxPath, pptxHash string, sourceMap *semantic.SourceMap, slidePayloads []json.RawMessage, diagnostics json.RawMessage) *AuthoringManifest {
	sourceHash := hashBytes(source)
	compiledHash := hashBytes(compiled)
	revision := hashBytes([]byte(sourceHash + ":" + templateHash + ":" + compiledHash))
	entries := map[string]semantic.SourceEntry{}
	if sourceMap != nil {
		for k, v := range sourceMap.Entries() {
			entries[k] = v
		}
	}
	seen := make(map[string]int)
	slides := make([]LogicalSlide, len(slidePayloads))
	for i, payload := range slidePayloads {
		base := hashBytes(payload)[:12]
		seen[base]++
		id := "slide-" + base
		if seen[base] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[base])
		}
		slides[i] = LogicalSlide{ID: id, SourceIndex: i, GeneratedSlides: []int{i}}
	}
	return &AuthoringManifest{Version: AuthoringManifestVersion, Revision: revision, CreatedAt: time.Now().UTC(), SourceFormat: sourceFormat, Source: string(source), SourceSHA256: sourceHash, TemplatePath: templatePath, TemplateSHA256: templateHash, CompiledJSON: append(json.RawMessage(nil), compiled...), CompiledSHA256: compiledHash, PPTXPath: pptxPath, PPTXSHA256: pptxHash, Slides: slides, SourceMap: entries, Diagnostics: diagnostics}
}

func WriteAuthoringManifest(path string, manifest *AuthoringManifest) error {
	if manifest == nil {
		return fmt.Errorf("authoring manifest is required")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".authoring-manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func ReadAuthoringManifest(path string) (*AuthoringManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m AuthoringManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Version != AuthoringManifestVersion {
		return nil, fmt.Errorf("unsupported authoring manifest version %q", m.Version)
	}
	return &m, nil
}

func (m *AuthoringManifest) EvidenceCurrent(sourceHash, templateHash, pptxHash string) bool {
	if m == nil || m.SourceSHA256 != sourceHash || m.TemplateSHA256 != templateHash || m.PPTXSHA256 != pptxHash {
		return false
	}
	return m.VisualEvidence != nil && m.VisualEvidence.ArtifactSHA256 == pptxHash
}

func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

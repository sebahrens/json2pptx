package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// WrittenArtifact describes a single file produced by a CLI command. It carries
// enough identity (path, kind, byte length, content hash) for an agent to verify
// and reuse the artifact without re-reading it.
type WrittenArtifact struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// WriteManifest is the structured success result emitted by CLI commands that
// write files to disk (preview-wireframe, preview-icon, preview-patterns, …).
// It gives agents a machine-readable list of written paths, content hashes,
// artifact kinds, and any warnings instead of forcing them to scrape
// human-friendly stdout/stderr logs.
type WriteManifest struct {
	Success   bool              `json:"success"`
	Command   string            `json:"command"`
	Artifacts []WrittenArtifact `json:"artifacts"`
	Warnings  []string          `json:"warnings,omitempty"`
}

// artifactSpec names a written file and its kind before the manifest builder
// stats it from disk. Kind is a short artifact label (e.g. "svg", "png",
// "pptx", "pattern-preview").
type artifactSpec struct {
	path string
	kind string
}

// describeArtifact stats the file at path and computes its SHA-256, streaming
// the bytes so large outputs are never fully buffered. It fails when the file
// cannot be read back — a missing file means the preceding write silently
// failed and the manifest must not claim success.
func describeArtifact(path, kind string) (WrittenArtifact, error) {
	f, err := os.Open(path) //nolint:gosec // path is a CLI-provided output target
	if err != nil {
		return WrittenArtifact{}, fmt.Errorf("stat artifact %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return WrittenArtifact{}, fmt.Errorf("hash artifact %q: %w", path, err)
	}
	return WrittenArtifact{
		Path:   path,
		Kind:   kind,
		Bytes:  n,
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// buildWriteManifest describes every spec and assembles a success manifest.
// The artifacts slice is always non-nil so the JSON encodes as [] rather than
// null when no files were written.
func buildWriteManifest(command string, specs []artifactSpec, warnings []string) (WriteManifest, error) {
	artifacts := make([]WrittenArtifact, 0, len(specs))
	for _, s := range specs {
		a, err := describeArtifact(s.path, s.kind)
		if err != nil {
			return WriteManifest{}, err
		}
		artifacts = append(artifacts, a)
	}
	return WriteManifest{
		Success:   true,
		Command:   command,
		Artifacts: artifacts,
		Warnings:  warnings,
	}, nil
}

// printWriteManifest writes the manifest to w as indented JSON with a trailing
// newline, matching the formatting of the other CLI JSON emitters.
func printWriteManifest(w io.Writer, m WriteManifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

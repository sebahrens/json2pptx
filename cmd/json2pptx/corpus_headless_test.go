//go:build integration
// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCorpusHeadlessOpen exercises the full "zero needs repair" guarantee:
// every examples/*.json is generated against every bundled template, then the
// resulting PPTX is round-tripped through headless LibreOffice. The test fails
// if LibreOffice prints anything that looks like a repair / corruption /
// invalid-structure warning.
//
// It is gated behind the `integration` build tag so the default `go test ./...`
// stays fast and does not require LibreOffice on contributor machines or in CI
// jobs that lack soffice. The CI workflow `corpus-headless` installs
// libreoffice-headless and invokes:
//
//	go test -tags=integration ./cmd/json2pptx/... -run Corpus
//
// Per task go-slide-creator-ir61.
func TestCorpusHeadlessOpen(t *testing.T) {
	soffice := findSofficeBinary()
	if soffice == "" {
		t.Skip("skipping corpus headless test: soffice/libreoffice not found on PATH")
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")
	examplesDir := filepath.Join(projectRoot, "examples")

	matches, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no examples/*.json files found at %s", examplesDir)
	}

	templates := []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"}

	// LibreOffice complains loudly when two concurrent invocations share a user
	// profile, so give each subtest its own --user-profile and keep the suite
	// sequential. Worst case ~92 round-trips × a few seconds each — acceptable
	// for an integration-tagged job.
	baseTmp := t.TempDir()

	for _, examplePath := range matches {
		example := examplePath
		base := strings.TrimSuffix(filepath.Base(example), ".json")
		for _, tmpl := range templates {
			tmpl := tmpl
			name := base + "/" + tmpl
			t.Run(name, func(t *testing.T) {
				runHeadlessRoundTrip(t, soffice, baseTmp, templatesDir, example, base, tmpl)
			})
		}
	}
}

func runHeadlessRoundTrip(t *testing.T, soffice, baseTmp, templatesDir, examplePath, base, tmpl string) {
	t.Helper()

	caseDir := filepath.Join(baseTmp, base+"_"+tmpl)
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatalf("mkdir case dir: %v", err)
	}

	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("parse example JSON: %v", err)
	}
	input["template"] = tmpl
	outputFilename := base + "_" + tmpl + ".pptx"
	input["output_filename"] = outputFilename

	patched, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal patched example: %v", err)
	}
	patchedJSON := filepath.Join(caseDir, "input.json")
	if err := os.WriteFile(patchedJSON, patched, 0o644); err != nil {
		t.Fatalf("write patched example: %v", err)
	}

	resultJSON := filepath.Join(caseDir, "result.json")
	// design_mode=free here intentionally bypasses the constrained-mode
	// validator: some bundled examples still carry legacy absolute font sizes
	// and raw hex colors. Those are orthogonal to the "zero needs repair"
	// guarantee we are testing — we only care that the generated PPTX opens
	// cleanly in LibreOffice. strict-fit is left off so density warnings on a
	// non-original template don't mask the actual round-trip outcome.
	if err := runJSONMode(patchedJSON, resultJSON, templatesDir, caseDir, "", false, false, tmpl, "off", false, "off", "free", false); err != nil {
		t.Fatalf("runJSONMode failed for %s × %s: %v", examplePath, tmpl, err)
	}

	generated := filepath.Join(caseDir, outputFilename)
	if _, err := os.Stat(generated); err != nil {
		t.Fatalf("generated PPTX not found at %s: %v", generated, err)
	}

	roundTripDir := filepath.Join(caseDir, "roundtrip")
	if err := os.MkdirAll(roundTripDir, 0o755); err != nil {
		t.Fatalf("mkdir roundtrip dir: %v", err)
	}
	userProfile := filepath.Join(caseDir, "lo-profile")
	if err := os.MkdirAll(userProfile, 0o755); err != nil {
		t.Fatalf("mkdir user profile: %v", err)
	}

	cmd := exec.Command( //nolint:gosec // arguments are test-controlled
		soffice,
		"-env:UserInstallation=file://"+userProfile,
		"--headless",
		"--nologo",
		"--nofirststartwizard",
		"--convert-to", "pptx",
		"--outdir", roundTripDir,
		generated,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("soffice round-trip failed for %s: %v\nstdout: %s\nstderr: %s",
			generated, err, stdout.String(), stderr.String())
	}

	roundTripped := filepath.Join(roundTripDir, outputFilename)
	if _, err := os.Stat(roundTripped); err != nil {
		t.Fatalf("round-tripped PPTX not found at %s: %v\nstdout: %s\nstderr: %s",
			roundTripped, err, stdout.String(), stderr.String())
	}

	if hits := scanRepairWarnings(stdout.Bytes(), stderr.Bytes()); len(hits) > 0 {
		t.Fatalf("LibreOffice reported repair/corruption warnings for %s × %s:\n  %s\nfull stdout:\n%s\nfull stderr:\n%s",
			base, tmpl, strings.Join(hits, "\n  "), stdout.String(), stderr.String())
	}
}

// repairWarningRe matches the words LibreOffice (and its loaders) use when
// signalling that a document needed structural repair, was malformed, or
// otherwise diverged from the OOXML contract. The check is intentionally broad
// — any one of these tokens on any output line is a failure, since the
// "zero needs repair" guarantee forbids them.
var repairWarningRe = regexp.MustCompile(`(?i)\b(repair|corrupt|invalid|malformed)\b`)

// soffice emits a handful of platform-level diagnostics that happen to contain
// our trigger words but say nothing about the PPTX being opened:
//   - macOS: "soffice[…]: Task policy set failed: 4 ((os/kern) invalid argument)"
//   - Linux: "javaldx", GTK / fontconfig / gpgme warnings, "invalid printer setup"
//
// We allowlist them explicitly so genuine OOXML repair messages from any
// loader survive the filter.
var benignNoiseRe = regexp.MustCompile(`(?i)javaldx|gtk-warning|fontconfig|gpgme|libgldi|warn:legacy\.tools|invalid printer setup|task policy set failed|os/kern`)

func scanRepairWarnings(stdout, stderr []byte) []string {
	var hits []string
	for _, chunk := range [][]byte{stdout, stderr} {
		for _, line := range strings.Split(string(chunk), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !repairWarningRe.MatchString(trimmed) {
				continue
			}
			if benignNoiseRe.MatchString(trimmed) {
				continue
			}
			hits = append(hits, trimmed)
		}
	}
	return hits
}

func findSofficeBinary() string {
	if path, err := exec.LookPath("soffice"); err == nil {
		return path
	}
	if path, err := exec.LookPath("libreoffice"); err == nil {
		return path
	}
	return ""
}

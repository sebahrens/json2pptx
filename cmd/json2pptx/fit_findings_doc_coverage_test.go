package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestFitFindingsDocCoversAdvertisedCodes guards go-slide-creator-csclk.70:
// every code advertised in get_capabilities().vocabularies.fit_finding_codes
// must be documented in docs/FIT_FINDINGS.md, and retired codes must not keep
// a full catalog section.
func TestFitFindingsDocCoversAdvertisedCodes(t *testing.T) {
	doc := readRepoFile(t, filepath.Join("docs", "FIT_FINDINGS.md"))
	for _, code := range patterns.AllFitFindingCodes() {
		if !strings.Contains(doc, "`"+code+"`") {
			t.Errorf("fit finding code %q is advertised but not documented in docs/FIT_FINDINGS.md", code)
		}
	}
	for _, retired := range []string{"diagram_aspect_conflict"} {
		if strings.Contains(doc, "### `"+retired+"`") {
			t.Errorf("docs/FIT_FINDINGS.md still has a section for retired code %q", retired)
		}
	}
}

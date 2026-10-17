package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

var updateFitAppendix = flag.Bool("update-fit-appendix", false, "regenerate the describe_finding appendix in docs/FIT_FINDINGS.md")

const fitAppendixBegin = "<!-- BEGIN GENERATED: describe_finding reference -->"
const fitAppendixEnd = "<!-- END GENERATED: describe_finding reference -->"

// The fit guide is the entry point linked by tools for finding explanations.
// Keep its additional reference complete and its remedies synced with the
// live registry, without duplicating the hand-authored walkthroughs above it.
func TestFitFindingsDocCoversDescribeCatalog(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "FIT_FINDINGS.md")
	doc := readRepoFile(t, filepath.Join("docs", "FIT_FINDINGS.md"))
	i, j := strings.Index(doc, fitAppendixBegin), strings.Index(doc, fitAppendixEnd)
	if i < 0 || j < i {
		t.Fatal("FIT_FINDINGS.md is missing describe_finding appendix markers")
	}
	prose := doc[:i] + doc[j+len(fitAppendixEnd):]
	var want strings.Builder
	want.WriteString(fitAppendixBegin + "\n\n")
	cell := strings.NewReplacer("|", `\|`, "\n", " ")
	for _, code := range diagnostics.AllDescribableCodes() {
		if docMentionsCode(prose, code) {
			continue
		}
		meta, ok := diagnostics.Describe(code)
		if !ok {
			t.Fatalf("catalogue code %q does not resolve", code)
		}
		fmt.Fprintf(&want, "### `%s`\n\n%s\n\n- **Severity:** %s\n- **When emitted:** %s\n- **Action:** %s\n\n", code,
			cell.Replace(meta.Summary), meta.Severity, cell.Replace(meta.WhenEmitted), cell.Replace(strings.Join(meta.RemediationSteps, " ")))
	}
	want.WriteString(fitAppendixEnd)
	if *updateFitAppendix {
		if err := os.WriteFile(path, []byte(doc[:i]+want.String()+doc[j+len(fitAppendixEnd):]), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if doc[i:j+len(fitAppendixEnd)] != want.String() {
		t.Fatal("describe_finding appendix is stale; run go test ./cmd/json2pptx -run '^TestFitFindingsDocCoversDescribeCatalog$' -update-fit-appendix")
	}
}

// Catalogue promises cover the full live vocabularies, including additions
// outside the contributor guide's longer worked examples.
func TestVisualCatalogDocsCoverRegistry(t *testing.T) {
	catalog := readRepoFile(t, filepath.Join("docs", "wiki", "layout-catalog.md"))
	guide := readRepoFile(t, filepath.Join("docs", "PATTERNS.md"))
	kinds, _, ok := strings.Cut(catalog, "## 3.")
	if !ok {
		t.Fatal("layout catalogue kind section is missing")
	}
	for _, kind := range semantic.AllSlideKinds() {
		if !strings.Contains(kinds, "\n| `"+string(kind)+"` |") {
			t.Errorf("layout catalogue omits semantic kind %s", kind)
		}
	}
	for _, pattern := range patterns.Default().List() {
		for name, doc := range map[string]string{"layout catalogue": catalog, "pattern guide": guide} {
			if !strings.Contains(doc, "`"+pattern.Name()+"`") {
				t.Errorf("%s omits named pattern %s", name, pattern.Name())
			}
		}
	}
}

func TestChartCatalogDocsCoverRegistry(t *testing.T) {
	types := buildSupportedTypes().ChartTypes
	for _, doc := range []struct {
		path, countExpression string
	}{
		{"README.md", `(\d+) chart types`},
		{"skills/template-deck/TEMPLATE_GUIDE.md", `Chart types\*\* \((\d+) types`},
	} {
		body := readRepoFile(t, filepath.FromSlash(doc.path))
		for _, kind := range types {
			if !docMentionsCode(body, kind) {
				t.Errorf("%s omits chart type %s", doc.path, kind)
			}
		}
		counts := regexp.MustCompile(doc.countExpression).FindAllStringSubmatch(body, -1)
		for _, count := range counts {
			n, err := strconv.Atoi(count[1])
			if err != nil || n != len(types) {
				t.Errorf("%s advertises %s chart types; live registry has %d", doc.path, count[1], len(types))
			}
		}
	}
}

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

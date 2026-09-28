package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

var updateDiagAppendix = flag.Bool("update-diag-appendix", false,
	"regenerate the undocumented-code appendix in docs/AGENT_DIAGNOSTICS.md")

const (
	diagAppendixDoc   = "docs/AGENT_DIAGNOSTICS.md"
	diagAppendixBegin = "<!-- BEGIN GENERATED: finding-code appendix (go test ./cmd/json2pptx -run TestFindingCatalogCodesAreDocumented -update-diag-appendix) -->"
	diagAppendixEnd   = "<!-- END GENERATED: finding-code appendix -->"
)

// findingDocRoots are the Markdown trees an agent or contributor reads to learn
// what a finding code means.
var findingDocRoots = []string{"docs", "skills"}

// TestFindingCatalogCodesAreDocumented mirrors the describe_finding catalogue
// into the docs (go-slide-creator-wizql): every code describe_finding resolves
// must be named in at least one doc under docs/ or skills/. Codes with no
// hand-written prose are listed in the generated appendix of
// docs/AGENT_DIAGNOSTICS.md; the appendix must not list codes that have left
// the catalogue.
func TestFindingCatalogCodesAreDocumented(t *testing.T) {
	catalog := diagnostics.AllDescribableCodes()
	if len(catalog) == 0 {
		t.Fatal("describe_finding catalogue is empty")
	}
	appendixPath := filepath.Join("..", "..", diagAppendixDoc)
	appendixDoc, err := os.ReadFile(appendixPath)
	if err != nil {
		t.Fatal(err)
	}
	before, generated, after, ok := splitDiagAppendix(string(appendixDoc))
	if !ok {
		t.Fatalf("%s is missing the generated appendix markers", diagAppendixDoc)
	}

	// Prose outside the generated block, across every doc tree.
	var prose strings.Builder
	prose.WriteString(before)
	prose.WriteString(after)
	for _, root := range findingDocRoots {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") || filepath.Clean(path) == filepath.Clean(appendixPath) {
				return nil
			}
			body, err := os.ReadFile(path) // #nosec G304 -- walking the repository's own doc trees.
			if err != nil {
				return err
			}
			prose.Write(body)
			prose.WriteByte('\n')
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	text := prose.String()

	var undocumented []string
	for _, code := range catalog {
		if !docMentionsCode(text, code) {
			undocumented = append(undocumented, code)
		}
	}

	want := renderDiagAppendix(t, undocumented)
	if *updateDiagAppendix {
		if err := os.WriteFile(appendixPath, []byte(before+want+after), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}

	var missing []string
	for _, code := range undocumented {
		if !docMentionsCode(generated, code) {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d describe_finding code(s) appear in no doc under docs/ or skills/: %s\n"+
			"Document them where they belong (docs/FIT_FINDINGS.md for fit codes) or regenerate the appendix:\n"+
			"  go test ./cmd/json2pptx -run TestFindingCatalogCodesAreDocumented -update-diag-appendix",
			len(missing), strings.Join(missing, ", "))
	}

	inCatalog := map[string]bool{}
	for _, code := range catalog {
		inCatalog[code] = true
	}
	for _, m := range diagAppendixRowRE.FindAllStringSubmatch(generated, -1) {
		if !inCatalog[m[1]] {
			t.Errorf("%s appendix lists %q, which describe_finding no longer resolves; regenerate the appendix", diagAppendixDoc, m[1])
		}
	}
}

var diagAppendixRowRE = regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")

func splitDiagAppendix(doc string) (before, generated, after string, ok bool) {
	i := strings.Index(doc, diagAppendixBegin)
	j := strings.Index(doc, diagAppendixEnd)
	if i < 0 || j < i {
		return "", "", "", false
	}
	return doc[:i], doc[i : j+len(diagAppendixEnd)], doc[j+len(diagAppendixEnd):], true
}

// docMentionsCode reports whether text names code as a whole token, so
// CANCELLED is not satisfied by "…_CANCELLED" or "cancelled".
func docMentionsCode(text, code string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_.])` + regexp.QuoteMeta(code) + `($|[^A-Za-z0-9_])`)
	return re.MatchString(text)
}

func renderDiagAppendix(t *testing.T, codes []string) string {
	t.Helper()
	sort.Strings(codes)
	var b strings.Builder
	b.WriteString(diagAppendixBegin + "\n\n")
	b.WriteString("| Code | Severity | Summary |\n|------|----------|---------|\n")
	for _, code := range codes {
		meta, ok := diagnostics.Describe(code)
		if !ok {
			t.Fatalf("catalogue code %q does not resolve", code)
		}
		summary := strings.ReplaceAll(strings.TrimSpace(meta.Summary), "|", `\|`)
		summary = strings.ReplaceAll(summary, "\n", " ")
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", code, meta.Severity, summary)
	}
	b.WriteString("\n" + diagAppendixEnd)
	return b.String()
}

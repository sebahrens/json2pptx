package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-7zrt: describe_finding claims to cover "every code emitted
// across the pipeline", but design_mode_violation — the one code that actually
// blocked generation — returned UNKNOWN_FINDING_CODE, because it was written as
// a bare string literal in cmd/json2pptx/design_mode.go and never added to the
// catalogue. This test closes that class of gap: every code STRING LITERAL
// assigned to a Code field under cmd/ must be describable.
func TestDescribeFindingCoversCodesEmittedInCmd(t *testing.T) {
	// Matches `Code: "some_code"` and `Code:    "SOME_CODE"` in Go source.
	codeLiteral := regexp.MustCompile(`\bCode:\s*"([A-Za-z][A-Za-z0-9_.]*)"`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	found := map[string][]string{} // code -> files
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range codeLiteral.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = append(found[m[1]], e.Name())
		}
	}
	if len(found) == 0 {
		t.Fatal("no Code string literals found — the scan regex has probably drifted")
	}

	var missing []string
	for code, files := range found {
		if _, ok := diagnostics.Describe(code); !ok {
			missing = append(missing, code+" (emitted in "+strings.Join(uniqueStrings(files), ", ")+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("describe_finding cannot resolve %d code(s) emitted in cmd/json2pptx; add them to the finding-meta or diagnostics catalogue:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// The two design-mode codes are the concrete regression: both must resolve and
// carry actionable remediation naming the design_mode field.
func TestDescribeFinding_DesignModeCodes(t *testing.T) {
	for _, code := range []string{"design_mode_violation", "CUSTOM_COLOR_DROPPED"} {
		t.Run(code, func(t *testing.T) {
			meta, ok := diagnostics.Describe(code)
			if !ok {
				t.Fatalf("describe_finding cannot resolve %q", code)
			}
			if meta.Summary == "" {
				t.Error("summary is empty")
			}
			if len(meta.RemediationSteps) == 0 {
				t.Fatal("no remediation steps")
			}
			joined := strings.Join(meta.RemediationSteps, " ")
			if !strings.Contains(joined, "design_mode") {
				t.Errorf("remediation should name the design_mode field, got: %s", joined)
			}
		})
	}

	// The namespaced form an agent copies out of a finding envelope must work too.
	if _, ok := diagnostics.Describe("FIT.design_mode_violation"); !ok {
		t.Error("namespaced code FIT.design_mode_violation should resolve")
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

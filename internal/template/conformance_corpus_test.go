package template_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// templatesDir is the canonical location of the shipped templates, relative
// to this test file (internal/template/conformance_corpus_test.go).
const templatesDir = "../../templates"

// allowlistPath is the path to the JSON allow-list relative to this test.
const allowlistPath = "testdata/conformance_allowlist.json"

// conformanceAllowlist mirrors the on-disk JSON schema in
// testdata/conformance_allowlist.json.
type conformanceAllowlist struct {
	SchemaVersion int `json:"$schema_version"`
	Comment       string
	Exceptions    []conformanceAllowlistEntry `json:"exceptions"`
}

// conformanceAllowlistEntry is a single allow-listed template.
type conformanceAllowlistEntry struct {
	Template      string `json:"template"`
	SHA256        string `json:"sha256"`
	TrackingIssue string `json:"tracking_issue"`
	Reason        string `json:"reason"`
}

// TestConformanceCorpus_TemplatesPassWithoutFailOrWarn iterates every
// templates/*.pptx file in the repository and asserts that
// template.CheckConformance returns zero FAIL and zero WARN entries.
//
// Templates whose sha256 appears in testdata/conformance_allowlist.json are
// exempt from the gate. Repairing such a template changes its sha256, which
// automatically drops the allow-list entry and re-enables enforcement.
//
// Acceptance criteria source: bd go-slide-creator-aom6.
func TestConformanceCorpus_TemplatesPassWithoutFailOrWarn(t *testing.T) {
	allowlist := loadAllowlist(t)
	allowed := make(map[string]conformanceAllowlistEntry, len(allowlist.Exceptions))
	for _, e := range allowlist.Exceptions {
		if e.SHA256 == "" {
			t.Fatalf("allow-list entry for %s is missing sha256", e.Template)
		}
		if e.TrackingIssue == "" {
			t.Fatalf("allow-list entry for %s is missing tracking_issue", e.Template)
		}
		allowed[strings.ToLower(e.SHA256)] = e
	}

	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)

	for _, file := range files {
		file := file
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			report, err := template.CheckConformance(file)
			if err != nil {
				t.Fatalf("CheckConformance(%s): %v", name, err)
			}

			fail := failedChecks(report)
			warn := warnChecks(report)

			if entry, ok := allowed[strings.ToLower(report.SHA256)]; ok {
				if entry.Template != name {
					t.Errorf(
						"allow-list entry %s names template %q but matched %q via sha256 %s — fix the allow-list entry",
						entry.TrackingIssue, entry.Template, name, report.SHA256,
					)
				}
				// Allow-listed: log the known failures so they remain visible
				// in CI output, but do not fail the test. Once the template is
				// repaired its sha256 changes and the gate kicks back in.
				if len(fail) == 0 && len(warn) == 0 {
					t.Errorf(
						"%s is on the allow-list (tracking %s) but now has zero FAIL and zero WARN — remove the allow-list entry",
						name, entry.TrackingIssue,
					)
				}
				t.Logf(
					"%s allow-listed under %s: %d FAIL, %d WARN (sha256=%s)",
					name, entry.TrackingIssue, len(fail), len(warn), report.SHA256,
				)
				return
			}

			if len(fail) > 0 || len(warn) > 0 {
				t.Errorf(
					"template %s is not on the conformance allow-list and has %d FAIL + %d WARN (sha256=%s).\n"+
						"  Either repair the template, or add an exception to %s with sha256=%s and a tracking issue.\n%s",
					name, len(fail), len(warn), report.SHA256,
					allowlistPath, report.SHA256,
					formatChecks(fail, warn),
				)
			}
		})
	}
}

// TestConformanceCorpus_AllowlistEntriesPointAtRealTemplates makes sure every
// allow-list entry matches a current templates/*.pptx file by sha256. A
// stale entry (template repaired or removed) means the allow-list is lying
// about what is currently broken.
func TestConformanceCorpus_AllowlistEntriesPointAtRealTemplates(t *testing.T) {
	allowlist := loadAllowlist(t)
	if len(allowlist.Exceptions) == 0 {
		t.Skip("allow-list is empty — nothing to validate")
	}

	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}

	hashes := make(map[string]string, len(files))
	for _, file := range files {
		report, err := template.CheckConformance(file)
		if err != nil {
			t.Fatalf("CheckConformance(%s): %v", file, err)
		}
		hashes[strings.ToLower(report.SHA256)] = filepath.Base(file)
	}

	for _, entry := range allowlist.Exceptions {
		key := strings.ToLower(entry.SHA256)
		got, ok := hashes[key]
		if !ok {
			t.Errorf(
				"stale allow-list entry: %s (sha256 %s) does not match any current template; remove it from %s",
				entry.Template, entry.SHA256, allowlistPath,
			)
			continue
		}
		if got != entry.Template {
			t.Errorf(
				"allow-list entry names %s but sha256 %s now belongs to %s; update %s",
				entry.Template, entry.SHA256, got, allowlistPath,
			)
		}
	}
}

func loadAllowlist(t *testing.T) conformanceAllowlist {
	t.Helper()
	data, err := os.ReadFile(allowlistPath)
	if err != nil {
		t.Fatalf("read allow-list %s: %v", allowlistPath, err)
	}
	var al conformanceAllowlist
	if err := json.Unmarshal(data, &al); err != nil {
		t.Fatalf("parse allow-list %s: %v", allowlistPath, err)
	}
	return al
}

func failedChecks(report *template.ConformanceReport) []template.ConformanceCheck {
	var out []template.ConformanceCheck
	for _, c := range report.Checks {
		if c.Status == template.ConformanceStatusFail {
			out = append(out, c)
		}
	}
	return out
}

func warnChecks(report *template.ConformanceReport) []template.ConformanceCheck {
	var out []template.ConformanceCheck
	for _, c := range report.Checks {
		if c.Status == template.ConformanceStatusWarn {
			out = append(out, c)
		}
	}
	return out
}

func formatChecks(fail, warn []template.ConformanceCheck) string {
	var b strings.Builder
	for _, c := range fail {
		fmt.Fprintf(&b, "  [FAIL] %s — %s\n", c.Check, c.Detail)
	}
	for _, c := range warn {
		fmt.Fprintf(&b, "  [WARN] %s — %s\n", c.Check, c.Detail)
	}
	return b.String()
}

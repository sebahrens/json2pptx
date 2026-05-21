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
	"github.com/sebahrens/json2pptx/internal/types"
)

// canonicalAllowlistPath is the path to the JSON allow-list relative to this
// test file. Templates whose sha256 appears here are exempted from the
// canonical-placeholder gate. Repairing a template changes its sha256, which
// automatically drops the allow-list entry and re-enables enforcement.
const canonicalAllowlistPath = "testdata/canonical_placeholders_allowlist.json"

// expectedCanonicalPlaceholder describes a canonical placeholder that MUST
// resolve on a mandatory layout. The reference for this table is
// docs/TEMPLATE_SPEC.md "Canonical Placeholder Names" + "Mandatory Layouts".
type expectedCanonicalPlaceholder struct {
	Name   string                // canonical name (matched case-insensitively against PlaceholderInfo.ID)
	Type   types.PlaceholderType // expected resolved type after normalization
	Idx    int                   // expected idx; 0 means "do not enforce"
	IdxAny bool                  // when true, accept any idx (used for Section Number)
}

// canonicalPlaceholderExpectations is the per-role expectation table. Each
// entry mirrors the placeholders listed for that mandatory layout in
// docs/TEMPLATE_SPEC.md. Blank intentionally has no required placeholders.
var canonicalPlaceholderExpectations = map[string][]expectedCanonicalPlaceholder{
	template.CanonicalRoleTitleSlide: {
		{Name: "title", Type: types.PlaceholderTitle},
		{Name: "subtitle", Type: types.PlaceholderSubtitle},
	},
	template.CanonicalRoleOneContent: {
		{Name: "title", Type: types.PlaceholderTitle},
		{Name: "body", Type: types.PlaceholderBody, Idx: 1},
	},
	template.CanonicalRoleTwoContent: {
		{Name: "title", Type: types.PlaceholderTitle},
		{Name: "body", Type: types.PlaceholderBody, Idx: 1},
		{Name: "body_2", Type: types.PlaceholderBody, Idx: 2},
	},
	template.CanonicalRoleSectionDivider: {
		{Name: "title", Type: types.PlaceholderTitle},
		{Name: "Section Number", Type: types.PlaceholderBody, IdxAny: true},
	},
	template.CanonicalRoleBlank: {},
	template.CanonicalRoleBlankTitle: {
		{Name: "title", Type: types.PlaceholderTitle},
	},
	template.CanonicalRoleClosing: {
		{Name: "title", Type: types.PlaceholderTitle},
		{Name: "subtitle", Type: types.PlaceholderSubtitle},
	},
}

// TestCanonicalPlaceholdersCorpus iterates every templates/*.pptx file and
// asserts that, after placeholder normalization, every canonical placeholder
// named in docs/TEMPLATE_SPEC.md resolves on its mandatory layout with the
// correct type and idx.
//
// Templates whose sha256 appears in testdata/canonical_placeholders_allowlist.json
// are exempt from the gate -- their failures are reported via t.Log so they
// remain visible in CI output, but do not fail the test. Once the template is
// repaired its sha256 changes and the gate kicks back in.
//
// Acceptance criteria source: bd go-slide-creator-z3o1.
func TestCanonicalPlaceholdersCorpus(t *testing.T) {
	allowlist := loadCanonicalAllowlist(t)
	allowed := make(map[string]canonicalAllowlistEntry, len(allowlist.Exceptions))
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
			reader, err := template.OpenTemplate(file)
			if err != nil {
				t.Fatalf("OpenTemplate(%s): %v", name, err)
			}
			defer func() { _ = reader.Close() }()

			layouts, err := template.ParseLayouts(reader)
			if err != nil {
				t.Fatalf("ParseLayouts(%s): %v", name, err)
			}
			// Apply canonical-name normalization in place. The returned synthetic
			// file map is not needed here -- the in-place rewrite of
			// layouts[i].Placeholders[j].ID is what this test exercises.
			if _, err := template.NormalizeLayoutFiles(reader, layouts); err != nil {
				t.Fatalf("NormalizeLayoutFiles(%s): %v", name, err)
			}

			failures := collectCanonicalPlaceholderFailures(layouts, name)

			sha := strings.ToLower(reader.Hash())
			if entry, ok := allowed[sha]; ok {
				if entry.Template != name {
					t.Errorf(
						"allow-list entry %s names template %q but matched %q via sha256 %s — fix the allow-list entry",
						entry.TrackingIssue, entry.Template, name, sha,
					)
				}
				if len(failures) == 0 {
					t.Errorf(
						"%s is on the canonical-placeholder allow-list (tracking %s) but every canonical placeholder now resolves — remove the allow-list entry",
						name, entry.TrackingIssue,
					)
				}
				for _, f := range failures {
					t.Logf("%s allow-listed under %s: %s", name, entry.TrackingIssue, f)
				}
				return
			}

			if len(failures) > 0 {
				t.Errorf(
					"template %s is not on the canonical-placeholder allow-list and has %d failure(s) (sha256=%s).\n"+
						"  Either repair the template, or add an exception to %s with sha256=%s and a tracking issue.\n  %s",
					name, len(failures), sha,
					canonicalAllowlistPath, sha,
					strings.Join(failures, "\n  "),
				)
			}
		})
	}
}

// collectCanonicalPlaceholderFailures walks every mandatory layout and returns
// human-readable failure strings for every missing/mismatched canonical
// placeholder. Format matches the AC: "template <name> layout <name>: missing
// canonical placeholder <name> (raw layout has: [list])".
func collectCanonicalPlaceholderFailures(layouts []types.LayoutMetadata, templateName string) []string {
	var failures []string

	for _, ml := range template.MandatoryLayouts {
		expected, ok := canonicalPlaceholderExpectations[ml.Role]
		if !ok {
			continue
		}
		layout, found := findCanonicalLayout(layouts, ml)
		if !found {
			// Blank layouts have no required placeholders, but we still want
			// to flag a missing Blank layout because the canonical-name
			// resolution gate cannot succeed for a layout that does not exist.
			failures = append(failures, fmt.Sprintf(
				"template %s layout %s: layout missing entirely (no name/tag/canonical-role match)",
				templateName, ml.Role,
			))
			continue
		}
		if len(expected) == 0 {
			continue
		}

		idIndex := make(map[string]types.PlaceholderInfo, len(layout.Placeholders))
		rawIDs := make([]string, 0, len(layout.Placeholders))
		for _, ph := range layout.Placeholders {
			idIndex[strings.ToLower(ph.ID)] = ph
			rawIDs = append(rawIDs, ph.ID)
		}
		sort.Strings(rawIDs)

		for _, exp := range expected {
			ph, present := idIndex[strings.ToLower(exp.Name)]
			if !present {
				failures = append(failures, fmt.Sprintf(
					"template %s layout %s: missing canonical placeholder %s (raw layout has: %v)",
					templateName, layout.Name, exp.Name, rawIDs,
				))
				continue
			}
			if ph.Type != exp.Type {
				failures = append(failures, fmt.Sprintf(
					"template %s layout %s: canonical placeholder %s has type %q, want %q (raw layout has: %v)",
					templateName, layout.Name, exp.Name, ph.Type, exp.Type, rawIDs,
				))
			}
			if !exp.IdxAny && exp.Idx != 0 && ph.Index != exp.Idx {
				failures = append(failures, fmt.Sprintf(
					"template %s layout %s: canonical placeholder %s has idx %d, want %d (raw layout has: %v)",
					templateName, layout.Name, exp.Name, ph.Index, exp.Idx, rawIDs,
				))
			}
		}
	}

	return failures
}

// findCanonicalLayout mirrors the unexported findMandatoryLayout fallback chain
// (name → tag → canonical-role classification) using only the exported API. It
// must stay in lockstep with the production resolver so this test fails for
// the same reasons template-check fails.
func findCanonicalLayout(layouts []types.LayoutMetadata, ml template.MandatoryLayout) (types.LayoutMetadata, bool) {
	for _, l := range layouts {
		lower := strings.ToLower(l.Name)
		for _, n := range ml.Names {
			if lower == n {
				return l, true
			}
		}
	}
	for _, l := range layouts {
		for _, req := range ml.Tags {
			for _, lt := range l.Tags {
				if lt == req {
					return l, true
				}
			}
		}
	}
	for i := range layouts {
		l := &layouts[i]
		role, _, conf := template.ClassifyCanonicalRole(l)
		if role == ml.Role && conf >= template.CanonicalConfidenceThreshold {
			return *l, true
		}
	}
	return types.LayoutMetadata{}, false
}

// canonicalAllowlist mirrors the on-disk JSON schema for the canonical
// placeholder allow-list. The schema is intentionally the same shape as
// conformance_allowlist.json so the two tests can share tooling.
type canonicalAllowlist struct {
	SchemaVersion int                       `json:"$schema_version"`
	Comment       string                    `json:"comment,omitempty"`
	Exceptions    []canonicalAllowlistEntry `json:"exceptions"`
}

type canonicalAllowlistEntry struct {
	Template      string `json:"template"`
	SHA256        string `json:"sha256"`
	TrackingIssue string `json:"tracking_issue"`
	Reason        string `json:"reason"`
}

func loadCanonicalAllowlist(t *testing.T) canonicalAllowlist {
	t.Helper()
	data, err := os.ReadFile(canonicalAllowlistPath)
	if err != nil {
		t.Fatalf("read allow-list %s: %v", canonicalAllowlistPath, err)
	}
	var al canonicalAllowlist
	if err := json.Unmarshal(data, &al); err != nil {
		t.Fatalf("parse allow-list %s: %v", canonicalAllowlistPath, err)
	}
	return al
}

// TestCanonicalPlaceholdersCorpus_AllowlistEntriesPointAtRealTemplates makes
// sure every allow-list entry matches a current templates/*.pptx file by
// sha256. A stale entry (template repaired or removed) means the allow-list is
// lying about what is currently broken.
func TestCanonicalPlaceholdersCorpus_AllowlistEntriesPointAtRealTemplates(t *testing.T) {
	al := loadCanonicalAllowlist(t)
	if len(al.Exceptions) == 0 {
		t.Skip("allow-list is empty — nothing to validate")
	}

	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}

	hashes := make(map[string]string, len(files))
	for _, f := range files {
		r, err := template.OpenTemplate(f)
		if err != nil {
			t.Fatalf("OpenTemplate(%s): %v", f, err)
		}
		hashes[strings.ToLower(r.Hash())] = filepath.Base(f)
		_ = r.Close()
	}

	for _, entry := range al.Exceptions {
		key := strings.ToLower(entry.SHA256)
		got, ok := hashes[key]
		if !ok {
			t.Errorf(
				"stale allow-list entry: %s (sha256 %s) does not match any current template; remove it from %s",
				entry.Template, entry.SHA256, canonicalAllowlistPath,
			)
			continue
		}
		if got != entry.Template {
			t.Errorf(
				"allow-list entry names %s but sha256 %s now belongs to %s; update %s",
				entry.Template, entry.SHA256, got, canonicalAllowlistPath,
			)
		}
	}
}

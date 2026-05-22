package testutil

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// TemplateTier classifies how thoroughly a shipped built-in template is
// exercised by the test suite. The tier controls which corpus a cross-template
// test iterates over so that expensive matrices stay bounded while every
// shipped template still receives at least smoke coverage.
type TemplateTier int

const (
	// TierCore templates are the curated, designer-owned built-ins that the
	// full (expensive) cross-template matrices iterate over in addition to the
	// cheap smoke/load tests.
	TierCore TemplateTier = iota
	// TierSmoke templates are shipped built-ins that are exercised only by the
	// cheap load/smoke tests. They are deliberately excluded from the expensive
	// fixture matrices to keep CI runtime bounded; the Reason field on the
	// classification entry records why.
	TierSmoke
)

// templateCoverage records the coverage decision for a single built-in template.
type templateCoverage struct {
	tier   TemplateTier
	reason string
}

// builtinTemplateCoverage is the authoritative classification of every template
// shipped under templates/. It is the single source of truth that the
// cross-template tests and the coverage guard share.
//
// Keep this map in sync with templates/*.pptx: adding a new *.pptx there
// without a matching entry here (or removing one without deleting its entry)
// makes TestBuiltinTemplateCoverage fail by design, forcing an explicit
// coverage decision for every shipped template. The reason field documents why
// a TierSmoke template is excluded from the expensive fixture matrices.
var builtinTemplateCoverage = map[string]templateCoverage{
	"forest-green":    {tier: TierCore},
	"midnight-blue":   {tier: TierCore},
	"modern-template": {tier: TierCore},
	"warm-coral":      {tier: TierCore},

	"abstract":          {tier: TierSmoke, reason: "designer-owned built-in; covered by template load/smoke tests, excluded from expensive fixture matrices to bound CI runtime (go-slide-creator-0oi3)"},
	"blue-corporate":    {tier: TierSmoke, reason: "designer-owned built-in; covered by template load/smoke tests, excluded from expensive fixture matrices to bound CI runtime (go-slide-creator-0oi3)"},
	"business-template": {tier: TierSmoke, reason: "designer-owned built-in; covered by template load/smoke tests, excluded from expensive fixture matrices to bound CI runtime (go-slide-creator-0oi3)"},
	"modern":            {tier: TierSmoke, reason: "designer-owned built-in; covered by template load/smoke tests, excluded from expensive fixture matrices to bound CI runtime (go-slide-creator-0oi3)"},
	"modern-yellow":     {tier: TierSmoke, reason: "designer-owned built-in; covered by template load/smoke tests, excluded from expensive fixture matrices to bound CI runtime (go-slide-creator-0oi3)"},
}

// RepoRoot returns the absolute path to the repository root, resolved relative
// to this helper's own source location. Using runtime.Caller (rather than the
// test's working directory) lets the helper return a correct path from any
// package's tests without each caller hard-coding "../.." hops.
func RepoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// thisFile == <root>/internal/testutil/templates.go
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// TemplatesDir returns the absolute path to the bundled templates directory.
func TemplatesDir() string {
	return filepath.Join(RepoRoot(), "templates")
}

// CoreTemplateNames returns the names (without the .pptx extension) of the
// TierCore built-in templates, sorted for deterministic subtest ordering. These
// are the templates expensive cross-template matrices should iterate over.
func CoreTemplateNames() []string {
	return templateNamesByTier(TierCore)
}

// AllBuiltinTemplateNames returns the names (without the .pptx extension) of
// every classified built-in template, sorted. Use this for cheap smoke/load
// tests that should exercise the full shipped corpus.
func AllBuiltinTemplateNames() []string {
	names := make([]string, 0, len(builtinTemplateCoverage))
	for name := range builtinTemplateCoverage {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func templateNamesByTier(tier TemplateTier) []string {
	var names []string
	for name, cov := range builtinTemplateCoverage {
		if cov.tier == tier {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// DiscoverBuiltinTemplates returns the base names (without the .pptx extension)
// of every template file actually present under templates/, sorted. It is used
// by the coverage guard to detect templates that are shipped but unclassified.
//
// Inclusion/exclusion rules: only real *.pptx files are returned. Editor and
// office-suite scratch files are skipped — PowerPoint owner-lock files
// (~$name.pptx) and dotfile/underscore-prefixed names — so a stray lock file
// left behind by an open editor never registers as an "unclassified template"
// or, worse, breaks the corpus. These are the same names go:embed rejects.
func DiscoverBuiltinTemplates() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(TemplatesDir(), "*.pptx"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		base := filepath.Base(m)
		if isScratchFile(base) {
			continue
		}
		names = append(names, strings.TrimSuffix(base, ".pptx"))
	}
	sort.Strings(names)
	return names, nil
}

// isScratchFile reports whether base looks like an editor/office-suite scratch
// file rather than a real shipped template.
func isScratchFile(base string) bool {
	return strings.HasPrefix(base, "~$") ||
		strings.HasPrefix(base, ".") ||
		strings.HasPrefix(base, "_")
}

// VerifyBuiltinTemplateCoverage checks that the on-disk template corpus and the
// builtinTemplateCoverage classification agree: every shipped template must be
// classified, every classified template must exist on disk, and every TierSmoke
// template must record a non-empty reason for its exclusion from the expensive
// matrices. It returns a slice of human-readable problems (empty when healthy)
// so callers in any test package can assert on the result.
func VerifyBuiltinTemplateCoverage() ([]string, error) {
	discovered, err := DiscoverBuiltinTemplates()
	if err != nil {
		return nil, err
	}

	var problems []string

	classified := make(map[string]bool, len(builtinTemplateCoverage))
	for name := range builtinTemplateCoverage {
		classified[name] = true
	}

	for _, name := range discovered {
		if !classified[name] {
			problems = append(problems, fmt.Sprintf(
				"template %q is shipped under templates/ but has no coverage classification; add it to builtinTemplateCoverage (TierCore for full-matrix coverage, or TierSmoke with a reason)",
				name))
		}
	}

	onDisk := make(map[string]bool, len(discovered))
	for _, name := range discovered {
		onDisk[name] = true
	}
	for name, cov := range builtinTemplateCoverage {
		if !onDisk[name] {
			problems = append(problems, fmt.Sprintf(
				"template %q is classified in builtinTemplateCoverage but no templates/%s.pptx exists; remove or rename its entry",
				name, name))
		}
		if cov.tier == TierSmoke && strings.TrimSpace(cov.reason) == "" {
			problems = append(problems, fmt.Sprintf(
				"template %q is TierSmoke but records no reason; document why it is excluded from the expensive matrices",
				name))
		}
	}

	sort.Strings(problems)
	return problems, nil
}

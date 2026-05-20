package patterns

import (
	"sort"
	"strings"
	"testing"
)

// allowedSeverities mirrors fit_findings[].action ranks. The describe_finding
// output schema enumerates these — keep this list in sync.
var allowedSeverities = map[string]bool{
	"refuse":          true,
	"shrink_or_split": true,
	"review":          true,
	"info":            true,
}

// TestFindingMetaCoversAllSentinelCodes asserts that every code listed in
// AllFitFindingCodes() has a metadata entry. This is the drift guarantee:
// adding a new ErrCode* constant without a metadata entry fails the build.
func TestFindingMetaCoversAllSentinelCodes(t *testing.T) {
	for _, code := range AllFitFindingCodes() {
		if _, ok := GetFindingMeta(code); !ok {
			t.Errorf("finding code %q is in AllFitFindingCodes() but missing from findingMetaRegistry — add an entry to finding_meta.go", code)
		}
	}
}

// TestFindingMetaWellFormed asserts each entry has the required fields and a
// recognized severity. This catches typos in severity strings that would
// otherwise pass the schema check at runtime.
func TestFindingMetaWellFormed(t *testing.T) {
	for _, code := range AllFindingMetaCodes() {
		meta, ok := GetFindingMeta(code)
		if !ok {
			t.Errorf("AllFindingMetaCodes() listed %q but GetFindingMeta did not return it", code)
			continue
		}
		if meta.Code != code {
			t.Errorf("entry for %q: meta.Code = %q (must echo the registry key)", code, meta.Code)
		}
		if strings.TrimSpace(meta.Summary) == "" {
			t.Errorf("entry for %q: summary is empty", code)
		}
		if !allowedSeverities[meta.Severity] {
			t.Errorf("entry for %q: severity %q is not one of refuse/shrink_or_split/review/info", code, meta.Severity)
		}
		if strings.TrimSpace(meta.WhenEmitted) == "" {
			t.Errorf("entry for %q: when_emitted is empty", code)
		}
		if len(meta.RemediationSteps) == 0 {
			t.Errorf("entry for %q: remediation_steps must have at least one entry", code)
		}
		for i, step := range meta.RemediationSteps {
			if strings.TrimSpace(step) == "" {
				t.Errorf("entry for %q: remediation_steps[%d] is empty", code, i)
			}
		}
	}
}

// TestFindingMetaRelatedCodesExist asserts that every code listed in
// related_codes resolves to a registry entry. This prevents the registry
// from accidentally pointing at codes that were renamed or removed.
func TestFindingMetaRelatedCodesExist(t *testing.T) {
	known := make(map[string]bool, len(findingMetaRegistry))
	for code := range findingMetaRegistry {
		known[code] = true
	}
	for _, code := range AllFindingMetaCodes() {
		meta, ok := GetFindingMeta(code)
		if !ok {
			continue
		}
		for _, rel := range meta.RelatedCodes {
			if !known[rel] {
				t.Errorf("entry for %q: related_codes references unknown code %q", code, rel)
			}
		}
	}
}

// TestAllFindingMetaCodesSorted documents that the helper returns codes in
// sorted order so callers can rely on stable ordering for diffing.
func TestAllFindingMetaCodesSorted(t *testing.T) {
	got := AllFindingMetaCodes()
	want := append([]string(nil), got...)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("AllFindingMetaCodes() not sorted at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

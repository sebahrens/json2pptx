package diagnostics

import (
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// describeSeverities mirrors the describe_finding output schema severity enum.
var describeSeverities = map[string]bool{
	"refuse":          true,
	"shrink_or_split": true,
	"review":          true,
	"info":            true,
}

// TestDescribeCoversAllDiagnosticCodes is the drift gate: every code declared in
// codes.go (AllCodes) must resolve through Describe. Adding a code to the
// taxonomy without a describe entry — here or in the patterns registry — fails
// the build, keeping describe-finding the single source of truth for diagnostics.
func TestDescribeCoversAllDiagnosticCodes(t *testing.T) {
	for _, code := range AllCodes() {
		if _, ok := Describe(code); !ok {
			t.Errorf("diagnostics code %q is in AllCodes() but Describe() cannot resolve it — add an entry to codeMetaRegistry in describe.go", code)
		}
	}
}

// TestDescribeRegistryWellFormed asserts each diagnostics-side entry has the
// required fields and a recognized severity.
func TestDescribeRegistryWellFormed(t *testing.T) {
	for code, meta := range codeMetaRegistry {
		if meta.Code != code {
			t.Errorf("entry for %q: meta.Code = %q (must echo the registry key)", code, meta.Code)
		}
		if strings.TrimSpace(meta.Summary) == "" {
			t.Errorf("entry for %q: summary is empty", code)
		}
		if !describeSeverities[meta.Severity] {
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

// TestDescribeRelatedCodesResolve asserts every related code in a diagnostics
// entry resolves through Describe, so the registry never points at a renamed or
// removed code.
func TestDescribeRelatedCodesResolve(t *testing.T) {
	for code, meta := range codeMetaRegistry {
		for _, rel := range meta.RelatedCodes {
			if _, ok := Describe(rel); !ok {
				t.Errorf("entry for %q: related code %q does not resolve through Describe", code, rel)
			}
		}
	}
}

// TestDescribeStripsNamespacePrefix asserts the dotted namespaced codes carried
// in finding envelopes resolve to the same metadata as the bare legacy code, so
// a finding's describe_command runs verbatim.
func TestDescribeStripsNamespacePrefix(t *testing.T) {
	cases := []struct {
		dotted string
		legacy string
	}{
		{DottedCode(NamespaceInput, CodeMissingParameter), CodeMissingParameter},
		{DottedCode(NamespaceTemplate, CodeTemplateNotFound), CodeTemplateNotFound},
		{DottedCode(NamespaceRender, CodeRenderFailed), CodeRenderFailed},
		{DottedCode(NamespaceFit, patterns.ErrCodePlaceholderOverflow), patterns.ErrCodePlaceholderOverflow},
	}
	for _, tc := range cases {
		got, ok := Describe(tc.dotted)
		if !ok {
			t.Errorf("Describe(%q) failed to resolve", tc.dotted)
			continue
		}
		if got.Code != tc.legacy {
			t.Errorf("Describe(%q).Code = %q, want %q", tc.dotted, got.Code, tc.legacy)
		}
	}
}

// TestDescribeChartDottedCodePreserved guards that non-namespace dotted prefixes
// (chart.*) are not mistaken for a namespace and stripped — they reach the
// patterns registry intact.
func TestDescribeChartDottedCodePreserved(t *testing.T) {
	const chartCode = "chart.zero_sum_pie"
	got, ok := Describe(chartCode)
	if !ok {
		t.Fatalf("Describe(%q) failed to resolve", chartCode)
	}
	if got.Code != chartCode {
		t.Errorf("Describe(%q).Code = %q, want %q", chartCode, got.Code, chartCode)
	}
}

// TestAllDescribableCodesSorted documents that the helper returns codes in
// sorted order and includes both registries.
func TestAllDescribableCodesSorted(t *testing.T) {
	got := AllDescribableCodes()
	want := append([]string(nil), got...)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("AllDescribableCodes() not sorted at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// Must include a patterns-owned code and a diagnostics-owned code.
	set := map[string]bool{}
	for _, c := range got {
		set[c] = true
	}
	if !set[patterns.ErrCodePlaceholderOverflow] {
		t.Errorf("AllDescribableCodes() missing patterns code %q", patterns.ErrCodePlaceholderOverflow)
	}
	if !set[CodeMissingParameter] {
		t.Errorf("AllDescribableCodes() missing diagnostics code %q", CodeMissingParameter)
	}
}

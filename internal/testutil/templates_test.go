package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuiltinTemplateCoverage is the guard that keeps the shared template
// classification honest. It fails when a template is shipped under templates/
// without a coverage decision (or vice versa), forcing every new built-in to be
// classified as TierCore (full-matrix coverage) or TierSmoke (load/smoke
// coverage with a documented reason) before it can land.
func TestBuiltinTemplateCoverage(t *testing.T) {
	problems, err := VerifyBuiltinTemplateCoverage()
	if err != nil {
		t.Fatalf("VerifyBuiltinTemplateCoverage: %v", err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// TestCoreTemplateNames asserts the helper returns a non-empty, on-disk corpus
// so callers that iterate CoreTemplateNames() never silently run zero subtests.
func TestCoreTemplateNames(t *testing.T) {
	core := CoreTemplateNames()
	if len(core) == 0 {
		t.Fatal("CoreTemplateNames() returned no templates")
	}
	for _, name := range core {
		path := filepath.Join(TemplatesDir(), name+".pptx")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("core template %q not found on disk: %v", name, err)
		}
	}
}

// TestAllBuiltinTemplateNamesSupersetOfCore asserts every core template is also
// part of the full corpus, so smoke coverage never omits a fully-exercised one.
func TestAllBuiltinTemplateNamesSupersetOfCore(t *testing.T) {
	all := make(map[string]bool)
	for _, name := range AllBuiltinTemplateNames() {
		all[name] = true
	}
	for _, name := range CoreTemplateNames() {
		if !all[name] {
			t.Errorf("core template %q missing from AllBuiltinTemplateNames()", name)
		}
	}
}

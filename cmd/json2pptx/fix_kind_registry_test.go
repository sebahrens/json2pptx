package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// fixLiteralRE matches a fix-suggestion literal with an inline kind:
//
//	&patterns.FixSuggestion{Kind: "reduce_text", …}
//	&diagnostics.Fix{Kind: "swap_layout", …}
//	Fix: &FixSuggestion{
//		Kind:   "review",
//
// It deliberately anchors on the Fix type name so unrelated Kind fields (shape
// kinds, image kinds, chart kinds) do not enter the vocabulary check.
var fixLiteralRE = regexp.MustCompile(`(?:patterns\.|diagnostics\.)?(?:FixSuggestion|Fix)\{\s*(?:\n\s*)?Kind:\s*"([a-z_]+)"`)

// TestEveryEmittedFixKindIsRegistered is the go-slide-creator-ui4c guard: a
// finding may only name a fix kind the vocabulary knows. Before this, 506 of 814
// fix-carrying findings named kinds repair_slide could not apply and nothing
// caught it — repair_slide answered "kind_not_supported" and propose_repairs
// filed them as unmapped, so the documented repair loop terminated with zero
// directives on exactly the decks that were nearly good.
//
// A new kind must be added to internal/patterns/fix_kinds.go as either
// executable (with a case in applyRepairFix) or advisory (with guidance).
func TestEveryEmittedFixKindIsRegistered(t *testing.T) {
	emitted := emittedFixKinds(t)
	if len(emitted) < 20 {
		t.Fatalf("scanned only %d fix kinds — the scanner is probably broken, not the code", len(emitted))
	}
	for kind, files := range emitted {
		if !patterns.KnownFixKind(kind) {
			t.Errorf("fix kind %q is emitted by %s but is not registered in internal/patterns/fix_kinds.go — add it as executable (with an applyRepairFix case) or advisory (with guidance)",
				kind, strings.Join(files, ", "))
		}
	}
}

// TestRegisteredFixKindsAreReachable is the other direction: a registry entry
// nobody emits and repair_slide does not apply is dead vocabulary that agents
// would be told about for nothing.
func TestRegisteredFixKindsAreReachable(t *testing.T) {
	emitted := emittedFixKinds(t)
	executable := map[string]bool{}
	for _, k := range patterns.ExecutableFixKinds() {
		executable[k] = true
	}
	for _, kind := range patterns.AllFixKinds() {
		if len(emitted[kind]) > 0 || executable[kind] {
			continue
		}
		t.Errorf("registered fix kind %q is neither emitted by any finding nor executable by repair_slide", kind)
	}
}

// TestAdvisoryFixKindsCarryGuidance: an advisory kind with no guidance is the
// bug this bead is about — a finding that says "something is wrong" and offers
// the agent nothing it can act on.
func TestAdvisoryFixKindsCarryGuidance(t *testing.T) {
	for _, kind := range patterns.AdvisoryFixKinds() {
		info, ok := patterns.FixKind(kind)
		if !ok {
			t.Fatalf("AdvisoryFixKinds returned unregistered %q", kind)
		}
		if len(info.Guidance) < 40 {
			t.Errorf("advisory kind %q guidance is too thin to act on: %q", kind, info.Guidance)
		}
		for _, alt := range info.Alternatives {
			if !patterns.FixKindIsExecutable(alt) {
				t.Errorf("advisory kind %q lists alternative %q, which repair_slide cannot execute", kind, alt)
			}
		}
	}
}

// TestRepairFixKindsMatchApplySwitch pins the advertised executable vocabulary
// against applyRepairFix's switch: an executable kind with no case would return
// kind_not_supported despite being advertised, and a case missing from the
// registry would be invisible to agents.
func TestRepairFixKindsMatchApplySwitch(t *testing.T) {
	src, err := os.ReadFile("mcp_repair.go")
	if err != nil {
		t.Fatal(err)
	}
	// Cases inside applyRepairFix's switch, which runs from the function header
	// to the default clause.
	body := string(src)
	start := strings.Index(body, "func applyRepairFix(")
	if start < 0 {
		t.Fatal("applyRepairFix not found")
	}
	end := strings.Index(body[start:], "\tdefault:")
	if end < 0 {
		t.Fatal("applyRepairFix default clause not found")
	}
	caseRE := regexp.MustCompile(`case "([a-z_]+)":`)
	var cases []string
	for _, m := range caseRE.FindAllStringSubmatch(body[start:start+end], -1) {
		cases = append(cases, m[1])
	}
	sort.Strings(cases)

	advertised := patterns.ExecutableFixKinds()
	if strings.Join(cases, ",") != strings.Join(advertised, ",") {
		t.Errorf("applyRepairFix cases and the executable vocabulary differ:\n  switch:   %v\n  registry: %v", cases, advertised)
	}
	// get_capabilities must advertise exactly the executable set.
	if strings.Join(repairFixKinds(), ",") != strings.Join(advertised, ",") {
		t.Errorf("get_capabilities.repair_fix_kinds = %v, want %v", repairFixKinds(), advertised)
	}
	for _, kind := range advisoryFixKinds() {
		if isRepairFixKind(kind) {
			t.Errorf("advisory kind %q is also advertised as executable", kind)
		}
	}
}

// emittedFixKinds scans the module's non-test Go sources for fix literals,
// returning kind -> the files that emit it.
func emittedFixKinds(t *testing.T) map[string][]string {
	t.Helper()
	root := filepath.Join("..", "..")
	out := map[string][]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip VCS internals, vendored trees, agent worktrees (stale copies
			// of this repo), and golden-file fixtures.
			switch info.Name() {
			case ".git", "node_modules", "vendor", "testdata", "worktrees":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path) //nolint:gosec // walking this module's own sources
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, path)
		for _, m := range fixLiteralRE.FindAllStringSubmatch(string(data), -1) {
			kind := m[1]
			if !containsPath(out[kind], rel) {
				out[kind] = append(out[kind], rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}
	for kind := range out {
		sort.Strings(out[kind])
	}
	return out
}

func containsPath(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

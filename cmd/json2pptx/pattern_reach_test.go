package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// DeckSpec is the recommended path but reaches a subset of the pattern
// registry, and nothing said which subset: three reviewers independently
// mis-modelled slides because "unknown slide kind" was the only signal and it
// arrived after the spec was written (go-slide-creator-4fr1). These tests pin
// the registry to the compiler and ensure the skill routes authors to its
// live compositions instead of maintaining a stale pattern table.

// TestPatternReachCoversTheRegistry fails when a pattern is registered without
// anyone saying whether a spec author can reach it.
func TestPatternReachCoversTheRegistry(t *testing.T) {
	var missing []string
	registered := map[string]bool{}
	for _, p := range patterns.Default().List() {
		registered[p.Name()] = true
		if _, known := semantic.PatternReach(p.Name()); !known {
			missing = append(missing, p.Name())
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("patterns with no reachability entry: %v\nadd each to patternReach in internal/semantic/reach.go — the kind that compiles to it, or \"\" for raw_json2pptx only", missing)
	}

	for _, pattern := range append(semantic.ReachablePatterns(), semantic.UnreachablePatterns()...) {
		if !registered[pattern] {
			t.Errorf("patternReach lists %q, which is not a registered pattern", pattern)
		}
	}
}

// TestPatternReachMatchesTheCompiler checks the reachable half against what the
// compiler actually advertises: a pattern claimed reachable through a kind must
// appear in that kind's own composition candidates.
func TestPatternReachMatchesTheCompiler(t *testing.T) {
	advertised := map[string]bool{}
	for _, k := range semantic.AllSlideKinds() {
		body, _ := semantic.KindExample(k)["body"].(map[string]any)
		if body == nil {
			body = semantic.KindExample(k)
		}
		for _, c := range semantic.SlideAlternatives(k, body) {
			if c.Pattern != "" {
				advertised[c.Pattern] = true
			}
		}
	}
	for _, pattern := range semantic.UnreachablePatterns() {
		if advertised[pattern] {
			t.Errorf("%q is listed as unreachable but a kind advertises it as a composition", pattern)
		}
	}
}

// TestGetStartedNamesTheEscapeHatchCount keeps the brief's pattern count in
// step with the code, so the first thing an agent reads is not stale.
func TestGetStartedNamesTheEscapeHatchCount(t *testing.T) {
	fast := fastPathFor("brief", nil)
	if fast == nil {
		t.Fatal("no fast path for a brief")
	}
	want := fmt.Sprintf("reaches %d of the %d patterns",
		len(semantic.ReachablePatterns()), len(semantic.ReachablePatterns())+len(semantic.UnreachablePatterns()))
	var joined string
	for _, step := range fast.Steps {
		joined += step.WhenToCall
	}
	if !strings.Contains(joined, want) {
		t.Errorf("get_started's brief path does not say %q; its pattern count is stale", want)
	}
}

// TestSkillRoutesPatternReachToLiveCatalog checks the durable instruction
// without duplicating the runtime's current reachable/unreachable names.
func TestSkillRoutesPatternReachToLiveCatalog(t *testing.T) {
	skill := readRepoFile(t, "skills/generate-deck/SKILL.md")
	spec := readRepoFile(t, "skills/generate-deck/DECKSPEC.md")
	if len(semantic.UnreachablePatterns()) == 0 {
		t.Fatal("expected an escape-hatch pattern to exercise this contract")
	}
	for _, want := range []string{"list_slide_kinds", "compositions", "raw_json2pptx", "SEMANTIC_PATTERN_NOT_AVAILABLE"} {
		if !strings.Contains(skill+spec, want) {
			t.Errorf("skill does not explain live pattern reach through %q", want)
		}
	}
}

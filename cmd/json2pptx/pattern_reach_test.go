package main

import (
	"fmt"
	"os"
	"regexp"
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
// the published answer to the code — the registry on one side, SKILL.md's
// "Patterns DeckSpec cannot reach" table on the other.

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

// skillUnreachableRow matches one row of SKILL.md's unreachable-pattern table.
var skillUnreachableRow = regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|")

// TestSkillUnreachableTableMatchesCode keeps the table an agent reads in step
// with the compiler. A pattern that becomes reachable and stays in the table
// sends an agent to raw_json2pptx for no reason; one that leaves the table
// without becoming reachable sends it to a kind that does not exist.
func TestSkillUnreachableTableMatchesCode(t *testing.T) {
	raw, err := os.ReadFile("../../skills/generate-deck/SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	const marker = "### Patterns DeckSpec cannot reach"
	start := strings.Index(string(raw), marker)
	if start < 0 {
		t.Fatalf("SKILL.md has no %q section; it publishes the list agents need", marker)
	}
	section := string(raw)[start:]
	if end := strings.Index(section[len(marker):], "\n## "); end >= 0 {
		section = section[:len(marker)+end]
	}

	var listed []string
	for _, m := range skillUnreachableRow.FindAllStringSubmatch(section, -1) {
		listed = append(listed, m[1])
	}
	sort.Strings(listed)

	want := semantic.UnreachablePatterns()
	if strings.Join(listed, ",") != strings.Join(want, ",") {
		t.Errorf("SKILL.md's unreachable-pattern table is out of step with internal/semantic.patternReach\n listed: %v\n  want: %v", listed, want)
	}

	// The counts in the sentence above the table drift as silently as the rows.
	wantSentence := fmt.Sprintf("compiles to %d of the %d registered patterns",
		len(semantic.ReachablePatterns()), len(semantic.ReachablePatterns())+len(want))
	if !strings.Contains(section, wantSentence) {
		t.Errorf("SKILL.md does not say %q; the counts above the table are stale", wantSentence)
	}
}

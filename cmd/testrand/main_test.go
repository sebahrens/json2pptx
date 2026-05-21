package main

import (
	"strings"
	"testing"
)

// TestUsageTextReflectsInvokedName guards the neutral-alias contract: the help
// banner must name whichever binary was invoked (testkit or testrand) and keep
// presenting the tool as a broader test toolbox rather than a random-only
// generator. See go-slide-creator-6ifs.
func TestUsageTextReflectsInvokedName(t *testing.T) {
	for _, prog := range []string{"testkit", "testrand"} {
		out := usageText(prog)
		if !strings.HasPrefix(out, prog+" — ") {
			t.Errorf("usageText(%q) banner should start with %q, got first line: %q",
				prog, prog+" — ", strings.SplitN(out, "\n", 2)[0])
		}
		if !strings.Contains(out, "test toolbox") {
			t.Errorf("usageText(%q) should describe the tool as a test toolbox", prog)
		}
		// Both names must be advertised so neither feels deprecated/removed.
		for _, name := range []string{"testkit", "testrand"} {
			if !strings.Contains(out, name) {
				t.Errorf("usageText(%q) should mention the %q name", prog, name)
			}
		}
		// All subcommands must remain documented.
		for _, cmd := range []string{"generate", "visual", "validate", "svg-stress", "qa"} {
			if !strings.Contains(out, cmd) {
				t.Errorf("usageText(%q) missing documentation for %q subcommand", prog, cmd)
			}
		}
	}
}

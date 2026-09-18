package main

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// groupCommands are CLI commands whose second word selects a sub-subcommand
// with its own FlagSet (e.g. `json2pptx icons list --json`).
var groupCommands = map[string]bool{
	"semantic":          true,
	"icons":             true,
	"patterns":          true,
	"template-settings": true,
	"tables":            true,
}

var (
	// inlineInvocation matches `json2pptx <cmd> <rest-of-code-span>`; the rest
	// stops at a closing backtick, table pipe, or quote.
	inlineInvocation = regexp.MustCompile("json2pptx ([a-z][a-z-]*)([^`|\"\n]*)")
	// argsInvocation matches MCP-config style `"args": ["mcp", "-flag", ...]`.
	argsInvocation = regexp.MustCompile(`"args":\s*\[\s*"([a-z][a-z-]*)"([^\]]*)\]`)
	flagToken      = regexp.MustCompile(`^-{1,2}([A-Za-z][A-Za-z0-9-]*)`)
	usageFlagLine  = regexp.MustCompile(`(?m)^\s+-{1,2}([A-Za-z][A-Za-z0-9-]*)`)
)

// docInvocation is one CLI invocation found in a doc.
type docInvocation struct {
	line  int
	cmd   []string // e.g. ["generate"] or ["icons", "list"]
	flags []string // flag names without dashes
}

// extractDocInvocations finds every `json2pptx <cmd> ...` invocation and every
// `"args": ["<cmd>", ...]` MCP-config snippet in a markdown doc.
func extractDocInvocations(doc string) []docInvocation {
	var out []docInvocation
	for i, line := range strings.Split(doc, "\n") {
		for _, m := range inlineInvocation.FindAllStringSubmatch(line, -1) {
			out = append(out, buildInvocation(i+1, m[1], strings.Fields(m[2])))
		}
		for _, m := range argsInvocation.FindAllStringSubmatch(line, -1) {
			var toks []string
			for _, q := range strings.Split(m[2], ",") {
				toks = append(toks, strings.Trim(strings.TrimSpace(q), `"`))
			}
			out = append(out, buildInvocation(i+1, m[1], toks))
		}
	}
	return out
}

func buildInvocation(line int, cmd string, toks []string) docInvocation {
	inv := docInvocation{line: line, cmd: []string{cmd}}
	if groupCommands[cmd] && len(toks) > 0 && regexp.MustCompile(`^[a-z][a-z-]*$`).MatchString(toks[0]) {
		inv.cmd = append(inv.cmd, toks[0])
		toks = toks[1:]
	}
	for _, tok := range toks {
		tok = strings.TrimLeft(tok, "[(")
		m := flagToken.FindStringSubmatch(tok)
		if m == nil {
			continue
		}
		name := strings.SplitN(m[1], "=", 2)[0]
		if name == "h" || name == "help" {
			continue
		}
		inv.flags = append(inv.flags, name)
	}
	return inv
}

// cliFlagSet returns the flag names a CLI (sub)command accepts, by running it
// with -h through the real dispatcher in a child process (some commands exit
// the process on -h) and parsing the usage it prints. ok is false when the
// command is unknown.
func cliFlagSet(t *testing.T, cmd []string) (map[string]bool, bool) {
	t.Helper()
	args := append([]string{"-test.run=^TestCLIDispatchHelper$", "--"}, cmd...)
	args = append(args, "-h")
	c := exec.Command(os.Args[0], args...) //nolint:gosec // re-exec of the test binary with fixed args
	c.Env = append(os.Environ(), "J2P_CLI_DISPATCH_HELPER=1")
	out, _ := c.CombinedOutput()
	usage := string(out)
	if strings.Contains(usage, "unknown command") {
		return nil, false
	}
	flags := map[string]bool{}
	for _, m := range usageFlagLine.FindAllStringSubmatch(usage, -1) {
		flags[m[1]] = true
	}
	return flags, true
}

// TestCLIDispatchHelper is not a real test: cliFlagSet re-executes the test
// binary into it to run dispatch() with the args after "--".
func TestCLIDispatchHelper(t *testing.T) {
	if os.Getenv("J2P_CLI_DISPATCH_HELPER") != "1" {
		t.Skip("helper process only")
	}
	var rest []string
	for i, a := range os.Args {
		if a == "--" {
			rest = os.Args[i+1:]
			break
		}
	}
	os.Args = append([]string{"json2pptx"}, rest...)
	if err := dispatch(); err != nil {
		_, _ = os.Stderr.WriteString("Error: " + err.Error() + "\n")
	}
	os.Exit(0)
}

// TestSkillDocCLIFlagsExist (go-slide-creator-og5i) fails when an agent-facing
// doc shows a `json2pptx <cmd>` invocation with a flag the command does not
// accept (e.g. the old `json2pptx mcp -output-dir`, whose real flag is
// --output) or names an unknown command.
func TestSkillDocCLIFlagsExist(t *testing.T) {
	docs := []string{
		"../../skills/generate-deck/SKILL.md",
		"../../skills/generate-deck/TOOLS.md",
		"../../skills/generate-deck/WORKFLOW.md",
	}
	cache := map[string]map[string]bool{}
	for _, path := range docs {
		doc := readDiscoveryDoc(t, path)
		invs := extractDocInvocations(doc)
		for _, inv := range invs {
			key := strings.Join(inv.cmd, " ")
			flags, seen := cache[key]
			if !seen {
				var ok bool
				flags, ok = cliFlagSet(t, inv.cmd)
				if !ok {
					t.Errorf("%s:%d: `json2pptx %s` is not a CLI command", path, inv.line, key)
				}
				cache[key] = flags
			}
			if flags == nil {
				continue
			}
			for _, f := range inv.flags {
				if !flags[f] {
					known := make([]string, 0, len(flags))
					for k := range flags {
						known = append(known, k)
					}
					sort.Strings(known)
					t.Errorf("%s:%d: `json2pptx %s` has no flag -%s (known: %s)", path, inv.line, key, f, strings.Join(known, ", "))
				}
			}
		}
	}
}

// TestExtractDocInvocations_CatchesStaleFlag proves the gate is not a no-op:
// the pre-fix SKILL.md line (-output-dir) is flagged, the fixed one is not.
func TestExtractDocInvocations_CatchesStaleFlag(t *testing.T) {
	stale := extractDocInvocations("- **Run as MCP:** `json2pptx mcp [-templates-dir <path>] [-output-dir <path>]`")
	if len(stale) != 1 || strings.Join(stale[0].flags, ",") != "templates-dir,output-dir" {
		t.Fatalf("extract = %+v", stale)
	}
	flags, ok := cliFlagSet(t, []string{"mcp"})
	if !ok || !flags["output"] || !flags["templates-dir"] || !flags["tools"] {
		t.Fatalf("mcp flags = %v", flags)
	}
	if flags["output-dir"] {
		t.Fatal("mcp unexpectedly accepts -output-dir; update this test")
	}
	grp := extractDocInvocations("`json2pptx icons list --json` and `\"args\": [\"mcp\", \"--tools\", \"all\"]`")
	if len(grp) != 2 || strings.Join(grp[0].cmd, " ") != "icons list" || grp[1].flags[0] != "tools" {
		t.Fatalf("extract = %+v", grp)
	}
}

// TestNoHardcodedToolCounts (go-slide-creator-og5i) forbids hard-coded MCP
// tool counts ("45-tool catalog", "37-tool surface", "40+ tool catalogue") in
// tool descriptions and agent-facing docs; they go stale every time a tool is
// added. Say "the tool list" / "the full catalogue" instead.
func TestNoHardcodedToolCounts(t *testing.T) {
	re := regexp.MustCompile(`(?i)\b\d+\+?[- ]tools?\b(?:[- ](?:catalog|catalogue|surface|list))|\(\d+ tools\)|\b\d+\+? (?:tool|row)s?\)`)
	s := server.NewMCPServer("count-check", "test")
	registerMCPTools(s, profileTestConfig(t))
	for name, st := range s.ListTools() {
		if m := re.FindString(st.Tool.Description); m != "" {
			t.Errorf("tool %s description hard-codes a tool count %q", name, m)
		}
	}
	for _, path := range []string{"../../skills/generate-deck/SKILL.md", "../../skills/generate-deck/TOOLS.md", "../../README.md"} {
		for i, line := range strings.Split(readDiscoveryDoc(t, path), "\n") {
			if m := re.FindString(line); m != "" {
				t.Errorf("%s:%d hard-codes a tool count %q", path, i+1, m)
			}
		}
	}
}

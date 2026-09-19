package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Projecting get_capabilities (go-slide-creator-5pta).
//
// get_started's first step is get_capabilities, and SKILL.md repeats it. One
// call returned 183,242 wire bytes — 46K tokens — of which tool_list alone was
// 55,475 B: a verbatim duplicate of what tools/list already sent. The single
// most useful field for an MCP agent (runtime.render_available / output_dir,
// 326 B) sat behind all of it, and the inputSchema was
// {"type":"object","properties":{}} — there was no way to ask for less.

// Capability sections an agent can request.
const (
	capSectionRuntime      = "runtime"
	capSectionFeatures     = "features"
	capSectionDeprecations = "deprecations"
	capSectionVocabularies = "vocabularies"
	capSectionRegistry     = "registry"
	capSectionTools        = "tools"
	capSectionErrorCodes   = "error_codes"
	capSectionCLI          = "cli"
	capSectionAll          = "all"
)

// capSectionsDefault is what a caller gets with no sections argument: enough to
// detect drift and decide what the server can do, and nothing that another call
// already answers. tools/list is the authoritative tool catalogue and the CLI
// table is not callable over MCP, so neither is in the default.
var capSectionsDefault = []string{capSectionRuntime, capSectionFeatures, capSectionDeprecations}

// capSectionsKnown is every accepted value, for validation and the error
// message.
var capSectionsKnown = []string{
	capSectionRuntime, capSectionFeatures, capSectionDeprecations,
	capSectionVocabularies, capSectionRegistry, capSectionTools,
	capSectionErrorCodes, capSectionCLI, capSectionAll,
}

// capabilitySections is a resolved set of requested sections.
type capabilitySections map[string]bool

// has reports whether a section should be included.
func (s capabilitySections) has(name string) bool { return s[capSectionAll] || s[name] }

// parseCapabilitySections reads the optional "sections" argument. It accepts a
// string array or a single comma-separated string. An empty or absent argument
// yields the default set.
func parseCapabilitySections(request mcp.CallToolRequest) (capabilitySections, error) {
	raw, ok := request.GetArguments()["sections"]
	if !ok || raw == nil {
		return sectionSet(capSectionsDefault), nil
	}

	var names []string
	switch v := raw.(type) {
	case string:
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				names = append(names, p)
			}
		}
	case []any:
		for _, item := range v {
			s, isString := item.(string)
			if !isString {
				return nil, fmt.Errorf("sections entries must be strings, got %T", item)
			}
			if p := strings.TrimSpace(s); p != "" {
				names = append(names, p)
			}
		}
	default:
		return nil, fmt.Errorf("sections must be a string or an array of strings, got %T", raw)
	}
	if len(names) == 0 {
		return sectionSet(capSectionsDefault), nil
	}

	out := capabilitySections{}
	for _, n := range names {
		n = strings.ToLower(n)
		if !sectionSet(capSectionsKnown)[n] {
			return nil, fmt.Errorf("unknown section %q: must be one of %s", n, strings.Join(capSectionsKnown, ", "))
		}
		out[n] = true
	}
	return out, nil
}

// sectionSet turns a slice into a lookup set.
func sectionSet(names []string) capabilitySections {
	out := make(capabilitySections, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// applyCapabilitySections blanks every section the caller did not ask for.
// schema_version, tool_version, schema_fingerprint and changelog_url always
// survive: drift detection must work in the smallest projection.
func applyCapabilitySections(resp *capabilitiesResponse, sections capabilitySections) {
	if !sections.has(capSectionTools) {
		resp.ToolList = nil
		resp.MCPToolsAvailable = nil
	}
	if !sections.has(capSectionCLI) {
		resp.CLIOnlyCommands = nil
	}
	if !sections.has(capSectionRegistry) {
		resp.Registry = capabilitiesRegistry{}
	}
	if !sections.has(capSectionVocabularies) {
		resp.Vocabularies = capabilitiesVocabularies{}
	}
	if !sections.has(capSectionErrorCodes) {
		resp.ErrorCodes = nil
	}
	if !sections.has(capSectionDeprecations) {
		resp.DeprecatedFields = nil
		resp.Deprecations = nil
	}
	if !sections.has(capSectionFeatures) {
		resp.Features = capabilitiesFeatures{}
	}
	if !sections.has(capSectionRuntime) {
		resp.Runtime = capabilitiesRuntime{}
	}
	resp.SectionsIncluded = sortedSections(sections)
}

// sortedSections lists the included sections, so a response says what it is.
func sortedSections(sections capabilitySections) []string {
	if sections.has(capSectionAll) {
		out := append([]string(nil), capSectionsKnown...)
		sort.Strings(out)
		return out
	}
	out := make([]string, 0, len(sections))
	for name := range sections {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

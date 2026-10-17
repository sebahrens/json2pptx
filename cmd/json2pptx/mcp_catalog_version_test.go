package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var catalogVersionRE = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`)

func catalogVersion(t *testing.T, s string) [3]int {
	t.Helper()
	if catalogVersionRE.FindString(s) != s {
		t.Fatalf("invalid catalog semver %q", s)
	}
	var v [3]int
	for i, n := range strings.Split(s, ".") {
		v[i], _ = strconv.Atoi(n)
	}
	return v
}
func compareCatalogVersion(a, b [3]int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func TestToolAddedInMatchesSchemaChronology(t *testing.T) {
	current := catalogVersion(t, SchemaVersion)
	newest := [3]int{}
	for _, tool := range mcpToolCatalog() {
		v := catalogVersion(t, tool.AddedIn)
		if compareCatalogVersion(v, current) > 0 {
			t.Errorf("%s added_in %s exceeds SchemaVersion %s", tool.Name, tool.AddedIn, SchemaVersion)
		}
		if compareCatalogVersion(v, newest) > 0 {
			newest = v
		}
	}
	raw, err := os.ReadFile("../../docs/SCHEMA_CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	// Changelog entries may be dated top-level bullets or version headings.
	// Use each entry's newest version for old→new version statements, then
	// find the earliest release that mentions the newest registered tools.
	boundaries := regexp.MustCompile(`(?m)^- \*\*|^## `).FindAllIndex(raw, -1)
	for _, tool := range mcpToolCatalog() {
		if compareCatalogVersion(catalogVersion(t, tool.AddedIn), newest) != 0 {
			continue
		}
		found := false
		first := [3]int{}
		for i, start := range boundaries {
			end := len(raw)
			if i+1 < len(boundaries) {
				end = boundaries[i+1][0]
			}
			entry := string(raw[start[0]:end])
			if !strings.Contains(entry, "`"+tool.Name+"`") {
				continue
			}
			versions := catalogVersionRE.FindAllString(entry, -1)
			if len(versions) == 0 {
				continue
			}
			released := [3]int{}
			for _, s := range versions {
				v := catalogVersion(t, s)
				if compareCatalogVersion(v, released) > 0 {
					released = v
				}
			}
			if !found || compareCatalogVersion(released, first) < 0 {
				first = released
				found = true
			}
		}
		if !found || compareCatalogVersion(first, newest) != 0 {
			t.Errorf("%s added_in=%s but earliest changelog mention=%v (found=%v)", tool.Name, tool.AddedIn, first, found)
		}
	}
}

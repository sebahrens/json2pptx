package icons

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// DefaultSet is the icon set used when no prefix is specified.
const DefaultSet = "outline"

// Lookup retrieves SVG bytes for a bundled icon by name.
//
// Name formats:
//   - "chart-pie"         → outline/chart-pie.svg
//   - "outline:chart-pie" → outline/chart-pie.svg
//   - "filled:chart-pie"  → filled/chart-pie.svg
//
// Returns the raw SVG content or an error if the icon is not found.
func Lookup(name string) ([]byte, error) {
	set, base := parseName(name)
	path := set + "/" + base + ".svg"
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return nil, fmt.Errorf("icon %q not found (tried %s): %w", name, path, err)
	}
	return data, nil
}

// Exists reports whether a bundled icon with the given name exists.
func Exists(name string) bool {
	set, base := parseName(name)
	path := set + "/" + base + ".svg"
	_, err := fs.Stat(FS, path)
	return err == nil
}

// List returns all icon names in the given set ("filled" or "outline").
// Names are returned without the set prefix or .svg extension.
func List(set string) ([]string, error) {
	entries, err := fs.ReadDir(FS, set)
	if err != nil {
		return nil, fmt.Errorf("listing icon set %q: %w", set, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".svg") {
			names = append(names, strings.TrimSuffix(n, ".svg"))
		}
	}
	return names, nil
}

// parseName splits "set:name" into (set, name).
// If no prefix is present, DefaultSet is used.
func parseName(name string) (set, base string) {
	name = strings.TrimSpace(name)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i], name[i+1:]
	}
	return DefaultSet, name
}

// Suggest returns up to maxResults bundled icon names ranked by Levenshtein
// distance to the provided name. Suggestions are formatted to match the
// caller's input style: bare when the input was bare, qualified
// ("filled:chart-pie") when the input was qualified or when the bare base
// name exists only in a non-default set. Returns nil if no candidate falls
// within an edit distance of 4.
func Suggest(name string, maxResults int) []string {
	if maxResults <= 0 {
		return nil
	}
	name = strings.TrimSpace(name)
	set, base := parseName(name)
	isQualified := strings.IndexByte(name, ':') >= 0

	outline, _ := List("outline")
	filled, _ := List("filled")

	type cand struct {
		full string
		dist int
	}
	var cands []cand

	addSet := func(setName string, names []string, qualify bool) {
		for _, n := range names {
			d := levenshteinDist(base, n)
			full := n
			if qualify {
				full = setName + ":" + n
			}
			cands = append(cands, cand{full: full, dist: d})
		}
	}

	switch {
	case !isQualified:
		addSet("outline", outline, false)
		// Cross-set hint: the bare base name resolves only in the non-default
		// set — promote the qualified form as a distance-0 candidate so it
		// outranks Levenshtein matches.
		if !Exists(DefaultSet+":"+base) && Exists("filled:"+base) {
			cands = append(cands, cand{full: "filled:" + base, dist: 0})
		}
		addSet("filled", filled, true)
	case set == "outline":
		addSet("outline", outline, true)
	case set == "filled":
		addSet("filled", filled, true)
	default:
		// Unknown set prefix — search both sets, always qualified.
		addSet("outline", outline, true)
		addSet("filled", filled, true)
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].full < cands[j].full
	})

	out := make([]string, 0, maxResults)
	seen := make(map[string]struct{})
	for _, c := range cands {
		if c.dist > 4 {
			continue
		}
		if _, ok := seen[c.full]; ok {
			continue
		}
		seen[c.full] = struct{}{}
		out = append(out, c.full)
		if len(out) >= maxResults {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// levenshteinDist computes the Levenshtein edit distance between two strings.
// Standard O(m*n) dynamic programming with single-row memory.
func levenshteinDist(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}
	prev := make([]int, la+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= lb; i++ {
		curr := make([]int, la+1)
		curr[0] = i
		for j := 1; j <= la; j++ {
			cost := 1
			if b[i-1] == a[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			m := ins
			if del < m {
				m = del
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev = curr
	}
	return prev[la]
}

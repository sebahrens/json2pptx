package core

import (
	"fmt"
	"sort"
	"strings"
)

// UnknownTypeError reports a diagram type the registry does not know, carrying
// the vocabulary an agent needs to self-correct.
//
// The bare error used to be `svggen: unknown diagram type "barchart"` with no
// allowed list and no suggestion, wrapped in the compiler's internal grid
// coordinates by the time it reached the caller — while every comparable error
// on the surface (unknown pattern, unknown finding code, unknown template) offers
// a did_you_mean plus the allowed set (go-slide-creator-rrjj).
type UnknownTypeError struct {
	// Type is the unrecognised type as supplied.
	Type string
	// DidYouMean is the closest registered type or alias, empty when nothing is
	// close enough to suggest.
	DidYouMean string
	// Allowed is the sorted list of canonical registered types.
	Allowed []string
}

// Error renders the message. It leads with the suggestion when there is one,
// because that is the single most actionable fact; the full vocabulary follows
// so an agent never needs a second lookup.
func (e *UnknownTypeError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "svggen: unknown diagram type %q", e.Type)
	if e.DidYouMean != "" {
		fmt.Fprintf(&b, "; did you mean %q?", e.DidYouMean)
	}
	if len(e.Allowed) > 0 {
		fmt.Fprintf(&b, " (allowed types: %s)", strings.Join(e.Allowed, ", "))
	}
	return b.String()
}

// NewUnknownTypeError builds an UnknownTypeError for a type the registry could
// not resolve, computing the closest match across canonical names AND aliases so
// a near-miss on an alias ("barchart" for "bar_chart") is suggested too.
func (r *Registry) NewUnknownTypeError(typ string) *UnknownTypeError {
	canonical := r.Types()
	sort.Strings(canonical)

	// Candidates include aliases so "barchart" or "orgchart" resolve to a
	// suggestion even though they are not canonical names.
	candidates := append([]string(nil), canonical...)
	r.mu.RLock()
	for alias := range r.aliases {
		candidates = append(candidates, alias)
	}
	r.mu.RUnlock()

	return &UnknownTypeError{
		Type:       typ,
		DidYouMean: closestType(typ, candidates),
		Allowed:    canonical,
	}
}

// maxTypeSuggestionDistance is the edit distance within which a typo is treated
// as a near-miss worth suggesting. 4 catches "barchart" → "bar_chart" and
// "stackedbar" → "stacked_bar_chart"-class slips without suggesting an
// unrelated type.
const maxTypeSuggestionDistance = 4

// closestType returns the best suggestion for target from candidates, or "" when
// nothing is close enough.
//
// A separator-insensitive match wins outright — "barchart" for "bar_chart" is by
// far the most common slip, and no edit-distance candidate should ever beat it.
// Otherwise the smallest Levenshtein distance within
// maxTypeSuggestionDistance wins, ties broken lexicographically so the
// suggestion is deterministic.
func closestType(target string, candidates []string) string {
	normalize := func(s string) string {
		return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(s))
	}
	normTarget := normalize(target)

	exact := ""
	for _, c := range candidates {
		if normalize(c) != normTarget {
			continue
		}
		// Prefer the canonical-looking form: the longest match, which is the
		// one carrying separators ("bar_chart" over "barchart").
		if exact == "" || len(c) > len(exact) || (len(c) == len(exact) && c < exact) {
			exact = c
		}
	}
	if exact != "" {
		return exact
	}

	// Prefix/suffix containment is the next-strongest signal: "stackedbar"
	// should reach "stacked_bar_chart" even though their edit distance is large.
	contained := ""
	for _, c := range candidates {
		nc := normalize(c)
		if !strings.HasPrefix(nc, normTarget) && !strings.HasPrefix(normTarget, nc) {
			continue
		}
		if contained == "" || len(c) < len(contained) || (len(c) == len(contained) && c < contained) {
			contained = c
		}
	}
	if contained != "" {
		return contained
	}

	// Edit distance is the last resort, and is bounded relative to the name's
	// own length as well as absolutely: without that, a genuinely unregistered
	// type like "sankey" or "combo" drew a confidently wrong suggestion
	// ("area", "donut"). No suggestion beats a misleading one.
	limit := maxTypeSuggestionDistance
	if half := len([]rune(target)) / 2; half < limit {
		limit = half
	}
	if limit <= 0 {
		return ""
	}

	best := ""
	bestDist := limit + 1
	for _, c := range candidates {
		d := editDistance(strings.ToLower(target), strings.ToLower(c))
		if d < bestDist || (d == bestDist && c < best) {
			best = c
			bestDist = d
		}
	}
	if bestDist > limit {
		return ""
	}
	return best
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

package semantic

import "sort"

// Pattern reachability (go-slide-creator-4fr1).
//
// DeckSpec is the recommended path, but it reaches a subset of the pattern
// registry. Nothing said which subset, so three reviewers independently
// mis-modelled slides — a table written as bullets, a stat written as a title —
// because "unknown slide kind" was the only signal, and it arrived after the
// spec was written. This table is that answer, published through
// list_slide_kinds and pinned against the registry by a test: a pattern with no
// entry fails the build, so a new pattern cannot ship without someone saying
// whether a spec author can reach it.

// patternReach maps every registered pattern to the DeckSpec kind that compiles
// to it, or "" when no kind does — in which case the slide has to be authored
// through raw_json2pptx.
var patternReach = map[string]SlideKind{
	// Reachable.
	"agenda":               KindAgenda,
	"agenda-with-images":   KindAgenda,
	"arch-stack":           KindArchitecture,
	"bmc-canvas":           KindFramework,
	"card-grid":            KindComparison,
	"chart-insights-split": KindChartInsight,
	"comparison-2col":      KindComparison,
	"exec-summary":         KindExecutiveSummary,
	"image-text-split":     KindImageCase,
	"kpi-2up":              KindKPISnapshot,
	"kpi-3up":              KindKPISnapshot,
	"kpi-4up":              KindKPISnapshot,
	"kpi-5up":              KindKPISnapshot,
	"kpi-6up":              KindKPISnapshot,
	"matrix-2x2":           KindMatrix2x2,
	"phase-roadmap":        KindRoadmap,
	"process-flow":         KindProcess,
	"stat-hero":            KindStat,
	"stylish-panels":       KindComparison,
	"table-highlight":      KindOptionMatrix,
	"team-bios":            KindTeam,
	"timeline-horizontal":  KindTimeline,

	// Not reachable: author these with raw_json2pptx.
	"before-after":                 "",
	"before-after-compact":         "",
	"driver-tree":                  "",
	"dual-org-ladder":              "",
	"hero-detail":                  "",
	"horizontal-bar-with-callouts": "",
	"icon-row":                     "",
	"journey-maturity-model":       "",
	"kpi-inline":                   "",
	"numbered-step-strip":          "",
	"process-flow-compact":         "",
	"process-grid-2row":            "",
	"pull-quote":                   "",
	"pyramid":                      "",
	"quote-cluster":                "",
	"roadmap-phased":               "",
	"scqa-summary":                 "",
	"strategy-house":               "",
	"swimlane":                     "",
	"value-chain":                  "",
	"waterfall-bridge":             "",
}

// PatternReach returns the DeckSpec kind that compiles to a pattern, and
// whether the pattern is registered in the reachability table at all.
func PatternReach(pattern string) (SlideKind, bool) {
	kind, known := patternReach[pattern]
	return kind, known
}

// UnreachablePatterns returns, sorted, the registered patterns no DeckSpec kind
// compiles to. SKILL.md publishes this list and a test pins the two together.
func UnreachablePatterns() []string {
	var out []string
	for pattern, kind := range patternReach {
		if kind == "" {
			out = append(out, pattern)
		}
	}
	sort.Strings(out)
	return out
}

// ReachablePatterns returns, sorted, the registered patterns a DeckSpec kind
// compiles to.
func ReachablePatterns() []string {
	var out []string
	for pattern, kind := range patternReach {
		if kind != "" {
			out = append(out, pattern)
		}
	}
	sort.Strings(out)
	return out
}

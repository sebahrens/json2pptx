// Package patterns defines the Pattern interface and a thread-safe Registry
// for named slide patterns. Patterns expand to shape_grid definitions at
// generation time, replacing ~600 tokens of boilerplate with ~100-token calls.
//
// See docs/PATTERNS.md for the authoring guide and contract.
package patterns

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Pattern interface — spec §3.3
// ---------------------------------------------------------------------------

// Pattern is the contract every named pattern must implement.
type Pattern interface {
	Name() string
	Description() string
	UseWhen() string
	NotWhen() string
	Version() int

	// Taxonomy returns classification metadata for agent-facing discovery:
	// category, narrative roles, density, accent weight, and sibling pairings.
	Taxonomy() PatternTaxonomy

	// NewValues returns a zero-value pointer to the pattern's Values struct,
	// ready for JSON unmarshalling.
	NewValues() any

	// NewOverrides returns a zero-value pointer to the pattern's Overrides
	// struct, or nil if the pattern has no overrides.
	NewOverrides() any

	// NewCellOverride returns a zero-value pointer to the pattern's per-cell
	// override struct, or nil if the pattern allows no per-cell overrides.
	NewCellOverride() any

	// Schema returns the hand-authored JSON Schema for agent-facing discovery.
	// Authoritative for external contracts (D13).
	Schema() *Schema

	// Validate checks values, overrides, and cell overrides for correctness.
	// Returns an errors.Join-aggregated error, or nil.
	Validate(values, overrides any, cellOverrides map[int]any) error

	// CellsHint returns a human-readable cell count description for compact
	// discovery output. Examples: "3", "3-7", "rows × cols", "9", "4 + axes".
	CellsHint() string

	// Expand produces a ShapeGridInput from the pattern's typed inputs.
	Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error)
}

// ---------------------------------------------------------------------------
// PatternTaxonomy — classification metadata for discovery and sequencing
// ---------------------------------------------------------------------------

// PatternTaxonomy provides compositional metadata so agents can plan deck arcs,
// not just individual slide moments.
type PatternTaxonomy struct {
	// Category groups patterns by function: "data-display", "narrative",
	// "structural", or "hero".
	Category string `json:"category"`

	// NarrativeRole describes where in a deck arc this pattern fits best.
	// Values: "open", "frame", "evidence", "compare", "conclude".
	NarrativeRole []string `json:"narrative_role"`

	// PairsWith lists sibling pattern names that flow well as the next slide.
	PairsWith []string `json:"pairs_with"`

	// ComposesWith lists sibling pattern names that can coexist on the SAME
	// slide via a compose envelope. Distinct from PairsWith, which is about
	// next-slide sequencing. Used by recommend_visual to populate
	// PlacementGuidance.ComposableWith and by plan_deck to assemble compose
	// envelopes.
	ComposesWith []string `json:"composes_with"`

	// RoleOnSlide describes which slot(s) inside a compose envelope this
	// pattern naturally occupies. Values: "banner", "pillars", "foundation",
	// "roof", "callout". Empty when the pattern is not intended for
	// compose-envelope assembly.
	RoleOnSlide []string `json:"role_on_slide"`

	// DensityClass describes visual density: "low", "medium", or "high".
	DensityClass string `json:"density_class"`

	// AccentWeight describes accent color prominence: "subtle", "normal",
	// or "strong".
	AccentWeight string `json:"accent_weight"`

	// SparseThresholdPct is the minimum average cell density (0–100) below
	// which expand_pattern emits a sparse_layout capacity warning. Patterns
	// leave this at 0 to use the default (20), or set it explicitly. Single-row
	// patterns that sprawl vertically (process-flow, kpi-Nup) should use 15.
	SparseThresholdPct int `json:"sparse_threshold_pct,omitempty"`

	// DataVisual marks a pattern whose whole job is to argue from data: a
	// chart, a chart drawn as a shape grid, a scored comparison matrix. Those
	// slides are the ones a missing "so what" hurts most, so the
	// takeaway_missing lint fires on this flag.
	//
	// It is deliberately narrower than Category == "data-display": a card grid
	// and an icon row display content without making a quantitative claim, and
	// a 2x2 matrix makes one while being structural (go-slide-creator-g2cy).
	DataVisual bool `json:"data_visual,omitempty"`
}

// EffectiveSparseThreshold returns SparseThresholdPct if set, otherwise the
// provided default.
func (t PatternTaxonomy) EffectiveSparseThreshold(defaultPct int) int {
	if t.SparseThresholdPct > 0 {
		return t.SparseThresholdPct
	}
	return defaultPct
}

// ---------------------------------------------------------------------------
// ExpandContext — inputs available to Expand
// ---------------------------------------------------------------------------

// ExpandContext carries template/slide context needed by Pattern.Expand.
type ExpandContext struct {
	Theme          types.ThemeInfo
	Metadata       *types.TemplateMetadata // optional; nil when template has no metadata
	ContentZone    *shapegrid.ContentZone  // optional; used to clamp authored slide-relative bounds like rendering
	SlideWidth     int64                   // EMU
	SlideHeight    int64                   // EMU
	LayoutBounds   LayoutBounds
	AccentStrategy AccentStrategy // deck-level accent rotation strategy
	SlideIndex     int            // zero-based slide position in the deck
	SectionIndex   int            // zero-based section index (0 when no sections)
	// RotationKey is a content-derived slide identity. It keeps rotate stable
	// when unrelated slides are inserted before this pattern.
	RotationKey string
	// AutoAccent is true only when the pattern did not author an accent or
	// semantic_accent override. Per-cell variation then stays in the safe pool.
	AutoAccent bool
}

// ResolveAccent resolves the accent color for this context, honoring the
// deck-level accent strategy when no explicit accent is specified.
func (c ExpandContext) ResolveAccent(accent, semanticAccent string) string {
	if accent != "" {
		return accent
	}
	if semanticAccent != "" && c.Metadata != nil {
		if resolved := c.Metadata.SemanticAccents[semanticAccent]; resolved != "" {
			return resolved
		}
	}
	if semanticAccent != "" {
		if resolved := c.Theme.SemanticAccents[semanticAccent]; resolved != "" {
			return resolved
		}
	}
	if c.AccentStrategy == AccentStrategyRotate {
		chosen, _, _ := c.rotatedAccent()
		return chosen
	}
	return AccentForStrategy(c.AccentStrategy, c.SlideIndex, c.SectionIndex)
}

// ResolveSurface returns the scheme color name for a surface tint role,
// falling back to defaultColor if the role is not defined in template metadata.
func (c ExpandContext) ResolveSurface(role, defaultColor string) string {
	return ResolveSurface(role, c.Metadata, defaultColor)
}

// SemanticAccentOr returns the template's declared accent for a semantic role
// ("positive", "negative", "neutral"), falling back to defaultColor when the
// template declares no semantic_accents or omits this role.
//
// This is for fills whose MEANING is fixed — a negative delta bar is "bad"
// whatever the palette — as opposed to a fill that just needs to be
// distinguishable from its neighbour. Hardcoding a literal for a semantic role
// puts the wrong colour on any template whose palette disagrees: waterfall
// bridges painted negative bars accent2, which on modern-template is a cool
// blue, while the template's declared negative colour sat unused
// (go-slide-creator-noa7).
func (c ExpandContext) SemanticAccentOr(role, defaultColor string) string {
	if c.Metadata != nil {
		if resolved, ok := c.Metadata.SemanticAccents[role]; ok && resolved != "" {
			return resolved
		}
	}
	return defaultColor
}

// LayoutBounds describes the usable content area on a slide in EMU.
type LayoutBounds struct {
	X      int64 // Left offset in EMU
	Y      int64 // Top offset in EMU
	Width  int64 // Width in EMU
	Height int64 // Height in EMU
}

// ---------------------------------------------------------------------------
// CalloutSupport — optional interface for envelope-level callout (D18)
// ---------------------------------------------------------------------------

// CalloutSupport is an optional interface patterns can implement to declare
// support for an envelope-level callout row appended below the pattern grid.
// Non-uniform patterns (e.g. matrix-2x2, bmc-canvas) should not implement
// this interface. Patterns that do implement it return true from SupportsCallout.
type CalloutSupport interface {
	SupportsCallout() bool
}

// PatternCallout is the DTO for an envelope-level callout band rendered
// below the pattern content. It is a plain text row (no nested shape_grid,
// no bullets) per D15 discipline.
type PatternCallout struct {
	Text     string `json:"text"`
	Emphasis string `json:"emphasis,omitempty"` // "bold", "italic", "bold-italic"
	Accent   string `json:"accent,omitempty"`   // scheme ref, e.g. "accent1"
}

// BannerSpec is the DTO for an envelope-level banner band rendered above the
// merged compose grid. Like PatternCallout, it is a plain text row (no nested
// shape_grid, no bullets) per D15 discipline. Used by ComposeInput.Banner to
// let agents add a Strategy-House-style banner above an arbitrary segment
// arrangement without spending a segment slot on a faux-banner pattern such
// as pull-quote.
type BannerSpec struct {
	Text     string `json:"text"`
	Emphasis string `json:"emphasis,omitempty"` // "bold", "italic", "bold-italic" (default: bold)
	Accent   string `json:"accent,omitempty"`   // scheme ref, e.g. "accent1" (default: accent1)
}

// ---------------------------------------------------------------------------
// PostExpandWarner — optional interface for post-expansion fit warnings
// ---------------------------------------------------------------------------

// PostExpandWarner is an optional interface patterns can implement to emit
// structured warning strings after expansion (e.g. for a known-degraded state
// like a chart-insights-split rendered without a chart spec). Warnings are
// formatted as "<CODE>: <message>" so downstream callers can parse them into
// FitFindings, mirroring the compose warning convention. expandPattern invokes
// this hook after Expand() and appends the returned warnings to its warning
// list.
type PostExpandWarner interface {
	PostExpandWarnings(ctx ExpandContext, values, overrides any) []string
}

// ---------------------------------------------------------------------------
// InlineMarkdownSupport — optional interface for markdown emphasis in body text
// ---------------------------------------------------------------------------

// InlineMarkdownSupport is an optional interface patterns can implement to
// declare that body text supports inline markdown emphasis (**bold**, *italic*).
// Patterns that implement this convert markdown to <b>/<i> tags at expand time.
type InlineMarkdownSupport interface {
	SupportsInlineMarkdown() bool
}

// ---------------------------------------------------------------------------
// BudgetConfigProvider — optional interface for text budget guide computation
// ---------------------------------------------------------------------------

// BudgetConfig describes a single grid configuration for which text budgets
// should be computed on demand (e.g. 3 columns × 2 rows).
type BudgetConfig struct {
	Columns int // grid column count
	Rows    int // grid row count
}

// BudgetConfigProvider is an optional interface grid-shaped patterns can
// implement to declare which column×row configurations should appear in the
// text_budget_guide returned by show_pattern. Non-grid patterns (pull-quote,
// stat-hero) should not implement this interface.
type BudgetConfigProvider interface {
	BudgetConfigurations() []BudgetConfig
}

// ---------------------------------------------------------------------------
// Schema — see schema.go for full implementation (bead 3)
// ---------------------------------------------------------------------------

// Schema holds a hand-authored JSON Schema (draft 2020-12) for a pattern's
// external contract. See schema.go for constructors and helpers.
type Schema struct {
	raw schemaJSON
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is a thread-safe collection of named patterns.
type Registry struct {
	mu       sync.RWMutex
	patterns map[string]Pattern
	aliases  map[string]string // alias → canonical name
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		patterns: make(map[string]Pattern),
		aliases:  make(map[string]string),
	}
}

// Register adds a pattern to the registry. It panics if a pattern with the
// same name is already registered.
func (r *Registry) Register(p Pattern) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.patterns[name]; exists {
		panic(fmt.Sprintf("patterns: duplicate registration for %q", name))
	}
	r.patterns[name] = p
}

// RegisterAlias adds an alternative name that resolves to an existing
// canonical pattern. It panics if the alias collides with a canonical name
// or another alias, or if the target pattern is not registered.
func (r *Registry) RegisterAlias(alias, canonical string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.patterns[alias]; exists {
		panic(fmt.Sprintf("patterns: alias %q collides with canonical name", alias))
	}
	if _, exists := r.aliases[alias]; exists {
		panic(fmt.Sprintf("patterns: duplicate alias %q", alias))
	}
	if _, exists := r.patterns[canonical]; !exists {
		panic(fmt.Sprintf("patterns: alias %q target %q is not registered", alias, canonical))
	}
	r.aliases[alias] = canonical
}

// Get returns the pattern with the given name and true, or (nil, false) if
// no such pattern is registered. Aliases are resolved transparently.
func (r *Registry) Get(name string) (Pattern, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p, ok := r.patterns[name]; ok {
		return p, true
	}
	if canonical, ok := r.aliases[name]; ok {
		p, ok := r.patterns[canonical]
		return p, ok
	}
	return nil, false
}

// ResolveAlias returns the canonical name for the given input. If the input
// is already a canonical name or is unknown, it is returned unchanged.
func (r *Registry) ResolveAlias(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if canonical, ok := r.aliases[name]; ok {
		return canonical
	}
	return name
}

// Aliases returns a copy of the alias→canonical mapping.
func (r *Registry) Aliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]string, len(r.aliases))
	for alias, canonical := range r.aliases {
		out[alias] = canonical
	}
	return out
}

// List returns all registered patterns sorted by name.
func (r *Registry) List() []Pattern {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Pattern, 0, len(r.patterns))
	for _, p := range r.patterns {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// CalloutSupportedPatterns returns the sorted names of all patterns that
// implement CalloutSupport and return true from SupportsCallout().
func (r *Registry) CalloutSupportedPatterns() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for _, p := range r.patterns {
		if cs, ok := p.(CalloutSupport); ok && cs.SupportsCallout() {
			names = append(names, p.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Suggest returns the closest registered pattern name to the given input,
// using Damerau-Levenshtein distance. It returns ("", false) if no pattern
// is within maxDist edits (default 2).
//
// Distance is measured on the raw name and on a normalized form that spells
// number words as digits and drops separators, so the names agents actually
// write for the numbered families resolve: "kpi-four-up" is 4 raw edits from
// "kpi-4up" — past any sane threshold — but 0 once "four" is 4
// (go-slide-creator-20jm).
func (r *Registry) Suggest(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	const maxDist = 2
	best := ""
	bestDist := maxDist + 1
	normalized := normalizePatternName(name)

	// Ties are broken lexicographically, NOT by map order. "kpi-9up" is
	// distance 1 from every one of kpi-2up…kpi-6up; picking whichever the map
	// yielded first made the suggestion differ between two calls in the same
	// process, so validate and generate could report different did_you_mean
	// values for one typo.
	consider := func(candidate string) {
		d := damerauLevenshtein(name, candidate)
		if nd := damerauLevenshtein(normalized, normalizePatternName(candidate)); nd < d {
			d = nd
		}
		if d < bestDist || (d == bestDist && candidate < best) {
			bestDist = d
			best = candidate
		}
	}
	for registered := range r.patterns {
		consider(registered)
	}
	for alias := range r.aliases {
		consider(alias)
	}
	if bestDist <= maxDist {
		return best, true
	}
	return "", false
}

// numberWords spells the counts that appear in pattern names. Only the counts a
// pattern family actually uses are listed; a name is not a place for prose.
var numberWords = map[string]string{
	"one": "1", "two": "2", "three": "3", "four": "4", "five": "5",
	"six": "6", "seven": "7", "eight": "8", "nine": "9", "ten": "10",
}

// normalizePatternName lowercases a pattern name, spells its number words as
// digits, and drops separators, so "kpi-four-up", "KPI_4_up" and "kpi4up" all
// collapse onto "kpi4up".
func normalizePatternName(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	word := make([]rune, 0, 8)
	flush := func() {
		if len(word) == 0 {
			return
		}
		w := string(word)
		if digits, ok := numberWords[w]; ok {
			b.WriteString(digits)
		} else {
			b.WriteString(w)
		}
		word = word[:0]
	}
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			word = append(word, r)
		case r >= '0' && r <= '9':
			flush()
			b.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return b.String()
}

// damerauLevenshtein computes the Damerau-Levenshtein distance between two
// strings (optimal string alignment variant). It counts insertions, deletions,
// substitutions, and transpositions of adjacent characters.
func damerauLevenshtein(a, b string) int {
	la, lb := len(a), len(b)
	// Quick bounds check.
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// d[i][j] = distance between a[:i] and b[:j].
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
			// Transposition.
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+cost)
			}
		}
	}
	return d[la][lb]
}

// ---------------------------------------------------------------------------
// Package-level default registry
// ---------------------------------------------------------------------------

var defaultRegistry = NewRegistry()

// Default returns the package-level registry used for static pattern
// registration via init() functions.
func Default() *Registry {
	return defaultRegistry
}

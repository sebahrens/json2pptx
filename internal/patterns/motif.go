package patterns

import (
	"encoding/json"
	"sort"
	"strings"
)

// Motif is what a slide looks like at a glance: the shape its content is drawn
// in, independent of which pattern drew it. Deck rhythm counts motifs as well
// as patterns, because a deck that alternates kpi-4up, stylish-panels and
// icon-row uses three patterns and shows the audience the same slide three
// times (go-slide-creator-rd7oj).
type Motif string

// The motif vocabulary. Each value names what the eye sees, so two patterns
// share a motif exactly when their slides are look-alikes.
const (
	// MotifTiles is a row or grid of filled tiles / cards.
	MotifTiles Motif = "tiles"
	// MotifOpenColumns is a row of peer columns standing open on the slide
	// (a heading, number, icon or portrait over text), with no tile.
	MotifOpenColumns Motif = "open-columns"
	// MotifOpenList is a stack of rows separated by rules or whitespace.
	MotifOpenList Motif = "open-list"
	// MotifTable is a grid read by row and column headers.
	MotifTable Motif = "table"
	// MotifChart is a data chart.
	MotifChart Motif = "chart"
	// MotifDiagram is a drawing whose shape carries the meaning (a house, a
	// pyramid, a hub, a tree, a 2x2).
	MotifDiagram Motif = "diagram"
	// MotifFlow is a sequence of steps or stops along one line.
	MotifFlow Motif = "flow"
	// MotifHeroNumber is one dominant number.
	MotifHeroNumber Motif = "hero-number"
	// MotifQuote is one dominant quotation.
	MotifQuote Motif = "quote"
	// MotifSplit is running text beside an image or a sidebar panel.
	MotifSplit Motif = "split"
	// MotifText and MotifImage are the motifs of slides drawn without a
	// pattern: a bullet / text body, and a picture.
	MotifText  Motif = "text"
	MotifImage Motif = "image"
)

// Motifs lists the vocabulary in a stable order.
func Motifs() []Motif {
	return []Motif{
		MotifTiles, MotifOpenColumns, MotifOpenList, MotifTable, MotifChart,
		MotifDiagram, MotifFlow, MotifHeroNumber, MotifQuote, MotifSplit,
		MotifText, MotifImage,
	}
}

// Valid reports whether m is part of the vocabulary.
func (m Motif) Valid() bool {
	for _, known := range Motifs() {
		if m == known {
			return true
		}
	}
	return false
}

// Distinctive reports whether slides of this motif differ enough from each
// other that only the same pattern (or chart type) repeated reads as a
// look-alike: a pyramid, a strategy house and a driver tree are all diagrams
// and no two of them look the same.
func (m Motif) Distinctive() bool {
	return m == MotifChart || m == MotifDiagram
}

// patternMotif is one pattern's motif declaration: what its default look
// draws, and the looks an explicit style switches it to.
type patternMotif struct {
	// base is the motif of the pattern's default rendering.
	base Motif
	// overrideStyles maps an overrides.style value to the motif it renders.
	overrideStyles map[string]Motif
	// valueStyles maps a values.style value to the motif it renders.
	valueStyles map[string]Motif
}

// kpiMotif is shared by the kpi-Nup family: open numbers between hairline
// dividers by default, a row of tiles when asked for.
var kpiMotif = patternMotif{base: MotifOpenColumns, overrideStyles: map[string]Motif{"tiles": MotifTiles}}

// patternMotifs declares the motif of every registered pattern, derived from
// what the pattern renders today (the exemplar of each, generated and looked
// at). A pattern registered without an entry fails TestEveryPatternDeclaresMotif.
var patternMotifs = map[string]patternMotif{
	"agenda":             {base: MotifOpenList},
	"agenda-with-images": {base: MotifOpenList},
	"arch-stack":         {base: MotifDiagram},
	"before-after": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"panels": MotifTiles}},
	"before-after-compact": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"panels": MotifTiles}},
	"bmc-canvas":           {base: MotifTiles},
	"capability-heatmap":   {base: MotifTiles},
	"card-grid":            {base: MotifTiles},
	"chart-insights-split": {base: MotifChart},
	"comparison-2col": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"tiles": MotifTiles}},
	"concentric-rings":  {base: MotifDiagram},
	"contact-directory": {base: MotifOpenColumns},
	"driver-tree":       {base: MotifDiagram},
	"dual-org-ladder": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"tiles": MotifTiles}},
	"exec-summary": {base: MotifOpenList},
	"framework-grid": {base: MotifOpenList,
		overrideStyles: map[string]Motif{"tiles": MotifTiles}},
	"hero-detail":                  {base: MotifHeroNumber},
	"horizontal-bar-with-callouts": {base: MotifChart},
	"icon-row": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"tile": MotifTiles}},
	"image-text-split": {base: MotifSplit},
	"journey-maturity-model": {base: MotifDiagram,
		overrideStyles: map[string]Motif{"flat": MotifFlow}},
	"kpi-2up": kpiMotif,
	"kpi-3up": kpiMotif,
	"kpi-4up": kpiMotif,
	"kpi-5up": kpiMotif,
	"kpi-6up": kpiMotif,
	"kpi-inline": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"tinted": MotifTiles, "solid": MotifTiles}},
	"labeled-rows": {base: MotifOpenList},
	"matrix-2x2": {base: MotifDiagram,
		overrideStyles: map[string]Motif{"tiles": MotifTiles}},
	"metric-list":  {base: MotifOpenList},
	"next-steps":   {base: MotifOpenList},
	"risk-heatmap": {base: MotifDiagram},
	"numbered-step-strip": {base: MotifOpenList,
		valueStyles: map[string]Motif{"chevron": MotifFlow}},
	"phase-roadmap":        {base: MotifFlow},
	"process-flow":         {base: MotifFlow},
	"process-flow-compact": {base: MotifFlow},
	"process-grid-2row":    {base: MotifTiles},
	"pull-quote":           {base: MotifQuote},
	"pyramid":              {base: MotifDiagram},
	"quote-cluster": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"bubble": MotifTiles, "tile": MotifTiles}},
	"roadmap-phased":  {base: MotifTable},
	"scqa-summary":    {base: MotifOpenList},
	"stat-hero":       {base: MotifHeroNumber},
	"state-shift-hub": {base: MotifDiagram},
	"strategy-house":  {base: MotifDiagram},
	"stylish-panels": {base: MotifOpenColumns,
		overrideStyles: map[string]Motif{"ribbon": MotifTiles}},
	"swimlane":            {base: MotifFlow},
	"table-highlight":     {base: MotifTable},
	"team-bios":           {base: MotifOpenColumns},
	"text-sidebar":        {base: MotifSplit},
	"timeline-horizontal": {base: MotifFlow},
	"value-chain":         {base: MotifFlow},
	"waterfall-bridge":    {base: MotifChart},
}

// PatternMotif returns the motif of a pattern's default look, or "" for a
// name that is not a registered pattern. Aliases resolve to their canonical
// pattern.
func PatternMotif(name string) Motif {
	return patternMotifs[Default().ResolveAlias(name)].base
}

// MotifFor returns the motif a pattern renders with the given values and
// overrides: its default look unless an explicit style switches it (a
// stylish-panels slide with overrides.style "ribbon" is a row of tiles, not
// open columns). Either payload may be empty. It returns "" for a name that
// is not a registered pattern.
func MotifFor(name string, values, overrides json.RawMessage) Motif {
	decl, ok := patternMotifs[Default().ResolveAlias(name)]
	if !ok {
		return ""
	}
	if m, ok := decl.overrideStyles[styleField(overrides)]; ok {
		return m
	}
	if m, ok := decl.valueStyles[styleField(values)]; ok {
		return m
	}
	return decl.base
}

// styleField reads the "style" string of a values / overrides object.
func styleField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Style string `json:"style"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	return strings.TrimSpace(obj.Style)
}

// PatternsWithMotif lists the registered patterns whose default look is m,
// sorted by name.
func PatternsWithMotif(m Motif) []string {
	var out []string
	for name, decl := range patternMotifs {
		if decl.base == m {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

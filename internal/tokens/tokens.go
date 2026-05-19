// Package tokens is the single source of truth for cross-cutting design
// constants that previously lived scattered across textfit, patterns, and
// template metadata.
//
// Anything here is part of the project's design system and is referenced
// from documentation (skills/generate-deck/RULES.md typography table),
// validators (text fit defaults, accent constraints), and renderers.
// Changing a value here changes the contract — bump SchemaVersion when
// renaming or removing a token.
//
// Conventions:
//   - Font sizes are expressed in **hundredths of a point** (HPt) to
//     match the OOXML `sz="…"` attribute and the existing
//     `FontSizeHPt` fields in `internal/textfit`. So 12pt = 1200.
//   - Spacing is expressed in **points** (Pt) to match the shape-grid
//     `gap` / `row_gap` / `col_gap` fields and the typographic-inset
//     conventions used throughout the codebase.
//   - Hex colors are uppercase 6-digit `#RRGGBB` form.
//
// Coverage roadmap: this package starts with the typography hierarchy
// (which RULES.md publishes to agents) and grows as scattered defaults
// are migrated. See bd issue go-slide-creator-clgi.
package tokens

// Typography hierarchy for shape_grid cells.
//
// These values mirror the published table in
// `skills/generate-deck/RULES.md` and the project AGENTS.md. The
// constants name the *role* (CardTitle, CardBody, StepNumber, …) rather
// than the size, so future point-size tweaks propagate through every
// reference automatically.
//
// Values are inclusive ranges where the *Min* / *Max* pair frames the
// healthy zone for that role. The *Default* is the value the engine
// reaches for when no explicit override is supplied.
const (
	// GridHeaderMinHPt / GridHeaderMaxHPt frame the banner / header
	// row of a shape_grid. Bold, white on accent fill, full-width.
	GridHeaderMinHPt     = 1400 // 14pt
	GridHeaderMaxHPt     = 1800 // 18pt
	GridHeaderDefaultHPt = 1600 // 16pt

	// CardTitle* frame the title line of a card cell. Bold, first
	// line of a `\n`-separated text run.
	CardTitleMinHPt     = 1200 // 12pt
	CardTitleMaxHPt     = 1400 // 14pt
	CardTitleDefaultHPt = 1300 // 13pt

	// CardBody* frame the body text of a card cell. Regular weight.
	// 11pt suits 3-4 cols, 10pt suits 5+ cols, 9pt is the floor.
	CardBodyMinHPt      = 900  // 9pt
	CardBodyMaxHPt      = 1100 // 11pt
	CardBodyDefaultHPt  = 1000 // 10pt
	CardBodyDenseHPt    = 900  // 9pt — for 5+ cols
	CardBodyRoomyHPt    = 1100 // 11pt — for 3-4 cols

	// StepNumber* frame a step or sequence numeral rendered in a narrow
	// accent column (e.g. roadmap, process-flow). Bold, white text.
	StepNumberMinHPt     = 2000 // 20pt
	StepNumberMaxHPt     = 2400 // 24pt
	StepNumberDefaultHPt = 2200 // 22pt

	// Footnote* frame source attribution and small print at the slide
	// bottom. Regular weight, grey (FootnoteColor).
	FootnoteMinHPt     = 700 // 7pt
	FootnoteMaxHPt     = 800 // 8pt
	FootnoteDefaultHPt = 800 // 8pt

	// BodyDefaultHPt is the fallback body font size used by textfit
	// when a template master does not surface its own body level. This
	// matches the typical slide master body level 1.
	BodyDefaultHPt = 2000 // 20pt
)

// Cell insets (text padding inside a shape_grid cell), expressed in
// typographic points. Body cells MUST set all four insets (rule 7 in
// RULES.md); these constants name the healthy range.
const (
	CellInsetMinPt     = 6
	CellInsetMaxPt     = 12
	CellInsetDefaultPt = 8
)

// Gaps (typographic points) between shape_grid rows and columns.
const (
	GridGapDefaultPt = 8
	GridGapTightPt   = 4
	GridGapDensePt   = 2
	GridGapLoosePt   = 12
)

// FootnoteColor is the canonical grey used for footnote/source text.
// Keep in sync with the RULES.md typography table.
const FootnoteColor = "#666666"

// TakeawayColor is the canonical dark fill used for the slide takeaway
// headline. Renders against a light slide background; on dark templates
// the renderer auto-flips per the WCAG contrast pass.
const TakeawayColor = "#1F1F1F"

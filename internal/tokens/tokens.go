// Package tokens is the single source of truth for cross-cutting design
// constants that previously lived scattered across textfit, patterns, and
// template metadata.
//
// Anything here is part of the project's design system and is referenced
// from documentation (skills/generate-deck/RULES.md typography table),
// validators (text fit defaults, readability floors), and renderers.
// Changing a value here changes the contract — bump SchemaVersion when
// renaming or removing a token.
//
// Conventions:
//   - Font sizes are expressed in **hundredths of a point** (HPt) to
//     match the OOXML `sz="…"` attribute and the existing
//     `FontSizeHPt` fields in `internal/textfit`. So 12pt = 1200. The
//     *Pt mirrors in typography.go serve pattern code, which sizes in points.
//
// The type scale (typography.go: TypeScale*) is the one authoring scale;
// RULES.md publishes it as the typography table. The role ladders that used
// to live here (grid header, card body 9-11pt, step number, 7-8pt footnote,
// cell insets, grid gaps) were read by no renderer and contradicted the 12pt
// readable floor, so they were removed (go-slide-creator-vmdfm).
package tokens

// Dense-report ("read" viewing mode) readability floors. These are policy
// floors that MinReadableHPt enforces for text read on screen or in print —
// never authoring sizes: authored text uses the type scale, whose body step
// is 12pt and caption step 10pt. In the default presentation mode every
// card role is held to 12pt and captions to 10pt.
const (
	// CardTitleMinHPt is the dense-report floor for card titles.
	CardTitleMinHPt = 1200 // 12pt
	// CardBodyMinHPt is the dense-report floor for card body copy.
	CardBodyMinHPt = 900 // 9pt
	// FootnoteMinHPt is the dense-report floor for footnotes and captions,
	// and the size below which autofit shrink is reported as unreadable.
	FootnoteMinHPt = 700 // 7pt

	// BodyDefaultHPt is the fallback body font size used by textfit
	// when a template master does not surface its own body level. This
	// matches the typical slide master body level 1.
	BodyDefaultHPt = 2000 // 20pt
)

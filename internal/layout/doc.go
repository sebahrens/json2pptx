// Package layout provides layout selection for slides.
//
// The layout package determines the optimal slide layout for content
// using deterministic heuristic scoring.
//
// # Layout Selection
//
// Layout selection uses heuristic scoring based on content analysis.
// The heuristic classifier analyzes content characteristics (text length,
// bullet count, image presence) to score layout candidates.
//
// # Layout Aliases
//
// Common layout names map to generated layout IDs:
//
//	"two-column"    → "content-2-50-50"
//	"sidebar-right" → "content-2-70-30"
//	"title"         → "slideLayout1"
//
// Use ResolveLayoutAlias to translate user-provided names.
package layout

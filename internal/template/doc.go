// Package template provides PPTX template analysis and layout extraction.
//
// The template package parses PowerPoint template files to extract layout
// definitions, color themes, typography settings, and placeholder positions.
// This metadata drives the slide generation process.
//
// # Template Analysis
//
// Template analysis extracts:
//
//   - Layout definitions with placeholder positions
//   - Master slide structure and inheritance
//   - Color themes with accent colors
//   - Typography settings (fonts, sizes)
//   - Layout classifications (title, content, two-column, etc.)
//
// # Architecture
//
// The package uses a reader/cache pattern:
//
//	TemplateReader → LayoutExtractor → Classifier → Cache
//
// Templates are parsed once and cached for subsequent requests.
// The cache uses LRU eviction with file modification tracking.
//
// # Layout Classification
//
// Layouts are classified by their structure:
//
//   - title: Title-only slides
//   - section: Section divider slides
//   - content: Single content area
//   - two-column: Two equal columns
//   - comparison: Side-by-side comparison
//   - picture: Image-focused layouts
//
// # Usage
//
//	reader := template.NewReader()
//	tmpl, err := reader.Read(templatePath)
//
//	layouts := tmpl.Layouts()
//	theme := tmpl.Theme()
//
// # Caching
//
// Template parsing results are cached with TTL-based expiration:
//
//	cache := template.NewCache(template.CacheConfig{
//	    TTL:     10 * time.Minute,
//	    MaxSize: 100,
//	})
//
// # Font Resolution
//
// The font resolver maps theme fonts to actual font families,
// handling fallbacks for missing fonts.
package template

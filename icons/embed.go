// Package icons embeds bundled SVG icon sets for zero-dependency deployment.
//
// Icons are organized into two sets:
//   - filled/  — solid/filled style icons
//   - outline/ — outline/stroke style icons
//
// Use [Lookup] to retrieve an icon by name. Names can be plain ("chart-pie")
// which defaults to the outline set, or prefixed ("filled:chart-pie") to
// select a specific set.
package icons

import "embed"

//go:embed filled/*.svg outline/*.svg
var FS embed.FS

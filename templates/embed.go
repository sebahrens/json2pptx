// Package templates embeds the default .pptx template files into the binary.
// This allows md2pptx to work without a separate templates directory on disk.
// The per-layout preview thumbnails under previews/ (make template-previews)
// are embedded too, so recommend_visual can point at them for embedded
// templates.
package templates

import "embed"

// Keep this list explicit: a local, gitignored PPTX in templates/ must not
// silently become part of the shipped binary or change built-in discovery.
// TestBuiltinTemplateCoverage checks it against tracked template files.
//
//go:embed abstract.pptx blue-corporate.pptx business-template.pptx forest-green.pptx midnight-blue.pptx modern-template.pptx modern-yellow.pptx modern.pptx warm-coral.pptx previews
var Embedded embed.FS

// Package templates embeds the default .pptx template files into the binary.
// This allows md2pptx to work without a separate templates directory on disk.
// The per-layout preview thumbnails under previews/ (make template-previews)
// are embedded too, so recommend_visual can point at them for embedded
// templates.
package templates

import "embed"

//go:embed *.pptx previews
var Embedded embed.FS

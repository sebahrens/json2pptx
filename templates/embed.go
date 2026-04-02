// Package templates embeds the default .pptx template files into the binary.
// This allows md2pptx to work without a separate templates directory on disk.
package templates

import "embed"

//go:embed *.pptx
var Embedded embed.FS

// Package skills embeds the agent-facing authoring documents that ship with
// the binary, so a server can serve them without a copy of the repository next
// to it (go-slide-creator-fx52). The MCP surface exposes SKILL.md as the
// json2pptx://skill resource: a host caches it once and every session after
// that reads it for free, instead of the guidance being reachable only through
// tool descriptions.
package skills

import (
	"embed"
	"io/fs"
)

//go:embed generate-deck/SKILL.md
var skillMarkdown string

// bundle is every skill the binary ships, one directory each, so
// `json2pptx skill install` can write the copy that matches the binary
// (go-slide-creator-4eu2o).
//
//go:embed all:generate-deck all:template-deck all:slide-visual-qa all:render-diagram
var bundle embed.FS

// Bundle returns the shipped skills as a file system rooted at the skills
// directory (generate-deck/SKILL.md, template-deck/TEMPLATE_GUIDE.md, …).
func Bundle() fs.FS { return bundle }

// SkillMarkdown returns the deck-authoring skill document.
func SkillMarkdown() string { return skillMarkdown }

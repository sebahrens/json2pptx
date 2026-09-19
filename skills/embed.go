// Package skills embeds the agent-facing authoring documents that ship with
// the binary, so a server can serve them without a copy of the repository next
// to it (go-slide-creator-fx52). The MCP surface exposes SKILL.md as the
// json2pptx://skill resource: a host caches it once and every session after
// that reads it for free, instead of the guidance being reachable only through
// tool descriptions.
package skills

import (
	_ "embed"
)

//go:embed generate-deck/SKILL.md
var skillMarkdown string

// SkillMarkdown returns the deck-authoring skill document.
func SkillMarkdown() string { return skillMarkdown }

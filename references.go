// Package json2pptx is the module root. It holds no code of the product: it
// exists to embed the repository files the agent skills link to — the
// contributor docs, the semantic examples, the token sources and the
// source-aware evidence examples — so `json2pptx skill install` can write them
// beside the skills and every link resolves on a machine with no checkout
// (go-slide-creator-4eu2o). Go's embed cannot reach a parent directory, and
// these files live in four of them; the module root is the one directory
// above them all.
package json2pptx

import (
	"embed"
	"io/fs"
)

// Keep the list explicit: the snapshot is what the skills link to, not the
// repository. TestSkillInstallReferencesResolve fails when an installed link
// has no file behind it.
//
//go:embed docs/INPUT_FORMAT.md docs/INPUT_FORMAT_ADVANCED.md docs/FIT_FINDINGS.md docs/SEMANTIC_COMPILER.md docs/TEMPLATE_SPEC.md docs/PATH_GRAMMAR.md docs/PATTERNS.md docs/TEMPLATE_ANALYSIS.md
//go:embed all:examples/semantic all:internal/tokens
//go:embed tests/quality/evidence/connectors/midnight-blue/source-aware-evidence-route.json
//go:embed tests/quality/evidence/connectors/midnight-blue/readable-source-companion-route.json
//go:embed tests/quality/evidence/connectors/midnight-blue/powerpoint-slide-4.png
var skillReferences embed.FS

// SkillReferences returns the repository files the skills link to, at their
// repository-relative paths (docs/PATTERNS.md, examples/semantic/qbr.yaml, …).
func SkillReferences() fs.FS { return skillReferences }
